# Update Plan: AgentCore Resource Creation

**Created:** January 4, 2026
**Updated:** January 4, 2026
**Status:** In Progress
**Target Version:** v0.2.0

## Overview

Update agentkit-aws-cdk to use the now-available CloudFormation L1 resources for fully automated AWS Bedrock AgentCore deployment.

### Background

- **September 2025**: AWS added CloudFormation, VPC, and PrivateLink support for AgentCore
- **October 2025**: AWS Bedrock AgentCore went GA
- **December 2025**: AWS added A2A (Agent-to-Agent) protocol support
- **Current State**: Core Runtime and Endpoint creation implemented; Gateway support added

---

## AgentCore Communication Architecture

Understanding the correct use of each AgentCore component:

### Communication Patterns

| Component | Purpose | Use Case |
|-----------|---------|----------|
| **Runtime + Endpoint** | Host individual agents | Each agent runs in a Runtime with an Endpoint for invocation |
| **A2A Protocol** | Agent-to-agent communication | Agents communicate directly via A2A endpoints |
| **Gateway** | Expose external tools to agents | APIs, Lambda functions, MCP servers |
| **GatewayTarget** | Register tools with Gateway | NOT for agent-to-agent communication |

### Key Insight: A2A vs Gateway

**Gateway + GatewayTarget** is for exposing **tools** (external APIs, Lambda functions, MCP servers) to agents - it does NOT provide agent-to-agent communication.

**A2A Protocol** is for **agent-to-agent communication**. Agents expose A2A endpoints and communicate directly using the standardized A2A protocol (JSON-RPC 2.0 over HTTP/S).

