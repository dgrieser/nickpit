package review

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/dedupe"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/workflow"
)

// reconcilePlan is the reconcile step's decision about how the run folds into
// the review a published-review import brought in. The step computes it once,
// before the verdict; the run's active findings then carry the published ids,
// and assembly only lays the plan out.
type reconcilePlan struct {
	baseline *model.PublishedReview
	headSHA  string
	// renames maps a run finding's id to the published id it takes over:
	// merge folded that published finding into it, and it still matches the
	// published thread.
	renames map[string]string
	// drop holds run findings that duplicate a thread already on the change
	// request (another published finding, or a foreign one).
	drop map[string]bool
	// records holds, per published id, the version to publish unless an
	// active finding with that id replaces it: resolved findings as they are,
	// refuted or merged-away ones resolved, and the rest unchanged.
	records map[string]model.Finding
	// openRecords are the published findings that stay open without having
	// been re-verified in this run. The verdict gets them as explicit input
	// (VerdictOptions.OpenRecords), so it cannot contradict threads left open.
	openRecords []model.Finding
	// exempt holds the ids (after renames) exempt from diff scope: published
	// ids and findings that absorbed a published finding. Assembly's final
	// safeguard honours the same set.
	exempt map[string]bool
	// mergedInto maps a published id resolved as merged away to the id of
	// the active finding that absorbed it. layout keeps the published finding
	// open should that finding not be published after all.
	mergedInto map[string]string
}

// verdictFilters are every filter a finding still passes after reconcile:
// the final diff-scope safeguard assembly applies and the verdict's own
// priority/confidence filters. Reconcile applies them first, so its plan is
// made on the final active set and nothing later removes a finding it
// counted on.
type verdictFilters struct {
	priorityThreshold   string
	confidenceThreshold float64
	diffScope           bool
	// summarizePriorityThreshold is the priority filter of a summarize step
	// that follows the verdict: the published result keeps only findings
	// that pass both, so reconcile applies it too.
	summarizePriorityThreshold string
}

func verdictFiltersOf(req model.ReviewRequest) verdictFilters {
	return verdictFilters{priorityThreshold: req.PriorityThreshold, confidenceThreshold: req.ConfidenceThreshold, diffScope: !req.DisableDiffScope}
}

// finalActive applies the filters and returns the ids that stay. Exempt ids
// skip the diff-scope safeguard (see planReconcile).
func (f verdictFilters) finalActive(findings []model.Finding, exempt map[string]bool, reviewCtx *model.ReviewContext) (map[string]bool, error) {
	if f.diffScope && reviewCtx != nil && reviewCtx.DiffScopeHunks != nil {
		var skip, fresh []model.Finding
		for _, finding := range findings {
			if exempt[finding.ID] {
				skip = append(skip, finding)
			} else {
				fresh = append(fresh, finding)
			}
		}
		fresh, _ = filterFindingsByDiffScope(fresh, reviewCtx.DiffScopeHunks, reviewCtx.ChangedFiles)
		findings = append(skip, fresh...)
	}
	filtered, _, err := filterResultByDisplayPriority(&model.ReviewResult{Findings: findings}, f.priorityThreshold)
	if err != nil {
		return nil, err
	}
	if f.summarizePriorityThreshold != "" {
		if filtered, _, err = filterResultByDisplayPriority(filtered, f.summarizePriorityThreshold); err != nil {
			return nil, err
		}
	}
	filtered, _, err = filterByConfidenceThreshold(filtered, f.confidenceThreshold)
	if err != nil {
		return nil, err
	}
	kept := make(map[string]bool, len(filtered.Findings))
	for _, finding := range filtered.Findings {
		kept[finding.ID] = true
	}
	return kept, nil
}

// eligibleFunc filters a view of the active findings (published ids already
// adopted) and returns the ids that stay; exempt ids skip the diff-scope
// safeguard. A nil eligibleFunc keeps everything.
type eligibleFunc func(view []model.Finding, exempt map[string]bool) (map[string]bool, error)

