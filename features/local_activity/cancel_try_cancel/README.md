# Local Activity cancellation - Try Cancel mode

Local Activities may be cancelled in three different ways, this feature spec covers the
Try Cancel mode.

Each feature workflow in this folder should start a local activity and cancel it
using the Try Cancel mode. The implementation should demonstrate that the activity
receives a cancel request after the workflow has issued it, but the workflow
immediately should proceed with the activity result being cancelled.

# Detailed spec

- When the SDK requests cancel of the activity, it does not send a command to server
- The workflow immediately resolves the activity with its result being cancelled
- The worker will notify the activity of cancellation
- The activity may ignore the cancellation request if it explicitly chooses to
- Whatever the activity result is, it's insignificant as it is ignored by the SDK
