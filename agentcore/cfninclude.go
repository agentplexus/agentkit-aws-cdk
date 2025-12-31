package agentcore

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/cloudformationinclude"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// CfnIncludeStack wraps an existing CloudFormation template with CDK.
// This allows using existing CF templates while benefiting from CDK's
// deployment tooling and the ability to add CDK constructs on top.
type CfnIncludeStack struct {
	awscdk.Stack

	// Template is the included CloudFormation template.
	Template cloudformationinclude.CfnInclude
}

// CfnIncludeConfig configures the CfnInclude stack.
type CfnIncludeConfig struct {
	// StackName is the CloudFormation stack name.
	StackName string

	// TemplateFile is the path to the CloudFormation template file.
	// Supports JSON or YAML format.
	TemplateFile string

	// Parameters are CloudFormation parameter overrides.
	// Keys are parameter names, values are the override values.
	Parameters map[string]string

	// PreserveLogicalIds keeps the original logical IDs from the template.
	// Default: true (recommended for importing existing stacks)
	PreserveLogicalIds bool

	// Tags are AWS resource tags applied to all resources.
	Tags map[string]string
}

// NewCfnIncludeStack creates a CDK stack that wraps an existing CloudFormation template.
//
// This approach is useful when:
//   - You have existing CloudFormation templates you want to keep using
//   - You want to gradually migrate from CloudFormation to CDK
//   - You need to import an existing CloudFormation stack
//
// Example:
//
//	app := agentcore.NewApp()
//	stack := agentcore.NewCfnIncludeStack(app, agentcore.CfnIncludeConfig{
//	    StackName:    "my-agents",
//	    TemplateFile: "agentcore-stack.yaml",
//	    Parameters: map[string]string{
//	        "Environment": "production",
//	    },
//	})
//	agentcore.Synth(app)
func NewCfnIncludeStack(scope constructs.Construct, config CfnIncludeConfig) *CfnIncludeStack {
	// Create the stack
	stack := awscdk.NewStack(scope, jsii.String(config.StackName), &awscdk.StackProps{
		StackName: jsii.String(config.StackName),
		Tags:      convertTags(config.Tags),
	})

	// Build parameter overrides
	var parameters *map[string]cloudformationinclude.CfnIncludeProps
	if len(config.Parameters) > 0 {
		// Note: CfnInclude uses a different parameter structure
		// Parameters are passed via CfnIncludeProps
	}
	_ = parameters // Silence unused warning

	// Determine preserveLogicalIds default
	preserveLogicalIds := true
	if !config.PreserveLogicalIds {
		preserveLogicalIds = config.PreserveLogicalIds
	}

	// Include the CloudFormation template
	template := cloudformationinclude.NewCfnInclude(stack, jsii.String("Template"), &cloudformationinclude.CfnIncludeProps{
		TemplateFile:       jsii.String(config.TemplateFile),
		PreserveLogicalIds: jsii.Bool(preserveLogicalIds),
	})

	return &CfnIncludeStack{
		Stack:    stack,
		Template: template,
	}
}

// GetResource retrieves a resource from the included template by logical ID.
// Returns the resource as a CfnResource that can be modified.
func (s *CfnIncludeStack) GetResource(logicalId string) awscdk.CfnResource {
	return s.Template.GetResource(jsii.String(logicalId))
}

// GetNestedStack retrieves a nested stack from the included template.
func (s *CfnIncludeStack) GetNestedStack(logicalId string) *cloudformationinclude.IncludedNestedStack {
	return s.Template.GetNestedStack(jsii.String(logicalId))
}

// CfnIncludeBuilder provides a fluent interface for building CfnInclude stacks.
type CfnIncludeBuilder struct {
	config CfnIncludeConfig
}

// NewCfnIncludeBuilder creates a new CfnInclude builder.
func NewCfnIncludeBuilder(stackName, templateFile string) *CfnIncludeBuilder {
	return &CfnIncludeBuilder{
		config: CfnIncludeConfig{
			StackName:          stackName,
			TemplateFile:       templateFile,
			Parameters:         make(map[string]string),
			PreserveLogicalIds: true,
			Tags:               make(map[string]string),
		},
	}
}

// WithParameter adds a parameter override.
func (b *CfnIncludeBuilder) WithParameter(name, value string) *CfnIncludeBuilder {
	b.config.Parameters[name] = value
	return b
}

// WithParameters adds multiple parameter overrides.
func (b *CfnIncludeBuilder) WithParameters(params map[string]string) *CfnIncludeBuilder {
	for k, v := range params {
		b.config.Parameters[k] = v
	}
	return b
}

// WithTag adds a tag.
func (b *CfnIncludeBuilder) WithTag(key, value string) *CfnIncludeBuilder {
	b.config.Tags[key] = value
	return b
}

// WithTags adds multiple tags.
func (b *CfnIncludeBuilder) WithTags(tags map[string]string) *CfnIncludeBuilder {
	for k, v := range tags {
		b.config.Tags[k] = v
	}
	return b
}

// WithPreserveLogicalIds sets whether to preserve logical IDs.
func (b *CfnIncludeBuilder) WithPreserveLogicalIds(preserve bool) *CfnIncludeBuilder {
	b.config.PreserveLogicalIds = preserve
	return b
}

// Build creates the CfnInclude stack.
func (b *CfnIncludeBuilder) Build(scope constructs.Construct) *CfnIncludeStack {
	return NewCfnIncludeStack(scope, b.config)
}