// planReconcile computes the plan for the run's active findings, in four
// passes:
//  1. Candidates. A finding carrying a published id, or matching an open
//     published thread (the rule publishers use for already-posted
//     comments, also when merge kept a duplicate's id), is a candidate for
//     that thread. A finding matching a foreign thread is dropped.
//  2. Eligibility. Candidates, and findings that absorbed a published finding,
//     are exempt from diff scope: they were in scope when published. eligible
//     applies the remaining filters to every finding; what it removes is
//     dropped. No thread is reserved before this, so a candidate that fails a
//     filter cannot crowd out one that passes.
//  3. Survivors. Each thread keeps one surviving candidate: the one already
//     carrying the published id, else the most confident. It takes over the
//     thread's id; the other candidates are dropped as duplicates.
//  4. Records, for every published finding without a survivor: refuted by the
//     verifier → resolved with its remarks; merged into an active finding
//     that no longer matches its thread → resolved, pointing at that finding,
//     which is posted as new; otherwise (dropped by a filter or the
//     classifier) → kept open unchanged, since that proves nothing.
func planReconcile(active []model.Finding, baseline *model.PublishedReview, refuted map[string]string, absorbed *absorptionLog, headSHA string, eligible eligibleFunc) (*reconcilePlan, error) {
	plan := &reconcilePlan{baseline: baseline, headSHA: headSHA, renames: map[string]string{}, drop: map[string]bool{}, records: map[string]model.Finding{}, mergedInto: map[string]string{}, exempt: map[string]bool{}}
	prior := baseline.Review
	published := make(map[string]bool, len(prior.Findings))
	var open []model.Finding
	for _, f := range prior.Findings {
		published[f.ID] = true
		if f.Resolution == nil {
			open = append(open, postedShell(f))
		}
	}
	// 1. Candidates.
	thread := map[string]string{}
	for _, f := range active {
		if published[f.ID] {
			thread[f.ID] = f.ID
		} else if i := matchPosted(f, open); i >= 0 {
			thread[f.ID] = open[i].ID
		} else if matchPosted(f, baseline.Foreign) >= 0 {
			plan.drop[f.ID] = true
		}
	}
	// 2. Eligibility.
	absorbedPublished := map[string]bool{}
	for _, p := range prior.Findings {
		if survivor := absorbed.survivor(p.ID); survivor != "" {
			absorbedPublished[survivor] = true
		}
	}
	var view []model.Finding
	exempt := map[string]bool{}
	for _, f := range active {
		if plan.drop[f.ID] {
			continue
		}
		view = append(view, f)
		if thread[f.ID] != "" || absorbedPublished[f.ID] {
			exempt[f.ID] = true
		}
	}
	if eligible != nil {
		kept, err := eligible(view, exempt)
		if err != nil {
			return nil, err
		}
		for _, f := range view {
			if !kept[f.ID] {
				plan.drop[f.ID] = true
			}
		}
	}
	// 3. Survivors.
	survivorOf := map[string]model.Finding{}
	for _, f := range active {
		pid := thread[f.ID]
		if pid == "" || plan.drop[f.ID] {
			continue
		}
		current, taken := survivorOf[pid]
		switch {
		case !taken:
			survivorOf[pid] = f
		case current.ID != pid && (f.ID == pid || f.ConfidenceScore > current.ConfidenceScore):
			plan.drop[current.ID] = true
			survivorOf[pid] = f
		default:
			plan.drop[f.ID] = true
		}
	}
	for pid, f := range survivorOf {
		if f.ID != pid {
			plan.renames[f.ID] = pid
		}
	}
	viewID := func(id string) string {
		if pid, ok := plan.renames[id]; ok {
			return pid
		}
		return id
	}
	for id := range exempt {
		if !plan.drop[id] {
			plan.exempt[viewID(id)] = true
		}
	}
	// 4. Records.
	titles := map[string]string{}
	for _, f := range active {
		if !plan.drop[f.ID] {
			titles[f.ID], _, _, _ = reviewmd.FindingDisplay(f)
		}
	}
	for _, p := range prior.Findings {
		_, present := survivorOf[p.ID]
		survivor := absorbed.survivor(p.ID)
		switch {
		case p.Resolution != nil || present:
			plan.records[p.ID] = p
		case refuted[p.ID] != "":
			plan.records[p.ID] = resolvedFinding(p, resolutionSentence(refuted[p.ID]))
		case survivor != "" && titles[survivor] != "":
			plan.records[p.ID] = resolvedFinding(p, boundedResolution("Merged into the finding \u201c"+titles[survivor]+"\u201d of this re-review."))
			plan.mergedInto[p.ID] = viewID(survivor)
		default:
			plan.records[p.ID] = p
			plan.openRecords = append(plan.openRecords, p)
		}
	}
	return plan, nil
}

// keepMask reports, by position, which findings stay active. Take it before
// apply: after the renames a dropped original and the survivor that took over
// its thread can carry the same id, so only position still tells them apart.
func (p *reconcilePlan) keepMask(findings []model.Finding) []bool {
	mask := make([]bool, len(findings))
	for i, f := range findings {
		mask[i] = !p.drop[f.ID]
	}
	return mask
}

// apply renames the active findings in place (order and length unchanged, so
// positional id adoption downstream keeps working) and returns the ones that
// stay active: all but the dropped duplicates.
func (p *reconcilePlan) apply(findings []model.Finding) []model.Finding {
	kept := make([]model.Finding, 0, len(findings))
	for i := range findings {
		f := &findings[i]
		if p.drop[f.ID] {
			continue
		}
		if pid, ok := p.renames[f.ID]; ok {
			f.ID = pid
			if f.Verification != nil {
				f.Verification.ID = pid
			}
		}
		kept = append(kept, *f)
	}
	return kept
}

