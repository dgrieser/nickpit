# Support review corrections in CLI chat

## Summary

CLI discussion gains `request_review_update`. Corrections run in background while conversation continues, update saved session, and publish to GitLab when session targets a GitLab review. Terminal conversation stays local. Previous review versions remain inspectable.

## Chat execution

- Wire CLI callback through existing catalog tool and availability gate. Allow one pending update per session; repeated identical calls within initiating turn return same scheduling result. Further requests report pending work.
- Capture review, context options, signal, and conversation through initiating question. Start worker after successful discussion reply; discard reservation if discussion fails.
- Run existing `RunUpdateWorkflow`: evidence assessment, then verdict and summarization only when warranted. Use separate engine and checkout; configure primary and `@small` clients correctly.
- Prepare fresh context through existing filtering/toolchain pipeline. Exclude obsolete rendered review comments from evidence. Reject stale results when source diff changes during evaluation.
- Keep session mutation, persistence, and terminal output on main chat loop. Worker returns completion through channel. Apply completion while idle or after active discussion turn finishes, preserving intervening messages.
- Print and record outcome once; subsequent questions receive corrected review. `/exit`, EOF, and one-shot mode wait for pending work. Ctrl+C cancels and joins worker before checkout cleanup.

## Persistence and publication

- Local and GitHub-backed sessions store corrections locally. GitLab-backed sessions load current owned review by review ID and publish through existing `Adapter.UpdateReview`, preserving revision checks, finding identities, resolution rules, comment history, and transaction recovery.
- GitLab publication edits review comments only. Questions, assistant answers, and completion notices stay in CLI/session.
- Use operation ID for GitLab publication and existing recovery/commit checks after uncertain writes. Refresh session from confirmed published result. Recover staged transactions before loading GitLab reviews on later invocations.
- Missing published GitLab review or publication failure produces explicit failure outcome; never claim remote success or silently substitute a local-only correction.
- Save current result, history, refreshed context, and completion message together. Preserve existing session conflict detection; report persistence failures separately from successfully completed remote publication.
- No detached worker or daemon dependency. Work canceled before publication is not automatically resumed; already-staged GitLab transactions retain existing recovery behavior.

## History and output

- Add optional session `ReviewHistory` entries containing previous review snapshot, replacement timestamp, and correction reason. Existing session files remain compatible.
- Archive only actual changes. Local corrections advance review revision and changed finding revisions; GitLab uses published revisions. No-op assessments add conversation outcome without history entry.
- Add `nickpit session --history`: archived versions oldest-first, including revision, timestamp, reason, and full review. Support existing Markdown/raw/JSON and clipboard modes; reject combination with `--warnings`. Default session output remains current review.
- Render resolved findings with explicit resolved status and explanation, suppressing obsolete body and suggestions. Mirror changed terminal rendering in palette reference script.
- Update CLI help and README with background execution, exit behavior, GitLab publication, and history examples.

## Validation and defaults

- Test scheduling, duplicate requests, pending-work rejection, continued conversation, completion ordering, preserved messages, and updated context on subsequent turns.
- Test changed findings, resolutions, overall-review disputes, no-op assessments, stale context, worker failure, cancellation, shutdown waiting, and session save conflicts.
- Test GitLab publication and recovery, missing owned review, concurrent remote changes, and absence of mirrored chat posts.
- Test history round-trip, legacy sessions, revision advancement, output modes, clipboard, and resolved rendering.
- Run focused tests, race tests for affected concurrent paths, `make test`, and palette script checks.
- `--no-session` keeps result/history in memory and still permits GitLab publication. Imported JSON remains unchanged. History contains versions observed by this session; earlier remote history is not reconstructed.
