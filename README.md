<p align="center">
  <img src="assets/nickpit.png" alt="NickPit logo" width="320">
</p>

# NickPit 🔎🐞

> **AI assisted code review, so you can merge with confidence** :100:

NickPit is a CLI that reviews local git changes, GitHub pull requests, and GitLab merge requests using any OpenAI-compatible LLM endpoint. Point it at a diff and it dispatches a small army of specialist agents who read your code, argue about it, double-check each other, throw out the duplicates, and hand you back a ranked, verified, de-duplicated list of findings — instead of one giant model monologue that confidently flags a bug on a line that doesn't exist.

## Why NickPit? 🎯

Most LLM review tools are one prompt in a trench coat. NickPit is a pipeline. Here's what you actually get:

### 6️⃣ Six specialists, not one generalist

Every review starts with a

1. **context agent** that scouts the change and
2. fans out into **six parallel reviewer lanes**:
  - Code Quality
  - Security
  - Architecture
  - Performance
  - Testing
  - Best Practices

Each lane is a focused agent with its own system prompt and its own question set — because the reviewer hunting SQL injection should not be the same one worrying about your test coverage.  

It's the difference between "a doctor" and "a hospital."

### ✔️ Findings are verified before you see them

Each lane runs **review → verify → dedupe** on its own findings the moment its reviewer finishes.  

A separate verification agent adversarially checks every finding against the actual code, and a dedupe stage collapses the echoes.  

Only clean, confirmed findings reach the merge stage.  

Hallucinated line numbers and confidently-wrong nitpicks get bounced at the door.  

### 💻 Reviewers can actually read your code

NickPit gives the model special retrieval tools:
- list and fetch files
- deep search
- language-aware **callers and callees** (go, python, nodejs, rust)
- exact line number lookups
- language detection
- versions of the toolchain

When a reviewer wonders "who calls this function?", it not only gets the call stack, but all fucntion bodies on that stack.  

Duplicate tool-call detection and per-agent call limits stop any LLM from doom-scrolling your repo.  

### 📚 Expert knowledge ships in the box

Most tools bet on a giant model that already *knows* every language's rules.  

NickPit takes the opposite bet, it carries **dense, opinionated coding guides**:  
- Go
- Helm
- Kubernetes
- Python
- Bash
- SQL
- JavaScript
- TypeScript
- C#
- HTML/CSS