// layout composes the findings to publish: the published review's findings
// in their order, each replaced by the active finding with its id unless only
// provenance changed (so the thread is not rewritten for nothing), then the
// run's new findings. It decides nothing the plan did not already decide.
func (p *reconcilePlan) layout(active []model.Finding) []model.Finding {
	byID := make(map[string]model.Finding, len(active))
	for _, f := range active {
		byID[f.ID] = f
	}
	out := make([]model.Finding, 0, len(p.baseline.Review.Findings)+len(active))
	for _, prior := range p.baseline.Review.Findings {
		record := p.records[prior.ID]
		if into, merged := p.mergedInto[prior.ID]; merged {
			if _, published := byID[into]; !published {
				// The absorbing finding did not make it (a later step dropped
				// it); closing the thread would leave nothing in its place.
				record = prior
			}
		}
		if f, ok := byID[prior.ID]; ok && record.Resolution == nil && !sameDisplayedFinding(record, f) {
			f.Revision, f.Resolution = prior.Revision, nil
			out = append(out, f)
			continue
		}
		out = append(out, record)
	}
	for _, f := range active {
		if _, published := p.records[f.ID]; !published {
			f.Revision, f.Resolution = 0, nil
			out = append(out, f)
		}
	}
	return out
}

// isPublished reports whether id belongs to the published review.
func (p *reconcilePlan) isPublished(id string) bool {
	_, ok := p.records[id]
	return ok
}

// exemptFromDiffScope reports whether active finding id skips the final
// diff-scope safeguard.
func (p *reconcilePlan) exemptFromDiffScope(id string) bool {
	return p.isPublished(id) || p.exempt[id]
}

// reconcileForGroup builds the plan for the group a reconcile step names, or
// returns nil when there is nothing to reconcile with: the import found no
// published review, or every reviewer crashed (such a run says nothing about
// the code and must not replace the published verdict).
//
// The plan is made on the final active set: the diff-scope safeguard and the
// verdict's filters run first, and the findings they remove count as dropped. A published finding removed
// that way stays open as a record, and a finding that absorbed a published
// one only closes it when it survives.
func (st *PipelineState) reconcileForGroup(ctx context.Context, e *Engine, group string, active []model.Finding, filters verdictFilters) (*reconcilePlan, error) {
	st.mu.Lock()
	g := st.groupByID[group]
	if g == nil || g.provenance == nil || g.provenance.Baseline == nil {
		st.mu.Unlock()
		return nil, nil
	}
	if st.allReviewersFailedLocked() {
		st.mu.Unlock()
		st.addWarningf("Reconcile skipped: every reviewer failed, so the published review stays as it is")
		return nil, nil
	}
	baseline, refuted := g.provenance.Baseline, g.refuted
	headSHA := ""
	if st.Enriched != nil {
		headSHA = st.Enriched.DiffHeadSHA
	}
	reviewCtx := st.Enriched
	st.mu.Unlock()
	plan, err := planReconcile(active, baseline, refuted, st.absorbed, headSHA, func(view []model.Finding, exempt map[string]bool) (map[string]bool, error) {
		return filters.finalActive(view, exempt, reviewCtx)
	})
	if err != nil {
		return nil, fmt.Errorf("reconcile: applying final filters: %w", err)
	}
	e.logProgress(logging.StageReview, logging.StateDone, fmt.Sprintf("reconcile review=%s renamed=%d dropped=%d open_records=%d", baseline.Review.ReviewID, len(plan.renames), len(plan.drop), len(plan.openRecords)))
	return plan, nil
}

// reconcileStepFunc is the flat reconcile step: it plans against the flat
// result and applies the plan to it. Inside a pipeline the fused runner does
// the same right before the verdict.
//
// verdictOverride is the config of the verdict step that must directly follow
// (Spec.Validate enforces it), summarizeOverride that of a summarize step
// right after it (nil when there is none): reconcile applies their filters.
func (e *Engine) reconcileStepFunc(group string, verdictOverride, summarizeOverride *workflow.StepOverride, hasSummarize bool) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		filters := verdictFiltersOf(e.stepContext(verdictOverride, sc.Req).Req)
		if hasSummarize {
			filters.summarizePriorityThreshold = e.stepContext(summarizeOverride, sc.Req).Req.PriorityThreshold
		}
		st.mu.Lock()
		if st.result == nil {
			st.setResultLocked(st.materializeFromGroupsLocked(sc.Req))
		}
		in := st.result
		st.mu.Unlock()
		plan, err := st.reconcileForGroup(ctx, sc.Engine, group, in.Findings, filters)
		if plan == nil || err != nil {
			return err
		}
		out, err := in.Clone()
		if err != nil {
			return fmt.Errorf("reconcile: cloning result: %w", err)
		}
		out.Findings = plan.apply(out.Findings)
		st.mu.Lock()
		defer st.mu.Unlock()
		st.setFilteredResultLocked(out)
		st.reconciled = plan
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
// leave the step; this keeps the part reconcile needs.
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
