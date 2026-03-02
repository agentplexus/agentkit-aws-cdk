// Example 4: Pure CloudFormation Generator
//
// This tool generates a CloudFormation template from a JSON/YAML config file.
// No CDK runtime needed - just deploy with AWS CLI.
//
// Usage:
//
//	go run generate.go                     # Generate from config.yaml
//	go run generate.go config.json         # Generate from specific file
//
// Then deploy with:
//
//	aws cloudformation deploy \
//	  --template-file template.yaml \
//	  --stack-name stats-agent-team \
//	  --capabilities CAPABILITY_IAM CAPABILITY_NAMED_IAM
package main

import (
	"fmt"
	"os"

	"github.com/plexusone/agentkit/platforms/agentcore/iac"
)

func main() {
	// Determine input config file
	configFile := "config.yaml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	// Load configuration
	config, err := iac.LoadStackConfigFromFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Generate CloudFormation template
	outputFile := "template.yaml"
	if err := iac.GenerateCloudFormationFile(config, outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating CloudFormation: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s from %s\n", outputFile, configFile)
	fmt.Printf("\nDeploy with:\n")
	fmt.Printf("  aws cloudformation deploy \\\n")
	fmt.Printf("    --template-file %s \\\n", outputFile)
	fmt.Printf("    --stack-name %s \\\n", config.StackName)
	fmt.Printf("    --capabilities CAPABILITY_IAM CAPABILITY_NAMED_IAM\n")
}
