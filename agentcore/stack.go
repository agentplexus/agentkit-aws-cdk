package agentcore

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsbedrockagentcore"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/plexusone/agentkit/platforms/agentcore/iac"
)

// Type aliases for convenience - re-export from agentkit.
type (
	StackConfig         = iac.StackConfig
	AgentConfig         = iac.AgentConfig
	VPCConfig           = iac.VPCConfig
	SecretsConfig       = iac.SecretsConfig
	ObservabilityConfig = iac.ObservabilityConfig
	IAMConfig           = iac.IAMConfig
	AuthorizerConfig    = iac.AuthorizerConfig
	GatewayConfig       = iac.GatewayConfig
)

// Re-export default config functions from agentkit.
var (
	DefaultAgentConfig          = iac.DefaultAgentConfig
	DefaultVPCConfig            = iac.DefaultVPCConfig
	DefaultObservabilityConfig  = iac.DefaultObservabilityConfig
	DefaultIAMConfig            = iac.DefaultIAMConfig
	ValidMemoryValues           = iac.ValidMemoryValues
	ValidObservabilityProviders = iac.ValidObservabilityProviders
)

// AgentCoreStack is a CDK stack that deploys agents to AWS Bedrock AgentCore.
type AgentCoreStack struct {
	awscdk.Stack

	// Config is the stack configuration.
	Config StackConfig

	// VPC is the VPC used by the agents.
	VPC awsec2.IVpc

	// SecurityGroup is the security group for agent communication.
	SecurityGroup awsec2.ISecurityGroup

	// ExecutionRole is the IAM role used by agents.
	ExecutionRole awsiam.IRole

	// Secret is the Secrets Manager secret containing API keys.
	Secret awssecretsmanager.ISecret

	// LogGroup is the CloudWatch log group for agent logs.
	LogGroup awslogs.ILogGroup

	// Agents contains the created agent constructs.
	Agents map[string]*AgentConstruct

	// Runtimes contains the AgentCore runtime resources.
	Runtimes map[string]awsbedrockagentcore.CfnRuntime

	// Endpoints contains the AgentCore runtime endpoint resources.
	Endpoints map[string]awsbedrockagentcore.CfnRuntimeEndpoint

	// Gateway is the multi-agent routing gateway (if enabled).
	Gateway awsbedrockagentcore.CfnGateway
}

// AgentConstruct represents a single AgentCore agent.
type AgentConstruct struct {
	constructs.Construct

	// Name is the agent name.
	Name string

	// Config is the agent configuration.
	Config AgentConfig

	// ARN is the agent ARN (available after deployment).
	ARN *string

	// InvokeURL is the agent invocation URL.
	InvokeURL *string
}

// NewAgentCoreStack creates a new AgentCore CDK stack.
func NewAgentCoreStack(scope constructs.Construct, id string, config StackConfig) *AgentCoreStack {
	// Validate and apply defaults
	config.ApplyDefaults()
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("invalid stack configuration: %v", err))
	}

	// Create the stack
	stack := awscdk.NewStack(scope, jsii.String(id), &awscdk.StackProps{
		StackName:   jsii.String(config.StackName),
		Description: jsii.String(config.Description),
		Tags:        convertTags(config.Tags),
	})

	s := &AgentCoreStack{
		Stack:     stack,
		Config:    config,
		Agents:    make(map[string]*AgentConstruct),
		Runtimes:  make(map[string]awsbedrockagentcore.CfnRuntime),
		Endpoints: make(map[string]awsbedrockagentcore.CfnRuntimeEndpoint),
	}

	// Create infrastructure
	s.createVPC()
	s.createSecurityGroup()
	s.createSecrets()
	s.createIAMRole()
	s.createLogGroup()

	// Create agents
	for _, agentConfig := range config.Agents {
		s.createAgent(agentConfig)
	}

	// Create gateway if enabled
	s.createGateway()

	// Add outputs
	s.addOutputs()

	return s
}

