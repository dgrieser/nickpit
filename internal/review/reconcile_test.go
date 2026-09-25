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

func TestReconcilePublishedMapsFindingsOntoPublishedReview(t *testing.T) {
	closed := reconcileTestFinding("closed", "c.go", "Leaked file handle")
	closed.Resolution = &model.FindingResolution{Reason: "Fixed."}
	prior := &model.ReviewResult{ReviewID: "published", Revision: 3, Findings: []model.Finding{
		reconcileTestFinding("kept", "a.go", "Missing nil check on user lookup"),
		reconcileTestFinding("changed", "b.go", "Unbounded retry loop"),
		reconcileTestFinding("refuted", "d.go", "Race on cache map"),
		reconcileTestFinding("folded", "e.go", "SQL built from user input"),
		reconcileTestFinding("dropped", "f.go", "Token logged in plain text"),
		closed,
		reconcileTestFinding("renamed", "i.go", "Stale cache entry after logout"),
	}}
	published := &model.PublishedReview{Review: prior, Foreign: []model.Finding{{ID: "legacy", Title: "Old legacy thread", CodeLocation: model.CodeLocation{FilePath: "g.go"}}}}

	kept := prior.Findings[0]
	kept.ConfidenceScore = 0.95 // only provenance moved
	changed := prior.Findings[1]
	changed.Body = "The retry loop never stops when the server keeps failing."
	folded := reconcileTestFinding("merge-survivor", "e.go", "SQL built from user input")
	folded.Body = "Merged wording."
	res := &model.ReviewResult{OverallExplanation: "Run verdict.", Findings: []model.Finding{
		kept, changed, folded,
		reconcileTestFinding("dup-of-kept", "a.go", "Missing nil check on user lookup"),
		reconcileTestFinding("dup-of-foreign", "g.go", "Old legacy thread"),
		reconcileTestFinding("new", "h.go", "Division by zero on empty input"),
		reconcileTestFinding("merge-renamed", "i.go", "Session invalidation leaves user data readable"),
	}}
	absorbed := &absorptionLog{}
	absorbed.recordIDs("merge-renamed", []string{"renamed"})
	rec := reconcilePublished(res, published, map[string]string{"refuted": "The cache map is now guarded by a mutex. It was added in the last commit."}, absorbed, "head123")

	if res.ReviewID != "published" || res.Revision != 3 {
		t.Fatalf("identity = %s rev %d", res.ReviewID, res.Revision)
	}
	byID := map[string]model.Finding{}
	var order []string
	for _, f := range res.Findings {
		byID[f.ID] = f
		order = append(order, f.ID)
	}
	if strings.Join(order, ",") != "kept,changed,refuted,folded,dropped,closed,renamed,new,merge-renamed" {
		t.Fatalf("order = %v", order)
	}
	if byID["kept"].ConfidenceScore != 0.8 {
		t.Fatal("provenance-only change rewrote the published finding")
	}
	if byID["changed"].Body != changed.Body || byID["folded"].Body != "Merged wording." {
		t.Fatalf("updated findings = %+v / %+v", byID["changed"], byID["folded"])
	}
	if r := byID["refuted"].Resolution; r == nil || r.Reason != "The cache map is now guarded by a mutex." {
		t.Fatalf("refuted resolution = %+v", r)
	}
	if byID["dropped"].Resolution != nil || byID["dropped"].Body != prior.Findings[4].Body {
		t.Fatal("a finding dropped without refutation must stay as published")
	}
	if r := byID["renamed"].Resolution; r == nil || r.Reason != "Merged into the finding \u201cSession invalidation leaves user data readable\u201d of this re-review." {
		t.Fatalf("merged-away resolution = %+v", r)
	}
	if strings.Join(rec.Added, ",") != "new,merge-renamed" || strings.Join(rec.Resolved, ",") != "refuted,renamed" || strings.Join(rec.Updated, ",") != "changed,folded" {
		t.Fatalf("reconciliation = %+v", rec)
	}
	if rec.Before != prior || rec.HeadSHA != "head123" {
		t.Fatalf("reconciliation header = %+v", rec)
	}
}

