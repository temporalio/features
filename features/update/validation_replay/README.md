# Replay does not perform update validation

On replay, validation is skipped so that changes to the validation
implementation do not affect the correctness of replay.

# Detailed spec

Validation must not run during replay so that if a user changes the
implementation of a validation function to be stronger (i.e. reject more cases),
that does not then reject updates under replay that had previously been
accepted.
