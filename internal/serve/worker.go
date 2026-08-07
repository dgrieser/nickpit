package serve

import (
	"context"
	"log/slog"
	"time"
)

// settleTimeout bounds the reaction update that ends a review. It is short and
// on a context of its own: the job context is already cancelled when a review
// was aborted or terminated by shutdown, and the flip must still happen without
// holding the shutdown grace open.
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
)

// reactions records the in-progress reactions this job is responsible for, so
// the outcome replaces exactly those and nothing else. Reactions the daemon
// never placed (start emoji disabled, or an auto event that carries no command
// note) are left untouched — an MR the daemon skipped must not be decorated.
type reactions struct {
	// mr is true once the start emoji was awarded on the merge request.
	mr bool
	// notes are the command notes wearing the ack emoji.
	notes []int
}

// process runs one review job end to end: opt-in check, authoritative MR
// recheck, start-emoji award, child process — then replaces the in-progress
// reactions with the outcome. Failures are logged, never fatal — the daemon
// outlives every review.
func (d *Dispatcher) process(ctx context.Context, event Event) {
	log := d.log.With("project", event.ProjectPath, "iid", event.IID, "trigger", event.Kind.String())
	// The command notes already wear the ack emoji: the handler awarded it when
	// it accepted the command, before this job was picked up.
	placed := reactions{notes: event.AckNoteIDs}
	outcome := d.review(ctx, event, &placed, log)
	d.settle(ctx, event, placed, outcome, log)
}

// review is process without the reaction bookkeeping: it reports how the review
// ended and records in placed which reactions it put in place.
func (d *Dispatcher) review(ctx context.Context, event Event, placed *reactions, log *slog.Logger) reviewOutcome {
	if event.Kind == TriggerAuto {
		optedIn, err := d.topics.HasTopic(ctx, event.Group, event.ProjectID, d.cfg.Topic)
		if err != nil {
			log.Error("topic check failed", "error", err)
			return outcomeFailed
		}
		if !optedIn {
			log.Info("skipping review", "reason", "not opted in", "topic", d.cfg.Topic)
			return outcomeFailed
		}
	}

	status, err := event.Group.Client.FetchMRStatus(ctx, event.ProjectID, event.IID)
	if err != nil {
		log.Error("mr status check failed", "error", err)
		return outcomeFailed
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

	if d.cfg.StartEmoji != "" {
		// Revoke a previous run's outcome in the same call: without it an MR
		// re-reviewed after a failure would carry both the old outcome and the
		// new one.
		err := event.Group.Client.ReplaceMREmoji(ctx, event.ProjectID, event.IID, event.Group.BotUserID,
			d.cfg.StartEmoji, d.cfg.DoneEmoji, d.cfg.FailEmoji)
		if err != nil {
			log.Warn("awarding start emoji failed", "emoji", d.cfg.StartEmoji, "error", err)
		}
		// Recorded even when the call failed: it may have awarded and only
		// failed to revoke, and settle's replace is harmless when there is
		// nothing to revoke.
		placed.mr = true
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
	// Per-job cancel while the pool is alive is a user abort, not a failure;
	// a SIGTERM'd child exits non-zero, so this case must come first. The
	// head is not marked reviewed: the same SHA stays re-reviewable.
	case ctx.Err() != nil && d.jobCtx.Err() == nil:
		log.Info("review aborted", "duration", duration, "log", logPath)
		return outcomeAborted
	case err != nil:
		log.Error("review failed to run", "error", err, "duration", duration)
		return outcomeFailed
	case exitCode != 0:
		log.Error("review exited with error", "exit_code", exitCode, "duration", duration, "log", logPath)
		return outcomeFailed
	default:
		// Only a successful run marks the head reviewed: a transient failure
		// must not make later auto events for the same SHA drop as
		// "already reviewed" when nothing was published.
		d.markReviewed(event.ProjectID, event.IID, status.HeadSHA)
		log.Info("review finished", "duration", duration, "log", logPath)
		return outcomeDone
	}
}

// settle replaces the in-progress reactions with the review's outcome, so the MR
// and every comment that asked for the review stop reading as "in progress".
func (d *Dispatcher) settle(ctx context.Context, event Event, placed reactions, outcome reviewOutcome, log *slog.Logger) {
	notes := placed.notes
	if d.cfg.AckEmoji == "" {
		// No ack emoji was awarded, so there is nothing on the notes to replace.
		notes = nil
	}
	if !placed.mr && len(notes) == 0 {
		return
	}
	add := ""
	switch outcome {
	case outcomeDone:
		add = d.cfg.DoneEmoji
	case outcomeFailed:
		add = d.cfg.FailEmoji
	}
	// A cancelled job context (abort, shutdown) must not skip the flip; the
	// in-progress reaction would then be stuck forever.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer cancel()
	client := event.Group.Client
	if placed.mr {
		err := client.ReplaceMREmoji(ctx, event.ProjectID, event.IID, event.Group.BotUserID, add, d.cfg.StartEmoji)
		if err != nil {
			log.Warn("updating merge request emoji failed", "emoji", add, "error", err)
		}
	}
	for _, noteID := range notes {
		err := client.ReplaceNoteEmoji(ctx, event.ProjectID, event.IID, noteID, event.Group.BotUserID, add, d.cfg.AckEmoji)
		if err != nil {
			log.Warn("updating command note emoji failed", "note", noteID, "emoji", add, "error", err)
		}
	}
}
