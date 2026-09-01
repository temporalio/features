# Oversized query result with external storage

An oversized query result succeeds when external storage is configured. The
SDK must offload the result before applying the server payload-size limit.

The query returns 3 MiB, above the server's default 2 MiB error limit. The
checker verifies the value and confirms that storage and retrieval occurred.

Go, Python, and TypeScript currently expose functional external storage.
