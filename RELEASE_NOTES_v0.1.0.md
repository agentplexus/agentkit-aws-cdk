# agentkit-aws-cdk v0.1.0 Release Notes

**Release Date:** December 31, 2025

Initial release of AWS CDK constructs for deploying agentkit-based agents to AWS Bedrock AgentCore.

## Highlights

- **AWS CDK constructs** for AgentCore deployment
- **Fluent builder API** for type-safe configuration
- **Shared configuration schema** with agentkit core and agentkit-aws-pulumi
- **Four deployment approaches** from full code to pure CloudFormation

## Features

### CDK Constructs

Type-safe Go constructs for AWS Bedrock AgentCore:

```go
import "github.com/agentplexus/agentkit-aws-cdk/agentcore"

app := agentcore.NewApp()

agentcore.NewStackBuilder("my-agents").
    WithAgents(research, orchestration).
    WithOpik("my-project", secretARN).
    WithTags(map[string]string{"Environment": "production"}).
    Build(app)

agentcore.Synth(app)
```

### Fluent Builders

- `AgentBuilder` - Configure individual agents with memory, timeout, environment variables
- `StackBuilder` - Compose agents with VPC, observability, and IAM settings
- `CfnIncludeBuilder` - Import existing CloudFormation templates

### Configuration Loading

Load stack configuration from JSON or YAML files:

```go
app := agentcore.NewApp()
agentcore.MustNewStackFromFile(app, "config.yaml")
agentcore.Synth(app)
```

### CfnInclude Support

Import existing CloudFormation templates into CDK:

```go
agentcore.NewCfnIncludeBuilder("my-agents", "template.yaml").
    WithParameter("Environment", "production").
    Build(app)
```

## Deployment Approaches

| Approach | Description |
|----------|-------------|
| **CDK Go Constructs** | Full type safety, IDE support, complex logic |
| **CDK + JSON/YAML** | Configuration-driven with minimal Go wrapper |
| **CfnInclude** | Import existing CloudFormation templates |
| **Pure CloudFormation** | No CDK runtime (use agentkit core) |

## Configuration

Uses shared configuration schema from `agentkit/platforms/agentcore/iac/`:

```yaml
stackName: my-agents
agents:
  - name: research
    containerImage: ghcr.io/example/research:latest
    memoryMB: 512
    timeoutSeconds: 30
  - name: orchestration
    containerImage: ghcr.io/example/orchestration:latest
    memoryMB: 1024
    isDefault: true
vpc:
  createVPC: true
  vpcCidr: 10.0.0.0/16
observability:
  provider: opik
  project: my-agents
```

## Dependencies

- **21 transitive packages** (CDK uses lightweight jsii bindings)
- Requires `github.com/agentplexus/agentkit` for shared configuration
- Requires Node.js runtime for CDK synthesis

## Related Modules

| Module | Purpose |
|--------|---------|
| [agentkit](https://github.com/agentplexus/agentkit) | Core library, shared IaC config, pure CloudFormation |
| [agentkit-aws-pulumi](https://github.com/agentplexus/agentkit-aws-pulumi) | Pulumi components for AWS |
| [agentkit-terraform](https://github.com/agentplexus/agentkit-terraform) | Terraform modules (planned) |

## Installation

```bash
go get github.com/agentplexus/agentkit-aws-cdk
```

## Prerequisites

1. AWS CDK CLI: `npm install -g aws-cdk`
2. AWS credentials configured
3. CDK bootstrapped: `cdk bootstrap aws://ACCOUNT-ID/REGION`

## License

MIT
