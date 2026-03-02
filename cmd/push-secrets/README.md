# push-secrets

Push environment variables from `.env` files to AWS Secrets Manager.

## Installation

```bash
go install github.com/plexusone/agentkit-aws-cdk/cmd/push-secrets@latest
```

## Usage

```bash
push-secrets [flags] [env-file]
```

If `env-file` is not specified, searches in order:

1. `.env` (current directory)
2. `../.env` (parent directory)
3. `~/.plexusone/projects/{project}/.env` (project-specific)
4. `~/.plexusone/.env` (global fallback)

Project is auto-detected from `config.json` stackName, or can be specified with `--project`.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--region` | `AWS_REGION` or `us-east-1` | AWS region |
| `--prefix` | `stats-agent` | Secret name prefix |
| `--project` | auto-detect | Project name for `~/.plexusone/projects/{project}/` lookup |
| `--dry-run` | `false` | Preview changes without creating secrets |
| `--verbose` | `false` | Show verbose output |

### Examples

```bash
# Auto-detect .env file (uses config.json stackName for project)
push-secrets

# Use specific project
push-secrets --project stats-agent-team

# Push from specific file
push-secrets .env

# Push to specific region
push-secrets --region us-west-2 .env

# Use custom prefix
push-secrets --prefix myapp .env

# Preview without creating (recommended first run)
push-secrets --dry-run .env

# Verbose output
push-secrets --verbose --dry-run .env
```

## Secret Groups

Keys are automatically categorized into logical groups:

| Secret | Keys | Description |
|--------|------|-------------|
| `{prefix}/llm` | `GOOGLE_API_KEY`, `GEMINI_API_KEY`, `ANTHROPIC_API_KEY`, `CLAUDE_API_KEY`, `OPENAI_API_KEY`, `XAI_API_KEY`, `LLM_API_KEY` | LLM provider API keys |
| `{prefix}/search` | `SERPER_API_KEY`, `SERPAPI_API_KEY` | Search provider API keys |
| `{prefix}/config` | `LLM_PROVIDER`, `LLM_MODEL`, `SEARCH_PROVIDER`, `OBSERVABILITY_*`, `OPIK_*`, `LANGFUSE_*`, `PHOENIX_*` | Configuration and observability |

## Input File Format

Supports both `.env` and `.envrc` formats:

```bash
# Comments are ignored
KEY=value
export KEY=value
KEY="quoted value"
KEY='single quoted'
```

Placeholder values (starting with `your-`) are automatically skipped.

## AWS Credentials

The tool uses the standard AWS SDK credential chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role (when running on EC2/ECS/Lambda)

## Required IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:CreateSecret",
        "secretsmanager:PutSecretValue",
        "secretsmanager:DescribeSecret"
      ],
      "Resource": "arn:aws:secretsmanager:*:*:secret:stats-agent/*"
    }
  ]
}
```

## Example Output

```
$ push-secrets --dry-run .env
Reading from: .env
AWS Region: us-east-1
Secret prefix: stats-agent
Mode: DRY RUN (no changes will be made)

Creating/updating: stats-agent/llm
  Keys: ANTHROPIC_API_KEY, OPENAI_API_KEY
  [DRY RUN] Would create with: {"ANTHROPIC_API_KEY":"sk-ant-a***","OPENAI_API_KEY":"sk-proj-***"}
Creating/updating: stats-agent/search
  Keys: SERPER_API_KEY
  [DRY RUN] Would create with: {"SERPER_API_KEY":"c5177d1c***"}
Creating/updating: stats-agent/config
  Keys: LLM_PROVIDER, LLM_MODEL, SEARCH_PROVIDER
  [DRY RUN] Would create with: {"LLM_MODEL":"gpt-4","LLM_PROVIDER":"openai","SEARCH_PROVIDER":"serper"}

Done!

To verify:
  aws secretsmanager list-secrets --region us-east-1 --filter Key=name,Values=stats-agent/ --no-cli-pager
```
