# deploy

Deploy to AWS AgentCore with a single command.

## Installation

```bash
go install github.com/plexusone/agentkit-aws-cdk/cmd/deploy@latest
```

## Usage

Run from your CDK project directory (where `config.json` and `main.go` are located):

```bash
deploy [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--region` | `AWS_REGION` or `us-east-1` | AWS region |
| `--env` | auto-detect | Path to .env file for secrets |
| `--prefix` | `stats-agent` | Secret name prefix |
| `--project` | auto-detect | Project name for `~/.plexusone/projects/{project}/` lookup |
| `--dry-run` | `false` | Preview changes without deploying |
| `--skip-secrets` | `false` | Skip pushing secrets to Secrets Manager |
| `--skip-bootstrap` | `false` | Skip CDK bootstrap |
| `--verbose` | `false` | Show verbose output |

### Env File Auto-Detection

If `--env` is not specified, the tool searches in order:

1. `.env` (current directory)
2. `../.env` (parent directory)
3. `~/.plexusone/projects/{project}/.env` (project-specific)
4. `~/.plexusone/.env` (global fallback)

Project is auto-detected from `config.json` stackName, or can be specified with `--project`.

### Examples

```bash
# Full deployment from CDK directory
cd myproject/cdk
deploy

# Preview without making changes
deploy --dry-run

# Deploy to specific region
deploy --region us-west-2

# Skip secrets if already created
deploy --skip-secrets

# Use env file from parent directory
deploy --env ../.env
```

## What It Does

```
┌─────────────────────────────────────────────────────────────┐
│                         deploy                               │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Step 1: Push Secrets                                       │
│  ├── Reads .env file                                        │
│  ├── Categorizes keys (llm, search, config)                 │
│  └── Creates/updates AWS Secrets Manager secrets            │
│                                                             │
│  Step 2: Bootstrap CDK                                      │
│  └── Runs: cdk bootstrap aws://{account}/{region}           │
│                                                             │
│  Step 3: Deploy                                             │
│  ├── Runs: go mod tidy                                      │
│  └── Runs: cdk deploy --require-approval never              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Prerequisites

- AWS CLI configured with credentials
- AWS CDK CLI installed (`npm install -g aws-cdk`)
- Go 1.21+
- A CDK project with `config.json` and `main.go`

For config.json templates, see:

- [examples/2-cdk-json/config.json](../../examples/2-cdk-json/config.json) (JSON)
- [examples/2-cdk-json/config.yaml](../../examples/2-cdk-json/config.yaml) (YAML)

## Secret Groups

The tool automatically categorizes environment variables:

| Secret | Variables |
|--------|-----------|
| `{prefix}/llm` | `*_API_KEY` for LLM providers (Google, OpenAI, Anthropic, xAI) |
| `{prefix}/search` | `SERPER_API_KEY`, `SERPAPI_API_KEY` |
| `{prefix}/config` | `LLM_PROVIDER`, `LLM_MODEL`, `OBSERVABILITY_*`, etc. |

## Output

After successful deployment:

```bash
# Get stack outputs (including Gateway URL)
aws cloudformation describe-stacks \
  --stack-name stats-agent-team \
  --query 'Stacks[0].Outputs' \
  --no-cli-pager
```
