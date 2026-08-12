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

Verification starts with a private, blind classifier that describes each finding without seeing routing outcomes or the patch. Go code then applies the configured drop policy and checks or uniquely relocates the finding against exact diff windows. A separate verification agent adversarially checks what remains against the actual code, and a dedupe stage collapses the echoes.

Only clean, confirmed findings reach the merge stage.  

Hallucinated line numbers and confidently-wrong nitpicks get bounced at the door.  

### 💻 Reviewers can actually read your code

NickPit gives the model special retrieval tools:
- list and fetch files
- deep search
- language-aware symbol definitions and references, with whole containing functions
- language-aware **callers and callees** (go, python, nodejs, rust)
- exact line number lookups
- language detection
- versions of the toolchain
- **commit history**: filtered commit listings and per-commit diffs

When a reviewer wonders "who calls this function?", it not only gets the call stack, but all fucntion bodies on that stack.  
When it wonders "why was this written like that?", it reads the commit that introduced the line — message, author and full diff.  

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

JSON output includes run-level `agent_runs` accounting plus separate token totals for internal per-finding classification and verification.

### 🤖 A GitLab review bot with no CI required

`nickpit gitlab serve` is a webhook daemon that auto-reviews MRs for opted-in projects — and anyone can summon a review on *any* MR (drafts included) by awarding a custom **`nickpit` emoji** or commenting **`/nickpit review`**.  

The daemon reacts with 👀 when it picks the review up. Comment `/nickpit abort` (or revoke the trigger emoji) to cancel a review, `/nickpit status` to see where it stands.  

Group-level tokens, longest-prefix routing for subgroups, graceful shutdown, idempotent re-reviews.  

### 🔕 Publishing that doesn't spam

`--publish` posts results back to the PR/MR:
- a summary plus one inline comment per finding
- anchored to diff lines where possible

Hidden fingerprint markers make re-runs **idempotent** — already-posted findings are skipped, and an interrupted publish heals itself on the next run.

### 💬 Chat about a Review

After a review you can talk to an agent about it.
- ask why a finding is a bug
- push back on a nitpick
- propose a fix and have it evaluated

### 🛡️ Structured output, enforced by the API

Findings are structured JSON with `p0`–`p3` priorities, confidence scores, optional fix suggestions, and an overall verdict. NickPit uses API-enforced `response_format` json_schema by default and **automatically falls back** to a prompt-embedded schema when the model doesn't support it (a pre-review model check figures this out for you — also runnable standalone via `nickpit check`).

### 🔋 Everything else you'd expect, plus some you wouldn't

- **Local review modes**: uncommitted changes, commit ranges, branch diffs.
- **GitHub PRs and GitLab MRs** via direct REST clients — by `--repo`/`--id` or just the URL.
- **Diff filters**: regex include/exclude by path *and* by file content.
- **Rate-limit aware**: parses 429 reset times and waits them out (capped), with a reasoning-effort fallback ladder for models having a bad day.
- **Rendered terminal, raw Markdown, and JSON output**, live progress with progress bars, `--show-progress` for running progress, `--verbose`/`--debug` down to raw LLM payloads.
- **Global concurrency cap** (`--concurrency`, default 10) shared across every agent loop in the run.
- **Rootless, distroless, Docker image.**
- **`nickpit inspect`**: the retrieval toolbox (files, search, symbol references, callers, callees, commit log and commit diffs) as a standalone command tree — no review required.

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

Images are published to `ghcr.io/dgrieser/nickpit` (`amd64`). The image is
**rootless** and **distroless** — it runs as a non-root user by default and
supports an arbitrary runtime UID, so you can map it to your host user to
operate on mounted repositories and config.

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

The profile to use follows the same order: `active_profile` from the config file selects the profile, `NICKPIT_PROFILE` overrides that, and an explicit `--profile` on the command line wins over both. A workflow spec's `profile:` field still retargets the profile for that run. A name that no profile defines fails immediately with `profile "<name>" not found`, so a typo cannot quietly turn into a half-configured run.

Run `make generate` or `make build` to generate `.nickpit.yaml.example` from the built-in defaults.

The built-in `default` profile targets OpenRouter at `https://openrouter.ai/api/v1`. You must specify a model explicitly, and unless you set `api_key` in config, NickPit expects the API key in `OPENROUTER_API_KEY`. When the active profile ends up with no API key at all, `NICKPIT_API_KEY` is used as a last-resort fallback.

