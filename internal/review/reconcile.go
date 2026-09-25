package review

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/dedupe"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/workflow"
)

// publishedFindingNote tells the verifier where a published finding came from.
const publishedFindingNote = "an earlier review of this change published it, possibly against an older revision, " +
	"so it might be outdated. Confirm the problem still exists in the current code."

// loadPublishedStepFunc fills the published group with the open findings of
// the review already on the change request, so the next steps verify and merge
// them together with the reviewers' findings. The group stays empty when the
// run does not publish, the source cannot read published reviews, or there is
// no review yet; a read failure is a warning and leaves the run a fresh review.
func (e *Engine) loadPublishedStepFunc() stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		group := agentResult{
			resp: &llm.ReviewResponse{},
			run:  model.AgentRun{Name: "Published Findings", Role: "published", Status: model.AgentRunStatusSkipped},
		}
		defer func() { st.setGroup(workflow.PublishedGroupID, group, nil) }()
		source, ok := e.source.(model.PublishedReviewSource)
		if !sc.Req.PostReview || !ok {
			return nil
		}
		published, err := source.PublishedReview(ctx, sc.Req)
		if err != nil {
			st.addWarningf("Could not read the published review: %v; publishing this run as a new review", err)
			return nil
		}
		if published == nil || published.Review == nil {
			return nil
		}
		for _, f := range published.Review.Findings {
			if f.Resolution == nil {
				// The verifier and the later steps see the text as published,
				// not the provenance of the run that first produced it.
				group.resp.Findings = append(group.resp.Findings, currentFinding(f))
			}
		}
		group.run.Status, group.run.Findings = model.AgentRunStatusOK, len(group.resp.Findings)
		st.mu.Lock()
		st.published = published
		st.mu.Unlock()
		sc.Engine.logProgress(logging.StageReview, logging.StateDone, fmt.Sprintf("published review=%s open=%d", published.Review.ReviewID, len(group.resp.Findings)))
		return nil
	}
}

// verifyPublishedStepFunc verifies the published group like a reviewer's
// findings, with a note that each one may be outdated. It records which ones
// the verifier refuted, so publishing can resolve them with its remarks.
func (e *Engine) verifyPublishedStepFunc() stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		vr, ok := st.vectorResult(workflow.PublishedGroupID)
		if !ok {
			return fmt.Errorf("workflow: verify:%s requires a preceding %s step", workflow.PublishedGroupID, workflow.StepLoadPublished)
		}
		if vr.resp == nil || len(vr.resp.Findings) == 0 {
			return nil
		}
		results := []agentResult{vr}
		budgets := verifyPhaseBudgetStarters(ctx, "verify:"+workflow.PublishedGroupID, sc.Override, sc.Req, sc.Engine.logf)
		// The diff-scope filter is for new findings. A published finding
		// whose lines left the diff (e.g. a later commit fixed it elsewhere or
		// reverted them) must still reach the verifier, or it could never be
		// resolved; it was in scope when it was published.
		req := sc.Req
		req.DisableDiffScope = true
		telemetry, warnings, err := sc.Engine.verifyAndFilterVectorFindings(ctx, st.Enriched, results, req, st.limiter, "Published", publishedFindingNote, sc.categorizeAgentContext(), budgets)
		st.setVectorResponse(workflow.PublishedGroupID, results[0].resp)
		st.addVerificationTelemetry(workflow.PublishedGroupID, telemetry, warnings)
		st.mu.Lock()
		st.publishedRefuted = telemetry.Refuted
		st.mu.Unlock()
		if err != nil {
			sc.Engine.logf(ctx, "Verifier failed for published findings: warnings=%d error=%v", len(warnings), err)
			return err
		}
		return nil
	}
}

