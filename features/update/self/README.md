# Workflow sends an update to itself

Workflows can send updates back to themselves

# Detailed spec

A workflow can use the workflowservice client in an activity to send an update
back to itself. Blocking on the activity future should guarantee that the update
effects are visible once the future returns.

