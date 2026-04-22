<p align="center">
  <img src="nickpit.png" alt="NickPit logo" width="320">
</p>

# NickPit

NickPit is a CLI for LLM-assisted code review across local git changes, GitHub pull requests, and GitLab merge requests.  
It uses a normalized review context, a provider-compatible chat completions client, and optional tool-driven retrieval rounds for additional code context.

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

The built-in `default` profile targets OpenRouter at `https://openrouter.ai/api/v1`. You must specify a model explicitly, and unless you set `api_key` in config, NickPit expects the API key in `OPENROUTER_API_KEY`.

## Usage

```bash
# Review current branch in current directory against default branch
nickpit local branch

# Review current branch in specified directory against default branch
nickpit local branch --workdir /path/to/dir

# Review feature/my-branch against main in specified directory
nickpit local branch --base main --head feature/my-branch --workdir /path/to/dir

# Review specific commit range in current directory
nickpit local commits --from HEAD~3 --to HEAD

# Review uncommitted changes in current directory
nickpit local uncommitted

# Review PR in GitHub
nickpit github pr --repo owner/repo --id 123
nickpit github pr --repo owner/repo --id 123 --local-repo ~/src/repo

# Review MR in GitLab
nickpit gitlab mr --repo group/project --id 456
```

### Progress

Append `--show-progress` to print review details and tool calls on stderr.

### Debug

Append `--verbose` or `--debug` to print step-by-step execution details to stderr, including prompt rendering and raw LLM request/response payloads.


### Output Schema Mode

By default, NickPit includes the expected JSON schema directly in the system prompt.
Use `--use-json-schema` to send the review schema via the API `response_format` field for providers that support JSON schema constrained output. The same setting can be stored in config as `use_json_schema: true`.

NickPit can lets the model request additional file context during review. Control the maximum number of tool-call iterations with `--max-tool-calls` or `max_tool_calls` in config. `0` means unlimited, which is the default. You can also stop tool use after too many duplicate requests with `--max-duplicate-tool-calls` or `max_duplicate_tool_calls`; the default is `5`.

### Filtering by Priority

Review output filtering uses `--priority-threshold` with `p0` through `p3`, where `p0` is highest priority and `p3` is lowest.

### Inspect Commands

The `inspect` command is a standalone retrieval command tree for using retrieval without review.

```bash
nickpit inspect file --path internal/review/engine.go
nickpit inspect file --path internal/review/engine.go --line-start 1 --line-end 80
nickpit inspect list --path internal/review
nickpit inspect search --path internal/review --query inspect_file
nickpit inspect callers --symbol Run --depth 2
nickpit inspect callers --path internal/review --symbol Run --depth 2
nickpit inspect callers --path internal/review/engine.go --symbol Run --depth 2
nickpit inspect callees --path internal/review/engine.go --symbol Run --depth 3
nickpit inspect search --path internal/review --query inspect_file --context-lines 3 --max-results 5 --json
nickpit inspect callers --path internal/review/engine.go --symbol Run --depth 2 --json
```

Retrieval supports `go`, `python`, and `nodejs` source files. `inspect file`, `inspect list`, and `inspect search` work generically across text files, while `inspect callers` and `inspect callees` use language-aware symbol and call-hierarchy analysis.

## Notes

- The CLI expects an OpenAI-compatible `/chat/completions` endpoint.
- Remote reviews clone the requested PR/MR head into a temporary checkout when retrieval needs local files.
- Use `--local-repo` to reuse an existing clone; NickPit creates a temporary worktree at the requested revision instead of cloning again.
