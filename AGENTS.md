# Repository Guidelines

## Project Structure & Module Organization

NickPit is a Go 1.25 CLI for LLM-assisted code review. The main binary lives in `cmd/nickpit`; helper generators live in `cmd/nickpit-config-example` and `cmd/nickpit-workflow-example`. Core packages are under `internal/`: review orchestration in `internal/review`, LLM client code in `internal/llm`, SCM adapters in `internal/scm`, retrieval tools in `internal/retrieval`, and shared API types in `internal/model`. Prompt templates live in `prompts/`, style guides in `prompts/styleguides/`, mappings in `mappings/`, built-in workflow specs in `workflows/`, and test fixtures/golden data in `testdata/`. Static assets are in `assets/`. See `CODE_STRUCTURE.md` for a file-by-file code map.

## Build, Test, and Development Commands

- `make build`: run generators, then build `./bin/nickpit`.
- `make debug`: build debug binary with optimizations disabled.
- `make generate`: regenerate checked-in config/workflow artifacts.
- `make test`: run `go test ./...`.
- `make race`: run tests with the race detector.
- `make lint`: run `golangci-lint run ./...`.
- `make vet`: run `go vet ./...`.
- `make fmt`: run `gofmt -w ./cmd ./internal`.

For local manual use, run examples such as `./bin/nickpit git uncommitted` or `./bin/nickpit git branch --show-progress` after building.

## Coding Style & Naming Conventions

Use standard Go formatting via `gofmt`; do not hand-align or introduce custom formatting. Keep packages focused and internal by default. Use clear Go names (`ReviewResult`, `runClusterMergeAgents`) and table-driven tests for rule-heavy behavior. Prefer structured parsers and typed models over ad hoc string parsing when data already has a schema. Prompt templates should stay concise and explicit; update matching tests when changing output shape or agent instructions.

## Terminal Colour Palette

`tools/print_colors.sh` is the reference sheet for every colour and rendering rule of the terminal UI: the stage column and message palette (`internal/logging/progress.go`), the live dashboard header, wordmark gradients, agent progress bars and findings footer (`internal/logging/live.go`), the reasoning block (`internal/logging/reasoning_renderer.go`), and the review-output badges (`internal/output/badge.go`, `internal/output/terminal.go`).

Keep it in sync: whenever you add, remove, or change a colour, an SGR code, or the way one of those elements is drawn, update the script in the same commit so running it still shows what the code actually emits. The script reimplements the drawing logic (bar fill and fraction rules, gradient sampling, badge padding) rather than importing it, so behaviour changes need mirroring too. Verify with `shellcheck tools/print_colors.sh` and by running `tools/print_colors.sh` in a truecolor terminal.

When picking new colours, keep them distinguishable: every stage colour sits at CIELAB L\* ≥ 64 so it stays readable on a dark background, and no two are closer than roughly ΔE\*ab 25.

## Testing Guidelines

Tests use Go's standard `testing` package. Place tests beside code as `*_test.go`; name cases like `TestClusterMergeCrossFileRootCauseRouteToLLM`. Run `make test` before submitting. Use focused package tests during development, for example `go test ./internal/review`. For generated files, run `make generate` and include resulting changes.

## Commit & Pull Request Guidelines

History follows Conventional Commits: `feat(review): ...`, `fix(llm): ...`, `docs(prompts): ...`. Keep subject lines imperative and scoped. Pull requests should describe behavior changes, mention config or workflow implications, include test results, and note any prompt/schema changes because they can affect replay logs and JSON compatibility.

## Security & Configuration Tips

Do not commit API keys or local `.nickpit.yaml` secrets. Prefer `NICKPIT_*` environment variables for SCM and model-provider tokens. Logs may contain prompts, diffs, and model output; treat files under `logs/` and generated review reports as sensitive.
