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
- **GitHub PRs and GitLab MRs** via direct REST clients — by `--repo`/`--id`, by URL, or [picked from a list](#pick-the-mrpr-branch-or-commit-interactively-) in a checkout of the repository.
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

Model and provider settings have their own variables: `NICKPIT_MODEL`, `NICKPIT_BASE_URL`, `NICKPIT_API_KEY`, `NICKPIT_REASONING_EFFORT`, the sampling knobs and their `NICKPIT_SMALL_*` counterparts — including `NICKPIT_SMALL_BASE_URL` and `NICKPIT_SMALL_API_KEY` (see [The `small` model alias](#the-small-model-alias)) — plus `NICKPIT_WORKDIR`, `NICKPIT_GITHUB_TOKEN`, `NICKPIT_GITLAB_TOKEN`, `NICKPIT_GITLAB_BASE_URL`, and `NICKPIT_CACHE_DIR`.

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
    min_p: 0.0
    presence_penalty: 0.1
    repetition_penalty: 1.0
    reasoning_effort: high
    small:
      model: small-model
      max_tokens: 2048
      temperature: 0.2
      top_p: 0.9
      top_k: 40
      min_p: 0.0
      presence_penalty: 0.1
      repetition_penalty: 1.0
      extra_body: {}
      reasoning_effort: low
```

`model: "@small"` in workflow step config selects the nested `small` config. Any unset small field falls back to the primary profile value. Small model settings can also be set with `NICKPIT_SMALL_*` environment variables or `--small-*` flags such as `--small-model`, `--small-reasoning-effort`, `--small-top-k`, `--small-min-p`, `--small-presence-penalty`, `--small-repetition-penalty`, and `--small-max-output-tokens`. The primary model has the same environment variables without the `SMALL_` part: `NICKPIT_MODEL`, `NICKPIT_REASONING_EFFORT`, `NICKPIT_MAX_TOKENS`, `NICKPIT_TEMPERATURE`, `NICKPIT_TOP_P`, `NICKPIT_TOP_K`, `NICKPIT_MIN_P`, `NICKPIT_PRESENCE_PENALTY`, `NICKPIT_REPETITION_PENALTY`, and `NICKPIT_EXTRA_BODY`.

#### A small model on its own endpoint

The `small` block may also carry `base_url` and `api_key` — put the big model on a local runtime and the cheap steps on a hosted provider:

```yaml
profiles:
  custom:
    model: Qwen3.8-27B-NVFP4
    base_url: http://localhost:10000/v1
    api_key: ${NICKPIT_CUSTOM_API_KEY}
    small:
      model: Qwen3.6-35B-A3B-FP8
      base_url: https://llm.aihosting.mittwald.de/v1
      api_key: ${MITTWALD_LLM_API_KEY}
      reasoning_effort: none
```

Every `@small` step then runs against the second endpoint with its own client, its own rate-limit backoff and its own model check (capabilities are cached per endpoint, so both are probed separately). They can also be set with `NICKPIT_SMALL_BASE_URL` / `NICKPIT_SMALL_API_KEY` or `--small-base-url` / `--small-api-key`, which override a configured value like every other `NICKPIT_SMALL_*` variable.

The one exception to "any unset small field falls back to the primary value": when `small.base_url` differs from the profile's `base_url`, `small.api_key` is **required**. Inheriting the primary key would send it to another provider, so NickPit fails the run with a config error instead. A `small.base_url` equal to the primary one (a trailing slash makes no difference) keeps inheriting the primary key, and `small.api_key` alone — same host, different credential — is allowed too.

Three things to know when the two endpoints are different providers:

- The `json_schema` response format is a single run-wide request setting. A small model that cannot do API-enforced `json_schema` degrades the primary model to the prompt-embedded schema as well.
- The profile's `supported_models` describes the primary serving stack and is matched by model name only, so it is ignored for a differing small endpoint — that model is probed instead. Declare a second profile if you want pre-declared capabilities for it.
- `--concurrency` remains one shared cap across both endpoints; only the rate-limit backoff is per endpoint.
- Only `model: "@small"` selects the second endpoint. A nested `mine_reasoning:` / `compile_findings:` / `nudge:` / `categorize:` override that names a concrete model inherits its step's endpoint like every other unset value, so inside an `@small` step that model is sent to the small provider — give it its own step if only the primary provider serves it.

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
# Review a branch pair — on a terminal both refs are picked from a list,
# preselected as default branch → current branch; non-interactively that pair is used as is
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

# Pick one of the repository's open MRs/PRs instead of naming it (in a checkout, on a terminal)
nickpit gitlab mr
nickpit github pr

# Pick the commit range from the log (--from is required, so it is asked for)
nickpit git commits

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

# Read a published review back from the PR/MR — print it, or copy it to the clipboard
nickpit github feedback --url https://github.com/owner/repo/pull/123
nickpit gitlab feedback --url https://gitlab.example.com/group/project/-/merge_requests/456 --clipboard
```

### Pick the MR/PR, Branch or Commit Interactively 🎯

Every command that addresses a merge request or pull request — `gitlab mr`, `github pr`, `gitlab feedback`, `github feedback`, `chat --gitlab` — can pick it from a list instead of taking `--repo`/`--id`/`--url`. In a checkout of the repository, on a terminal, just leave the identifier out:

```bash
# Open MRs of the project the origin remote points at, newest activity first
nickpit gitlab mr

# Same for GitHub, and for the read-back and chat commands
nickpit github pr
nickpit gitlab feedback
nickpit chat --gitlab

# Force the list even where a target could be resolved without it
nickpit gitlab mr --select
```

The project comes from `--repo` or, without it, from the `origin` remote of the current directory — which is why this works only inside a checkout. The list shows the open requests (drafts included and labeled), and the one whose source branch is the checked-out branch is starred `★` and preselected, so the common case is one keypress. Typing filters the list, and it matches the branch names too even though they are not shown. A merged or closed request is not listed; address those with `--id` or `--url` as before.

Local reviews pick refs the same way. `nickpit git branch` always asks on a terminal — the head prompt opens on the newest branch when the base is the branch you are on, since reviewing a branch against itself is an empty diff — with one row per branch — a local branch and its remote-tracking refs (`main`, `origin/main`, `origin`'s HEAD alias) are folded into one, and a branch that exists only on a remote is named by that ref — with the checked-out branch starred `★` in its own column and the command's own defaults preselected — the default branch as the base, the checked-out branch as the head — so accepting both prompts reproduces exactly what the command does without a terminal. A folded row resolves to the side the prompt is for: the base takes the remote-tracking ref (what `--base main` resolves to anyway), the head the local branch. That holds on the default branch too, where the pair is `origin/main..main`: the commits not pushed yet. `nickpit git commits` asks for the commits to review, since `--from` is required and has nothing to fall back to: one list, where the first `Enter` opens a range on the commit under the cursor, moving covers the commits between (the whole span is highlighted and the title line counts it — `Commits to review: 3 commits · d077dba..82a6dbd`), and the second `Enter` takes it. `Esc` closes an open range before it leaves the list. The range is stated in the commits you want reviewed; the exclusive `--from` git needs is the parent of the oldest of them, and the repository's very first commit is diffed against the empty tree instead of failing. With an explicit `--to` only the other end is open, so the prompt asks for the first commit to review out of that head's history. Selectors given on the command line are kept, so `nickpit git branch --base main` only asks for the head branch.

Keys: `↑`/`↓` (or `Ctrl-P`/`Ctrl-N`) move, `PgUp`/`PgDn` page, `Home`/`End` jump, typing filters (`Ctrl-U` clears the filter), `Enter` selects, and `Esc`/`Ctrl-C` aborts without running anything (exit code 130). A list that has several scopes — [`nickpit session`](#discuss-a-review-chat-) — switches between them with `Tab`/`Shift-Tab` or `←`/`→`, keeping the typed filter and staying on the same row where the new scope also holds it; the scopes sit in the position line below the rows, the current one in brackets. The list is drawn on stderr, so the review output on stdout stays exactly what it is with `--id`.

Rows are coloured like the progress lines of the same run, in the palette `tools/print_colors.sh` documents: identifiers green, messages in a pale lavender, a stable colour per person so a list can be scanned by who wrote what, ages grey and green while still inside the hour. Two rules paint inside a cell, the same ones the `git-color` dev helper applies to a `git log`: a conventional-commit prefix is taken apart (`feat` green, `(scope)` turquoise, the punctuation and a `!` for a breaking change stepping out of the text), and the `/` separators of a branch name fade back the way a progress line renders `head → base`. In a branch list green is reserved for the default branch. The selected row carries a highlight bar — the live dashboard's lavender pastel, dimmed the way a progress bar dims its unfilled half — and the title line spells out its full value after the prompt — `Base to review against: origin/feat/tree-sitter-parse-cap-and-cache` — so the name column is free to be the first thing shortened when the row does not fit, and the tip message keeps the room. The two branch prompts wear the aqua green and gold a progress line paints `base → head` in, so `Base to review against:` and `Branch to review:` are never confused. The pick is confirmed on stderr in italic grey with the chosen ref in that same colour, and `NO_COLOR` keeps every layout while dropping the colour.

Nothing changes without a terminal: piped, redirected and daemon-spawned runs keep the non-interactive behaviour — the same "`--id` must be a positive integer" error for a request, the command's default refs for a local review — instead of waiting for a keypress, and `--select` there fails immediately.

### Publishing

With `--publish`, findings whose lines are part of the diff are posted inline anchored to those lines; the rest fall back to general comments that include `file:line` after the priority badge. Confidence scores are not rendered in the terminal output or in published comments — they remain in `--output json` and in the hidden review envelope. On GitHub this is a single PR review (the summary as the review body, findings as inline review comments); on GitLab it is a summary note plus one inline discussion per finding. Hidden markers make re-runs idempotent (already-posted comments are skipped), and a publish failure is reported as a warning without failing the review.

Known limitation: the hidden fingerprint markers are read from all existing PR/MR comments regardless of who wrote them. Anyone who can comment on the PR/MR can therefore forge a marker and suppress a matching finding from being posted on the next run.

### Reading a Published Review Back 📋

`nickpit github feedback` and `nickpit gitlab feedback` print a review NickPit already published on a PR/MR — reassembled from the same hidden markers a chat uses, so there is no re-review, no LLM call, and no local session needed. That is how feedback posted by the [serve daemon](#gitlab-webhook-daemon) or from another machine gets onto your terminal, and with `--clipboard` into an editor or coding agent.

The request is selected exactly as in the review commands: `--url`, `--repo` plus `--id`, or [interactively](#pick-the-mrpr-branch-or-commit-interactively-) (omit `--id` in a checkout, or pass `--select`). Output uses the normal review formats (`-o markdown|json|raw`), and `--clipboard` copies instead of printing, with the same helper chain and unstyled payload as [`nickpit session --clipboard`](#discuss-a-review-chat-). The command is read-only — nothing is posted or changed on the PR/MR.

When a request carries several reviews the newest is printed; `--list` shows them all (newest first, with publish time, revision, finding count, verdict, model and NickPit version) and `--review-id` picks one. Only markers in comments authored by the token's own user are trusted, so a marker planted by another commenter is ignored — on GitHub this needs a token whose `/user` resolves, which rules out a GitHub App installation token.

```bash
# Print the newest review published on a PR/MR
nickpit github feedback --repo owner/repo --id 123
nickpit gitlab feedback --repo group/project --id 456

# Copy it to the clipboard instead of printing it
nickpit gitlab feedback --repo group/project --id 456 --clipboard

# List the reviews on the MR, then print a specific one
nickpit gitlab feedback --repo group/project --id 456 --list
nickpit gitlab feedback --repo group/project --id 456 --review-id <review-id>

# Machine-readable, e.g. to hand the findings to another tool
nickpit github feedback --repo owner/repo --id 123 --output json
```

## Discuss a Review (Chat) 💬

After a review you can talk to an agent about it. The discussion agent gets the same context a reviewer/verifier has — the diff, the toolchain, the applicable styleguides, and the same retrieval tools — plus the **complete findings JSON and the overall verdict**. It is free-form: no workflow, no output schema, no priority gates. Ask why a finding is a bug, push back on a nitpick, or propose a fix and have it evaluated.

In GitLab, point out a mistake or a fix in a review thread. NickPit can schedule an update, check it against the current code, and follow up in the same thread. Findings and the overall verdict are updated when the evidence supports a correction. Resolved findings show a badge and a short explanation; previous versions stay in a collapsible history section.

CLI chat supports the same evidence-based corrections. One update can run in the background per session while you continue the conversation. The update uses the conversation through the question that requested it; later messages apply to later requests. Completion appears in the terminal and becomes part of the saved conversation. One-shot chat, `/exit`, and EOF wait for pending work; Ctrl+C cancels it. There is no detached worker or daemon requirement.

For GitLab-backed sessions, corrections also update the existing published review, preserving GitLab's revision history and recovery behavior. The terminal conversation is not posted to the MR. Local and GitHub-backed sessions update their saved review only. `--no-session` keeps changes in memory (GitLab publication still applies), and imported JSON files are never overwritten. Updates require tools to be enabled; `max_tool_calls: -1` disables the update tool too.

CLI corrections keep previous review versions in session history. Use `nickpit session --history` to print archived versions, oldest first, with their replacement time and correction reason; `--output json`, `--output raw`, and `--clipboard` work with history too. Default session output shows the current review and marks resolved findings with their resolution reason. History covers versions observed by that session, not older GitLab comment history. A failed session save is reported separately from a successful GitLab publication.

A new commit alone doesn't trigger an update.

An 👀 reaction on NickPit's reply means an update is pending. It disappears when NickPit follows up.

Corrections use the initiating discussion and linked threads for the requested findings, through the initiating question. An overall-review correction uses its initiating discussion. Later replies belong to later requests; unrelated MR comments do not invalidate an update. Live response controls still apply.

Update jobs run in strict order within each MR, including retry waits, response-policy blocks, and follow-up delivery. Different MRs can run concurrently: `chat.update_max_concurrent` defaults to 2, independently of normal chat capacity; set it to 1 for serial updates. Same-MR coordination across daemon/manual processes requires a shared state directory and host lock filesystem; it is not distributed coordination across hosts.

Execution failures have three attempts; changes to selected evidence, review state, or commits have a separate five-conflict limit. Retries rebuild against the current review. Activated publication transactions are recovered before a job can be retired, including when the original staging response was lost.

Every review automatically saves a resumable session — including the exact prepared context the reviewers saw — so chatting needs no re-fetch (disable with `--no-session`). A review that found nothing is saved too, so "why did you find nothing here?" stays answerable. Session files live under `$NICKPIT_CACHE_DIR/sessions` (or `<user cache>/nickpit/sessions`); override with `--session-dir`. The store keeps every session by default; cap it with `--max-sessions` or `max_sessions` in config (`0` = unlimited) and each save deletes the oldest files beyond the cap. Resuming a GitLab session checks the MR's live head and recreates the diff when new commits landed. For remote sessions the retrieval tools read from a temporary checkout of the live head, cloned automatically for the duration of the chat (the same mechanism reviews use) and removed when it ends; pass `--repo-root <checkout>` to use a local checkout instead (full history, local edits). Code-reading tools stay off only when tools are disabled (`max_tool_calls: -1`) or the checkout cannot be prepared.

```bash
# Chat about the most recent review (interactive REPL)
nickpit chat

# One-shot question about a specific finding, then exit
nickpit chat --finding <finding-id> "why is this actually a bug?"

# Resume a specific session
nickpit chat --session <session-id>

# Print the review stored in a previous session (pick one from a list when omitted)
nickpit session [session-id]
nickpit session --session <session-id> --output json

# Inspect previous review versions after chat corrections
nickpit session [session-id] --history
nickpit session [session-id] --history --output json

# Copy that review to the system clipboard instead of printing it
nickpit session [session-id] --clipboard

# Print only the warnings the run recorded, not the review
nickpit session [session-id] --warnings

# In the picker's remote scope, list merged and closed requests too
nickpit session --include-closed

# Start a chat from a saved review JSON (e.g. a CI artifact from `--output json`)
nickpit chat --from-json review.json

# Start a chat from a GitLab MR — findings are reassembled from the review NickPit posted
nickpit chat --gitlab --url https://gitlab.example.com/group/project/-/merge_requests/456

# ... or pick the MR from the project's open ones (in a checkout, on a terminal)
nickpit chat --gitlab
```

Pin the chat to one finding with `--finding <id>` and the agent opens by pointing at it; omit it to discuss the whole review. On GitLab, the review NickPit publishes now embeds the full findings JSON (and the overall verdict) as hidden, gzip-compressed markers in the notes, each tagged with a review id and timestamp, so a later chat can regroup them into the exact (newest) review — no local state needed. Because the markers are encoded but not cryptographically signed, only markers in notes authored by the chat token's own user (the bot that published the review) are trusted; markers planted by other commenters are ignored. When an MR carries several reviews, the newest is chosen (`--review-id` overrides). The retrieval tools read from an automatic temporary checkout of the MR head (or a local checkout: `--repo-root`, or the current directory for local sessions) — this includes the daemon's in-thread replies, which answer with code-reading tools enabled.

`nickpit session` without a session id asks which one to print. In a checkout the list opens on the sessions of that repository, and `Tab` (or `←`/`→`) switches the scope: the sessions of the checked-out branch, the sessions of this repository, the reviews published on the project's open requests, all saved sessions. The line under the rows carries all of it — `1 of 65 · branch · [repository] · remote · all · filter: dd` — with the filter named only while one is typed. Outside a checkout there is one scope: all sessions.

A fourth scope, `remote`, lists what is not saved here at all: the reviews NickPit published on the project's **open merge or pull requests** — by the serve daemon, by `--publish`, by a colleague — reassembled from their hidden markers. One row per review, so a re-reviewed request contributes several, each collapsed onto its newest revision; the requests on the checked-out branch lead, the rest follow newest first. The other scopes never show them, and nothing is fetched until this scope is drawn — the status line says `reading published reviews…` while it is. `--include-closed` widens the scope past the open requests to the merged and closed ones — the review published on a request that has since been merged is still the review of that work. Unlike `nickpit gitlab feedback`, this scope reads the markers **whoever published them**: a project reviewed by another group's bot, or by a colleague's token, carries markers no local token can claim, and requiring one's own would show nothing. Markers are encoded but not signed, so a review listed here can in principle have been forged by anyone who can comment on the request — it is read-only either way, and publishing and correcting keep the strict author check. The platform comes from the `origin` remote (github.com means GitHub, anything else the profile's GitLab, and only when its host matches the one the token belongs to), at most 15 requests are read, newest activity first, and a server that does not answer within 8 seconds leaves the rows it did answer with plus a note. Without a project, without a token, or outside a checkout the scope simply says so.

A session belongs to the repository when it names the same project, or when it ran in a checkout of it. The second test is on the repository itself (git's shared directory), not on the path, so every worktree of a clone and every subdirectory a review was started from count as one repository — a review records the directory it ran in. A worktree that has since been deleted is claimed by the folder this repository's checkouts live in, so removing a feature branch's worktree does not hide its sessions. It belongs to the branch when it recorded that branch, and those sessions are starred `★` in the wider scopes too.

The rows show the short session id, the project, what was reviewed and how the review ended. The project column is capped at 24 cells and gives up its path before its name: `asylum/services/archiefmeester` becomes `asylum/…/archiefmeester`, then `…/archiefmeester`, and only a name still too long for that is cut in its own middle (`…/arc…ter`), where both ends survive to tell projects apart. What was reviewed is `GitLab MR !42` / `GitHub PR #7` for a request; the branch for a local branch review, with `(uncommitted)` in grey after it when the review was of the working tree rather than of commits; the commit range (`01234567..fedcba98`) for a `git commits` review; and `<unknown>` for a session that recorded none of those, followed by where its checkout sits among this repository's others (`<unknown> feat/x`, or the path of one that is gone) unless that is just the repository's own folder again. The branch of an older working-tree review is recovered from its cached context, which recorded it as the base of the diff. How the review ended is its verdict and how many findings it holds — `Correct, 0 findings`, `Incorrect, 3 findings`, `No verdict, 2 findings` for a workflow without a verdict step. Both columns are coloured by kind: each kind of review wears its own hue (GitLab MR, GitHub PR, branch, commit range, working tree, and the grey of a review that can only be placed by its directory), and the verdict wears the green and red the review output badges it in, a caveat colour for a verdict that is neither and grey where none was recorded. The finding count never enters that choice. The last column is the age. The "what was reviewed" column is capped at 40 cells — the whole cell, qualifier included, so the column is never wider than what it shows and the next one starts two spaces after its widest row — and shortened by its parts rather than from the end: a `(uncommitted)` qualifier is never what gets cut (the branch pays for it), and the two ends of a range share what is left, an end that fits keeping its full width while two long ones level down together. The header above the rows names the session the cursor is on and nothing else — `Session · <full id> · <directory the review ran in>` — the id in the colour its column wears, the directory in the project column's, which is what `--session` is copied from. Which scope is on screen is said by the status line, not the header. Typing filters on what a row is: its full id (from four characters up — a shorter term searches everything else instead, since two characters of a uuid hit a third of the store by accident), the project and the directory it belongs to (both in full, whatever the columns had to fold away), what was reviewed, the verdict word, and the number of findings — so `incorrect` narrows the list to the reviews that found the patch wrong (the finding text does not drag in the ones that merely mention it) and `incorrect 12` to those with twelve of them. Terms narrow each other across fields, so `feat 3f1a` is the session whose branch and id each carry their part. The word `findings` is deliberately not part of it: every row carries it, so typing it would narrow nothing — while a branch or project of that name still matches. The keys are the ones every NickPit picker uses (see [Pick the MR/PR, Branch or Commit Interactively](#pick-the-mrpr-branch-or-commit-interactively-)), and `Esc` in the list leaves without doing anything.

The chosen session then asks what to do with it:

```
Session · 57e092c6-e0f9-4b14-9b88-00b3cacb410a · /home/you/src/nickpit
❯ Print  the review to stdout
  Copy   the review to the clipboard
  Chat   about the review, resuming this session
```

`Print` is what the command has always done, `Copy` hands the review to the clipboard as unstyled source (the same payload and helper chain as `--clipboard`), and `Chat` resumes that session in `nickpit chat`. On a row that came from an open request the three mean the same thing against the server. `Print` writes one document: the published review with the replies its own threads collected, each under what it answers — the answers to the summary under the summary, the answers to a finding under that finding, in the order they were written. Only NickPit's threads count: a thread is one whose root note carries the review's marker, so a conversation somebody else started on the request stays there, and so do the threads of the request's other reviews. Resolved threads are read like any other. The answers are laid out as the messages they are — `> **David Grieser** · 2026-09-07 15:03` and the text quoted under it — after the finding and its suggestions, since an answer is about both. People are named by their display name; a group access token, whose display name GitLab masks and whose username is a hash, is named `NickPit` when it is the account that opened the thread. `-o json` carries the same document as the review itself, with `replies` on the review (the summary thread) and on each finding; `-o markdown|raw` renders those replies as messages. `Copy` puts exactly that on the clipboard unstyled, and `Chat` reassembles the review from the request's markers and pulls its discussion into the context, so what has already been said about a finding is part of the conversation. A review this token did not publish can be discussed but not corrected — the notes that carry it belong to another user — so that chat opens without the correction tool and says so once instead of failing on the turn that would have used it. Replies are read on GitLab; a GitHub pull request's review prints without them. `Esc` or `Backspace` goes back to the list, which reopens on the scope and row it was left at. The action is confirmed on stderr with the session's full id — what `--session` takes. Passing `--clipboard` skips the prompt: it has already said what to do. Without a terminal nothing changes: the most recently updated session is printed, as before, so pipes, redirects and scripts keep working.

A local review now records the project of its `origin` remote and the branch it ran on, which a review of the working tree otherwise names nowhere. Sessions saved before that carry neither: they list under the repository (never guessed onto a branch) and are named by their checkout.

`nickpit session` uses the normal review output. Select it with `-o|--output markdown|json|raw`: `markdown` is the default and renders on a terminal, while `raw` always emits unrendered Markdown with no colors or terminal styling. The same flag works on review commands. `--json` remains as a compatibility alias for `--output json`.

`--warnings` prints only the warnings the run recorded instead of the review. The review output reduces them to a single `! Warnings: <n> (<type>: <count>, ...)` count line, so this is how the full text of a degraded run is read back: the same count line, then every warning in the order the run produced it, each tagged with the type it is grouped under. `--output json` emits `{"warnings": [...]}` — the key is always present, empty when there were none — and text output prints `No warnings.` for a clean run. `--clipboard` combines with it and copies the warnings.

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

- **Discussion (chat)**: reply in a thread NickPit started — under a finding's comment or the summary — and the daemon answers in-thread with the discussion agent, no keyword needed. Like reviews, each reply runs as a separate `nickpit chat` child process (the daemon itself never loads the LLM), which reassembles the review from the hidden markers on the MR, rebuilds the diff from the current MR, and posts the answer threaded (a reply under a finding is focused on that finding; under the summary it is about the whole review). The whole conversation lives in the MR thread, so it survives daemon restarts. A question the daemon is going to answer gets the acknowledgement reaction on the comment itself (`ack_emoji`, default `:eyes:`) as soon as the thread is confirmed as nickpit's own, and it is taken back once the answer is posted — or once the attempts are exhausted, so a lingering `:eyes:` never promises an answer that is not coming. `ack_emoji: ""` disables it. The same `nickpit chat --gitlab --url <MR> --reply-discussion <id>` is runnable from the terminal.

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
3. Group → Settings → Webhooks:
  - URL: `https://<daemon>/webhooks/gitlab`
  - Click **Generate signing token** and copy the token
  - Select **Trigger**:
    - **Merge request events**
    - **Emoji events** and
    - **Comments** (for the `/nickpit` commands)
  - Enable SSL verification
  - Click **Add webhook**
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

NickPit lets the model request additional file context during review. Control the maximum number of tool-call iterations with `--max-tool-calls` or `max_tool_calls` in config. `0` means unlimited, which is the default. You can also stop tool use after too many duplicate requests with `--max-duplicate-tool-calls` or `max_duplicate_tool_calls`; the default is `2`. Invalid model output is retried with `--max-output-retries` or `max_output_retries`; the default is `5`, and `0` means unlimited.

Each JSON tool result sent to the model is capped at `10%` of the context tokens remaining when that result is appended. Parallel results share the remainder in order, so later results cannot each claim the original allowance. When the window is already full, results still get a small readable floor, bounded across the batch so a wide parallel batch cannot overshoot the window by the sum of its floors; once that budget is spent, the remaining results are returned as a self-describing truncation note. Configure this with `--max-tool-result-percent`, `max_tool_result_percent`, or `NICKPIT_MAX_TOOL_RESULT_PERCENT`; `0` disables only the context cap, while tool-specific limits such as hierarchy depth and commit/result counts remain. Capped payloads stay valid JSON and report `truncated: true` plus a `truncated_note`; narrow the request when more detail is needed. Human-facing `nickpit inspect` output is not capped by this setting.

### Nudge Rounds

NickPit always runs the first configured nudge round, even when the initial reviewer pass found nothing. After that, a nudge that adds no accepted unique findings stops the reviewer's remaining automatic rounds. This avoids spending more turns after an empty or duplicate-only response while preserving productive nudges after an empty initial pass. Use `--force-all-nudges` or `force_all_nudges: true` in the active profile to retain fixed-round behavior. Explicit standalone `nudge:<vector>` workflow steps always run because each one was requested directly by the workflow author.

### Finding Caps

Cap how many findings each review agent may report with `--max-findings` or `max_findings` in config (also overridable per `review:` step in a workflow). The default is `0`, meaning unlimited. When a limit is set it is added to the reviewer prompt; a response exceeding the limit is retried once with guidance to keep only the strongest findings, after which the weakest findings (lowest priority, then lowest confidence) are cut and the agent run is marked partial. The limit counts the agent's whole session: initial pass plus nudge rounds; once the limit is reached, remaining nudge rounds (including standalone `nudge:` steps) are skipped.

### Reasoning Caps and Loop Detection

Reasoning calls are capped with `--max-reasoning-seconds` or `max_reasoning_seconds`; the default is `300`. When the cap is hit, NickPit aborts the stream and retries through the existing lower-reasoning-effort fallback path. NickPit also watches streamed reasoning for loops; detection is built in and needs no configuration. Three layered signals cover the observed failure modes: degenerate character runs (one character or a short unit repeated back-to-back), exact repeated lines or blocks (whitespace runs collapsed, empty lines ignored), and shingle recurrence — the fraction of recently emitted token shingles (lowercased, punctuation removed, code identifiers and numbers masked) that already appeared earlier in the same stream. Verbatim loops drive that recurrence to ~1.0 and are cancelled quickly; paraphrase loops (the same decision cycle reworded) plateau lower and must persist longer before they fire. Thresholds are staged over the reasoning time budget: early in a stream only ironclad repetition may cancel it, and detection becomes progressively more aggressive as the stream approaches `max_reasoning_seconds`, where it would be cancelled anyway. When a loop is detected, the stream is aborted and retried with lower reasoning effort and an added instruction to avoid repeating the same analysis.

### Concurrency and Accounting

Reviews run a context agent first, then six specialist reviewer lanes in parallel: Code Quality, Security, Architecture, Performance, Testing, and Best Practices. Each lane categorizes, verifies, and de-duplicates its reviewer's findings as soon as that reviewer finishes, so only clean findings reach the merge agent. Concurrent LLM agent loops — reviewers, categorizers, verifiers, dedupe, merge, finalize, verdict, summarize — are capped globally with `--concurrency` (default `10`, `0` = unlimited). The classifier is tool-free; tool-call limits apply independently to tool-enabled context, reviewer, verifier, and discussion agents. JSON output includes `total_tool_calls` at the root plus an `agent_runs` summary for workflow agents. Internal per-finding verifier tool calls contribute to `total_tool_calls`; the classifier and verifier appear in `agent_runs` as one entry per verify step rather than one per finding.

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
  - `categorize` — one entry **per verify step** (per reviewer lane in the built-in workflow), aggregating every finding that step classified; named after its lane, e.g. `Categorize Security`
  - `verify` — one entry **per verify step**, aggregating every finding that step verified and named after its lane, e.g. `Verify Security`; its `runtime_seconds` is the step's wall-clock span across the concurrent per-finding agents, and `duplicate_tool_calls` the repeats its verifiers made
  - `dedupe` — a **per-reviewer** de-duplication agent
  - `merge` — the cross-lane merge agent, one entry **per merge cluster**
  - `finalize` — the finalizer that fixes finding wording, priority, and confidence
  - `verdict` — the verdict agent that sets the top-level `overall_*` fields
  - `summarize` — the review summarizer

The root `tokens_used` is already the sum of everything, so **do not sum any of the breakdowns**. The `categorize` and `verify` entries are the same spend as `categorize_tokens_used` and `verify_tokens_used`, reported per step — summing those runs on top of the phase totals double-counts them.


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

See [`workflow.yaml.example`](workflow.yaml.example) for the full format. A spec lists `steps` (optionally grouped under `parallel:` to run concurrently); a parallel child can be a `lane:` — a list of steps that run sequentially within the group, e.g. one reviewer's `review:` → `verify:` → `dedupe:` chain. A `pipeline:` group is the explicit, streamed post-review tail (`merge` → `finalize` → `verdict`, optionally `summarize`): its steps overlap with no barrier between them — each merge cluster flows straight into finalize/summarize while other clusters are still merging, and verdict gates on all finalizes. Listing those steps flat instead runs them strictly sequentially, each over the whole finding set. Each step may carry a `config:` block overriding any model parameter or budget for that step only (model, temperature, top_p, top_k, min_p, presence_penalty, repetition_penalty, reasoning_effort, scope, max_tool_calls, max_output_retries, nudge_count, max_findings, disable_patch_summary, disable_suggestions, verify_drop_policy, confidence_threshold, …) — anything unset inherits the active profile/flags. `scope` makes a step's fan-out explicit — the work unit each agent operates on: `all` (whole finding set), `cluster` (per merge cluster, `merge`/`finalize`/`summarize`), `finding` (per finding, `verify`), or `reviewer` (per reviewer group, `dedupe`); cluster-scoped finalize/summarize is valid only inside a `pipeline:`. Use `model: "@small"` to select the configured nested `small` profile for a step — including its endpoint, when the profile's `small` block carries its own `base_url`/`api_key` (see [A small model on its own endpoint](#a-small-model-on-its-own-endpoint)); a step's own `config:` block cannot set an endpoint. `review:<vector>` configs can also override internal agents with `mine_reasoning:`, `compile_findings:`, and `nudge:` subconfigs; `verify` / `verify:<vector>` configs take a `categorize:` subconfig for the classifier that runs before evidence verification (the default workflow runs it on `model: "@small"`). Its `time_budget.weight` splits the verify step's budget between the two phases: the classifier gets that share and the verifier takes the rest, so a classifier stalling on an unresponsive endpoint cannot consume the whole step and leave findings unverified. The weight must be between 1 and 99 — 100 leaves the verifier nothing, and 0 means "optional, unallocated", which hands the classifier the whole step deadline and recreates the starvation. Only a weight splits the step: a `categorize.time_budget` carrying only `max_seconds`/`speedup_threshold` bounds the classifier and leaves the verifier the undivided step budget, and with no `categorize.time_budget` at all both phases share that budget as one unit. The default workflow gives the classifier `15`. `dedupe` / `dedupe:<vector>` / `merge` configs take a `context:` subconfig that trims what their prompt carries — those two stages judge findings against each other and receive the review context to ground that call. Its keys (`styleguides`, `diff`, `commits`, `comments`, `toolchain`) all default to `true`; set one to `false` to buy back its tokens. `styleguides` drops the styleguide rules from the system prompt, `diff` the patch itself, `commits` the commit summaries, `comments` the MR/PR discussion, and `toolchain` both `toolchain_versions` and the instruction to consult it. `toolchain: false` only trims the prompt — the detected toolchain still selects the styleguide version, so a Go 1.25 change keeps the Go 1.25 guide rather than falling back to the generic one. The changed-file list and the context agent's supplemental files are always sent. The default workflow sets all five to `true` explicitly. A `verify` step privately classifies each finding before evidence verification. The classifier sees only the submitted finding and toolchain versions; it does not see routing outcomes, diff scope, tools, or verifier categories. Go maps the descriptive categories through `verify_drop_policy`, then deterministically requires an exact old- or new-side diff overlap. It relocates an anchor only when its submitted content has one unambiguous in-diff match; otherwise a live reviewer gets one response-validation retry whose allowed ranges are complete `code_location` JSON objects. Injected findings have no originating reviewer call to retry; their deterministic relocation requires retrieval and a repository root, and unresolved anchors are dropped with an aggregate warning. An unresolved out-of-diff finding is always dropped, even under policy `none`; `--disable-diff-scope` is the only switch for that. Classification failures fail open so they cannot silently discard a real finding. LLM concurrency is run-level only (`--concurrency`, default `10`, `0` = unlimited): one shared cap across every agent loop in the run — it is intentionally not in the spec. Per-vector steps are addressed as `review:security`, `verify:security`, `dedupe:security`, …; `nudge:<vector>` / `reasoning-extract:<vector>` let you drive extra rounds manually. Any global step can take `findings_from:` to inject previously-emitted findings JSON (the same format `--output json` produces; one file = one merge group); inside a `pipeline:` only the `merge` step may carry it. Steps that only consume injected findings (e.g. `merge`, `finalize`, `verdict`, `summarize`) run without a git/PR source. `finalize` now only finalizes finding wording/priority/confidence; include `verdict` after it when a workflow needs final top-level `overall_correctness`, `overall_explanation`, and `overall_confidence_score`. `confidence_threshold` is applied only by the `verdict` step.

### Time budgets

A spec's `time_budget` blocks bound wall-clock time per step and per group: `max_seconds` is an absolute cap, `weight` a lazily-started share of the parent's budget, and `speedup_threshold` (50–100) the point at which retries switch to urgent mode. Every budget resolves to the tightest of its weight share, its own `max_seconds`, and its parent's deadline, so a child can never outlive its group.

For reviewers, the speed-up threshold ends exploration and reserves the remaining time for completing the current review or nudge round. A short reasoning-mining pass combines unmined notes (including interrupted output) with collected candidates, followed by a no-tools finalization request with low reasoning and any existing findings. Mining uses at most 25% of the remaining time, capped at 30 seconds, its computed `mine_reasoning` weighted allocation, and its configured absolute cap; the tightest bound wins. An optional `mine_reasoning` phase (`weight: 0`) or a zero allocation skips final mining. On skip or failure, finalization receives the notes directly. `disable_reasoning_extract` skips mining but still supplies captured notes. Finalization allows at most one output-repair retry within the existing retry allowance and hard deadline. No further nudges run after this transition, including standalone workflow nudges. Recovered findings continue through verification and merge. Other agents retain their existing urgent behavior. A threshold of 100 disables early finalization. Saved agent runs optionally include `budget_stop` metadata; confirmed deadline stops produce a budget warning rather than a duplicate reviewer error.

`--time-budget-scale` (or `time_budget_scale`) multiplies every absolute wall-clock cap in the run without touching the spec file: each `max_seconds` — on steps, on their `mine_reasoning`/`compile_findings`/`nudge` subconfigs, and on lane/pipeline group configs (a `parallel:` group takes no `config:` of its own) — and each `max_reasoning_seconds`, whether it comes from the profile or from a step/agent override. The reasoning cap moves with the step budget on purpose: it bounds one call's reasoning stream before the effort fallback kicks in, so holding it fixed while the step budget halves would make the step die on its deadline instead of degrading effort, and doubling the step budget alone would still degrade at the same second. `2` gives the whole review twice the time it declares, `0.5` half; the default `1` runs the spec as written, and a non-positive or non-finite factor is rejected. `weight` and `speedup_threshold` are deliberately left alone: both are relative to the parent budget, so they already move with it. Scaled caps are clamped to at least one second and at most ten days; a clamped cap no longer follows the factor, so the run warns when any was clamped. This is the knob for "let this review take longer" — raising an outer bound alone would not help, because each step's own `max_seconds` still applies. `--disable-workflow-time-budget` ignores every `time_budget` entry instead, and takes precedence over the scale.

```bash
# Twice the declared time everywhere
nickpit review --mr 42 --time-budget-scale=2
```

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