// createVPC creates or imports the VPC.
func (s *AgentCoreStack) createVPC() {
	vpcConfig := s.Config.VPC

	if vpcConfig.VPCID != "" {
		// Import existing VPC
		s.VPC = awsec2.Vpc_FromLookup(s.Stack, jsii.String("VPC"), &awsec2.VpcLookupOptions{
			VpcId: jsii.String(vpcConfig.VPCID),
		})
	} else if vpcConfig.CreateVPC {
		// Create new VPC
		s.VPC = awsec2.NewVpc(s.Stack, jsii.String("VPC"), &awsec2.VpcProps{
			VpcName:            jsii.String(fmt.Sprintf("%s-vpc", s.Config.StackName)),
			IpAddresses:        awsec2.IpAddresses_Cidr(jsii.String(vpcConfig.VPCCidr)),
			MaxAzs:             jsii.Number(float64(vpcConfig.MaxAZs)),
			NatGateways:        jsii.Number(1),
			EnableDnsHostnames: jsii.Bool(true),
			EnableDnsSupport:   jsii.Bool(true),
			SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
				{
					Name:       jsii.String("Public"),
					SubnetType: awsec2.SubnetType_PUBLIC,
					CidrMask:   jsii.Number(24),
				},
				{
					Name:       jsii.String("Private"),
					SubnetType: awsec2.SubnetType_PRIVATE_WITH_EGRESS,
					CidrMask:   jsii.Number(24),
				},
			},
		})

		// Add VPC endpoints if enabled
		if vpcConfig.EnableVPCEndpoints {
			s.createVPCEndpoints()
		}
	}
}

// createVPCEndpoints creates VPC endpoints for AWS services.
func (s *AgentCoreStack) createVPCEndpoints() {
	vpc, ok := s.VPC.(awsec2.Vpc)
	if !ok {
		return // Can't add endpoints to imported VPC
	}

	// Bedrock endpoint
	vpc.AddInterfaceEndpoint(jsii.String("BedrockEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_BEDROCK(),
	})

	// Bedrock Runtime endpoint
	vpc.AddInterfaceEndpoint(jsii.String("BedrockRuntimeEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_BEDROCK_RUNTIME(),
	})

	// Secrets Manager endpoint
	vpc.AddInterfaceEndpoint(jsii.String("SecretsManagerEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_SECRETS_MANAGER(),
	})

	// CloudWatch Logs endpoint
	vpc.AddInterfaceEndpoint(jsii.String("LogsEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_CLOUDWATCH_LOGS(),
	})

	// ECR endpoints for pulling container images
	vpc.AddInterfaceEndpoint(jsii.String("EcrApiEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECR(),
	})
	vpc.AddInterfaceEndpoint(jsii.String("EcrDkrEndpoint"), &awsec2.InterfaceVpcEndpointOptions{
		Service: awsec2.InterfaceVpcEndpointAwsService_ECR_DOCKER(),
	})

	// S3 Gateway endpoint (for ECR layers)
	vpc.AddGatewayEndpoint(jsii.String("S3Endpoint"), &awsec2.GatewayVpcEndpointOptions{
		Service: awsec2.GatewayVpcEndpointAwsService_S3(),
	})
}

// createSecurityGroup creates the security group for agent communication.
func (s *AgentCoreStack) createSecurityGroup() {
	if len(s.Config.VPC.SecurityGroupIDs) > 0 {
		// Import existing security group
		s.SecurityGroup = awsec2.SecurityGroup_FromSecurityGroupId(
			s.Stack,
			jsii.String("SecurityGroup"),
			jsii.String(s.Config.VPC.SecurityGroupIDs[0]),
			&awsec2.SecurityGroupImportOptions{},
		)
	} else {
		// Create new security group
		s.SecurityGroup = awsec2.NewSecurityGroup(s.Stack, jsii.String("SecurityGroup"), &awsec2.SecurityGroupProps{
			Vpc:               s.VPC,
			SecurityGroupName: jsii.String(fmt.Sprintf("%s-sg", s.Config.StackName)),
			Description:       jsii.String(fmt.Sprintf("Security group for %s AgentCore agents", s.Config.StackName)),
			AllowAllOutbound:  jsii.Bool(true),
		})

		// Allow intra-agent communication
		s.SecurityGroup.AddIngressRule(
			s.SecurityGroup,
			awsec2.Port_AllTraffic(),
			jsii.String("Allow communication between agents"),
			jsii.Bool(false),
		)
	}
}

// createSecrets creates or imports secrets.
func (s *AgentCoreStack) createSecrets() {
	if s.Config.Secrets == nil {
		return
	}

	secretsConfig := s.Config.Secrets

	if secretsConfig.CreateSecrets && len(secretsConfig.SecretValues) > 0 {
		secretName := secretsConfig.SecretName
		if secretName == "" {
			secretName = fmt.Sprintf("%s-secrets", s.Config.StackName)
		}

		// Convert secret values to JSON structure
		secretJSON := make(map[string]interface{})
		for k, v := range secretsConfig.SecretValues {
			secretJSON[k] = v
		}

		s.Secret = awssecretsmanager.NewSecret(s.Stack, jsii.String("Secrets"), &awssecretsmanager.SecretProps{
			SecretName:        jsii.String(secretName),
			Description:       jsii.String(fmt.Sprintf("Secrets for %s AgentCore agents", s.Config.StackName)),
			SecretObjectValue: &map[string]awscdk.SecretValue{
				// Note: In production, use SecretValue.unsafePlainText only for initial setup
				// Prefer external secret management or CDK context for sensitive values
			},
		})
	}
}