```
┌─────────────────────────────────────────────────────────────────┐
│                    Multi-Agent Architecture                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌──────────────┐     A2A      ┌──────────────┐                │
│   │  Research    │◄────────────►│ Orchestrator │                │
│   │   Agent      │              │    Agent     │                │
│   │  (Runtime)   │              │  (Runtime)   │                │
│   └──────────────┘              └──────┬───────┘                │
│          ▲                             │                        │
│          │ A2A                    A2A  │                        │
│          │                             ▼                        │
│   ┌──────────────┐              ┌──────────────┐                │
│   │  Synthesis   │◄────────────►│ Verification │                │
│   │    Agent     │     A2A      │    Agent     │                │
│   │  (Runtime)   │              │  (Runtime)   │                │
│   └──────────────┘              └──────────────┘                │
│                                                                  │
│   ┌─────────────────────────────────────────────┐               │
│   │              Gateway (Optional)              │               │
│   │  ┌─────────┐  ┌─────────┐  ┌─────────┐     │               │
│   │  │ Serper  │  │ Weather │  │ Custom  │     │               │
│   │  │   API   │  │   API   │  │ Lambda  │     │               │
│   │  └─────────┘  └─────────┘  └─────────┘     │               │
│   │           (External Tools via MCP)          │               │
│   └─────────────────────────────────────────────┘               │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Implications for stats-agent-team

The stats-agent-team agents already implement A2A protocol (`a2a.go` files). This is the **correct approach**:

1. Each agent gets a Runtime + RuntimeEndpoint
2. Agents communicate via A2A protocol (already implemented)
3. Gateway is optional - useful for exposing external tools (e.g., Serper API)
4. GatewayTarget is NOT needed for agent-to-agent routing

---

## Implementation Status

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 1 | Core Runtime + Endpoint creation | ✅ Complete |
| Phase 2 | Config schema updates | ✅ Complete |
| Phase 3 | Gateway support | ✅ Complete (basic) |
| Phase 3b | GatewayTarget | ⏸️ Deferred (not needed for A2A) |
| Phase 4 | Memory support | 🔲 Pending |

---

## Previous State (Historical)

**Location**: Lines 442-448 of `agentcore/stack.go` (now replaced)

## Current State

**Location**: Lines 442-448 of `agentcore/stack.go`

```go
// Note: The actual Bedrock AgentCore resource creation would go here.
// As of the current AWS CDK version, AgentCore may require L1 constructs
// (CfnAgent) or custom resources. The exact implementation depends on
// the AWS Bedrock AgentCore API availability in CDK.
//
// For now, we output the configuration that would be used.
// When AgentCore L2 constructs become available, this will be updated.
```

**Infrastructure Already Implemented:**

- VPC with private subnets
- Security groups
- IAM execution role with Bedrock permissions
- Secrets Manager integration
- CloudWatch logging
- Observability setup (Opik/Langfuse/Phoenix)

## Target CloudFormation Resources

| Resource | Purpose | Priority | Status |
|----------|---------|----------|--------|
| `AWS::BedrockAgentCore::Runtime` | Agent execution environment | P0 | ✅ Done |
| `AWS::BedrockAgentCore::RuntimeEndpoint` | Agent invocation endpoint | P0 | ✅ Done |
| `AWS::BedrockAgentCore::Gateway` | Expose external tools via MCP | P1 | ✅ Done |
| `AWS::BedrockAgentCore::GatewayTarget` | Register tools with Gateway | P1 | ⏸️ Deferred |
| `AWS::BedrockAgentCore::Memory` | Agent memory/state | P2 | 🔲 Pending |
| `AWS::BedrockAgentCore::WorkloadIdentity` | Workload IAM | P2 | 🔲 Pending |

**Note:** GatewayTarget is for registering external tools (APIs, Lambda, MCP servers) with the Gateway. It is NOT used for agent-to-agent communication - agents communicate directly via the A2A protocol.

## CloudFormation Resource Schemas

### AWS::BedrockAgentCore::Runtime

**Required Properties:**

| Property | Type | Description |
|----------|------|-------------|
| `AgentRuntimeName` | String | Name of the runtime (pattern: `[a-zA-Z][a-zA-Z0-9_]{0,47}`) |
| `AgentRuntimeArtifact` | Object | Container or code configuration |
| `NetworkConfiguration` | Object | VPC, subnets, security groups |
| `RoleArn` | String | IAM execution role ARN |

**Optional Properties:**

| Property | Type | Description |
|----------|------|-------------|
| `Description` | String | Runtime description (max 1200 chars) |
| `EnvironmentVariables` | Map | Environment variables |
| `ProtocolConfiguration` | String | `MCP`, `HTTP`, or `A2A` |
| `AuthorizerConfiguration` | Object | Inbound authorization config |
| `LifecycleConfiguration` | Object | Timeout and memory settings |
| `Tags` | Map | Resource tags |

**Return Values (Fn::GetAtt):**

- `AgentRuntimeArn`
- `AgentRuntimeId`
- `AgentRuntimeVersion`
- `Status`
- `CreatedAt`
- `LastUpdatedAt`

### AWS::BedrockAgentCore::RuntimeEndpoint

**Required Properties:**

| Property | Type | Description |
|----------|------|-------------|
| `Name` | String | Endpoint name (max 48 chars) |
| `AgentRuntimeId` | String | ID of the parent runtime |

**Optional Properties:**

| Property | Type | Description |
|----------|------|-------------|
| `AgentRuntimeVersion` | String | Runtime version to use |
| `Description` | String | Endpoint description |
| `Tags` | Map | Resource tags |

**Return Values (Fn::GetAtt):**

- `AgentRuntimeEndpointArn`
- `Id`
- `Status`
- `LiveVersion`

---

## Implementation Tasks

### Phase 1: Core Runtime Creation (P0)

#### 1.1 Add CDK L1 Construct Imports

```go
// agentcore/stack.go
import (
    "github.com/aws/aws-cdk-go/awscdk/v2/awsbedrockagentcore"
)
```

#### 1.2 Update Stack Struct

```go
type AgentCoreStack struct {
    Stack         awscdk.Stack
    Config        *iac.StackConfig
    VPC           awsec2.IVpc
    SecurityGroup awsec2.ISecurityGroup
    ExecutionRole awsiam.IRole
    Secret        awssecretsmanager.ISecret
    LogGroup      awslogs.ILogGroup
    Agents        map[string]constructs.Construct
    Runtimes      map[string]awsbedrockagentcore.CfnRuntime         // NEW
    Endpoints     map[string]awsbedrockagentcore.CfnRuntimeEndpoint // NEW
}
```

#### 1.3 Create Runtime Resource

Replace placeholder (lines 442-458) with:

```go
func (s *AgentCoreStack) createAgentRuntime(config *iac.AgentConfig, envVars map[string]string) {
    // Convert env vars to CloudFormation format
    cfnEnvVars := make(map[string]*string)
    for k, v := range envVars {
        cfnEnvVars[k] = jsii.String(v)
    }

    // Create the AgentCore Runtime
    runtime := awsbedrockagentcore.NewCfnRuntime(s.Stack,
        jsii.String(fmt.Sprintf("Runtime-%s", config.Name)),
        &awsbedrockagentcore.CfnRuntimeProps{
            AgentRuntimeName: jsii.String(config.Name),
            RoleArn:          s.ExecutionRole.RoleArn(),
            Description:      jsii.String(config.Description),

            AgentRuntimeArtifact: &awsbedrockagentcore.CfnRuntime_AgentRuntimeArtifactProperty{
                ContainerConfiguration: &awsbedrockagentcore.CfnRuntime_ContainerConfigurationProperty{
                    ContainerUri: jsii.String(config.ContainerImage),
                },
            },

            NetworkConfiguration: &awsbedrockagentcore.CfnRuntime_NetworkConfigurationProperty{
                SecurityGroupIds: &[]*string{s.SecurityGroup.SecurityGroupId()},
                SubnetIds:        s.getPrivateSubnetIds(),
            },

            EnvironmentVariables: cfnEnvVars,
            ProtocolConfiguration: jsii.String(s.getProtocol(config)),

            LifecycleConfiguration: &awsbedrockagentcore.CfnRuntime_LifecycleConfigurationProperty{
                TimeoutSeconds: jsii.Number(float64(config.TimeoutSeconds)),
                MemoryMb:       jsii.Number(float64(config.MemoryMB)),
            },

            Tags: s.getTags(config),
        },
    )

    s.Runtimes[config.Name] = runtime
}
```

#### 1.4 Create Runtime Endpoint

```go
func (s *AgentCoreStack) createRuntimeEndpoint(config *iac.AgentConfig) {
    runtime := s.Runtimes[config.Name]

    endpoint := awsbedrockagentcore.NewCfnRuntimeEndpoint(s.Stack,
        jsii.String(fmt.Sprintf("Endpoint-%s", config.Name)),
        &awsbedrockagentcore.CfnRuntimeEndpointProps{
            Name:           jsii.String(fmt.Sprintf("%s-endpoint", config.Name)),
            AgentRuntimeId: runtime.AttrAgentRuntimeId(),
            Description:    jsii.String(fmt.Sprintf("Endpoint for %s", config.Name)),
            Tags:           s.getTags(config),
        },
    )

    s.Endpoints[config.Name] = endpoint
}
```

#### 1.5 Add Helper Methods

```go
func (s *AgentCoreStack) getPrivateSubnetIds() *[]*string {
    subnets := s.VPC.PrivateSubnets()
    ids := make([]*string, len(*subnets))
    for i, subnet := range *subnets {
        ids[i] = subnet.SubnetId()
    }
    return &ids
}

