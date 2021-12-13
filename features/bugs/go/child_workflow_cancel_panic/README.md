# Child workflow cancel causing panic in Go SDK

In certain situations in versions <= v1.11.1, cancellation of a workflow with child workflows in certain situations
would cause the internal Go state machine counter to fail. This has been fixed in 
https://github.com/temporalio/sdk-go/pull/647 and is tested here.