// reconcilePublished folds the assembled result into the published review so
// the publisher updates that review in place. Published findings keep their
// ids, including one merge folded into a new finding that still matches its
// thread (the rule publishers use for already-posted comments). Per published
// finding:
//   - still in the result: its current text, unless only provenance changed;
//   - refuted by the verifier: resolved with the verifier's remarks;
//   - merged into a finding of the result that no longer matches its thread:
//     resolved, pointing at that finding, which is posted as a new one;
//   - otherwise dropped along the way (diff scope, filters): kept unchanged,
//     since that is no proof it is fixed.
//
// New findings duplicating a published or foreign thread are dropped; the
// rest are added.
func reconcilePublished(res *model.ReviewResult, published *model.PublishedReview, refuted map[string]string, absorbed *absorptionLog, headSHA string) *model.Reconciliation {
	prior := published.Review
	priorByID := make(map[string]model.Finding, len(prior.Findings))
	var open []model.Finding
	for _, f := range prior.Findings {
		priorByID[f.ID] = f
		if f.Resolution == nil {
			open = append(open, postedShell(f))
		}
	}
	current := make(map[string]model.Finding)
	var fresh []model.Finding
	for _, f := range res.Findings {
		if _, ok := priorByID[f.ID]; ok {
			current[f.ID] = f
		} else {
			fresh = append(fresh, f)
		}
	}
	rec := &model.Reconciliation{Before: prior, HeadSHA: headSHA}
	var added []model.Finding
	for _, f := range fresh {
		if i := matchPosted(f, open); i >= 0 {
			if _, taken := current[open[i].ID]; !taken {
				// Merge folded the published finding into this one.
				f.ID = open[i].ID
				current[f.ID] = f
			}
			continue
		}
		if matchPosted(f, published.Foreign) >= 0 {
			continue
		}
		f.Revision, f.Resolution = 0, nil
		added = append(added, f)
		rec.Added = append(rec.Added, f.ID)
	}
	resultTitles := make(map[string]string, len(res.Findings))
	for _, f := range res.Findings {
		resultTitles[f.ID], _, _, _ = reviewmd.FindingDisplay(f)
	}
	merged := make([]model.Finding, 0, len(prior.Findings)+len(added))
	for _, p := range prior.Findings {
		switch f, ok := current[p.ID]; {
		case p.Resolution != nil:
			merged = append(merged, p)
		case ok && !sameDisplayedFinding(p, f):
			f.Revision, f.Resolution = p.Revision, nil
			merged = append(merged, f)
			rec.Updated = append(rec.Updated, p.ID)
		case ok:
			merged = append(merged, p)
		case refuted[p.ID] != "":
			merged = append(merged, resolvedFinding(p, resolutionSentence(refuted[p.ID])))
			rec.Resolved = append(rec.Resolved, p.ID)
		case resultTitles[absorbed.survivor(p.ID)] != "":
			title := resultTitles[absorbed.survivor(p.ID)]
			merged = append(merged, resolvedFinding(p, boundedResolution("Merged into the finding \u201c"+title+"\u201d of this re-review.")))
			rec.Resolved = append(rec.Resolved, p.ID)
		default:
			merged = append(merged, p)
		}
	}
	res.Findings = append(merged, added...)
	res.ReviewID, res.Revision, res.CreatedAt = prior.ReviewID, prior.Revision, prior.CreatedAt
	return rec
}

// resolvedFinding closes a published finding with reason, the way the chat
// correction resolves one.
func resolvedFinding(p model.Finding, reason string) model.Finding {
	resolved := currentFinding(p)
	resolved.Revision = p.Revision
	resolved.Resolution = &model.FindingResolution{Reason: reason}
	resolved.Body, resolved.Suggestions = reason, nil
	return resolved
}

// sameDisplayedFinding reports whether two versions of a finding render the
// same. The confidence score is not rendered, so a re-verification that only
// moved it does not rewrite the thread.
func sameDisplayedFinding(a, b model.Finding) bool {
	da, db := currentFinding(a), currentFinding(b)
	da.ConfidenceScore, db.ConfidenceScore = 0, 0
	da.Resolution, db.Resolution = nil, nil
	return reflect.DeepEqual(da, db)
}

// resolutionSentence turns verifier remarks into the one short sentence a
// resolution shows.
func resolutionSentence(remarks string) string {
	s := strings.Join(strings.Fields(remarks), " ")
	if i := strings.Index(s, ". "); i >= 0 {
		s = s[:i+1]
	}
	return boundedResolution(s)
}

