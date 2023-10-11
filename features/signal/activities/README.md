# Run activities from within a Workflow Signal

Workflows can run activities from within a Signal handler.

# Detailed spec

Workflow Signal handlers are like workflow code in that they can invoke and wait on activities.
This test invokes a number of activities within the Signal and blocks on them, returning from the Signal when they have all completed.
