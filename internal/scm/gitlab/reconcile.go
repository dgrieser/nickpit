package gitlab

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/google/uuid"
)

// maxReconcileAttempts bounds how often a re-review is replayed onto chat
// corrections that land while it publishes.
const maxReconcileAttempts = 3

// PublishedReview reads the review this token already published on the merge
// request: the one whose summary thread is newest, reassembled from the
// carriers in the token's own notes. Open findings that earlier runs published
// under other review ids come back as Foreign. It returns nil when there is no
// summary thread or its review cannot be reassembled completely; the publisher
// then falls back to posting a fresh review.
func (a *Adapter) PublishedReview(ctx context.Context, req model.ReviewRequest) (*model.PublishedReview, error) {
	user, err := a.client.CurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("gitlab: resolving token user for carrier verification: %w", err)
	}
	discussions, err := a.client.MRDiscussions(ctx, req.Repo, req.Identifier)
	if err != nil {
		return nil, err
	}
	rid := currentSummaryReviewID(discussions, user.ID)
	if rid == "" {
		return nil, nil
	}
	review := reviewmd.ReviewResultsByID(ownedBodies(discussions, user.ID))[rid]
	if review == nil {
		return nil, nil
	}
	return &model.PublishedReview{Review: review, Foreign: foreignOpenFindings(discussions, user.ID, rid)}, nil
}

// currentSummaryReviewID returns the review id of the newest summary thread the
// user started, or "" when there is none.
func currentSummaryReviewID(discussions []MRDiscussion, userID int) string {
	rid, newest := "", 0
	for _, d := range discussions {
		if len(d.Notes) == 0 {
			continue
		}
		root := d.Notes[0]
		if root.AuthorID != userID || !strings.Contains(reviewmd.StripHistory(root.Body), reviewmd.SummaryMarker) {
			continue
		}
		r, f, ok := reviewmd.DetectThreadReview(root.Body)
		if !ok || f != "" || root.ID <= newest {
			continue
		}
		rid, newest = r, root.ID
	}
	return rid
}

// foreignOpenFindings collects the open findings of the user's threads that
// belong to any review other than rid. They come from the visible fingerprint
// markers rather than from carriers: an older run's carrier note may already
// have been pruned while its finding threads are still on the MR.
func foreignOpenFindings(discussions []MRDiscussion, userID int, rid string) []model.Finding {
	var priors reviewmd.Priors
	for _, d := range discussions {
		if len(d.Notes) == 0 {
			continue
		}
		root := d.Notes[0]
		if root.AuthorID != userID {
			continue
		}
		if r, _, ok := reviewmd.DetectThreadReview(root.Body); ok && r == rid {
			continue
		}
		reviewmd.ScanComment(root.Body, &priors)
	}
	return priors.Findings
}

// publishReconciled publishes a run the reconcile step folded into the review
// already on the merge request. It goes through UpdateReview, exactly like a
// chat correction: the summary thread gets a history entry, changed findings
// are edited in place, and new findings open new threads. When a chat
// correction lands while this publishes, the run is replayed onto it (see
// rebaseReconciled). A reply in the summary thread says what changed.
func (a *Adapter) publishReconciled(ctx context.Context, req model.ReviewRequest, result *model.ReviewResult) error {
	ctx, unlock, err := a.client.LockMR(ctx, req.Repo, req.Identifier)
	if err != nil {
		return err
	}
	defer unlock()
	rec := result.Reconciliation
	before := rec.Before
	after, err := result.Clone()
	if err != nil {
		return err
	}
	if after.ContextOptions == nil {
		// Pipeline results carry no context options; the summary envelope
		// needs them so a chat rebuilds this run's filtered context.
		after.ContextOptions = model.ContextOptionsFromRequest(req)
	}
	for attempt := 1; ; attempt++ {
		published, err := a.UpdateReview(ctx, req.Repo, req.Identifier, ReviewUpdateRequest{
			Operation:      uuid.NewString(),
			Before:         before,
			After:          after,
			HeadSHA:        rec.HeadSHA,
			AllowAdditions: true,
			SkipDiffCheck:  true,
		})
		var conflict *UpdateConflict
		if errors.As(err, &conflict) && attempt < maxReconcileAttempts {
			reviews, readErr := a.ReviewResults(ctx, req.Repo, req.Identifier)
			if readErr != nil {
				return fmt.Errorf("gitlab publish: rereading review after conflict: %w", readErr)
			}
			current := reviews[before.ReviewID]
			if current == nil {
				return fmt.Errorf("gitlab publish: review %s disappeared during re-review", before.ReviewID)
			}
			if after, err = rebaseReconciled(before, after, current); err != nil {
				return err
			}
			before = current
			continue
		}
		if err != nil {
			return fmt.Errorf("gitlab publish: updating review %s: %w", before.ReviewID, err)
		}
		// Report what this publish changed, not what the run planned: a
		// replay onto a concurrent correction can discard planned changes.
		report := reconciledChanges(before, published)
		report.HeadSHA = rec.HeadSHA
		var errs []error
		if err := a.replyReconciled(ctx, req, published, report); err != nil {
			errs = append(errs, fmt.Errorf("re-review reply: %w", err))
		}
		// Earlier runs could leave carrier-only notes behind; this review now
		// carries everything in its own threads.
		a.pruneStaleCarriers(ctx, req.Repo, req.Identifier, published.ReviewID)
		return errors.Join(errs...)
	}
}

