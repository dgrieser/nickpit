package serve

import (
	"context"
	"strings"
	"time"
)

// StartUpdateWorker uses a separate bounded worker, so a slow correction never
// occupies chat admission slots. Files are scanned at startup and on each tick;
// no webhook redelivery or in-memory wakeup is required for recovery.
func (h *Handler) StartUpdateWorker() {
	if h.chatRunner == nil || h.chatCfg.UpdateStateDir == "" {
		return
	}
	h.chatWG.Go(func() {
		var store *UpdateStore
		for store == nil {
			h.chatAdmitMu.Lock()
			closed := h.chatClosed
			h.chatAdmitMu.Unlock()
			if closed || h.chatCtx.Err() != nil {
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
		for {
			h.chatAdmitMu.Lock()
			closed := h.chatClosed
			h.chatAdmitMu.Unlock()
			if closed {
				return
			}
			if h.chatCtx.Err() != nil {
				return
			}
			jobs, err := store.Pending()
			if err != nil {
				h.log.Error("reading update jobs", "error", err)
			}
			for _, job := range jobs {
				h.chatAdmitMu.Lock()
				closed := h.chatClosed
				h.chatAdmitMu.Unlock()
				if closed {
					return
				}
				if h.chatCtx.Err() != nil {
					return
				}
				group := h.groups.Match(job.ProjectPath)
				if group == nil || strings.TrimRight(job.BaseURL, "/") != strings.TrimRight(h.chatCfg.BaseURL, "/") {
					h.log.Warn("update job no longer matches configured GitLab group/host", "job", job.ID)
					continue
				}
				ctx, cancel := context.WithTimeout(h.chatCtx, chatEventTimeout)
				if h.responses != nil {
					state, err := h.responses.State(ctx, group, job.ProjectPath, job.IID, job.DiscussionID)
					if err != nil || !state.Allows(job.Requested) {
						cancel()
						continue // Keep durable work pending until policy permits it.
					}
				}
				code, path, err := h.chatRunner.RunChat(ctx, ChatSpec{
					ProjectPath: job.ProjectPath, IID: job.IID, DiscussionID: job.DiscussionID, NoteID: job.NoteID,
					Requested: job.Requested, Token: group.Token, BaseURL: h.chatCfg.BaseURL,
					ConfigPath: h.chatCfg.ConfigPath, ExtraArgs: h.chatCfg.ExtraArgs, LogDir: h.chatCfg.LogDir,
					MuteEmoji: responseMuteEmoji(h.responses), CommandKeyword: h.cfg.CommandKeyword,
					SkipPhrases: responseSkipPhrases(h.responses), UpdateStateDir: h.chatCfg.UpdateStateDir, UpdateJobID: job.ID,
				})
				cancel()
				if err != nil || code != 0 {
					h.log.Warn("update job attempt failed", "job", job.ID, "exit_code", code, "log", path, "error", err)
				}
			}
			select {
			case <-h.chatCtx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	})
}
