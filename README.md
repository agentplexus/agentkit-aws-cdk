# AgentKit for AWS CDK

[![Build Status][build-status-svg]][build-status-url]
[![Lint Status][lint-status-svg]][lint-status-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![License][license-svg]][license-url]

AWS CDK constructs for deploying [agentkit](https://github.com/agentplexus/agentkit)-based agents to AWS Bedrock AgentCore.

## Scope

This module provides **AWS CDK** constructs only. For other IaC tools:

| IaC Tool | Module | Dependencies |
|----------|--------|--------------|
| **AWS CDK** | `agentkit-aws-cdk` (this module) | 21 |
| **Pulumi** | [agentkit-aws-pulumi](https://github.com/agentplexus/agentkit-aws-pulumi) | 340 |
| **CloudFormation** | [agentkit](https://github.com/agentplexus/agentkit) (core) | 0 extra |

All modules share the same YAML/JSON configuration schema from `agentkit/platforms/agentcore/iac/`.

## Architecture

```
agentkit/                              # Core library (no CDK deps)
├── platforms/agentcore/iac/
│   ├── config.go                      # Shared config structs
│   ├── loader.go                      # JSON/YAML loading
│   └── cloudformation.go              # Pure CloudFormation generator

agentkit-aws-cdk/                          # AWS CDK constructs (this module)
├── agentcore/
│   ├── stack.go                       # CDK constructs
│   ├── builder.go                     # Fluent builders
│   ├── cfninclude.go                  # CfnInclude wrapper
│   └── loader.go                      # CDK stack loaders
```

**Why two modules?**
- `agentkit` stays lean - no CDK runtime dependencies
- `agentkit-aws-cdk` adds CDK tooling for those who want it
- Pure CloudFormation (approach 4) works with just `agentkit`

## Four Deployment Approaches

| Approach | Module Required | Best For |
|----------|-----------------|----------|
| [1. CDK Go Constructs](#1-cdk-go-constructs) | agentkit-aws-cdk | Type safety, IDE support, complex logic |
| [2. CDK + JSON/YAML](#2-cdk--jsonyaml-config) | agentkit-aws-cdk | Configuration-driven deployments |
| [3. CfnInclude](#3-cfninclude) | agentkit-aws-cdk | Existing CloudFormation templates |
| [4. Pure CloudFormation](#4-pure-cloudformation) | agentkit only | No CDK runtime, AWS CLI only |

## Installation

**For CDK approaches (1-3):**
```bash
go get github.com/agentplexus/agentkit-aws-cdk
```

**For Pure CloudFormation (4):**
```bash
go get github.com/agentplexus/agentkit
```

---

## 1. CDK Go Constructs

Type-safe Go code with full IDE support and compile-time validation.

```go
package main

import "github.com/agentplexus/agentkit-aws-cdk/agentcore"

func main() {
    app := agentcore.NewApp()

    // Build agents with fluent API
    research := agentcore.NewAgentBuilder("research", "ghcr.io/example/research:latest").
        WithMemory(512).
        WithTimeout(30).
        Build()

    orchestration := agentcore.NewAgentBuilder("orchestration", "ghcr.io/example/orchestration:latest").
        WithMemory(1024).
        WithTimeout(300).
        AsDefault().
        Build()

    // Build stack
    agentcore.NewStackBuilder("my-agents").
        WithAgents(research, orchestration).
        WithOpik("my-project", "arn:aws:secretsmanager:us-east-1:123456789:secret:opik-key").
        WithTags(map[string]string{"Environment": "production"}).
        Build(app)

    agentcore.Synth(app)
}
```

**Deploy:**
```bash
cdk deploy
```

See [examples/1-cdk-go](examples/1-cdk-go/) for complete example.

---

## 2. CDK + JSON/YAML Config

Minimal Go wrapper that loads configuration from JSON or YAML files. Perfect for teams who prefer configuration over code.

**main.go** (never changes):
```go
package main

import "github.com/agentplexus/agentkit-aws-cdk/agentcore"

func main() {
    app := agentcore.NewApp()
    agentcore.MustNewStackFromFile(app, "config.yaml")
    agentcore.Synth(app)
}
```

**config.yaml**:
```yaml
stackName: my-agents
description: My AgentCore deployment

agents:
  - name: research
    containerImage: ghcr.io/example/research:latest
    memoryMB: 512
    timeoutSeconds: 30

  - name: orchestration
    containerImage: ghcr.io/example/orchestration:latest
    memoryMB: 1024
    timeoutSeconds: 300
    isDefault: true

observability:
  provider: opik
  project: my-project
  enableCloudWatchLogs: true

tags:
  Environment: production
```

**Deploy:**
```bash
cdk deploy
```

See [examples/2-cdk-json](examples/2-cdk-json/) for complete example.

---

## 3. CfnInclude

Import existing CloudFormation templates into CDK. Use CDK deployment tooling while keeping your existing templates.

**main.go**:
```go
package main

import "github.com/agentplexus/agentkit-aws-cdk/agentcore"

func main() {
    app := agentcore.NewApp()

    agentcore.NewCfnIncludeBuilder("my-agents", "template.yaml").
        WithParameter("Environment", "production").
        Build(app)

    agentcore.Synth(app)
}
```

**Deploy:**
```bash
cdk deploy
```

See [examples/3-cfn-include](examples/3-cfn-include/) for complete example.

---

## 4. Pure CloudFormation

Generate CloudFormation templates from configuration files. **No CDK runtime needed** - deploy with AWS CLI. Uses only `agentkit` (not `agentkit-aws-cdk`).

**generate.go**:
```go
package main

import (
    "fmt"
    "os"

    "github.com/agentplexus/agentkit/platforms/agentcore/iac"
)

func main() {
    config, err := iac.LoadStackConfigFromFile("config.yaml")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    if err := iac.GenerateCloudFormationFile(config, "template.yaml"); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("Generated template.yaml")
}
```

**Deploy with AWS CLI:**
```bash
go run generate.go
aws cloudformation deploy \
  --template-file template.yaml \
  --stack-name my-agents \
  --capabilities CAPABILITY_IAM CAPABILITY_NAMED_IAM
```

See [examples/4-pure-cloudformation](examples/4-pure-cloudformation/) for complete example.

---

## Configuration Reference

### StackConfig

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `stackName` | string | Yes | CloudFormation stack name |
| `description` | string | No | Stack description |
| `agents` | []AgentConfig | Yes | List of agents to deploy |
| `vpc` | VPCConfig | No | VPC configuration |
| `observability` | ObservabilityConfig | No | Monitoring configuration |
| `iam` | IAMConfig | No | IAM configuration |
| `tags` | map[string]string | No | Resource tags |
| `removalPolicy` | string | No | "destroy" or "retain" |

### AgentConfig

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Agent identifier |
| `containerImage` | string | Yes | ECR image URI |
| `description` | string | No | Human-readable description |
| `memoryMB` | int | No | Memory: 512, 1024, 2048, 4096, 8192, 16384 |
| `timeoutSeconds` | int | No | Timeout: 1-900 seconds |
| `environment` | map[string]string | No | Environment variables |
| `secretsARNs` | []string | No | Secret ARNs to inject |
| `isDefault` | bool | No | Mark as default agent |

### VPCConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `createVPC` | bool | true | Create new VPC |
| `vpcCidr` | string | 10.0.0.0/16 | VPC CIDR block |
| `maxAZs` | int | 2 | Number of availability zones |
| `enableVPCEndpoints` | bool | true | Create VPC endpoints |
| `vpcId` | string | - | Existing VPC ID |
| `subnetIds` | []string | - | Existing subnet IDs |

### ObservabilityConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `provider` | string | opik | opik, langfuse, phoenix, cloudwatch |
| `project` | string | stackName | Project name for traces |
| `apiKeySecretARN` | string | - | Secret ARN for API key |
| `enableCloudWatchLogs` | bool | true | Enable CloudWatch Logs |
| `logRetentionDays` | int | 30 | Log retention period |
| `enableXRay` | bool | false | Enable X-Ray tracing |

---

## Prerequisites

1. **Install AWS CDK CLI** (for approaches 1-3):
   ```bash
   npm install -g aws-cdk
   ```

2. **Configure AWS credentials**:
   ```bash
   aws configure
   ```

3. **Bootstrap CDK** (first time only, for approaches 1-3):
   ```bash
   cdk bootstrap aws://ACCOUNT-ID/REGION
   ```

---

## Project Structure

```
my-project/
├── infrastructure/
│   └── cdk/
│       ├── go.mod
│       ├── main.go          # CDK app (approaches 1-3)
│       ├── config.yaml      # Configuration (approaches 2, 4)
│       └── cdk.json         # CDK config
├── agents/
│   ├── research/
│   ├── synthesis/
│   └── orchestration/
└── go.mod
```

---

## License

Apache 2.0

 [build-status-svg]: https://github.com/agentplexus/agentkit-aws-cdk/actions/workflows/ci.yaml/badge.svg?branch=main
 [build-status-url]: https://github.com/agentplexus/agentkit-aws-cdk/actions/workflows/ci.yaml
 [lint-status-svg]: https://github.com/agentplexus/agentkit-aws-cdk/actions/workflows/lint.yaml/badge.svg?branch=main
 [lint-status-url]: https://github.com/agentplexus/agentkit-aws-cdk/actions/workflows/lint.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/agentplexus/agentkit-aws-cdk
 [goreport-url]: https://goreportcard.com/report/github.com/agentplexus/agentkit-aws-cdk
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/agentplexus/agentkit-aws-cdk
 [docs-godoc-url]: https://pkg.go.dev/github.com/agentplexus/agentkit-aws-cdk
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/agentplexus/agentkit-aws-cdk/blob/master/LICENSE
