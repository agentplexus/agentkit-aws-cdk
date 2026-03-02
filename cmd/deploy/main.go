// deploy orchestrates the full AWS AgentCore deployment process.
//
// It handles:
//  1. Pushing secrets from .env to AWS Secrets Manager
//  2. Bootstrapping AWS CDK
//  3. Deploying the CDK stack
//
// Usage:
//
//	deploy [flags]
//
// Examples:
//
//	deploy                              # Deploy from current directory
//	deploy --env ../.env                # Specify env file location
//	deploy --region us-west-2           # Deploy to specific region
//	deploy --dry-run                    # Preview without deploying
//	deploy --skip-secrets               # Skip secrets push (if already created)
//
// Install:
//
//	go install github.com/plexusone/agentkit-aws-cdk/cmd/deploy@latest
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	// DefaultConfigDir is the default directory for plexusone configuration
	DefaultConfigDir = ".plexusone"
)

var (
	region        = flag.String("region", "", "AWS region (default: AWS_REGION or us-east-1)")
	envFile       = flag.String("env", "", "Path to .env file (default: auto-detect)")
	prefix        = flag.String("prefix", "stats-agent", "Secret name prefix")
	project       = flag.String("project", "", "Project name for ~/.plexusone/projects/{project}/.env lookup")
	dryRun        = flag.Bool("dry-run", false, "Preview changes without deploying")
	skipSecrets   = flag.Bool("skip-secrets", false, "Skip pushing secrets")
	skipBootstrap = flag.Bool("skip-bootstrap", false, "Skip CDK bootstrap")
	verbose       = flag.Bool("verbose", false, "Show verbose output")
)

func main() {
	flag.Usage = func() {
		//nolint:gosec // G705: os.Args[0] in CLI usage text is safe
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Deploy to AWS AgentCore.\n\n")
		fmt.Fprintf(os.Stderr, "Env file search order (if --env not specified):\n")
		fmt.Fprintf(os.Stderr, "  1. .env (current directory)\n")
		fmt.Fprintf(os.Stderr, "  2. ../.env (parent directory)\n")
		fmt.Fprintf(os.Stderr, "  3. ~/.plexusone/projects/{project}/.env (if --project specified)\n")
		fmt.Fprintf(os.Stderr, "  4. ~/.plexusone/.env (global fallback)\n\n")
		fmt.Fprintf(os.Stderr, "Project is auto-detected from config.json stackName if not specified.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSteps:\n")
		fmt.Fprintf(os.Stderr, "  1. Push secrets from .env to AWS Secrets Manager\n")
		fmt.Fprintf(os.Stderr, "  2. Bootstrap AWS CDK (if needed)\n")
		fmt.Fprintf(os.Stderr, "  3. Deploy CDK stack\n")
	}
	flag.Parse()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
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

	// Detect project name
	projectName := *project
	if projectName == "" {
		projectName = detectProjectName()
	}

	fmt.Println("=== AWS AgentCore Deployment ===")
	fmt.Println()
	fmt.Printf("Region: %s\n", awsRegion)
	if projectName != "" {
		fmt.Printf("Project: %s\n", projectName)
	}
	fmt.Printf("Working directory: %s\n", mustGetwd())
	if *dryRun {
		fmt.Println("Mode: DRY RUN (no changes will be made)")
	}
	fmt.Println()

	ctx := context.Background()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	// Get account ID
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("getting AWS identity: %w", err)
	}
	accountID := *identity.Account
	fmt.Printf("AWS Account: %s\n", accountID)
	fmt.Println()

	// Step 1: Push secrets
	if !*skipSecrets {
		fmt.Println("=== Step 1: Push Secrets ===")
		if err := pushSecrets(ctx, cfg, *envFile, *prefix, projectName, *dryRun, *verbose); err != nil {
			return fmt.Errorf("pushing secrets: %w", err)
		}
		fmt.Println()
	} else {
		fmt.Println("=== Step 1: Skipping secrets (--skip-secrets) ===")
		fmt.Println()
	}

	// Step 2: Bootstrap CDK
	if !*skipBootstrap {
		fmt.Println("=== Step 2: Bootstrap CDK ===")
		bootstrapCDK(ctx, accountID, awsRegion, *dryRun)
		fmt.Println()
	} else {
		fmt.Println("=== Step 2: Skipping bootstrap (--skip-bootstrap) ===")
		fmt.Println()
	}

	// Step 3: Deploy
	fmt.Println("=== Step 3: Deploy ===")
	if err := deployCDK(ctx, *dryRun); err != nil {
		return fmt.Errorf("deploying: %w", err)
	}
	fmt.Println()

	fmt.Println("=== Deployment Complete ===")
	if !*dryRun {
		fmt.Println()
		fmt.Println("To get outputs:")
		fmt.Printf("  aws cloudformation describe-stacks --stack-name stats-agent-team --region %s --query 'Stacks[0].Outputs' --no-cli-pager\n", awsRegion)
	}

	return nil
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// pushSecrets pushes environment variables to AWS Secrets Manager
func pushSecrets(ctx context.Context, cfg aws.Config, envFile, prefix, projectName string, dryRun, verbose bool) error {
	// Find env file
	var envPath string
	if envFile != "" {
		envPath = envFile
		if !filepath.IsAbs(envPath) {
			// Try relative to current directory, then parent
			if _, err := os.Stat(envPath); os.IsNotExist(err) {
				parentPath := filepath.Join("..", envPath)
				if _, err := os.Stat(parentPath); err == nil {
					envPath = parentPath
				}
			}
		}
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			fmt.Printf("Warning: %s not found, skipping secrets push\n", envFile)
			return nil
		}
	} else {
		// Auto-detect env file
		var err error
		envPath, err = findEnvFile(projectName)
		if err != nil {
			fmt.Println("No .env file found, skipping secrets push")
			fmt.Println("  Searched: .env, ../.env, ~/.plexusone/")
			return nil
		}
	}

	fmt.Printf("Reading from: %s\n", envPath)

	// Define secret groups
	groups := []secretGroup{
		{
			name:        "llm",
			description: "LLM provider API keys",
			keys:        make(map[string]string),
			patterns: []string{
				"GOOGLE_API_KEY", "GEMINI_API_KEY", "ANTHROPIC_API_KEY",
				"CLAUDE_API_KEY", "OPENAI_API_KEY", "XAI_API_KEY", "LLM_API_KEY",
			},
		},
		{
			name:        "search",
			description: "Search provider API keys",
			keys:        make(map[string]string),
			patterns:    []string{"SERPER_API_KEY", "SERPAPI_API_KEY"},
		},
		{
			name:        "config",
			description: "Configuration and observability settings",
			keys:        make(map[string]string),
			patterns: []string{
				"LLM_PROVIDER", "LLM_MODEL", "LLM_BASE_URL", "SEARCH_PROVIDER",
				"OBSERVABILITY_ENABLED", "OBSERVABILITY_PROVIDER",
				"OPIK_API_KEY", "OPIK_WORKSPACE", "OPIK_PROJECT",
				"LANGFUSE_PUBLIC_KEY", "LANGFUSE_SECRET_KEY", "PHOENIX_API_KEY",
			},
		},
	}

	// Parse env file
	if err := parseEnvFile(envPath, groups, verbose); err != nil {
		return err
	}

	// Create secrets client
	var client *secretsmanager.Client
	if !dryRun {
		client = secretsmanager.NewFromConfig(cfg)
	}

	// Process each group
	for _, group := range groups {
		secretName := fmt.Sprintf("%s/%s", prefix, group.name)
		if err := createOrUpdateSecret(ctx, client, secretName, group, dryRun); err != nil {
			return err
		}
	}

	return nil
}

