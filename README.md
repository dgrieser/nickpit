<p align="center">
  <img src="nickpit.png" alt="NickPit logo" width="320">
</p>

# NickPit

NickPit is a Go CLI for LLM-assisted code review across local git changes, GitHub pull requests, and GitLab merge requests. It uses a normalized review context, a provider-compatible chat completions client, and optional follow-up retrieval rounds for additional code context.

## Features

- Local review modes for uncommitted changes, commit ranges, and branch diffs
- GitHub PR and GitLab MR review via direct REST clients
- OpenAI-compatible chat completions client
- Structured JSON findings with priority filtering and overall verdicts
- Retrieval commands for files, slices, symbols, callers, and callees
- Terminal and JSON output modes

## Installation

```bash
go build ./cmd/nickpit
```

## Configuration

NickPit loads configuration in this order:

1. Built-in defaults
2. YAML config file from `--config` or `.nickpit.yaml`
3. Environment variables
4. CLI flags

See [.nickpit.yaml.example](/home/grieser/workspace/dgrieser/nickpit/.nickpit.yaml.example) for a complete example.

## Usage

```bash
nickpit local uncommitted
nickpit local commits --from HEAD~3 --to HEAD
nickpit local branch --base main --head feature/my-branch

nickpit github pr --repo owner/repo --pr 123
nickpit github pr --repo owner/repo --pr 123 --local-repo ~/src/repo
nickpit gitlab mr --project group/project --mr 456
nickpit local uncommitted --verbose
nickpit github pr --repo owner/repo --pr 123 --debug

nickpit inspect file --path internal/review/engine.go
nickpit inspect lines --path internal/review/engine.go --start 1 --end 80
nickpit inspect callers --path internal/review/engine.go --symbol Run --depth 2
nickpit inspect callees --path internal/review/engine.go --symbol Run --depth 3
```

`--verbose` and `--debug` are aliases. They print step-by-step execution details to stderr, prefixed with `+ ` in grey when ANSI colors are enabled, including prompt rendering and raw LLM request/response payloads.

Prompt overrides are split by role:

- `--review-system-prompt-file`
- `--review-user-prompt-file`
- `--followup-system-prompt-file`
- `--followup-user-prompt-file`

Review output filtering uses `--priority-threshold` with `p0` through `p3`, where `p0` is highest priority and `p3` is lowest.

`inspect` is the standalone retrieval command tree for using retrieval without review.

## Development

```bash
make fmt
make test
make build
```

## Notes

- The default LLM endpoint is OpenRouter (`https://openrouter.ai/api/v1`) using `openai/gpt-oss-120b:free`.
- The default API key env var is `OPENROUTER_API_KEY`.
- The CLI expects an OpenAI-compatible `/chat/completions` endpoint.
- Remote reviews clone the requested PR/MR head into a temporary checkout when retrieval needs local files.
- Use `--local-repo` or profile `local_repo` to reuse an existing clone; NickPit creates a temporary worktree at the requested revision instead of cloning again.
- SARIF output is stubbed and returns a not-implemented error.