// boundedResolution fits a resolution reason on one line of at most 240
// characters ending in a period.
func boundedResolution(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > 240 {
		s = strings.TrimRight(string([]rune(s)[:239]), " ,;:") + "…"
	}
	if s == "" {
		return "Re-verification found the problem is no longer present."
	}
	if !strings.HasSuffix(s, ".") && !strings.HasSuffix(s, "…") {
		s += "."
	}
	return s
}

// postedShell reduces a finding to what a posted comment's fingerprint keeps:
// id, file, and displayed title.
func postedShell(f model.Finding) model.Finding {
	title, _, _, _ := reviewmd.FindingDisplay(f)
	return model.Finding{ID: f.ID, Title: title, CodeLocation: model.CodeLocation{FilePath: f.CodeLocation.FilePath}}
}

// matchPosted returns the index of the posted finding f duplicates, or -1,
// using the rule publishers apply to already-posted comments.
func matchPosted(f model.Finding, posted []model.Finding) int {
	for i, p := range posted {
		if p.ID != "" && p.ID == f.ID {
			return i
		}
	}
	probe := f
	probe.Title, _, _, _ = reviewmd.FindingDisplay(f)
	i, _ := dedupe.FindBest(probe, posted, dedupe.Duplicate)
	return i
}

// splitPublishedFindings separates the findings a reconciliation kept from
// the published review from the run's own. Without a reconciliation every
// finding is the run's own.
func splitPublishedFindings(findings []model.Finding, rec *model.Reconciliation) (published, fresh []model.Finding) {
	if rec == nil || rec.Before == nil {
		return nil, findings
	}
	ids := make(map[string]struct{}, len(rec.Before.Findings))
	for _, f := range rec.Before.Findings {
		ids[f.ID] = struct{}{}
	}
	for _, f := range findings {
		if _, ok := ids[f.ID]; ok {
			published = append(published, f)
		} else {
			fresh = append(fresh, f)
		}
	}
	return published, fresh
}

// allReviewersFailedLocked reports whether every reviewer that ran failed,
// the same collapse the CLI reports as a failed review. The caller must hold
// st.mu.
func (st *PipelineState) allReviewersFailedLocked() bool {
	seen := false
	for _, id := range st.groupOrder {
		g := st.groupByID[id]
		if g == nil || !g.filled || g.result.run.Role != "review" {
			continue
		}
		seen = true
		if g.result.run.Status != model.AgentRunStatusFailed {
			return false
		}
	}
	return seen
}

// absorptionLog records which finding merge folded each finding into, so a
// published finding merged into one that no longer matches its thread can be
// closed instead of lingering. Merge strips its own provenance before findings
// leave the step; this keeps the part reconcilePublished needs.
type absorptionLog struct {
	mu   sync.Mutex
	into map[string]string
}

type absorptionsContextKey struct{}

func withAbsorptions(ctx context.Context, log *absorptionLog) context.Context {
	return context.WithValue(ctx, absorptionsContextKey{}, log)
}

// absorptionsFromContext returns the run's absorption log, or nil outside a
// pipeline run. Every method is nil-safe.
func absorptionsFromContext(ctx context.Context) *absorptionLog {
	log, _ := ctx.Value(absorptionsContextKey{}).(*absorptionLog)
	return log
}

// record notes that survivor absorbed every other member of a folded cluster.
func (l *absorptionLog) record(survivor string, members []model.Finding) {
	ids := make([]string, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.ID)
	}
	l.recordIDs(survivor, ids)
}

// recordIDs notes that survivor absorbed the findings with the given ids.
func (l *absorptionLog) recordIDs(survivor string, absorbed []string) {
	if l == nil || survivor == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, id := range absorbed {
		if id = strings.TrimSpace(id); id == "" || id == survivor {
			continue
		}
		if l.into == nil {
			l.into = map[string]string{}
		}
		l.into[id] = survivor
	}
}

// survivor follows the absorption chain from id to the finding that finally
// holds it, or "" when merge never absorbed id.
func (l *absorptionLog) survivor(id string) string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := ""
	for seen := map[string]bool{}; !seen[id]; {
		seen[id] = true
		next, ok := l.into[id]
		if !ok {
			break
		}
		out, id = next, next
	}
	return out
}
