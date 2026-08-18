package serve

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/scm/gitlab"
)

// settleTimeout bounds EACH reaction flip that ends a review (the flips run
// concurrently, so it is a per-request budget, not a shared one). It is short
// and on a context of its own: the job context is already cancelled when a
// review was aborted or terminated by shutdown, and the flip must still happen
// without holding the shutdown grace open.
const settleTimeout = 15 * time.Second

// reviewOutcome is how a review ended, which decides the reaction it leaves
// behind.
type reviewOutcome int

const (
	// outcomeDone is a review that ran to completion and published.
	outcomeDone reviewOutcome = iota
	// outcomeFailed is a review that could not be delivered: a failed API
	// check, a policy that rules the MR out after the request was
	// acknowledged, a spawn failure, or a non-zero child exit.
	outcomeFailed
	// outcomeAborted is a review the user cancelled (abort command or trigger
	// emoji revoke). It leaves no outcome reaction: the in-progress one is only
	// revoked, because nothing went wrong.
	outcomeAborted
	// outcomeInterrupted is accepted work terminated by root shutdown. It is
	// never settled: its runnable journal snapshot resumes after restart.
	outcomeInterrupted
)

// reactions records the in-progress reactions this job is responsible for, so
// the outcome replaces exactly those and nothing else. Reactions the daemon
// never placed (start emoji disabled, or an auto event that carries no command
// note) are left untouched — an MR the daemon skipped must not be decorated.
type reactions struct {
	// mr is true once this worker attempted the start emoji, or when a restored
	// journal contains a name recorded at that attempt boundary.
	mr bool
	// notes are the command notes wearing the ack emoji.
	notes []int
}

// reactionSettlement separates a real retryable remote failure from a target
// retained only for a timed-out award's final uncertainty sweep.
// Durable cleanup uses the distinction to back off outages without delaying a
// known deadline, and carries any server-provided rate-limit time with it.
type reactionSettlement struct {
	remaining        reactions
	retryableFailure bool
	retryAfter       time.Time
}

// settlementResult carries the remaining reaction work and identifies the
// pre-settlement cleanup snapshot that finish may retire or reduce.
type settlementResult struct {
	event          Event
	outcome        reviewOutcome
	settled        bool
	cleanupVersion uint64
	resume         bool
}

// process runs one review job end to end: opt-in check, authoritative MR
// recheck, start-emoji award, child process — then replaces the in-progress
// reactions with the outcome. Failures are logged, never fatal — the daemon
// outlives every review.
func (d *Dispatcher) process(ctx context.Context, event Event) settlementResult {
	log := d.log.With("project", event.ProjectPath, "iid", event.IID, "trigger", event.Kind.String())
	// The command notes already wear the ack emoji: the handler awarded it when
	// it accepted the command, before this job was picked up.
	placed := reactions{mr: eventSettlesMR(event), notes: event.AckNoteIDs}
	outcome := d.review(ctx, &event, &placed, log)
	if outcome == outcomeInterrupted {
		return settlementResult{event: event, outcome: outcome, resume: true}
	}
	cleanupVersion := d.beginSettlement(event, placed, outcome)
	remaining := d.settle(context.WithoutCancel(ctx), event, placed, outcome, log)
	return settlementResult{
		event:          eventWithReactions(event, remaining),
		outcome:        outcome,
		settled:        !hasReactions(remaining),
		cleanupVersion: cleanupVersion,
	}
}