### Environment variables

Useful when the config file is baked into an image or CI runner and only a few knobs should differ per environment. An explicitly passed flag always wins over the variable; an unset or empty variable changes nothing.

| Variable | Flag equivalent |
| --- | --- |
| `NICKPIT_PROFILE` | `--profile` |
| `NICKPIT_CONFIG` | `--config` |
| `NICKPIT_SPEC` | `--spec` (ignored when `--step` is passed, since the two are mutually exclusive) |
| `NICKPIT_SESSION_DIR` | `--session-dir` |
| `NICKPIT_OUTPUT` | `--output` / `-o` |
| `NICKPIT_PRIORITY_THRESHOLD` | `--priority-threshold` |
| `NICKPIT_VERIFY_DROP_POLICY` | `--verify-drop-policy` |
| `NICKPIT_CONFIDENCE_THRESHOLD` | `--confidence-threshold` |
| `NICKPIT_DIFF_FORMAT` | `--diff-format` |
| `NICKPIT_MAX_CONTEXT_TOKENS` | `--max-context-tokens` |
| `NICKPIT_MAX_REQUEST_BYTES` | `--max-request-bytes` |
| `NICKPIT_MAX_TOOL_CALLS` | `--max-tool-calls` |
| `NICKPIT_MAX_DUPLICATE_TOOL_CALLS` | `--max-duplicate-tool-calls` |
| `NICKPIT_MAX_OUTPUT_RETRIES` | `--max-output-retries` |
| `NICKPIT_MAX_REASONING_SECONDS` | `--max-reasoning-seconds` |
| `NICKPIT_MAX_RATE_LIMIT_DELAY_SECONDS` | `--max-rate-limit-delay-seconds` |
| `NICKPIT_NUDGE_COUNT` | `--nudge-count` |
| `NICKPIT_MAX_FINDINGS` | `--max-findings` |
| `NICKPIT_MAX_SESSIONS` | `--max-sessions` |

A `0` from the environment is honored where `0` is meaningful (`--max-tool-calls`, `--nudge-count`, `--max-findings`, `--max-sessions`, `--max-request-bytes`, `--max-rate-limit-delay-seconds`), so it is not mistaken for "unset". A non-numeric value fails the run with the variable name in the error.

