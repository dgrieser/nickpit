package main

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/review"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/serve"
)

type gitLabChatUpdate struct {
	queuedJob               *serve.UpdateJob
	queuedRelease           func()
	job                     *serve.UpdateJob
	store                   *serve.UpdateStore
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

func (u *gitLabChatUpdate) validate(ctx context.Context) error {
	if u.job != nil {
		return u.validateJob(ctx)
	}
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

func (u *gitLabChatUpdate) run(ctx context.Context, signal review.ReviewUpdateSignal) (*review.ReviewUpdateOutcome, error) {
	if u.job == nil || u.store == nil {
		return nil, fmt.Errorf("review updates require a durable job and state directory")
	}
	if err := u.validate(ctx); err != nil {
		return nil, err
	}
	req := u.request
	req.UpdateReview = nil
	var err error
	cutoff := u.pending
	if u.job != nil {
		cutoff = int(^uint(0) >> 1)
	}
	req.Messages, err = linkedFindingMessages(ctx, u.adapter.Client(), u.project, u.iid, req.Result.ReviewID, signal.FindingIDs, u.triggerNotes, cutoff, u.botUserID, u.controls)
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
	after, report, err := u.engine.UpdateFindings(ctx, review.UpdateFindingsRequest{DiscussRequest: req, Signal: signal})
	if err != nil {
		return nil, fmt.Errorf("chat: checking finding update: %w", err)
	}
	var changed []model.Finding
	usage := report.TokensUsed
	for i, f := range after.Findings {
		if !reflect.DeepEqual(f, req.Result.Findings[i]) {
			changed = append(changed, f)
		}
	}
	if len(changed) > 0 || report.ReviewCorrectionWarranted() {
		verdictInput, err := after.Clone()
		if err != nil {
			return nil, err
		}
		verdictInput.OverallCorrectness, verdictInput.OverallExplanation, verdictInput.OverallConfidenceScore = "", "", 0
		var contextNotes strings.Builder
		contextNotes.WriteString(signal.Reason)
		if report.ReviewCheck != nil {
			contextNotes.WriteString("\n\nIndependent evidence assessment:\n")
			contextNotes.WriteString(report.ReviewCheck.Reason)
		}
		// Explicit chat evidence is available even when general MR comments are disabled.
		for _, m := range slices.Backward(req.Messages) {
			if m.Role == "user" {
				contextNotes.WriteString("\n\nLatest author message:\n")
				contextNotes.WriteString(m.Content)
				break
			}
		}
		verdict, verdictRun, err := u.engine.Verdict(ctx, &clean, verdictInput, review.VerdictOptions{
			RepoRoot: req.RepoRoot, DiffFormat: req.DiffFormat, DisableSuggestions: req.DisableSuggestions,
			DisableJSONResponseFormat: u.profile.DisableJSONResponseFormat, MaxOutputRetries: req.MaxOutputRetries,
			MaxReasoningSeconds: req.MaxReasoningSeconds, DisableParallelToolCalls: req.DisableParallelToolCalls,
			DisablePatchSummary: u.profile.DisablePatchSummary, PriorityThreshold: u.app.priorityThreshold,
			ConfidenceThreshold: u.app.confidenceThreshold, ContextNotes: contextNotes.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("chat: regenerating verdict: %w", err)
		}
		usage.PromptTokens += verdictRun.TokensUsed.PromptTokens
		usage.CompletionTokens += verdictRun.TokensUsed.CompletionTokens
		usage.TotalTokens += verdictRun.TokensUsed.TotalTokens
		after.OverallCorrectness, after.OverallExplanation, after.OverallConfidenceScore = verdict.OverallCorrectness, verdict.OverallExplanation, verdict.OverallConfidenceScore
		operation := ""
		if u.job != nil {
			operation = u.job.ID
			outcome := &review.ReviewUpdateOutcome{Checks: report.Checks, ReviewCheck: report.ReviewCheck, Changed: changed, OverallCorrectness: after.OverallCorrectness, OverallExplanation: after.OverallExplanation}
			u.job.Plan = &serve.UpdatePublication{Before: req.Result, After: after, BaseSHA: clean.DiffBaseSHA, HeadSHA: clean.DiffHeadSHA, Evidence: u.job.Evidence, Followup: updateFollowup(outcome)}
			if err := u.store.Save(u.job); err != nil {
				return nil, err
			}
		}
		published, err := u.adapter.UpdateReview(ctx, u.project, u.iid, glscm.ReviewUpdateRequest{
			Operation: operation,
			Before:    req.Result, After: after, BaseSHA: clean.DiffBaseSHA, HeadSHA: clean.DiffHeadSHA, Validate: u.validate,
		})
		if err != nil {
			return nil, fmt.Errorf("chat: publishing review update: %w", err)
		}
		*req.Result = *published
		after = published
	}
	return &review.ReviewUpdateOutcome{TokensUsed: usage, Checks: report.Checks, ReviewCheck: report.ReviewCheck, Changed: changed, OverallCorrectness: after.OverallCorrectness, OverallExplanation: after.OverallExplanation}, nil
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
	return chatThreadToMessages(append([]glscm.DiscussionNote{{}}, ordered...), botUserID, controls), nil
}
