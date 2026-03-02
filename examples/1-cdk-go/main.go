// Example 1: CDK Go Constructs
//
// This approach uses type-safe Go code with CDK constructs.
// Best for: Teams comfortable with Go who want full IDE support and type safety.
//
// Deploy with:
//
//	cd examples/1-cdk-go
//	cdk deploy
package main

import (
	"github.com/plexusone/agentkit-aws-cdk/agentcore"
)

func main() {
	app := agentcore.NewApp()

	// Build agent configurations using the fluent builder API
	research := agentcore.NewAgentBuilder("research", "ghcr.io/agentplexus/stats-agent-research:latest").
		WithDescription("Research agent - web search via Serper").
		WithMemory(512).
		WithTimeout(30).
		WithEnvVar("LOG_LEVEL", "info").
		Build()

	synthesis := agentcore.NewAgentBuilder("synthesis", "ghcr.io/agentplexus/stats-agent-synthesis:latest").
		WithDescription("Synthesis agent - extract statistics from URLs").
		WithMemory(1024).
		WithTimeout(120).
		Build()

	verification := agentcore.NewAgentBuilder("verification", "ghcr.io/agentplexus/stats-agent-verification:latest").
		WithDescription("Verification agent - validate sources").
		WithMemory(512).
		WithTimeout(60).
		Build()

	orchestration := agentcore.NewAgentBuilder("orchestration", "ghcr.io/agentplexus/stats-agent-orchestration-eino:latest").
		WithDescription("Orchestration agent - coordinate workflow").
		WithMemory(512).
		WithTimeout(300).
		AsDefault().
		Build()

	// Build the stack using the fluent builder API
	agentcore.NewStackBuilder("stats-agent-team").
		WithDescription("Statistics research and verification multi-agent system").
		WithAgents(research, synthesis, verification, orchestration).
		WithNewVPC("10.0.0.0/16", 2).
		WithOpik("stats-agent-team", "arn:aws:secretsmanager:us-east-1:123456789:secret:opik-key").
		WithTags(map[string]string{
			"Project":     "stats-agent-team",
			"Environment": "production",
			"Team":        "ai-platform",
		}).
		Build(app)

	agentcore.Synth(app)
}
