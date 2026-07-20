package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

// PublishReview posts the review back to the merge request: one summary note and
// one comment per finding (pinned inline when the line is part of the diff, else
// a general note carrying file:line). Hidden markers make re-runs
// idempotent. Per-item failures are aggregated; a failure never aborts the rest.
func (a *Adapter) PublishReview(ctx context.Context, req model.ReviewRequest, result *model.ReviewResult) error {
	if result == nil {
		return nil
	}
	info, err := a.client.FetchMRPositionInfo(ctx, req.Repo, req.Identifier)
	if err != nil {
		return fmt.Errorf("gitlab publish: fetch position info: %w", err)
	}

	escaped := escapeProject(req.Repo)
	notesPath := fmt.Sprintf("/projects/%s/merge_requests/%d/notes", escaped, req.Identifier)
	discussionsPath := fmt.Sprintf("/projects/%s/merge_requests/%d/discussions", escaped, req.Identifier)

	prior := a.existingComments(ctx, req.Repo, req.Identifier)
	changesByPath := make(map[string]MRChange, len(info.Changes))
	for _, change := range info.Changes {
		changesByPath[change.NewPath] = change
	}

	// Bind the renderer to this run's review id so every note carries the hidden
	// carrier markers used to regroup findings later (e.g. for a discussion).
	render := a.render.ForReview(result.ReviewID)

	var errs []error
	summaryPosted := false
	summaryCarried := false
	if _, ok := prior.Markers[reviewmd.SummaryMarker]; !ok {
		body, carried := render.SummaryBodyCarried(result)
		if err := a.client.Post(ctx, notesPath, map[string]string{"body": body}, nil); err != nil {
			errs = append(errs, fmt.Errorf("summary: %w", err))
		} else {
			summaryPosted = true
			summaryCarried = carried
		}
	}
	// missing collects findings without an own successfully-posted carrier:
	// skipped as already-posted duplicates (their old note carries no carrier for
	// THIS run's review id), failed to post, or posted with the carrier omitted
	// because it would have pushed the comment past the platform size limit.
	var missing []model.Finding
	for _, finding := range result.Findings {
		title, _, _, _ := reviewmd.FindingDisplay(finding)
		if reviewmd.AlreadyPosted(finding, title, prior) {
			missing = append(missing, finding)
			continue
		}
		change, hasChange := changesByPath[finding.CodeLocation.FilePath]
		carried, err := a.publishFinding(ctx, render, notesPath, discussionsPath, change, hasChange, info.DiffRefs, finding)
		if err != nil {
			errs = append(errs, fmt.Errorf("finding %s: %w", finding.ID, err))
		}
		if err != nil || !carried {
			missing = append(missing, finding)
		}
	}
	// When the visible summary was suppressed, failed, or shipped without its
	// envelope (size-omitted), or any finding lacks its own carrier, the
	// distributed carriers no longer cover this run in full. Post hidden,
	// size-bounded carrier chunks holding the review envelope and exactly the
	// missing findings, so a chat can still reassemble this review by id instead
	// of discussing an older run. Verdict-only re-reviews post just the envelope.
	if !summaryPosted || !summaryCarried || len(missing) > 0 {
		for _, finding := range reviewmd.UniqueFindingsByID(missing) {
			// A finding past the reader's decoded budget is never emitted (the
			// writer refuses the poisoned marker) — surface the gap instead of
			// silently leaving the finding unavailable to chat.
			if result.ReviewID != "" && reviewmd.FindingMarker(result.ReviewID, finding) == "" {
				errs = append(errs, fmt.Errorf("finding %s: payload exceeds the carrier budget; it will not be available for discussion", finding.ID))
			}
		}
		for _, body := range render.CarrierNotes(result, reviewmd.UniqueFindingsByID(missing)) {
			if err := a.client.Post(ctx, notesPath, map[string]string{"body": body}, nil); err != nil {
				errs = append(errs, fmt.Errorf("carrier: %w", err))
			}
		}
	}
	// The current run's carriers are all posted; the hidden fallback chunks of
	// previous runs are now superseded garbage. Best-effort cleanup.
	a.pruneStaleCarriers(ctx, req.Repo, req.Identifier, result.ReviewID)
	return errors.Join(errs...)
}

