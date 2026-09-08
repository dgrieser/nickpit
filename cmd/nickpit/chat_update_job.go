package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/review"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/serve"
)

const updateFailed = "I could not complete the review update after repeated failures; please ask again in a new reply."

func (u *gitLabChatUpdate) discussionUpdateHandler() func(context.Context, review.ReviewUpdateSignal) (review.ReviewUpdateToolResult, error) {
	if strings.TrimSpace(u.opts.updateStateDir) == "" {
		return nil
	}
	return u.enqueue
}

func (u *gitLabChatUpdate) enqueue(ctx context.Context, signal review.ReviewUpdateSignal) (review.ReviewUpdateToolResult, error) {
	failed := review.ReviewUpdateToolResult{Status: review.ReviewUpdateQueueFailed}
	if u.queuedJob != nil {
		if reflect.DeepEqual(u.queuedJob.FindingIDs, signal.FindingIDs) && u.queuedJob.Reason == signal.Reason {
			return review.ReviewUpdateToolResult{Status: review.ReviewUpdateScheduled}, nil
		}
		return review.ReviewUpdateToolResult{Status: review.ReviewUpdateError, Message: "An update is already scheduled for this question."}, nil
	}
	if err := signal.Validate(u.request.Result); err != nil {
		return review.ReviewUpdateToolResult{}, err
	}
	if err := u.validate(ctx); err != nil {
		return review.ReviewUpdateToolResult{}, err
	}
	store, err := serve.NewUpdateStore(u.opts.updateStateDir)
	if err != nil {
		return failed, nil
	}
	defer func() { _ = store.Close() }()
	job := &serve.UpdateJob{ProjectPath: u.project, IID: u.iid, BaseURL: u.profile.GitLabBaseURL,
		ReviewID: u.request.Result.ReviewID, DiscussionID: u.opts.replyDiscussion, NoteID: u.pending,
		Requested: u.opts.replyRequested, FindingIDs: signal.FindingIDs, Reason: signal.Reason, Created: time.Now().UTC()}
	for _, note := range u.triggerNotes {
		if note.ID == u.pending {
			job.Question = note.Body
		}
	}
	job.SetID()
	_, release, err := u.adapter.Client().LockMR(ctx, "update-job/"+job.ID, 1)
	if err != nil {
		return review.ReviewUpdateToolResult{}, err
	}
	retained := false
	defer func() {
		if !retained {
			release()
		}
	}()
	existing, err := store.Load(job.ID)
	if err == nil {
		job = existing
	} else if os.IsNotExist(err) {
		if err := store.Save(job); err != nil {
			return failed, nil
		}
	} else {
		return failed, nil
	}
	if job.Done {
		return review.ReviewUpdateToolResult{Status: review.ReviewUpdateError, Message: "An update was already processed for this question."}, nil
	}
	// Let chat finish its normal reply before the worker can follow up. A
	// process exit releases this lock, leaving the durable job runnable.
	u.queuedJob, u.queuedRelease, retained = job, release, true
	return review.ReviewUpdateToolResult{Status: review.ReviewUpdateScheduled}, nil
}

