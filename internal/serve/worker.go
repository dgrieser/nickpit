package serve

import (
	"context"
	"time"
)

// process runs one review job end to end: opt-in check, authoritative MR
// recheck, start-emoji award, child process. Failures are logged, never
// fatal — the daemon outlives every review.
func (d *Dispatcher) process(ctx context.Context, event Event) {
	log := d.log.With("project", event.ProjectPath, "iid", event.IID, "trigger", event.Kind.String())

	if event.Kind == TriggerAuto {
		optedIn, err := d.topics.HasTopic(ctx, event.Group, event.ProjectID, d.cfg.Topic)
		if err != nil {
			log.Error("topic check failed", "error", err)
			return
		}
		if !optedIn {
			log.Info("skipping review", "reason", "not opted in", "topic", d.cfg.Topic)
			return
		}
	}

	status, err := event.Group.Client.FetchMRStatus(ctx, event.ProjectID, event.IID)
	if err != nil {
		log.Error("mr status check failed", "error", err)
		return
	}
	if status.State != "opened" {
		log.Info("skipping review", "reason", "state "+status.State)
		return
	}
	// Drafts block auto reviews only; a manual emoji request is an explicit
	// human override.
	if status.Draft && event.Kind == TriggerAuto {
		log.Info("skipping review", "reason", "draft")
		return
	}
	if event.Kind == TriggerAuto && d.alreadyReviewed(event.ProjectID, event.IID, status.HeadSHA) {
		log.Info("skipping review", "reason", "head already reviewed", "sha", status.HeadSHA)
		return
	}

	if d.cfg.StartEmoji != "" {
		if err := event.Group.Client.AwardMREmoji(ctx, event.ProjectID, event.IID, d.cfg.StartEmoji); err != nil {
			log.Warn("awarding start emoji failed", "emoji", d.cfg.StartEmoji, "error", err)
		}
	}

	spec := ReviewSpec{
		ProjectPath: event.ProjectPath,
		IID:         event.IID,
		Token:       event.Group.Token,
		BaseURL:     d.cfg.BaseURL,
		ConfigPath:  d.cfg.ConfigPath,
		ExtraArgs:   d.cfg.ExtraArgs,
		LogDir:      d.cfg.LogDir,
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
	case err != nil:
		log.Error("review failed to run", "error", err, "duration", duration)
	case exitCode != 0:
		log.Error("review exited with error", "exit_code", exitCode, "duration", duration, "log", logPath)
	default:
		// Only a successful run marks the head reviewed: a transient failure
		// must not make later auto events for the same SHA drop as
		// "already reviewed" when nothing was published.
		d.markReviewed(event.ProjectID, event.IID, status.HeadSHA)
		log.Info("review finished", "duration", duration, "log", logPath)
	}
}