// createIAMRole creates the IAM execution role for agents.
func (s *AgentCoreStack) createIAMRole() {
	iamConfig := s.Config.IAM

	if iamConfig.RoleARN != "" {
		// Import existing role
		s.ExecutionRole = awsiam.Role_FromRoleArn(
			s.Stack,
			jsii.String("ExecutionRole"),
			jsii.String(iamConfig.RoleARN),
			&awsiam.FromRoleArnOptions{},
		)
		return
	}

	// Create new role
	role := awsiam.NewRole(s.Stack, jsii.String("ExecutionRole"), &awsiam.RoleProps{
		RoleName:    jsii.String(fmt.Sprintf("%s-execution-role", s.Config.StackName)),
		Description: jsii.String(fmt.Sprintf("Execution role for %s AgentCore agents", s.Config.StackName)),
		AssumedBy: awsiam.NewCompositePrincipal(
			awsiam.NewServicePrincipal(jsii.String("bedrock.amazonaws.com"), nil),
			awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
		),
	})

	// Add Bedrock access if enabled
	if iamConfig.EnableBedrockAccess {
		if len(iamConfig.BedrockModelIDs) > 0 {
			// Specific model access
			resources := make([]*string, len(iamConfig.BedrockModelIDs))
			for i, modelID := range iamConfig.BedrockModelIDs {
				resources[i] = jsii.String(fmt.Sprintf("arn:aws:bedrock:*:*:foundation-model/%s", modelID))
			}
			role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect:    awsiam.Effect_ALLOW,
				Actions:   jsii.Strings("bedrock:InvokeModel", "bedrock:InvokeModelWithResponseStream"),
				Resources: &resources,
			}))
		} else {
			// All Bedrock models
			role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect:    awsiam.Effect_ALLOW,
				Actions:   jsii.Strings("bedrock:InvokeModel", "bedrock:InvokeModelWithResponseStream"),
				Resources: jsii.Strings("arn:aws:bedrock:*:*:foundation-model/*"),
			}))
		}
	}

	// Add CloudWatch Logs access
	role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: jsii.Strings(
			"logs:CreateLogGroup",
			"logs:CreateLogStream",
			"logs:PutLogEvents",
		),
		Resources: jsii.Strings("arn:aws:logs:*:*:*"),
	}))

	// Add Secrets Manager access if secrets exist
	if s.Secret != nil {
		s.Secret.GrantRead(role, nil)
	}

	// Add access to secrets specified in agent configs
	for _, agent := range s.Config.Agents {
		for _, secretARN := range agent.SecretsARNs {
			secret := awssecretsmanager.Secret_FromSecretCompleteArn(
				s.Stack,
				jsii.String(fmt.Sprintf("Secret-%s-%s", agent.Name, secretARN)),
				jsii.String(secretARN),
			)
			secret.GrantRead(role, nil)
		}
	}

	// Add ECR access for pulling container images
	role.AddToPolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect: awsiam.Effect_ALLOW,
		Actions: jsii.Strings(
			"ecr:GetAuthorizationToken",
			"ecr:BatchCheckLayerAvailability",
			"ecr:GetDownloadUrlForLayer",
			"ecr:BatchGetImage",
		),
		Resources: jsii.Strings("*"),
	}))

	// Add additional policies
	for _, policyARN := range iamConfig.AdditionalPolicies {
		role.AddManagedPolicy(awsiam.ManagedPolicy_FromManagedPolicyArn(
			s.Stack,
			jsii.String(fmt.Sprintf("Policy-%s", policyARN)),
			jsii.String(policyARN),
		))
	}

	// Set permissions boundary if specified
	if iamConfig.PermissionsBoundaryARN != "" {
		awsiam.PermissionsBoundary_Of(role).Apply(
			awsiam.ManagedPolicy_FromManagedPolicyArn(
				s.Stack,
				jsii.String("PermissionsBoundary"),
				jsii.String(iamConfig.PermissionsBoundaryARN),
			),
		)
	}

	s.ExecutionRole = role
}

