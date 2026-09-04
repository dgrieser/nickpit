package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/review"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/workflow"
)

type chatFindingChange struct {
	Before model.Finding
	After  model.Finding
}

func applyChatFindingUpdates(result *model.ReviewResult, updates []review.DiscussFindingUpdate, source model.RevisionSource) (*model.ReviewResult, []chatFindingChange, error) {
	out, err := result.Clone()
	if err != nil {
		return nil, nil, fmt.Errorf("chat: cloning review for update: %w", err)
	}
	byID := make(map[string]int, len(out.Findings))
	for i := range out.Findings {
		byID[out.Findings[i].ID] = i
	}
	var changes []chatFindingChange
	for _, update := range updates {
		i, ok := byID[update.ID]
		if !ok {
			return nil, nil, fmt.Errorf("chat: update references unknown finding %q", update.ID)
		}
		old := out.Findings[i]
		if old.LastUpdateID == source.UpdateID {
			continue
		}
		proposed := update.Finding
		proposed.ID = old.ID
		proposed.Revision = old.Revision
		proposed.LastUpdateID = ""
		proposed.History = nil
		proposed.Verification = nil
		proposed.Finalization = nil
		proposed.Summarization = nil
		proposed.MergedFrom = nil
		proposed.State = strings.ToLower(strings.TrimSpace(update.State))
		if proposed.State == model.FindingStateResolved {
			resolutionSource := source
			resolutionSource.Reason = strings.TrimSpace(update.Reason)
			proposed.Resolution = &model.FindingResolution{Summary: strings.TrimSpace(update.Resolution), RevisionSource: resolutionSource}
		} else {
			proposed.State = model.FindingStateActive
			proposed.Resolution = nil
		}
		if sameChatFinding(old, proposed) {
			continue
		}
		snapshot, err := model.SnapshotFinding(old)
		if err != nil {
			return nil, nil, fmt.Errorf("chat: snapshotting finding %s: %w", old.ID, err)
		}
		revisionSource := source
		revisionSource.Reason = strings.TrimSpace(update.Reason)
		proposed.History = append(append([]model.FindingRevision(nil), old.History...), model.FindingRevision{
			Revision: old.Revision, RevisionSource: revisionSource, Finding: snapshot,
		})
		proposed.Revision = old.Revision + 1
		proposed.LastUpdateID = source.UpdateID
		out.Findings[i] = proposed
		changes = append(changes, chatFindingChange{Before: old, After: proposed})
	}
	return out, changes, nil
}

func sameChatFinding(old, proposed model.Finding) bool {
	old.State = normalizedFindingState(old)
	old.Revision, proposed.Revision = 0, 0
	old.LastUpdateID, proposed.LastUpdateID = "", ""
	old.History, proposed.History = nil, nil
	old.Verification, proposed.Verification = nil, nil
	old.Finalization, proposed.Finalization = nil, nil
	old.Summarization, proposed.Summarization = nil, nil
	old.MergedFrom, proposed.MergedFrom = nil, nil
	return reflect.DeepEqual(old, proposed)
}

func normalizedFindingState(f model.Finding) string {
	if f.IsResolved() {
		return model.FindingStateResolved
	}
	return model.FindingStateActive
}

func activeReview(result *model.ReviewResult) (*model.ReviewResult, error) {
	out, err := result.Clone()
	if err != nil {
		return nil, err
	}
	out.Findings = out.Findings[:0]
	for _, finding := range result.Findings {
		if !finding.IsResolved() {
			out.Findings = append(out.Findings, finding)
		}
	}
	return out, nil
}

