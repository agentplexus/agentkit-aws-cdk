# AgentKit for AWS CDK v0.2.0 Release Notes

**Release Date:** March 2026

This release adds full AWS Bedrock AgentCore resource creation, CLI deployment tools, and migrates the module from `agentplexus` to `plexusone` organization.

## Highlights

- **Module migration** - Module path changed from `github.com/agentplexus/agentkit-aws-cdk` to `github.com/plexusone/agentkit-aws-cdk`
- **CLI deployment tools** - New `cmd/deploy` and `cmd/push-secrets` for one-command deployments
- **AgentCore Runtime creation** - Full `AWS::BedrockAgentCore::Runtime` resource support
- **Runtime Endpoint creation** - Automatic `AWS::BedrockAgentCore::RuntimeEndpoint` for each agent
- **Gateway support** - External tool integration via `AWS::BedrockAgentCore::Gateway`
- **Protocol support** - HTTP, MCP, and A2A protocol configuration

## New Features

### AgentCore Runtime Resources

Agents are now deployed as actual AgentCore Runtime resources:

```go
// Each agent creates:
// - AWS::BedrockAgentCore::Runtime
// - AWS::BedrockAgentCore::RuntimeEndpoint
```

The runtime is configured with:

- Container image from agent config
- VPC networking (private subnets, security groups)
- Environment variables (including observability settings)
- Protocol configuration (HTTP, MCP, A2A)
- Lifecycle settings (timeout)
- Resource tags

### Runtime Endpoints

Each agent automatically gets a Runtime Endpoint for invocation:

```go
// Endpoint created with:
// - Name: {agent-name}-endpoint
// - AgentRuntimeId: linked to parent runtime
// - Description and tags
```

### Enhanced Stack Outputs

New CloudFormation outputs for each agent:

```
Agent-{name}-RuntimeArn    # Runtime ARN for IAM policies
Agent-{name}-RuntimeId     # Runtime ID for API calls
Agent-{name}-EndpointArn   # Endpoint ARN for invocation
Agent-{name}-Image         # Container image reference
```

### Protocol Configuration

Agents can specify their communication protocol:

```yaml
agents:
  - name: mcp-agent
    containerImage: ghcr.io/example/agent:latest
    protocol: MCP  # HTTP (default), MCP, or A2A
```

### Gateway Support

**Important:** Gateway is for exposing **external tools** to agents via MCP, NOT for agent-to-agent communication. Agents communicate directly via A2A protocol.

```yaml
# Gateway is OPTIONAL - only needed for external tool integration
gateway:
  enabled: true
  name: my-tools-gateway
  description: Gateway for external tool access
```

When enabled, creates:

- `AWS::BedrockAgentCore::Gateway` resource
- Gateway URL output for tool invocation

**Note:** GatewayTarget (for registering external MCP servers, Lambda functions, or APIs) is deferred until a concrete use case is identified. For agent-to-agent communication, use the A2A protocol which agents implement directly.

## API Changes

### New Stack Fields

```go
type AgentCoreStack struct {
    // ... existing fields ...

    // NEW: Maps of created AgentCore resources
    Runtimes  map[string]awsbedrockagentcore.CfnRuntime
    Endpoints map[string]awsbedrockagentcore.CfnRuntimeEndpoint

    // NEW: Multi-agent Gateway (if enabled)
    Gateway awsbedrockagentcore.CfnGateway
}
```

### New Type Aliases

```go
// Re-exported from agentkit for convenience
type AuthorizerConfig = iac.AuthorizerConfig
type GatewayConfig    = iac.GatewayConfig
```

### New Helper Methods

```go
// Get protocol for agent (defaults to HTTP)
(s *AgentCoreStack) getProtocol(config *AgentConfig) string

// Get private subnet IDs for VPC configuration
(s *AgentCoreStack) getPrivateSubnetIds() *[]*string

// Get security group IDs for VPC configuration
(s *AgentCoreStack) getSecurityGroupIds() *[]*string

// Get tags for agent resources
(s *AgentCoreStack) getTags(config *AgentConfig) *map[string]*string

// Get tags for stack-level resources
(s *AgentCoreStack) getStackTags() *map[string]*string

// Add CloudFormation outputs for an agent
(s *AgentCoreStack) addAgentOutputs(config *AgentConfig)

// Create Gateway resource if enabled
(s *AgentCoreStack) createGateway()
```

## Example Deployment

