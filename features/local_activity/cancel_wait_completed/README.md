# Local Activity cancellation - Wait Cancel mode

Local Activities may be cancelled in three different ways, this feature spec covers the
Wait Cancel mode.

Each feature workflow in this folder should start a local activity and cancel it
using the Wait Cancel mode. The implementation should demonstrate that the activity
receives the cancel request, and that the workflow does not resolve the activity
until the activity acknowledges cancellation. The activity implementation may choose
to ignore the cancel or not.

# Detailed spec

- When the SDK requests cancel of the activity, it does not send a command to server
- Activity completes, maybe saying it cancelled, maybe saying it completed any other way
- Workflow does not resolve the activity until it is activated with the activity result
