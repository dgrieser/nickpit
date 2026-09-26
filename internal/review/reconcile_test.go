package review

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
)

type publishedSource struct {
	stubSource
	published *model.PublishedReview
}

func (s *publishedSource) PublishedReview(context.Context, model.ReviewRequest) (*model.PublishedReview, error) {
	return s.published, nil
}

func reconcileTestFinding(id, file, title string) model.Finding {
	p := 2
	return model.Finding{ID: id, Title: title, Body: title + " evidence.", Priority: &p, ConfidenceScore: 0.8,
		CodeLocation: model.CodeLocation{FilePath: file, LineRange: model.LineRange{Start: 3, End: 3}, Content: "call()"}}
}

func testBaseline() *model.PublishedReview {
	closed := reconcileTestFinding("closed", "c.go", "Leaked file handle")
	closed.Resolution = &model.FindingResolution{Reason: "Fixed."}
	return &model.PublishedReview{
		Review: &model.ReviewResult{ReviewID: "published", Revision: 3, Findings: []model.Finding{
			reconcileTestFinding("kept", "a.go", "Missing nil check on user lookup"),
			reconcileTestFinding("changed", "b.go", "Unbounded retry loop"),
			reconcileTestFinding("refuted", "d.go", "Race on cache map"),
			reconcileTestFinding("folded", "e.go", "SQL built from user input"),
			reconcileTestFinding("dropped", "f.go", "Token logged in plain text"),
			closed,
			reconcileTestFinding("renamed", "i.go", "Stale cache entry after logout"),
		}},
		Foreign: []model.Finding{{ID: "legacy", Title: "Old legacy thread", CodeLocation: model.CodeLocation{FilePath: "g.go"}}},
	}
}

func TestPlanReconcileDecidesBeforeTheVerdict(t *testing.T) {
	baseline := testBaseline()
	kept := baseline.Review.Findings[0]
	kept.ConfidenceScore = 0.95 // only provenance moved
	changed := baseline.Review.Findings[1]
	changed.Body = "The retry loop never stops when the server keeps failing."
	folded := reconcileTestFinding("merge-survivor", "e.go", "SQL built from user input")
	folded.Body = "Merged wording."
	active := []model.Finding{
		kept, changed, folded,
		reconcileTestFinding("dup-of-kept", "a.go", "Missing nil check on user lookup"),
		reconcileTestFinding("dup-of-foreign", "g.go", "Old legacy thread"),
		reconcileTestFinding("new", "h.go", "Division by zero on empty input"),
		reconcileTestFinding("merge-renamed", "i.go", "Session invalidation leaves user data readable"),
	}
	absorbed := &absorptionLog{}
	absorbed.recordIDs("merge-renamed", []string{"renamed"})
	plan, err := planReconcile(active, baseline, map[string]string{"refuted": "The cache map is now guarded by a mutex. It was added in the last commit."}, absorbed, "head123", nil)
	if err != nil {
		t.Fatal(err)
	}

	if plan.renames["merge-survivor"] != "folded" || len(plan.renames) != 1 {
		t.Fatalf("renames = %v", plan.renames)
	}
	if !plan.drop["dup-of-kept"] || !plan.drop["dup-of-foreign"] || len(plan.drop) != 2 {
		t.Fatalf("drop = %v", plan.drop)
	}
	if len(plan.openRecords) != 1 || plan.openRecords[0].ID != "dropped" {
		t.Fatalf("open records = %+v", plan.openRecords)
	}
	if r := plan.records["refuted"].Resolution; r == nil || r.Reason != "The cache map is now guarded by a mutex." {
		t.Fatalf("refuted record = %+v", r)
	}
	if r := plan.records["renamed"].Resolution; r == nil || r.Reason != "Merged into the finding \u201cSession invalidation leaves user data readable\u201d of this re-review." {
		t.Fatalf("merged-away record = %+v", r)
	}

	// The active set the verdict sees: renamed in place, duplicates gone.
	stayActive := plan.apply(active)
	if active[2].ID != "folded" || len(active) != 7 {
		t.Fatal("apply must rename in place without reordering")
	}
	var ids []string
	for _, f := range stayActive {
		ids = append(ids, f.ID)
	}
	if strings.Join(ids, ",") != "kept,changed,folded,new,merge-renamed" {
		t.Fatalf("active = %v", ids)
	}

	// Layout only places what the plan decided.
	out := plan.layout(stayActive)
	byID := map[string]model.Finding{}
	ids = nil
	for _, f := range out {
		byID[f.ID] = f
		ids = append(ids, f.ID)
	}
	if strings.Join(ids, ",") != "kept,changed,refuted,folded,dropped,closed,renamed,new,merge-renamed" {
		t.Fatalf("layout = %v", ids)
	}
	if byID["kept"].ConfidenceScore != 0.8 {
		t.Fatal("provenance-only change rewrote the published finding")
	}
	if byID["changed"].Body != changed.Body || byID["folded"].Body != "Merged wording." || byID["folded"].Revision != 0 {
		t.Fatalf("updated findings = %+v / %+v", byID["changed"], byID["folded"])
	}
	if byID["dropped"].Resolution != nil || byID["dropped"].Body != baseline.Review.Findings[4].Body {
		t.Fatal("a finding dropped without refutation must stay as published")
	}
}