Model and provider settings have their own variables: `NICKPIT_MODEL`, `NICKPIT_BASE_URL`, `NICKPIT_API_KEY`, `NICKPIT_REASONING_EFFORT`, the sampling knobs and their `NICKPIT_SMALL_*` counterparts (see [The `small` model alias](#the-small-model-alias)), plus `NICKPIT_WORKDIR`, `NICKPIT_GITHUB_TOKEN`, `NICKPIT_GITLAB_TOKEN`, `NICKPIT_GITLAB_BASE_URL`, and `NICKPIT_CACHE_DIR`.

`--concurrency` stays CLI-only on purpose: the execution shape of a run should be visible in the command that started it, not inherited from the environment.

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

Beyond the built-in language styleguides (selected automatically from the languages in the diff), profiles can list additional styleguides that every agent receives — review, verification, dedupe, merge, finalization, and verdict. Each entry is a local file path or an HTTP(S) URL:

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

Every review automatically saves a resumable session — including the exact prepared context the reviewers saw — so chatting needs no re-fetch (disable with `--no-session`). A review that found nothing is saved too, so "why did you find nothing here?" stays answerable. Session files live under `$NICKPIT_CACHE_DIR/sessions` (or `<user cache>/nickpit/sessions`); override with `--session-dir`. The store keeps every session by default; cap it with `--max-sessions` or `max_sessions` in config (`0` = unlimited) and each save deletes the oldest files beyond the cap. Resuming a GitLab session checks the MR's live head and recreates the diff when new commits landed. For remote sessions the retrieval tools read from a temporary checkout of the live head, cloned automatically for the duration of the chat (the same mechanism reviews use) and removed when it ends; pass `--repo-root <checkout>` to use a local checkout instead (full history, local edits). Code-reading tools stay off only when tools are disabled (`max_tool_calls: -1`) or the checkout cannot be prepared.

```bash
# Chat about the most recent review (interactive REPL)
nickpit chat

# One-shot question about a specific finding, then exit
nickpit chat --finding <finding-id> "why is this actually a bug?"

# Resume a specific session
nickpit chat --session <session-id>

# Print the review stored in a previous session (latest when omitted)
nickpit session [session-id]
nickpit session --session <session-id> --output json

# Copy that review to the system clipboard instead of printing it
nickpit session [session-id] --clipboard

# Start a chat from a saved review JSON (e.g. a CI artifact from `--output json`)
nickpit chat --from-json review.json

# Start a chat from a GitLab MR — findings are reassembled from the review NickPit posted
nickpit chat --gitlab --url https://gitlab.example.com/group/project/-/merge_requests/456
```

Pin the chat to one finding with `--finding <id>` and the agent opens by pointing at it; omit it to discuss the whole review. On GitLab, the review NickPit publishes now embeds the full findings JSON (and the overall verdict) as hidden, gzip-compressed markers in the notes, each tagged with a review id and timestamp, so a later chat can regroup them into the exact (newest) review — no local state needed. Because the markers are encoded but not cryptographically signed, only markers in notes authored by the chat token's own user (the bot that published the review) are trusted; markers planted by other commenters are ignored. When an MR carries several reviews, the newest is chosen (`--review-id` overrides). The retrieval tools read from an automatic temporary checkout of the MR head (or a local checkout: `--repo-root`, or the current directory for local sessions) — this includes the daemon's in-thread replies, which answer with code-reading tools enabled.

`nickpit session` uses the normal review output. Select it with `-o|--output markdown|json|raw`: `markdown` is the default and renders on a terminal, while `raw` always emits unrendered Markdown with no colors or terminal styling. The same flag works on review commands. `--json` remains as a compatibility alias for `--output json`.

`--clipboard` copies the review to the system clipboard instead of printing it, and prints a one-line confirmation naming the helper it used. The clipboard always receives unstyled source — Markdown for `--output markdown|raw`, JSON for `--output json` — so it pastes cleanly into an MR comment, an issue, or a chat. There is no portable clipboard syscall, so NickPit shells out to the first working helper for the platform: `pbcopy` on macOS, `clip.exe` on Windows (fed UTF-16LE so non-ASCII survives), and on Linux/BSD `wl-copy` first under Wayland, otherwise `xclip` then `xsel`, falling back to `termux-clipboard-set` and WSL's `clip.exe`. When none of them is installed or works (a headless session, for example), the command fails with the list of candidates tried and what to install; pipe the output instead. The hint printed after every review shows both the chat and the clipboard command for the session it just saved.

### Shell Completion

Generate Bash or Zsh completion scripts:

```bash
# Install persistently for current user
nickpit completion bash --install
nickpit completion zsh --install

# Current shell
source <(nickpit completion bash)

# Bash, persistent
nickpit completion bash > "${XDG_DATA_HOME:-$HOME/.local/share}/bash-completion/completions/nickpit"

# Zsh, current shell
source <(nickpit completion zsh)

# Zsh, persistent (ensure this directory is in fpath)
nickpit completion zsh > "${XDG_DATA_HOME:-$HOME/.local/share}/zsh/site-functions/_nickpit"
```

Installed completions suggest commands and flags plus saved session/finding ids, output and workflow enums, configured profiles, local Git refs and recent commits, relevant files/directories, and repository-relative `inspect --path` values. Bash-completion discovers its user directory automatically. For Zsh, add `${XDG_DATA_HOME:-$HOME/.local/share}/zsh/site-functions` to `fpath` before running `compinit` if it is not already present.

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

  The command must be the comment's first non-blank line; anything after it on that line is ignored, so `/nickpit review please` works like `/nickpit review`. A bare `/nickpit` replies with the command list. A command quoted further down a comment is deliberately not executed.

Reactions track the review: when it starts, the daemon awards a start emoji on the MR (default `:eyes:`, `start_emoji: ""` disables), and a `/nickpit review` comment gets the same acknowledgement on the comment itself (default `:eyes:`, `ack_emoji: ""` disables). When the review ends, both are **replaced** by its outcome — `done_emoji` (default `:white_check_mark:`) once it landed, `fail_emoji` (default `:x:`) when it could not be delivered (a failed run, or an MR that turned out not to be reviewable, e.g. closed meanwhile). An aborted review only loses the in-progress reaction: nothing went wrong. Set an outcome emoji to `""` to only revoke instead. Because the outcome replaces the in-progress reaction, `start_emoji: ""` leaves the MR undecorated end to end. `/nickpit abort` is acknowledged with `abort_emoji` (default `:stop_button:`), and `status`, `help`, and `abort` also get a comment reply, threaded under the command.

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
  state-init:
    image: busybox:1.37
    user: "0:0"
    command: ["sh", "-c", "mkdir -p /state/journal && chown 65532:65532 /state/journal && chmod 0700 /state/journal"]
    volumes:
      - nickpit-state:/state

  nickpit:
    image: nickpit
    command: ["gitlab", "serve"]
    depends_on:
      state-init:
        condition: service_completed_successfully
    ports:
      - "8080:8080"
    volumes:
      - ./nickpit.yaml:/work/.nickpit.yaml:ro
      - ./server.yaml:/work/server.yaml:ro
      - nickpit-logs:/work/logs
      - nickpit-state:/work/state    # use state_dir: "/work/state/journal"
    environment:
      OPENROUTER_API_KEY: "..."
      NICKPIT_GL_TOKEN_PLATFORM: "..."
      NICKPIT_GL_SECRET_PLATFORM: "..."
volumes:
  nickpit-logs:
  nickpit-state:
```

Per-review child logs land in `log_dir` (default `logs/`) as `review-<project>-<iid>-<timestamp>.log`; `GET /healthz` reports queue depth. On SIGTERM the daemon stops accepting events and lets running reviews finish within `shutdown_grace` (default `10m`) before terminating them — an interrupted publish heals on the next run via the comment fingerprints.

The queue lives in memory, with an optional on-disk journal: set `state_dir` (or `--state-dir`) and every accepted-but-unfinished review job is persisted as a small JSON file (no tokens — groups are re-resolved from the config) and resumed at the next start, so a restart or upgrade neither loses queued reviews nor strands the in-progress reaction on acknowledged command comments. The state directory must be owned by the daemon user and must not be writable by group or other users; NickPit creates a missing directory with mode `0700` and rejects an existing shared-writable directory. Docker named-volume roots start as `root`, so the Compose init service above creates a private UID-65532-owned child for `state_dir`. The journal survives exactly as long as its directory does — put it on durable storage (a volume, a PVC) to cover pod replacement. Without a `state_dir`, queued jobs release their ack reactions at shutdown and are lost; events arriving while the daemon is down are recovered by awarding the trigger emoji (or the review command) again.

## Tuning a Review

### Progress

Append `--show-progress` to print review details and tool calls on stderr.

Every run names the build it came from: a `NickPit <version>` line under `--show-progress`/`--show-reasoning`, a `nickpit: version <version>` line under `--verbose`, the version beside the wordmark in the live dashboard header (and in its frozen final line), and a `nickpit serve starting` log entry for the daemon.

The review itself carries the version too, stamped once when the review completes: a `NickPit: <version>` footer line above runtime and tokens in rendered and raw Markdown output, `nickpit_version` in JSON, and a field in the hidden gzipped review envelope published to GitLab/GitHub. It travels with the saved session, so `nickpit session` and a chat reassembled from MR/PR markers report the build that produced the review — not the one reading it.

A released build reports its tag (`v0.0.14`) — the tag already pins the commit. Any other build appends the short commit (`dev+995c910`, or `dev+995c910-dirty` when the tree had uncommitted changes), taken from `-ldflags "-X main.commit=..."` when set and otherwise from the revision Go embeds. `make build` stamps it explicitly, because the embedded fallback is missing exactly where it is most wanted: Go's VCS detection expects `.git` to be a directory, so a build from a linked git worktree would report a bare `dev`. Container images pass the tag and SHA as `VERSION`/`COMMIT` build args, because the build context excludes `.git`. Override either with `make build VERSION=v0.1.0 COMMIT=abc1234`.

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

Each JSON tool result sent to the model is capped at `10%` of the context tokens remaining when that result is appended. Parallel results share the remainder in order, so later results cannot each claim the original allowance. When the window is already full, results still get a small readable floor, bounded across the batch so a wide parallel batch cannot overshoot the window by the sum of its floors; once that budget is spent, the remaining results are returned as a self-describing truncation note. Configure this with `--max-tool-result-percent`, `max_tool_result_percent`, or `NICKPIT_MAX_TOOL_RESULT_PERCENT`; `0` disables only the context cap, while tool-specific limits such as hierarchy depth and commit/result counts remain. Capped payloads stay valid JSON and report `truncated: true` plus a `truncated_note`; narrow the request when more detail is needed. Human-facing `nickpit inspect` output is not capped by this setting.

### Finding Caps

Cap how many findings each review agent may report with `--max-findings` or `max_findings` in config (also overridable per `review:` step in a workflow). The default is `0`, meaning unlimited. When a limit is set it is added to the reviewer prompt; a response exceeding the limit is retried once with guidance to keep only the strongest findings, after which the weakest findings (lowest priority, then lowest confidence) are cut and the agent run is marked partial. The limit counts the agent's whole session: initial pass plus nudge rounds; once the limit is reached, remaining nudge rounds (including standalone `nudge:` steps) are skipped.

### Reasoning Caps and Loop Detection

Reasoning calls are capped with `--max-reasoning-seconds` or `max_reasoning_seconds`; the default is `300`. When the cap is hit, NickPit aborts the stream and retries through the existing lower-reasoning-effort fallback path. NickPit also watches streamed reasoning for loops; detection is built in and needs no configuration. Three layered signals cover the observed failure modes: degenerate character runs (one character or a short unit repeated back-to-back), exact repeated lines or blocks (whitespace runs collapsed, empty lines ignored), and shingle recurrence — the fraction of recently emitted token shingles (lowercased, punctuation removed, code identifiers and numbers masked) that already appeared earlier in the same stream. Verbatim loops drive that recurrence to ~1.0 and are cancelled quickly; paraphrase loops (the same decision cycle reworded) plateau lower and must persist longer before they fire. Thresholds are staged over the reasoning time budget: early in a stream only ironclad repetition may cancel it, and detection becomes progressively more aggressive as the stream approaches `max_reasoning_seconds`, where it would be cancelled anyway. When a loop is detected, the stream is aborted and retried with lower reasoning effort and an added instruction to avoid repeating the same analysis.

### Concurrency and Accounting

Reviews run a context agent first, then six specialist reviewer lanes in parallel: Code Quality, Security, Architecture, Performance, Testing, and Best Practices. Each lane categorizes, verifies, and de-duplicates its reviewer's findings as soon as that reviewer finishes, so only clean findings reach the merge agent. Concurrent LLM agent loops — reviewers, categorizers, verifiers, dedupe, merge, finalize, verdict, summarize — are capped globally with `--concurrency` (default `10`, `0` = unlimited). The classifier is tool-free; tool-call limits apply independently to tool-enabled context, reviewer, verifier, and discussion agents. JSON output includes `total_tool_calls` at the root plus an `agent_runs` summary for workflow agents. Internal per-finding verifier tool calls contribute to `total_tool_calls` even though those verifier calls are represented by phase totals rather than individual `agent_runs` entries.

Token accounting in the JSON output works as follows:
- `tokens_used` at root is the grand total for the whole run (including retried calls)
- `categorize_tokens_used` is a breakdown of tokens used by the categorize agents (the first pass of verification)
- `verify_tokens_used` is a breakdown of tokens used by the verifier agents
- `finalize_tokens_used` is a breakdown of tokens used by the finalizer agents
- `verdict_tokens_used` is a breakdown of tokens used by the verdict agent
- `summarize_tokens_used` is a breakdown of tokens used by the summarizer agent
- `agent_runs` entries each carry their own `tokens_used` breakdown per `role`:
  - `context` — the context-gathering agent that scouts the change before the reviewer lanes
  - `review` — a reviewer lane's whole session: initial pass, all nudge rounds, and reasoning-extraction
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
nickpit git branch --step merge --findings reviewer_a.json --findings reviewer_b.json --output json
nickpit git branch --step finalize --findings merged.json --output json
nickpit git branch --step verdict --findings finalized.json --output json
nickpit git branch --step summarize --findings finalized.json --output json
```

See [`workflow.yaml.example`](workflow.yaml.example) for the full format. A spec lists `steps` (optionally grouped under `parallel:` to run concurrently); a parallel child can be a `lane:` — a list of steps that run sequentially within the group, e.g. one reviewer's `review:` → `verify:` → `dedupe:` chain. A `pipeline:` group is the explicit, streamed post-review tail (`merge` → `finalize` → `verdict`, optionally `summarize`): its steps overlap with no barrier between them — each merge cluster flows straight into finalize/summarize while other clusters are still merging, and verdict gates on all finalizes. Listing those steps flat instead runs them strictly sequentially, each over the whole finding set. Each step may carry a `config:` block overriding any model parameter or budget for that step only (model, temperature, top_p, top_k, presence_penalty, reasoning_effort, scope, max_tool_calls, max_output_retries, nudge_count, max_findings, disable_patch_summary, disable_suggestions, verify_drop_policy, confidence_threshold, …) — anything unset inherits the active profile/flags. `scope` makes a step's fan-out explicit — the work unit each agent operates on: `all` (whole finding set), `cluster` (per merge cluster, `merge`/`finalize`/`summarize`), `finding` (per finding, `verify`), or `reviewer` (per reviewer group, `dedupe`); cluster-scoped finalize/summarize is valid only inside a `pipeline:`. Use `model: "@small"` to select the configured nested `small` profile for a step. `review:<vector>` configs can also override internal agents with `mine_reasoning:`, `compile_findings:`, and `nudge:` subconfigs; `verify` / `verify:<vector>` configs take a `categorize:` subconfig for the classifier that runs before evidence verification (same keys minus `time_budget`, since classification and verification share the verify step's budget — the default workflow runs it on `model: "@small"`). `dedupe` / `dedupe:<vector>` / `merge` configs take a `context:` subconfig that trims what their prompt carries — those two stages judge findings against each other and receive the review context to ground that call. Its keys (`styleguides`, `diff`, `commits`, `comments`, `toolchain`) all default to `true`; set one to `false` to buy back its tokens. `styleguides` drops the styleguide rules from the system prompt, `diff` the patch itself, `commits` the commit summaries, `comments` the MR/PR discussion, and `toolchain` both `toolchain_versions` and the instruction to consult it. `toolchain: false` only trims the prompt — the detected toolchain still selects the styleguide version, so a Go 1.25 change keeps the Go 1.25 guide rather than falling back to the generic one. The changed-file list and the context agent's supplemental files are always sent. The default workflow sets all five to `true` explicitly. A `verify` step privately classifies each finding before evidence verification. The classifier sees only the submitted finding and toolchain versions; it does not see routing outcomes, diff scope, tools, or verifier categories. Go maps the descriptive categories through `verify_drop_policy`, then deterministically requires an exact old- or new-side diff overlap. It relocates an anchor only when its submitted content has one unambiguous in-diff match; otherwise a live reviewer gets one response-validation retry whose allowed ranges are complete `code_location` JSON objects. Injected findings have no originating reviewer call to retry; their deterministic relocation requires retrieval and a repository root, and unresolved anchors are dropped with an aggregate warning. An unresolved out-of-diff finding is always dropped, even under policy `none`; `--disable-diff-scope` is the only switch for that. Classification failures fail open so they cannot silently discard a real finding. LLM concurrency is run-level only (`--concurrency`, default `10`, `0` = unlimited): one shared cap across every agent loop in the run — it is intentionally not in the spec. Per-vector steps are addressed as `review:security`, `verify:security`, `dedupe:security`, …; `nudge:<vector>` / `reasoning-extract:<vector>` let you drive extra rounds manually. Any global step can take `findings_from:` to inject previously-emitted findings JSON (the same format `--output json` produces; one file = one merge group); inside a `pipeline:` only the `merge` step may carry it. Steps that only consume injected findings (e.g. `merge`, `finalize`, `verdict`, `summarize`) run without a git/PR source. `finalize` now only finalizes finding wording/priority/confidence; include `verdict` after it when a workflow needs final top-level `overall_correctness`, `overall_explanation`, and `overall_confidence_score`. `confidence_threshold` is applied only by the `verdict` step.

## Filtering Review Output

Review output filtering uses `--priority-threshold` with `0` through `3`, where `0` is highest priority and `3` is lowest (the default, showing everything). Findings are still displayed with `p0`–`p3` badges. `--confidence-threshold` filters findings at the start of the `verdict` step using finalized confidence; workflows without `verdict` do not apply the confidence threshold and emit a warning.

## Inspect Commands

The `inspect` command is a standalone retrieval command tree for using retrieval without review.

```bash
nickpit inspect file --path internal/review/engine.go
nickpit inspect file --path internal/review/engine.go --line-start 1 --line-end 80
nickpit inspect list --path internal/review
nickpit inspect search --path internal/review --query inspect_file
nickpit inspect references --symbol DefaultListFilesDepth
nickpit inspect references --path internal/tools/catalog.go --symbol DefaultListFilesDepth --output json
nickpit inspect references --path internal/review/tool_result_limit.go --symbol toolResultTruncatedNote --line 15
nickpit inspect callers --symbol Run --depth 2
nickpit inspect callers --path internal/review --symbol Run --depth 2
nickpit inspect callers --path internal/review/engine.go --symbol Run --depth 2
nickpit inspect callees --path internal/review/engine.go --symbol Run --depth 3
nickpit inspect search --path internal/review --query inspect_file --context-lines 3 --max-results 5 --output json
nickpit inspect callers --path internal/review/engine.go --symbol Run --depth 2 --output json
nickpit inspect log --limit 5
nickpit inspect log --author Ada --since "2 weeks ago" --paths internal/review,cmd/nickpit
nickpit inspect log --message '^feat\(.*serve' --message-regex --limit 3
nickpit inspect show --commit dc80d0c
nickpit inspect show --commit dc80d0c..a44f11c --max-commits 5 --output json
nickpit inspect show --commit a44f11c --paths internal/serve --diff-format git-json
```

`inspect log` mirrors the `git_log` tool and `inspect show` mirrors `git_show` (aliases: `commits` and `commit`). Both accept a commit SHA abbreviated to any length, a ref, or a range (`a..b`); `inspect log` lists commit metadata plus each commit's changed files without diff content, while `inspect show` returns one diff per commit in the configured `diff_format`. `--since`/`--until` filter on the **commit** date, matching git — a rebased or cherry-picked commit is selected by when it was rewritten, not authored; results carry both `date` (author) and `commit_date`. `--paths` narrows each diff, never which commits are returned: a commit that touched none of the paths still appears with an empty diff and a note. Path filters are always repository-relative, so both commands work the same from any subdirectory. Merge commits are shown as a combined diff, falling back to the diff against the first parent when the combined diff is empty (`diff_mode` reports which). A shallow checkout — what remote PR/MR reviews clone — is deepened once on first use; when that is refused, the result reports `shallow` with a note instead of silently listing a single commit.

Every patch nickpit reads carries exactly **3 lines of context**, and that is not configurable. git's own default is also 3, but `diff.context` in your global git configuration or in the reviewed repository would otherwise override it — and hunk windows are what the diff-scope gate, the finding fingerprints and the inline-comment positions are derived from, so the same review would produce different results on different machines. 3 is also what the GitHub and GitLab diff APIs serve (neither exposes a context parameter at all), which keeps a local review and a remote review of the same change comparable. For the same reason `color.ui`, `diff.external`, gitattributes `textconv` filters, `diff.noprefix` and `diff.mnemonicPrefix` are all neutralized per invocation. When a hunk is too narrow to judge, use `inspect file` with `--line-start`/`--line-end` or the `inspect_file` tool rather than widening the diff.

Retrieval supports `go`, `python`, `nodejs` (including `.jsx`/`.tsx`), and `rust` source files. `inspect file`, `inspect list`, and `inspect search` work generically across text files. `inspect references` resolves a named variable, constant, parameter, field, import, type, function, or similar binding; it returns the definition, each whole function containing a use, and package/module/class-level reads or writes. Only `--symbol` is required. `--path` optionally identifies the declaration, but references are always collected across the whole repository. Go references use type-checked object identity; dynamic-language matches that cannot be proven are retained and marked as possible. `inspect callers` and `inspect callees` use language-aware call-hierarchy analysis: Go is resolved with the type checker (`go/packages`), TypeScript/JavaScript with esbuild's parser, and Python/Rust with a pure-Go tree-sitter runtime — all CGo-free, so the single static binary stays self-contained.

## Notes

- The CLI expects an OpenAI-compatible `/chat/completions` endpoint.
- Remote reviews clone the requested PR/MR head into a temporary checkout when retrieval needs local files.
- Use `--workdir` (or `workdir` in config / the `NICKPIT_WORKDIR` env var) to reuse an existing clone for remote reviews; NickPit creates a temporary worktree at the requested revision instead of cloning again.
- Note the workdir asymmetry: for local (`nickpit git ...`) reviews only the `--workdir` CLI flag changes the directory the review runs in; `workdir` from config or `NICKPIT_WORKDIR` applies only to where remote PR/MR checkouts (clones/worktrees) are placed.