// review is process without the reaction bookkeeping: it reports how the review
// ended and records in placed which reactions it put in place.
func (d *Dispatcher) review(ctx context.Context, event *Event, placed *reactions, log *slog.Logger) reviewOutcome {
	if event.Kind == TriggerAuto {
		optedIn, err := d.topics.HasTopic(ctx, event.Group, event.ProjectID, d.cfg.Topic)
		if err != nil {
			return d.preRunFailure(ctx, log, "topic check", err)
		}
		if !optedIn {
			log.Info("skipping review", "reason", "not opted in", "topic", d.cfg.Topic)
			return outcomeFailed
		}
	}

	status, err := event.Group.Client.FetchMRStatus(ctx, event.ProjectID, event.IID)
	if err != nil {
		return d.preRunFailure(ctx, log, "mr status check", err)
	}
	if status.State != "opened" {
		log.Info("skipping review", "reason", "state "+status.State)
		return outcomeFailed
	}
	// Drafts block auto reviews only; a manual emoji request is an explicit
	// human override.
	if status.Draft && event.Kind == TriggerAuto {
		log.Info("skipping review", "reason", "draft")
		return outcomeFailed
	}
	if event.Kind == TriggerAuto && d.alreadyReviewed(event.ProjectID, event.IID, status.HeadSHA) {
		log.Info("skipping review", "reason", "head already reviewed", "sha", status.HeadSHA)
		return outcomeFailed
	}

	if event.Group.BotUserID != 0 {
		// Persist crash-cleanup metadata at the last possible local boundary:
		// after all skip checks, immediately before the remote request.
		d.recordStartReactionAttempt(event)
		placed.mr = true
		// Revoke every previous bot-owned status reaction in the same call,
		// including outcomes left under older configurations. Preserve the
		// trigger reaction: if the bot itself requested this review, revoking it
		// would emit an abort event. An empty start emoji makes this revoke-only;
		// settle repeats cleanup and never adds an MR outcome in that mode.
		err := event.Group.Client.ReplaceOwnMREmoji(ctx, event.ProjectID, event.IID, event.Group.BotUserID,
			d.cfg.StartEmoji, d.cfg.TriggerEmoji)
		if err != nil && d.cfg.StartEmoji != "" && uncertainReactionRequest(err) {
			window := d.startReactionUncertaintyWindow
			if window <= 0 {
				window = reactionUncertaintyWindow
			}
			d.recordStartReactionUncertainty(event, time.Now().Add(window))
		}
		if err != nil {
			if d.cfg.StartEmoji == "" {
				log.Warn("clearing previous merge request emoji failed", "error", err)
			} else {
				log.Warn("awarding start emoji failed", "emoji", d.cfg.StartEmoji, "error", err)
			}
		}
		// Recorded even when the call failed: it may have awarded and only
		// failed to revoke, and settle's replace is harmless when there is
		// nothing to revoke.
	}

	spec := ReviewSpec{
		ProjectPath: event.ProjectPath,
		IID:         event.IID,
		Token:       event.Group.Token,
		BaseURL:     d.cfg.BaseURL,
		ConfigPath:  d.cfg.ConfigPath,
		ExtraArgs:   d.cfg.ExtraArgs,
		LogDir:      d.cfg.LogDir,
		HeadSHA:     status.HeadSHA,
		Trigger:     event.Kind.String(),
	}
	log.Info("review starting", "sha", status.HeadSHA)
	start := time.Now()
	exitCode, logPath, err := d.runner.Run(ctx, spec)
	duration := time.Since(start).Round(time.Second)
	switch {
	// A live per-job cancellation is an explicit user abort, even if a runner
	// ignores cancellation and reports zero. Check it before successful exit.
	case ctx.Err() != nil && d.jobCtx.Err() == nil:
		log.Info("review aborted", "duration", duration, "log", logPath)
		return outcomeAborted
	// Once the child reports successful delivery, a root cancellation that
	// lands while its output is still draining must not make the work runnable
	// again after restart.
	case err == nil && exitCode == 0:
		d.markReviewed(event.ProjectID, event.IID, status.HeadSHA)
		if d.responses != nil {
			syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), commandReplyTimeout)
			// Only roots this publish added lack a footer, so post-publish work
			// stays proportional to NEW findings instead of every root the MR has
			// accumulated. MR-wide state changes still trigger a full SyncMR.
			if syncErr := d.responses.SyncNewRoots(syncCtx, event.Group, event.ProjectPath, event.IID); syncErr != nil {
				log.Warn("syncing review response footers failed", "error", syncErr)
			}
			cancel()
		}
		log.Info("review finished", "duration", duration, "log", logPath)
		return outcomeDone
	// Root shutdown cancellation preserves accepted work for journal resume.
	// It must not publish a failed reaction or retire the runnable snapshot.
	case ctx.Err() != nil && d.jobCtx.Err() != nil:
		log.Info("review interrupted by shutdown", "duration", duration, "log", logPath)
		return outcomeInterrupted
	case err != nil:
		log.Error("review failed to run", "error", err, "duration", duration)
		return outcomeFailed
	case exitCode != 0:
		log.Error("review exited with error", "exit_code", exitCode, "duration", duration, "log", logPath)
		return outcomeFailed
	default:
		log.Error("review returned invalid result", "exit_code", exitCode, "duration", duration, "log", logPath)
		return outcomeFailed
	}
}

