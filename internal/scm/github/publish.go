package github

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

// reviewFallbackBody is the review body used when the summary was already posted
// on a prior run but new inline findings still need a review to carry them.
// GitHub requires a non-empty body for a submitted COMMENT review, and a
// marker-free line here avoids re-posting (and re-deduping) the verdict.
const reviewFallbackBody = "Additional nickpit findings below.  "

// inlineItem pairs a finding with its rendered inline comment so the finding can
// be re-rendered as a general comment if the atomic review POST is rejected.
type inlineItem struct {
	finding model.Finding
	comment reviewComment
}

// PublishReview posts the review back to the pull request as a single GitHub PR
// review (event COMMENT): the overall verdict as the review body and one inline
// comment per finding anchored to its diff line. Findings whose line is not part
// of the diff fall back to a general PR comment carrying file:line. Hidden
// markers make re-runs idempotent. Per-item failures are aggregated; a failure
// never aborts the rest.
func (a *Adapter) PublishReview(ctx context.Context, req model.ReviewRequest, result *model.ReviewResult) error {
	if result == nil {
		return nil
	}
	info, err := a.client.FetchPRPositionInfo(ctx, req.Repo, req.Identifier)
	if err != nil {
		return fmt.Errorf("github publish: fetch position info: %w", err)
	}

	escaped := escapeRepo(req.Repo)
	reviewsPath := fmt.Sprintf("/repos/%s/pulls/%d/reviews", escaped, req.Identifier)
	issueCommentsPath := fmt.Sprintf("/repos/%s/issues/%d/comments", escaped, req.Identifier)

	prior := a.existingComments(ctx, req.Repo, req.Identifier)

	// Bind the renderer to this run's review id so every comment carries the
	// hidden carrier markers used to regroup findings later, and embed the
	// request's context options so a PR-reassembled chat rebuilds the review's
	// FILTERED context, never the then-current configuration's.
	render := a.render.ForReview(result.ReviewID).WithContextOptions(model.ContextOptionsFromRequest(req))

	// Partition not-yet-posted findings: those whose line maps to the diff become
	// inline review comments; the rest become general comments. missing collects
	// findings without an own successfully-posted carrier: skipped as
	// already-posted duplicates, failed to post, or posted with the carrier
	// omitted because it would have pushed the comment past GitHub's size limit.
	var inline []inlineItem
	var overflow []model.Finding
	var missing []model.Finding
	for _, finding := range result.Findings {
		title, _, _, _ := reviewmd.FindingDisplay(finding)
		if reviewmd.AlreadyPosted(finding, title, prior) {
			missing = append(missing, finding)
			continue
		}
		body, carried := render.FindingBodyCarried(finding, "")
		if !carried {
			missing = append(missing, finding)
		}
		if comment, ok := inlineComment(info.Hunks[finding.CodeLocation.FilePath], finding.CodeLocation.FilePath, finding.CodeLocation.LineRange, body); ok {
			inline = append(inline, inlineItem{finding: finding, comment: comment})
		} else {
			overflow = append(overflow, finding)
		}
	}

	summaryBody := ""
	summaryCarried := false
	_, summarySuppressed := prior.Markers[reviewmd.SummaryMarker]
	if !summarySuppressed {
		summaryBody, summaryCarried = render.SummaryBodyCarried(result)
	}

	var errs []error
	reviewPostFailed := false
	if err := a.publishReview(ctx, render, reviewsPath, issueCommentsPath, info.HeadSHA, summaryBody, inline); err != nil {
		errs = append(errs, err)
		reviewPostFailed = true
		// The create-review call is atomic and its per-finding fallback outcome is
		// not tracked individually; treat every inline finding as missing (a
		// superset — reassembly de-duplicates by finding id).
		for _, item := range inline {
			missing = append(missing, item.finding)
		}
	}
	for _, finding := range overflow {
		// carried is prefix-independent, so the partition loop already recorded
		// size-omitted carriers for overflow findings; appending here again would
		// serialize the same envelope twice. Only post failures add.
		if _, err := a.postIssueComment(ctx, render, issueCommentsPath, finding); err != nil {
			errs = append(errs, fmt.Errorf("finding %s: %w", finding.ID, err))
			missing = append(missing, finding)
		}
	}
	// When the visible summary was suppressed, failed, or shipped without its
	// envelope (size-omitted), or any finding lacks its own carrier, the
	// distributed carriers no longer cover this run in full. Post hidden,
	// size-bounded carrier chunks holding the review envelope and exactly the
	// missing findings, so a chat can still reassemble this review by id.
	// Verdict-only re-reviews post just the envelope.
	carrierFailed := false
	if summarySuppressed || reviewPostFailed || !summaryCarried || len(missing) > 0 {
		for _, finding := range reviewmd.UniqueFindingsByID(missing) {
			// A finding past the reader's decoded budget is never emitted (the
			// writer refuses the poisoned marker) — surface the gap instead of
			// silently leaving the finding unavailable to chat.
			if result.ReviewID != "" && reviewmd.FindingMarker(result.ReviewID, finding) == "" {
				errs = append(errs, fmt.Errorf("finding %s: payload exceeds the carrier budget; it will not be available for discussion", finding.ID))
			}
		}
		for _, body := range render.CarrierNotes(result, reviewmd.UniqueFindingsByID(missing)) {
			if err := a.client.Post(ctx, issueCommentsPath, map[string]string{"body": body}, nil); err != nil {
				errs = append(errs, fmt.Errorf("carrier: %w", err))
				carrierFailed = true
			}
		}
	}
	// Prune only once every required carrier of the CURRENT run is confirmed
	// posted: a failed carrier chunk leaves this run incomplete, and deleting
	// the previous run's fallback carriers then could destroy the last complete
	// review on the PR — reassembly would find neither run. Best-effort cleanup.
	if !carrierFailed {
		a.pruneStaleCarriers(ctx, req.Repo, req.Identifier, result.ReviewID)
	}
	return errors.Join(errs...)
}

