package review

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestFinalizePromptIncludesInlineFinalizeSchema(t *testing.T) {
	const findingID = "11111111-1111-4111-8111-111111111111"
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title:           "Fix issue",
						Body:            "body",
						ConfidenceScore: 0.7,
						Priority:        intPtr(1),
						CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
						Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
						Finalization:    &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 1, ConfidenceScore: 0.75, Remarks: "keep"},
					},
				},
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "ok",
				OverallConfidenceScore: 0.7,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				ID:              findingID,
				Title:           "Fix issue",
				Body:            "body",
				ConfidenceScore: 0.7,
				Priority:        intPtr(1),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
			},
		},
		OverallCorrectness:     "patch is correct",
		OverallExplanation:     "ok",
		OverallConfidenceScore: 0.7,
	}

	_, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(llmClient.reqs))
	}
	req := llmClient.reqs[0]
	if req.SchemaKind != llm.SchemaKindFinalize {
		t.Fatalf("schema kind = %v, want finalize", req.SchemaKind)
	}
	systemPrompt := req.Messages[0].Content
	for _, want := range []string{`"verification"`, `"finalization"`, `"title"`, `"body"`, `"remarks"`} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("finalize system prompt missing %s:\n%s", want, systemPrompt)
		}
	}
	if !strings.Contains(req.Messages[1].Content, `"id": "`+findingID+`"`) {
		t.Fatalf("finalize user prompt missing finding id:\n%s", req.Messages[1].Content)
	}
}

func TestExampleSnippetForFinalizeIncludesFinalization(t *testing.T) {
	snippet := exampleSnippetFor(llm.SchemaKindFinalize)
	if !strings.Contains(snippet, `"finalization"`) {
		t.Fatalf("finalize retry example missing finalization: %s", snippet)
	}
}

func TestFinalizePreservesInputSuggestionsWhenLLMDropsThem(t *testing.T) {
	inputSuggestions := []model.Suggestion{
		{Body: "use bufio.Scanner", LineRange: model.LineRange{Start: 10, End: 12}},
		{Body: "log error before return", LineRange: model.LineRange{Start: 14, End: 14}},
	}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title:           "Fix issue",
						Body:            "body",
						ConfidenceScore: 0.7,
						Priority:        intPtr(1),
						CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
						Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
						Finalization:    &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 1, ConfidenceScore: 0.75, Remarks: "keep"},
						// no Suggestions echoed back by the model
					},
				},
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "ok",
				OverallConfidenceScore: 0.7,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				Title:           "Fix issue",
				Body:            "body",
				ConfidenceScore: 0.7,
				Priority:        intPtr(1),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Suggestions:     inputSuggestions,
				Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
			},
		},
		OverallCorrectness:     "patch is correct",
		OverallExplanation:     "ok",
		OverallConfidenceScore: 0.7,
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	if len(out.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(out.Findings))
	}
	got := out.Findings[0].Suggestions
	if len(got) != len(inputSuggestions) {
		t.Fatalf("suggestions len = %d, want %d (%+v)", len(got), len(inputSuggestions), got)
	}
	for i := range inputSuggestions {
		if got[i] != inputSuggestions[i] {
			t.Fatalf("suggestions[%d] = %+v, want %+v", i, got[i], inputSuggestions[i])
		}
	}
}

func TestFinalizePreservesInputVerificationWhenLLMDropsIt(t *testing.T) {
	const findingID = "11111111-1111-4111-8111-111111111111"
	inputVerification := &model.FindingVerification{ID: findingID, Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title:           "Fix issue",
						Body:            "body",
						ConfidenceScore: 0.7,
						Priority:        intPtr(1),
						CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
						Finalization:    &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 1, ConfidenceScore: 0.75, Remarks: "keep"},
						// no Verification echoed back by the model
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				ID:              findingID,
				Title:           "Fix issue",
				Body:            "body",
				ConfidenceScore: 0.7,
				Priority:        intPtr(1),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification:    inputVerification,
			},
		},
		OverallCorrectness: "patch is correct",
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	got := out.Findings[0].Verification
	if got == nil {
		t.Fatalf("verification was dropped; want it restored from input")
	}
	if *got != *inputVerification {
		t.Fatalf("verification = %+v, want %+v", *got, *inputVerification)
	}
}