func TestImportFindingsSources(t *testing.T) {
	closed := reconcileTestFinding("closed", "c.go", "Closed")
	closed.Resolution = &model.FindingResolution{Reason: "Fixed."}
	published := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published", Findings: []model.Finding{
		reconcileTestFinding("open", "a.go", "Open"), closed,
	}}}
	file := writeFindingsFile(t, "scanner.json", model.ReviewResult{Findings: []model.Finding{reconcileTestFinding("00000000-0000-4000-8000-000000000009", "s.go", "Scanner hit")}})
	for name, tc := range map[string]struct {
		source   model.ReviewSource
		req      model.ReviewRequest
		cfg      importConfig
		want     int
		baseline bool
	}{
		"published, publishing":     {&publishedSource{published: published}, model.ReviewRequest{PostReview: true}, importConfig{group: "published", source: workflow.ImportSourcePublishedReview}, 1, true},
		"published, not publishing": {&publishedSource{published: published}, model.ReviewRequest{}, importConfig{group: "published", source: workflow.ImportSourcePublishedReview}, 0, false},
		"published, first review":   {&publishedSource{}, model.ReviewRequest{PostReview: true}, importConfig{group: "published", source: workflow.ImportSourcePublishedReview}, 0, false},
		"published, cannot read":    {stubSource{}, model.ReviewRequest{PostReview: true}, importConfig{group: "published", source: workflow.ImportSourcePublishedReview}, 0, false},
		"file":                      {stubSource{}, model.ReviewRequest{}, importConfig{group: "scanner", source: workflow.ImportSourceFile, findingsFrom: []string{file}, note: "Static scanner."}, 1, false},
	} {
		t.Run(name, func(t *testing.T) {
			e := NewEngine(tc.source, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
			st := newPipelineState(&model.ReviewContext{}, nil)
			entry := workflow.StepEntry{Type: workflow.StepImportFindings, FindingsFrom: tc.cfg.findingsFrom,
				Config: &workflow.StepOverride{Group: &tc.cfg.group, Source: &tc.cfg.source, Note: &tc.cfg.note}}
			if err := e.importFindingsStepFunc(entry)(context.Background(), e.stepContext(nil, tc.req), st); err != nil {
				t.Fatal(err)
			}
			vr, ok := st.vectorResult(tc.cfg.group)
			if !ok || len(vr.resp.Findings) != tc.want {
				t.Fatalf("group = %+v, %v", vr.resp, ok)
			}
			prov := st.groupProvenance(tc.cfg.group)
			if prov == nil || (prov.Baseline != nil) != tc.baseline {
				t.Fatalf("provenance = %+v", prov)
			}
			if tc.cfg.source == workflow.ImportSourcePublishedReview && (!prov.ExemptDiffScope || prov.Note != publishedFindingNote) {
				t.Fatalf("published provenance = %+v", prov)
			}
			if tc.cfg.source == workflow.ImportSourceFile && (prov.ExemptDiffScope || prov.Note != "Static scanner.") {
				t.Fatalf("file provenance = %+v", prov)
			}
		})
	}
}

