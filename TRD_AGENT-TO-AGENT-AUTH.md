# TRD: Agent-to-Agent Authentication on AWS AgentCore

**Document Type:** Technical Requirements Document
**Status:** Draft
**Date:** January 4, 2026
**Author:** AgentPlexus Team

## Executive Summary

This document defines the authentication architecture for agent-to-agent (A2A) communication in AgentPlexus when deployed on AWS Bedrock AgentCore. The key finding is that **AgentCore does NOT provide automatic mTLS or Nitro-enforced A2A encryption**. Instead, AWS expects you to use **IAM SigV4 or OAuth** for inter-agent authentication.

## Background

### What AgentCore Does NOT Provide

AgentCore runs your code inside Nitro-backed infrastructure, but that only protects:

- The host
- The runtime isolation
- Memory and CPU boundaries

It does **not** automatically secure traffic between agents.

> Nitro protects *where* agents run, not *how* they talk.

### What You Get for Free

Even without extra work:

1. **TLS in transit** - All AWS-managed endpoints require HTTPS (TLS 1.2+)
2. **IAM identity per agent** - Each AgentCore agent runs with a dedicated IAM role and short-lived STS credentials

## Architecture Decision

### Recommendation: IAM SigV4 for Agent-to-Agent Authentication

| Pattern | Use Case |
|---------|----------|
| **IAM SigV4** | All agents in AWS (RECOMMENDED) |
| OAuth/JWT | Cross-cloud agents or external IdP |
| mTLS | Regulated environments, legacy systems |

### Why IAM SigV4?

This is the canonical AWS approach:

- **Calling agent**: Signs HTTP requests using SigV4
- **Receiving agent**: Fronted by API Gateway or ALB with IAM auth

Security properties:

- Mutual authentication
- Strong identity (IAM role)
- Short-lived credentials
- No certificate management
- Auditable via CloudTrail

> In AWS, **IAM ≈ mTLS**, but at the API layer.

## Implementation Design

### Responsibility Boundaries

| Layer | Responsibility |
|-------|----------------|
| **AgentKit** | Transport, auth, signing, verification |
| Agent implementations | Business logic only |
| AgentCore | Runtime + IAM role |
| AgentPlexus A2A | Protocol semantics |

### Key Decision: SigV4 Belongs in AgentKit

SigV4 should be implemented **once in AgentKit** and reused everywhere. This mirrors:

- AWS SDKs (signing is in the SDK, not app code)
- gRPC interceptors
- HTTP middleware stacks

```
Agent
 └─ AgentKit
     ├─ A2A Client (SigV4 signer)
     ├─ A2A Server (SigV4 verifier via API Gateway)
     └─ Transport (HTTP)
```

### Auth Abstraction Interface

```go
type AuthProvider interface {
    Sign(req *http.Request) error
    Verify(req *http.Request) error  // Only needed if not using API Gateway
}
```

This allows future support for:

- OAuth
- mTLS
- Custom auth

### SigV4 Client Implementation (Outbound A2A)

```go
import (
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

type SigV4Signer struct {
    Creds   aws.CredentialsProvider
    Region  string
    Service string // e.g., "execute-api"
}

func (s *SigV4Signer) Sign(req *http.Request) error {
    creds, err := s.Creds.Retrieve(context.Background())
    if err != nil {
        return err
    }

    signer := v4.NewSigner()
    payloadHash := "UNSIGNED-PAYLOAD"

    return signer.SignHTTP(
        context.Background(),
        creds,
        req,
        payloadHash,
        s.Service,
        s.Region,
        time.Now(),
    )
}
```

### Server-Side Verification

**AgentCore does NOT validate SigV4 for you.**

Recommended pattern: **API Gateway in front of agents**

```
Agent A
  └─ AgentKit (SigV4)
        ↓
     API Gateway (IAM auth)
        ↓
     Agent B (AgentCore)
```

Benefits:

- AWS validates SigV4
- No crypto code in AgentKit server
- IAM policies enforce agent allowlists
- CloudTrail auditing

