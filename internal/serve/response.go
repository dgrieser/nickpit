package serve

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

// ResponseConfig is the serve daemon's discussion-response policy.
type ResponseConfig struct {
	Enabled        bool
	OptIn          bool
	MuteEmoji      string
	RequestEmoji   string
	CommandKeyword string
	SkipPhrases    []string
}

// ThreadResponseState is the current effective state of one nickpit review
// thread, assembled from config, persistent root metadata, and live reactions.
type ThreadResponseState struct {
	Ours         bool
	Root         gitlab.DiscussionNote
	DiscussionID string
	Status       reviewmd.ResponseStatus
}

// Allows reports whether this event may invoke the discussion agent.
func (s ThreadResponseState) Allows(requested bool) bool {
	status := s.Status
	return s.Ours && status.Enabled && !status.CommandMuted && !status.MRMuted &&
		!status.ThreadMuted && (!status.OptIn || requested)
}

// ResponseController keeps remote response state coherent. GitLab is the
// source of truth, so restarts require no local state migration.
type ResponseController struct {
	cfg   ResponseConfig
	locks keyedMutex
	log   *slog.Logger
}

func NewResponseController(cfg ResponseConfig, log *slog.Logger) *ResponseController {
	return &ResponseController{cfg: cfg, log: log}
}

func (c *ResponseController) Config() ResponseConfig { return c.cfg }

// policyStatus is the configuration half of a rendered footer, before any live
// per-thread state is layered on.
func (c *ResponseController) policyStatus() reviewmd.ResponseStatus {
	return reviewmd.ResponseStatus{
		Enabled:        c.cfg.Enabled,
		OptIn:          c.cfg.OptIn,
		MuteEmoji:      c.cfg.MuteEmoji,
		RequestEmoji:   c.cfg.RequestEmoji,
		CommandKeyword: c.cfg.CommandKeyword,
	}
}

func (c *ResponseController) lockKey(project string, iid int) string {
	return fmt.Sprintf("%s!%d", project, iid)
}

// State reads and verifies one discussion root and its live response policy.
func (c *ResponseController) State(ctx context.Context, group *Group, project string, iid int, discussionID string) (ThreadResponseState, error) {
	unlock := c.locks.lock(c.lockKey(project, iid))
	defer unlock()
	return c.stateLocked(ctx, group, project, iid, discussionID, nil)
}

func (c *ResponseController) stateLocked(ctx context.Context, group *Group, project string, iid int, discussionID string, mrMuted *bool) (ThreadResponseState, error) {
	var state ThreadResponseState
	if group == nil || group.BotUserID == 0 || discussionID == "" {
		return state, nil
	}
	notes, err := group.Client.DiscussionNotes(ctx, project, iid, discussionID)
	if err != nil {
		return state, err
	}
	if len(notes) == 0 || notes[0].AuthorID != group.BotUserID {
		return state, nil
	}
	if _, _, ok := reviewmd.DetectThreadReview(notes[0].Body); !ok || reviewmd.StripMarkers(notes[0].Body) == "" {
		return state, nil
	}
	state.Ours = true
	state.Root = notes[0]
	state.DiscussionID = discussionID
	state.Status = c.policyStatus()
	state.Status.CommandMuted = reviewmd.ThreadCommandMuted(notes[0].Body)
	if c.cfg.MuteEmoji == "" {
		return state, nil
	}
	if mrMuted == nil {
		awards, err := group.Client.MREmojis(ctx, project, iid)
		if err != nil {
			return ThreadResponseState{}, err
		}
		muted := hasHumanEmoji(awards, c.cfg.MuteEmoji, group.BotUserID)
		mrMuted = &muted
	}
	state.Status.MRMuted = *mrMuted
	awards, err := group.Client.NoteEmojis(ctx, project, iid, notes[0].ID)
	if err != nil {
		return ThreadResponseState{}, err
	}
	state.Status.ThreadMuted = hasHumanEmoji(awards, c.cfg.MuteEmoji, group.BotUserID)
	return state, nil
}

func hasHumanEmoji(awards []gitlab.AwardEmoji, name string, botUserID int) bool {
	for _, award := range awards {
		if award.Name == name && award.User.ID != botUserID {
			return true
		}
	}
	return false
}

// SetCommandMuted persists a thread command and refreshes its footer. It
// reports ours=false for unrelated discussions.
func (c *ResponseController) SetCommandMuted(ctx context.Context, group *Group, project string, iid int, discussionID string, muted bool) (ThreadResponseState, error) {
	unlock := c.locks.lock(c.lockKey(project, iid))
	defer unlock()
	state, err := c.stateLocked(ctx, group, project, iid, discussionID, nil)
	if err != nil || !state.Ours {
		return state, err
	}
	state.Status.CommandMuted = muted
	updated := reviewmd.UpsertResponseFooter(state.Root.Body, state.Status)
	if updated != state.Root.Body {
		if err := group.Client.UpdateMRDiscussionNote(ctx, project, iid, discussionID, state.Root.ID, updated); err != nil {
			return ThreadResponseState{}, err
		}
		state.Root.Body = updated
	}
	return state, nil
}