// verify:<group> applies an imported group's provenance: its note reaches the
// verifier, an exempt group skips the diff-scope filter, and refutations are
// recorded on the group.
func TestVerifyImportedGroupAppliesProvenance(t *testing.T) {
	for name, exempt := range map[string]bool{"exempt": true, "in scope only": false} {
		t.Run(name, func(t *testing.T) {
			client := &scriptedVerifyLLM{responses: []*llm.ReviewResponse{
				{Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 2, ConfidenceScore: 0.9, Remarks: "Fixed in the caller."}},
			}}
			e := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
			reviewCtx := sampleReviewCtx()
			reviewCtx.DiffScopeHunks = []model.DiffHunk{{FilePath: "main.go", NewStart: 1, NewLines: 1, Content: "+x"}}
			st := newPipelineState(reviewCtx, nil)
			id := "00000000-0000-4000-8000-000000000003"
			outside := reconcileTestFinding(id, "other.go", "Missing guard far from the diff")
			outside.CodeLocation.LineRange = model.LineRange{Start: 50, End: 50}
			st.setImportedGroup("imported", agentResult{
				resp: &llm.ReviewResponse{Findings: []model.Finding{outside}},
				run:  model.AgentRun{Name: "Imported", Role: "import"},
			}, groupProvenance{Note: "might be outdated", ExemptDiffScope: exempt})
			req := model.ReviewRequest{VerifyDropPolicy: model.DropPolicyRefutedOnly, DisableWorkflowTimeBudget: true}
			if err := e.verifyVectorStepFunc("imported")(context.Background(), e.stepContext(nil, req), st); err != nil {
				t.Fatal(err)
			}
			var system string
			for _, r := range client.requests {
				if r.SchemaKind == llm.SchemaKindVerify {
					system = r.Messages[0].Content
				}
			}
			if !exempt {
				if system != "" {
					t.Fatal("an out-of-diff finding of a non-exempt group reached the verifier")
				}
				return
			}
			if !strings.Contains(system, "NOTE ON THIS FINDING: might be outdated") {
				t.Fatalf("verifier system prompt lacks the note:\n%s", system)
			}
			st.mu.Lock()
			refuted := st.groupByID["imported"].refuted[id]
			st.mu.Unlock()
			if refuted != "Fixed in the caller." {
				t.Fatalf("refutation not recorded: %q", refuted)
			}
		})
	}
}

func TestFlatReconcileStepAppliesPlanAndFeedsVerdictNotes(t *testing.T) {
	baseline := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published", Findings: []model.Finding{
		reconcileTestFinding("open", "a.go", "Missing nil check on user lookup"),
		reconcileTestFinding("carried", "b.go", "Token logged in plain text"),
	}}}
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	st := newPipelineState(&model.ReviewContext{DiffHeadSHA: "head123"}, nil)
	st.absorbed = &absorptionLog{}
	st.setImportedGroup("published", agentResult{resp: &llm.ReviewResponse{}, run: model.AgentRun{Role: "import"}}, groupProvenance{Baseline: baseline})
	st.result = &model.ReviewResult{Findings: []model.Finding{
		reconcileTestFinding("run-1", "a.go", "Missing nil check on user lookup"),
		reconcileTestFinding("run-2", "h.go", "Division by zero on empty input"),
	}}
	if err := e.reconcileStepFunc("published", nil, nil, false)(context.Background(), e.stepContext(nil, model.ReviewRequest{}), st); err != nil {
		t.Fatal(err)
	}
	if st.reconciled == nil || st.result.Findings[0].ID != "open" || len(st.result.Findings) != 2 {
		t.Fatalf("result after reconcile = %+v", st.result.Findings)
	}
	if _, records := st.verdictInputs(); len(records) != 1 || records[0].ID != "carried" {
		t.Fatalf("verdict open records = %+v", records)
	}
	res := (&Pipeline{engine: e}).assemble(st, model.ReviewRequest{})
	if res.ReviewID != "published" || res.Reconciliation == nil || res.Reconciliation.HeadSHA != "head123" || len(res.Findings) != 3 {
		t.Fatalf("assembled = %+v", res)
	}
}