func (s *AgentCoreStack) getProtocol(config *iac.AgentConfig) string {
    if config.Protocol != "" {
        return config.Protocol
    }
    return "HTTP" // Default
}

func (s *AgentCoreStack) getTags(config *iac.AgentConfig) map[string]*string {
    tags := make(map[string]*string)
    for k, v := range s.Config.Tags {
        tags[k] = jsii.String(v)
    }
    tags["Agent"] = jsii.String(config.Name)
    return tags
}
```

#### 1.6 Add Stack Outputs

```go
func (s *AgentCoreStack) addAgentOutputs(config *iac.AgentConfig) {
    runtime := s.Runtimes[config.Name]
    endpoint := s.Endpoints[config.Name]

    awscdk.NewCfnOutput(s.Stack,
        jsii.String(fmt.Sprintf("Agent-%s-RuntimeArn", config.Name)),
        &awscdk.CfnOutputProps{
            Value:       runtime.AttrAgentRuntimeArn(),
            Description: jsii.String(fmt.Sprintf("Runtime ARN for %s", config.Name)),
        })

    awscdk.NewCfnOutput(s.Stack,
        jsii.String(fmt.Sprintf("Agent-%s-RuntimeId", config.Name)),
        &awscdk.CfnOutputProps{
            Value:       runtime.AttrAgentRuntimeId(),
            Description: jsii.String(fmt.Sprintf("Runtime ID for %s", config.Name)),
        })

    awscdk.NewCfnOutput(s.Stack,
        jsii.String(fmt.Sprintf("Agent-%s-EndpointArn", config.Name)),
        &awscdk.CfnOutputProps{
            Value:       endpoint.AttrAgentRuntimeEndpointArn(),
            Description: jsii.String(fmt.Sprintf("Endpoint ARN for %s", config.Name)),
        })
}
```

---

### Phase 2: Configuration Schema Updates

#### 2.1 Update AgentConfig (in agentkit)

**File:** `agentkit/platforms/agentcore/iac/config.go`

```go
type AgentConfig struct {
    Name           string            `json:"name" yaml:"name"`
    ContainerImage string            `json:"containerImage" yaml:"containerImage"`
    MemoryMB       int               `json:"memoryMB,omitempty" yaml:"memoryMB,omitempty"`
    TimeoutSeconds int               `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
    Description    string            `json:"description,omitempty" yaml:"description,omitempty"`
    Environment    map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
    SecretsARNs    []string          `json:"secretsARNs,omitempty" yaml:"secretsARNs,omitempty"`
    IsDefault      bool              `json:"isDefault,omitempty" yaml:"isDefault,omitempty"`

    // NEW: AgentCore-specific settings
    Protocol       string            `json:"protocol,omitempty" yaml:"protocol,omitempty"`       // MCP, HTTP, A2A
    Authorizer     *AuthorizerConfig `json:"authorizer,omitempty" yaml:"authorizer,omitempty"`
    EnableMemory   bool              `json:"enableMemory,omitempty" yaml:"enableMemory,omitempty"`
}

type AuthorizerConfig struct {
    Type      string `json:"type" yaml:"type"`                         // IAM, LAMBDA, NONE
    LambdaArn string `json:"lambdaArn,omitempty" yaml:"lambdaArn,omitempty"`
}
```

#### 2.2 Update StackConfig

```go
type StackConfig struct {
    // ... existing fields ...

    // NEW: Gateway configuration for multi-agent communication
    Gateway *GatewayConfig `json:"gateway,omitempty" yaml:"gateway,omitempty"`
}

type GatewayConfig struct {
    Enabled     bool     `json:"enabled,omitempty" yaml:"enabled,omitempty"`
    Name        string   `json:"name,omitempty" yaml:"name,omitempty"`
    Description string   `json:"description,omitempty" yaml:"description,omitempty"`
    Targets     []string `json:"targets,omitempty" yaml:"targets,omitempty"` // Agent names
}
```

---

### Phase 3: Gateway for External Tools (P1) ✅ Complete

**Important Clarification:** Gateway is for exposing **external tools** to agents via MCP, NOT for agent-to-agent communication. Agents communicate directly via A2A protocol.

#### 3.1 Create Gateway Resource (Implemented)

```go
func (s *AgentCoreStack) createGateway() {
    if s.Config.Gateway == nil || !s.Config.Gateway.Enabled {
        return
    }

    // Determine protocol type from first agent or default to MCP
    protocolType := "MCP"
    if len(s.Config.Agents) > 0 && s.Config.Agents[0].Protocol != "" {
        protocolType = s.Config.Agents[0].Protocol
    }

    gateway := awsbedrockagentcore.NewCfnGateway(s.Stack,
        jsii.String("Gateway"),
        &awsbedrockagentcore.CfnGatewayProps{
            Name:           jsii.String(s.Config.Gateway.Name),
            Description:    jsii.String(s.Config.Gateway.Description),
            AuthorizerType: jsii.String("NONE"),
            ProtocolType:   jsii.String(protocolType),
            RoleArn:        s.ExecutionRole.RoleArn(),
            Tags:           s.getStackTags(),
        },
    )

    s.Gateway = gateway
}
```

#### 3.2 GatewayTarget (Deferred)

GatewayTarget is for registering **external tools** with the Gateway, not for routing to agents. The API requires:

- `CredentialProviderConfigurations` - How Gateway authenticates to the tool
- `TargetConfiguration` - MCP server endpoint, Lambda ARN, or OpenAPI schema

This is useful for integrating external services (e.g., Serper API, custom Lambda tools) but is NOT needed for agent-to-agent communication.

```go
// Example: Registering an external MCP server as a tool
awsbedrockagentcore.NewCfnGatewayTarget(s.Stack,
    jsii.String("SerperTool"),
    &awsbedrockagentcore.CfnGatewayTargetProps{
        Name:              jsii.String("serper-search"),
        GatewayIdentifier: gateway.AttrGatewayIdentifier(),
        CredentialProviderConfigurations: []interface{}{
            &CredentialProviderConfigurationProperty{
                CredentialProviderType: jsii.String("API_KEY"),
                CredentialProvider: &CredentialProviderProperty{
                    ApiKeyCredentialProvider: &ApiKeyCredentialProviderProperty{
                        ProviderArn: jsii.String("arn:aws:secretsmanager:..."),
                    },
                },
            },
        },
        TargetConfiguration: &TargetConfigurationProperty{
            Mcp: &McpTargetConfigurationProperty{
                McpServer: &McpServerTargetConfigurationProperty{
                    Endpoint: jsii.String("https://mcp.serper.dev"),
                },
            },
        },
    },
)
```

**Status:** Deferred until a concrete use case for external tool integration is identified.

---

### Phase 4: Memory Support (P2)

```go
func (s *AgentCoreStack) createAgentMemory(config *iac.AgentConfig) {
    if !config.EnableMemory {
        return
    }

    memory := awsbedrockagentcore.NewCfnMemory(s.Stack,
        jsii.String(fmt.Sprintf("Memory-%s", config.Name)),
        &awsbedrockagentcore.CfnMemoryProps{
            Name:        jsii.String(fmt.Sprintf("%s-memory", config.Name)),
            Description: jsii.String(fmt.Sprintf("Memory for agent %s", config.Name)),
            Tags:        s.getTags(config),
        },
    )

    // Pass memory ID to agent via environment variable
    s.agentEnvVars[config.Name]["AGENTCORE_MEMORY_ID"] = *memory.AttrMemoryId()
}
```

---

## File Changes Summary

| File | Changes |
|------|---------|
| `agentcore/stack.go` | Add runtime/endpoint creation, update struct, add helpers |
| `agentcore/runtime.go` | NEW: Runtime-specific helpers and validation |
| `agentcore/endpoint.go` | NEW: Endpoint-specific helpers |
| `agentcore/gateway.go` | NEW: Gateway creation (Phase 3) |
| `agentcore/memory.go` | NEW: Memory support (Phase 4) |
| `go.mod` | Verify CDK version includes BedrockAgentCore L1 constructs |
| `examples/1-cdk-go/main.go` | Update with new outputs and options |
| `examples/2-cdk-json/config.yaml` | Add protocol and gateway examples |
| `README.md` | Document new features and outputs |

---

## Example Configuration (stats-agent-team)

```yaml
stackName: stats-agent-team
description: Statistics research and verification multi-agent system

agents:
  # Agents communicate via A2A protocol (implemented in a2a.go)
  - name: research
    containerImage: ghcr.io/agentplexus/stats-agent-research:v0.5.1
    memoryMB: 512
    timeoutSeconds: 30
    protocol: A2A  # Agent-to-agent protocol
    description: Research agent - web search via Serper

  - name: synthesis
    containerImage: ghcr.io/agentplexus/stats-agent-synthesis:v0.5.1
    memoryMB: 1024
    timeoutSeconds: 120
    protocol: A2A
    description: Synthesis agent - extract statistics from URLs

  - name: verification
    containerImage: ghcr.io/agentplexus/stats-agent-verification:v0.5.1
    memoryMB: 512
    timeoutSeconds: 60
    protocol: A2A
    description: Verification agent - validate sources

  - name: orchestration-eino
    containerImage: ghcr.io/agentplexus/stats-agent-orchestration-eino:v0.5.1
    memoryMB: 512
    timeoutSeconds: 300
    protocol: A2A
    isDefault: true
    description: Orchestration agent - coordinate workflow

# Gateway is OPTIONAL - only needed if exposing external tools via MCP
# Agents communicate directly via A2A, NOT through the Gateway
# gateway:
#   enabled: true
#   name: stats-tools-gateway
#   description: Gateway for external tool access (Serper, etc.)

vpc:
  createVPC: true
  vpcCidr: 10.0.0.0/16
  maxAZs: 2
  enableVPCEndpoints: true

observability:
  provider: opik
  project: stats-agent-team
  enableCloudWatchLogs: true
  logRetentionDays: 30

secrets:
  createSecrets: true
  secretName: stats-agent-team-secrets
  secretValues:
    SERPER_API_KEY: ${SERPER_API_KEY}
    OPIK_API_KEY: ${OPIK_API_KEY}

iam:
  enableBedrockAccess: true

tags:
  Project: stats-agent-team
  Environment: production
  ManagedBy: agentkit-cdk
```

---

## Expected Stack Outputs

After deployment, the stack will output:

```
Outputs:
  VPCID = vpc-0123456789abcdef0
  SecurityGroupID = sg-0123456789abcdef0
  ExecutionRoleARN = arn:aws:iam::123456789012:role/stats-agent-team-execution-role

  # Each agent gets Runtime + Endpoint outputs
  Agent-research-RuntimeArn = arn:aws:bedrock:us-east-1:123456789012:agent-runtime/...
  Agent-research-RuntimeId = ...
  Agent-research-EndpointArn = arn:aws:bedrock:us-east-1:123456789012:agent-runtime-endpoint/...

  Agent-synthesis-RuntimeArn = ...
  Agent-synthesis-RuntimeId = ...
  Agent-synthesis-EndpointArn = ...

  Agent-verification-RuntimeArn = ...
  Agent-verification-RuntimeId = ...
  Agent-verification-EndpointArn = ...

  Agent-orchestration-eino-RuntimeArn = ...
  Agent-orchestration-eino-RuntimeId = ...
  Agent-orchestration-eino-EndpointArn = ...

  # Gateway outputs (only if gateway.enabled = true)
  GatewayArn = arn:aws:bedrock:us-east-1:123456789012:gateway/...
  GatewayId = ...
  GatewayUrl = https://...bedrock-agentcore.us-east-1.amazonaws.com/...
```

**Note:** Agents use the RuntimeEndpoint ARNs to discover and communicate with each other via A2A protocol. The Gateway URL is only used if you're exposing external tools.

---

## Testing Plan

### Unit Tests

- Mock CDK constructs
- Verify resource creation with correct properties
- Test configuration validation

### Synth Tests

- `cdk synth` produces valid CloudFormation
- Validate resource dependencies
- Check IAM policy statements

### Integration Tests

- Deploy to AWS sandbox account
- Verify runtime status becomes ACTIVE
- Test A2A communication between agents
- Verify endpoint invocation

### stats-agent-team Validation

- Full deployment of 4-agent system
- A2A protocol communication test
- End-to-end workflow test (research → synthesis → verification)
- Observability verification

---

## Progress Summary

| Phase | Scope | Status |
|-------|-------|--------|
| Phase 1 | Core Runtime + Endpoint | ✅ Complete |
| Phase 2 | Config schema updates | ✅ Complete |
| Phase 3 | Gateway support | ✅ Complete |
| Phase 3b | GatewayTarget | ⏸️ Deferred |
| Phase 4 | Memory support | 🔲 Pending |
| Testing | All phases | 🔲 Pending |
| Documentation | README, examples | 🔲 Pending |

---

## Dependencies

### CDK Version Requirements

Verify `go.mod` has a CDK version that includes `awsbedrockagentcore`:

```go
require (
    github.com/aws/aws-cdk-go/awscdk/v2 v2.240.0 // or later
)
```

### agentkit Dependency

Config schema changes require coordinated update to:

```go
require (
    github.com/agentplexus/agentkit v0.3.0 // with updated iac/config.go
)
```

---

## References

- [AWS::BedrockAgentCore::Runtime](https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/aws-resource-bedrockagentcore-runtime.html)
- [AWS::BedrockAgentCore::RuntimeEndpoint](https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/aws-resource-bedrockagentcore-runtimeendpoint.html)
- [AWS BedrockAgentCore CloudFormation Reference](https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/AWS_BedrockAgentCore.html)
- [Amazon Bedrock AgentCore Samples](https://github.com/awslabs/amazon-bedrock-agentcore-samples)
- [AgentCore GA Announcement](https://aws.amazon.com/about-aws/whats-new/2025/10/amazon-bedrock-agentcore-available/)
