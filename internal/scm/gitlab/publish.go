package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/textsan"
)

// markerOpen is the token collectMarkers scans for; both the real markers below
// and any injected lookalike in untrusted text begin with it.
const markerOpen = "<!-- nickpit:"

const summaryMarker = markerOpen + "summary -->"

func findingMarker(id string) string {
	return markerOpen + "finding:" + id + " -->"
}

// sanitizeForPublish prepares untrusted, LLM-generated text for posting as
// GitLab markdown. It strips terminal control characters (consistent with the
// terminal formatter) and defuses any embedded nickpit marker so untrusted
// content cannot inject a lookalike that poisons re-run dedupe: the marker's
// leading "<" is HTML-escaped, which GitLab renders as a literal "<" while
// breaking collectMarkers' scan for markerOpen.
func sanitizeForPublish(s string) string {
	s = textsan.StripControl(s)
	return strings.ReplaceAll(s, markerOpen, "&lt;"+strings.TrimPrefix(markerOpen, "<"))
}

// PublishReview posts the review back to the merge request: one summary note and
// one comment per finding (pinned inline when the line is part of the diff, else
// a general note prefixed with file:line). Hidden markers make re-runs
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
	if _, ok := posted[summaryMarker]; !ok {
		if err := a.client.Post(ctx, notesPath, map[string]string{"body": summaryBody(result)}, nil); err != nil {
			errs = append(errs, fmt.Errorf("summary: %w", err))
		}
	}
	for _, finding := range result.Findings {
		if _, ok := posted[findingMarker(finding.ID)]; ok {
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
// GitLab) a general note prefixed with file:line.
func (a *Adapter) publishFinding(ctx context.Context, notesPath, discussionsPath string, change MRChange, hasChange bool, refs DiffRefs, finding model.Finding) error {
	if hasChange {
		if pos, ok := multiLinePosition(change, refs, finding.CodeLocation.LineRange); ok {
			err := a.client.Post(ctx, discussionsPath, discussionPayload(findingBody(finding, ""), pos), nil)
			if err == nil {
				return nil
			}
			if !isUnprocessable(err) {
				return err
			}
		}
		if pos, ok := bestPosition(change, refs, finding.CodeLocation.LineRange); ok {
			err := a.client.Post(ctx, discussionsPath, discussionPayload(findingBody(finding, ""), pos), nil)
			if err == nil {
				return nil
			}
			if !isUnprocessable(err) {
				return err
			}
		}
	}
	prefix := fmt.Sprintf("`%s:%d`", sanitizeForPublish(finding.CodeLocation.FilePath), finding.CodeLocation.LineRange.Start)
	return a.client.Post(ctx, notesPath, map[string]string{"body": findingBody(finding, prefix)}, nil)
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
			collectMarkers(note.Body, markers)
		}
	}
	var discussions discussionsResponse
	if err := a.client.GetPaginated(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/discussions", escaped, iid), &discussions); err == nil {
		for _, discussion := range discussions {
			for _, note := range discussion.Notes {
				collectMarkers(note.Body, markers)
			}
		}
	}
	return markers
}

func collectMarkers(body string, out map[string]struct{}) {
	rest := body
	for {
		i := strings.Index(rest, markerOpen)
		if i < 0 {
			return
		}
		rest = rest[i:]
		j := strings.Index(rest, "-->")
		if j < 0 {
			return
		}
		out[strings.TrimSpace(rest[:j+3])] = struct{}{}
		rest = rest[j+3:]
	}
}

func summaryBody(result *model.ReviewResult) string {
	var b strings.Builder
	b.WriteString(summaryMarker)
	b.WriteString("\n\n")
	correctness := sanitizeForPublish(strings.TrimSpace(result.OverallCorrectness))
	if correctness == "" {
		correctness = "review complete"
	}
	fmt.Fprintf(&b, "**Overall: %s** · confidence %.2f\n", correctness, result.OverallConfidenceScore)
	if explanation := sanitizeForPublish(strings.TrimSpace(result.OverallExplanation)); explanation != "" {
		b.WriteString("\n")
		b.WriteString(explanation)
		b.WriteString("\n")
	}
	return b.String()
}

// findingBody renders a finding as MR markdown, as-is. When locationPrefix is
// non-empty (general-note fallback) it is shown above the body so the location
// is still visible without an inline anchor.
func findingBody(finding model.Finding, locationPrefix string) string {
	title, body, rank, confidence := findingDisplay(finding)
	var b strings.Builder
	b.WriteString(findingMarker(finding.ID))
	b.WriteString("\n\n")
	if locationPrefix != "" {
		b.WriteString(locationPrefix)
		b.WriteString("\n\n")
	}
	fmt.Fprintf(&b, "**P%d** · confidence %.2f\n\n", rank, confidence)
	if title != "" {
		fmt.Fprintf(&b, "### %s\n\n", sanitizeForPublish(title))
	}
	b.WriteString(sanitizeForPublish(body))
	if len(finding.Suggestions) > 0 {
		b.WriteString("\n\n**Suggestions**\n")
		for _, suggestion := range finding.Suggestions {
			text := strings.TrimSpace(suggestion.Body)
			if text == "" {
				continue
			}
			fmt.Fprintf(&b, "\n- %s", sanitizeForPublish(text))
		}
	}
	return b.String()
}

// findingDisplay prefers the finalized title/body/priority/confidence when a
// finalization pass produced them, else the original finding fields.
func findingDisplay(finding model.Finding) (title, body string, rank int, confidence float64) {
	title = finding.Title
	body = finding.Body
	confidence = finding.ConfidenceScore
	rank = model.PriorityRank(finding.Priority)
	if finding.Finalization != nil {
		if t := strings.TrimSpace(finding.Finalization.Title); t != "" {
			title = t
		}
		if bodyText := strings.TrimSpace(finding.Finalization.Body); bodyText != "" {
			body = finding.Finalization.Body
		}
		confidence = finding.Finalization.ConfidenceScore
		priority := finding.Finalization.Priority
		rank = model.PriorityRank(&priority)
	}
	// The summarize pass produces a shortened body (other fields copied from
	// finalization); prefer it for the published comment when present.
	if finding.Summarization != nil {
		if t := strings.TrimSpace(finding.Summarization.Title); t != "" {
			title = t
		}
		if bodyText := strings.TrimSpace(finding.Summarization.Body); bodyText != "" {
			body = finding.Summarization.Body
		}
		confidence = finding.Summarization.ConfidenceScore
		priority := finding.Summarization.Priority
		rank = model.PriorityRank(&priority)
	}
	return title, body, rank, confidence
}
