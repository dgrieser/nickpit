<p align="center">
  <img src="assets/nickpit.png" alt="NickPit logo" width="320">
</p>

# NickPit

NickPit is a CLI for LLM-assisted code review across local git changes, GitHub pull requests, and GitLab merge requests.  
It uses a normalized review context, a provider-compatible chat completions client, and optional tool-driven retrieval rounds for additional code context.

## Features

- Local review modes for uncommitted changes, commit ranges, and branch diffs
- GitHub PR and GitLab MR review via direct REST clients
- OpenAI-compatible chat completions client
- Structured JSON findings with priority filtering and overall verdicts
- Default multi-agent review with context collection, specialist reviewers, merge, and verification stages
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

### Docker

Images are published to `ghcr.io/dgrieser/nickpit` (multi-arch: `amd64`, `arm64`,
`arm/v7`). The image is **rootless** and **distroless** — it runs as a non-root user
by default and supports an arbitrary runtime UID, so you can map it to your host user
to operate on mounted repositories and config.

```bash
# Review a host-mounted repo as your own user (rootless).
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e OPENROUTER_API_KEY -e NICKPIT_GITHUB_TOKEN -e NICKPIT_GITLAB_TOKEN \
  -v "$PWD:/work" -w /work \
  ghcr.io/dgrieser/nickpit:latest local branch

# Review a remote PR/MR (no mount needed); pass the SCM token via env.
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e OPENROUTER_API_KEY -e NICKPIT_GITHUB_TOKEN \
  ghcr.io/dgrieser/nickpit:latest github pr --repo owner/repo --id 123
```

Notes:
- `--user "$(id -u):$(id -g)"` makes the container read mounts and write temp files as
  your host user. The image trusts mounted repositories (`git safe.directory=*`), so git
  does not reject a repo owned by a different UID.
- Pass auth via env: `OPENROUTER_API_KEY`, plus `NICKPIT_GITHUB_TOKEN` /
  `NICKPIT_GITLAB_TOKEN` for remote reviews. `NICKPIT_GITLAB_BASE_URL` sets a custom
  GitLab API root. `GITHUB_TOKEN`, `GITLAB_TOKEN`, and `GITLAB_BASE_URL` also work, but
  the `NICKPIT_` names win when both are set. The bare `-e NAME` form forwards the value
  from your shell.
- Provide config by mounting `.nickpit.yaml` into `/work`, or with an absolute
  `--config /work/.nickpit.yaml`. When running as an arbitrary UID, prefer an absolute
  `--config` path over `~` expansion (the image `HOME` is not readable by a foreign UID).
- Clones/worktrees are written under `/tmp`. For large repos use `--tmpfs /tmp:rw,size=1g`.
- Only **HTTPS** clone URLs are supported in the container (use a token); SSH clone URLs
  are not, as no `ssh` client is bundled.

## Configuration

NickPit loads configuration in this order:

1. Built-in defaults
2. YAML config file from `--config` or `.nickpit.yaml`
3. Environment variables
4. CLI flags

Run `make generate` or `make build` to generate `.nickpit.yaml.example` from the built-in defaults.

The built-in `default` profile targets OpenRouter at `https://openrouter.ai/api/v1`. You must specify a model explicitly, and unless you set `api_key` in config, NickPit expects the API key in `OPENROUTER_API_KEY`.

Profiles can also define a cheaper/faster alias for workflow steps:

```yaml
profiles:
  default:
    model: primary-model
    reasoning_effort: high
    small_model: small-model
    small_reasoning_effort: low
```

`small_model` and `small_reasoning_effort` can also be set with `NICKPIT_SMALL_MODEL` / `NICKPIT_SMALL_REASONING_EFFORT` or `--small-model` / `--small-reasoning-effort`. In workflow step config, `model: "@small"` selects `small_model`; when `small_model` is unset it falls back to `model`, and when `small_reasoning_effort` is unset it falls back to `reasoning_effort`.

### Diff Filters

Profiles can filter changed files before review. Path and content values are Go regular expressions; path regexes match repo-relative paths, while content regexes match the full post-change file content.

```yaml
profiles:
  default:
    include_paths: ["\\.go$"]
    exclude_paths: ["\\.pb\\.go$", "(^|/)package-lock\\.json$"]
    include_content: ["(?m)^package "]
    exclude_content: ["(?m)Code generated .* DO NOT EDIT"]
```

Deleted files have no post-change content, so a non-empty `include_content` always drops them; `exclude_content` leaves them in. Path filters still apply to deletions.

The same filters can be set per run with repeatable flags such as `--include-path`, `--exclude-path`, `--include-content`, and `--exclude-content`.

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

# Review a GitHub PR and post the result back as a review (summary + one comment per finding)
nickpit github pr --repo owner/repo --id 123 --publish

# Review MR in GitLab
nickpit gitlab mr --repo group/project --id 456

