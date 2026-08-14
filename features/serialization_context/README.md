# Serialization context

A `DataConverter`, `PayloadCodec` or `FailureConverter` can opt into receiving
the context a payload is being converted in, so that it can, for example, derive
an encryption key from the namespace or use the workflow ID as associated data.

The features in this directory share a `sercontext` helper per language, which
provides:

- a payload codec that stamps the signature of its serialization context onto
  every payload it encodes and refuses to decode a payload encoded under a
  different context, so any asymmetry between the encoding and the decoding side
  fails the feature wherever it happens
- a failure converter that records the signature of its serialization context in
  `Failure.source`

Each feature then asserts the exact signature recorded in history, which pins
down the context values themselves rather than only their symmetry. Contexts
that never reach history are asserted against the set of signatures the codec was
actually asked to convert with.

The signature format is per language, because the SDKs expose different context
fields. Signatures are only ever compared within a single run, so they do not
need to agree across languages.

## History replay

Go and Java disable the harness history check. The replayer runs histories under
a placeholder namespace and workflow ID, so payloads recorded by a real execution
can never decode under a context derived from them.

## Language notes

- **Go** — `local_activity_payloads` fails: the SDK encodes the local activity
  result with the plain worker converter and decodes it with the workflow
  context. See that feature's README.
- **Python** — the workflow side of an activity context only carries an activity
  ID when the workflow sets one explicitly, so the features that schedule
  activities pass an explicit `activity_id`.
- **TypeScript** — no `local_activity_payloads`: the SDK has no local
  activities. The activity context carries no workflow or activity type.
- **Java** — the activity context carries no activity ID.
