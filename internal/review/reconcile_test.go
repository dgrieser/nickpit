package review

import (
	"context"
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

// loggedState returns a pipeline state with a finding log, and a context that
// carries it, as Pipeline.Run sets them up.
func loggedState(reviewCtx *model.ReviewContext, order []string) (*PipelineState, context.Context) {
	st := newPipelineState(reviewCtx, order)
	st.findings = &findingLog{}
	return st, withFindingLog(context.Background(), st.findings)
}

// Every published finding the final findings no longer carry gets the fate
// the run recorded for it; only an unexplained loss keeps it open.
func TestPlanReconcileMapsRecordedFates(t *testing.T) {
	closed := reconcileTestFinding("closed", "c.go", "Leaked file handle")
	closed.Resolution = &model.FindingResolution{Reason: "Fixed."}
	baseline := &model.PublishedReview{
		Review: &model.ReviewResult{ReviewID: "published", Revision: 3, Findings: []model.Finding{
			reconcileTestFinding("kept", "a.go", "Missing nil check on user lookup"),
			reconcileTestFinding("changed", "b.go", "Unbounded retry loop"),
			reconcileTestFinding("refuted", "d.go", "Race on cache map"),
			reconcileTestFinding("classified", "e.go", "Looks fine"),
			reconcileTestFinding("merged", "f.go", "Same problem, second report"),
			reconcileTestFinding("chained", "g.go", "Absorbed into a filtered finding"),
			reconcileTestFinding("filtered", "h.go", "Minor style nit"),
			reconcileTestFinding("lost", "i.go", "Vanished without a reason"),
			closed,
		}},
		Foreign: []model.Finding{{ID: "legacy", Title: "Old legacy thread", CodeLocation: model.CodeLocation{FilePath: "l.go"}}},
	}
	log := &findingLog{}
	log.remove("refuted", resolutionSentence("The cache map is now guarded by a mutex. It was added later."))
	log.remove("classified", categorizeDropReason([]string{model.CategoryConfirmation}, model.VerdictRefuted))
	log.recordIDs("kept", []string{"merged"})
	log.recordIDs("gone", []string{"chained"})
	log.remove("gone", priorityDropReason("p2"))
	log.remove("filtered", priorityDropReason("p2"))

	kept := baseline.Review.Findings[0]
	kept.ConfidenceScore = 0.95 // only provenance moved
	changed := baseline.Review.Findings[1]
	changed.Body = "The retry loop never stops when the server keeps failing."
	final := []model.Finding{kept, changed,
		reconcileTestFinding("new", "n.go", "Division by zero on empty input"),
		reconcileTestFinding("dup-of-foreign", "l.go", "Old legacy thread"),
	}
	plan := planReconcile(final, baseline, log, "head123")
	out := plan.layout(final)

	byID := map[string]model.Finding{}
	var ids []string
	for _, f := range out {
		byID[f.ID] = f
		ids = append(ids, f.ID)
	}
	if strings.Join(ids, ",") != "kept,changed,refuted,classified,merged,chained,filtered,lost,closed,new" {
		t.Fatalf("layout = %v", ids)
	}
	if byID["kept"].ConfidenceScore != 0.8 || byID["changed"].Body != changed.Body {
		t.Fatal("present findings: provenance-only change must keep the published text, a visible change must apply")
	}
	for id, want := range map[string]string{
		"refuted":    "The cache map is now guarded by a mutex.",
		"classified": "Re-review classified it as a confirmation of correct code, not a problem.",
		"merged":     "Merged into the finding “Missing nil check on user lookup” of this re-review.",
		"chained":    "Re-review rated it below the P2 priority threshold.",
		"filtered":   "Re-review rated it below the P2 priority threshold.",
	} {
		if r := byID[id].Resolution; r == nil || r.Reason != want {
			t.Errorf("%s resolution = %+v, want %q", id, r, want)
		}
	}
	if byID["lost"].Resolution != nil || byID["lost"].Body != baseline.Review.Findings[7].Body {
		t.Fatal("a finding lost without a recorded reason must stay open unchanged")
	}
	if byID["closed"].Resolution == nil {
		t.Fatal("an already resolved finding must stay resolved")
	}
}

// A published finding survives a mechanical fold with its id and text, even
// against a more confident duplicate, which only extends it.
func TestFoldClusterPrefersPublishedFinding(t *testing.T) {
	log := &findingLog{}
	log.prefer([]string{"published"})
	published := reconcileTestFinding("published", "a.go", "Missing nil check on user lookup")
	published.ConfidenceScore = 0.3
	duplicate := reconcileTestFinding("fresh", "a.go", "Missing nil check on user lookup")
	duplicate.ConfidenceScore, duplicate.Body = 0.9, "Fresh wording."
	folded := foldCluster(log, []model.Finding{duplicate, published})
	if folded.ID != "published" || folded.Body != published.Body {
		t.Fatalf("folded = %+v, want the published finding as base", folded)
	}
	if folded.ConfidenceScore <= 0.9 {
		t.Fatalf("confidence = %.2f, want the duplicate folded in", folded.ConfidenceScore)
	}
	if got := foldCluster(&findingLog{}, []model.Finding{published, duplicate}); got.ID != "fresh" {
		t.Fatalf("without a preferred member the strongest finding is the base, got %s", got.ID)
	}
}

// An LLM merge that absorbed a published finding under another id hands the
// published id to the merged finding; with two, the more confident one wins
// and the other stays absorbed.
func TestKeepPreferredSurvivors(t *testing.T) {
	log := &findingLog{}
	log.prefer([]string{"p1", "p2"})
	cluster := []model.Finding{
		{ID: "fresh", ConfidenceScore: 0.9},
		{ID: "p1", ConfidenceScore: 0.4},
		{ID: "p2", ConfidenceScore: 0.7},
	}
	merged := []model.Finding{{ID: "fresh", Body: "Merged wording.", MergedFrom: []string{"p1", "p2"},
		Verification: &model.FindingVerification{ID: "fresh"}}}
	keepPreferredSurvivors(log, merged, cluster)
	if merged[0].ID != "p2" || merged[0].Verification.ID != "p2" || merged[0].Body != "Merged wording." {
		t.Fatalf("merged = %+v, want the most confident published id with the merge's text", merged[0])
	}
	if strings.Join(merged[0].MergedFrom, ",") != "p1,fresh" {
		t.Fatalf("merged_from = %v, want the other published finding and the fresh one absorbed", merged[0].MergedFrom)
	}
	unchanged := []model.Finding{{ID: "p1", MergedFrom: []string{"fresh"}}}
	keepPreferredSurvivors(log, unchanged, cluster)
	if unchanged[0].ID != "p1" {
		t.Fatal("a merged finding already carrying a published id keeps it")
	}
}

// Dedupe skips a group marked as already deduplicated unless forced.
func TestDedupeSkipsImportedGroupUnlessForced(t *testing.T) {
	for name, force := range map[string]bool{"default": false, "forced": true} {
		t.Run(name, func(t *testing.T) {
			client := &multiAgentLLM{}
			e := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
			st, ctx := loggedState(&model.ReviewContext{}, nil)
			a := reconcileTestFinding("00000000-0000-4000-8000-000000000021", "a.go", "First published finding")
			b := reconcileTestFinding("00000000-0000-4000-8000-000000000022", "z.go", "Second published finding")
			st.setImportedGroup("imported", agentResult{resp: &llm.ReviewResponse{Findings: []model.Finding{a, b}}, run: model.AgentRun{Name: "Imported", Role: "import"}},
				groupProvenance{SkipDedupe: true})
			req := model.ReviewRequest{ForceDedupeImported: force, DisableWorkflowTimeBudget: true}
			if err := e.dedupeVectorStepFunc("imported")(ctx, e.stepContext(nil, req), st); err != nil {
				t.Fatal(err)
			}
			if ran := len(client.mergeRequests) > 0; ran != force {
				t.Fatalf("dedupe agent ran = %v, want %v", ran, force)
			}
		})
	}
}

// The classifier and the verifier record why they removed a finding.
func TestVerificationRecordsDropReasons(t *testing.T) {
	client := &scriptedVerifyLLM{responses: []*llm.ReviewResponse{
		{Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 2, ConfidenceScore: 0.9, Remarks: "The guard exists now. More detail."}},
	}}
	e := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	st, ctx := loggedState(sampleReviewCtx(), nil)
	id := "00000000-0000-4000-8000-000000000031"
	st.setImportedGroup("imported", agentResult{resp: &llm.ReviewResponse{Findings: []model.Finding{reconcileTestFinding(id, "main.go", "Missing guard")}}, run: model.AgentRun{Name: "Imported", Role: "import"}},
		groupProvenance{ExemptDiffScope: true})
	req := model.ReviewRequest{VerifyDropPolicy: model.DropPolicyRefutedOnly, DisableWorkflowTimeBudget: true}
	if err := e.verifyVectorStepFunc("imported")(ctx, e.stepContext(nil, req), st); err != nil {
		t.Fatal(err)
	}
	if _, reason := st.findings.fate(id); reason != "The guard exists now." {
		t.Fatalf("refutation reason = %q", reason)
	}

	categorizeClient := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{categorized(model.CategoryConfirmation)}}
	e = NewEngine(stubSource{}, categorizeClient, stubRetrieval{}, config.Profile{Model: "test"})
	log := &findingLog{}
	cid := "00000000-0000-4000-8000-000000000032"
	results := []agentResult{{resp: &llm.ReviewResponse{Findings: []model.Finding{reconcileTestFinding(cid, "main.go", "Looks fine")}}, run: model.AgentRun{Name: "Imported", Role: "import"}}}
	if _, _, err := e.categorizeAndFilterVectorFindings(withFindingLog(context.Background(), log), sampleReviewCtx(), results, model.ReviewRequest{VerifyDropPolicy: model.DropPolicyRefutedOnly}, NewLimiter(1), ""); err != nil {
		t.Fatal(err)
	}
	if _, reason := log.fate(cid); !strings.Contains(reason, "confirmation") {
		t.Fatalf("classifier reason = %q", reason)
	}
}