// preRunFailure classifies an error from a step before the child process ran.
// A per-job cancel while the pool is alive is the user's abort landing mid-step,
// not a failure — the fail reaction would tell the asker something went wrong
// when nothing did. Everything else is the step failing.
func (d *Dispatcher) preRunFailure(ctx context.Context, log *slog.Logger, step string, err error) reviewOutcome {
	if ctx.Err() != nil && d.jobCtx.Err() != nil {
		log.Info("review interrupted by shutdown", "during", step)
		return outcomeInterrupted
	}
	if ctx.Err() != nil && d.jobCtx.Err() == nil {
		log.Info("review aborted", "during", step)
		return outcomeAborted
	}
	log.Error(step+" failed", "error", err)
	return outcomeFailed
}

// settle replaces the in-progress reactions with the review's outcome, so the MR
// and every comment that asked for the review stop reading as "in progress".
func (d *Dispatcher) settle(ctx context.Context, event Event, placed reactions, outcome reviewOutcome, log *slog.Logger) reactions {
	return d.settleAttemptWithLimit(ctx, event, placed, outcome, log, nil).remaining
}

// settleWithLimit optionally caps live remote reaction requests across several
// concurrent settle calls. A nil channel retains the normal per-job fan-out.
func (d *Dispatcher) settleWithLimit(
	ctx context.Context,
	event Event,
	placed reactions,
	outcome reviewOutcome,
	log *slog.Logger,
	reactionSlots chan struct{},
) reactions {
	return d.settleAttemptWithLimit(ctx, event, placed, outcome, log, reactionSlots).remaining
}

