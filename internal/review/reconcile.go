package review

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/dedupe"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/workflow"
)

// reconcilePlan maps the run's final findings onto the review a
// published-review import brought in. The reconcile step builds it once every
// other step is done, so it only records what already happened to each
// published finding; assembly lays it out.
type reconcilePlan struct {
	baseline *model.PublishedReview
	headSHA  string
	// records holds, per published id, the version to publish unless the
	// final findings still carry that id: resolved findings as they are,
	// findings the run removed resolved with the reason, and findings that
	// vanished without a recorded reason unchanged.
	records map[string]model.Finding
	// foreignDup holds the run's new findings that duplicate a thread an
	// earlier review id left on the change request; they are not reposted.
	foreignDup map[string]bool
}

// planReconcile decides, for every published finding the final findings no
// longer carry, what happened to it. The imported findings went through the
// whole run, so each one either survived under its own id (merge keeps the published
// finding of a cluster, see findingLog.prefer) or was removed by a step that
// recorded why:
//   - refuted by the verifier, or classified out: resolved with that reason;
//   - merged into another finding the run keeps: resolved, pointing at it;
//   - filtered by a priority or confidence threshold: resolved with that;
//   - nothing recorded: kept open unchanged, since a finding lost without a
//     reason proves nothing.
func planReconcile(final []model.Finding, baseline *model.PublishedReview, log *findingLog, headSHA string) *reconcilePlan {
	plan := &reconcilePlan{baseline: baseline, headSHA: headSHA, records: map[string]model.Finding{}, foreignDup: map[string]bool{}}
	titles := make(map[string]string, len(final))
	for _, f := range final {
		titles[f.ID], _, _, _ = reviewmd.FindingDisplay(f)
	}
	for _, p := range baseline.Review.Findings {
		if _, present := titles[p.ID]; present || p.Resolution != nil {
			plan.records[p.ID] = p
			continue
		}
		survivor, reason := log.fate(p.ID)
		switch {
		case survivor != "" && titles[survivor] != "":
			plan.records[p.ID] = resolvedFinding(p, boundedResolution("Merged into the finding “"+titles[survivor]+"” of this re-review."))
		case reason != "":
			plan.records[p.ID] = resolvedFinding(p, reason)
		default:
			plan.records[p.ID] = p
		}
	}
	for _, f := range final {
		if _, published := plan.records[f.ID]; !published && matchPosted(f, baseline.Foreign) >= 0 {
			plan.foreignDup[f.ID] = true
		}
	}
	return plan
}

// isPublished reports whether id belongs to the published review.
func (p *reconcilePlan) isPublished(id string) bool {
	_, ok := p.records[id]
	return ok
}

// layout composes the findings to publish: the published review's findings in
// their order, each replaced by the final finding with its id unless only
// provenance changed (so the thread is not rewritten for nothing), then the
// run's new findings.
func (p *reconcilePlan) layout(final []model.Finding) []model.Finding {
	byID := make(map[string]model.Finding, len(final))
	for _, f := range final {
		byID[f.ID] = f
	}
	out := make([]model.Finding, 0, len(p.baseline.Review.Findings)+len(final))
	for _, prior := range p.baseline.Review.Findings {
		record := p.records[prior.ID]
		if f, ok := byID[prior.ID]; ok && record.Resolution == nil && !sameDisplayedFinding(record, f) {
			f.Revision, f.Resolution = prior.Revision, nil
			out = append(out, f)
			continue
		}
		out = append(out, record)
	}
	for _, f := range final {
		if !p.isPublished(f.ID) && !p.foreignDup[f.ID] {
			f.Revision, f.Resolution = 0, nil
			out = append(out, f)
		}
	}
	return out
}

// reconcileStepFunc is the reconcile step, the last of the workflow: it maps
// the final result onto the review the named group imported. There is nothing
// to reconcile when the import found no published review, or when every
// reviewer crashed (such a run says nothing about the code and must not
// replace the published verdict).
func (e *Engine) reconcileStepFunc(group string) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		st.mu.Lock()
		defer st.mu.Unlock()
		g := st.groupByID[group]
		if g == nil || g.provenance == nil || g.provenance.Baseline == nil {
			return nil
		}
		if st.allReviewersFailedLocked() {
			st.warnings.addf("Reconcile skipped: every reviewer failed, so the published review stays as it is")
			return nil
		}
		if st.result == nil {
			st.setResultLocked(st.materializeFromGroupsLocked(sc.Req))
		}
		headSHA := ""
		if st.Enriched != nil {
			headSHA = st.Enriched.DiffHeadSHA
		}
		st.reconciled = planReconcile(st.result.Findings, g.provenance.Baseline, st.findings, headSHA)
		resolved := 0
		for _, r := range st.reconciled.records {
			if r.Resolution != nil {
				resolved++
			}
		}
		sc.Engine.logProgress(logging.StageReview, logging.StateDone, fmt.Sprintf("reconcile review=%s published=%d resolved=%d", g.provenance.Baseline.Review.ReviewID, len(st.reconciled.records), resolved))
		return nil
	}
}

