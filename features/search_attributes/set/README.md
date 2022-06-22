# Setting search attributes

Search Attributes can be set on Workflow start.

This feature starts a Workflow with these Search Attributes:

```ts
{ 
  CustomKeywordField: ['test-value'],
  CustomIntField: [1, 2],
  CustomBoolField: [true],
  CustomDatetimeField: [date],
}
```

The Search Attributes in Workflow Info should match. Those in Workflow Description should match, with an added `BinaryChecksums` Search Attribute.