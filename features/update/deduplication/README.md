# Deduplication by update ID

When multiple updates targeting the same workflow use the same update identifier, only one update is performed.

# Detailed spec

- Create a `Count` workflow initialized to zero, and with an update handler that increments the count by one, and returns its value.
- Update the workflow multiple times with the same `UpdateID`.
- Verify that the final value of count is one.