// pruneStaleCarriers deletes carrier-only issue comments left by previous runs
// (see the GitLab twin and reviewmd.IsStaleCarrierBody). GitHub's API exposes
// no cheap "current user" probe on this client, so the author check is
// omitted: a pure foreign-carrier body is with overwhelming probability our
// own previous chunk, deleting a forged one only removes the forger's own
// comment, and a permission error on someone else's is tolerated. Best-effort
// throughout.
func (a *Adapter) pruneStaleCarriers(ctx context.Context, repo string, number int, currentReviewID string) {
	if currentReviewID == "" {
		return
	}
	escaped := escapeRepo(repo)
	var comments []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
	}
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/repos/%s/issues/%d/comments", escaped, number), &comments); err != nil {
		return
	}
	// Reviews still referenced by a VISIBLE ref stub keep their carriers (see
	// the GitLab twin). Stubs can live in review bodies and inline review
	// comments too, so all three comment surfaces feed the protected set. (A
	// forged ref can at worst RETAIN old carriers, never delete.)
	var bodies []string
	for _, comment := range comments {
		bodies = append(bodies, comment.Body)
	}
	var reviews []reviewResponse
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews", escaped, number), &reviews); err == nil {
		for _, review := range reviews {
			bodies = append(bodies, review.Body)
		}
	}
	var lineComments []commentResponse
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/comments", escaped, number), &lineComments); err == nil {
		for _, comment := range lineComments {
			bodies = append(bodies, comment.Body)
		}
	}
	protected := reviewmd.RefReviewIDs(bodies)
	for _, comment := range comments {
		if !reviewmd.IsStaleCarrierBody(comment.Body, currentReviewID, protected) {
			continue
		}
		_ = a.client.Delete(ctx, fmt.Sprintf("/repos/%s/issues/comments/%d", escaped, comment.ID))
	}
}