func TestReconcileSkipsWithoutBaselineOrWhenReviewersCollapsed(t *testing.T) {
	baseline := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published"}}
	for name, setup := range map[string]func(st *PipelineState){
		"no baseline": func(st *PipelineState) {
			st.setImportedGroup("published", agentResult{resp: &llm.ReviewResponse{}}, groupProvenance{})
		},
		"collapsed": func(st *PipelineState) {
			st.setImportedGroup("published", agentResult{resp: &llm.ReviewResponse{}}, groupProvenance{Baseline: baseline})
			st.setGroup("security", agentResult{run: model.AgentRun{Role: "review", Status: model.AgentRunStatusFailed}}, nil)
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
			st := newPipelineState(&model.ReviewContext{}, nil)
			setup(st)
			st.result = &model.ReviewResult{}
			if err := e.reconcileStepFunc("published", nil, nil, false)(context.Background(), e.stepContext(nil, model.ReviewRequest{}), st); err != nil {
				t.Fatal(err)
			}
			if st.reconciled != nil {
				t.Fatal("reconcile planned without a usable baseline")
			}
		})
	}
}

func TestDefaultSpecBinds(t *testing.T) {
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	if _, err := e.BuildPipeline(workflow.DefaultSpec()); err != nil {
		t.Fatalf("default spec does not bind: %v", err)
	}
}

func TestResolutionSentence(t *testing.T) {
	for in, want := range map[string]string{
		"The guard exists now. Details follow.": "The guard exists now.",
		"no period":                             "no period.",
		"":                                      "Re-verification found the problem is no longer present.",
	} {
		if got := resolutionSentence(in); got != want {
			t.Errorf("resolutionSentence(%q) = %q, want %q", in, got, want)
		}
	}
	if got := resolutionSentence(strings.Repeat("a", 400)); len([]rune(got)) > 240 {
		t.Fatalf("long remarks not bounded: %d runes", len([]rune(got)))
	}
}

func TestAbsorptionLogFollowsChains(t *testing.T) {
	var none *absorptionLog
	none.recordIDs("x", []string{"y"})
	if none.survivor("y") != "" {
		t.Fatal("nil log must record nothing")
	}
	l := &absorptionLog{}
	l.recordIDs("b", []string{"a", "b", ""})
	l.recordIDs("c", []string{"b"})
	if got := l.survivor("a"); got != "c" {
		t.Fatalf("survivor(a) = %q, want c", got)
	}
	if got := l.survivor("c"); got != "" {
		t.Fatalf("survivor(c) = %q, want none", got)
	}
}

func TestMechanicalDedupeRecordsAbsorptions(t *testing.T) {
	l := &absorptionLog{}
	a := reconcileTestFinding("a", "a.go", "Missing nil check on user lookup")
	b := reconcileTestFinding("b", "a.go", "Missing nil check on user lookup")
	b.ConfidenceScore = 0.9
	out, folded := mechanicallyDedupeFindings(withAbsorptions(context.Background(), l), []model.Finding{a, b})
	if folded != 1 || len(out) != 1 {
		t.Fatalf("fold = %d, %d findings", folded, len(out))
	}
	loser := "a"
	if out[0].ID == "a" {
		loser = "b"
	}
	if l.survivor(loser) != out[0].ID {
		t.Fatalf("absorption not recorded: %v", l.into)
	}
}

func TestVerifierPromptOmitsNoteSectionWithoutNote(t *testing.T) {
	client := &scriptedVerifyLLM{}
	e := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: []model.Finding{reconcileTestFinding("00000000-0000-4000-8000-000000000002", "main.go", "Missing guard")}},
		run:  model.AgentRun{Name: "Security", Role: "review", Status: model.AgentRunStatusOK},
	}}
	if _, _, err := e.verifyAndFilterVectorFindings(context.Background(), sampleReviewCtx(), vectorResults, nil, model.ReviewRequest{}, NewLimiter(1), "", internalAgentContext{}, disabledVerifyPhaseBudgets(context.Background())); err != nil {
		t.Fatal(err)
	}
	for _, r := range client.requests {
		if r.SchemaKind == llm.SchemaKindVerify && strings.Contains(r.Messages[0].Content, "NOTE ON THIS FINDING") {
			t.Fatalf("note section rendered without a note:\n%s", r.Messages[0].Content)
		}
	}
}

