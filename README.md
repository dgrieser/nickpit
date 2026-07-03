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
- API-enforced JSON response format (`response_format` json_schema) by default, with automatic fallback to a prompt-embedded schema when the model lacks support
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
  ghcr.io/dgrieser/nickpit:latest git branch

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

The profile to use follows the same order: `active_profile` from the config file selects the profile unless `--profile` is passed explicitly on the command line.

Run `make generate` or `make build` to generate `.nickpit.yaml.example` from the built-in defaults.

The built-in `default` profile targets OpenRouter at `https://openrouter.ai/api/v1`. You must specify a model explicitly, and unless you set `api_key` in config, NickPit expects the API key in `OPENROUTER_API_KEY`. When the active profile ends up with no API key at all, `NICKPIT_API_KEY` is used as a last-resort fallback.

Profiles can also define a cheaper/faster alias for workflow steps:

```yaml
profiles:
  default:
    model: primary-model
    max_tokens: 4096
    temperature: 0.2
    top_p: 0.9
    top_k: 40
    presence_penalty: 0.1
    reasoning_effort: high
    small:
      model: small-model
      max_tokens: 2048
      temperature: 0.2
      top_p: 0.9
      top_k: 40
      presence_penalty: 0.1
      extra_body: {}
      reasoning_effort: low
```

`model: "@small"` in workflow step config selects the nested `small` config. Any unset small field falls back to the primary profile value. Small model settings can also be set with `NICKPIT_SMALL_*` environment variables or `--small-*` flags such as `--small-model`, `--small-reasoning-effort`, `--small-top-k`, `--small-presence-penalty`, and `--small-max-output-tokens`. The primary model has the same environment variables without the `SMALL_` part: `NICKPIT_MODEL`, `NICKPIT_REASONING_EFFORT`, `NICKPIT_MAX_TOKENS`, `NICKPIT_TEMPERATURE`, `NICKPIT_TOP_P`, `NICKPIT_TOP_K`, `NICKPIT_PRESENCE_PENALTY`, and `NICKPIT_EXTRA_BODY`.

The primary `max_tokens` output cap (max completion tokens the model may generate) can also be set with `--max-output-tokens`. This is the output side; the separate `--max-context-tokens` is the input budget used to trim the prompt before sending. Both default to unset for `max_tokens` (provider default) and `120000` for the context budget.

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

### Additional Styleguides

Beyond the built-in language styleguides (selected automatically from the languages in the diff), profiles can list additional styleguides that every agent receives — review, verification, finalization, verdict, and merge. Each entry is a local file path or an HTTP(S) URL:

```yaml
profiles:
  default:
    styleguides:
      - docs/team-style.md
      - https://raw.githubusercontent.com/org/styleguides/main/go.md
```

The repeatable `--styleguide` flag adds more per run. Unlike the filter flags, CLI values **append** to the profile's list instead of replacing it.

Rules:

- Guides are loaded before the review starts; an unreadable file or failed fetch aborts the run immediately.
- URLs are fetched fresh on every run with a plain unauthenticated GET (no caching); redirects are followed.
- Each guide is capped at 1 MiB and must be non-empty text.
- Relative file paths resolve against `--workdir` when given, otherwise against the invocation directory.

Built-in styleguides can be turned off per language with the `disable_styleguides` profile list or the repeatable `--disable-styleguide` flag (e.g. `--disable-styleguide python --disable-styleguide sql`); CLI values append to the profile's list. The flag's `--help` text lists the available languages. Note that some languages share one guide file (`html`, `css`, and `scss` all map to the HTML & CSS guide), so the shared guide is only dropped when every language selecting it is disabled or absent from the diff.

```yaml
profiles:
  default:
    disable_styleguides: [python, sql]
```

## Usage