func (d *Dispatcher) settleAttemptWithLimit(
	ctx context.Context,
	event Event,
	placed reactions,
	outcome reviewOutcome,
	log *slog.Logger,
	reactionSlots chan struct{},
) reactionSettlement {
	// Safe replacement needs the token owner's user id. Production startup
	// requires it; keep this guard for tests and defensive direct callers so a
	// lookup failure can never add an outcome beside an unrevokable marker.
	if event.Group == nil || event.Group.BotUserID == 0 {
		return reactionSettlement{}
	}
	notes := placed.notes
	ackEmojis := appendUniqueStrings(event.AckEmojis, d.cfg.AckEmoji)
	if len(ackEmojis) == 0 {
		// No ack emoji was awarded, so there is nothing on the notes to replace.
		notes = nil
	}
	if !placed.mr && len(notes) == 0 {
		return reactionSettlement{}
	}
	add := ""
	switch outcome {
	case outcomeDone:
		add = d.cfg.DoneEmoji
	case outcomeFailed:
		add = d.cfg.FailEmoji
	}
	mrAdd := add
	if d.cfg.StartEmoji == "" || event.RevokeMROnly {
		mrAdd = ""
	}
	// Flips are independent and run concurrently, each on its own deadline: one
	// slow request must not eat the budget of remaining notes (a coalesced review
	// settles up to maxAckNotes of them). process passes a cancellation-free
	// context so an aborted review still settles; cleanup workers pass their
	// cancellable context so shutdown can bound queue draining.
	client := event.Group.Client
	var wg sync.WaitGroup
	mrFailed := false
	mrRetryable := false
	mrTerminal := false
	var mrRetryAfter time.Time
	noteFailed := make([]bool, len(notes))
	noteRetryable := make([]bool, len(notes))
	noteRetryAfter := make([]time.Time, len(notes))
	if placed.mr {
		wg.Go(func() {
			completed := withReactionSlot(ctx, reactionSlots, func() {
				requestCtx, cancel := context.WithTimeout(ctx, settleTimeout)
				defer cancel()
				err := client.ReplaceOwnMREmoji(requestCtx, event.ProjectID, event.IID, event.Group.BotUserID, mrAdd, d.cfg.TriggerEmoji)
				if err != nil && mrAdd != "" && reactionOutcomeRejected(err) {
					// A bad configured outcome cannot improve on retry. Revoke the
					// in-progress marker so a terminal POST validation response does
					// not leave the MR looking active forever.
					log.Warn("outcome emoji rejected; revoking merge request marker", "emoji", mrAdd, "error", err)
					err = client.ReplaceOwnMREmoji(requestCtx, event.ProjectID, event.IID, event.Group.BotUserID, "", d.cfg.TriggerEmoji)
				}
				if err != nil {
					if terminalReactionTargetError(err) {
						mrTerminal = true
						log.Warn("stopping merge request emoji retries after terminal error", "emoji", mrAdd, "error", err)
						return
					}
					mrFailed = true
					mrRetryable = true
					mrRetryAfter = reactionRetryAfter(err)
					log.Warn("updating merge request emoji failed", "emoji", mrAdd, "error", err)
				}
			})
			if !completed {
				mrFailed = true
				mrRetryable = true
				return
			}
			// A cancelled/timed-out start POST can commit after this successful
			// replacement observed no marker. Retain MR for final deadline sweep.
			if !mrTerminal && !event.StartCleanupUntil.IsZero() && time.Now().Before(event.StartCleanupUntil) {
				mrFailed = true
			}
		})
	}
	for index, noteID := range notes {
		wg.Go(func() {
			terminal := false
			completed := withReactionSlot(ctx, reactionSlots, func() {
				requestCtx, cancel := context.WithTimeout(ctx, settleTimeout)
				defer cancel()
				err := client.ReplaceOwnNoteEmoji(requestCtx, event.ProjectID, event.IID, noteID, event.Group.BotUserID, add)
				if err != nil && add != "" && reactionOutcomeRejected(err) {
					// As above, an invalid outcome must degrade to revoke-only cleanup
					// rather than strand the command acknowledgement.
					log.Warn("outcome emoji rejected; revoking command note marker", "note", noteID, "emoji", add, "error", err)
					err = client.ReplaceOwnNoteEmoji(requestCtx, event.ProjectID, event.IID, noteID, event.Group.BotUserID, "")
				}
				if err != nil {
					if terminalReactionTargetError(err) {
						terminal = true
						log.Warn("stopping command note emoji retries after terminal error", "note", noteID, "emoji", add, "error", err)
						return
					}
					noteFailed[index] = true
					noteRetryable[index] = true
					noteRetryAfter[index] = reactionRetryAfter(err)
					log.Warn("updating command note emoji failed", "note", noteID, "emoji", add, "error", err)
				}
			})
			if !completed {
				noteFailed[index] = true
				noteRetryable[index] = true
				return
			}
			// A timed-out award can commit after a successful replacement that
			// observed no acknowledgement. Keep this target for a later pass at
			// the end of the uncertainty window; that pass removes any delayed
			// award while preserving the outcome reaction.
			if !terminal && slices.Contains(event.UncertainAckNoteIDs, noteID) && time.Now().Before(event.AckCleanupUntil) {
				noteFailed[index] = true
			}
		})
	}
	wg.Wait()
	result := reactionSettlement{
		remaining:        reactions{mr: mrFailed},
		retryableFailure: mrRetryable,
		retryAfter:       mrRetryAfter,
	}
	for index, failed := range noteFailed {
		if failed {
			result.remaining.notes = append(result.remaining.notes, notes[index])
		}
		if noteRetryable[index] {
			result.retryableFailure = true
			if noteRetryAfter[index].After(result.retryAfter) {
				result.retryAfter = noteRetryAfter[index]
			}
		}
	}
	return result
}

