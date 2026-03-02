// Example 3: CfnInclude - Import Existing CloudFormation Templates
//
// This approach imports an existing CloudFormation template into CDK.
// Best for: Teams with existing CloudFormation templates who want CDK deployment tooling.
//
// Deploy with:
//
//	cd examples/3-cfn-include
//	cdk deploy
package main

import (
	"github.com/plexusone/agentkit-aws-cdk/agentcore"
)

func main() {
	app := agentcore.NewApp()

	// Import existing CloudFormation template
	agentcore.NewCfnIncludeBuilder("stats-agent-team", "template.yaml").
		WithParameter("Environment", "production").
		WithTags(map[string]string{
			"Project":   "stats-agent-team",
			"ManagedBy": "agentkit-cdk",
		}).
		Build(app)

	agentcore.Synth(app)
}
