# Update a workflow from a gRPC client returning the update response directly

In Temporal we can invoke a registered workflow update function and by
indicating that the call should include the update result "inline," the value
returned by the function call is returned as part of the gRPC response.

# Detailed spec

The OSS server supports the `UPDATE_WORKFLOW_RESULT_ACCESS_STYLE_REQUIRE_INLINE`
value for the update request's `result_access_style`. When this value is
supplied, the server blocks thread handling the gRPC invocation until either (a)
the call times out (in which case an error is returned) or (b) the the workflow
update function has run to completion and returned a value. In the latter case,
the returned value is included as part of the gRPC response.
