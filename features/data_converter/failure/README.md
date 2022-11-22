# Failure converter

When using the default failure converter `stack_trace` and `message` can be encoded in the `EncodedAttributes` field.

Steps:

- run a workflow that calls an activity that fails with an error and has a `cause` sub error
- get result payload of ActivityTaskFailed event from workflow history
- verify the message and stack trace are encoded on the main error and sub error. 
Failure.message should be "Encoded failure" and Failure.stack_trace should be "". 

Note: Typescript lets us modify the stack trace of the error so we can test more than other languages.