func TestFinalizeKeepsLLMVerificationWhenProvided(t *testing.T) {
	llmVerification := &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.9, Remarks: "refined"}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title:           "Fix issue",
						Body:            "body",
						ConfidenceScore: 0.7,
						Priority:        intPtr(1),
						CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
						Verification:    llmVerification,
						Finalization:    &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 2, ConfidenceScore: 0.85, Remarks: "ok"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				Title:           "Fix issue",
				Body:            "body",
				ConfidenceScore: 0.7,
				Priority:        intPtr(1),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.5, Remarks: "stale"},
			},
		},
		OverallCorrectness: "patch is correct",
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	got := out.Findings[0].Verification
	if got == nil || *got != *llmVerification {
		t.Fatalf("verification = %+v, want %+v (LLM output must win when present)", got, llmVerification)
	}
}

func TestFinalizeKeepsLLMSuggestionsWhenProvided(t *testing.T) {
	llmSuggestions := []model.Suggestion{
		{Body: "refined fix", LineRange: model.LineRange{Start: 20, End: 21}},
	}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title:           "Fix issue",
						Body:            "body",
						ConfidenceScore: 0.7,
						Priority:        intPtr(1),
						CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
						Suggestions:     llmSuggestions,
						Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
						Finalization:    &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 1, ConfidenceScore: 0.75, Remarks: "refined"},
					},
				},
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "ok",
				OverallConfidenceScore: 0.7,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				Title:           "Fix issue",
				Body:            "body",
				ConfidenceScore: 0.7,
				Priority:        intPtr(1),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Suggestions:     []model.Suggestion{{Body: "stale", LineRange: model.LineRange{Start: 99, End: 99}}},
				Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
			},
		},
		OverallCorrectness: "patch is correct",
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	got := out.Findings[0].Suggestions
	if len(got) != len(llmSuggestions) || got[0] != llmSuggestions[0] {
		t.Fatalf("suggestions = %+v, want %+v (LLM output must win when present)", got, llmSuggestions)
	}
}

