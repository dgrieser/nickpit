# Code Structure

This document maps the production Go code. Test files live beside the code they exercise as `*_test.go`; use them as executable examples for expected behavior and edge cases.

## Commands

- `cmd/nickpit/main.go`: Main CLI entry point. Defines commands, flags, profile loading, workflow execution, local review modes, SCM review modes, output selection, publishing, and seed-finding handling.
- `cmd/nickpit-config-example/main.go`: Generator binary that prints the example config from `internal/config`.
- `cmd/nickpit-workflow-example/main.go`: Generator binary that prints the embedded example workflow.

## Configuration

- `internal/config/config.go`: Loads and merges config files, environment variables, profiles, defaults, and CLI overrides.
- `internal/config/example.go`: Provides the checked-in example config text.
- `internal/config/generate.go`: Shared helpers for generator commands.
- `internal/config/profiles.go`: Built-in model/provider profile definitions and capability defaults.

## Review Pipeline

- `internal/review/engine.go`: Core review engine. Builds prompts, runs agent loops, verifies findings, dedupes and merges results, applies filters, handles time-budget retries, and logs progress.
- `internal/review/agent_loop.go`: Generic LLM agent retry loop, validation retry flow, and response parsing.
- `internal/review/pipeline.go`: Pipeline model and execution state for workflow steps, groups, lanes, and result aggregation.
- `internal/review/pipeline_steps.go`: Step implementations for context collection, review lanes, merge, finalize, verdict, summarize, and fused post-merge execution.
- `internal/review/reviewer_session.go`: Reviewer session state, main review execution, nudge handling, and reasoning-mining/update subagents.
- `internal/review/verifier.go`: Per-finding verification agents, verifier options, fallback unverified results, and verifier telemetry.
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
- `internal/retrieval/findlines.go`: Exact code-to-line-number matching backing the `find_lines` tool.
- `internal/retrieval/callgraph.go`: Call hierarchy API and orchestration.
- `internal/retrieval/static_graph.go`: Static call graph storage and lookup.
- `internal/retrieval/symbols.go`: Symbol references and symbol lookup helpers.
- `internal/retrieval/nodejs_backend.go`, `python_backend.go`, `rust_backend.go`: Language-specific retrieval backends.
- `internal/retrieval/goparser/parser.go`: Go parser wrapper for symbols and call information.
- `internal/retrieval/goparser/callgraph.go`: Go call graph extraction.
- `internal/retrieval/repofs/repofs.go`: Filesystem abstraction over repository roots.

## Git and SCM Integrations

- `internal/git/git.go`: Git command wrapper and repository helpers.
- `internal/git/diff.go`: Diff loading and changed-file extraction.
- `internal/git/parser.go`: Git diff parser and hunk model.
- `internal/git/checkout.go`: Temporary checkout/worktree helpers.
- `internal/scm/types.go`: Common SCM interfaces and review publication types.
- `internal/scm/github/adapter.go`: GitHub adapter wiring.
- `internal/scm/github/client.go`: GitHub API client.
- `internal/scm/github/pr.go`: Pull request loading and review source construction.
- `internal/scm/github/position.go`: GitHub inline-comment position mapping.
- `internal/scm/github/publish.go`: GitHub review/comment publishing.
- `internal/scm/gitlab/adapter.go`: GitLab adapter wiring.
- `internal/scm/gitlab/client.go`: GitLab API client.
- `internal/scm/gitlab/mr.go`: Merge request loading and review source construction.
- `internal/scm/gitlab/position.go`: GitLab inline-comment position mapping.
- `internal/scm/gitlab/publish.go`: GitLab review/comment publishing.
- `internal/scm/reviewmd/render.go`: Markdown review report rendering.

## Output, Logging, and Support Packages

- `internal/output/terminal.go`: Human terminal output.
- `internal/output/json.go`: JSON output.
- `internal/output/sarif.go`: SARIF output for code scanning integrations.
- `internal/output/badge.go`: Badge/status formatting helpers.
- `internal/logging/logger.go`: Base logger, reasoning sections, JSON rendering, and raw output.
- `internal/logging/progress.go`: Progress-line data model, formatting, coloring, and workflow labels.
- `internal/logging/reasoning_renderer.go`: Live reasoning renderer for terminal output.
- `internal/logging/verbose.go`: Verbose log blocks, JSON pretty-printing, and context-aware formatting.
- `internal/filetype/language.go`: File extension/content language detection.
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
- `mappings/`: Language and file mapping data.
- `assets/`: Static assets used by output or packaging.
- `testdata/`: Fixtures and golden data used by tests.
