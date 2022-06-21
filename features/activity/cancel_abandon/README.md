# Activity cancellation - Abandon mode
Activities may be cancelled in three different ways, this feature spec covers the
Abandon mode.

Each feature workflow in this folder should start an activity and cancel it
using the abandon mode. The implementation should demonstrate that the activity
keeps running and receives no cancel notification after the workflow requests one.
The workflow code should immmediately unblock the activity with its result being
cancelled

# Detailed spec
* When the SDK requests cancel of an abandon mode activity, the workflow code
  immediately unblocks, the activity result being cancelled.
* Nothing is sent to the server, the activity worker never is notified of the
  cancellation attempt