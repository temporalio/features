# Workflow Queries: Unexpected Arguments
A query which is issued with arguments that do not type match against the query handler should be
rejected according to the best efforts of the language. Some languages do not have the necessary
information to reject based on type. Rejection based on number of arguments is more possible in
all languages.


# Detailed spec
Issue queries with:
* One argument, wrong type
* An extra argument when only one is expected
* No argument when one is expected

And verify they are rejected if the langauge supports such rejection