```yaml
# config.yaml
stackName: stats-agent-team
description: Statistics research multi-agent system

agents:
  - name: research
    containerImage: ghcr.io/agentplexus/stats-agent-research:v0.5.1
    memoryMB: 512
    timeoutSeconds: 30
    protocol: HTTP

  - name: orchestration
    containerImage: ghcr.io/agentplexus/stats-agent-orchestration:v0.5.1
    memoryMB: 512
    timeoutSeconds: 300
    protocol: HTTP
    isDefault: true

vpc:
  createVPC: true
  vpcCidr: 10.0.0.0/16
  maxAZs: 2
  enableVPCEndpoints: true

observability:
  provider: opik
  project: stats-agent-team
  enableCloudWatchLogs: true

iam:
  enableBedrockAccess: true
```

```go
// main.go
package main

import (
    "github.com/plexusone/agentkit-aws-cdk/agentcore"
)

func main() {
    app := agentcore.NewApp()
    agentcore.MustNewStackFromFile(app, "config.yaml")
    agentcore.Synth(app)
}
```

Deploy:

```bash
cdk deploy
```

Expected outputs:

```
Outputs:
stats-agent-team.VPCID = vpc-0123456789abcdef0
stats-agent-team.SecurityGroupID = sg-0123456789abcdef0
stats-agent-team.ExecutionRoleARN = arn:aws:iam::123456789012:role/stats-agent-team-execution-role
stats-agent-team.Agent-research-RuntimeArn = arn:aws:bedrock:us-east-1:123456789012:agent-runtime/...
stats-agent-team.Agent-research-RuntimeId = ...
stats-agent-team.Agent-research-EndpointArn = arn:aws:bedrock:us-east-1:123456789012:agent-runtime-endpoint/...
stats-agent-team.Agent-orchestration-RuntimeArn = ...
stats-agent-team.Agent-orchestration-RuntimeId = ...
stats-agent-team.Agent-orchestration-EndpointArn = ...
stats-agent-team.GatewayArn = arn:aws:bedrock:us-east-1:123456789012:gateway/...  # if gateway enabled
stats-agent-team.GatewayId = ...
stats-agent-team.GatewayUrl = https://...bedrock-agentcore.us-east-1.amazonaws.com/...
```

## Dependencies

- Requires `github.com/plexusone/agentkit v0.5.0` for shared IaC configuration
- Uses `awsbedrockagentcore` L1 constructs from AWS CDK
- AWS SDK Go v2 for CLI tools (secretsmanager, sts)

## Roadmap

### Completed in v0.2.0

- [x] AgentCore Runtime creation
- [x] Runtime Endpoint creation
- [x] Protocol configuration (HTTP, MCP, A2A)
- [x] Gateway creation (for external tools)
- [x] Enhanced outputs

### Planned for Future Releases

- [ ] Memory support (`AWS::BedrockAgentCore::Memory`)
- [ ] Authorizer configuration (IAM, Lambda)
- [ ] GatewayTarget - for external tool registration (when use case identified)

### Architecture Notes

- **Agent-to-Agent Communication**: Use A2A protocol directly (agents implement this)
- **External Tool Integration**: Use Gateway + GatewayTarget (optional)
- **Gateway is NOT for agent routing** - it's for exposing external APIs/tools to agents

## Breaking Changes

- **Module path changed** from `github.com/agentplexus/agentkit-aws-cdk` to `github.com/plexusone/agentkit-aws-cdk`
- **Config directory changed** from `~/.agentplexus/` to `~/.plexusone/` for CLI tools

## Migration Guide

1. Update import paths from `github.com/agentplexus/agentkit-aws-cdk` to `github.com/plexusone/agentkit-aws-cdk`
2. Update go.mod dependency to `github.com/plexusone/agentkit-aws-cdk v0.2.0`
3. If using CLI tools, move config from `~/.agentplexus/` to `~/.plexusone/`
4. Run `cdk diff` to see the new resources that will be created
5. Run `cdk deploy` to create the AgentCore resources

## Installation

```bash
go get github.com/plexusone/agentkit-aws-cdk@v0.2.0
```

### CLI Tools

```bash
# One-command deployment
go install github.com/plexusone/agentkit-aws-cdk/cmd/deploy@latest

# Push secrets to AWS Secrets Manager
go install github.com/plexusone/agentkit-aws-cdk/cmd/push-secrets@latest
```

## License

MIT License