// The plan is made on the verdict's final active set: a finding the verdict
// would filter out neither closes the published finding it absorbed nor keeps
// a published finding active.
func TestReconcileAppliesVerdictFiltersBeforePlanning(t *testing.T) {
	baseline := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published", Findings: []model.Finding{
		reconcileTestFinding("absorbed", "i.go", "Stale cache entry after logout"),
		reconcileTestFinding("weak", "a.go", "Missing nil check on user lookup"),
	}}}
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	st := newPipelineState(&model.ReviewContext{}, nil)
	st.absorbed = &absorptionLog{}
	st.absorbed.recordIDs("survivor", []string{"absorbed"})
	st.setImportedGroup("published", agentResult{resp: &llm.ReviewResponse{}}, groupProvenance{Baseline: baseline})
	survivor := reconcileTestFinding("survivor", "i.go", "Session invalidation leaves user data readable")
	survivor.ConfidenceScore = 0.3
	weak := baseline.Review.Findings[1]
	weak.ConfidenceScore = 0.2
	strong := reconcileTestFinding("strong", "h.go", "Division by zero on empty input")
	st.result = &model.ReviewResult{Findings: []model.Finding{survivor, weak, strong}}
	threshold := 0.5
	verdict := &workflow.StepOverride{ConfidenceThreshold: &threshold}
	if err := e.reconcileStepFunc("published", verdict, nil, false)(context.Background(), e.stepContext(nil, model.ReviewRequest{}), st); err != nil {
		t.Fatal(err)
	}
	if len(st.result.Findings) != 1 || st.result.Findings[0].ID != "strong" {
		t.Fatalf("active after reconcile = %+v", st.result.Findings)
	}
	_, records := st.verdictInputs()
	var ids []string
	for _, r := range records {
		ids = append(ids, r.ID)
	}
	if strings.Join(ids, ",") != "absorbed,weak" {
		t.Fatalf("open records = %v, want both published findings kept open", ids)
	}
	res := (&Pipeline{engine: e}).assemble(st, model.ReviewRequest{})
	for _, f := range res.Findings {
		if f.Resolution != nil {
			t.Fatalf("finding %s closed although nothing replaced it", f.ID)
		}
	}
}

// layout keeps a published finding open when the finding that absorbed it is
// not published after all.
func TestLayoutReopensMergedAwayFindingWithoutSurvivor(t *testing.T) {
	baseline := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published", Findings: []model.Finding{
		reconcileTestFinding("absorbed", "i.go", "Stale cache entry after logout"),
	}}}
	absorbed := &absorptionLog{}
	absorbed.recordIDs("survivor", []string{"absorbed"})
	survivor := reconcileTestFinding("survivor", "i.go", "Session invalidation leaves user data readable")
	plan, err := planReconcile([]model.Finding{survivor}, baseline, nil, absorbed, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.records["absorbed"].Resolution == nil {
		t.Fatal("plan should resolve the absorbed finding")
	}
	out := plan.layout(nil)
	if len(out) != 1 || out[0].Resolution != nil {
		t.Fatalf("layout = %+v, want the published finding open", out)
	}
}

