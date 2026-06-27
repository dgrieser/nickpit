# Repository Guidelines

## Project Structure & Module Organization

NickPit is a Go 1.25 CLI for LLM-assisted code review. The main binary lives in `cmd/nickpit`; helper generators live in `cmd/nickpit-config-example` and `cmd/nickpit-workflow-example`. Core packages are under `internal/`: review orchestration in `internal/review`, LLM client code in `internal/llm`, SCM adapters in `internal/scm`, retrieval tools in `internal/retrieval`, and shared API types in `internal/model`. Prompt templates live in `prompts/`, style guides in `prompts/styleguides/`, mappings in `mappings/`, built-in workflow specs in `workflows/`, and test fixtures/golden data in `testdata/`. Static assets are in `assets/`.

## Build, Test, and Development Commands

- `make build`: run generators, then build `./bin/nickpit`.
- `make debug`: build debug binary with optimizations disabled.
- `make generate`: regenerate checked-in config/workflow artifacts.
- `make test`: run `go test ./...`.
- `make race`: run tests with the race detector.
- `make lint`: run `golangci-lint run ./...`.
- `make vet`: run `go vet ./...`.
- `make fmt`: run `gofmt -w ./cmd ./internal`.

For local manual use, run examples such as `./bin/nickpit local uncommitted` or `./bin/nickpit local branch --show-progress` after building.

## Coding Style & Naming Conventions

Use standard Go formatting via `gofmt`; do not hand-align or introduce custom formatting. Keep packages focused and internal by default. Use clear Go names (`ReviewResult`, `runClusterMergeAgents`) and table-driven tests for rule-heavy behavior. Prefer structured parsers and typed models over ad hoc string parsing when data already has a schema. Prompt templates should stay concise and explicit; update matching tests when changing output shape or agent instructions.

## Testing Guidelines

Tests use Go's standard `testing` package. Place tests beside code as `*_test.go`; name cases like `TestClusterMergeCrossFileRootCauseRouteToLLM`. Run `make test` before submitting. Use focused package tests during development, for example `go test ./internal/review`. For generated files, run `make generate` and include resulting changes.

## Commit & Pull Request Guidelines

History follows Conventional Commits: `feat(review): ...`, `fix(llm): ...`, `docs(prompts): ...`. Keep subject lines imperative and scoped. Pull requests should describe behavior changes, mention config or workflow implications, include test results, and note any prompt/schema changes because they can affect replay logs and JSON compatibility.

## Security & Configuration Tips

Do not commit API keys or local `.nickpit.yaml` secrets. Prefer `NICKPIT_*` environment variables for SCM and model-provider tokens. Logs may contain prompts, diffs, and model output; treat files under `logs/` and generated review reports as sensitive.
