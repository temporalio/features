# Workflow-to-workflow signals

Signals can be sent from one workflow to another workflow in the
same namespace via the language's Temporal SDK API.

# Detailed spec

Signals sent from within one workflow via the workflow api are transmitted to the target
workflow execution and delivered for processing.
