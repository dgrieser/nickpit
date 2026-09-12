# Code Structure

This document maps the production Go code. Test files live beside the code they exercise as `*_test.go`; use them as executable examples for expected behavior and edge cases.

## Commands

- `cmd/nickpit/main.go`: Main CLI entry point. Defines commands, flags, profile loading, workflow execution, local review modes, SCM review modes, output selection, publishing, seed-finding handling, and post-review chat-session persistence.
- `cmd/nickpit/chat.go`: `nickpit chat` command. Starts or resumes a discussion session (from a saved review JSON, a GitLab MR's markers, or the latest/last session), prefers the session's cached prepared context and recreates the diff through the review pipeline when the MR gained commits, and drives the discussion agent interactively (REPL) or one-shot. `--reply-discussion` is the non-interactive GitLab thread-reply mode (read a thread, gate on its root marker, answer only the latest note, post the reply back) that the serve daemon spawns and the terminal can run directly.
- `cmd/nickpit/feedback_cmd.go`: `nickpit gitlab feedback` / `nickpit github feedback`. Reads the reviews NickPit published on an MR/PR back from their hidden carrier markers (no LLM, no session, read-only) and prints one, copies it to the clipboard, or lists the reviews the request carries.
- `cmd/nickpit/select.go`: Interactive target selection for the CLI: the selector policy every MR/PR-addressed command shares (`--url` exclusivity, repo inference, and when a missing `--id` defers to a picker), the open-request, branch and commit pickers (branch refs folded per branch, resolving to the remote side for a base and the local one for a head), the per-row colouring, and the terminal detection plus test seam behind them.
- `cmd/nickpit/session_select.go`: The session picker behind a bare `nickpit session`: where the invocation is standing (working tree, shared git directory, `origin` project, checked-out branch), which scope each saved session falls into (this branch, this repository, all — the fourth, `remote`, is the published half in `session_remote.go`) — matched on the repository rather than the path, so worktrees and review subdirectories fold into one, with anchor directories claiming the checkouts that no longer exist — the Print/Copy/Chat prompt the chosen session opens (backspace and Esc going back to the list, which reopens where it was left), the rows and their labels (request, branch, commit range or checkout, with an uncommitted review marked as such and its branch recovered from the cached context when the source recorded none, plus the verdict and finding count of the review), the colour each kind of review and each verdict wears, the 40-cell cap on what was reviewed and the part-aware shortening under it, the 24-cell cap on the project and the path folding under that, which scope the prompt opens on, and the project/branch a local review records at save time so its session can be placed at all.
- `cmd/nickpit/session_remote.go`: The published half of the session picker: which platform the checkout's `origin` names (and whether its host is the one the token belongs to), the merge/pull requests of that project (open ones, or every state with `--include-closed`) and the reviews their hidden markers carry (read without the author check — the scope exists to show what other tokens published — in parallel, capped and time-bounded, partial results kept), the rows of the `remote` scope they become (its own scope: none of the session scopes ever shows them), the current-branch-first ordering, and what Print/Copy/Chat do against the server — one document of the review plus the replies to it (threads recognized by the marker in their root note, hung on the review and its findings as `model.Reply` so every format carries them, with display names — `NickPit` where a token account's name is masked), the same text on the clipboard, and a chat reassembled from the request with its discussion in context (read-only where the review was published by another token: corrections rewrite that user's notes, so the tool is withheld).
- `cmd/nickpit/gitlab_templates.go`: `nickpit gitlab templates sync|list`. Resolves comment-template scopes (user, group, project, or every group in a serve config with its own token) and converges them on the command templates.
- `cmd/nickpit-config-example/main.go`: Generator binary that prints the example config from `internal/config`.
- `cmd/nickpit-workflow-example/main.go`: Generator binary that prints the embedded example workflow.

## Configuration

- `internal/config/config.go`: Loads and merges config files, environment variables, profiles, defaults, and CLI overrides.
- `internal/config/example.go`: Provides the checked-in example config text.
- `internal/config/generate.go`: Shared helpers for generator commands.
- `internal/config/profiles.go`: Profile resolution (`ResolveProfile`) and profile merging (`mergeProfiles`); the built-in provider profiles live in `config.go` (`defaultProfiles`).
- `internal/config/configtest/configtest.go`: Test-only helper that clears every config-influencing environment variable, shared by the config and CLI test suites.

## Review Pipeline

- `internal/review/engine.go`: Core review engine. Builds prompts, runs agent loops, categorizes and verifies findings, dedupes and merges results, applies filters, handles time-budget retries, and logs progress.
- `internal/review/agent_loop.go`: Generic LLM agent retry loop, validation retry flow, and response parsing.
- `internal/review/pipeline.go`: Pipeline model and execution state for workflow steps, groups, lanes, and result aggregation.
- `internal/review/pipeline_steps.go`: Step implementations for context collection, review lanes, merge, finalize, verdict, summarize, and fused post-merge execution.
- `internal/review/reviewer_session.go`: Reviewer session state, main review execution, nudge handling, and reasoning-mining/update subagents.
- `internal/review/reviewer_budget.go`: Reviewer soft-deadline finalization, bounded candidate mining, partial-output recovery, and round termination.
- `internal/review/categorizer.go`: Private per-finding descriptive classification inside verification. The classifier is blind to routing outcomes, diff scope, tools, and verifier evidence; Go applies the configured drop policy and classification failures fail open.
- `internal/review/diff_scope.go`: Canonical old/new diff windows (plus a line-1 window for metadata-only symlink changes, which have no hunk), deterministic overlap checks, and scope filtering used for location repair and retry guidance.
- `internal/review/verifier.go`: Per-finding evidence verification, verifier options, fallback unverified results, and verifier telemetry.
- `internal/review/discuss.go`: Discussion (chat) agent. Free-form, schema-less, tool-enabled `Engine.Discuss` turn: builds the system prompt from the full findings JSON, diff, and styleguides, optionally opens on a pinned finding, and runs one conversation turn returning the reply plus the messages to persist.
- `internal/review/finalizer.go`: Final finding polishing, priority constraints, finalization payloads, and finalizer output application.
- `internal/review/verdict.go`: Overall verdict agent prompt payloads, confidence-threshold filtering before verdict, and verdict fallback behavior.
- `internal/review/update.go`: Independent correction agent; validates selected-finding replacements and terminal resolutions, or assesses review-level disputes without generating verdicts, publishing, or exposing history.
- `internal/review/update_summary.go`: Reuses default-workflow finding and overall summarization passes for corrections, including small-model routing and failure fallback, while preserving unchanged and resolved findings.
- `internal/review/update_workflow.go`: Executes the embedded `workflows/update.yaml` correction stages. Durable jobs checkpoint its result before publishing.
- `internal/workflow/update.go`: Loads the built-in correction YAML with the shared spec parser; no external override path.
- `internal/review/custom_tools.go`: Serial mutation callbacks alongside batched retrieval tools in the shared agent loop.
- `internal/llm/update_schema.go`: Structured correction decisions using standard finding fields, excluding code-owned revision and provenance state.
- `cmd/nickpit/chat_update.go`: GitLab chat correction callback, linked discussion evidence, freshness checks, and existing verdict-agent orchestration.
- `cmd/nickpit/chat_update_cli.go`: Invocation-scoped CLI correction coordinator and background runner, immutable conversation snapshots, shared GitLab execution locks, bounded retries and publication recovery, and local review revisions.
- `cmd/nickpit/chat_update_evidence.go`: Selected-thread snapshots shared by correction prompts and freshness checks, with note cutoffs and anchored fallback replies.
- `internal/review/summarizer.go`: Finding and overall-summary agents, summary payloads, and summarized-body application.
- `internal/review/context_filter.go`: Context trimming and file filtering before prompts are built.
- `internal/review/classify.go`: Stamps generated-file marks across the changed-file and diff-file views, plus symlink metadata from the reviewed head tree: marks (changed files, diff files and hunks) for sources whose diff carries no git file mode (GitHub), and link targets for any source whose patch shows none (a pure rename).
- `internal/review/limiter.go`: Global concurrency limiter used around agent calls.
- `internal/review/time_budget.go`: Hierarchical time budgets, local caps, weights, speedup thresholds, and context deadlines.
- `internal/review/tool_exec.go`: Tool-call dispatcher for retrieval tools exposed to review agents.
- `internal/review/tool_result_limit.go`: Context-aware JSON tool-result token limits with payload-specific pruning and truncation metadata.
- `internal/review/trimmer.go`: Prompt/context size reduction helpers.
- `internal/review/review_file_unix.go`, `internal/review/review_file_nonunix.go`: Platform-specific file reading helpers used for review context.

## LLM Client and Schemas

- `internal/llm/client.go`: OpenAI-compatible client, request construction, streaming, tool calls, retries, reasoning handling, and JSON/schema response modes.
- `internal/llm/clientset.go`: Endpoint→client resolution, so a run's primary model and a `@small` model on another endpoint use their own clients.
- `internal/llm/schema.go`: Schema-kind dispatch and shared schema helpers.
- `internal/llm/categorize_schema.go`: Descriptive categorization response schema.
- `internal/llm/verify_schema.go`: Verification response schema.
- `internal/llm/merge_schema.go`: Merge/dedupe response schema.
- `internal/llm/finalize_schema.go`: Finalization response schema and suggestion-shape handling.
- `internal/llm/verdict_schema.go`: Overall verdict response schema.
- `internal/llm/summarize_schema.go`: Summarization response schema.
- `internal/llm/jsonx.go`: Lenient JSON parsing and extraction helpers.
- `internal/llm/prompt.go`: Prompt message helpers.
- `internal/llm/reasoning_buffer.go`: Captures and bounds streamed reasoning text.
- `internal/llm/reasoning_loop.go`: Streaming reasoning-loop detector (character runs, exact line/block repetition, token-shingle recurrence), staged over the reasoning time budget; zero-config.
- `internal/llm/retry.go`: Retryability classification and retry helpers.

## Data Models and Formatting

- `internal/model/types.go`: Shared domain types for requests, results, findings, verification, finalization, SCM data, toolchain data, and token usage.
- `internal/model/format.go`: Human-readable formatting helpers for model values.
- `internal/tokenestimate/tokenestimate.go`: Central prompt-token estimation API and current four-bytes-per-token heuristic.
- `internal/workflow/spec.go`: Workflow YAML schema, parsing, default workflow construction, aliases, step config, and validation.

## Deduplication

- `internal/dedupe/dedupe.go`: Rule-based finding similarity. Computes same-file, cross-file, title/body, location, and root-cause signals used to route possible duplicates.
- `internal/dedupe/merge.go`: Mechanical finding merge rules: confidence combination, priority selection, line-range extension, suggestion selection, and verification merge.

## Retrieval and Repository Indexing

- `internal/retrieval/engine.go`: Retrieval engine that wires file access, search, symbols, and call graph operations.
- `internal/retrieval/backend.go`: Backend interfaces and shared result types.
- `internal/retrieval/backend_files.go`: Backend file discovery and filtering.
- `internal/retrieval/file.go`: File, slice, and directory retrieval.
- `internal/retrieval/findlines.go`: Exact code-to-line-number matching backing multi-line `search` queries and code-location repair.
- `internal/retrieval/references.go`: Definition-centered symbol reference analysis, whole-function grouping, top-level writes, alias following, confidence marking, and human rendering.
- `internal/retrieval/callgraph.go`: Call hierarchy API and orchestration.
- `internal/retrieval/static_graph.go`: Static call graph storage and lookup.
- `internal/retrieval/symbols.go`: Symbol references and symbol lookup helpers.
- `internal/retrieval/nodejs_backend.go`, `python_backend.go`, `rust_backend.go`: Language-specific retrieval backends; cross-file resolution (imports, exports, class methods) over the tsparser IR.
- `internal/retrieval/irparse.go`: Parallel tsparser parsing helper and graph-backed symbol lookup shared by the non-Go backends.
- `internal/retrieval/tsparser/tsparser.go`: Language dispatch and line indexing for the AST extraction layer.
- `internal/retrieval/tsparser/ir.go`: Language-neutral IR (symbols, classified calls, imports, exports).
- `internal/retrieval/tsparser/javascript.go`: JS/TS/JSX/TSX symbol and call extraction via esbuild's parser.
- `internal/retrieval/tsparser/python.go`, `internal/retrieval/tsparser/rust.go`: Python and Rust extraction via the pure-Go tree-sitter runtime.
- `internal/retrieval/tsparser/treesitter.go`: Shared tree-sitter parsing and error-scan helpers.
- `internal/retrieval/goparser/parser.go`: Go parser wrapper for symbols and call information.
- `internal/retrieval/goparser/callgraph.go`: Go call graph extraction.
- `internal/retrieval/repofs/repofs.go`: Filesystem abstraction over repository roots.

## Git and SCM Integrations

- `internal/git/git.go`: Git command wrapper and repository helpers.
- `internal/git/diff.go`: Diff loading and changed-file extraction. Owns `patchArgs`/`stableDiffArgs`, the pinned `-U3` plus configuration-neutralizing flags every patch-emitting git invocation must use.
- `internal/git/parser.go`: Git diff parser and hunk model.
- `internal/git/modes.go`: Git file-mode lookups: symlinks (with their blob names) in a given commit tree (`ls-tree`, literal pathspecs), post-change modes plus blob names from a `--raw` listing, and verbatim blob reads, so symlinks are recognized — and their targets recoverable — independently of how a worktree materialized them.
- `internal/git/history.go`: Commit history provider for the git_log/git_show tools and `nickpit inspect log|show`.
- `internal/git/refs.go`: `Where` (the working tree a directory sits in plus the shared git directory that identifies its repository across worktrees) and branch and commit listings for the interactive ref pickers, each commit carrying its first parent (and `EmptyTreeSHA` for a root commit) so a range picked over the commits to review can derive the exclusive base a diff needs (`CurrentBranch`, `DefaultBranch`, `Branches`, `Commits`), NUL-framed so a crafted subject cannot fake a record separator. Each branch ref carries its remote and its remote-independent branch name, so a picker can fold the refs of one branch together, and symbolic refs (a remote's HEAD) are dropped as aliases.
- `internal/git/checkout.go`: Temporary checkout/worktree helpers.
- `internal/scm/github/adapter.go`: GitHub adapter wiring, plus reassembly of published reviews from the carrier markers on the PR's reviews, review comments, and issue comments (author-verified).
- `internal/scm/github/client.go`: GitHub API client.
- `internal/scm/github/pr.go`: Pull request loading and review source construction.
- `internal/scm/github/prlist.go`: Open pull requests of a repo as `model.OpenRequest` rows for the interactive picker, newest activity first with deterministic ties.
- `internal/scm/github/position.go`: GitHub inline-comment position mapping.
- `internal/scm/github/publish.go`: GitHub review/comment publishing.
- `internal/scm/github/user.go`: Authenticated token owner lookup, used to verify carrier-marker authorship.
- `internal/scm/gitlab/adapter.go`: GitLab adapter wiring.
- `internal/scm/gitlab/client.go`: GitLab API client.
- `internal/scm/gitlab/mr.go`: Merge request loading, review source construction, and live MR status (`FetchMRStatus`).
- `internal/scm/gitlab/mrlist.go`: Open merge requests of a project as `model.OpenRequest` rows for the interactive picker, newest activity first with deterministic ties.
- `internal/scm/gitlab/project.go`: Project lookup (topics), current-user lookup, and award-emoji posting.
- `internal/scm/gitlab/notes.go`: Note/discussion operations used by the serve daemon: plain MR notes, threaded replies, discussion listing, and root-note updates.
- `internal/scm/gitlab/graphql.go`: GraphQL transport (endpoint derivation from the REST base URL, `errors`-array handling) for the parts of GitLab that have no REST API.
- `internal/scm/gitlab/savedreply.go`: Comment templates ("saved replies") per scope — user, project, or group: listing and prefix-scoped idempotent sync (create/update/prune, dry run).
- `internal/scm/gitlab/position.go`: GitLab inline-comment position mapping.
- `internal/scm/gitlab/publish.go`: GitLab review/comment publishing.
- `internal/scm/gitlab/update.go`: Original-review-scoped revision publishing, linked location replacements, durable pending updates, and crash recovery.
- `internal/scm/gitlab/lock*.go`: Reentrant process-safe MR write locks shared by publishing, corrections, and response controls.
- `internal/scm/reviewmd/history.go`: Bounded flat comment archives, hidden update/thread metadata, and highest-current-revision carrier selection.
- `internal/scm/reviewmd/render.go`: Markdown review report rendering; hidden idempotency markers and the base64+gzip carrier markers (`nickpit:review:` / `nickpit:finding:`) that embed the full review and each finding in note bodies, grouped by review id, plus `ReviewResultsByID` to reassemble a `ReviewResult` from an MR/PR's notes.
- `internal/scm/reviewmd/response.go`: Visible GitLab response-mode footers plus hidden persistent thread-mute metadata and a rendered-policy fingerprint (so footers stamped under earlier settings are detectable), with stripping before LLM context assembly.

## GitLab Webhook Daemon (`nickpit gitlab serve`)

- `internal/serve/server.go`: HTTP server wiring, /healthz, and graceful-shutdown sequencing.
- `internal/serve/handler.go`: Webhook endpoint: body limit, group match, constant-time secret check, event classification, fast-ack enqueue, and command routing (ack emoji and replies posted async). Chat events additionally wear the ack emoji on the question note from the moment the thread gate admits them until the event ends.
- `internal/serve/event.go`: Webhook payload envelope and the pure `Decide()` trigger policy (auto vs manual vs command vs chat vs ignore); a plain reply in a discussion thread becomes a `CommandChat` candidate.
- `internal/serve/command.go`: `/keyword` note-command parsing, full-line response/skip directives, and help/status/abort reply texts.
- `internal/serve/response.go`: Live GitLab response policy from config, MR/root reactions, and persistent command state; reconciles status footers on review roots.
- `internal/serve/templates.go`: The note commands expressed as GitLab comment templates (names, bodies, prune prefix) so the comment box's template picker can offer them.
- `internal/serve/groups.go`: Per-group tokens/secrets/clients with longest-prefix project matching and bot-user IDs.
- `internal/serve/dispatcher.go`: Coalescing per-MR job queue, worker pool, reviewed-SHA LRU, per-job abort (`Abort`/`JobInfo`), and shutdown grace handling.
- `internal/serve/update_jobs.go`: Strict atomic, fsynced correction-job checkpoints in the private serve state directory; no credentials or temporary checkout paths.
- `internal/serve/update_worker.go`: Bounded correction scheduler with strict per-MR ordering, separate chat capacity, and current credentials and response policy.
- `cmd/nickpit/chat_update_job.go`: Durable enqueue returning scheduling status to chat, idempotent follow-up, fresh-evidence evaluation, and checkpointed GitLab publication recovery.
- `internal/scm/reviewmd/update_reply.go`: Bot-owned asynchronous reply metadata binding late follow-ups to their original question.
- `internal/serve/worker.go`: Per-job pipeline: topic opt-in check, authoritative MR recheck, start-emoji award, child-process review run.
- `internal/serve/runner.go`: `ReviewRunner`/`ChatRunner` seams and `ExecRunner` spawning `nickpit gitlab mr --publish` (review) and `nickpit chat --gitlab … --reply-discussion` (chat) children, with shared log capture. The daemon runs no LLM itself; the chat child self-gates and posts its own reply.
- `internal/serve/topics.go`: TTL + singleflight cache for project topics.
- `internal/config/serve.go`: `server.yaml` schema, loading (env expansion), defaults, and validation for the daemon.

## Output, Logging, and Support Packages

- `internal/output/terminal.go`: Human terminal output.
- `internal/output/json.go`: JSON output.
- `internal/output/badge.go`: Badge/status formatting helpers.
- `internal/logging/logger.go`: Base logger, reasoning sections, JSON rendering, and raw output.
- `internal/logging/progress.go`: Progress-line data model, formatting, coloring, and workflow labels.
- `internal/logging/reasoning_renderer.go`: Live reasoning renderer for terminal output.
- `internal/logging/verbose.go`: Verbose log blocks, JSON pretty-printing, and context-aware formatting.
- `internal/filetype/language.go`: Unified file classification API (language detection, generated-file flags, trim eviction classes) backed by the mappings data.
- `internal/styleguide/styleguide.go`: Resolves user-supplied additional styleguides (local files or HTTP(S) URLs) into prompt-ready guides.
- `internal/session/session.go`: Resumable discussion (chat) session store: atomic JSON files (one per session) under the user cache dir, caching the review source descriptor, the prepared review context plus the head SHA it was built at, `ReviewResult`, the full message transcript, and archived review revisions; load/save/list/latest helpers (a listing decodes only the file's header — source, model, counts, the first finding's title, the cached context's two refs and timestamps — never the context itself or the transcript; the source records the branch a local review ran on, for listings only), an unconditional sweep of orphaned temp files, and opt-in oldest-first pruning past a caller-supplied cap (`WithMaxStored`, wired from `--max-sessions`/`max_sessions`; unlimited by default, and the session just saved is never a victim).
- `internal/pick/pick.go`: Keyboard-driven single-choice list on a terminal (raw mode, in-place redraw, abort/no-items sentinels, and the exported cell-width helpers `DisplayWidth`/`Truncate`/`TruncateMiddle` a caller shortens a value with before it reaches a column): the interactive counterpart of the `--id`/`--url`/`--base` selectors. Widths are display cells, not runes, so wide runes cannot wrap a row out of sync with the redraw. `SelectView` is the multi-scope form: several row sets in one list, switched with `Tab`, returning the scope the user ended in together with the row inside it; a scope whose rows cost a round trip supplies them through `View.Load`, run once with its loading line on screen and its failure (or partial result) reported in the status line.
- `internal/pick/list.go`: The picker's pure state and rendering — cursor, scroll window, AND-term filter over per-row match fields, each able to demand a minimum term length so a short term cannot reach an opaque id, column fitting, per-column and per-row colours, the selected row's pastel highlight, the one status line under the rows (position, the scopes of a multi-view list with the current one in brackets, and the filter only while one is typed), the scope switch itself carrying the filter across and following the row's key, an anchored range (the span highlighted as one block, counted in the title line, with the filter cleared and frozen so nothing can hide inside it), and the header above the rows naming the current row, either as one value or as `Item.Details` fields joined by the list's separator, each in the colour its own column wears — driven by keys and returning lines, so its behaviour is testable without a terminal. Its palette is the 256-colour message palette of `internal/logging/progress.go`, mirrored in `tools/print_colors.sh`.
- `internal/pick/message.go`: What the picker paints inside a cell: a conventional-commit prefix taken apart into type, scope and punctuation, the faded `/`/`:` separators of a ref, and the light grey a trailing parenthetical note recedes into — the rules the `git-color` dev helper applies to a git log, in NickPit's palette. Columns declare which they want via `ColumnKinds`.
- `internal/pick/keys.go`: Terminal key decoding (CSI/SS3 sequences, control keys, `Tab`/`Shift-Tab` and the horizontal arrows as scope switches, sequences split across reads, lone `ESC` as abort).
- `internal/clipboard/clipboard.go`: Cross-platform clipboard writes for `session --clipboard`: a per-GOOS chain of helper commands (`pbcopy`, `clip.exe` with UTF-16LE encoding, `wl-copy`/`xclip`/`xsel` ordered by session type, `termux-clipboard-set`) tried until one succeeds, bounded by a shared timeout.
- `internal/toolchain/toolchain.go`: Toolchain version capture and normalization.
- `internal/tools/catalog.go`: Tool catalog exposed to agents; describes each tool and its arguments to the model.
- `internal/toollimits/toollimits.go`: The tool defaults and non-token limits themselves, in a package with no dependencies so git, config and retrieval can honor them without depending on the LLM layer.
- `internal/textsan/textsan.go`: Text sanitization utilities.
- `internal/testutil/testutil.go`: Shared test fixture and golden-file helpers.

## Model Capability Checks

- `internal/modelcheck/checker.go`: Probes model support for tools, JSON output, JSON schema, reasoning efforts, and retry behavior.
- `internal/modelcheck/cache.go`: Reads/writes cached model capability results keyed by provider/model settings.

## Non-Go Assets

- `prompts/`: Agent system prompts and shared prompt snippets.
- `prompts/styleguides/`: Language/tool style rules injected into review and verification prompts.
- `workflows/`: Embedded workflow YAML definitions: `default.yaml` for reviews and internal-only `update.yaml` for chat corrections.
- `mappings/`: Data backend for file classification: language path/content rules (incl. shebangs), generated-file patterns and markers, trim eviction classes, and styleguide detectors. All detection rules live in the YAML files; the Go code is a generic PatternSet matching engine.
- `assets/`: Static assets used by output or packaging.
- `testdata/`: Fixtures and golden data used by tests.