func TestFinalizeMergesSuggestionsWithCollidingLocationByTitle(t *testing.T) {
	loc := model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 5, End: 5}}
	suggA := []model.Suggestion{{Body: "fix A", LineRange: model.LineRange{Start: 5, End: 5}}}
	suggB := []model.Suggestion{{Body: "fix B", LineRange: model.LineRange{Start: 5, End: 5}}}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title:           "Issue B",
						Body:            "b",
						ConfidenceScore: 0.6,
						Priority:        intPtr(2),
						CodeLocation:    loc,
						Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"},
						Finalization:    &model.FindingFinalization{Title: "Issue B", Body: "b", Priority: 2, ConfidenceScore: 0.6, Remarks: "keep"},
					},
					{
						Title:           "Issue A",
						Body:            "a",
						ConfidenceScore: 0.6,
						Priority:        intPtr(2),
						CodeLocation:    loc,
						Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"},
						Finalization:    &model.FindingFinalization{Title: "Issue A", Body: "a", Priority: 2, ConfidenceScore: 0.6, Remarks: "keep"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: loc, Suggestions: suggA, Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"}},
			{Title: "Issue B", Body: "b", Priority: intPtr(2), CodeLocation: loc, Suggestions: suggB, Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"}},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	if out.Findings[0].Title != "Issue B" || out.Findings[0].Suggestions[0].Body != "fix B" {
		t.Fatalf("findings[0] = %+v, want title=Issue B suggestion=fix B", out.Findings[0])
	}
	if out.Findings[1].Title != "Issue A" || out.Findings[1].Suggestions[0].Body != "fix A" {
		t.Fatalf("findings[1] = %+v, want title=Issue A suggestion=fix A", out.Findings[1])
	}
}

func TestFinalizePriorityFloorSurvivesReorder(t *testing.T) {
	locA := model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}
	locB := model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}
	// LLM reorders B before A AND tries to escalate B from P2 -> P0.
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title: "Issue B", Body: "b", Priority: intPtr(2), CodeLocation: locB,
						Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"},
						Finalization: &model.FindingFinalization{Title: "Issue B", Body: "b", Priority: 0, ConfidenceScore: 0.6, Remarks: "escalate"},
					},
					{
						Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA,
						Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"},
						Finalization: &model.FindingFinalization{Title: "Issue A", Body: "a", Priority: 2, ConfidenceScore: 0.6, Remarks: "keep"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA, Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"}},
			{Title: "Issue B", Body: "b", Priority: intPtr(2), CodeLocation: locB, Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"}},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	// Index 0 of out is Issue B (LLM reordered). Index-based matching would have
	// taken Issue A's floor. With code_location matching, floor for B is P2.
	if out.Findings[0].Title != "Issue B" || out.Findings[0].Finalization.Priority != 2 {
		t.Fatalf("findings[0] = %+v, want title=Issue B priority=2 (clamped)", out.Findings[0])
	}
	if out.Findings[1].Title != "Issue A" || out.Findings[1].Finalization.Priority != 2 {
		t.Fatalf("findings[1] = %+v, want title=Issue A priority=2", out.Findings[1])
	}
}

func TestFinalizeDropsHallucinatedFindingsWithoutInputMatch(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title: "Hallucinated", Body: "x", Priority: intPtr(0), CodeLocation: model.CodeLocation{FilePath: "ghost.go", LineRange: model.LineRange{Start: 9, End: 9}},
						Finalization: &model.FindingFinalization{Title: "Hallucinated", Body: "x", Priority: 0, ConfidenceScore: 0.5, Remarks: "new"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Real", Body: "r", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "real.go", LineRange: model.LineRange{Start: 1, End: 1}}, Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"}},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	// Prompt forbids new findings; in-code defence drops them so arbitrary
	// LLM priority/confidence cannot ship as a real finding.
	if len(out.Findings) != 0 {
		t.Fatalf("findings = %d, want 0 (hallucinated finding dropped)", len(out.Findings))
	}
}

func TestFinalizeWeightedConfidenceStandardAverage(t *testing.T) {
	loc := model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title: "Issue", Body: "b", Priority: intPtr(1), CodeLocation: loc,
						Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "ok"},
						// LLM emits an arbitrary value; code must overwrite.
						Finalization: &model.FindingFinalization{Title: "Issue", Body: "b", Priority: 1, ConfidenceScore: 0.123, Remarks: "keep"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Issue", Body: "b", ConfidenceScore: 0.6, Priority: intPtr(1), CodeLocation: loc,
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "ok"}},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	// 0.6*0.8 + 0.4*0.6 = 0.72, divergence 0.2 < 0.3 → no clamp.
	got := out.Findings[0].Finalization.ConfidenceScore
	if math.Abs(got-0.72) > 1e-9 {
		t.Fatalf("confidence = %v, want 0.72", got)
	}
}

func TestFinalizeWeightedConfidenceRoundsToTwoDecimals(t *testing.T) {
	loc := model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title: "Issue", Body: "b", Priority: intPtr(1), CodeLocation: loc,
						Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.79, Remarks: "ok"},
						Finalization: &model.FindingFinalization{Title: "Issue", Body: "b", Priority: 1, ConfidenceScore: 0.123, Remarks: "keep"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Issue", Body: "b", ConfidenceScore: 0.51, Priority: intPtr(1), CodeLocation: loc,
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.79, Remarks: "ok"}},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	// 0.6*0.79 + 0.4*0.51 = 0.678, divergence 0.28 < 0.3 -> round to 0.68.
	got := out.Findings[0].Finalization.ConfidenceScore
	if math.Abs(got-0.68) > 1e-9 {
		t.Fatalf("confidence = %v, want 0.68", got)
	}
}

