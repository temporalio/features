# Timers

Timers serve to replace `sleep`-like APIs in workflows. Timers may be arbitrarily long
and are scheduled on the server.

# Detailed spec

* When workflow code calls a timer api, the next WFT completion will contain a 
  ScheduleTimer command with the duration
* Timers may be cancelled. When a timer is cancelled within a workflow, the SDK immediately
  resolves the timer as cancelled. The next WFT completion will contain a CancelTimer
  command
* When a timer has elapsed, server enters a TimerFired event into history, and when
  the worker receives the next WFT, the timer is resolved as fired
