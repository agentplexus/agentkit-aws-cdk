// Example 2: CDK + JSON Config
//
// This approach uses a minimal Go wrapper that loads configuration from JSON/YAML.
// Best for: Teams who prefer configuration files over code.
//
// Deploy with:
//
//	cd examples/2-cdk-json
//	cdk deploy
package main

import (
	"github.com/agentplexus/agentkit-aws-cdk/agentcore"
)

func main() {
	app := agentcore.NewApp()

	// Load configuration from JSON file - that's it!
	// All configuration is in config.json, no Go code changes needed.
	agentcore.MustNewStackFromFile(app, "config.json")

	agentcore.Synth(app)
}
