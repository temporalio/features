# Workflow Queries: Unexpected return type
Upon providing the name of a query handler which does not exist (yet), the SDK should reject that
query.


# Detailed spec
Issue a query with an unregistered type name, expect an error.