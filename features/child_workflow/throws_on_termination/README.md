# Child workflows throw on terminate

Executing a Child Workflow throws if the Child is terminated.

This feature: 

- starts a blocked Child Workflow
- queries the Parent for the Child's Workflow Id and Run Id
- terminates the Child
- verifies that `child.result()` throws