// reconcileGroup returns the group a reconcile step entry names.
func reconcileGroup(entry workflow.StepEntry) string {
	if entry.Config != nil && entry.Config.Group != nil {
		return *entry.Config.Group
	}
	return ""
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

// findingLog records, for one run, what happened to findings that left it:
// which finding merge or dedupe folded each one into, and why a step removed
// the others. It also holds the ids merge must prefer as a cluster's survivor.
// Steps strip their own provenance before findings move on; this keeps the part
// reconcile needs. Every method is nil-safe.
type findingLog struct {
	mu        sync.Mutex
	into      map[string]string
	removed   map[string]string
	preferred map[string]bool
}

type findingLogContextKey struct{}

func withFindingLog(ctx context.Context, log *findingLog) context.Context {
	return context.WithValue(ctx, findingLogContextKey{}, log)
}

// findingLogFrom returns the run's finding log, or nil outside a pipeline run.
func findingLogFrom(ctx context.Context) *findingLog {
	log, _ := ctx.Value(findingLogContextKey{}).(*findingLog)
	return log
}

// prefer marks ids merge keeps as the survivor of any cluster they are in.
func (l *findingLog) prefer(ids []string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.preferred == nil {
		l.preferred = map[string]bool{}
	}
	for _, id := range ids {
		l.preferred[id] = true
	}
}

// isPreferred reports whether merge should keep id as a cluster's survivor.
func (l *findingLog) isPreferred(id string) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.preferred[id]
}

// record notes that survivor absorbed every other member of a folded cluster.
func (l *findingLog) record(survivor string, members []model.Finding) {
	ids := make([]string, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.ID)
	}
	l.recordIDs(survivor, ids)
}

// recordIDs notes that survivor absorbed the findings with the given ids.
func (l *findingLog) recordIDs(survivor string, absorbed []string) {
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

// remove notes that a step removed finding id, with reason as the one short
// sentence a resolution shows. The first reason recorded wins.
func (l *findingLog) remove(id, reason string) {
	if l == nil || id == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.removed == nil {
		l.removed = map[string]string{}
	}
	if _, seen := l.removed[id]; !seen {
		l.removed[id] = boundedResolution(reason)
	}
}

// removeDropped records reason for every finding of before that after no
// longer holds.
func (l *findingLog) removeDropped(before, after []model.Finding, reason string) {
	if l == nil || len(before) == len(after) {
		return
	}
	kept := make(map[string]bool, len(after))
	for _, f := range after {
		kept[f.ID] = true
	}
	for _, f := range before {
		if !kept[f.ID] {
			l.remove(f.ID, reason)
		}
	}
}

// survivor follows the absorption chain from id to the finding that finally
// holds it, or "" when nothing absorbed id.
func (l *findingLog) survivor(id string) string {
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

// fate reports what happened to a finding that left the run: the finding that
// finally absorbed it, and the reason recorded for it or, when its survivor
// was removed in turn, for that survivor.
func (l *findingLog) fate(id string) (survivor, reason string) {
	if l == nil {
		return "", ""
	}
	survivor = l.survivor(id)
	l.mu.Lock()
	defer l.mu.Unlock()
	if reason = l.removed[id]; reason == "" && survivor != "" {
		reason = l.removed[survivor]
	}
	return survivor, reason
}

// verifyDropReason is the resolution for a finding the verifier refuted, or
// could not confirm under the refuted-and-unverified policy.
func verifyDropReason(verdict, remarks string) string {
	if verdict == model.VerdictUnverified {
		return "Re-verification could not confirm it: " + resolutionSentence(remarks)
	}
	return resolutionSentence(remarks)
}

// categorizeDropReason is the resolution for a finding the classifier sorted
// out before verification.
func categorizeDropReason(categories []string, verdict string) string {
	switch {
	case verdict == model.VerdictUnverified:
		return "Re-review classified it as a finding bundled with other issues, which the verify policy drops."
	case slices.Contains(categories, model.CategoryConfirmation):
		return "Re-review classified it as a confirmation of correct code, not a problem."
	case slices.Contains(categories, model.CategoryCompilation):
		return "Re-review classified it as a compiler diagnostic, which the toolchain reports."
	default:
		return "Re-review classified it as not a finding."
	}
}

// priorityDropReason and confidenceDropReason are the resolutions a published
// finding gets when a threshold filtered it out.
func priorityDropReason(threshold string) string {
	return fmt.Sprintf("Re-review rated it below the %s priority threshold.", strings.ToUpper(priorityThresholdLabel(threshold)))
}

func confidenceDropReason(threshold float64) string {
	return fmt.Sprintf("Re-review scored its confidence below the %.2f threshold.", threshold)
}