func (a *app) runUpdateJob(ctx context.Context, profile config.Profile, opts chatOptions) error {
	store, err := serve.NewUpdateStore(opts.updateStateDir)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	client := glscm.NewClient(profile.GitLabBaseURL, profile.GitLabToken)
	ctx, release, err := client.LockMR(ctx, "update-job/"+opts.updateJobID, 1)
	if err != nil {
		return err
	}
	defer release()
	job, err := store.Load(opts.updateJobID)
	if err != nil {
		return err
	}
	if job.Done {
		return nil
	}
	if job.ProjectPath != opts.repo || job.IID != opts.mrID || job.DiscussionID != opts.replyDiscussion ||
		strings.TrimRight(job.BaseURL, "/") != strings.TrimRight(profile.GitLabBaseURL, "/") {
		return fmt.Errorf("update job does not match current invocation or GitLab host")
	}
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return err
	}
	adapter := glscm.NewAdapter(client, profile.AssetBaseURL)
	if err := adapter.RecoverReviewUpdates(ctx, job.ProjectPath, job.IID); err != nil {
		return err
	}
	if job.Plan != nil {
		committed, err := adapter.ReviewUpdateCommitted(ctx, job.ProjectPath, job.IID, job.ReviewID, job.ID)
		if err != nil {
			return err
		}
		if committed {
			job.Followup, job.Plan = job.Plan.Followup, nil
			if err := store.Save(job); err != nil {
				return err
			}
		}
	}
	if job.Followup == "" {
		if job.Attempts >= 3 && job.Plan == nil {
			job.Followup = updateFailed
		} else {
			job.Attempts++
			job.Evidence, err = updateEvidence(ctx, client, job.ProjectPath, job.IID, user.ID)
			if err != nil {
				return err
			}
			if err := store.Save(job); err != nil {
				return err
			}
			opts.updateJob, opts.updateStore = job, store
			opts.replyNote, opts.replyRequested = job.NoteID, job.Requested
			err = a.runChatGitLabReply(ctx, profile, opts)
			if errors.Is(err, errChatReplySuppressed) {
				job.Done = true
				return store.Save(job)
			}
			if err != nil || job.Followup == "" {
				if ctx.Err() != nil {
					return ctx.Err()
				} // Keep checkpoint for restart.
				if job.Attempts < 3 || job.Plan != nil {
					job.NextAttempt = time.Now().Add(time.Duration(min(job.Attempts, 6)) * 10 * time.Second)
					if saveErr := store.Save(job); saveErr != nil {
						return saveErr
					}
					if err == nil {
						err = fmt.Errorf("original review or question is unavailable")
					}
					return fmt.Errorf("update attempt incomplete: %w", err)
				}
				job.Followup = updateFailed
			}
		}
		if err := store.Save(job); err != nil {
			return err
		}
	}
	if err := postUpdateJobMessage(ctx, client, job, user.ID, opts, "followup", job.Followup); err != nil {
		if !errors.Is(err, errChatReplySuppressed) {
			return err
		}
	}
	job.Done = true
	return store.Save(job)
}

// postUpdateJobMessage is retry-safe even if the POST succeeds but its response
// is lost. Only bot-authored markers count. No unthreaded fallback can strand a
// follow-up outside the original discussion.
func postUpdateJobMessage(ctx context.Context, client *glscm.Client, job *serve.UpdateJob, bot int, opts chatOptions, phase, text string) error {
	ctx, release, err := client.LockMR(ctx, job.ProjectPath, job.IID)
	if err != nil {
		return err
	}
	defer release()
	notes, err := client.DiscussionNotes(ctx, job.ProjectPath, job.IID, job.DiscussionID)
	if err != nil {
		return err
	}
	if len(notes) == 0 || notes[0].AuthorID != bot {
		return errChatReplySuppressed
	}
	rid, _, ok := reviewmd.DetectThreadReview(notes[0].Body)
	if !ok || rid != job.ReviewID {
		return errChatReplySuppressed
	}
	for _, note := range notes {
		marker := reviewmd.ReadUpdateJobReply(note.Body)
		if note.AuthorID == bot && marker.JobID == job.ID && marker.Phase == phase {
			return nil
		}
	}
	controls := resolveChatMessageControls([]chatMessageControls{{keyword: opts.replyCommandKeyword, skipPhrases: opts.replySkipPhrases}})
	if !chatNoteDirectives(notes, job.NoteID, controls).AllowsReply(job.Requested) {
		return errChatReplySuppressed
	}
	allowed, err := gitLabThreadResponsesAllowed(ctx, client, job.ProjectPath, job.IID, notes, bot, opts.replyMuteEmoji)
	if err != nil {
		return err
	}
	if !allowed {
		return errChatReplySuppressed
	}
	body := reviewmd.EscapeQuickActions(reviewmd.Sanitize(text)) + "\n\n" + reviewmd.UpdateJobReplyMarker(reviewmd.UpdateJobReply{JobID: job.ID, NoteID: job.NoteID, Phase: phase})
	return client.ReplyToMRDiscussionPath(ctx, job.ProjectPath, job.IID, job.DiscussionID, body)
}

