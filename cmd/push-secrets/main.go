// push-secrets pushes environment variables from .env files to AWS Secrets Manager.
//
// It reads KEY=VALUE pairs from a file and creates/updates secrets in AWS Secrets Manager,
// organizing them into logical groups (llm, search, config).
//
// Usage:
//
//	push-secrets [flags] [env-file]
//
// Examples:
//
//	push-secrets .env                          # Push from .env to us-east-1
//	push-secrets --region us-west-2 .env       # Push to specific region
//	push-secrets --prefix myapp .env           # Use custom prefix (myapp/llm, myapp/search, etc.)
//	push-secrets --dry-run .env                # Preview without creating
//
// Install:
//
//	go install github.com/plexusone/agentkit-aws-cdk/cmd/push-secrets@latest
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

const (
	// DefaultConfigDir is the default directory for plexusone configuration
	DefaultConfigDir = ".plexusone"
)

// SecretGroup represents a logical grouping of secrets
type SecretGroup struct {
	Name        string
	Description string
	Keys        map[string]string
	Patterns    []string // Key patterns that belong to this group
}

var (
	region  = flag.String("region", "", "AWS region (default: AWS_REGION or us-east-1)")
	prefix  = flag.String("prefix", "stats-agent", "Secret name prefix")
	project = flag.String("project", "", "Project name for ~/.plexusone/projects/{project}/.env lookup")
	dryRun  = flag.Bool("dry-run", false, "Preview changes without creating secrets")
	verbose = flag.Bool("verbose", false, "Show verbose output")
)

func main() {
	flag.Usage = func() {
		//nolint:gosec // G705: os.Args[0] in CLI usage text is safe
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [env-file]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Push environment variables to AWS Secrets Manager.\n\n")
		fmt.Fprintf(os.Stderr, "If env-file is not specified, searches in order:\n")
		fmt.Fprintf(os.Stderr, "  1. .env (current directory)\n")
		fmt.Fprintf(os.Stderr, "  2. ../.env (parent directory)\n")
		fmt.Fprintf(os.Stderr, "  3. ~/.plexusone/projects/{project}/.env (if --project specified)\n")
		fmt.Fprintf(os.Stderr, "  4. ~/.plexusone/.env (global fallback)\n\n")
		fmt.Fprintf(os.Stderr, "Project is auto-detected from config.json stackName if not specified.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		//nolint:gosec // G705: os.Args[0] in CLI usage text is safe
		fmt.Fprintf(os.Stderr, "  %s                           # Auto-detect env file\n", os.Args[0])
		//nolint:gosec // G705: os.Args[0] in CLI usage text is safe
		fmt.Fprintf(os.Stderr, "  %s --project stats-agent-team # Use project-specific env\n", os.Args[0])
		//nolint:gosec // G705: os.Args[0] in CLI usage text is safe
		fmt.Fprintf(os.Stderr, "  %s .env                      # Push from .env\n", os.Args[0])
		//nolint:gosec // G705: os.Args[0] in CLI usage text is safe
		fmt.Fprintf(os.Stderr, "  %s --region us-west-2 .env   # Push to specific region\n", os.Args[0])
		//nolint:gosec // G705: os.Args[0] in CLI usage text is safe
		fmt.Fprintf(os.Stderr, "  %s --dry-run .env            # Preview without creating\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nSecret Groups:\n")
		fmt.Fprintf(os.Stderr, "  {prefix}/llm     - LLM provider API keys (GOOGLE_API_KEY, OPENAI_API_KEY, etc.)\n")
		fmt.Fprintf(os.Stderr, "  {prefix}/search  - Search provider keys (SERPER_API_KEY, SERPAPI_API_KEY)\n")
		fmt.Fprintf(os.Stderr, "  {prefix}/config  - Configuration and observability settings\n")
	}
	flag.Parse()

	// Detect project name
	projectName := *project
	if projectName == "" {
		projectName = detectProjectName()
	}

	var envFile string
	if flag.NArg() >= 1 {
		envFile = flag.Arg(0)
	} else {
		// Auto-detect env file
		var err error
		envFile, err = findEnvFile(projectName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "\nCreate ~/.plexusone/.env or ~/.plexusone/projects/%s/.env\n", projectName)
			os.Exit(1)
		}
	}

	// Determine region
	awsRegion := *region
	if awsRegion == "" {
		awsRegion = os.Getenv("AWS_REGION")
	}
	if awsRegion == "" {
		awsRegion = os.Getenv("AWS_DEFAULT_REGION")
	}
	if awsRegion == "" {
		awsRegion = "us-east-1"
	}

	if err := run(envFile, awsRegion, *prefix, *dryRun, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(envFile, region, prefix string, dryRun, verbose bool) error {
	// Define secret groups
	groups := []SecretGroup{
		{
			Name:        "llm",
			Description: "LLM provider API keys",
			Keys:        make(map[string]string),
			Patterns: []string{
				"GOOGLE_API_KEY",
				"GEMINI_API_KEY",
				"ANTHROPIC_API_KEY",
				"CLAUDE_API_KEY",
				"OPENAI_API_KEY",
				"XAI_API_KEY",
				"LLM_API_KEY",
			},
		},
		{
			Name:        "search",
			Description: "Search provider API keys",
			Keys:        make(map[string]string),
			Patterns: []string{
				"SERPER_API_KEY",
				"SERPAPI_API_KEY",
			},
		},
		{
			Name:        "config",
			Description: "Configuration and observability settings",
			Keys:        make(map[string]string),
			Patterns: []string{
				"LLM_PROVIDER",
				"LLM_MODEL",
				"LLM_BASE_URL",
				"SEARCH_PROVIDER",
				"OBSERVABILITY_ENABLED",
				"OBSERVABILITY_PROVIDER",
				"OPIK_API_KEY",
				"OPIK_WORKSPACE",
				"OPIK_PROJECT",
				"LANGFUSE_PUBLIC_KEY",
				"LANGFUSE_SECRET_KEY",
				"PHOENIX_API_KEY",
			},
		},
	}

	// Parse env file
	fmt.Printf("Reading from: %s\n", envFile)
	if err := parseEnvFile(envFile, groups, verbose); err != nil {
		return fmt.Errorf("parsing env file: %w", err)
	}

	fmt.Printf("AWS Region: %s\n", region)
	fmt.Printf("Secret prefix: %s\n", prefix)
	if dryRun {
		fmt.Printf("Mode: DRY RUN (no changes will be made)\n")
	}
	fmt.Println()

	// Create AWS client
	var client *secretsmanager.Client
	if !dryRun {
		cfg, err := config.LoadDefaultConfig(context.Background(),
			config.WithRegion(region),
		)
		if err != nil {
			return fmt.Errorf("loading AWS config: %w", err)
		}
		client = secretsmanager.NewFromConfig(cfg)
	}

	// Process each group
	ctx := context.Background()
	for _, group := range groups {
		secretName := fmt.Sprintf("%s/%s", prefix, group.Name)
		if err := processGroup(ctx, client, secretName, group, dryRun); err != nil {
			return fmt.Errorf("processing %s: %w", secretName, err)
		}
	}

	fmt.Println()
	fmt.Println("Done!")
	fmt.Println()
	fmt.Printf("To verify:\n")
	fmt.Printf("  aws secretsmanager list-secrets --region %s --filter Key=name,Values=%s/ --no-cli-pager\n", region, prefix)

	return nil
}

func parseEnvFile(filename string, groups []SecretGroup, verbose bool) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Regex to match: optional "export", KEY, =, VALUE
	envRegex := regexp.MustCompile(`^\s*(export\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		matches := envRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		key := matches[2]
		value := matches[3]

		// Remove surrounding quotes
		value = strings.Trim(value, `"'`)

		// Skip empty or placeholder values
		if value == "" || strings.HasPrefix(value, "your-") {
			continue
		}

		// Categorize into groups
		for i := range groups {
			for _, pattern := range groups[i].Patterns {
				if key == pattern {
					groups[i].Keys[key] = value
					if verbose {
						fmt.Printf("  Found %s key: %s\n", groups[i].Name, key)
					}
					break
				}
			}
		}
	}

	return scanner.Err()
}

func processGroup(ctx context.Context, client *secretsmanager.Client, secretName string, group SecretGroup, dryRun bool) error {
	if len(group.Keys) == 0 {
		fmt.Printf("Skipping %s (no keys found)\n", secretName)
		return nil
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(group.Keys)
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	secretValue := string(jsonBytes)

	fmt.Printf("Creating/updating: %s\n", secretName)

	// Show keys found
	var keyNames []string
	for k := range group.Keys {
		keyNames = append(keyNames, k)
	}
	fmt.Printf("  Keys: %s\n", strings.Join(keyNames, ", "))

	if dryRun {
		// Mask sensitive values for display
		masked := maskSecretValues(secretValue)
		fmt.Printf("  [DRY RUN] Would create with: %s\n", masked)
		return nil
	}

	// Try to update existing secret first
	_, err = client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(secretValue),
	})
	if err != nil {
		// Check if secret doesn't exist
		var notFound *types.ResourceNotFoundException
		if errors.As(err, &notFound) {
			// Create new secret
			_, err = client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
				Name:         aws.String(secretName),
				Description:  aws.String(group.Description),
				SecretString: aws.String(secretValue),
			})
			if err != nil {
				return fmt.Errorf("creating secret: %w", err)
			}
			fmt.Printf("  Created new secret\n")
			return nil
		}
		return fmt.Errorf("updating secret: %w", err)
	}

	fmt.Printf("  Updated existing secret\n")
	return nil
}

