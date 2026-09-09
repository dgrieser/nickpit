package main

import (
	"context"
	"fmt"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/review"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
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
	job      *serve.UpdateJob
	store    *serve.UpdateStore
	snapshot *updateEvidenceSnapshot
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
	req.Messages = u.snapshot.Messages
	// Current findings are structured input. Do not send their previous rendered
	// roots back as evidence after applying a correction in memory.
	clean := *req.ReviewCtx
	clean.Comments = nil
	req.ReviewCtx = &clean
	result, err := u.engine.RunUpdateWorkflow(ctx, review.UpdateWorkflowRequest{
		UpdateFindingsRequest: review.UpdateFindingsRequest{DiscussRequest: req, Signal: signal},
		PriorityThreshold:     u.app.priorityThreshold, ConfidenceThreshold: u.app.confidenceThreshold,
		DisablePatchSummary: u.profile.DisablePatchSummary,
		InitiatingMessage:   u.snapshot.Question,
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
			Before:    req.Result, After: result.Review, BaseSHA: clean.DiffBaseSHA, HeadSHA: clean.DiffHeadSHA,
			Validate: u.validateJob, OnStaged: u.markPlanStaged,
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
	var discussions []glscm.MRDiscussion
	if len(findingIDs) > 0 {
		var err error
		discussions, err = client.MRDiscussions(ctx, project, iid)
		if err != nil {
			return nil, err
		}
	}
	return collectUpdateEvidence(discussions, trigger, rid, findingIDs, pending, botUserID, controls).Messages, nil
}
