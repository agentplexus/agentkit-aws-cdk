# TRD: External Tool Communication

**Document Type:** Technical Requirements Document
**Status:** Draft
**Priority:** P2 - Future Enhancement (v0.2.0 uses Direct + VaultGuard)
**Date:** January 4, 2026
**Author:** AgentPlexus Team

## Executive Summary

This document defines the architecture for agent communication with external tools and services (search APIs, observability platforms, etc.) when deployed on AWS Bedrock AgentCore.

### v0.2.0 Scope

v0.2.0 uses **Direct + VaultGuard** for all external tool communication:

- Agents make outbound HTTPS calls directly
- API keys stored in AWS Secrets Manager via VaultGuard
- NAT Gateway provides internet access from VPC

### Future Enhancement (P2)

Gateway + OpenAPI for centralized credential management when:

- Multiple agent teams share the same external tools
- Agents should have zero knowledge of API credentials
- Centralized audit logging of external API usage

## Communication Patterns

### Pattern 1: Direct + VaultGuard (v0.2.0)

```
Agent (AgentCore)
    ↓ HTTPS + API key (from VaultGuard)
External API (Serper, Opik, etc.)
```

**Characteristics:**

- Simple, fewer moving parts
- Agent holds API keys (via env vars or VaultGuard)
- Each agent manages its own tool calls
- NAT Gateway required for outbound internet

**Configuration:**

```yaml
vpc:
  createVPC: true
  enableVPCEndpoints: true  # For AWS services
  # NAT Gateway created by default

# API keys via environment variables
agents:
  - name: research
    containerImage: ghcr.io/agentplexus/research:latest
    # Keys injected from Secrets Manager
```

### Pattern 2: Gateway + OpenAPI (Future)

```
Agent (IAM identity)
    ↓ MCP protocol (authenticated via IAM)
Gateway
    ↓ Injects API key from GatewayTarget
External API
```

**Characteristics:**

- Agents have zero knowledge of API credentials
- Centralized credential management
- Requires OpenAPI spec for each external service
- More infrastructure complexity

**GatewayTarget with OpenAPI:**

```go
McpTargetConfigurationProperty{
    OpenApiSchema: &ApiSchemaConfigurationProperty{
        InlinePayload: jsii.String("{ openapi spec }"),
        // or S3 location
        S3: &S3ConfigurationProperty{
            Uri: jsii.String("s3://bucket/openapi.yaml"),
        },
    },
}
```

## External Service Analysis

### Search APIs

| Service | OpenAPI Spec | v0.2.0 Approach | Gateway Compatible |
|---------|--------------|-----------------|-------------------|
| **Serper** | ❌ No official | Direct + VaultGuard | Needs MCP server |
| **SerpAPI** | ❌ Not found | Direct + VaultGuard | Needs MCP server |
| **OmniSerp** | ✅ MCP server | Direct or Gateway | ✅ Yes |

**Recommendation:** Use Direct + VaultGuard for Serper/SerpAPI. Simple HTTP calls, no OpenAPI spec available.

### Observability Platforms

| Service | OpenAPI Spec | Protocol | v0.2.0 Approach |
|---------|--------------|----------|-----------------|
| **Opik** | ✅ Yes | OTLP + REST | Direct OTLP |
| **Phoenix** | ✅ Partial | OTEL + REST | Direct OTLP |
| **Arize** | N/A | OTLP | Direct OTLP |
| **Datadog** | ✅ Yes | OTLP + REST | Direct OTLP |