// createLogGroup creates the CloudWatch log group.
func (s *AgentCoreStack) createLogGroup() {
	if s.Config.Observability == nil || !s.Config.Observability.EnableCloudWatchLogs {
		return
	}

	retentionDays := s.Config.Observability.LogRetentionDays
	if retentionDays == 0 {
		retentionDays = 30
	}

	var retention awslogs.RetentionDays
	switch {
	case retentionDays <= 1:
		retention = awslogs.RetentionDays_ONE_DAY
	case retentionDays <= 7:
		retention = awslogs.RetentionDays_ONE_WEEK
	case retentionDays <= 14:
		retention = awslogs.RetentionDays_TWO_WEEKS
	case retentionDays <= 30:
		retention = awslogs.RetentionDays_ONE_MONTH
	case retentionDays <= 90:
		retention = awslogs.RetentionDays_THREE_MONTHS
	case retentionDays <= 180:
		retention = awslogs.RetentionDays_SIX_MONTHS
	case retentionDays <= 365:
		retention = awslogs.RetentionDays_ONE_YEAR
	default:
		retention = awslogs.RetentionDays_INFINITE
	}

	removalPolicy := awscdk.RemovalPolicy_DESTROY
	if s.Config.RemovalPolicy == "retain" {
		removalPolicy = awscdk.RemovalPolicy_RETAIN
	}

	s.LogGroup = awslogs.NewLogGroup(s.Stack, jsii.String("LogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String(fmt.Sprintf("/aws/agentcore/%s", s.Config.StackName)),
		Retention:     retention,
		RemovalPolicy: removalPolicy,
	})
}

// createAgent creates a single AgentCore agent.
func (s *AgentCoreStack) createAgent(config AgentConfig) {
	agentConstruct := &AgentConstruct{
		Construct: constructs.NewConstruct(s.Stack, jsii.String(fmt.Sprintf("Agent-%s", config.Name))),
		Name:      config.Name,
		Config:    config,
	}

	// Build environment variables
	envVars := make(map[string]string)
	for k, v := range config.Environment {
		envVars[k] = v
	}

	// Add observability environment variables
	if s.Config.Observability != nil {
		envVars["OBSERVABILITY_ENABLED"] = "true"
		envVars["OBSERVABILITY_PROVIDER"] = s.Config.Observability.Provider
		envVars["OBSERVABILITY_PROJECT"] = s.Config.Observability.Project
		if s.Config.Observability.Endpoint != "" {
			envVars["OBSERVABILITY_ENDPOINT"] = s.Config.Observability.Endpoint
		}
	}

	// Add AgentCore-specific environment variables
	envVars["AGENTCORE_AGENT_NAME"] = config.Name
	if config.IsDefault {
		envVars["AGENTCORE_DEFAULT_AGENT"] = config.Name
	}

	// Create AgentCore Runtime
	s.createAgentRuntime(&config, envVars)

	// Create Runtime Endpoint
	s.createRuntimeEndpoint(&config)

	// Add agent-specific outputs
	s.addAgentOutputs(&config)

	s.Agents[config.Name] = agentConstruct
}

// createAgentRuntime creates the AWS::BedrockAgentCore::Runtime resource.
func (s *AgentCoreStack) createAgentRuntime(config *AgentConfig, envVars map[string]string) {
	// Convert env vars to CDK format
	cfnEnvVars := make(map[string]*string)
	for k, v := range envVars {
		cfnEnvVars[k] = jsii.String(v)
	}

	// Build network configuration
	networkConfig := &awsbedrockagentcore.CfnRuntime_NetworkConfigurationProperty{
		NetworkMode: jsii.String("VPC"),
		NetworkModeConfig: &awsbedrockagentcore.CfnRuntime_VpcConfigProperty{
			SecurityGroups: s.getSecurityGroupIds(),
			Subnets:        s.getPrivateSubnetIds(),
		},
	}

	// Build runtime props
	runtimeProps := &awsbedrockagentcore.CfnRuntimeProps{
		AgentRuntimeName: jsii.String(config.Name),
		RoleArn:          s.ExecutionRole.RoleArn(),
		Description:      jsii.String(config.Description),

		AgentRuntimeArtifact: &awsbedrockagentcore.CfnRuntime_AgentRuntimeArtifactProperty{
			ContainerConfiguration: &awsbedrockagentcore.CfnRuntime_ContainerConfigurationProperty{
				ContainerUri: jsii.String(config.ContainerImage),
			},
		},

		NetworkConfiguration:  networkConfig,
		EnvironmentVariables:  &cfnEnvVars,
		ProtocolConfiguration: jsii.String(s.getProtocol(config)),
		Tags:                  s.getTags(config),
	}

	// Add lifecycle configuration if timeout or memory specified
	if config.TimeoutSeconds > 0 || config.MemoryMB > 0 {
		runtimeProps.LifecycleConfiguration = &awsbedrockagentcore.CfnRuntime_LifecycleConfigurationProperty{}
		if config.TimeoutSeconds > 0 {
			runtimeProps.LifecycleConfiguration.(*awsbedrockagentcore.CfnRuntime_LifecycleConfigurationProperty).MaxLifetime = jsii.Number(float64(config.TimeoutSeconds))
		}
	}

	// Create the runtime
	runtime := awsbedrockagentcore.NewCfnRuntime(s.Stack,
		jsii.String(fmt.Sprintf("Runtime-%s", config.Name)),
		runtimeProps,
	)

	s.Runtimes[config.Name] = runtime
}

