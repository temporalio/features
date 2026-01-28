# Upgrade on Continue-as-New

This snippet demonstrates how a pinned Workflow can upgrade to a new Worker Deployment Version at Continue-as-New boundaries.

## Pattern

Long-running Workflows that use Continue-as-New can upgrade to newer Worker Deployment Versions without patching by:

1. Checking `GetContinueAsNewSuggested()` periodically
2. Looking for `ContinueAsNewSuggestedReasonTargetWorkerDeploymentVersionChanged`
3. Using `ContinueAsNewVersioningBehaviorAutoUpgrade` when continuing

## Use Cases

- Entity Workflows running for months or years
- Batch processing Workflows that checkpoint with Continue-as-New
- AI agent Workflows with long sleeps waiting for user input