**Opik OpenAPI Location:** [`sdks/code_generation/fern/openapi/openapi.yaml`](https://github.com/comet-ml/opik)

**Phoenix:** Uses `arize-phoenix-client` with OpenAPI REST interface for queries. Tracing uses OTEL protocol.

**Recommendation:** Use Direct OTLP for observability. Agents send traces via OpenTelemetry protocol. REST API (with OpenAPI) is primarily for querying/dashboard, not agent instrumentation.

### LLM Providers

| Provider | OpenAPI Spec | v0.2.0 Approach |
|----------|--------------|-----------------|
| **OpenAI** | ✅ Yes | Direct via OmniLLM |
| **Anthropic** | ✅ Yes | Direct via OmniLLM |
| **Google AI** | ✅ Yes | Direct via OmniLLM |
| **AWS Bedrock** | N/A (AWS SDK) | Direct via OmniLLM |

**Recommendation:** Use OmniLLM adapter for all LLM calls. Handles provider abstraction, retries, and credential management.

## Infrastructure Requirements

### VPC Configuration

```yaml
vpc:
  createVPC: true
  vpcCidr: 10.0.0.0/16
  maxAZs: 2
  enableVPCEndpoints: true  # Bedrock, Secrets Manager, CloudWatch
```

**NAT Gateway:** Created automatically for outbound internet access. Required for:

- External API calls (Serper, Opik, etc.)
- Container image pulls (if not using VPC endpoints)

### Secrets Management

**VaultGuard + AWS Secrets Manager:**

```go
// In agent code
vaultguard.GetSecret(ctx, "serper-api-key")
vaultguard.GetSecret(ctx, "opik-api-key")
```

**Environment Variables:**

```yaml
agents:
  - name: research
    environment:
      SERPER_API_KEY: "{{resolve:secretsmanager:serper-api-key}}"
      OPIK_API_KEY: "{{resolve:secretsmanager:opik-api-key}}"
```

## Decision Matrix

| Scenario | Recommended Approach |
|----------|---------------------|
| Simple external API calls | Direct + VaultGuard |
| Observability/tracing | Direct OTLP |
| LLM provider calls | OmniLLM adapter |
| Multiple teams sharing tools | Gateway + OpenAPI (future) |
| Agent credential isolation | Gateway + OpenAPI (future) |
| No OpenAPI spec available | Direct or custom MCP server |

## Gateway Target Configuration

For future Gateway + OpenAPI integration:

### Option 1: OpenAPI Schema

```go
GatewayTarget{
    TargetConfiguration: &TargetConfigurationProperty{
        Mcp: &McpTargetConfigurationProperty{
            OpenApiSchema: &ApiSchemaConfigurationProperty{
                S3: &S3ConfigurationProperty{
                    Uri: jsii.String("s3://specs/opik-openapi.yaml"),
                },
            },
        },
    },
    CredentialProviders: []CredentialProviderProperty{
        {
            ApiKey: &ApiKeyCredentialProviderProperty{
                // Secrets Manager reference
            },
        },
    },
}
```

### Option 2: MCP Server

```go
GatewayTarget{
    TargetConfiguration: &TargetConfigurationProperty{
        Mcp: &McpTargetConfigurationProperty{
            McpServer: &McpServerTargetConfigurationProperty{
                Endpoint: jsii.String("https://mcp.example.com"),
            },
        },
    },
}
```

### Option 3: Lambda

```go
GatewayTarget{
    TargetConfiguration: &TargetConfigurationProperty{
        Mcp: &McpTargetConfigurationProperty{
            Lambda: &McpLambdaTargetConfigurationProperty{
                LambdaArn: jsii.String("arn:aws:lambda:..."),
                ToolSchema: &ToolSchemaProperty{
                    // Tool definitions
                },
            },
        },
    },
}
```

## MCP Server Hosting Options

### Mental Model

An MCP server is just a network service. You can host it on Lambda, AgentCore, Fargate, EC2, EKS — anything that can expose HTTP(S). **Gateway does not care where it runs.**

The real constraints are about **lifecycle, latency, state, and credential boundaries**, not "what AWS service."

#### Three Distinct Roles

| Component | Role |
|-----------|------|
| **AgentCore agent** | Reasoning + orchestration |
| **Gateway** | Credential injection + tool exposure |
| **MCP server** | Tool implementation |

Gateway injects credentials, standardizes auth, and exposes MCP endpoints. It **does not host** MCP servers.

### Hosting Options

#### Lambda

**Best for:** Stateless, short-lived tools

| Pros | Cons |
|------|------|
| Zero infra management | Cold starts |
| Scales automatically | Execution limits |
| Cheap for bursty workloads | Not great for long-running MCP streams |
| IAM-native | |

**Good MCP examples:** Read-only API wrappers, lightweight data lookups, simple transformations

#### AgentCore

**Best for:** Tight agent/tool coupling

| Pros | Cons |
|------|------|
| Same runtime model as agents | You manage MCP server lifecycle |
| Easy IAM isolation | Not a "managed MCP service" |
| Low latency | |
| Good for agent-adjacent tools | |

**Typical use:** Internal MCP tools owned by the agent team, tools that share context or logic with agents

> Especially attractive if you already use AgentKit — MCP server + agent in the same codebase.

#### Fargate

**Best for:** Persistent, stateful MCP servers

| Pros | Cons |
|------|------|
| Long-running processes | Higher baseline cost |
| Good for streaming MCP | More infra setup |
| No EC2 management | |
| Predictable latency | |

**Good MCP examples:** Database-backed tools, long-lived connections, stateful analyzers

#### EC2

**Best for:** Custom runtime or legacy services

| Pros | Cons |
|------|------|
| Full control | You own everything |
| Any language / protocol | Scaling, patching, security |
| Easy migration path | |

Usually chosen only if MCP wraps an existing service or you already run EC2 fleets.

#### EKS (Kubernetes)

**Best for:** Multi-tenant MCP platforms

| Pros | Cons |
|------|------|
| Horizontal scaling | Operational complexity |
| Service mesh support | Overkill for many teams |
| Good for many MCP servers | |

Makes sense if MCP servers are first-class products, you need tenant isolation, or you already run Kubernetes.

### AWS Implicit Guidance

Gateway-supported targets signal AWS's expectations:

| Target | Implied Hosting |
|--------|-----------------|
| Lambda | "Serverless tools" |
| HTTP APIs | Any compute |
| Existing MCP servers | Bring-your-own |
| AWS services | Managed |

There is **no "MCP hosting service"** on purpose. AWS expects:

- **Small MCPs → Lambda**
- **Internal MCPs → AgentCore or Fargate**
- **Platform MCPs → Fargate / EKS**

### Credential Injection Implications

Gateway handles credentials — not runtime identity.

**Gateway:**

- Injects credentials into requests
- Signs calls
- Enforces policies

**Your MCP server:**

- Does **not** need AWS credentials
- Does **not** know about IAM
- Just trusts the Gateway

This makes **Lambda, AgentCore, Fargate, EC2 all equivalent** from a security standpoint.

### Common Deployment Patterns

#### Pattern A: Agent + MCP Side-by-Side (AgentCore)

```
AgentCore
 ├─ Agent
 └─ MCP Server
```

- Lowest latency
- Tight ownership
- Great for internal tools

#### Pattern B: Serverless MCPs

```
Agent → Gateway → Lambda MCP
```

- Clean separation
- Easy scaling
- Ideal starting point

#### Pattern C: Central MCP Platform

```
Agent → Gateway → Fargate/EKS MCP cluster
```

- Shared tools
- Multi-agent reuse
- Platform ownership

### MCP Hosting Recommendation

| Requirement | Best Choice |
|-------------|-------------|
| Simple, stateless MCP | Lambda |
| Agent-owned tools | AgentCore |
| Stateful / streaming MCP | Fargate |
| Existing service | EC2 |
| Multi-tenant MCP platform | EKS |

### What AWS Does NOT Offer

- ❌ A managed MCP hosting service
- ❌ Auto-scaled MCP runtime
- ❌ MCP registry with lifecycle management

This is intentional — MCP is meant to stay **portable and framework-agnostic**.

## Implementation Roadmap

### v0.2.0 (Current)

- [x] VPC with NAT Gateway for outbound access
- [x] Environment variable injection
- [ ] VaultGuard integration for Secrets Manager

### P2 (Future)

- [ ] Gateway + OpenAPI for Opik
- [ ] Centralized credential management
- [ ] Audit logging for external API calls

## Open Questions

1. Should we create MCP servers for Serper/SerpAPI, or continue with direct calls?
2. Is there value in Gateway for observability, or is Direct OTLP sufficient?
3. Cost implications of NAT Gateway vs VPC endpoints for external access?

## References

- [Opik GitHub - OpenAPI Spec](https://github.com/comet-ml/opik)
- [Phoenix GitHub](https://github.com/Arize-ai/phoenix)
- [Phoenix OTEL Setup](https://arize.com/docs/phoenix/tracing/how-to-tracing/setup-tracing/setup-using-phoenix-otel)
- [AWS Secrets Manager](https://docs.aws.amazon.com/secretsmanager/)
- [VaultGuard Documentation](https://github.com/agentplexus/vaultguard)

## Changelog

| Date | Version | Changes |
|------|---------|---------|
| 2026-01-04 | Draft | Initial document |