// createRuntimeEndpoint creates the AWS::BedrockAgentCore::RuntimeEndpoint resource.
func (s *AgentCoreStack) createRuntimeEndpoint(config *AgentConfig) {
	runtime := s.Runtimes[config.Name]

	endpoint := awsbedrockagentcore.NewCfnRuntimeEndpoint(s.Stack,
		jsii.String(fmt.Sprintf("Endpoint-%s", config.Name)),
		&awsbedrockagentcore.CfnRuntimeEndpointProps{
			Name:           jsii.String(fmt.Sprintf("%s-endpoint", config.Name)),
			AgentRuntimeId: runtime.AttrAgentRuntimeId(),
			Description:    jsii.String(fmt.Sprintf("Endpoint for agent %s", config.Name)),
			Tags:           s.getTags(config),
		},
	)

	s.Endpoints[config.Name] = endpoint
}

// getPrivateSubnetIds returns the private subnet IDs for VPC configuration.
func (s *AgentCoreStack) getPrivateSubnetIds() *[]*string {
	if s.VPC == nil {
		return &[]*string{}
	}
	subnets := s.VPC.PrivateSubnets()
	if subnets == nil {
		return &[]*string{}
	}
	ids := make([]*string, len(*subnets))
	for i, subnet := range *subnets {
		ids[i] = subnet.SubnetId()
	}
	return &ids
}

// getSecurityGroupIds returns the security group IDs for VPC configuration.
func (s *AgentCoreStack) getSecurityGroupIds() *[]*string {
	if s.SecurityGroup == nil {
		return &[]*string{}
	}
	return &[]*string{s.SecurityGroup.SecurityGroupId()}
}

// getProtocol returns the protocol for the agent runtime.
func (s *AgentCoreStack) getProtocol(config *AgentConfig) string {
	if config.Protocol != "" {
		return config.Protocol
	}
	return "HTTP" // Default protocol
}

// getTags returns the tags for an agent resource.
func (s *AgentCoreStack) getTags(config *AgentConfig) *map[string]*string {
	tags := make(map[string]*string)
	for k, v := range s.Config.Tags {
		tags[k] = jsii.String(v)
	}
	tags["Agent"] = jsii.String(config.Name)
	return &tags
}

// addAgentOutputs adds CloudFormation outputs for an agent.
func (s *AgentCoreStack) addAgentOutputs(config *AgentConfig) {
	runtime := s.Runtimes[config.Name]
	endpoint := s.Endpoints[config.Name]

	awscdk.NewCfnOutput(s.Stack,
		jsii.String(fmt.Sprintf("Agent-%s-RuntimeArn", config.Name)),
		&awscdk.CfnOutputProps{
			Value:       runtime.AttrAgentRuntimeArn(),
			Description: jsii.String(fmt.Sprintf("Runtime ARN for agent %s", config.Name)),
		})

	awscdk.NewCfnOutput(s.Stack,
		jsii.String(fmt.Sprintf("Agent-%s-RuntimeId", config.Name)),
		&awscdk.CfnOutputProps{
			Value:       runtime.AttrAgentRuntimeId(),
			Description: jsii.String(fmt.Sprintf("Runtime ID for agent %s", config.Name)),
		})

	awscdk.NewCfnOutput(s.Stack,
		jsii.String(fmt.Sprintf("Agent-%s-EndpointArn", config.Name)),
		&awscdk.CfnOutputProps{
			Value:       endpoint.AttrAgentRuntimeEndpointArn(),
			Description: jsii.String(fmt.Sprintf("Endpoint ARN for agent %s", config.Name)),
		})

	awscdk.NewCfnOutput(s.Stack,
		jsii.String(fmt.Sprintf("Agent-%s-Image", config.Name)),
		&awscdk.CfnOutputProps{
			Value:       jsii.String(config.ContainerImage),
			Description: jsii.String(fmt.Sprintf("Container image for agent %s", config.Name)),
		})
}

