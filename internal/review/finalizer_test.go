package review

import (
	"context"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestFinalizePromptIncludesInlineFinalizeSchema(t *testing.T) {
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
						Verification:    &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
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
				Title:           "Fix issue",
				Body:            "body",
				ConfidenceScore: 0.7,
				Priority:        intPtr(1),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification:    &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
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
						Verification:    &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
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
				Verification:    &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
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
						Verification:    &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
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
				Verification:    &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
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
						Verification:    &model.FindingVerification{Valid: true, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"},
						Finalization:    &model.FindingFinalization{Title: "Issue B", Body: "b", Priority: 2, ConfidenceScore: 0.6, Remarks: "keep"},
					},
					{
						Title:           "Issue A",
						Body:            "a",
						ConfidenceScore: 0.6,
						Priority:        intPtr(2),
						CodeLocation:    loc,
						Verification:    &model.FindingVerification{Valid: true, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"},
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
			{Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: loc, Suggestions: suggA, Verification: &model.FindingVerification{Valid: true, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"}},
			{Title: "Issue B", Body: "b", Priority: intPtr(2), CodeLocation: loc, Suggestions: suggB, Verification: &model.FindingVerification{Valid: true, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"}},
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
						Verification: &model.FindingVerification{Valid: true, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"},
						Finalization: &model.FindingFinalization{Title: "Issue B", Body: "b", Priority: 0, ConfidenceScore: 0.6, Remarks: "escalate"},
					},
					{
						Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA,
						Verification: &model.FindingVerification{Valid: true, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"},
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
			{Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA, Verification: &model.FindingVerification{Valid: true, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"}},
			{Title: "Issue B", Body: "b", Priority: intPtr(2), CodeLocation: locB, Verification: &model.FindingVerification{Valid: true, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"}},
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

func TestFinalizePriorityFloorSkipsWhenNoInputMatch(t *testing.T) {
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
			{Title: "Real", Body: "r", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "real.go", LineRange: model.LineRange{Start: 1, End: 1}}, Verification: &model.FindingVerification{Valid: true, Priority: 2, ConfidenceScore: 0.7, Remarks: "ok"}},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	// No matching input → floor skipped; LLM's P0 stays untouched (the prompt forbids new findings,
	// but defensively we don't apply a wrong floor).
	if out.Findings[0].Finalization.Priority != 0 {
		t.Fatalf("priority = %d, want 0 (unchanged when no input match)", out.Findings[0].Finalization.Priority)
	}
}