type secretGroup struct {
	name        string
	description string
	keys        map[string]string
	patterns    []string
}

func parseEnvFile(filename string, groups []secretGroup, verbose bool) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	envRegex := regexp.MustCompile(`^\s*(export\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		matches := envRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		key := matches[2]
		value := strings.Trim(matches[3], `"'`)

		if value == "" || strings.HasPrefix(value, "your-") {
			continue
		}

		for i := range groups {
			for _, pattern := range groups[i].patterns {
				if key == pattern {
					groups[i].keys[key] = value
					if verbose {
						fmt.Printf("  Found %s: %s\n", groups[i].name, key)
					}
					break
				}
			}
		}
	}

	return scanner.Err()
}

func createOrUpdateSecret(ctx context.Context, client *secretsmanager.Client, secretName string, group secretGroup, dryRun bool) error {
	if len(group.keys) == 0 {
		fmt.Printf("  Skipping %s (no keys found)\n", secretName)
		return nil
	}

	jsonBytes, err := json.Marshal(group.keys)
	if err != nil {
		return err
	}
	secretValue := string(jsonBytes)

	var keyNames []string
	for k := range group.keys {
		keyNames = append(keyNames, k)
	}
	fmt.Printf("  %s: %s\n", secretName, strings.Join(keyNames, ", "))

	if dryRun {
		fmt.Printf("    [DRY RUN] Would create/update\n")
		return nil
	}

	_, err = client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(secretValue),
	})
	if err != nil {
		var notFound *types.ResourceNotFoundException
		if errors.As(err, &notFound) {
			_, err = client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
				Name:         aws.String(secretName),
				Description:  aws.String(group.description),
				SecretString: aws.String(secretValue),
			})
			if err != nil {
				return err
			}
			fmt.Printf("    Created\n")
			return nil
		}
		return err
	}
	fmt.Printf("    Updated\n")
	return nil
}

// bootstrapCDK runs cdk bootstrap
func bootstrapCDK(ctx context.Context, accountID, region string, dryRun bool) {
	target := fmt.Sprintf("aws://%s/%s", accountID, region)
	fmt.Printf("Bootstrap target: %s\n", target)

	if dryRun {
		fmt.Println("[DRY RUN] Would run: cdk bootstrap " + target)
		return
	}

	//nolint:gosec // G702: target is built from AWS SDK values (accountID, region), not user input
	cmd := exec.CommandContext(ctx, "cdk", "bootstrap", target)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Bootstrap might fail if already done, that's OK
		fmt.Println("  Bootstrap completed (or already bootstrapped)")
	}
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

	return "", fmt.Errorf("no .env file found")
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

// deployCDK runs cdk deploy
func deployCDK(ctx context.Context, dryRun bool) error {
	// Run go mod tidy first
	fmt.Println("Running go mod tidy...")
	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		fmt.Printf("Warning: go mod tidy failed: %v\n", err)
	}

	if dryRun {
		fmt.Println("Running cdk diff...")
		cmd := exec.CommandContext(ctx, "cdk", "diff")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // Ignore error, diff returns non-zero if there are differences
		return nil
	}

	fmt.Println("Running cdk deploy...")
	cmd := exec.CommandContext(ctx, "cdk", "deploy", "--require-approval", "never")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