// The verdict's filters record why they removed a finding.
func TestVerdictFiltersRecordDropReasons(t *testing.T) {
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	log := &findingLog{}
	low := reconcileTestFinding("low", "a.go", "Low priority")
	p3 := 3
	low.Priority = &p3
	weak := reconcileTestFinding("weak", "b.go", "Weak")
	weak.ConfidenceScore = 0.2
	_, _, err := e.Verdict(withFindingLog(context.Background(), log), &model.ReviewContext{}, &model.ReviewResult{Findings: []model.Finding{low, weak}},
		VerdictOptions{PriorityThreshold: "p2", ConfidenceThreshold: 0.5, DisablePatchSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, reason := log.fate("low"); reason != "Re-review rated it below the P2 priority threshold." {
		t.Fatalf("priority reason = %q", reason)
	}
	if _, reason := log.fate("weak"); reason != "Re-review scored its confidence below the 0.50 threshold." {
		t.Fatalf("confidence reason = %q", reason)
	}
}

func TestReconcileStepPlansAndAssemblyLaysOut(t *testing.T) {
	baseline := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published", Revision: 2, Findings: []model.Finding{
		reconcileTestFinding("present", "a.go", "Missing nil check on user lookup"),
		reconcileTestFinding("refuted", "b.go", "Token logged in plain text"),
	}}}
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	st, ctx := loggedState(&model.ReviewContext{DiffHeadSHA: "head123"}, nil)
	st.setImportedGroup("imported", agentResult{resp: &llm.ReviewResponse{}, run: model.AgentRun{Role: "import"}}, groupProvenance{Baseline: baseline})
	st.findings.remove("refuted", "Gone.")
	st.result = &model.ReviewResult{Findings: []model.Finding{
		baseline.Review.Findings[0],
		reconcileTestFinding("new", "h.go", "Division by zero on empty input"),
	}}
	if err := e.reconcileStepFunc("imported")(ctx, e.stepContext(nil, model.ReviewRequest{}), st); err != nil {
		t.Fatal(err)
	}
	res := (&Pipeline{engine: e}).assemble(st, model.ReviewRequest{})
	if res.ReviewID != "published" || res.Revision != 2 || res.Reconciliation == nil || res.Reconciliation.HeadSHA != "head123" || len(res.Findings) != 3 {
		t.Fatalf("assembled = %+v", res)
	}
	if res.Findings[1].ID != "refuted" || res.Findings[1].Resolution == nil || res.Findings[2].ID != "new" {
		t.Fatalf("findings = %+v", res.Findings)
	}
}

