# Deployment Versioning: Routing with Override

It is possible to override the Version of a new workflow with Start Workflow
Options, so that it is pinned to a Version different from the Current one in that Deployment.


# Detailed spec

* Create a random deployment name `deployment_name`
* Start a `deployment_name.1-0` worker, register workflow type `WaitForSignal` as `Pinned`, the implementation of that workflow should end returning `prefix_v1`.
* Start a `deployment_name.2-0` worker, register workflow type `WaitForSignal` as `AutoUpgrade`, the implementation of that workflow should end returning `prefix_v2`.
* Set Current version for `deployment_name` to `deployment_name.2-0`
* Start `workflow_1` of type `WaitForSignal`, and override for `Pinned` to `deployment_name.1.0`. It should start Pinned and with version `deployment_name.1-0`.
* Signal workflow. The workflow (pinned) should exit returning `prefix_v1`. 


* Create a random deployment name `deployment_name`
* Start a `deployment_name.1-0` worker, register workflow type `DoNotWaitForSignal` as `Pinned`, the implementation of that workflow should end returning `1.0`.
* Start a `deployment_name.2-0` worker, register workflow type `DoNotWaitForSignal` as `Pinned`, the implementation of that workflow should end returning `2.0`.
* Set Current version for `deployment_name` to `deployment_name.1-0`
* Start `workflow_1` of type `DoNotWaitForSignal`, it should start pinned and with version `deployment_name.1-0`
* Start  `workflow_2` of type `DoNotWaitForSignal` with override start options for pinning to `deployment_name.1-0`. It should start pinned and with version `deployment_name.2-0`
* The first workflow (pinned) should exit returning `1.0`, and the second one `2.0`. 
