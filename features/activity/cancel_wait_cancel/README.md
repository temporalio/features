# Activity cancellation - Wait Cancel mode
Activities may be cancelled in three different ways, this feature spec covers the
Wait Cancel mode.

Each feature workflow in this folder should start an activity and cancel it
using the Wait Cancel mode. The implementation should demonstrate that the activity
receives the cancel request, and that the workflow does not resolve the activity
until the activity handles the cancel task. The activity implementation may choose
to ignore the cancel or not.

# Detailed spec
* When the SDK requests cancel of the activity, it sends a command to server
* Server writes an activity cancel requested event
* Server will notify the activity cancellation has been requested via a response
  to activity heartbeating
* Activity completes, maybe saying it cancelled, maybe saying it completed any other way
* Workflow does not resolve the activity until it receives a WFT with the activity result event