// SyncThread reconciles one root's visible footer with current reactions and
// config while preserving its persistent command marker.
func (c *ResponseController) SyncThread(ctx context.Context, group *Group, project string, iid int, discussionID string) error {
	unlock := c.locks.lock(c.lockKey(project, iid))
	defer unlock()
	state, err := c.stateLocked(ctx, group, project, iid, discussionID, nil)
	if err != nil || !state.Ours {
		return err
	}
	updated := reviewmd.UpsertResponseFooter(state.Root.Body, state.Status)
	if updated == state.Root.Body {
		return nil
	}
	return group.Client.UpdateMRDiscussionNote(ctx, project, iid, discussionID, state.Root.ID, updated)
}

// SyncReactedRoot refreshes a thread only when the reacted note is its
// bot-authored review root. Reactions on user replies must never mute a thread.
func (c *ResponseController) SyncReactedRoot(ctx context.Context, group *Group, project string, iid int, discussionID string, noteID int) (bool, error) {
	unlock := c.locks.lock(c.lockKey(project, iid))
	defer unlock()
	state, err := c.stateLocked(ctx, group, project, iid, discussionID, nil)
	if err != nil || !state.Ours || state.Root.ID != noteID {
		return false, err
	}
	updated := reviewmd.UpsertResponseFooter(state.Root.Body, state.Status)
	if updated == state.Root.Body {
		return true, nil
	}
	return true, group.Client.UpdateMRDiscussionNote(ctx, project, iid, discussionID, state.Root.ID, updated)
}

// SyncMR refreshes every visible bot-authored nickpit review root on an MR.
// Hidden carrier notes and ordinary bot replies are excluded. Use it for
// MR-wide state changes (a mute reaction on the MR itself), where every root's
// rendered status can have changed.
func (c *ResponseController) SyncMR(ctx context.Context, group *Group, project string, iid int) error {
	return c.syncMR(ctx, group, project, iid, false)
}

// SyncNewRoots stamps only roots whose footer does not already match the
// current response CONFIGURATION. A review publishes new discussions and never
// rewrites older roots, so after a publish this normally costs one reaction
// read per NEW root instead of one per root the MR has ever accumulated — the
// full scan stays reserved for MR-wide state changes. A settings change (a
// renamed mute emoji, chat switched off) changes the stamped fingerprint, so
// roots advertising controls the daemon no longer honors are re-stamped
// instead of skipped forever. A root left unstamped by a failed read is
// retried by the next publish, because a non-matching footer is exactly the
// selection criterion.
func (c *ResponseController) SyncNewRoots(ctx context.Context, group *Group, project string, iid int) error {
	return c.syncMR(ctx, group, project, iid, true)
}

func (c *ResponseController) syncMR(ctx context.Context, group *Group, project string, iid int, onlyMissingFooter bool) error {
	unlock := c.locks.lock(c.lockKey(project, iid))
	defer unlock()
	discussions, err := group.Client.MRDiscussions(ctx, project, iid)
	if err != nil {
		return err
	}
	type reviewRoot struct {
		discussionID string
		note         gitlab.DiscussionNote
	}
	policy := c.policyStatus()
	var roots []reviewRoot
	for _, discussion := range discussions {
		if len(discussion.Notes) == 0 {
			continue
		}
		root := discussion.Notes[0]
		if root.AuthorID != group.BotUserID || reviewmd.StripMarkers(root.Body) == "" {
			continue
		}
		if _, _, ok := reviewmd.DetectThreadReview(root.Body); !ok {
			continue
		}
		if onlyMissingFooter && reviewmd.FooterMatchesPolicy(root.Body, policy) {
			continue
		}
		roots = append(roots, reviewRoot{discussionID: discussion.ID, note: root})
	}
	if len(roots) == 0 {
		return nil
	}
	mrMuted := false
	if c.cfg.MuteEmoji != "" {
		awards, err := group.Client.MREmojis(ctx, project, iid)
		if err != nil {
			return err
		}
		mrMuted = hasHumanEmoji(awards, c.cfg.MuteEmoji, group.BotUserID)
	}
	var errs []error
	for _, root := range roots {
		status := policy
		status.CommandMuted = reviewmd.ThreadCommandMuted(root.note.Body)
		status.MRMuted = mrMuted
		if c.cfg.MuteEmoji != "" {
			awards, emojiErr := group.Client.NoteEmojis(ctx, project, iid, root.note.ID)
			if emojiErr != nil {
				errs = append(errs, emojiErr)
				continue
			}
			status.ThreadMuted = hasHumanEmoji(awards, c.cfg.MuteEmoji, group.BotUserID)
		}
		updated := reviewmd.UpsertResponseFooter(root.note.Body, status)
		if updated == root.note.Body {
			continue
		}
		if err := group.Client.UpdateMRDiscussionNote(ctx, project, iid, root.discussionID, root.note.ID, updated); err != nil {
			errs = append(errs, err)
		}
	}
	return joinResponseErrors(errs)
}

func joinResponseErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return fmt.Errorf("response footer sync: %s", strings.Join(parts, "; "))
}
