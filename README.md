<p align="center">
  <img src="nickpit.png" alt="NickPit logo" width="320">
</p>

# NickPit

NickPit is a CLI for LLM-assisted code review across local git changes, GitHub pull requests, and GitLab merge requests. It uses a normalized review context, a provider-compatible chat completions client, and optional tool-driven retrieval rounds for additional code context.

## Features

- Local review modes for uncommitted changes, commit ranges, and branch diffs
- GitHub PR and GitLab MR review via direct REST clients
- OpenAI-compatible chat completions client
- Structured JSON findings with priority filtering and overall verdicts
- Prompt-embedded JSON output schema by default, with optional API-enforced JSON schema mode
- Retrieval commands for files, slices, symbols, callers, and callees
- Terminal and JSON output modes

## Installation

```bash
make build
sudo make install
```

To install somewhere other than `/usr/local/bin`, override `PREFIX`:

```bash
make install PREFIX=$HOME/.local
```

## Configuration

NickPit loads configuration in this order:

1. Built-in defaults
2. YAML config file from `--config` or `.nickpit.yaml`
3. Environment variables
4. CLI flags

See [.nickpit.yaml.example](.nickpit.yaml.example) for a complete example.

## Usage

```bash
nickpit local uncommitted
nickpit local commits --from HEAD~3 --to HEAD
nickpit local branch --base main --head feature/my-branch

nickpit github pr --repo owner/repo --id 123
nickpit github pr --repo owner/repo --id 123 --local-repo ~/src/repo
nickpit gitlab mr --repo group/project --id 456
nickpit local uncommitted --verbose
nickpit github pr --repo owner/repo --pr 123 --debug

```

### Debug

Append `--verbose` or `--debug` to print step-by-step execution details to stderr, including prompt rendering and raw LLM request/response payloads.


### Output Schema Mode

By default, NickPit includes the expected JSON schema directly in the system prompt.

NickPit does not send `response_format` unless `--use-json-schema` is enabled.

Use `--use-json-schema` to send the review schema via the API `response_format` field for providers that support JSON schema constrained output. The same setting can be stored in config as `use_json_schema: true`.

NickPit can also let the model request additional file context during review. Control the maximum number of tool-call iterations with `--tool-rounds` or `default_tool_rounds` in config.

### Temperature

NickPit does not send a `temperature` parameter unless it is explicitly configured in the active profile, for example `temperature: 0.2`.

NickPit also does not send `max_tokens` unless it is explicitly configured in the active profile, for example `max_tokens: 4096`.

### Filtering by Priority

Review output filtering uses `--priority-threshold` with `p0` through `p3`, where `p0` is highest priority and `p3` is lowest.

### Inspect Commands

The `inspect` command is a standalone retrieval command tree for using retrieval without review.

```bash
nickpit inspect file --path internal/review/engine.go
nickpit inspect lines --path internal/review/engine.go --start 1 --end 80
nickpit inspect callers --path internal/review/engine.go --symbol Run --depth 2
nickpit inspect callees --path internal/review/engine.go --symbol Run --depth 3
nickpit inspect callers --path internal/review/engine.go --symbol Run --depth 2 --json
```

`inspect callers` and `inspect callees` now include each mentioned function's full source in JSON output and print the source inline in terminal output.

## Notes

- Currently the default LLM endpoint is OpenRouter
  - `https://openrouter.ai/api/v1`
  - `openai/gpt-oss-120b:free`
  - API key env var `OPENROUTER_API_KEY`
- The CLI expects an OpenAI-compatible `/chat/completions` endpoint.
- Remote reviews clone the requested PR/MR head into a temporary checkout when retrieval needs local files.
- Use `--local-repo` to reuse an existing clone; NickPit creates a temporary worktree at the requested revision instead of cloning again.
