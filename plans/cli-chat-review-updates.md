# Support review corrections in CLI chat

## Summary

CLI discussion gains `request_review_update`. Corrections run in background while conversation continues, update saved session, and publish to GitLab when session targets a GitLab review. Terminal conversation stays local. Previous review versions remain inspectable.

Plan refreshed against `de786d6`, including `cbbb3e2` (bounded scheduler and scoped evidence) and deleted-discussion recovery. The implementation reuses the existing catalog tool, update workflow, scoped GitLab evidence, typed conflicts, publication recovery, and daemon scheduler.

Implemented on this branch: background CLI corrections, GitLab publication, bounded retries and recovery, local session history, `session --history`, and resolved terminal output. Validation includes local CLI and GitLab fixtures, continued conversation and cancellation tests, `make test`, race tests for affected concurrent packages, focused lint, and palette script checks.

## Chat execution

- Wire CLI callback through existing catalog tool and availability gate. Allow one pending update per session; repeated identical calls within initiating turn return same scheduling result. Further requests report pending work.
- Capture review, context options, signal, and an immutable conversation snapshot through the initiating question. Start worker after successful discussion reply; discard reservation if discussion fails.
- Run existing `RunUpdateWorkflow`: evidence assessment, then verdict and summarization only when warranted. Set its new `InitiatingMessage` explicitly to the original question and pass the same snapshot as `DiscussRequest.Messages`; the workflow already forwards it through `VerdictOptions.ContextMessages`. Use separate engine and checkout; configure primary and `@small` clients correctly.
- Follow the scoped-evidence contract in `cmd/nickpit/chat_update_evidence.go`: derive a versioned fingerprint from the exact ordered messages supplied to the workflow. For CLI, source evidence from the session transcript through a fixed message cutoff; later questions, the scheduling reply, and unrelated MR threads must neither enter evidence nor invalidate it. Use session identity and message position, not fabricated GitLab discussion/note IDs.
- Prepare fresh context through existing filtering/toolchain pipeline with `IncludeComments = false`, and clear `ReviewCtx.Comments` defensively. All conversation evidence comes through the selected snapshot. Recheck source diff and review state before applying a correction or reporting a no-op; stale evaluations require fresh context.
- Keep session mutation, persistence, and terminal output on main chat loop. Worker returns completion through channel. Apply completion while idle or after active discussion turn finishes, preserving intervening messages.
- Print and record outcome once; subsequent questions receive corrected review. `/exit`, EOF, and one-shot mode wait for pending work. Ctrl+C cancels and joins worker before checkout cleanup.

## Scheduling and retries

- Keep the CLI worker invocation-scoped, with one pending slot covering lock waits, evaluation, publication, and retry backoff. Do not enqueue CLI work into `serve.UpdateStore`: its job identity and delivery contract require a real initiating GitLab discussion and note.
- For GitLab corrections, acquire the existing `update-execution/<project>` MR lock before loading fresh review/context and hold it through each execution attempt. Reuse `TryLockMR` and `ErrLockBusy`; contention defers work without model calls or retry-budget charges. Release between attempts and use cancellable waits. Keep the adapter's shorter publication lock and propagate the returned lock context.
- Preserve the daemon's existing `update_max_concurrent` limit and per-MR FIFO within its durable queue. CLI uses its own single slot and the shared execution lock; FIFO across independent CLI invocations and daemon queues is not promised.
- Reuse typed `gitlab.UpdateConflict`, `SameReviewState`, and `ValidateReviewState` for GitLab freshness checks. Match current retry policy: separate limits of three execution failures and five conflicts, with `min(counter, 6) * 10s` backoff. Conflicts discard only uncommitted evaluations and rebuild from fresh state without consuming execution-failure budget. CLI counters live for the current invocation; cancellation stops retries.
- Before each retry or final failure, recover uncertain publication and check whether the operation committed. Never discard a possibly staged correction merely because a retry limit was reached. Recoverable publication failures remain distinct from stale pre-publication evaluations.

## Persistence and publication

