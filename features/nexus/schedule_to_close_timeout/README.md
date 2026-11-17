# Schedule-to-close timeout

A Nexus operation with no handler polling its endpoint times out before it starts.
Both the execution future and result future fail with a Nexus operation failure
whose immediate cause is a schedule-to-close timeout, with no operation token.
The history contains scheduled and timed-out events, but no started event.