// createGateway creates the AWS::BedrockAgentCore::Gateway resource if enabled.
func (s *AgentCoreStack) createGateway() {
	if s.Config.Gateway == nil || !s.Config.Gateway.Enabled {
		return
	}

	// Determine protocol type from first agent or default to MCP
	protocolType := "MCP"
	if len(s.Config.Agents) > 0 && s.Config.Agents[0].Protocol != "" {
		protocolType = s.Config.Agents[0].Protocol
	}

	// Default authorizer type to NONE
	authorizerType := "NONE"

	gateway := awsbedrockagentcore.NewCfnGateway(s.Stack,
		jsii.String("Gateway"),
		&awsbedrockagentcore.CfnGatewayProps{
			Name:           jsii.String(s.Config.Gateway.Name),
			Description:    jsii.String(s.Config.Gateway.Description),
			AuthorizerType: jsii.String(authorizerType),
			ProtocolType:   jsii.String(protocolType),
			RoleArn:        s.ExecutionRole.RoleArn(),
			Tags:           s.getStackTags(),
		},
	)

	s.Gateway = gateway
}

// getStackTags returns tags for stack-level resources.
func (s *AgentCoreStack) getStackTags() *map[string]*string {
	tags := make(map[string]*string)
	for k, v := range s.Config.Tags {
		tags[k] = jsii.String(v)
	}
	return &tags
}

// addOutputs adds CloudFormation outputs.
func (s *AgentCoreStack) addOutputs() {
	if s.VPC != nil {
		awscdk.NewCfnOutput(s.Stack, jsii.String("VPCID"), &awscdk.CfnOutputProps{
			Value:       s.VPC.VpcId(),
			Description: jsii.String("VPC ID"),
		})
	}

	if s.SecurityGroup != nil {
		awscdk.NewCfnOutput(s.Stack, jsii.String("SecurityGroupID"), &awscdk.CfnOutputProps{
			Value:       s.SecurityGroup.SecurityGroupId(),
			Description: jsii.String("Security Group ID"),
		})
	}

	if s.ExecutionRole != nil {
		awscdk.NewCfnOutput(s.Stack, jsii.String("ExecutionRoleARN"), &awscdk.CfnOutputProps{
			Value:       s.ExecutionRole.RoleArn(),
			Description: jsii.String("IAM Execution Role ARN"),
		})
	}

	if s.LogGroup != nil {
		awscdk.NewCfnOutput(s.Stack, jsii.String("LogGroupName"), &awscdk.CfnOutputProps{
			Value:       s.LogGroup.LogGroupName(),
			Description: jsii.String("CloudWatch Log Group Name"),
		})
	}

	// Output agent count
	awscdk.NewCfnOutput(s.Stack, jsii.String("AgentCount"), &awscdk.CfnOutputProps{
		Value:       jsii.String(fmt.Sprintf("%d", len(s.Agents))),
		Description: jsii.String("Number of deployed agents"),
	})

	// Gateway outputs
	if s.Gateway != nil {
		awscdk.NewCfnOutput(s.Stack, jsii.String("GatewayArn"), &awscdk.CfnOutputProps{
			Value:       s.Gateway.AttrGatewayArn(),
			Description: jsii.String("Gateway ARN"),
		})

		awscdk.NewCfnOutput(s.Stack, jsii.String("GatewayId"), &awscdk.CfnOutputProps{
			Value:       s.Gateway.AttrGatewayIdentifier(),
			Description: jsii.String("Gateway ID"),
		})

		awscdk.NewCfnOutput(s.Stack, jsii.String("GatewayUrl"), &awscdk.CfnOutputProps{
			Value:       s.Gateway.AttrGatewayUrl(),
			Description: jsii.String("Gateway URL for invocation"),
		})
	}
}

// convertTags converts a map to CDK tags.
func convertTags(tags map[string]string) *map[string]*string {
	if tags == nil {
		return nil
	}
	result := make(map[string]*string)
	for k, v := range tags {
		result[k] = jsii.String(v)
	}
	return &result
}