// pruneStaleCarriers deletes the bot's own carrier-only notes left by previous
// runs. Every re-review posts a fresh carrier set under a new review id;
// without pruning, a daemon-watched MR accumulates one blank bot-note timeline
// per push and every chat turn gunzips all of them. Only notes authored by the
// token's own user whose body is nothing but foreign-review carriers are
// deleted (see reviewmd.IsStaleCarrierBody) — visible comments, thread roots,
// and fallback chat answers are never touched. Best-effort: any failure leaves
// old notes in place (stale carriers are garbage, not corruption).
func (a *Adapter) pruneStaleCarriers(ctx context.Context, project string, iid int, currentReviewID string) {
	if currentReviewID == "" {
		return
	}
	user, err := a.client.CurrentUser(ctx)
	if err != nil {
		return
	}
	escaped := escapeProject(project)
	var notes []struct {
		ID     int    `json:"id"`
		Body   string `json:"body"`
		Author struct {
			ID int `json:"id"`
		} `json:"author"`
	}
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/notes", escaped, iid), &notes); err != nil {
		return
	}
	for _, note := range notes {
		if note.Author.ID != user.ID || !reviewmd.IsStaleCarrierBody(note.Body, currentReviewID) {
			continue
		}
		_ = a.client.Delete(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/notes/%d", escaped, iid, note.ID))
	}
}

// publishFinding posts a single finding. It tries a multi-line inline comment,
// then a single-line inline comment, then (on an unmappable line or a 422 from
// GitLab) a general note carrying file:line. carried reports whether the posted
// body embedded the full-finding carrier marker (false when it was omitted for
// size, so the caller can route the finding into the fallback carrier notes).
func (a *Adapter) publishFinding(ctx context.Context, render reviewmd.Renderer, notesPath, discussionsPath string, change MRChange, hasChange bool, refs DiffRefs, finding model.Finding) (carried bool, err error) {
	if hasChange {
		body, bodyCarried := render.FindingBodyCarried(finding, "")
		if pos, ok := multiLinePosition(change, refs, finding.CodeLocation.LineRange); ok {
			err := a.client.Post(ctx, discussionsPath, discussionPayload(body, pos), nil)
			if err == nil {
				return bodyCarried, nil
			}
			if !isUnprocessable(err) {
				return false, err
			}
		}
		if pos, ok := bestPosition(change, refs, finding.CodeLocation.LineRange); ok {
			err := a.client.Post(ctx, discussionsPath, discussionPayload(body, pos), nil)
			if err == nil {
				return bodyCarried, nil
			}
			if !isUnprocessable(err) {
				return false, err
			}
		}
	}
	prefix := fmt.Sprintf("`%s:%d`", reviewmd.Sanitize(finding.CodeLocation.FilePath), finding.CodeLocation.LineRange.Start)
	body, bodyCarried := render.FindingBodyCarried(finding, prefix)
	if err := a.client.Post(ctx, notesPath, map[string]string{"body": body}, nil); err != nil {
		return false, err
	}
	return bodyCarried, nil
}

func discussionPayload(body string, pos position) map[string]any {
	return map[string]any{"body": body, "position": pos}
}

func isUnprocessable(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity
}

// existingComments collects the markers and finding fingerprints already present
// in the MR's notes and discussions so re-runs skip findings posted before. Fetch
// errors are tolerated (worst case: a duplicate comment).
func (a *Adapter) existingComments(ctx context.Context, project string, iid int) reviewmd.Priors {
	prior := reviewmd.Priors{Markers: map[string]struct{}{}}
	escaped := escapeProject(project)

	var notes []struct {
		Body string `json:"body"`
	}
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/notes", escaped, iid), &notes); err == nil {
		for _, note := range notes {
			reviewmd.ScanComment(note.Body, &prior)
		}
	}
	var discussions discussionsResponse
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/discussions", escaped, iid), &discussions); err == nil {
		for _, discussion := range discussions {
			for _, note := range discussion.Notes {
				reviewmd.ScanComment(note.Body, &prior)
			}
		}
	}
	return prior
}