func TestLoadPublishedStepFillsGroupWithOpenFindings(t *testing.T) {
	closed := reconcileTestFinding("closed", "c.go", "Closed")
	closed.Resolution = &model.FindingResolution{Reason: "Fixed."}
	published := &model.PublishedReview{Review: &model.ReviewResult{ReviewID: "published", Findings: []model.Finding{
		reconcileTestFinding("open", "a.go", "Open"), closed,
	}}}
	for name, tc := range map[string]struct {
		source model.ReviewSource
		req    model.ReviewRequest
		want   int
	}{
		"publishing":         {&publishedSource{published: published}, model.ReviewRequest{PostReview: true}, 1},
		"not publishing":     {&publishedSource{published: published}, model.ReviewRequest{}, 0},
		"first review":       {&publishedSource{}, model.ReviewRequest{PostReview: true}, 0},
		"source cannot read": {stubSource{}, model.ReviewRequest{PostReview: true}, 0},
	} {
		t.Run(name, func(t *testing.T) {
			e := NewEngine(tc.source, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
			st := newPipelineState(&model.ReviewContext{}, nil)
			if err := e.loadPublishedStepFunc()(context.Background(), e.stepContext(nil, tc.req), st); err != nil {
				t.Fatal(err)
			}
			vr, ok := st.vectorResult(workflow.PublishedGroupID)
			if !ok || len(vr.resp.Findings) != tc.want {
				t.Fatalf("group = %+v, %v", vr.resp, ok)
			}
			if (st.published != nil) != (tc.want > 0) {
				t.Fatalf("published state = %+v", st.published)
			}
		})
	}
}

func TestVerifyPublishedStepNotesOutdatedAndRecordsRefutations(t *testing.T) {
	client := &scriptedVerifyLLM{responses: []*llm.ReviewResponse{
		{Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 2, ConfidenceScore: 0.9, Remarks: "The guard exists now."}},
	}}
	e := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	st := newPipelineState(sampleReviewCtx(), nil)
	st.setGroup(workflow.PublishedGroupID, agentResult{
		resp: &llm.ReviewResponse{Findings: []model.Finding{reconcileTestFinding("00000000-0000-4000-8000-000000000001", "main.go", "Missing guard")}},
		run:  model.AgentRun{Name: "Published Findings", Role: "published"},
	}, nil)
	req := model.ReviewRequest{VerifyDropPolicy: model.DropPolicyRefutedOnly, DisableWorkflowTimeBudget: true}
	if err := e.verifyPublishedStepFunc()(context.Background(), e.stepContext(nil, req), st); err != nil {
		t.Fatal(err)
	}
	if st.publishedRefuted["00000000-0000-4000-8000-000000000001"] != "The guard exists now." {
		t.Fatalf("refuted = %v", st.publishedRefuted)
	}
	vr, _ := st.vectorResult(workflow.PublishedGroupID)
	if len(vr.resp.Findings) != 0 {
		t.Fatal("refuted published finding stayed in its group")
	}
	var system, user string
	for _, r := range client.requests {
		if r.SchemaKind == llm.SchemaKindVerify {
			system, user = r.Messages[0].Content, r.Messages[len(r.Messages)-1].Content
		}
	}
	if !strings.Contains(system, "NOTE ON THIS FINDING: an earlier review of this change published it") {
		t.Fatalf("verifier system prompt lacks the outdated note:\n%s", system)
	}
	if strings.Contains(user, "note") {
		t.Fatal("note leaked into the finding payload")
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

func TestSplitPublishedFindings(t *testing.T) {
	rec := &model.Reconciliation{Before: &model.ReviewResult{Findings: []model.Finding{{ID: "p"}}}}
	published, fresh := splitPublishedFindings([]model.Finding{{ID: "p"}, {ID: "n"}}, rec)
	if len(published) != 1 || len(fresh) != 1 || fresh[0].ID != "n" {
		t.Fatalf("split = %v / %v", published, fresh)
	}
	if published, fresh := splitPublishedFindings([]model.Finding{{ID: "n"}}, nil); published != nil || len(fresh) != 1 {
		t.Fatal("no reconciliation must keep every finding fresh")
	}
}

func TestDefaultSpecBindsPublishedLane(t *testing.T) {
	e := NewEngine(stubSource{}, &updateTestLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	if _, err := e.BuildPipeline(workflow.DefaultSpec()); err != nil {
		t.Fatalf("default spec does not bind: %v", err)
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
	if _, _, err := e.verifyAndFilterVectorFindings(context.Background(), sampleReviewCtx(), vectorResults, model.ReviewRequest{}, NewLimiter(1), "", "", internalAgentContext{}, disabledVerifyPhaseBudgets(context.Background())); err != nil {
		t.Fatal(err)
	}
	for _, r := range client.requests {
		if r.SchemaKind == llm.SchemaKindVerify && strings.Contains(r.Messages[0].Content, "NOTE ON THIS FINDING") {
			t.Fatalf("note section rendered without a note:\n%s", r.Messages[0].Content)
		}
	}
}