Alternative: ALB with IAM auth (slightly more setup, still no verification code needed).

### AgentKit Configuration API

```go
agentkit.NewAgent(
    agentkit.WithA2ATransport(
        agentkit.HTTPTransport{
            Auth: agentkit.SigV4(
                agentkit.SigV4Config{
                    Region:  "us-east-1",
                    Service: "execute-api",
                },
            ),
        },
    ),
)
```

Agent authors never need to:

- Import AWS SDK auth code
- Know about IAM
- Handle signatures

### Example IAM Policy

```json
{
  "Effect": "Allow",
  "Action": "execute-api:Invoke",
  "Resource": "arn:aws:execute-api:us-east-1:123456789012:agent-a2a/*",
  "Condition": {
    "StringEquals": {
      "aws:PrincipalArn": "arn:aws:iam::123456789012:role/agent-planner"
    }
  }
}
```

## Alternative Patterns

### OAuth/JWT (Cross-Cloud)

Use when:

- Agents may run outside AWS
- You want portable A2A
- You already have an IdP (Cognito, Okta, Auth0)

Flow:

1. Agent authenticates to IdP
2. Receives JWT
3. Sends `Authorization: Bearer <token>`
4. Receiving agent validates token

### mTLS (Advanced/Niche)

Use when:

- Regulated environments require it
- Existing mTLS mesh
- Non-HTTP transports

Requirements:

- Private CA (ACM PCA)
- Certificate distribution
- Certificate rotation
- Custom TLS servers in agents

**For most AgentCore users: overkill.**

## Defense-in-Depth Recommendations

### Minimum Viable Secure Setup

1. HTTPS everywhere
2. IAM SigV4 for A2A
3. VPC + security groups
4. Least-privilege IAM roles

### Production Setup

- IAM auth for A2A
- Private ALB / API Gateway
- VPC-only traffic
- CloudWatch + X-Ray with propagated trace IDs
- Explicit agent allowlists in IAM

## Decision Matrix

| Requirement | Best Choice |
|-------------|-------------|
| All agents in AWS | IAM (SigV4) |
| Cross-cloud agents | OAuth / JWT |
| Zero-trust + simplicity | IAM |
| Regulated + legacy | mTLS |
| "Nitro-secured channels" | Not a thing |

## AgentCore WorkloadIdentity

AgentCore provides `CfnWorkloadIdentity` for OAuth2 scenarios:

```go
type CfnWorkloadIdentityProps struct {
    Name                            *string      // Unique name
    AllowedResourceOauth2ReturnUrls *[]*string   // OAuth2 return URLs
    Tags                            *[]*CfnTag   // Resource tags
}
```

This is primarily for:

- External OAuth providers
- Cross-cloud agent communication
- Web-based OAuth flows

For pure AWS-to-AWS A2A, IAM SigV4 is preferred.

## Implementation Roadmap

### Phase 1: AgentKit Auth Abstraction

- [ ] Define `AuthProvider` interface in AgentKit
- [ ] Implement `SigV4Signer` for outbound requests
- [ ] Add configuration options to A2A client

### Phase 2: Infrastructure Support

- [ ] Add API Gateway creation to agentkit-aws-cdk
- [ ] Configure IAM auth on API Gateway
- [ ] Generate per-agent IAM policies

### Phase 3: OAuth Support (Optional)

- [ ] Implement `OAuthProvider` in AgentKit
- [ ] Add WorkloadIdentity to agentkit-aws-cdk
- [ ] Document OAuth configuration

## Open Questions

1. Should we support direct agent-to-agent calls without API Gateway?
2. What is the policy structure for per-agent allowlists?
3. How do we handle local development (auth bypass)?

## References

- AWS Bedrock AgentCore Documentation
- AWS SigV4 Signing Process
- A2A Protocol Specification
- MCP Gateway Architecture

## Changelog

| Date | Version | Changes |
|------|---------|---------|
| 2026-01-04 | Draft | Initial document |
