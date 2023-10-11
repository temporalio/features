# Signal a child workflow

A signal can be sent from within a workflow to a child workflow.

# Detailed spec

- Start a child workflow that does not terminate until a signal is sent.
- Use its handle to send a signal.
- Confirm that the signal had its intended effect within the child workflow.