func updateEvidence(ctx context.Context, client *glscm.Client, project string, iid, bot int) (string, error) {
	discussions, err := client.MRDiscussions(ctx, project, iid)
	if err != nil {
		return "", err
	}
	type evidence struct {
		ID   int
		Body string
	}
	var items []evidence
	for _, discussion := range discussions {
		for _, note := range discussion.Notes {
			if note.AuthorID != bot && !note.System {
				items = append(items, evidence{note.ID, note.Body})
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	raw, _ := json.Marshal(items)
	return fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}

func (u *gitLabChatUpdate) validateJob(ctx context.Context) error {
	notes, err := u.adapter.Client().DiscussionNotes(ctx, u.project, u.iid, u.job.DiscussionID)
	if err != nil {
		return err
	}
	if len(notes) == 0 || notes[0].AuthorID != u.botUserID {
		return errChatReplySuppressed
	}
	rid, _, ok := reviewmd.DetectThreadReview(notes[0].Body)
	if !ok || rid != u.job.ReviewID {
		return errChatReplySuppressed
	}
	if !chatNoteDirectives(notes, u.job.NoteID, u.controls).AllowsReply(u.job.Requested) {
		return errChatReplySuppressed
	}
	for _, note := range notes {
		if note.ID == u.job.NoteID && note.Body != u.job.Question {
			return fmt.Errorf("original question was edited")
		}
	}
	allowed, err := gitLabThreadResponsesAllowed(ctx, u.adapter.Client(), u.project, u.iid, notes, u.botUserID, u.opts.replyMuteEmoji)
	if err != nil {
		return err
	}
	if !allowed {
		return errChatReplySuppressed
	}
	evidence, err := updateEvidence(ctx, u.adapter.Client(), u.project, u.iid, u.botUserID)
	if err != nil {
		return err
	}
	if evidence != u.job.Evidence {
		return fmt.Errorf("comments changed during update evaluation; retry with fresh context")
	}
	info, err := u.adapter.Client().FetchMRPositionInfo(ctx, u.project, u.iid)
	if err != nil {
		return err
	}
	if info.DiffRefs.HeadSHA != u.request.ReviewCtx.DiffHeadSHA || info.DiffRefs.BaseSHA != u.request.ReviewCtx.DiffBaseSHA {
		return fmt.Errorf("commits changed during update evaluation; retry with fresh context")
	}
	return nil
}

func (u *gitLabChatUpdate) executeJob(ctx context.Context) error {
	if err := u.validateJob(ctx); err != nil {
		return err
	}
	if plan := u.job.Plan; plan != nil {
		current := u.request.Result
		if plan.Evidence == u.job.Evidence && plan.HeadSHA == u.request.ReviewCtx.DiffHeadSHA && plan.BaseSHA == u.request.ReviewCtx.DiffBaseSHA &&
			current.Revision == plan.Before.Revision && reflect.DeepEqual(current.Findings, plan.Before.Findings) &&
			current.OverallCorrectness == plan.Before.OverallCorrectness && current.OverallExplanation == plan.Before.OverallExplanation {
			_, err := u.adapter.UpdateReview(ctx, u.project, u.iid, glscm.ReviewUpdateRequest{Operation: u.job.ID, Before: plan.Before, After: plan.After, HeadSHA: plan.HeadSHA, BaseSHA: plan.BaseSHA, Validate: u.validate})
			if err != nil {
				return err
			}
			u.job.Followup, u.job.Plan = plan.Followup, nil
			return u.store.Save(u.job)
		}
		// No committed operation exists and the evidence changed: discard only
		// the uncommitted decision and evaluate against the newly prepared context.
		u.job.Plan = nil
		if err := u.store.Save(u.job); err != nil {
			return err
		}
	}
	ids := make([]string, 0, len(u.job.FindingIDs))
	for _, id := range u.job.FindingIDs {
		found := false
		for _, finding := range u.request.Result.Findings {
			if finding.ID == id {
				found = true
				if finding.Resolution == nil {
					ids = append(ids, id)
				}
			}
		}
		if !found {
			return fmt.Errorf("finding %s no longer available in original review", id)
		}
	}
	if len(u.job.FindingIDs) > 0 && len(ids) == 0 {
		u.job.Followup = "The requested findings are already resolved; no further changes were needed."
		return u.store.Save(u.job)
	}
	outcome, err := u.run(ctx, review.ReviewUpdateSignal{FindingIDs: ids, Reason: u.job.Reason})
	if err != nil {
		return err
	}
	if u.job.Plan == nil {
		if err := u.validateJob(ctx); err != nil {
			return err
		}
	}
	u.job.Followup, u.job.Plan = updateFollowup(outcome), nil
	return u.store.Save(u.job)
}

func updateFollowup(outcome *review.ReviewUpdateOutcome) string {
	var b strings.Builder
	if len(outcome.Changed) > 0 {
		b.WriteString("Review updated.")
	} else {
		b.WriteString("Review check complete.")
	}
	for _, check := range outcome.Checks {
		fmt.Fprintf(&b, "\n\n%s: %s", check.ID, check.Reason)
	}
	if outcome.ReviewCheck != nil {
		fmt.Fprintf(&b, "\n\n%s", outcome.ReviewCheck.Reason)
	}
	if outcome.OverallCorrectness != "" {
		fmt.Fprintf(&b, "\n\nOverall verdict: %s.", outcome.OverallCorrectness)
	}
	text := []rune(b.String())
	if len(text) > 8000 {
		return string(text[:8000]) + "…"
	}
	return string(text)
}
