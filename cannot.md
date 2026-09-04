# Rust SDK feature tests that cannot be ported

## Investigate

## activity/shutdown

The test needs separate notifications for worker shutdown and server-requested activity
cancellation, including the pre-graceful-shutdown notification. Rust's `ActivityContext` exposes
only a combined cancellation token, so the test cannot distinguish or assert both events.

### Analysis
It looks like TS also doesn't have great support for this and does this via a side channel method we could probably copy.

Probably should add a worker shutdown future and assert expected behavior

## telemetry/metrics

The test records custom counters from both workflow and activity code. Rust Core can emit SDK
metrics, but Rust's public `WorkflowContext` and `ActivityContext` expose no user metric meter or
counter API, so those assertions cannot be reproduced.

### Analysis
Need to add metrics meter etc to rust

## Will not fix

## build_id_versioning/activity_and_child_on_correct_version

This test requires legacy Build ID worker versioning. Although Core still defines a legacy
strategy, the public Rust `WorkerOptions` API only configures deployment-based versioning and
cannot create the legacy versioned workers required by the test.

## build_id_versioning/continues_as_new_on_correct_version

This test requires legacy Build ID worker versioning. Although Core still defines a legacy
strategy, the public Rust `WorkerOptions` API only configures deployment-based versioning and
cannot create the legacy versioned workers required by the test.

## build_id_versioning/only_appropriate_worker_gets_task

This test requires legacy Build ID worker versioning. Although Core still defines a legacy
strategy, the public Rust `WorkerOptions` API only configures deployment-based versioning and
cannot create the legacy versioned workers required by the test.

## build_id_versioning/unversioned_worker_gets_unversioned_task

This test requires legacy Build ID worker versioning. Although Core still defines a legacy
strategy, the public Rust `WorkerOptions` API only configures deployment-based versioning and
cannot create the legacy versioned workers required by the test.

## build_id_versioning/unversioned_worker_no_task

This test requires legacy Build ID worker versioning. Although Core still defines a legacy
strategy, the public Rust `WorkerOptions` API only configures deployment-based versioning and
cannot create the legacy versioned workers required by the test.

## build_id_versioning/versions_added_while_worker_polling

This test requires legacy Build ID worker versioning. Although Core still defines a legacy
strategy, the public Rust `WorkerOptions` API only configures deployment-based versioning and
cannot create the legacy versioned workers required by the test.

## bugs/go/activity_start_race

This is a Go SDK replay regression test for temporalio/sdk-go#670. It exercises a panic in the Go
SDK's workflow state machine rather than a cross-SDK behavior, so there is no corresponding Rust
behavior to invoke or assert.

## bugs/go/child_workflow_cancel_panic

This is a regression test for an internal state-machine counter panic in Go SDK versions through
1.11.1. Rust does not use that Go state machine, so the failing behavior and its assertions cannot
be ported.

## client/http_proxy_nativeconn

This feature specifically tests the TypeScript SDK's distinct `NativeConnection` surface. The
Rust SDK has only its standard connection type, already covered by `client/http_proxy`, and has no
equivalent alternate connection API to test.

## client/http_proxy_nativeconn_auth

This feature specifically tests proxy authentication through the TypeScript SDK's distinct
`NativeConnection` surface. The Rust SDK has no equivalent alternate connection API; proxy
authentication through its standard connection is covered by `client/http_proxy_auth`.

## nexus/sync_success

Rust exposes workflow-side Nexus operation APIs, but its public worker API has no Nexus service or
operation-handler registration API. Without a way to register the synchronous operation, this
end-to-end test cannot be implemented.

