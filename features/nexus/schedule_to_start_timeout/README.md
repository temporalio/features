# Schedule-to-start timeout

A Nexus operation with no handler polling its endpoint reaches its
schedule-to-start timeout before its longer schedule-to-close timeout.
Both the execution future and result future fail with a Nexus operation failure
whose immediate cause is a schedule-to-start timeout, with no operation token.
The history contains scheduled and timed-out events, but no started event.

Requires Temporal Server 1.31.0 or later.