func TestFinalizeWeightedConfidenceClampsOnLargeDivergence(t *testing.T) {
	loc := model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title: "Issue", Body: "b", Priority: intPtr(1), CodeLocation: loc,
						Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.9, Remarks: "ok"},
						Finalization: &model.FindingFinalization{Title: "Issue", Body: "b", Priority: 1, ConfidenceScore: 0.9, Remarks: "keep"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Issue", Body: "b", ConfidenceScore: 0.4, Priority: intPtr(1), CodeLocation: loc,
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.9, Remarks: "ok"}},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	// 0.6*0.9 + 0.4*0.4 = 0.70, divergence 0.5 > 0.3, clamp to min(0.9, 0.4) = 0.4.
	got := out.Findings[0].Finalization.ConfidenceScore
	if math.Abs(got-0.4) > 1e-9 {
		t.Fatalf("confidence = %v, want 0.4 (clamped)", got)
	}
}

func TestFinalizeWeightedConfidenceFallsBackWhenNoVerification(t *testing.T) {
	loc := model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title: "Issue", Body: "b", Priority: intPtr(1), CodeLocation: loc,
						Finalization: &model.FindingFinalization{Title: "Issue", Body: "b", Priority: 1, ConfidenceScore: 0.0, Remarks: "keep"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Issue", Body: "b", ConfidenceScore: 0.65, Priority: intPtr(1), CodeLocation: loc},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	got := out.Findings[0].Finalization.ConfidenceScore
	if math.Abs(got-0.65) > 1e-9 {
		t.Fatalf("confidence = %v, want 0.65 (review fallback)", got)
	}
}

func TestFinalizeWeightedConfidenceSurvivesReorder(t *testing.T) {
	locA := model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}
	locB := model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				// LLM reorders B before A.
				Findings: []model.Finding{
					{
						Title: "Issue B", Body: "b", Priority: intPtr(2), CodeLocation: locB,
						Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.5, Remarks: "ok"},
						Finalization: &model.FindingFinalization{Title: "Issue B", Body: "b", Priority: 2, ConfidenceScore: 0.99, Remarks: "x"},
					},
					{
						Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA,
						Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.9, Remarks: "ok"},
						Finalization: &model.FindingFinalization{Title: "Issue A", Body: "a", Priority: 2, ConfidenceScore: 0.99, Remarks: "x"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Issue A", Body: "a", ConfidenceScore: 0.8, Priority: intPtr(2), CodeLocation: locA,
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.9, Remarks: "ok"}},
			{Title: "Issue B", Body: "b", ConfidenceScore: 0.4, Priority: intPtr(2), CodeLocation: locB,
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.5, Remarks: "ok"}},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	// Index 0 = Issue B: 0.6*0.5 + 0.4*0.4 = 0.46, divergence 0.1 < 0.3 → 0.46.
	gotB := out.Findings[0].Finalization.ConfidenceScore
	if math.Abs(gotB-0.46) > 1e-9 {
		t.Fatalf("B confidence = %v, want 0.46", gotB)
	}
	// Index 1 = Issue A: 0.6*0.9 + 0.4*0.8 = 0.86, divergence 0.1 < 0.3 → 0.86.
	gotA := out.Findings[1].Finalization.ConfidenceScore
	if math.Abs(gotA-0.86) > 1e-9 {
		t.Fatalf("A confidence = %v, want 0.86", gotA)
	}
}

func TestFinalizeWeightedConfidenceSkipsWhenNoInputMatch(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						Title: "Hallucinated", Body: "x", Priority: intPtr(1),
						CodeLocation: model.CodeLocation{FilePath: "ghost.go", LineRange: model.LineRange{Start: 9, End: 9}},
						Finalization: &model.FindingFinalization{Title: "Hallucinated", Body: "x", Priority: 1, ConfidenceScore: 0.42, Remarks: "new"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Real", Body: "r", ConfidenceScore: 0.7, Priority: intPtr(2),
				CodeLocation: model.CodeLocation{FilePath: "real.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"}},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	// Hallucinated finding has no input match → dropped, real finding has no
	// finalization (because the LLM only returned the hallucination) → also dropped.
	if len(out.Findings) != 0 {
		t.Fatalf("findings = %d, want 0 (hallucinated dropped)", len(out.Findings))
	}
}

