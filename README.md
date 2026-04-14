# NickPit

NickPit is a Go CLI for LLM-assisted code review across local git changes, GitHub pull requests, and GitLab merge requests. It uses a normalized review context, a provider-compatible chat completions client, and optional follow-up retrieval rounds for additional code context.

## Features

- Local review modes for uncommitted changes, commit ranges, and branch diffs
- GitHub PR and GitLab MR review via direct REST clients
- OpenAI-compatible chat completions client
- Structured JSON findings with severity filtering
- Retrieval commands for files, slices, symbols, callers, and callees
- Terminal and JSON output modes

## Installation

```bash
go build ./cmd/llm-review
```

## Configuration

NickPit loads configuration in this order:

1. Built-in defaults
2. YAML config file from `--config` or `.llm-review.yaml`
3. Environment variables
4. CLI flags

See [.llm-review.yaml.example](/home/grieser/workspace/dgrieser/nickpit/.llm-review.yaml.example) for a complete example.

## Usage

```bash
llm-review local uncommitted
llm-review local commits --from HEAD~3 --to HEAD
llm-review local branch --base main --head feature/my-branch

llm-review github pr --repo owner/repo --pr 123
llm-review gitlab mr --project group/project --mr 456

llm-review retrieve file --path internal/review/engine.go
llm-review retrieve lines --path internal/review/engine.go --start 1 --end 80
llm-review retrieve callers --symbol Run --depth 2
llm-review retrieve function-stack --symbol Run --direction callees --depth 3
```

## Development

```bash
make fmt
make test
make build
```

## Notes

- The CLI expects an OpenAI-compatible `/chat/completions` endpoint.
- Retrieval for remote reviews assumes the repository is available locally when follow-up file access is needed.
- SARIF output is stubbed and returns a not-implemented error.