// Bare verify and verify:<group> share group-aware verification.
func TestBareVerifyHonorsImportedGroupProvenance(t *testing.T) {
	client := &scriptedVerifyLLM{responses: []*llm.ReviewResponse{
		{Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 2, ConfidenceScore: 0.9, Remarks: "Gone."}},
	}}
	e := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	reviewCtx := sampleReviewCtx()
	reviewCtx.DiffScopeHunks = []model.DiffHunk{{FilePath: "main.go", NewStart: 1, NewLines: 1, Content: "+x"}}
	st := newPipelineState(reviewCtx, []string{"security"})
	reviewerOutside := reconcileTestFinding("00000000-0000-4000-8000-000000000011", "other.go", "Reviewer finding outside the diff")
	reviewerOutside.CodeLocation.LineRange = model.LineRange{Start: 70, End: 70}
	st.setGroup("security", agentResult{resp: &llm.ReviewResponse{Findings: []model.Finding{reviewerOutside}}, run: model.AgentRun{Name: "Security", Role: "review"}}, nil)
	id := "00000000-0000-4000-8000-000000000012"
	importedOutside := reconcileTestFinding(id, "other.go", "Published finding outside the diff")
	importedOutside.CodeLocation.LineRange = model.LineRange{Start: 50, End: 50}
	st.setImportedGroup("published", agentResult{resp: &llm.ReviewResponse{Findings: []model.Finding{importedOutside}}, run: model.AgentRun{Name: "Imported", Role: "import"}},
		groupProvenance{Note: "might be outdated", ExemptDiffScope: true})
	req := model.ReviewRequest{VerifyDropPolicy: model.DropPolicyRefutedOnly, DisableWorkflowTimeBudget: true}
	if err := e.verifyStepFunc(nil)(context.Background(), e.stepContext(nil, req), st); err != nil {
		t.Fatal(err)
	}
	var verified []string
	for _, r := range client.requests {
		if r.SchemaKind == llm.SchemaKindVerify {
			verified = append(verified, r.Messages[0].Content)
		}
	}
	if len(verified) != 1 || !strings.Contains(verified[0], "NOTE ON THIS FINDING: might be outdated") {
		t.Fatalf("verified %d findings, want only the exempt imported one with its note", len(verified))
	}
	st.mu.Lock()
	refuted := st.groupByID["published"].refuted[id]
	st.mu.Unlock()
	if refuted != "Gone." {
		t.Fatalf("bare verify did not record the refutation: %q", refuted)
	}
}

func TestVerdictDeterministicPathNamesOpenRecords(t *testing.T) {
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	p0 := 0
	blocking := reconcileTestFinding("blocking", "a.go", "Credentials written to the log")
	blocking.Priority = &p0
	for name, tc := range map[string]struct {
		records     []model.Finding
		correctness string
	}{
		"minor record": {[]model.Finding{reconcileTestFinding("minor", "b.go", "Unbounded retry loop")}, "patch is correct"},
		// Records shape wording only; correctness comes from active findings.
		"blocking record": {[]model.Finding{blocking}, "patch is correct"},
	} {
		t.Run(name, func(t *testing.T) {
			out, run, err := e.Verdict(context.Background(), &model.ReviewContext{}, &model.ReviewResult{}, VerdictOptions{DisablePatchSummary: true, OpenRecords: tc.records})
			if err != nil {
				t.Fatal(err)
			}
			if run.Status != model.AgentRunStatusSkipped {
				t.Fatal("deterministic path called the agent")
			}
			if out.OverallCorrectness != tc.correctness || !strings.Contains(out.OverallExplanation, "1 finding from an earlier review of this change stays open without re-verification in this run: P") {
				t.Fatalf("verdict = %q / %q", out.OverallCorrectness, out.OverallExplanation)
			}
		})
	}
}

// Diff scope is decided before planning and agrees with assembly: a
// replacement that takes over a published thread is exempt in both (the
// verdict assesses it and it is published), while a new finding outside the
// diff is filtered before the verdict ever sees it.
func TestReconcileAppliesDiffScopeBeforePlanning(t *testing.T) {
	baseline := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published", Findings: []model.Finding{
		reconcileTestFinding("published", "other.go", "Missing guard far from the diff"),
	}}}
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	reviewCtx := &model.ReviewContext{
		DiffScopeHunks: []model.DiffHunk{{FilePath: "main.go", NewStart: 1, NewLines: 1, Content: "+x"}},
		ChangedFiles:   []model.ChangedFile{{Path: "main.go", Status: model.FileModified}},
	}
	st := newPipelineState(reviewCtx, nil)
	st.absorbed = &absorptionLog{}
	st.setImportedGroup("published", agentResult{resp: &llm.ReviewResponse{}}, groupProvenance{Baseline: baseline})
	replacement := reconcileTestFinding("replacement", "other.go", "Missing guard far from the diff")
	replacement.CodeLocation.LineRange = model.LineRange{Start: 90, End: 90}
	outside := reconcileTestFinding("outside", "far.go", "Unrelated finding outside the diff")
	outside.CodeLocation.LineRange = model.LineRange{Start: 12, End: 12}
	st.result = &model.ReviewResult{Findings: []model.Finding{replacement, outside}}
	if err := e.reconcileStepFunc("published", nil, nil, false)(context.Background(), e.stepContext(nil, model.ReviewRequest{}), st); err != nil {
		t.Fatal(err)
	}
	if len(st.result.Findings) != 1 || st.result.Findings[0].ID != "published" {
		t.Fatalf("verdict input = %+v, want only the replacement under the published id", st.result.Findings)
	}
	res := (&Pipeline{engine: e}).assemble(st, model.ReviewRequest{})
	if len(res.Findings) != 1 || res.Findings[0].ID != "published" || res.Findings[0].CodeLocation.LineRange.Start != 90 {
		t.Fatalf("assembled = %+v, want the replacement the verdict saw", res.Findings)
	}
}

