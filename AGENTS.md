# Repository Guidelines

This repository hosts Temporal SDK feature examples in multiple languages. Follow these rules when modifying code or adding new features.

## Running and Building
- Build the CLI with `go build -o temporal-features` (or use `go run .`). It requires Go, JDK 11+, Node 16+, uv, .NET 7+, and PHP 8.1+ to run the features.
- Run features with `temporal-features run --lang LANG [--version VERSION] [PATTERN...]`. Use `temporal-features prepare` to prebuild a project, and `build-image` to create docker images.
- History files live under `features/<path>/history/history.<lang>.<version>.json`. Generate history with `--generate-history` only when developing a new feature or when a version intentionally changes history.
- Always test changes to a given language by running `go run . run --lang <language>`. If adding a new feature test, test that feature specifically by running `go run . run --lang <language> <feature_directory>` where `<feature_directory>` is the path to the feature directory inside of `features/`.

## Writing Features
- Each feature lives under `features/` in its own directory. Keep code short and clear and avoid unnecessary assertions.
- File naming conventions:
  - Go: `feature.go` with optional version-specific files like `feature_pre1.11.0.go`.
  - Java: `feature.java`
  - TypeScript: `feature.ts` (workflow and non-workflow code may share this file).
  - Python: `feature.py`
- Add a `README.md` to every feature directory with a short title and explanation.
- Bug regression tests belong under `features/bugs/<lang>`. Favor including as many languages as possible for non-bug features and feel free to extract helpers since backward compatibility is not a concern.

## Formatting and Linting
- **TypeScript**: Format with Prettier and lint with ESLint using `npm run format` and `npm run lint`, you must ensure type checking passes with `npm run build`.
- **Python**: Install tools with `uv tool install poethepoet` and `uv sync`. Format with `poe format` and lint with `poe lint` which runs Ruff and mypy.
- **Go**: Run `go fmt ./...` before committing.
- **Java**: Ensure the project builds with `./gradlew assemble` and autoformat with `./gradlew spotlessApply`.
- **C#**: Builds treat warnings as errors. `.editorconfig` disables warning `CS1998`.

These conventions help keep the examples consistent across languages and ensure CI passes.

## Understanding SDK capabilities
Agents should use the docs.temporal.io website, as well as github.com to learn about the capabilities and the API specifics of each langauge SDK.

* Go - https://github.com/temporalio/sdk-go and https://docs.temporal.io/develop/go/
* Java - https://github.com/temporalio/sdk-java and https://docs.temporal.io/develop/java/
* TypeScript - https://github.com/temporalio/sdk-typescript and https://docs.temporal.io/develop/typescript/
* Python - https://github.com/temporalio/sdk-python and https://docs.temporal.io/develop/python/
* .NET / C# - https://github.com/temporalio/sdk-dotnet and https://docs.temporal.io/develop/dotnet/
* Ruby - https://github.com/temporalio/sdk-ruby and https://docs.temporal.io/develop/ruby/
* PHP - https://github.com/temporalio/sdk-php and https://docs.temporal.io/develop/php/
