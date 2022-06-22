# Child workflows throw on start

Attempting to start a Child Workflow may throw an error.

This feature: 

- starts two Child Workflows with the same Workflow Id, the second one with `REJECT_DUPLICATE` Workflow Id Reuse Policy
- verifies that the second start command throws an already-started error

# Detailed spec

TODO