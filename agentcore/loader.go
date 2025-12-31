// Package agentcore provides AWS CDK constructs for AgentCore deployments.
package agentcore

import (
	"fmt"

	"github.com/agentplexus/agentkit/platforms/agentcore/iac"
	"github.com/aws/constructs-go/constructs/v10"
)

// Re-export config loading functions from agentkit for convenience.
var (
	// LoadStackConfigFromFile loads a StackConfig from a JSON or YAML file.
	LoadStackConfigFromFile = iac.LoadStackConfigFromFile

	// LoadStackConfigFromJSON parses a StackConfig from JSON data.
	LoadStackConfigFromJSON = iac.LoadStackConfigFromJSON

	// LoadStackConfigFromYAML parses a StackConfig from YAML data.
	LoadStackConfigFromYAML = iac.LoadStackConfigFromYAML

	// JSONConfigExample returns an example JSON configuration.
	JSONConfigExample = iac.JSONConfigExample

	// YAMLConfigExample returns an example YAML configuration.
	YAMLConfigExample = iac.YAMLConfigExample

	// WriteExampleConfig writes an example configuration file.
	WriteExampleConfig = iac.WriteExampleConfig

	// GenerateCloudFormation generates a CloudFormation template from StackConfig.
	GenerateCloudFormation = iac.GenerateCloudFormation

	// GenerateCloudFormationFile generates a CloudFormation template and writes it to a file.
	GenerateCloudFormationFile = iac.GenerateCloudFormationFile

	// GenerateCloudFormationFromFile loads a config file and generates CloudFormation.
	GenerateCloudFormationFromFile = iac.GenerateCloudFormationFromFile
)

// NewStackFromFile creates an AgentCoreStack from a JSON or YAML config file.
// This is the simplest way to deploy - just provide a config file.
func NewStackFromFile(scope constructs.Construct, configPath string) (*AgentCoreStack, error) {
	config, err := iac.LoadStackConfigFromFile(configPath)
	if err != nil {
		return nil, err
	}

	return NewAgentCoreStack(scope, config.StackName, *config), nil
}

// MustNewStackFromFile is like NewStackFromFile but panics on error.
func MustNewStackFromFile(scope constructs.Construct, configPath string) *AgentCoreStack {
	stack, err := NewStackFromFile(scope, configPath)
	if err != nil {
		panic(fmt.Sprintf("failed to create stack from %s: %v", configPath, err))
	}
	return stack
}

// NewStackFromJSON creates an AgentCoreStack from JSON data.
func NewStackFromJSON(scope constructs.Construct, jsonData []byte) (*AgentCoreStack, error) {
	config, err := iac.LoadStackConfigFromJSON(jsonData)
	if err != nil {
		return nil, err
	}

	return NewAgentCoreStack(scope, config.StackName, *config), nil
}

// NewStackFromYAML creates an AgentCoreStack from YAML data.
func NewStackFromYAML(scope constructs.Construct, yamlData []byte) (*AgentCoreStack, error) {
	config, err := iac.LoadStackConfigFromYAML(yamlData)
	if err != nil {
		return nil, err
	}

	return NewAgentCoreStack(scope, config.StackName, *config), nil
}