[These guides](https://github.com/dgrieser/nickpit/tree/main/prompts/styleguides) are automatically injected based on your diff as **hard rules** into every agent.  

The guides are even **version-aware**:
- the Go guide tracks `1.19`–`1.26`
- Bash `3.2`–`5.2`

So NickPit picks the correct guide for the toolchain version it detects.  
When sources disagree, the most authoritative one per language decides — `go.mod` for Go, manifests like `pyproject.toml` for Python, lockfiles for TypeScript — so a stale `Dockerfile` or CI config can't drag the guide below the version the code is actually built against.  

Selection isn't done by the LLM, nor is it just file extensions.  

**Content detectors** catch embedded languages too:
- SQL inside Go
- Kubernetes YAML inside Helm templates

The verifier reads them as **evidence** — a finding that breaks a rule is confirmed, a nitpick, a rule explicitly allows, gets bounced.  

Because the expertise rides in the system prompt, **a small, cheap model reviews like it memorized the styleguide** — no huge knowledge-model required.  

The guides are **constantly updated**, so you get the latest best practices.  

Bring your own guides (local files or URLs), or turn built-ins off per language.  


### 👀 The "look again" machine

After the first pass, each reviewer gets **nudge rounds** (3 by default) asking it to look again — and a **reasoning-extractor agent** mines the reviewer's chain-of-thought for issues it *noticed but never reported*.  

Yes, NickPit reads the model's mind and files tickets for it.  

### 🌀 Loop detection for rambling models

Reasoning models sometimes get stuck rethinking the same thing forever, at your expense.  

NickPit watches the reasoning stream with a **three-layer loop detector**:
- degenerate character runs
- repeated lines/blocks
- and shingle-recurrence analysis that catches even *paraphrased* rumination

On detection the stream aborts, retries multiple times with lower reasoning effort, down to the lowest setting, and retries with special instructions to stop going in circles.  

No configuration needed.   
Your token bill will thank you. 🤑

### 🗒️ The whole pipeline is a `YAML` file

The review workflow is a portable spec — the single source of truth for execution, with zero hidden magic in code.  
Rewire it:
- reorder steps
- drop lanes
- add nudges
- run per-step model overrides
- or pipe previously-exported findings back in with `findings_from:`.

Or skip workflows entirely and run a single step (`--step merge`, `--step verdict`, …) on findings JSON you already have.

### 💸 Cheap where cheap works

Profiles can define a nested **`small` model alias** — put an expensive model on review and a budget model on summarize with `model: "@small"` per step.  

Every parameter (temperature, reasoning effort, token caps, …) can be overridden per step.  

JSON output includes a full `agent_runs` accounting of every agent's token and tool usage, so you can see exactly where the money went.

### 🤖 A GitLab review bot with no CI required

`nickpit gitlab serve` is a webhook daemon that auto-reviews MRs for opted-in projects — and anyone can summon a review on *any* MR (drafts included) by awarding a custom **`nickpit` emoji** or commenting **`/nickpit review`**.  

The daemon reacts with 👀 when it picks the review up. Comment `/nickpit abort` (or revoke the trigger emoji) to cancel a review, `/nickpit status` to see where it stands.  

Group-level tokens, longest-prefix routing for subgroups, graceful shutdown, idempotent re-reviews.  

### 🔕 Publishing that doesn't spam

`--publish` posts results back to the PR/MR:
- a summary plus one inline comment per finding
- anchored to diff lines where possible

Hidden fingerprint markers make re-runs **idempotent** — already-posted findings are skipped, and an interrupted publish heals itself on the next run.

### 🛡️ Structured output, enforced by the API

Findings are structured JSON with `p0`–`p3` priorities, confidence scores, optional fix suggestions, and an overall verdict. NickPit uses API-enforced `response_format` json_schema by default and **automatically falls back** to a prompt-embedded schema when the model doesn't support it (a pre-review model check figures this out for you — also runnable standalone via `nickpit check`).

### 🔋 Everything else you'd expect, plus some you wouldn't

- **Local review modes**: uncommitted changes, commit ranges, branch diffs.
- **GitHub PRs and GitLab MRs** via direct REST clients — by `--repo`/`--id` or just the URL.
- **Diff filters**: regex include/exclude by path *and* by file content.
- **Rate-limit aware**: parses 429 reset times and waits them out (capped), with a reasoning-effort fallback ladder for models having a bad day.
- **Terminal and JSON output**, `--show-progress` live progress, `--verbose`/`--debug` down to raw LLM payloads.
- **Global concurrency cap** (`--concurrency`, default 10) shared across every agent loop in the run.
- **Rootless, distroless, multi-arch Docker image.**
- **`nickpit inspect`**: the retrieval toolbox (files, search, callers, callees) as a standalone command tree — no review required.

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

## Quick Start

```bash
export OPENROUTER_API_KEY=sk-...
nickpit git branch --model some/model --show-progress
```

That's it: current branch vs. default branch, six reviewers, verified findings, verdict.

## Configuration

NickPit loads configuration in this order (later wins):

1. Built-in defaults
2. YAML config file from `--config` or `.nickpit.yaml`
3. Environment variables
4. CLI flags

The profile to use follows the same order: `active_profile` from the config file selects the profile unless `--profile` is passed explicitly on the command line.

Run `make generate` or `make build` to generate `.nickpit.yaml.example` from the built-in defaults.

The built-in `default` profile targets OpenRouter at `https://openrouter.ai/api/v1`. You must specify a model explicitly, and unless you set `api_key` in config, NickPit expects the API key in `OPENROUTER_API_KEY`. When the active profile ends up with no API key at all, `NICKPIT_API_KEY` is used as a last-resort fallback.

### The `small` model alias

Profiles can define a cheaper/faster alias for workflow steps:

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

The primary `max_tokens` output cap (max completion tokens the model may generate) can also be set with `--max-output-tokens`. This is the output side; the separate `--max-context-tokens` is the input budget used to trim the prompt before sending. Both default to unset for `max_tokens` (provider default) and `240000` for the context budget.

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
- Relative file paths resolve against the effective workdir — from `--workdir`, the profile's `workdir`, or `NICKPIT_WORKDIR` — and against the invocation directory when none is set.

Built-in styleguides can be turned off per language with the `disable_styleguides` profile list or the repeatable `--disable-styleguide` flag (e.g. `--disable-styleguide python --disable-styleguide sql`); CLI values append to the profile's list. The flag's `--help` text lists the available languages. The special value `all` disables every built-in styleguide (`--disable-styleguide all` or `disable_styleguides: [all]`); additional styleguides from `--styleguide`/`styleguides` are unaffected. Note that some languages share one guide file (`html`, `css`, and `scss` all map to the HTML & CSS guide), so the shared guide is only dropped when every language selecting it is disabled or absent from the diff.

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

# Review staged + unstaged tracked changes in current directory
# Plain untracked files are excluded until staged with git add.
nickpit git uncommitted

# Review staged changes only
nickpit git staged

# Review unstaged tracked changes only
nickpit git unstaged

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

### Publishing

With `--publish`, findings whose lines are part of the diff are posted inline anchored to those lines; the rest fall back to general comments that include `file:line` after the priority badge and confidence line. On GitHub this is a single PR review (the summary as the review body, findings as inline review comments); on GitLab it is a summary note plus one inline discussion per finding. Hidden markers make re-runs idempotent (already-posted comments are skipped), and a publish failure is reported as a warning without failing the review.

Known limitation: the hidden fingerprint markers are read from all existing PR/MR comments regardless of who wrote them. Anyone who can comment on the PR/MR can therefore forge a marker and suppress a matching finding from being posted on the next run.

## Discuss a Review (Chat) 💬

After a review you can talk to an agent about it. The discussion agent gets the same context a reviewer/verifier has — the diff, the toolchain, the applicable styleguides, and the same retrieval tools — plus the **complete findings JSON and the overall verdict**. It is free-form: no workflow, no output schema, no priority gates. Ask why a finding is a bug, push back on a nitpick, or propose a fix and have it evaluated.

Every review automatically saves a resumable session — including the exact prepared context the reviewers saw — so chatting needs no re-fetch (disable with `--no-session`). Session files live under `$NICKPIT_CACHE_DIR/sessions` (or `<user cache>/nickpit/sessions`); override with `--session-dir`. The store keeps the 50 most recent sessions. Resuming a GitLab session checks the MR's live head and recreates the diff when new commits landed; pass `--repo-root <checkout>` on resume to point the retrieval tools at a local checkout (without one, code-reading tools stay off for remote sessions).

```bash
# Chat about the most recent review (interactive REPL)
nickpit chat

# One-shot question about a specific finding, then exit
nickpit chat --finding <finding-id> "why is this actually a bug?"

# Resume a specific session
nickpit chat --session <session-id>

# Start a chat from a saved review JSON (e.g. a CI artifact from `--json`)
nickpit chat --from-json review.json

# Start a chat from a GitLab MR — findings are reassembled from the review NickPit posted
nickpit chat --gitlab --url https://gitlab.example.com/group/project/-/merge_requests/456
```

Pin the chat to one finding with `--finding <id>` and the agent opens by pointing at it; omit it to discuss the whole review. On GitLab, the review NickPit publishes now embeds the full findings JSON (and the overall verdict) as hidden, gzip-compressed markers in the notes, each tagged with a review id and timestamp, so a later chat can regroup them into the exact (newest) review — no local state needed. Because the markers are encoded but not cryptographically signed, only markers in notes authored by the chat token's own user (the bot that published the review) are trusted; markers planted by other commenters are ignored. When an MR carries several reviews, the newest is chosen (`--review-id` overrides). The retrieval tools read from a local checkout when one is available (`--repo-root`, or the current directory for local sessions).

## GitLab Webhook Daemon

`nickpit gitlab serve` runs an HTTP daemon that reviews MRs automatically from GitLab **group webhooks** — no CI pipeline integration needed. Each review runs as a separate `nickpit gitlab mr --publish` child process; comment fingerprints keep re-reviews idempotent.

Triggers:

- **Auto**: MR opened, reopened, or marked ready — only for projects carrying the opt-in topic (default `nickpit`). Draft MRs are skipped. New commits never re-review automatically; request one with the trigger emoji or the review command.
- **Manual**: a user awards the trigger emoji (default a custom emoji named `nickpit`) on an MR — works regardless of topic and also on drafts. Revoking the trigger emoji aborts the MR's queued or running review.
- **Commands**: an MR comment starting with `/nickpit <command>` (keyword configurable via `command_keyword`):
  - `/nickpit review` — request a review (same semantics as the trigger emoji: any project, drafts too)
  - `/nickpit abort` — cancel the queued or running review for this MR
  - `/nickpit status` — reply with the MR's review state
  - `/nickpit help` — reply with the command list

When a review starts, the daemon awards a start emoji on the MR (default `:eyes:`, `start_emoji: ""` disables). Command comments are acknowledged with a reaction emoji (default `:white_check_mark:`, `ack_emoji: ""` disables); `status`, `help`, and `abort` also get a comment reply, threaded under the command.

- **Discussion (chat)**: reply in a thread NickPit started — under a finding's comment or the summary — and the daemon answers in-thread with the discussion agent, no keyword needed. Like reviews, each reply runs as a separate `nickpit chat` child process (the daemon itself never loads the LLM), which reassembles the review from the hidden markers on the MR, rebuilds the diff from the current MR, and posts the answer threaded (a reply under a finding is focused on that finding; under the summary it is about the whole review). The whole conversation lives in the MR thread, so it survives daemon restarts. The same `nickpit chat --gitlab --url <MR> --reply-discussion <id>` is runnable from the terminal.

```bash
nickpit gitlab serve --serve-config server.yaml
```

The daemon config is a separate file (default `server.yaml`, see [`server.yaml.example`](server.yaml.example)); `${VAR}` references are expanded from the environment:

```yaml
gitlab_base_url: "https://gitlab.example.com"
groups:
  - path: "platform"                              # group (or subgroup) path prefix
    token: "${NICKPIT_GL_TOKEN_PLATFORM}"         # group access token, api scope
    webhook_secret: "${NICKPIT_GL_SECRET_PLATFORM}"
```

Events are routed to the group with the longest matching path prefix, so nested groups can carry their own token and secret. The group list can also live in a separate file appended via `groups_file` (same `groups:` shape, also env-expanded) — useful when the inventory comes from a mounted Kubernetes Secret while the rest of the config is a ConfigMap. The regular `.nickpit.yaml` (LLM profile) is still read by the review child processes; `--config` is forwarded to them, and the group token/base URL are injected via `NICKPIT_GITLAB_TOKEN`/`NICKPIT_GITLAB_BASE_URL`.

GitLab setup per group (group webhooks require GitLab Premium; emoji events require GitLab >= 17.5):

1. Create a group access token (role Developer, scope `api`) — reviews are posted as this bot user.
2. Create the custom emoji `nickpit` in the group (for manual trigger).
3. Group → Settings → Webhooks: URL `https://<daemon>/webhooks/gitlab`, the secret token, and enable **Merge request events**, **Emoji events**, and **Comment events** (for the `/nickpit` commands).
4. Opt projects into auto-review by adding the topic `nickpit` (Project → Settings → General → Topics).

Docker compose example:

```yaml
services:
  nickpit:
    image: nickpit
    command: ["gitlab", "serve"]
    ports:
      - "8080:8080"
    volumes:
      - ./nickpit.yaml:/work/.nickpit.yaml:ro
      - ./server.yaml:/work/server.yaml:ro
      - nickpit-logs:/work/logs
    environment:
      OPENROUTER_API_KEY: "..."
      NICKPIT_GL_TOKEN_PLATFORM: "..."
      NICKPIT_GL_SECRET_PLATFORM: "..."
volumes:
  nickpit-logs:
```

Per-review child logs land in `log_dir` (default `logs/`) as `review-<project>-<iid>-<timestamp>.log`; `GET /healthz` reports queue depth. On SIGTERM the daemon stops accepting events and lets running reviews finish within `shutdown_grace` (default `10m`) before terminating them — an interrupted publish heals on the next run via the comment fingerprints. Queue state is in-memory only; events arriving while the daemon is down are recovered by awarding the trigger emoji (or the review command).

## Tuning a Review

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

### Tool Calls and Retries

NickPit lets the model request additional file context during review. Control the maximum number of tool-call iterations with `--max-tool-calls` or `max_tool_calls` in config. `0` means unlimited, which is the default. You can also stop tool use after too many duplicate requests with `--max-duplicate-tool-calls` or `max_duplicate_tool_calls`; the default is `5`. Invalid model output is retried with `--max-output-retries` or `max_output_retries`; the default is `5`, and `0` means unlimited.

### Finding Caps

Cap how many findings each review agent may report with `--max-findings` or `max_findings` in config (also overridable per `review:` step in a workflow). The default is `0`, meaning unlimited. When a limit is set it is added to the reviewer prompt; a response exceeding the limit is retried once with guidance to keep only the strongest findings, after which the weakest findings (lowest priority, then lowest confidence) are cut and the agent run is marked partial. The limit counts the agent's whole session: initial pass plus nudge rounds; once the limit is reached, remaining nudge rounds (including standalone `nudge:` steps) are skipped.

### Reasoning Caps and Loop Detection

Reasoning calls are capped with `--max-reasoning-seconds` or `max_reasoning_seconds`; the default is `300`. When the cap is hit, NickPit aborts the stream and retries through the existing lower-reasoning-effort fallback path. NickPit also watches streamed reasoning for loops; detection is built in and needs no configuration. Three layered signals cover the observed failure modes: degenerate character runs (one character or a short unit repeated back-to-back), exact repeated lines or blocks (whitespace runs collapsed, empty lines ignored), and shingle recurrence — the fraction of recently emitted token shingles (lowercased, punctuation removed, code identifiers and numbers masked) that already appeared earlier in the same stream. Verbatim loops drive that recurrence to ~1.0 and are cancelled quickly; paraphrase loops (the same decision cycle reworded) plateau lower and must persist longer before they fire. Thresholds are staged over the reasoning time budget: early in a stream only ironclad repetition may cancel it, and detection becomes progressively more aggressive as the stream approaches `max_reasoning_seconds`, where it would be cancelled anyway. When a loop is detected, the stream is aborted and retried with lower reasoning effort and an added instruction to avoid repeating the same analysis.

### Concurrency and Accounting

Reviews run a context agent first, then six specialist reviewer lanes in parallel: Code Quality, Security, Architecture, Performance, Testing, and Best Practices. Each lane verifies and de-duplicates its reviewer's findings as soon as that reviewer finishes, so only clean findings reach the merge agent. Concurrent LLM agent loops — reviewers, verifiers, dedupe, merge, finalize, verdict, summarize — are capped globally with `--concurrency` (default `10`, `0` = unlimited). Tool-call limits apply independently to each context, reviewer, and verifier agent. JSON output includes `total_tool_calls` at the root plus an `agent_runs` summary with each agent's token usage, tool usage, duplicate tool calls, and configured tool-call limits.

Token accounting in the JSON output works as follows:
- `tokens_used` at root is the grand total for the whole run (including retried calls)
- `verify_tokens_used` is a breakdown of tokens used by the verifier agents
- `finalize_tokens_used` is a breakdown of tokens used by the finalizer agents
- `verdict_tokens_used` is a breakdown of tokens used by the verdict agent
- `summarize_tokens_used` is a breakdown of tokens used by the summarizer agent
- `agent_runs` entries each carry their own `tokens_used` breakdown per `role`:
  - `context` — the context-gathering agent that scouts the change before the reviewer lanes
  - `review` — a reviewer lane's whole session: initial pass, all nudge rounds, and reasoning-extraction
  - `verify` — a **per-finding** verification agent
  - `dedupe` — a **per-reviewer** de-duplication agent
  - `merge` — the cross-lane merge agent, one entry **per merge cluster**
  - `finalize` — the finalizer that fixes finding wording, priority, and confidence
  - `verdict` — the verdict agent that sets the top-level `overall_*` fields
  - `summarize` — the review summarizer

The root `tokens_used` is already the sum of everything, so **do not sum any of the breakdowns**.


## Workflows

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

See [`workflow.yaml.example`](workflow.yaml.example) for the full format. A spec lists `steps` (optionally grouped under `parallel:` to run concurrently); a parallel child can be a `lane:` — a list of steps that run sequentially within the group, e.g. one reviewer's `review:` → `verify:` → `dedupe:` chain. A `pipeline:` group is the explicit, streamed post-review tail (`merge` → `finalize` → `verdict`, optionally `summarize`): its steps overlap with no barrier between them — each merge cluster flows straight into finalize/summarize while other clusters are still merging, and verdict gates on all finalizes. Listing those steps flat instead runs them strictly sequentially, each over the whole finding set. Each step may carry a `config:` block overriding any model parameter or budget for that step only (model, temperature, top_p, top_k, presence_penalty, reasoning_effort, scope, max_tool_calls, max_output_retries, nudge_count, max_findings, disable_patch_summary, disable_suggestions, verify_drop_policy, confidence_threshold, …) — anything unset inherits the active profile/flags. `scope` makes a step's fan-out explicit — the work unit each agent operates on: `all` (whole finding set), `cluster` (per merge cluster, `merge`/`finalize`/`summarize`), `finding` (per finding, `verify`), or `reviewer` (per reviewer group, `dedupe`); cluster-scoped finalize/summarize is valid only inside a `pipeline:`. Use `model: "@small"` to select the configured nested `small` profile for a step. `review:<vector>` configs can also override internal agents with `mine_reasoning:`, `compile_findings:`, and `nudge:` subconfigs. LLM concurrency is run-level only (`--concurrency`, default `10`, `0` = unlimited): one shared cap across every agent loop in the run — it is intentionally not in the spec. Per-vector steps are addressed as `review:security`, `verify:security`, `dedupe:security`, …; `nudge:<vector>` / `reasoning-extract:<vector>` let you drive extra rounds manually. Any global step can take `findings_from:` to inject previously-emitted findings JSON (the same format `--json` produces; one file = one merge group); inside a `pipeline:` only the `merge` step may carry it. Steps that only consume injected findings (e.g. `merge`, `finalize`, `verdict`, `summarize`) run without a git/PR source. `finalize` now only finalizes finding wording/priority/confidence; include `verdict` after it when a workflow needs final top-level `overall_correctness`, `overall_explanation`, and `overall_confidence_score`. `confidence_threshold` is applied only by the `verdict` step.

## Filtering Review Output

Review output filtering uses `--priority-threshold` with `0` through `3`, where `0` is highest priority and `3` is lowest (the default, showing everything). Findings are still displayed with `p0`–`p3` badges. `--confidence-threshold` filters findings at the start of the `verdict` step using finalized confidence; workflows without `verdict` do not apply the confidence threshold and emit a warning.

## Inspect Commands

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

Retrieval supports `go`, `python`, `nodejs` (including `.jsx`/`.tsx`), and `rust` source files. `inspect file`, `inspect list`, and `inspect search` work generically across text files, while `inspect callers` and `inspect callees` use language-aware symbol and call-hierarchy analysis: Go is resolved with the type checker (`go/packages`), TypeScript/JavaScript with esbuild's parser, and Python/Rust with a pure-Go tree-sitter runtime — all CGo-free, so the single static binary stays self-contained.

## Notes

- The CLI expects an OpenAI-compatible `/chat/completions` endpoint.
- Remote reviews clone the requested PR/MR head into a temporary checkout when retrieval needs local files.
- Use `--workdir` (or `workdir` in config / the `NICKPIT_WORKDIR` env var) to reuse an existing clone for remote reviews; NickPit creates a temporary worktree at the requested revision instead of cloning again.
- Note the workdir asymmetry: for local (`nickpit git ...`) reviews only the `--workdir` CLI flag changes the directory the review runs in; `workdir` from config or `NICKPIT_WORKDIR` applies only to where remote PR/MR checkouts (clones/worktrees) are placed.
