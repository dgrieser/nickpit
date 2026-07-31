package review

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestCategorizeProgressPrintsAssignedCategoryAndDrop(t *testing.T) {
	client := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{
		categorized(model.CategoryConfirmation),
	}}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	var progress bytes.Buffer
	logger := logging.New(&progress, false, false)
	logger.SetShowProgress(true)
	engine.SetLogger(logger)

	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: []model.Finding{
			sampleFinding("code is correct", "main.go", 1),
		}},
		run: model.AgentRun{Name: "Code Quality", Role: "review", Status: model.AgentRunStatusOK},
	}}
	_, warnings, err := engine.categorizeAndFilterVectorFindings(
		context.Background(),
		sampleReviewCtx(),
		vectorResults,
		model.ReviewRequest{VerifyDropPolicy: model.DropPolicyRefutedOnly},
		NewLimiter(1),
		"Code Quality",
	)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}

	got := progress.String()
	for _, want := range []string{
		"ok categories=confirmation",
		"skip dropped categories=confirmation reason=refuted policy=refuted-only",
		"code is correct",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("progress missing %q:\n%s", want, got)
		}
	}
}

func TestVerifyProgressPrintsVerdictDrop(t *testing.T) {
	client := &scriptedVerifyLLM{responses: []*llm.ReviewResponse{{
		Verification: &model.FindingVerification{
			Verdict:         model.VerdictRefuted,
			Priority:        2,
			ConfidenceScore: 0.9,
			Remarks:         "not reproducible",
		},
	}}}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	var progress bytes.Buffer
	logger := logging.New(&progress, false, false)
	logger.SetShowProgress(true)
	engine.SetLogger(logger)

	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: []model.Finding{
			sampleFinding("claimed bug", "main.go", 1),
		}},
		run: model.AgentRun{Name: "Security", Role: "review", Status: model.AgentRunStatusOK},
	}}
	_, warnings, err := engine.verifyAndFilterVectorFindings(
		context.Background(),
		sampleReviewCtx(),
		vectorResults,
		model.ReviewRequest{VerifyDropPolicy: model.DropPolicyRefutedOnly},
		NewLimiter(1),
		"Security",
		internalAgentContext{},
	)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if got := progress.String(); !strings.Contains(got, "skip dropped verdict=refuted policy=refuted-only") {
		t.Fatalf("progress missing verifier drop:\n%s", got)
	}
}

func TestDiffScopeProgressPrintsEachDroppedFinding(t *testing.T) {
	reviewCtx := &model.ReviewContext{DiffScopeHunks: []model.DiffHunk{{
		FilePath: "main.go",
		OldStart: 10,
		OldLines: 1,
		NewStart: 10,
		NewLines: 1,
		Content:  " changed()\n",
	}}}
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: []model.Finding{
			sampleFinding("outside patch", "other.go", 1),
		}},
		run: model.AgentRun{Name: "Architecture", Role: "review", Status: model.AgentRunStatusOK},
	}}
	engine := NewEngine(nil, nil, nil, config.Profile{Model: "test"})
	var progress bytes.Buffer
	logger := logging.New(&progress, false, false)
	logger.SetShowProgress(true)
	engine.SetLogger(logger)

	warnings := engine.prepareFindingsForVerification(context.Background(), reviewCtx, vectorResults, model.ReviewRequest{})
	if len(warnings) != 1 {
		t.Fatalf("warnings=%v", warnings)
	}
	got := progress.String()
	for _, want := range []string{"outside patch", "skip dropped reason=out-of-diff"} {
		if !strings.Contains(got, want) {
			t.Errorf("progress missing %q:\n%s", want, got)
		}
	}
}
