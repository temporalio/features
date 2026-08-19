# Nexus sync operation fails

A workflow invokes a synchronous Nexus operation that raises an application failure and
inspects the resulting error.

# Detailed spec

- The sync operation returns an operation error in the failed state whose cause is an
  application error with a known type and message.
- The caller receives a Nexus operation error whose cause is that application error, with the
  original type and message preserved across the operation boundary.
- The caller handles the failure and completes successfully.
- A failed operation produces a failed event and never a completed one.
