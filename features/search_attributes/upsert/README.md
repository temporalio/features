# Upserting search attributes

Search Attributes can be upserted from a Workflow.

This feature starts a Workflow that upserts these Search Attributes:

```ts
{
  CustomIntField: [123],
  CustomBoolField: [true],
}
```

And then these:

```ts
{
  CustomIntField: [],
  CustomKeywordField: ['durable code'],
  CustomTextField: ['is useful'],
  CustomDatetimeField: [date],
  CustomDoubleField: [3.14],
}
```

The Workflow Description and Workflow Info should have:

```ts
{
  CustomBoolField: [true],
  CustomIntField: [],
  CustomKeywordField: ['durable code'],
  CustomTextField: ['is useful'],
  CustomDatetimeField: [date],
  CustomDoubleField: [3.14],
}
```

The Workflow Description should also have a `BinaryChecksums` Search Attribute.

# Detailed spec

`WorkflowInfo.searchAttributes`: 

- is originally set with `WorkflowExecutionStartedEvent.attributes.searchAttributes`
- is updated whenever upsert is called
- is not updated with `BinaryChecksums`

TODO?