- Local and GitHub-backed sessions store corrections locally. GitLab-backed sessions load current owned review by review ID and publish through existing `Adapter.UpdateReview`, preserving revision checks, finding identities, resolution rules, comment history, and transaction recovery.
- GitLab publication edits review comments only. Questions, assistant answers, and completion notices stay in CLI/session.
- Use a stable operation ID across retries of one accepted CLI request. Retain the prepared result, before-state, evidence fingerprint, and base/head SHAs in memory; reuse an uncommitted result only when all still match. Use the adapter's durable remote transaction for staged-write recovery. Refresh session from confirmed published result, and recover staged transactions before loading GitLab reviews on later invocations.
- Preserve current recovery order: `RecoverReviewUpdates`, then `ReviewUpdateCommitted`, then decide whether a prepared result remains reusable or must be discarded. Use `SameReviewState` instead of comparing entire results, since local telemetry is absent from GitLab carriers. Remote success followed by a local save failure must not trigger another correction.
- Revalidate selected IDs against fresh review state. Already-resolved findings are skipped; if all selected findings are resolved, return a no-change completion. Missing finding identities fail explicitly, and an emptied selection must never become an overall-review dispute.
- Missing published GitLab review or publication failure produces explicit failure outcome; never claim remote success or silently substitute a local-only correction.
- Save current result, history, refreshed context, and completion message together. Preserve existing session conflict detection; report persistence failures separately from successfully completed remote publication.
- No detached worker or daemon dependency. Work canceled before publication is not automatically resumed; already-staged GitLab transactions retain existing recovery behavior.
- Keep deleted-discussion handling specific to daemon thread jobs. Preserve `ErrDiscussionNotFound`/empty-discussion retirement only after MR access and publication recovery succeed; transient API failures must remain retryable. CLI requests have no initiating remote discussion to retire, and must not create one or run thread-reply/eyes cleanup.

## History and output

- Add optional session `ReviewHistory` entries containing previous review snapshot, replacement timestamp, and correction reason. Existing session files remain compatible.
- Archive only actual changes. Local corrections advance review revision and changed finding revisions; GitLab uses published revisions. No-op assessments add conversation outcome without history entry.
- Add `nickpit session --history`: archived versions oldest-first, including revision, timestamp, reason, and full review. Support existing Markdown/raw/JSON and clipboard modes; reject combination with `--warnings`. Default session output remains current review.
- Render resolved findings with explicit resolved status and explanation, suppressing obsolete body and suggestions. Mirror changed terminal rendering in palette reference script.
- Update CLI help and README with background execution, exit behavior, GitLab publication, and history examples.

## Validation and defaults

- Test scheduling, duplicate requests, pending-work rejection, continued conversation, completion ordering, preserved messages, and updated context on subsequent turns.
- Test that update and verdict receive the same bounded evidence and explicit initiating question; unrelated MR comments and later CLI turns never leak into the request or affect its fingerprint. Retain existing GitLab selected-thread, fallback identity, and cutoff tests.
- Test changed findings, resolutions, overall-review disputes, no-op freshness checks, already-resolved selections, missing IDs, stale context, worker failure, cancellation, shutdown waiting, and session save conflicts.
- Test CLI/daemon and CLI/CLI execution-lock contention, cancellation during lock/backoff waits, separate retry budgets, prepared-result reuse, and invalidation after evidence/diff/review changes. Retain daemon bounded-pool and FIFO tests.
- Test GitLab publication and recovery after ambiguous staging or commit, recovery before retry exhaustion, missing owned review, concurrent remote changes, and absence of mirrored chat posts. Retain deleted/empty-discussion retirement tests and transient API failure coverage without applying thread-only rules to CLI work.
- Test history round-trip, legacy sessions, revision advancement, output modes, clipboard, and resolved rendering.
- Run focused tests, race tests for affected concurrent paths, `make test`, and palette script checks.
- `--no-session` keeps result/history in memory and still permits GitLab publication. Imported JSON remains unchanged. History contains versions observed by this session; earlier remote history is not reconstructed.