func TestDropUnmatchedFindingsRestampsSwappedIDLocation(t *testing.T) {
	const idA = "11111111-1111-4111-8111-111111111111"
	const idB = "22222222-2222-4222-8222-222222222222"
	locA := model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}
	locB := model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}
	in := []model.Finding{
		{ID: idA, Title: "Issue A", CodeLocation: locA},
		{ID: idB, Title: "Issue B", CodeLocation: locB},
	}
	out := []model.Finding{
		{ID: idA, Title: "Issue B", CodeLocation: locB},
	}

	got := dropUnmatchedFindings(out, in)
	if len(got) != 1 {
		t.Fatalf("findings = %d, want 1 location match", len(got))
	}
	if got[0].ID != idB {
		t.Fatalf("id = %q, want restamped to location match %q", got[0].ID, idB)
	}
	if got[0].CodeLocation != locB {
		t.Fatalf("location = %#v, want %#v", got[0].CodeLocation, locB)
	}
}

// Regression: with no input findings, Finalize must short-circuit before any
// LLM call. Skips network, schema work, and the rest of the pipeline.
func TestFinalizeEarlySkipsOnEmptyFindings(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings:           nil,
		OverallCorrectness: "patch is correct",
	}

	out, run, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	if len(llmClient.reqs) != 0 {
		t.Fatalf("LLM requests = %d, want 0 (no findings → no LLM call)", len(llmClient.reqs))
	}
	if out == nil || out.OverallCorrectness != "patch is correct" {
		t.Fatalf("out = %+v, want input cloned through unchanged", out)
	}
	if run.Name != "Finalize Review" {
		t.Fatalf("run.Name = %q, want Finalize Review", run.Name)
	}
}

// Regression: when all input findings are P2 / P3, the finalizer must be
// constrained so it cannot flip overall_correctness to "patch is incorrect".
func TestFinalizeRefusesPatchIncorrectWithoutCriticalFindings(t *testing.T) {
	if cs := finalizeConstraintsFor([]model.Finding{
		{Priority: intPtr(2)},
		{Priority: intPtr(3)},
	}).AllowedCorrectness; len(cs) != 1 || cs[0] != "patch is correct" {
		t.Fatalf("constraints = %#v, want [patch is correct]", cs)
	}
	if cs := finalizeConstraintsFor([]model.Finding{
		{Priority: intPtr(2)},
		{Priority: intPtr(1)},
	}).AllowedCorrectness; len(cs) != 0 {
		t.Fatalf("constraints = %#v, want unconstrained (P1 present)", cs)
	}
}

func TestFinalizePreservesInputIDWhenLLMDropsIt(t *testing.T) {
	const findingID = "11111111-1111-4111-8111-111111111111"
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{{
			Findings: []model.Finding{{
				Title:           "Fix issue",
				Body:            "body",
				ConfidenceScore: 0.7,
				Priority:        intPtr(1),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Finalization:    &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 1, ConfidenceScore: 0.75, Remarks: "keep"},
			}},
			OverallCorrectness:     "patch is correct",
			OverallExplanation:     "ok",
			OverallConfidenceScore: 0.7,
		}},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{{
			ID:              findingID,
			Title:           "Fix issue",
			Body:            "body",
			ConfidenceScore: 0.7,
			Priority:        intPtr(1),
			CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
			Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
		}},
		OverallCorrectness:     "patch is correct",
		OverallExplanation:     "ok",
		OverallConfidenceScore: 0.7,
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	if got := out.Findings[0].ID; got != findingID {
		t.Fatalf("id = %q, want %q", got, findingID)
	}
}
