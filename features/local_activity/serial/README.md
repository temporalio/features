# Serial Local Activities

3 Local Activities run serially in the same Workflow Task.
The Workflow Task should be resolved generating 3 marker events.


# Detailed spec
* When local activities resolve, the SDK does not immediately complete the current WFT. Instead,
  it waits until all LAs or complete (or not, see [heartbeating](../complete_eventually)).
* If multiple LAs in a row complete within the same workflow task the SDK will issue `RecordMarker`
  commands for each, containing their respective results.