func withReactionSlot(ctx context.Context, slots chan struct{}, fn func()) bool {
	if slots == nil {
		fn()
		return true
	}
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
		fn()
		return true
	case <-ctx.Done():
		return false
	}
}

// reactionOutcomeRejected identifies GitLab's permanent validation responses
// for an award POST. Callers can then fall back to revoke-only cleanup instead
// of either preserving the in-progress marker or retrying an invalid outcome.
func reactionOutcomeRejected(err error) bool {
	return matchingAPIError(err, func(apiErr *gitlab.APIError) bool {
		return apiErr.Method == http.MethodPost &&
			(apiErr.Status == http.StatusBadRequest || apiErr.Status == http.StatusUnprocessableEntity)
	})
}

// matchingAPIError walks both ordinary wrapped errors and errors.Join trees.
// Replacement can report a list failure and an award failure together, so
// errors.As alone may stop at an unrelated first API response.
func matchingAPIError(err error, match func(*gitlab.APIError) bool) bool {
	if err == nil {
		return false
	}
	if apiErr, ok := err.(*gitlab.APIError); ok {
		return match(apiErr)
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if matchingAPIError(child, match) {
				return true
			}
		}
		return false
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return matchingAPIError(wrapped.Unwrap(), match)
	}
	return false
}

// reactionRetryAfter returns the latest server-requested retry time from an
// ordinary wrapped error or errors.Join tree.
func reactionRetryAfter(err error) time.Time {
	if err == nil {
		return time.Time{}
	}
	var retryAfter time.Time
	if apiErr, ok := err.(*gitlab.APIError); ok && apiErr.RetryAfter.After(retryAfter) {
		retryAfter = apiErr.RetryAfter
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if childRetryAfter := reactionRetryAfter(child); childRetryAfter.After(retryAfter) {
				retryAfter = childRetryAfter
			}
		}
		return retryAfter
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		if childRetryAfter := reactionRetryAfter(wrapped.Unwrap()); childRetryAfter.After(retryAfter) {
			retryAfter = childRetryAfter
		}
	}
	return retryAfter
}

// terminalReactionTargetError identifies a target that no longer exists. Other
// responses remain retryable: notably 400/422 can mean an invalid outcome POST,
// while the old marker still exists and needs revoke-only cleanup.
func terminalReactionTargetError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var apiErr *gitlab.APIError
	return errors.As(err, &apiErr) &&
		(apiErr.Status == http.StatusNotFound || apiErr.Status == http.StatusGone)
}

func uncertainReactionRequest(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func hasReactions(placed reactions) bool {
	return placed.mr || len(placed.notes) > 0
}

func eventSettlesMR(event Event) bool {
	// StartEmojis keeps old journal entries compatible; SettleMR covers
	// revoke-only operation when start reactions are disabled.
	return event.SettleMR || len(event.StartEmojis) > 0
}

// eventWithReactions reduces durable cleanup state to targets that still need
// work. Successful targets are not repeatedly updated on every retry.
func eventWithReactions(event Event, remaining reactions) Event {
	if !remaining.mr {
		event.SettleMR = false
		event.RevokeMROnly = false
		event.StartEmojis = nil
		event.StartCleanupUntil = time.Time{}
	} else {
		event.SettleMR = true
	}
	event.AckNoteIDs = remaining.notes
	event.UncertainAckNoteIDs = slices.DeleteFunc(event.UncertainAckNoteIDs, func(noteID int) bool {
		return !slices.Contains(remaining.notes, noteID)
	})
	if len(event.UncertainAckNoteIDs) == 0 {
		event.AckCleanupUntil = time.Time{}
	}
	return event
}
