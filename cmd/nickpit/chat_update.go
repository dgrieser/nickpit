package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/review"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/serve"
)

type gitLabChatUpdate struct {
	queuedJob               *serve.UpdateJob
	queuedRelease           func()
	app                     *app
	engine                  *review.Engine
	adapter                 *glscm.Adapter
	profile                 config.Profile
	opts                    chatOptions
	project                 string
	iid, botUserID, pending int
	controls                chatMessageControls
	request                 review.DiscussRequest
	triggerNotes            []glscm.DiscussionNote
}

type gitLabUpdateExecution struct {
	*gitLabChatUpdate
	job   *serve.UpdateJob
	store *serve.UpdateStore
}

func (u *gitLabChatUpdate) validate(ctx context.Context) error {
	client := u.adapter.Client()
	check := func() ([]glscm.DiscussionNote, error) {
		fresh, err := chatFreshTarget(ctx, client, u.project, u.iid, u.opts.replyDiscussion, u.pending, u.botUserID, u.opts.replyRequested, u.controls)
		if err != nil {
			return nil, err
		}
		for _, n := range fresh {
			if n.ID == u.pending {
				for _, original := range u.triggerNotes {
					if original.ID == u.pending && original.Body != n.Body {
						return nil, fmt.Errorf("chat: question changed during update evaluation")
					}
				}
			}
		}
		return fresh, nil
	}
	fresh, err := check()
	if err != nil {
		return err
	}
	allowed, err := gitLabThreadResponsesAllowed(ctx, client, u.project, u.iid, fresh, u.botUserID, u.opts.replyMuteEmoji)
	if err != nil {
		return err
	}
	if !allowed {
		return errChatReplySuppressed
	}
	_, err = check()
	return err
}

func (u *gitLabUpdateExecution) run(ctx context.Context, signal review.ReviewUpdateSignal) (*review.ReviewUpdateOutcome, error) {
	if u.job == nil || u.store == nil {
		return nil, fmt.Errorf("review updates require a durable job and state directory")
	}
	if err := u.validateJob(ctx); err != nil {
		return nil, err
	}
	req := u.request
	req.UpdateReview = nil
	var err error
	req.Messages, err = linkedFindingMessages(ctx, u.adapter.Client(), u.project, u.iid, req.Result.ReviewID, signal.FindingIDs, u.triggerNotes, int(^uint(0)>>1), u.botUserID, u.controls)
	if err != nil {
		return nil, err
	}
	// Current findings are structured input. Do not send their previous rendered
	// roots back as evidence after applying a correction in memory.
	clean := *req.ReviewCtx
	clean.Comments = nil
	for _, comment := range req.ReviewCtx.Comments {
		if !comment.IsReview {
			comment.Body = reviewmd.StripMarkers(comment.Body)
			clean.Comments = append(clean.Comments, comment)
		}
	}
	req.ReviewCtx = &clean
	result, err := u.engine.RunUpdateWorkflow(ctx, review.UpdateWorkflowRequest{
		UpdateFindingsRequest: review.UpdateFindingsRequest{DiscussRequest: req, Signal: signal},
		PriorityThreshold:     u.app.priorityThreshold, ConfidenceThreshold: u.app.confidenceThreshold,
		DisablePatchSummary: u.profile.DisablePatchSummary,
	})
	if err != nil {
		return nil, err
	}
	if result.Publish {
		u.job.Plan = &serve.UpdatePublication{Before: req.Result, After: result.Review, BaseSHA: clean.DiffBaseSHA, HeadSHA: clean.DiffHeadSHA, Evidence: u.job.Evidence, Followup: updateFollowup(&result.Outcome)}
		if err := u.store.Save(u.job); err != nil {
			return nil, err
		}
		published, err := u.adapter.UpdateReview(ctx, u.project, u.iid, glscm.ReviewUpdateRequest{
			Operation: u.job.ID,
			Before:    req.Result, After: result.Review, BaseSHA: clean.DiffBaseSHA, HeadSHA: clean.DiffHeadSHA, Validate: u.validateJob,
		})
		if err != nil {
			return nil, fmt.Errorf("chat: publishing review update: %w", err)
		}
		*req.Result = *published
	}
	return &result.Outcome, nil
}

// Linked thread roots are identified by stable review/finding IDs and bot
// ownership. Only their replies enter the transcript, ordered once by note ID.
// The triggering note remains last even if another thread gained replies later.
func linkedFindingMessages(ctx context.Context, client *glscm.Client, project string, iid int, rid string, findingIDs []string, trigger []glscm.DiscussionNote, pending, botUserID int, controls chatMessageControls) ([]llm.Message, error) {
	if len(findingIDs) == 0 {
		return chatThreadToMessages(trigger, botUserID, controls), nil
	}
	discussions, err := client.MRDiscussions(ctx, project, iid)
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, id := range findingIDs {
		wanted[id] = true
	}
	notes := map[int]glscm.DiscussionNote{}
	add := func(items []glscm.DiscussionNote) {
		for _, note := range items {
			if note.ID <= pending {
				notes[note.ID] = note
			}
		}
	}
	for _, d := range discussions {
		if len(d.Notes) == 0 || d.Notes[0].AuthorID != botUserID || reviewmd.StripMarkers(d.Notes[0].Body) == "" {
			continue
		}
		r, f, ok := reviewmd.DetectThreadReview(d.Notes[0].Body)
		if !ok || r != rid || !wanted[f] {
			continue
		}
		add(d.Notes[1:])
	}
	if len(trigger) > 0 {
		add(trigger[1:])
	}
	ordered := make([]glscm.DiscussionNote, 0, len(notes))
	for _, note := range notes {
		ordered = append(ordered, note)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	return chatNotesToMessages(ordered, botUserID, controls), nil
}