// Imported findings of the published review stay exempt from the final
// diff-scope safeguard.
func TestAssemblyKeepsPublishedFindingsOutsideDiff(t *testing.T) {
	baseline := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published", Findings: []model.Finding{
		reconcileTestFinding("published", "other.go", "Missing guard far from the diff"),
	}}}
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	reviewCtx := &model.ReviewContext{
		DiffScopeHunks: []model.DiffHunk{{FilePath: "main.go", NewStart: 1, NewLines: 1, Content: "+x"}},
		ChangedFiles:   []model.ChangedFile{{Path: "main.go", Status: model.FileModified}},
	}
	st, ctx := loggedState(reviewCtx, nil)
	st.setImportedGroup("imported", agentResult{resp: &llm.ReviewResponse{}}, groupProvenance{Baseline: baseline})
	outside := reconcileTestFinding("new-outside", "far.go", "New finding outside the diff")
	st.result = &model.ReviewResult{Findings: []model.Finding{baseline.Review.Findings[0], outside}}
	if err := e.reconcileStepFunc("imported")(ctx, e.stepContext(nil, model.ReviewRequest{}), st); err != nil {
		t.Fatal(err)
	}
	res := (&Pipeline{engine: e}).assemble(st, model.ReviewRequest{})
	if len(res.Findings) != 1 || res.Findings[0].ID != "published" || res.Findings[0].Resolution != nil {
		t.Fatalf("assembled = %+v, want only the published finding, still open", res.Findings)
	}
}