func (a *app) recomputeChatReview(ctx context.Context, profile config.Profile, engine *review.Engine, reviewCtx *model.ReviewContext, result *model.ReviewResult, changes []chatFindingChange, repoRoot string) (*model.ReviewResult, error) {
	active, err := activeReview(result)
	if err != nil {
		return nil, fmt.Errorf("chat: preparing active findings: %w", err)
	}
	verdict, _, err := engine.Verdict(ctx, reviewCtx, active, review.VerdictOptions{
		DisableJSONResponseFormat: a.disableJSONResponseFormat, MaxOutputRetries: profile.MaxOutputRetries,
		MaxReasoningSeconds: profile.MaxReasoningSeconds, DisableParallelToolCalls: a.disableParallelToolCalls,
		DisablePatchSummary: a.disablePatchSummary, DisableSuggestions: profile.DisableSuggestions,
		RepoRoot: repoRoot, DiffFormat: profile.DiffFormat, PriorityThreshold: a.priorityThreshold,
		ConfidenceThreshold: a.confidenceThreshold,
	})
	if err != nil {
		return nil, fmt.Errorf("chat: recomputing verdict: %w", err)
	}
	result.OverallCorrectness = verdict.OverallCorrectness
	result.OverallExplanation = verdict.OverallExplanation
	result.OverallConfidenceScore = verdict.OverallConfidenceScore

	spec, err := a.resolveActiveSpec()
	if err != nil {
		return nil, fmt.Errorf("chat: resolving summarize workflow: %w", err)
	}
	var summarizeStep *workflow.StepEntry
	for _, step := range spec.FlatSteps() {
		if step.Type == workflow.StepSummarize {
			summarizeStep = step
			break
		}
	}
	if summarizeStep != nil {
		req := model.ReviewRequest{DisableJSONResponseFormat: a.disableJSONResponseFormat, DisablePatchSummary: a.disablePatchSummary, DisableSuggestions: profile.DisableSuggestions, MaxOutputRetries: profile.MaxOutputRetries, MaxReasoningSeconds: profile.MaxReasoningSeconds}
		summaryProfile, summaryReq := summarizeStep.Config.Resolve(profile, req)
		summaryEngine, buildErr := a.chatEngine(ctx, summaryProfile, glscm.NewAdapter(glscm.NewClient(summaryProfile.GitLabBaseURL, summaryProfile.GitLabToken), summaryProfile.AssetBaseURL), retrieval.NewLocalEngine(), a.logger)
		if buildErr != nil {
			return nil, buildErr
		}
		changed := &model.ReviewResult{OverallExplanation: result.OverallExplanation}
		changedIDs := make(map[string]struct{}, len(changes))
		for _, change := range changes {
			changedIDs[change.After.ID] = struct{}{}
		}
		for _, finding := range result.Findings {
			if _, ok := changedIDs[finding.ID]; ok && !finding.IsResolved() {
				changed.Findings = append(changed.Findings, finding)
			}
		}
		opts := review.SummarizeOptions{DisableJSONResponseFormat: summaryReq.DisableJSONResponseFormat, MaxOutputRetries: summaryReq.MaxOutputRetries, MaxReasoningSeconds: summaryReq.MaxReasoningSeconds, DisableParallelToolCalls: summaryReq.DisableParallelToolCalls, DisablePatchSummary: summaryReq.DisablePatchSummary, DisableSuggestions: summaryReq.DisableSuggestions, RepoRoot: repoRoot}
		if len(changed.Findings) == 0 {
			result.OverallExplanation, _, err = summaryEngine.SummarizeOverall(ctx, result.OverallExplanation, opts)
		} else {
			var summarized *model.ReviewResult
			summarized, _, err = summaryEngine.Summarize(ctx, changed, opts)
			if err == nil {
				result.OverallExplanation = summarized.OverallExplanation
				byID := make(map[string]model.Finding, len(summarized.Findings))
				for _, finding := range summarized.Findings {
					byID[finding.ID] = finding
				}
				for i := range result.Findings {
					if f, ok := byID[result.Findings[i].ID]; ok {
						result.Findings[i].Summarization = f.Summarization
					}
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("chat: summarizing updated review: %w", err)
		}
	}
	return result, nil
}

func publishChatReviewUpdates(ctx context.Context, client *glscm.Client, adapter *glscm.Adapter, profile config.Profile, project string, mrID, botUserID int, result *model.ReviewResult, changes []chatFindingChange) error {
	discussions, err := client.MRDiscussions(ctx, project, mrID)
	if err != nil {
		return err
	}
	type findingRoot struct {
		discussion glscm.MRDiscussion
		finding    model.Finding
		revision   int
	}
	roots := make(map[string]findingRoot)
	staleRoots := make(map[string][]findingRoot)
	var summary glscm.MRDiscussion
	summaryRevision := -1
	for _, discussion := range discussions {
		if len(discussion.Notes) == 0 || discussion.Notes[0].AuthorID != botUserID {
			continue
		}
		rid, fid, ok := reviewmd.DetectThreadReview(discussion.Notes[0].Body)
		if !ok || rid != result.ReviewID {
			continue
		}
		if fid == "" {
			revision := -1
			for _, env := range reviewmd.CollectReviewEnvelopes(discussion.Notes[0].Body) {
				if env.ReviewID == rid && !env.Ref && env.Revision > revision {
					revision = env.Revision
				}
			}
			if revision >= summaryRevision {
				summary, summaryRevision = discussion, revision
			}
		} else {
			candidate := findingRoot{discussion: discussion, revision: -1}
			for _, env := range reviewmd.CollectFindingEnvelopes(discussion.Notes[0].Body) {
				if env.ReviewID == rid && env.Finding.ID == fid && !env.Ref && env.Finding.Revision > candidate.revision {
					candidate.finding, candidate.revision = env.Finding, env.Finding.Revision
				}
			}
			if current, exists := roots[fid]; exists {
				if candidate.revision >= current.revision {
					staleRoots[fid] = append(staleRoots[fid], current)
					roots[fid] = candidate
				} else {
					staleRoots[fid] = append(staleRoots[fid], candidate)
				}
			} else {
				roots[fid] = candidate
			}
		}
	}
	render := reviewmd.NewRenderer(profile.AssetBaseURL).ForReview(result.ReviewID).WithContextOptions(result.ContextOptions)
	req := model.ReviewRequest{Repo: project, Identifier: mrID}
	for _, change := range changes {
		rootEntry, ok := roots[change.After.ID]
		root := rootEntry.discussion
		if !ok || len(root.Notes) == 0 {
			return fmt.Errorf("chat: finding root %q not found", change.After.ID)
		}
		oldBody := root.Notes[0].Body
		if rootEntry.revision < 0 {
			rootEntry.finding = change.Before
			rootEntry.revision = change.Before.Revision
		}
		moved := !reflect.DeepEqual(change.Before.CodeLocation, change.After.CodeLocation)
		if moved {
			created, createErr := adapter.CreateFindingDiscussion(ctx, req, result, change.After)
			if createErr != nil {
				return fmt.Errorf("chat: moving finding %s: %w", change.After.ID, createErr)
			}
			if footerBody := reviewmd.PreserveResponseFooter(oldBody, created.Body); footerBody != created.Body {
				if err := client.UpdateMRDiscussionNote(ctx, project, mrID, created.DiscussionID, created.NoteID, footerBody); err != nil {
					return err
				}
			}
			if !created.Carried {
				for _, body := range render.CarrierNotes(result, []model.Finding{change.After}) {
					if err := client.CreateMRNotePath(ctx, project, mrID, body); err != nil {
						return err
					}
				}
			}
			staleRoots[change.After.ID] = append(staleRoots[change.After.ID], rootEntry)
			roots[change.After.ID] = findingRoot{discussion: glscm.MRDiscussion{ID: created.DiscussionID, Notes: []glscm.DiscussionNote{{ID: created.NoteID, Body: created.Body}}}, finding: change.After, revision: change.After.Revision}
			continue
		}
		prefix := ""
		if !root.Notes[0].Positioned {
			prefix = fmt.Sprintf("`%s:%d`", reviewmd.Sanitize(change.After.CodeLocation.FilePath), change.After.CodeLocation.LineRange.Start)
		}
		body, carried := render.FindingBodyCarried(change.After, prefix)
		if err := client.UpdateMRDiscussionNote(ctx, project, mrID, root.ID, root.Notes[0].ID, reviewmd.PreserveResponseFooter(oldBody, body)); err != nil {
			return err
		}
		if err := client.ResolveMRDiscussion(ctx, project, mrID, root.ID, change.After.IsResolved()); err != nil {
			return err
		}
		if !carried {
			for _, carrier := range render.CarrierNotes(result, []model.Finding{change.After}) {
				if err := client.CreateMRNotePath(ctx, project, mrID, carrier); err != nil {
					return err
				}
			}
		}
	}
	// A relocation is a create-then-retire operation. Reconcile every older root
	// here as well, so retrying after any partial API failure finishes the move.
	for fid, candidates := range staleRoots {
		for _, stale := range candidates {
			if len(stale.discussion.Notes) == 0 || stale.revision < 0 {
				continue
			}
			title, _, _, _ := reviewmd.FindingDisplay(stale.finding)
			tombstone := reviewmd.FingerprintMarker(stale.finding, title) + "\n" + reviewmd.FindingRoutingMarker(result.ReviewID, stale.finding.ID) + "\n\n" + render.ResolvedBadge() + "\n\nFinding moved to the updated code location."
			rootNote := stale.discussion.Notes[0]
			if err := client.UpdateMRDiscussionNote(ctx, project, mrID, stale.discussion.ID, rootNote.ID, reviewmd.PreserveResponseFooter(rootNote.Body, tombstone)); err != nil {
				return fmt.Errorf("retiring old root for %s: %w", fid, err)
			}
			if err := client.ResolveMRDiscussion(ctx, project, mrID, stale.discussion.ID, true); err != nil {
				return err
			}
		}
	}
	if len(summary.Notes) == 0 {
		return fmt.Errorf("chat: root review comment not found")
	}
	body, carried := render.SummaryBodyCarried(result)
	if err := client.UpdateMRDiscussionNote(ctx, project, mrID, summary.ID, summary.Notes[0].ID, reviewmd.PreserveResponseFooter(summary.Notes[0].Body, body)); err != nil {
		return err
	}
	if !carried {
		for _, carrier := range render.CarrierNotes(result, nil) {
			if err := client.CreateMRNotePath(ctx, project, mrID, carrier); err != nil {
				return err
			}
		}
	}
	return nil
}

func chatRevisionSource(result *model.ReviewResult, reviewCtx *model.ReviewContext, noteID int) model.RevisionSource {
	updateID := fmt.Sprintf("%s:%d:%s", result.ReviewID, noteID, reviewCtx.DiffHeadSHA)
	url := strings.TrimRight(reviewCtx.Repository.URL, "/")
	if url != "" {
		url += fmt.Sprintf("#note_%d", noteID)
	}
	return model.RevisionSource{UpdatedAt: time.Now().UTC(), UpdateID: updateID, SourceNoteID: noteID, SourceURL: url, HeadSHA: reviewCtx.DiffHeadSHA}
}

func appendReviewHistory(before, after *model.ReviewResult, source model.RevisionSource) {
	after.Revision = before.Revision + 1
	after.LastUpdateID = source.UpdateID
	old := []any{before.OverallCorrectness, before.OverallExplanation, before.OverallConfidenceScore}
	current := []any{after.OverallCorrectness, after.OverallExplanation, after.OverallConfidenceScore}
	oldJSON, _ := json.Marshal(old)
	currentJSON, _ := json.Marshal(current)
	if string(oldJSON) == string(currentJSON) {
		return
	}
	after.History = append(append([]model.ReviewRevision(nil), before.History...), model.ReviewRevision{Revision: before.Revision, RevisionSource: source, OverallCorrectness: before.OverallCorrectness, OverallExplanation: before.OverallExplanation, OverallConfidenceScore: before.OverallConfidenceScore})
}
