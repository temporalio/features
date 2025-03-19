# Deployment Versioning: Version Routing Pinned

When the Current Version for a Deployment changes, already started workflows
will never transition to the new version if they are `Pinned`.

# Detailed spec

* Create a random deployment name `deployment_name`
* Start a `deployment_name.1-0` worker, register workflow type `WaitForSignal` as `Pinned`, the implementation of that workflow should end returning `prefix_v1`.
* Start a `deployment_name.2-0` worker, register workflow type `WaitForSignal` as `AutoUpgrade`, the implementation of that workflow should end returning `prefix_v2`.
* Set Current version for `deployment_name` to `deployment_name.1-0`
* Start `workflow_1` of type `WaitForSignal`, it should start pinned and with version `deployment_name.1-0`
* Set Current version for `deployment_name` to `deployment_name.2-0`
* Signal workflow. The workflow (pinned) should exit returning `prefix_v1`. 
