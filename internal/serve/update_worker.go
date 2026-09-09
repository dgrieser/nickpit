package serve

import (
	"context"
	"errors"
	"sync"
	"time"

	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

// StartUpdateWorker admits the oldest unfinished job per MR into a bounded pool.
func (h *Handler) StartUpdateWorker() {
	if h.chatRunner == nil || h.chatCfg.UpdateStateDir == "" {
		return
	}
	h.chatAdmitMu.Lock()
	defer h.chatAdmitMu.Unlock()
	if h.chatClosed || h.updateStarted {
		return
	}
	h.updateStarted = true
	interval := h.updatePollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	h.chatWG.Go(func() {
		var workers sync.WaitGroup
		defer workers.Wait()
		var store *UpdateStore
		for store == nil {
			if h.chatStopping() {
				return
			}
			var err error
			store, err = NewUpdateStore(h.chatCfg.UpdateStateDir)
			if err != nil {
				h.log.Error("opening update jobs; will retry", "error", err)
				select {
				case <-h.chatCtx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
		}
		defer func() { _ = store.Close() }()
		limit := h.chatCfg.UpdateMaxConcurrent
		if limit <= 0 {
			limit = 2
		}
		completed := make(chan string, limit)
		active := map[string]bool{}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		// A deferred child must wait for the next tick, avoiding a contention spin.
		deferred := map[string]bool{}
		for {
			if h.chatStopping() {
				return
			}
			jobs, err := store.Unfinished()
			if err != nil {
				h.log.Error("reading update jobs", "error", err)
			}
			seen := map[string]bool{}
			for _, job := range jobs {
				key := job.MRKey()
				if seen[key] {
					continue
				}
				seen[key] = true
				if active[key] || deferred[job.ID] || job.NextAttempt.After(time.Now()) || len(active) >= limit {
					continue
				}
				group := h.groups.Match(job.ProjectPath)
				if group == nil || glscm.NormalizeBaseURL(job.BaseURL) != glscm.NormalizeBaseURL(h.chatCfg.BaseURL) {
					continue
				}
				h.chatAdmitMu.Lock()
				if h.chatClosed {
					h.chatAdmitMu.Unlock()
					return
				}
				active[key] = true
				deferred[job.ID] = true
				workers.Go(func() {
					defer func() { completed <- key }()
					ctx, cancel := context.WithTimeout(h.chatCtx, chatEventTimeout)
					defer cancel()
					if h.responses != nil {
						state, err := h.responses.State(ctx, group, job.ProjectPath, job.IID, job.DiscussionID)
						// Missing discussions need the child to recover publication before
						// retiring the orphan. Other API failures and policy blocks wait.
						missing := errors.Is(err, glscm.ErrDiscussionNotFound) || (err == nil && state.Missing)
						if !missing && (err != nil || !state.Allows(job.Requested)) {
							return
						}
					}
					code, path, err := h.chatRunner.RunChat(ctx, ChatSpec{
						ProjectPath: job.ProjectPath, IID: job.IID, DiscussionID: job.DiscussionID, NoteID: job.NoteID,
						Requested: job.Requested, Token: group.Token, BaseURL: h.chatCfg.BaseURL,
						ConfigPath: h.chatCfg.ConfigPath, ExtraArgs: h.chatCfg.ExtraArgs, LogDir: h.chatCfg.LogDir,
						MuteEmoji: responseMuteEmoji(h.responses), CommandKeyword: h.cfg.CommandKeyword,
						SkipPhrases: responseSkipPhrases(h.responses), UpdateStateDir: h.chatCfg.UpdateStateDir, UpdateJobID: job.ID,
					})
					if err != nil || (code != 0 && code != UpdateDeferredExitCode) {
						h.log.Warn("update job attempt failed", "job", job.ID, "exit_code", code, "log", path, "error", err)
					}
				})
				h.chatAdmitMu.Unlock()
			}
			select {
			case <-h.chatCtx.Done():
				return
			case key := <-completed:
				delete(active, key)
			case <-ticker.C:
				clear(deferred)
			}
		}
	})
}

func (h *Handler) chatStopping() bool {
	h.chatAdmitMu.Lock()
	defer h.chatAdmitMu.Unlock()
	return h.chatClosed || h.chatCtx.Err() != nil
}
