# Code Structure

This document maps the production Go code. Test files live beside the code they exercise as `*_test.go`; use them as executable examples for expected behavior and edge cases.

## Commands

- `cmd/nickpit/main.go`: Main CLI entry point. Defines commands, flags, profile loading, workflow execution, local review modes, SCM review modes, output selection, publishing, seed-finding handling, and post-review chat-session persistence.
- `cmd/nickpit/chat.go`: `nickpit chat` command. Starts or resumes a discussion session (from a saved review JSON, a GitLab MR's markers, or the latest/last session), resolves the review source and context, and drives the discussion agent interactively (REPL) or one-shot. `--reply-discussion` is the non-interactive GitLab thread-reply mode (read a thread, gate on its root marker, run one turn, post the reply back) that the serve daemon spawns and the terminal can run directly.
- `cmd/nickpit-config-example/main.go`: Generator binary that prints the example config from `internal/config`.
- `cmd/nickpit-workflow-example/main.go`: Generator binary that prints the embedded example workflow.

## Configuration

- `internal/config/config.go`: Loads and merges config files, environment variables, profiles, defaults, and CLI overrides.
- `internal/config/example.go`: Provides the checked-in example config text.
- `internal/config/generate.go`: Shared helpers for generator commands.
- `internal/config/profiles.go`: Profile resolution (`ResolveProfile`) and profile merging (`mergeProfiles`); the built-in provider profiles live in `config.go` (`defaultProfiles`).

## Review Pipeline

- `internal/review/engine.go`: Core review engine. Builds prompts, runs agent loops, verifies findings, dedupes and merges results, applies filters, handles time-budget retries, and logs progress.
- `internal/review/agent_loop.go`: Generic LLM agent retry loop, validation retry flow, and response parsing.
- `internal/review/pipeline.go`: Pipeline model and execution state for workflow steps, groups, lanes, and result aggregation.
- `internal/review/pipeline_steps.go`: Step implementations for context collection, review lanes, merge, finalize, verdict, summarize, and fused post-merge execution.
- `internal/review/reviewer_session.go`: Reviewer session state, main review execution, nudge handling, and reasoning-mining/update subagents.
- `internal/review/verifier.go`: Per-finding verification agents, verifier options, fallback unverified results, and verifier telemetry.
- `internal/review/discuss.go`: Discussion (chat) agent. Free-form, schema-less, tool-enabled `Engine.Discuss` turn: builds the system prompt from the full findings JSON, diff, and styleguides, optionally opens on a pinned finding, and runs one conversation turn returning the reply plus the messages to persist.
- `internal/review/finalizer.go`: Final finding polishing, priority constraints, finalization payloads, and finalizer output application.
- `internal/review/verdict.go`: Overall verdict agent prompt payloads, confidence-threshold filtering before verdict, and verdict fallback behavior.
- `internal/review/summarizer.go`: Finding and overall-summary agents, summary payloads, and summarized-body application.
- `internal/review/context_filter.go`: Context trimming and file filtering before prompts are built.
- `internal/review/limiter.go`: Global concurrency limiter used around agent calls.
- `internal/review/time_budget.go`: Hierarchical time budgets, local caps, weights, speedup thresholds, and context deadlines.
- `internal/review/tool_exec.go`: Tool-call dispatcher for retrieval tools exposed to review agents.
- `internal/review/trimmer.go`: Prompt/context size reduction helpers.
- `internal/review/review_file_unix.go`, `internal/review/review_file_nonunix.go`: Platform-specific file reading helpers used for review context.

## LLM Client and Schemas

- `internal/llm/client.go`: OpenAI-compatible client, request construction, streaming, tool calls, retries, reasoning handling, and JSON/schema response modes.
- `internal/llm/schema.go`: Schema-kind dispatch and shared schema helpers.
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
- `internal/git/diff.go`: Diff loading and changed-file extraction.
- `internal/git/parser.go`: Git diff parser and hunk model.
- `internal/git/checkout.go`: Temporary checkout/worktree helpers.
- `internal/scm/github/adapter.go`: GitHub adapter wiring.
- `internal/scm/github/client.go`: GitHub API client.
- `internal/scm/github/pr.go`: Pull request loading and review source construction.
- `internal/scm/github/position.go`: GitHub inline-comment position mapping.
- `internal/scm/github/publish.go`: GitHub review/comment publishing.
- `internal/scm/gitlab/adapter.go`: GitLab adapter wiring.
- `internal/scm/gitlab/client.go`: GitLab API client.
- `internal/scm/gitlab/mr.go`: Merge request loading, review source construction, and live MR status (`FetchMRStatus`).
- `internal/scm/gitlab/project.go`: Project lookup (topics), current-user lookup, and award-emoji posting.
- `internal/scm/gitlab/notes.go`: Note-level operations used by the serve daemon: note award-emoji, plain MR notes, and threaded discussion replies.
- `internal/scm/gitlab/position.go`: GitLab inline-comment position mapping.
- `internal/scm/gitlab/publish.go`: GitLab review/comment publishing.
- `internal/scm/reviewmd/render.go`: Markdown review report rendering; hidden idempotency markers and the base64+gzip carrier markers (`nickpit:review:` / `nickpit:finding:`) that embed the full review and each finding in note bodies, grouped by review id, plus `ReviewResultsByID` to reassemble a `ReviewResult` from an MR/PR's notes.

## GitLab Webhook Daemon (`nickpit gitlab serve`)

- `internal/serve/server.go`: HTTP server wiring, /healthz, and graceful-shutdown sequencing.
- `internal/serve/handler.go`: Webhook endpoint: body limit, group match, constant-time secret check, event classification, fast-ack enqueue, and command routing (ack emoji and replies posted async).
- `internal/serve/event.go`: Webhook payload envelope and the pure `Decide()` trigger policy (auto vs manual vs command vs chat vs ignore); a plain reply in a discussion thread becomes a `CommandChat` candidate.
- `internal/serve/command.go`: `/keyword` note-command parsing and the help/status/abort reply texts.
- `internal/serve/groups.go`: Per-group tokens/secrets/clients with longest-prefix project matching and bot-user IDs.
- `internal/serve/dispatcher.go`: Coalescing per-MR job queue, worker pool, reviewed-SHA LRU, per-job abort (`Abort`/`JobInfo`), and shutdown grace handling.
- `internal/serve/worker.go`: Per-job pipeline: topic opt-in check, authoritative MR recheck, start-emoji award, child-process review run.
- `internal/serve/runner.go`: `ReviewRunner`/`ChatRunner` seams and `ExecRunner` spawning `nickpit gitlab mr --publish` (review) and `nickpit chat --gitlab … --reply-discussion` (chat) children, with shared log capture. The daemon runs no LLM itself; the chat child self-gates and posts its own reply.
- `internal/serve/topics.go`: TTL + singleflight cache for project topics.
- `internal/config/serve.go`: `server.yaml` schema, loading (env expansion), defaults, and validation for the daemon.

## Output, Logging, and Support Packages

- `internal/output/terminal.go`: Human terminal output.
- `internal/output/json.go`: JSON output.
- `internal/output/sarif.go`: Unused SARIF formatter stub; `FormatFindings` returns "not yet implemented".
- `internal/output/badge.go`: Badge/status formatting helpers.
- `internal/logging/logger.go`: Base logger, reasoning sections, JSON rendering, and raw output.
- `internal/logging/progress.go`: Progress-line data model, formatting, coloring, and workflow labels.
- `internal/logging/reasoning_renderer.go`: Live reasoning renderer for terminal output.
- `internal/logging/verbose.go`: Verbose log blocks, JSON pretty-printing, and context-aware formatting.
- `internal/filetype/language.go`: Unified file classification API (language detection, generated-file flags, trim eviction classes) backed by the mappings data.
- `internal/styleguide/styleguide.go`: Resolves user-supplied additional styleguides (local files or HTTP(S) URLs) into prompt-ready guides.
- `internal/session/session.go`: Resumable discussion (chat) session store: atomic JSON files (one per session) under the user cache dir, caching the review source descriptor, resolved context, `ReviewResult`, and the full message transcript; load/save/list/latest helpers.
- `internal/toolchain/toolchain.go`: Toolchain version capture and normalization.
- `internal/tools/catalog.go`: Tool catalog exposed to agents.
- `internal/textsan/textsan.go`: Text sanitization utilities.
- `internal/testutil/testutil.go`: Shared test fixture and golden-file helpers.

## Model Capability Checks

- `internal/modelcheck/checker.go`: Probes model support for tools, JSON output, JSON schema, reasoning efforts, and retry behavior.
- `internal/modelcheck/cache.go`: Reads/writes cached model capability results keyed by provider/model settings.

## Non-Go Assets

- `prompts/`: Agent system prompts and shared prompt snippets.
- `prompts/styleguides/`: Language/tool style rules injected into review and verification prompts.
- `workflows/`: Embedded workflow YAML definitions.
- `mappings/`: Data backend for file classification: language path/content rules (incl. shebangs), generated-file patterns and markers, trim eviction classes, and styleguide detectors. All detection rules live in the YAML files; the Go code is a generic PatternSet matching engine.
- `assets/`: Static assets used by output or packaging.
- `testdata/`: Fixtures and golden data used by tests.
