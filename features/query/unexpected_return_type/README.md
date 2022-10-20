# Workflow Queries: Unexpected return type
Upon returning a type different from the one expected by the query caller, the client should
return some kind of deserialization error if the SDK has sufficient type information to do so.


# Detailed spec
Query a handler which returns a string, but try to interpret it as an int.