type failingVerdictLLM struct{}

func (failingVerdictLLM) Review(context.Context, *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	return nil, errors.New("model unavailable")
}

// A failed verdict still names the open records, in the flat and the fused
// (runVerdictShard) paths.
func TestVerdictFailureKeepsOpenRecordsNotice(t *testing.T) {
	record := reconcileTestFinding("carried", "b.go", "Token logged in plain text")
	newState := func() *PipelineState {
		st := newPipelineState(&model.ReviewContext{}, nil)
		st.reconciled = &reconcilePlan{openRecords: []model.Finding{record}}
		active := reconcileTestFinding("active", "a.go", "Missing nil check on user lookup")
		st.result = &model.ReviewResult{OverallExplanation: "Merged.", Findings: []model.Finding{active}}
		return st
	}
	e := NewEngine(stubSource{}, failingVerdictLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	sc := e.stepContext(nil, model.ReviewRequest{MaxOutputRetries: 1, DisableWorkflowTimeBudget: true})

	flat := newState()
	if err := e.verdictStepFunc(nil)(context.Background(), sc, flat); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(flat.result.OverallExplanation, "Token logged in plain text") {
		t.Fatalf("flat fallback explanation = %q", flat.result.OverallExplanation)
	}

	fused := newState()
	out, _, _ := runVerdictShard(context.Background(), sc, fused, fused.result)
	if !strings.Contains(out.OverallExplanation, "Token logged in plain text") {
		t.Fatalf("fused fallback explanation = %q", out.OverallExplanation)
	}
}

// A confirmed published finding outside the latest diff that merge folded into
// a duplicate keeping the duplicate's id keeps its diff exemption: identity is
// resolved before the filters, so it stays active and counts for the verdict.
// Also when the absorbing finding's title no longer matches the thread, the
// absorber inherits the exemption through merge provenance.
func TestReconcileKeepsDiffExemptionThroughMerge(t *testing.T) {
	p0 := 0
	published := reconcileTestFinding("published", "other.go", "Credentials written to the log")
	published.Priority = &p0
	published.CodeLocation.LineRange = model.LineRange{Start: 90, End: 90}
	baseline := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published-review", Findings: []model.Finding{published}}}
	reviewCtx := &model.ReviewContext{
		DiffScopeHunks: []model.DiffHunk{{FilePath: "main.go", NewStart: 1, NewLines: 1, Content: "+x"}},
		ChangedFiles:   []model.ChangedFile{{Path: "main.go", Status: model.FileModified}},
	}
	for name, tc := range map[string]struct {
		title      string
		wantActive string
		wantMerged bool
	}{
		"duplicate keeps its id": {"Credentials written to the log", "published", false},
		"absorber retitled":      {"Secrets leak through debug logging of the request", "survivor", true},
	} {
		t.Run(name, func(t *testing.T) {
			e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
			st := newPipelineState(reviewCtx, nil)
			st.absorbed = &absorptionLog{}
			st.absorbed.recordIDs("survivor", []string{"published"})
			st.setImportedGroup("published", agentResult{resp: &llm.ReviewResponse{}}, groupProvenance{Baseline: baseline})
			survivor := reconcileTestFinding("survivor", "other.go", tc.title)
			survivor.Priority = &p0
			survivor.CodeLocation.LineRange = model.LineRange{Start: 90, End: 90}
			st.result = &model.ReviewResult{Findings: []model.Finding{survivor}}
			if err := e.reconcileStepFunc("published", nil, nil, false)(context.Background(), e.stepContext(nil, model.ReviewRequest{}), st); err != nil {
				t.Fatal(err)
			}
			if len(st.result.Findings) != 1 || st.result.Findings[0].ID != tc.wantActive {
				t.Fatalf("active = %+v, want %s kept for the verdict", st.result.Findings, tc.wantActive)
			}
			if _, records := st.verdictInputs(); len(records) != 0 {
				t.Fatalf("open records = %+v, want none", records)
			}
			res := (&Pipeline{engine: e}).assemble(st, model.ReviewRequest{})
			byID := map[string]model.Finding{}
			for _, f := range res.Findings {
				byID[f.ID] = f
			}
			if _, ok := byID[tc.wantActive]; !ok {
				t.Fatalf("assembly dropped the exempt finding: %+v", res.Findings)
			}
			if merged := byID["published"].Resolution != nil; merged != tc.wantMerged {
				t.Fatalf("published resolution = %+v, want merged=%v", byID["published"].Resolution, tc.wantMerged)
			}
		})
	}
}

// Thread survivors are chosen after the filters: a published candidate that
// fails the confidence threshold does not crowd out a confident duplicate,
// which then takes over the thread.
func TestReconcileChoosesThreadSurvivorAfterFilters(t *testing.T) {
	baseline := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published-review", Findings: []model.Finding{
		reconcileTestFinding("published", "a.go", "Missing nil check on user lookup"),
	}}}
	for name, tc := range map[string]struct {
		publishedConfidence, duplicateConfidence float64
		wantBody                                 string
	}{
		"published candidate filtered": {0.1, 0.95, "duplicate wording"},
		"both pass, published kept":    {0.9, 0.95, "published wording"},
	} {
		t.Run(name, func(t *testing.T) {
			e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
			st := newPipelineState(&model.ReviewContext{}, nil)
			st.absorbed = &absorptionLog{}
			st.setImportedGroup("published", agentResult{resp: &llm.ReviewResponse{}}, groupProvenance{Baseline: baseline})
			candidate := baseline.Review.Findings[0]
			candidate.ConfidenceScore, candidate.Body = tc.publishedConfidence, "published wording"
			duplicate := reconcileTestFinding("duplicate", "a.go", "Missing nil check on user lookup")
			duplicate.ConfidenceScore, duplicate.Body = tc.duplicateConfidence, "duplicate wording"
			st.result = &model.ReviewResult{Findings: []model.Finding{candidate, duplicate}}
			threshold := 0.5
			verdict := &workflow.StepOverride{ConfidenceThreshold: &threshold}
			if err := e.reconcileStepFunc("published", verdict, nil, false)(context.Background(), e.stepContext(nil, model.ReviewRequest{}), st); err != nil {
				t.Fatal(err)
			}
			if len(st.result.Findings) != 1 || st.result.Findings[0].ID != "published" || st.result.Findings[0].Body != tc.wantBody {
				t.Fatalf("active = %+v, want one finding on the published thread with %q", st.result.Findings, tc.wantBody)
			}
			if _, records := st.verdictInputs(); len(records) != 0 {
				t.Fatalf("open records = %+v, want none", records)
			}
		})
	}
}

