## SDK Features

This repository contains snippets for many Temporal SDK features written in different Temporal SDK languages. This also
contains a runner and language-specific harnesses to confirm feature behavior across versions.

These SDK features serve several purposes:

* Ensure parity across SDKs by having same-feature snippets adjacent to one another
* Confirm feature behavior across SDK versions
* Confirm history across SDK versions
* Document features in different SDKs
* Easy-to-use environment for writing quick workflows in all languages/versions

## Building

With latest [Go](https://golang.org/) installed, run:

    go build

## Running

Prerequisites:

* [Go](https://golang.org/) 1.17+
* [JDK](https://adoptium.net/?variant=openjdk11&jvmVariant=hotspot) 11+
* [Node](https://nodejs.org) 16+

Command:

    sdk-features run --lang LANG [--version VERSION] [PATTERN...]

Note, `go run .` can be used in place of `go build` + `sdk-features` to save on the build step.

`LANG` can be `go`, `java`, or `ts`. `VERSION` is per SDK and if left off, uses the latest version set for the language
in this repository.

`PATTERN` must match either the features relative directory _or_ the relative directory + `/feature.<ext>` via
[Go path match rules](https://pkg.go.dev/path#Match) which notably does not include recursive depth matching. If
`PATTERN` arguments are not present, the default is to run all features.

Several other options are available, some of which are described below. Run `sdk-features run --help` to see all
options.

**NOTE** There is [currently a bug](https://github.com/temporalio/temporal/issues/2207) with Temporalite that causes
history checks to fail. Until it is fixed, the external server as shown below must be used.

### External Server and Namespace

By default, a [temporalite](https://github.com/DataDog/temporalite) is dynamically started at runtime to handle all
feature runs and a namespace is dynamically generated. To not start the embedded server, use `--server` to specify the
address of a server to use. Similarly, to not use a dynamic namespace (that may not be registered on the external
server), use `--namespace`.

### History Checking

History files are present at `features/<path/to/feature>/history/history.<lang>.<version>.json`. By default, three
history checks are performed:

1. Specific languages use their replayers to replay the just-executed workflow's history to confirm it works.
2. Specific languages use their replayers to replay all history files with versions <= the current version to confirm it
   works.
3. The primary runner scrubs the history of the just-executed workflow of all execution-dependent values. Then it
   compares the exact events to all similarly-scrubbed history files.

Currently there are not ways for features to opt out of specific history checks. To opt out of all history checking for
a specific run, use `--no-history-check`.

## Writing Features

Developers can write workflows and activities that represent a SDK "feature". These are organized into directories under
the `features/` directory.

In addition to code for the feature, there are configuration settings that can be set in `.config.json`. The possible
settings are:

* `go`
  * `minVersion` - Minimum version in Go this feature should be run in. The feature will be skipped in older versions.

There are also files in the `history/` subdirectory which contain history files used during run. See the
"History Checking" and "Generating History" sections for more info.

### Best Practices

* Try to only demonstrate/test one feature per feature directory.
  * Code should be kept as short and clear as possible.
  * No need to over-assert on a bunch of values, just confirm that the feature does what is expected via its output.
* A Go feature should be in `feature.go`.
  * For incompatible versions, different files like `feature_pre1.11.0.go` can be present using build tags
* A Java feature in `feature.java`.
* A TypeScript feature should be in `feature.ts` for all non-workflow code and `feature.workflow.ts` for all workflow
  code.
* Add a README.md to each feature directory.
  * README should have a title summarizing the feature (only first letter needs to be in title case), then a short
    paragraph explaining the feature and its purpose, and then optionally another paragraph explaining details of the
    specific code steps.
  * Other sections can also explain decisions made in language specific ways or other details about versions/approaches.
  * Feel free to add links and more text as necessary.
* Verification/regression feature directories for bugs should be under `features/bugs/<lang>`.
  * Ideally the checking of the result has a version condition that shows in earlier versions it should fail and in
    newer versions it should succeed.
* The more languages per non-bug feature, the better. Try not to create non-bug features that use specific language
  constructs unless that's the purpose of the feature.
* Refactor liberally to create shortcuts and better harnesses for making features easy to read and write.
  * This is not a library used by anyone, there are no backwards compatibility concerns. If one feature uses something
    that has value to another, extract and put in helper/harness and have both use it.
* History should be generated for each feature on the earliest version the feature is expected to work at without
  history changes.

#### Generating History

To generate history, run the same test (see the "Running" section) for the version to generate at, but use the
`--generate-history` option. When generating history, only one test can be specified and the version of the SDK must be
specified. Any existing history for that feature, language, and version will be overwritten.

History generation should only be needed when first developing a feature or when a version intentionally introduces an
incompatibility. Otherwise, history files should remain checked in and not regenerated.

## TODO

* Add support for replaying testing of all versions _inside_ each SDKs harness as part of the run
* Add TypeScript support
  * The main support is present, but there are outstanding questions on what constitutes a "version" since really
    TypeScript has many versions
* Add many more feature workflows
* Document how to use this framework to easily write and test features even when not committing
* Log swallowing and concurrent execution
* Investigate support for changing runtime versions (i.e. Go, Java, and Node versions)
* Investigate support for changing server versions
* CI support
  * Support using a commit hash and alternative git location for an SDK to run against
  * Decide whether the matrix of different SDK versions and such is really part of this repo or part of CI tooling