// rebaseReconciled replays a re-review onto corrections that landed while it
// ran. A finding the re-review changed keeps that change only when the
// published copy still equals the one the re-review started from; otherwise
// the published copy wins. Findings the re-review added are appended. The
// overall verdict and metadata are the re-review's.
func rebaseReconciled(before, after, current *model.ReviewResult) (*model.ReviewResult, error) {
	out, err := after.Clone()
	if err != nil {
		return nil, err
	}
	// Clone normalizes legacy location fields the same way reassembly does, so
	// only real edits make the published copy differ from the starting point.
	if before, err = before.Clone(); err != nil {
		return nil, err
	}
	if current, err = current.Clone(); err != nil {
		return nil, err
	}
	beforeByID := make(map[string]model.Finding, len(before.Findings))
	for _, f := range before.Findings {
		beforeByID[f.ID] = f
	}
	afterByID := make(map[string]model.Finding, len(after.Findings))
	for _, f := range after.Findings {
		afterByID[f.ID] = f
	}
	currentIDs := make(map[string]bool, len(current.Findings))
	out.Findings = nil
	for _, published := range current.Findings {
		currentIDs[published.ID] = true
		mine, changed := afterByID[published.ID]
		started, known := beforeByID[published.ID]
		if changed && known && reflect.DeepEqual(started, published) {
			out.Findings = append(out.Findings, mine)
			continue
		}
		out.Findings = append(out.Findings, published)
	}
	for _, f := range after.Findings {
		if _, known := beforeByID[f.ID]; !known && !currentIDs[f.ID] {
			out.Findings = append(out.Findings, f)
		}
	}
	return out, nil
}

// reconciledChanges lists the findings published added, resolved, and
// updated relative to before, the review it replaced.
func reconciledChanges(before, published *model.ReviewResult) *model.Reconciliation {
	out := &model.Reconciliation{Before: before}
	prior := make(map[string]model.Finding, len(before.Findings))
	for _, f := range before.Findings {
		prior[f.ID] = f
	}
	for _, f := range published.Findings {
		old, ok := prior[f.ID]
		switch {
		case !ok:
			out.Added = append(out.Added, f.ID)
		case old.Resolution != nil:
		case f.Resolution != nil:
			out.Resolved = append(out.Resolved, f.ID)
		case f.Revision != old.Revision:
			// UpdateReview stamps the new revision on exactly the findings
			// it rewrote.
			out.Updated = append(out.Updated, f.ID)
		}
	}
	return out
}

// replyReconciled answers in the summary thread with what the re-review
// changed, so the update is visible without opening the history. A thread
// muted with the mute command gets no reply.
func (a *Adapter) replyReconciled(ctx context.Context, req model.ReviewRequest, published *model.ReviewResult, rec *model.Reconciliation) error {
	user, err := a.client.CurrentUser(ctx)
	if err != nil {
		return err
	}
	discussions, err := a.client.MRDiscussions(ctx, req.Repo, req.Identifier)
	if err != nil {
		return err
	}
	root := indexUpdateTargets(discussions, user.ID, published.ReviewID)[""]
	if root.DiscussionID == "" {
		return fmt.Errorf("summary thread of review %s not found", published.ReviewID)
	}
	if reviewmd.ThreadCommandMuted(root.Body) {
		// The author silenced this thread with the mute command; the summary
		// itself still carries the update.
		return nil
	}
	return a.client.ReplyToMRDiscussionPath(ctx, req.Repo, req.Identifier, root.DiscussionID, reconciledReplyBody(published, rec))
}

// reconciledReplyBody lists the findings a re-review added, resolved, and
// updated by their displayed titles.
func reconciledReplyBody(published *model.ReviewResult, rec *model.Reconciliation) string {
	titles := make(map[string]string, len(published.Findings))
	for _, f := range published.Findings {
		title, _, _, _ := reviewmd.FindingDisplay(f)
		titles[f.ID] = reviewmd.Sanitize(title)
	}
	var b strings.Builder
	b.WriteString("Review updated")
	if sha := rec.HeadSHA; sha != "" {
		fmt.Fprintf(&b, " for `%s`", sha[:min(len(sha), 8)])
	}
	b.WriteString(".")
	sections := []struct {
		label string
		ids   []string
	}{{"New", rec.Added}, {"Resolved", rec.Resolved}, {"Updated", rec.Updated}}
	listed := false
	for _, section := range sections {
		var lines []string
		for _, id := range section.ids {
			if title, ok := titles[id]; ok {
				lines = append(lines, "- "+title)
			}
		}
		if len(lines) == 0 {
			continue
		}
		listed = true
		fmt.Fprintf(&b, "\n\n**%s**\n\n%s", section.label, strings.Join(lines, "\n"))
	}
	if !listed {
		b.WriteString(" No new findings; the existing findings are unchanged.")
	}
	return b.String()
}
