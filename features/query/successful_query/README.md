# Successful workflow query completion
A workflow query completes successfully if a worker running the respective workflow is available and
responsive, the workflow query matches the supported workflow query signature and the state of the
workflow is compatible with the workflow query options.

Each scenario in this folder should start a workflow and issue a well known query against it.
The query is expected to succeed and to return a well known value.
Sibling folders contain scenarios where a query fails in a well-known manner.


# Detailed spec
* Queries are received by the SDK inside either the `query` or `queries` fields of a poll WFT
  response.
* `query` queries must be replied to using the `RespondQueryTaskCompleted` RPC
* `queries` queries are replied to in the WFT response
* Query handlers run after applying everything else in a WFT