```bash
# Review current branch in current directory against default branch
nickpit git branch

# Review current branch in specified directory against default branch
nickpit git branch --workdir /path/to/dir

# Review feature/my-branch against main in specified directory
nickpit git branch --base main --head feature/my-branch --workdir /path/to/dir

# Review specific commit range in current directory
nickpit git commits --from HEAD~3 --to HEAD

# Review uncommitted changes in current directory
nickpit git uncommitted

# Review PR in GitHub
nickpit github pr --repo owner/repo --id 123
nickpit github pr --repo owner/repo --id 123 --workdir ~/src/repo
nickpit github pr --url https://github.com/owner/repo/pull/123

# Review a GitHub PR and post the result back as a review (summary + one comment per finding)
nickpit github pr --repo owner/repo --id 123 --publish

# Review MR in GitLab
nickpit gitlab mr --repo group/project --id 456
nickpit gitlab mr --url https://gitlab.example.com/group/project/-/merge_requests/456

# Review a GitLab MR and post the result back as comments (summary + one per finding)
nickpit gitlab mr --repo group/project --id 456 --publish
```

With `--publish`, findings whose lines are part of the diff are posted inline anchored to those lines; the rest fall back to general comments that include `file:line` after the priority badge and confidence line. On GitHub this is a single PR review (the summary as the review body, findings as inline review comments); on GitLab it is a summary note plus one inline discussion per finding. Hidden markers make re-runs idempotent (already-posted comments are skipped), and a publish failure is reported as a warning without failing the review.

Known limitation: the hidden fingerprint markers are read from all existing PR/MR comments regardless of who wrote them. Anyone who can comment on the PR/MR can therefore forge a marker and suppress a matching finding from being posted on the next run.

### Progress

Append `--show-progress` to print review details and tool calls on stderr.

### Patch Summary

By default, the final overall explanation starts with an assumed summary of what the patch is intended to do. Use `--disable-patch-summary` or `disable_patch_summary: true` in the active profile to omit that summary from final output while still allowing internal agents to use context notes.

### Suggestions

By default, NickPit may include suggested fixes when an obvious replacement exists. Use `--disable-suggestions` or `disable_suggestions: true` in the active profile to suppress suggestions in prompts, JSON output, terminal output, and published PR/MR comments.

### Debug

Append `--verbose` or `--debug` to print step-by-step execution details to stderr, including prompt rendering and raw LLM request/response payloads.


### Output Schema Mode

By default, NickPit sends the review schema via the API `response_format` field (json_schema constrained output). When the pre-review model check finds the model does not support it, NickPit warns and automatically falls back to embedding the schema in the system prompt — the review still runs.
Use `--disable-json-response-format` to force the prompt-embedded schema instead. The same setting can be stored in config as `disable_json_response_format: true` in the active profile, or per workflow step.

NickPit lets the model request additional file context during review. Control the maximum number of tool-call iterations with `--max-tool-calls` or `max_tool_calls` in config. `0` means unlimited, which is the default. You can also stop tool use after too many duplicate requests with `--max-duplicate-tool-calls` or `max_duplicate_tool_calls`; the default is `5`. Invalid model output is retried with `--max-output-retries` or `max_output_retries`; the default is `5`, and `0` means unlimited.

Reasoning calls are capped with `--max-reasoning-seconds` or `max_reasoning_seconds`; the default is `300`. When the cap is hit, NickPit aborts the stream and retries through the existing lower-reasoning-effort fallback path. NickPit also watches streamed reasoning for loops; detection is built in and needs no configuration. Three layered signals cover the observed failure modes: degenerate character runs (one character or a short unit repeated back-to-back), exact repeated lines or blocks (whitespace runs collapsed, empty lines ignored), and shingle recurrence — the fraction of recently emitted token shingles (lowercased, punctuation removed, code identifiers and numbers masked) that already appeared earlier in the same stream. Verbatim loops drive that recurrence to ~1.0 and are cancelled quickly; paraphrase loops (the same decision cycle reworded) plateau lower and must persist longer before they fire. Thresholds are staged over the reasoning time budget: early in a stream only ironclad repetition may cancel it, and detection becomes progressively more aggressive as the stream approaches `max_reasoning_seconds`, where it would be cancelled anyway. When a loop is detected, the stream is aborted and retried with lower reasoning effort and an added instruction to avoid repeating the same analysis.

Reviews run a context agent first, then six specialist reviewer lanes in parallel: Code Quality, Security, Architecture, Performance, Testing, and Best Practices. Each lane verifies and de-duplicates its reviewer's findings as soon as that reviewer finishes, so only clean findings reach the merge agent. Concurrent LLM agent loops — reviewers, verifiers, dedupe, merge, finalize, verdict, summarize — are capped globally with `--concurrency` (default `10`, `0` = unlimited). Tool-call limits apply independently to each context, reviewer, and verifier agent. JSON output includes `total_tool_calls` at the root plus an `agent_runs` summary with each agent's token usage, tool usage, duplicate tool calls, and configured tool-call limits.

