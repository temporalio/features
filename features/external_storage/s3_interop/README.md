# External storage S3 interoperability

Payloads larger than the external storage threshold are stored in S3 by the `aws.s3driver` driver, and the
workflow history contains only references to them. All SDKs must write the same references and objects, and
each SDK must read the references and objects that another SDK wrote.

The feature:

* Starts a workflow with a 300 KiB string. The client stores the input, and the worker stores the result.
* Signals the workflow with a reference to an object that the Go SDK wrote. The workflow fails if it does not
  read the value `hello` from the object.
* Checks that the client gets the input back as the result.
* Checks that the history contains references with the same driver name, bucket, key and SHA-256 hash in all
  SDKs, and that the stored object has the expected hash.

# Configuration

The feature is skipped if `TEMPORAL_FEATURES_S3_BUCKET` is not set. It uses these environment variables:

* `TEMPORAL_FEATURES_S3_BUCKET` - the bucket. It must exist.
* `TEMPORAL_FEATURES_S3_ENDPOINT` - an optional S3-compatible endpoint, for example MinIO. Path-style
  addressing is used with it.
* `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` - the credentials and the region.

The PHP worker runs in RoadRunner. The PHP harness passes the bucket and the endpoint to the
`temporal.external_storage` section of RoadRunner, so the RoadRunner binary must support this section.
