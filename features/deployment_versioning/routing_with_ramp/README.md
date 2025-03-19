# Deployment Versioning: Routing with Ramp

It is possible to redirect a subset of new workflow tasks to a new version, different from the current one, by setting the Deployment Ramping Version and Percentage.

# Detailed spec

* Create a random deployment name `deployment_name`
* Start a `deployment_name.1-0` worker, register workflow type `WaitForSignal` as `AutoUpgrade`, the implementation of that workflow should end returning `prefix_v1`.
* Start a `deployment_name.2-0` worker, register workflow type `WaitForSignal` as `AutoUpgrade`, the implementation of that workflow should end returning `prefix_v2`.
* Set Current version for `deployment_name` to `deployment_name.1-0`
* Start `workflow_1` of type `WaitForSignal`. It should start auto_upgrade and with version `deployment_name.1-0`.
* Set Ramp for `deployment_name` to `deployment_name.2-0` and `Percentage` to 100.
* Signal workflow. The workflow (pinned) should exit returning `prefix_v2`. 
