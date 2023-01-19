# Run activities from within a workflow update

Workflows can run activities from within an update handler

# Detailed spec

Workflow update handlers are like workflow code in that they can run for
indefinitely long periods but must do so by invoking and waiting on activites.
This test invokes a number of activities within the update and blocks on them,
returning from the update when they have all completed.

