# Basic schedule

Workflows can be scheduled using the scheduling client. Schedules can be listed, referenced, updated, etc.

Each feature:

* Creates a 2s schedule for a workflow with an argument
* After at least one has run, does a schedule update to change the arg
* Confirms a pre-change and post-change workflow run and latter has completed within 10s