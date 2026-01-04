# AgentKit for AWS CDK v0.2.0 Release Notes

**Release Date:** January 4, 2026

This release adds full AWS Bedrock AgentCore resource creation using CloudFormation L1 constructs, replacing the placeholder implementation from v0.1.0.

## Highlights

- **AgentCore Runtime creation** - Full `AWS::BedrockAgentCore::Runtime` resource support
- **Runtime Endpoint creation** - Automatic `AWS::BedrockAgentCore::RuntimeEndpoint` for each agent
- **Gateway support** - Multi-agent routing via `AWS::BedrockAgentCore::Gateway`
- **Protocol support** - HTTP, MCP, and A2A protocol configuration
- **Enhanced outputs** - Runtime ARNs, IDs, Endpoint ARNs, and Gateway URLs

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

Enable multi-agent routing with a Gateway:

```yaml
gateway:
  enabled: true
  name: my-gateway
  description: Multi-agent routing gateway
  targets:
    - research
    - orchestration
```

When enabled, creates:

- `AWS::BedrockAgentCore::Gateway` resource
- Gateway URL output for invocation

Note: GatewayTarget support (linking to RuntimeEndpoints) requires additional configuration and is planned for a future release.

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
    "github.com/agentplexus/agentkit-aws-cdk/agentcore"
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

- Requires `github.com/agentplexus/agentkit v0.3.0` for new config fields
- Uses `awsbedrockagentcore` L1 constructs from AWS CDK

## Roadmap

### Completed in v0.2.0

- [x] AgentCore Runtime creation
- [x] Runtime Endpoint creation
- [x] Protocol configuration
- [x] Gateway creation
- [x] Enhanced outputs

### Planned for v0.3.0

- [ ] Gateway targets (`AWS::BedrockAgentCore::GatewayTarget`)
- [ ] Memory support (`AWS::BedrockAgentCore::Memory`)
- [ ] Authorizer configuration

## Breaking Changes

None. The API is backward compatible, but the underlying implementation now creates actual AWS resources instead of placeholder outputs.

## Migration Guide

No code changes required. After upgrading:

1. Update agentkit dependency to v0.3.0
2. Run `cdk diff` to see the new resources that will be created
3. Run `cdk deploy` to create the AgentCore resources

## Installation

```bash
go get github.com/agentplexus/agentkit-aws-cdk@v0.2.0
```

## License

MIT License
