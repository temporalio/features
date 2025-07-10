# Repository Guidelines

This repository hosts Temporal SDK feature examples in multiple languages. Follow these rules when modifying code or adding new features.

## Running and Building
- Build the CLI with `go build -o temporal-features` (or use `go run .`). It requires Go, JDK 11+, Node 16+, uv, .NET 7+, and PHP 8.1+ to run the features.
- Run features with `temporal-features run --lang LANG [--version VERSION] [PATTERN...]`. Use `temporal-features prepare` to prebuild a project, and `build-image` to create docker images.
- History files live under `features/<path>/history/history.<lang>.<version>.json`. Generate history with `--generate-history` only when developing a new feature or when a version intentionally changes history.

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
- **TypeScript**: Format with Prettier and lint with ESLint using `npm run format` and `npm run lint`. Prettier is configured for single quotes and a 120‑character line width.
- **Python**: Install tools with `uv tool install poethepoet` and `uv sync`. Format with `poe format` and lint with `poe lint` which runs Ruff and mypy.
- **Go**: Run `go fmt ./...` before committing.
- **Java**: Apply Google Java Format with `./gradlew spotlessApply`.
- **C#**: Builds treat warnings as errors. `.editorconfig` disables warning `CS1998`.

These conventions help keep the examples consistent across languages and ensure CI passes.