# Review a GitLab MR and post the result back as comments (summary + one per finding)
nickpit gitlab mr --repo group/project --id 456 --publish
```

With `--publish`, findings whose lines are part of the diff are posted inline anchored to those lines; the rest fall back to general comments that include `file:line` after the priority badge and confidence line. On GitHub this is a single PR review (the summary as the review body, findings as inline review comments); on GitLab it is a summary note plus one inline discussion per finding. Hidden markers make re-runs idempotent (already-posted comments are skipped), and a publish failure is reported as a warning without failing the review.

### Progress

Append `--show-progress` to print review details and tool calls on stderr.

### Patch Summary

By default, the final overall explanation starts with an assumed summary of what the patch is intended to do. Use `--disable-patch-summary` or `disable_patch_summary: true` in the active profile to omit that summary from final output while still allowing internal agents to use context notes.

### Debug

Append `--verbose` or `--debug` to print step-by-step execution details to stderr, including prompt rendering and raw LLM request/response payloads.


### Output Schema Mode

By default, NickPit includes the expected JSON schema directly in the system prompt.
Use `--use-json-schema` to send the review schema via the API `response_format` field for providers that support JSON schema constrained output. The same setting can be stored in config as `use_json_schema: true`.

NickPit can lets the model request additional file context during review. Control the maximum number of tool-call iterations with `--max-tool-calls` or `max_tool_calls` in config. `0` means unlimited, which is the default. You can also stop tool use after too many duplicate requests with `--max-duplicate-tool-calls` or `max_duplicate_tool_calls`; the default is `5`. Invalid model output is retried with `--max-output-retries` or `max_output_retries`; the default is `5`, and `0` means unlimited.

Reasoning calls are capped with `--max-reasoning-seconds` or `max_reasoning_seconds`; the default is `300`. When the cap is hit, NickPit aborts the stream and retries through the existing lower-reasoning-effort fallback path. NickPit also watches streamed reasoning for loops: exact repeated lines or blocks, plus fuzzy repeated reasoning windows after normalization. Use `--max-reasoning-loop-repeats` or `max_reasoning_loop_repeats` to control how many repeats are allowed after the original; the default is `5`, and `0` disables loop detection. The fuzzy detector compares adjacent windows of completed reasoning lines after lowercasing, removing punctuation, replacing code identifiers and numbers, and comparing token shingles. It only fires on substantial repeated windows, and lower-similarity matches must also repeat review-decision markers such as finding formulation, priority, suggestion, reconsideration, or old/new-code analysis. When a loop is detected, the stream is aborted and retried with lower reasoning effort and an added instruction to avoid repeating the same analysis.

Reviews run a context agent first, then six specialist reviewer lanes in parallel: Code Quality, Security, Architecture, Performance, Testing, and Best Practices. Each lane verifies and de-duplicates its reviewer's findings as soon as that reviewer finishes, so only clean findings reach the merge agent. Concurrent LLM agent loops — reviewers, verifiers, dedupe, merge, finalize, summarize — are capped globally with `--concurrency` (default `10`, `0` = unlimited). Tool-call limits apply independently to each context, reviewer, and verifier agent. JSON output includes `total_tool_calls` at the root plus an `agent_runs` summary with each agent's token usage, tool usage, duplicate tool calls, and configured tool-call limits.

### Workflows

The review pipeline is driven by a portable workflow spec. By default `nickpit local`/`github`/`gitlab` run the built-in workflow (collect context → six reviewer lanes in parallel, each running review → verify → dedupe for its vector → merge → finalize → summarize). You can supply your own spec or run a single step instead, on any of those commands:

```bash
# Run a custom workflow spec (YAML)
nickpit local branch --spec workflow.yaml

# Run a single step on imported findings (no review needed)
nickpit local branch --step merge --findings reviewer_a.json --findings reviewer_b.json --json
nickpit local branch --step finalize --findings merged.json --json
nickpit local branch --step summarize --findings finalized.json --json
```

See [`workflow.yaml.example`](workflow.yaml.example) for the full format. A spec lists `steps` (optionally grouped under `parallel:` to run concurrently); a parallel child can be a `lane:` — a list of steps that run sequentially within the group, e.g. one reviewer's `review:` → `verify:` → `dedupe:` chain. Each step may carry a `config:` block overriding any model parameter or budget for that step only (model, temperature, reasoning_effort, max_tool_calls, max_output_retries, max_reasoning_loop_repeats, nudge_count, disable_patch_summary, verify_drop_policy, verify_drop_confidence, …) — anything unset inherits the active profile/flags. Use `model: "@small"` to select the configured `small_model` alias for a step. LLM concurrency is run-level only (`--concurrency`, default `10`, `0` = unlimited): one shared cap across every agent loop in the run. Per-vector steps are addressed as `review:security`, `verify:security`, `dedupe:security`, …; `nudge:<vector>` / `reasoning-extract:<vector>` let you drive extra rounds manually. Any global step can take `findings_from:` to inject previously-emitted findings JSON (the same format `--json` produces; one file = one merge group). Steps that only consume injected findings (e.g. `merge`, `finalize`, `summarize`) run without a git/PR source.

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