func TestReconcileSkipsWithoutBaselineOrWhenReviewersCollapsed(t *testing.T) {
	baseline := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published"}}
	for name, setup := range map[string]func(st *PipelineState){
		"no baseline": func(st *PipelineState) {
			st.setImportedGroup("imported", agentResult{resp: &llm.ReviewResponse{}}, groupProvenance{})
		},
		"collapsed": func(st *PipelineState) {
			st.setImportedGroup("imported", agentResult{resp: &llm.ReviewResponse{}}, groupProvenance{Baseline: baseline})
			st.setGroup("security", agentResult{run: model.AgentRun{Role: "review", Status: model.AgentRunStatusFailed}}, nil)
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
			st, ctx := loggedState(&model.ReviewContext{}, nil)
			setup(st)
			st.result = &model.ReviewResult{}
			if err := e.reconcileStepFunc("imported")(ctx, e.stepContext(nil, model.ReviewRequest{}), st); err != nil {
				t.Fatal(err)
			}
			if st.reconciled != nil {
				t.Fatal("reconcile planned without a usable baseline")
			}
			// A collapsed re-review must not be published at all: posting it
			// would add a fresh review next to the published one.
			res := (&Pipeline{engine: e}).assemble(st, model.ReviewRequest{})
			if blocked := res.PublishBlocked != ""; blocked != (name == "collapsed") {
				t.Fatalf("PublishBlocked = %q", res.PublishBlocked)
			}
		})
	}
}

