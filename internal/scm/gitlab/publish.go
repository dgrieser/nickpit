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

	posted := a.existingMarkers(ctx, req.Repo, req.Identifier)
	changesByPath := make(map[string]MRChange, len(info.Changes))
	for _, change := range info.Changes {
		changesByPath[change.NewPath] = change
	}

	var errs []error
	if _, ok := posted[reviewmd.SummaryMarker]; !ok {
		if err := a.client.Post(ctx, notesPath, map[string]string{"body": a.render.SummaryBody(result)}, nil); err != nil {
			errs = append(errs, fmt.Errorf("summary: %w", err))
		}
	}
	for _, finding := range result.Findings {
		if _, ok := posted[reviewmd.FindingMarker(finding.ID)]; ok {
			continue
		}
		change, hasChange := changesByPath[finding.CodeLocation.FilePath]
		if err := a.publishFinding(ctx, notesPath, discussionsPath, change, hasChange, info.DiffRefs, finding); err != nil {
			errs = append(errs, fmt.Errorf("finding %s: %w", finding.ID, err))
		}
	}
	return errors.Join(errs...)
}

// publishFinding posts a single finding. It tries a multi-line inline comment,
// then a single-line inline comment, then (on an unmappable line or a 422 from
// GitLab) a general note carrying file:line.
func (a *Adapter) publishFinding(ctx context.Context, notesPath, discussionsPath string, change MRChange, hasChange bool, refs DiffRefs, finding model.Finding) error {
	if hasChange {
		if pos, ok := multiLinePosition(change, refs, finding.CodeLocation.LineRange); ok {
			err := a.client.Post(ctx, discussionsPath, discussionPayload(a.render.FindingBody(finding, ""), pos), nil)
			if err == nil {
				return nil
			}
			if !isUnprocessable(err) {
				return err
			}
		}
		if pos, ok := bestPosition(change, refs, finding.CodeLocation.LineRange); ok {
			err := a.client.Post(ctx, discussionsPath, discussionPayload(a.render.FindingBody(finding, ""), pos), nil)
			if err == nil {
				return nil
			}
			if !isUnprocessable(err) {
				return err
			}
		}
	}
	prefix := fmt.Sprintf("`%s:%d`", reviewmd.Sanitize(finding.CodeLocation.FilePath), finding.CodeLocation.LineRange.Start)
	return a.client.Post(ctx, notesPath, map[string]string{"body": a.render.FindingBody(finding, prefix)}, nil)
}

func discussionPayload(body string, pos position) map[string]any {
	return map[string]any{"body": body, "position": pos}
}

func isUnprocessable(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity
}

// existingMarkers collects the nickpit markers already present in the MR's notes
// and discussions so re-runs skip findings that were posted before. Fetch errors
// are tolerated (worst case: a duplicate comment).
func (a *Adapter) existingMarkers(ctx context.Context, project string, iid int) map[string]struct{} {
	markers := map[string]struct{}{}
	escaped := escapeProject(project)

	var notes []struct {
		Body string `json:"body"`
	}
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/notes", escaped, iid), &notes); err == nil {
		for _, note := range notes {
			reviewmd.CollectMarkers(note.Body, markers)
		}
	}
	var discussions discussionsResponse
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/discussions", escaped, iid), &discussions); err == nil {
		for _, discussion := range discussions {
			for _, note := range discussion.Notes {
				reviewmd.CollectMarkers(note.Body, markers)
			}
		}
	}
	return markers
}