// A summarize step after the verdict filters the published result too, so a
// stricter summarize threshold counts in reconcile's plan.
func TestReconcileAppliesSummarizePriorityFilter(t *testing.T) {
	baseline := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published-review", Findings: []model.Finding{
		reconcileTestFinding("published", "a.go", "Missing nil check on user lookup"),
	}}}
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	st := newPipelineState(&model.ReviewContext{}, nil)
	st.absorbed = &absorptionLog{}
	st.setImportedGroup("published", agentResult{resp: &llm.ReviewResponse{}}, groupProvenance{Baseline: baseline})
	st.result = &model.ReviewResult{Findings: []model.Finding{baseline.Review.Findings[0]}} // P2
	threshold := "p1"
	summarize := &workflow.StepOverride{PriorityThreshold: &threshold}
	if err := e.reconcileStepFunc("published", nil, summarize, true)(context.Background(), e.stepContext(nil, model.ReviewRequest{}), st); err != nil {
		t.Fatal(err)
	}
	if len(st.result.Findings) != 0 {
		t.Fatalf("active = %+v, want the P2 finding filtered by summarize's p1", st.result.Findings)
	}
	if _, records := st.verdictInputs(); len(records) != 1 {
		t.Fatalf("open records = %+v, want the published finding kept open", records)
	}
}