### Workflows

The review pipeline is driven by a portable workflow spec. The spec is the single source of truth for execution — there is no auto-fusion or hidden execution-shape decisions in code. By default `nickpit git`/`github`/`gitlab` run the built-in workflow (collect context → six reviewer lanes in parallel, each running review → verify → dedupe for its vector → then a `pipeline:` tail that streams merge → finalize → verdict → summarize). You can supply your own spec or run a single step instead, on any of those commands:

```bash
# Run a custom workflow spec (YAML)
nickpit git branch --spec workflow.yaml

# Run a single step on imported findings (no review needed)
nickpit git branch --step merge --findings reviewer_a.json --findings reviewer_b.json --json
nickpit git branch --step finalize --findings merged.json --json
nickpit git branch --step verdict --findings finalized.json --json
nickpit git branch --step summarize --findings finalized.json --json
```

See [`workflow.yaml.example`](workflow.yaml.example) for the full format. A spec lists `steps` (optionally grouped under `parallel:` to run concurrently); a parallel child can be a `lane:` — a list of steps that run sequentially within the group, e.g. one reviewer's `review:` → `verify:` → `dedupe:` chain. A `pipeline:` group is the explicit, streamed post-review tail (`merge` → `finalize` → `verdict`, optionally `summarize`): its steps overlap with no barrier between them — each merge cluster flows straight into finalize/summarize while other clusters are still merging, and verdict gates on all finalizes. Listing those steps flat instead runs them strictly sequentially, each over the whole finding set. Each step may carry a `config:` block overriding any model parameter or budget for that step only (model, temperature, top_p, top_k, presence_penalty, reasoning_effort, scope, max_tool_calls, max_output_retries, nudge_count, disable_patch_summary, disable_suggestions, verify_drop_policy, confidence_threshold, …) — anything unset inherits the active profile/flags. `scope` makes a step's fan-out explicit — the work unit each agent operates on: `all` (whole finding set), `cluster` (per merge cluster, `merge`/`finalize`/`summarize`), `finding` (per finding, `verify`), or `reviewer` (per reviewer group, `dedupe`); cluster-scoped finalize/summarize is valid only inside a `pipeline:`. Use `model: "@small"` to select the configured nested `small` profile for a step. `review:<vector>` configs can also override internal agents with `mine_reasoning:`, `compile_findings:`, and `nudge:` subconfigs. LLM concurrency is run-level only (`--concurrency`, default `10`, `0` = unlimited): one shared cap across every agent loop in the run — it is intentionally not in the spec. Per-vector steps are addressed as `review:security`, `verify:security`, `dedupe:security`, …; `nudge:<vector>` / `reasoning-extract:<vector>` let you drive extra rounds manually. Any global step can take `findings_from:` to inject previously-emitted findings JSON (the same format `--json` produces; one file = one merge group); inside a `pipeline:` only the `merge` step may carry it. Steps that only consume injected findings (e.g. `merge`, `finalize`, `verdict`, `summarize`) run without a git/PR source. `finalize` now only finalizes finding wording/priority/confidence; include `verdict` after it when a workflow needs final top-level `overall_correctness`, `overall_explanation`, and `overall_confidence_score`. `confidence_threshold` is applied only by the `verdict` step.

### Filtering Review Output

Review output filtering uses `--priority-threshold` with `0` through `3`, where `0` is highest priority and `3` is lowest (the default, showing everything). Findings are still displayed with `p0`–`p3` badges. `--confidence-threshold` filters findings at the start of the `verdict` step using finalized confidence; workflows without `verdict` do not apply the confidence threshold and emit a warning.

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
- Use `--workdir` (or `workdir` in config / the `NICKPIT_WORKDIR` env var) to reuse an existing clone for remote reviews; NickPit creates a temporary worktree at the requested revision instead of cloning again.
- Note the workdir asymmetry: for local (`nickpit git ...`) reviews only the `--workdir` CLI flag changes the directory the review runs in; `workdir` from config or `NICKPIT_WORKDIR` applies only to where remote PR/MR checkouts (clones/worktrees) are placed.
