# Async start-to-close timeout

A workflow-backed Nexus operation starts successfully but never completes before
its start-to-close timeout, which is shorter than its schedule-to-close timeout.
The result fails with a Nexus operation failure that retains the operation token
and has an immediate start-to-close timeout cause.
The history contains scheduled, started, and timed-out events.

The backing workflow has a longer execution timeout and is canceled during cleanup,
so a backing workflow timeout cannot be mistaken for a Nexus operation timeout.
Requires Temporal Server 1.31.0 or later.