func maskSecretValues(jsonStr string) string {
	// Mask API key values, showing only first 8 chars
	re := regexp.MustCompile(`("(?:[^"]*API_KEY|KEY)[^"]*"\s*:\s*")([^"]{8})([^"]*)"`)
	return re.ReplaceAllString(jsonStr, `$1$2***"`)
}

// findEnvFile searches for .env file in standard locations
func findEnvFile(projectName string) (string, error) {
	// Search order:
	// 1. .env in current directory
	// 2. ../.env in parent directory
	// 3. ~/.plexusone/projects/{project}/.env (if project specified)
	// 4. ~/.plexusone/.env (global fallback)

	candidates := []string{
		".env",
		"../.env",
	}

	// Add project-specific and global paths
	if home, err := os.UserHomeDir(); err == nil {
		if projectName != "" {
			candidates = append(candidates, filepath.Join(home, DefaultConfigDir, "projects", projectName, ".env"))
		}
		candidates = append(candidates, filepath.Join(home, DefaultConfigDir, ".env"))
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no .env file found in: .env, ../.env, or ~/.plexusone/")
}

// detectProjectName tries to detect the project name from config.json or directory name
func detectProjectName() string {
	// Try to read stackName from config.json
	configPaths := []string{"config.json", "../config.json"}
	for _, path := range configPaths {
		if data, err := os.ReadFile(path); err == nil {
			var config struct {
				StackName string `json:"stackName"`
			}
			if json.Unmarshal(data, &config) == nil && config.StackName != "" {
				return config.StackName
			}
		}
	}

	// Fall back to current directory name
	if wd, err := os.Getwd(); err == nil {
		return filepath.Base(wd)
	}

	return ""
}