// The final diff-scope safeguard honours an imported group's exemption even
// without a reconcile step, for the imported finding itself and for a finding
// merge folded one into.
func TestFinalDiffScopeHonorsImportExemptionWithoutReconcile(t *testing.T) {
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	reviewCtx := &model.ReviewContext{
		DiffScopeHunks: []model.DiffHunk{{FilePath: "main.go", NewStart: 1, NewLines: 1, Content: "+x"}},
		ChangedFiles:   []model.ChangedFile{{Path: "main.go", Status: model.FileModified}},
	}
	st, _ := loggedState(reviewCtx, nil)
	st.findings.exemptFromDiffScope([]string{"imported", "absorbed"})
	st.findings.recordIDs("absorber", []string{"absorbed"})
	st.result = &model.ReviewResult{Findings: []model.Finding{
		reconcileTestFinding("imported", "far.go", "Imported finding outside the diff"),
		reconcileTestFinding("absorber", "far.go", "Fresh finding that absorbed an imported one"),
		reconcileTestFinding("fresh", "far.go", "Fresh finding outside the diff"),
	}}
	res := (&Pipeline{engine: e}).assemble(st, model.ReviewRequest{})
	var ids []string
	for _, f := range res.Findings {
		ids = append(ids, f.ID)
	}
	if strings.Join(ids, ",") != "imported,absorber" {
		t.Fatalf("kept = %v, want the imported finding and its absorber", ids)
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
		"published, publishing":     {&publishedSource{published: published}, model.ReviewRequest{PostReview: true}, importConfig{group: "imported", source: workflow.ImportSourcePublishedReview}, 1, true},
		"published, not publishing": {&publishedSource{published: published}, model.ReviewRequest{}, importConfig{group: "imported", source: workflow.ImportSourcePublishedReview}, 0, false},
		"published, first review":   {&publishedSource{}, model.ReviewRequest{PostReview: true}, importConfig{group: "imported", source: workflow.ImportSourcePublishedReview}, 0, false},
		"published, cannot read":    {stubSource{}, model.ReviewRequest{PostReview: true}, importConfig{group: "imported", source: workflow.ImportSourcePublishedReview}, 0, false},
		"file":                      {stubSource{}, model.ReviewRequest{}, importConfig{group: "scanner", source: workflow.ImportSourceFile, findingsFrom: []string{file}, note: "Static scanner."}, 1, false},
	} {
		t.Run(name, func(t *testing.T) {
			e := NewEngine(tc.source, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
			st, ctx := loggedState(&model.ReviewContext{}, nil)
			entry := workflow.StepEntry{Type: workflow.StepImportFindings, FindingsFrom: tc.cfg.findingsFrom,
				Config: &workflow.StepOverride{Group: &tc.cfg.group, Source: &tc.cfg.source, Note: &tc.cfg.note}}
			if err := e.importFindingsStepFunc(entry)(ctx, e.stepContext(nil, tc.req), st); err != nil {
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
			if tc.cfg.source == workflow.ImportSourcePublishedReview && tc.want > 0 && !st.findings.isExemptFromDiffScope("open") {
				t.Fatal("imported findings of an exempt source must be exempt from the final diff-scope safeguard")
			}
			if tc.cfg.source == workflow.ImportSourcePublishedReview {
				if !prov.ExemptDiffScope || !prov.PreferInMerge || !prov.SkipDedupe || !strings.Contains(prov.Note, "might be outdated") {
					t.Fatalf("published provenance = %+v", prov)
				}
				if tc.want > 0 && !st.findings.isPreferred("open") {
					t.Fatal("imported findings must be preferred in merge")
				}
			}
			if tc.cfg.source == workflow.ImportSourceFile && (prov.ExemptDiffScope || prov.PreferInMerge || prov.SkipDedupe || prov.Note != "Static scanner.") {
				t.Fatalf("file provenance = %+v", prov)
			}
		})
	}
}

// verify:<group> applies an imported group's provenance: its note reaches the
// verifier and an exempt group skips the diff-scope filter.
func TestVerifyImportedGroupAppliesProvenance(t *testing.T) {
	for name, exempt := range map[string]bool{"exempt": true, "in scope only": false} {
		t.Run(name, func(t *testing.T) {
			client := &scriptedVerifyLLM{}
			e := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
			reviewCtx := sampleReviewCtx()
			reviewCtx.DiffScopeHunks = []model.DiffHunk{{FilePath: "main.go", NewStart: 1, NewLines: 1, Content: "+x"}}
			st, ctx := loggedState(reviewCtx, nil)
			outside := reconcileTestFinding("00000000-0000-4000-8000-000000000003", "other.go", "Missing guard far from the diff")
			outside.CodeLocation.LineRange = model.LineRange{Start: 50, End: 50}
			st.setImportedGroup("imported", agentResult{
				resp: &llm.ReviewResponse{Findings: []model.Finding{outside}},
				run:  model.AgentRun{Name: "Imported", Role: "import"},
			}, groupProvenance{Note: "might be outdated", ExemptDiffScope: exempt})
			req := model.ReviewRequest{VerifyDropPolicy: model.DropPolicyRefutedOnly, DisableWorkflowTimeBudget: true}
			if err := e.verifyVectorStepFunc("imported")(ctx, e.stepContext(nil, req), st); err != nil {
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
		})
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
	st, ctx := loggedState(reviewCtx, []string{"security"})
	reviewerOutside := reconcileTestFinding("00000000-0000-4000-8000-000000000011", "other.go", "Reviewer finding outside the diff")
	reviewerOutside.CodeLocation.LineRange = model.LineRange{Start: 70, End: 70}
	st.setGroup("security", agentResult{resp: &llm.ReviewResponse{Findings: []model.Finding{reviewerOutside}}, run: model.AgentRun{Name: "Security", Role: "review"}}, nil)
	id := "00000000-0000-4000-8000-000000000012"
	importedOutside := reconcileTestFinding(id, "other.go", "Published finding outside the diff")
	importedOutside.CodeLocation.LineRange = model.LineRange{Start: 50, End: 50}
	st.setImportedGroup("imported", agentResult{resp: &llm.ReviewResponse{Findings: []model.Finding{importedOutside}}, run: model.AgentRun{Name: "Imported", Role: "import"}},
		groupProvenance{Note: "might be outdated", ExemptDiffScope: true})
	req := model.ReviewRequest{VerifyDropPolicy: model.DropPolicyRefutedOnly, DisableWorkflowTimeBudget: true}
	if err := e.verifyStepFunc(nil)(ctx, e.stepContext(nil, req), st); err != nil {
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
	if _, reason := st.findings.fate(id); reason != "Gone." {
		t.Fatalf("bare verify did not record the refutation: %q", reason)
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

func TestFindingLogFollowsChains(t *testing.T) {
	var none *findingLog
	none.recordIDs("x", []string{"y"})
	none.remove("y", "gone")
	if survivor, reason := none.fate("y"); survivor != "" || reason != "" {
		t.Fatal("nil log must record nothing")
	}
	l := &findingLog{}
	l.recordIDs("b", []string{"a", "b", ""})
	l.recordIDs("c", []string{"b"})
	if got := l.survivor("a"); got != "c" {
		t.Fatalf("survivor(a) = %q, want c", got)
	}
	l.remove("c", "Dropped.")
	l.remove("c", "Second reason ignored.")
	if survivor, reason := l.fate("a"); survivor != "c" || reason != "Dropped." {
		t.Fatalf("fate(a) = %q, %q", survivor, reason)
	}
}

func TestMechanicalDedupeRecordsAbsorptions(t *testing.T) {
	l := &findingLog{}
	a := reconcileTestFinding("a", "a.go", "Missing nil check on user lookup")
	b := reconcileTestFinding("b", "a.go", "Missing nil check on user lookup")
	b.ConfidenceScore = 0.9
	out, folded := mechanicallyDedupeFindings(withFindingLog(context.Background(), l), []model.Finding{a, b})
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

func TestDefaultSpecBinds(t *testing.T) {
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	if _, err := e.BuildPipeline(workflow.DefaultSpec()); err != nil {
		t.Fatalf("default spec does not bind: %v", err)
	}
}

// A step's own note replaces its source's default note; the source's other
// defaults stay.
func TestImportFindingsStepNoteReplacesSourceDefault(t *testing.T) {
	published := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published", Findings: []model.Finding{reconcileTestFinding("open", "a.go", "Open")}}}
	e := NewEngine(&publishedSource{published: published}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	st, ctx := loggedState(&model.ReviewContext{}, nil)
	group, source, note := "imported", workflow.ImportSourcePublishedReview, "Carried over from the last review."
	entry := workflow.StepEntry{Type: workflow.StepImportFindings, Config: &workflow.StepOverride{Group: &group, Source: &source, Note: &note}}
	if err := e.importFindingsStepFunc(entry)(ctx, e.stepContext(nil, model.ReviewRequest{PostReview: true}), st); err != nil {
		t.Fatal(err)
	}
	prov := st.groupProvenance(group)
	if prov == nil || prov.Note != note || !prov.ExemptDiffScope || !prov.PreferInMerge || !prov.SkipDedupe {
		t.Fatalf("provenance = %+v, want the step note with the source's other defaults", prov)
	}
}
