#!/bin/bash
# Deploy CloudFormation template without CDK
#
# Usage:
#   ./deploy.sh                 # Generate and deploy
#   ./deploy.sh --generate-only # Only generate template

set -e

STACK_NAME="stats-agent-team"
TEMPLATE_FILE="template.yaml"
CONFIG_FILE="config.yaml"

# Generate template
echo "Generating CloudFormation template..."
go run generate.go "$CONFIG_FILE"

if [ "$1" == "--generate-only" ]; then
    echo "Template generated: $TEMPLATE_FILE"
    exit 0
fi

# Validate template
echo "Validating template..."
aws cloudformation validate-template --template-body "file://$TEMPLATE_FILE"

# Deploy
echo "Deploying stack: $STACK_NAME..."
aws cloudformation deploy \
    --template-file "$TEMPLATE_FILE" \
    --stack-name "$STACK_NAME" \
    --capabilities CAPABILITY_IAM CAPABILITY_NAMED_IAM \
    --no-fail-on-empty-changeset

# Show outputs
echo ""
echo "Stack outputs:"
aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --query 'Stacks[0].Outputs' \
    --output table

echo ""
echo "Deployment complete!"