// publishReview posts the summary + inline findings as one review. GitHub's
// create-review call is atomic, so if it rejects an inline line (422) the whole
// review fails; we then degrade gracefully — post the summary as a body-only
// review and each inline finding as a general comment — rather than drop them.
func (a *Adapter) publishReview(ctx context.Context, render reviewmd.Renderer, reviewsPath, issueCommentsPath, headSHA, summaryBody string, inline []inlineItem) error {
	body := summaryBody
	if body == "" && len(inline) > 0 {
		body = reviewFallbackBody
	}
	if body == "" && len(inline) == 0 {
		return nil // Nothing new to post.
	}

	comments := make([]reviewComment, len(inline))
	for i, item := range inline {
		comments[i] = item.comment
	}
	err := a.postReview(ctx, reviewsPath, headSHA, body, comments)
	if err == nil {
		return nil
	}
	if !IsUnprocessable(err) || len(comments) == 0 {
		return fmt.Errorf("review: %w", err)
	}

	var errs []error
	if summaryBody != "" {
		if err := a.postReview(ctx, reviewsPath, headSHA, summaryBody, nil); err != nil {
			errs = append(errs, fmt.Errorf("summary review: %w", err))
		}
	}
	for _, item := range inline {
		// carried is prefix-independent (see FindingBodyCarried), so the caller's
		// partition-time record already covers a size-omitted carrier here.
		if _, err := a.postIssueComment(ctx, render, issueCommentsPath, item.finding); err != nil {
			errs = append(errs, fmt.Errorf("finding %s: %w", item.finding.ID, err))
		}
	}
	return errors.Join(errs...)
}

func (a *Adapter) postReview(ctx context.Context, path, commitID, body string, comments []reviewComment) error {
	payload := map[string]any{"event": "COMMENT"}
	if commitID != "" {
		payload["commit_id"] = commitID
	}
	if body != "" {
		payload["body"] = body
	}
	if len(comments) > 0 {
		payload["comments"] = comments
	}
	return a.client.Post(ctx, path, payload, nil)
}

// postIssueComment posts a finding as a general PR comment. carried reports
// whether the body embedded the full-finding carrier (false when omitted for
// size), so callers can route the finding into the fallback carrier notes.
func (a *Adapter) postIssueComment(ctx context.Context, render reviewmd.Renderer, path string, finding model.Finding) (carried bool, err error) {
	prefix := fmt.Sprintf("`%s:%d`", reviewmd.Sanitize(finding.CodeLocation.FilePath), finding.CodeLocation.LineRange.Start)
	body, bodyCarried := render.FindingBodyCarried(finding, prefix)
	if err := a.client.Post(ctx, path, map[string]string{"body": body}, nil); err != nil {
		return false, err
	}
	return bodyCarried, nil
}

// existingComments collects the markers and finding fingerprints already present
// in the PR's reviews, review comments, and issue comments so re-runs skip
// findings posted before. Fetch errors are tolerated (worst case: a duplicate
// comment).
func (a *Adapter) existingComments(ctx context.Context, repo string, number int) reviewmd.Priors {
	prior := reviewmd.Priors{Markers: map[string]struct{}{}}
	escaped := escapeRepo(repo)

	var reviews []reviewResponse
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews", escaped, number), &reviews); err == nil {
		for _, review := range reviews {
			reviewmd.ScanComment(review.Body, &prior)
		}
	}
	var comments []commentResponse
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/comments", escaped, number), &comments); err == nil {
		for _, comment := range comments {
			reviewmd.ScanComment(comment.Body, &prior)
		}
	}
	var issueComments []issueCommentResponse
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/repos/%s/issues/%d/comments", escaped, number), &issueComments); err == nil {
		for _, comment := range issueComments {
			reviewmd.ScanComment(comment.Body, &prior)
		}
	}
	return prior
}
