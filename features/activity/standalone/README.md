# Standalone activity

A client executes an activity directly, without a workflow.

Each feature:

* Starts the activity with the client on the feature task queue
* Waits for the activity result
* Describes the activity execution and confirms the `Completed` status

Requires a server with the dynamic config `frontend.activityAPIsEnabled=true`, enabled in `dockerfiles/dynamicconfig/docker.yaml`.
