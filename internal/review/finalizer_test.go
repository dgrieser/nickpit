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

func TestVerdictContextNotesInPrompt(t *testing.T) {
	const findingID = "11111111-1111-4111-8111-111111111111"
	const notes = "## Notes\n\nCONTEXT_NOTES_MARKER the patch wires notes into the verdict."
	newClient := func() *capturingLLM {
		return &capturingLLM{
			resps: []*llm.ReviewResponse{
				{
					OverallCorrectness:     "patch is correct",
					OverallExplanation:     "CONTEXT_NOTES_MARKER summary.",
					OverallConfidenceScore: 0.7,
				},
			},
		}
	}
	newInput := func() *model.ReviewResult {
		return &model.ReviewResult{
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
	}

	// With notes: the verdict user prompt carries `notes`.
	withNotes := newClient()
	engine := NewEngine(stubSource{}, withNotes, stubRetrieval{}, config.Profile{Model: "test"})
	if _, _, err := engine.Verdict(context.Background(), sampleReviewCtx(), newInput(), VerdictOptions{ContextNotes: notes}); err != nil {
		t.Fatalf("Verdict (with notes) returned err: %v", err)
	}
	userPrompt := withNotes.reqs[0].Messages[1].Content
	if !strings.Contains(userPrompt, `"notes"`) || !strings.Contains(userPrompt, "CONTEXT_NOTES_MARKER") {
		t.Fatalf("verdict user prompt missing notes:\n%s", userPrompt)
	}
	if !strings.Contains(userPrompt, `"priority_floor": 1`) {
		t.Fatalf("verdict user prompt missing priority_floor:\n%s", userPrompt)
	}
	// The system prompt tasks the model to merge notes into overall_explanation.
	if sys := withNotes.reqs[0].Messages[0].Content; !strings.Contains(sys, "notes") || !strings.Contains(sys, "priority_floor") || !strings.Contains(sys, "even if `finalization.priority` downgraded it") {
		t.Fatalf("verdict system prompt does not mention notes:\n%s", sys)
	}

	// Without notes: the `notes` key is omitted entirely.
	withoutNotes := newClient()
	engine = NewEngine(stubSource{}, withoutNotes, stubRetrieval{}, config.Profile{Model: "test"})
	if _, _, err := engine.Verdict(context.Background(), sampleReviewCtx(), newInput(), VerdictOptions{}); err != nil {
		t.Fatalf("Verdict (no notes) returned err: %v", err)
	}
	if up := withoutNotes.reqs[0].Messages[1].Content; strings.Contains(up, `"notes"`) {
		t.Fatalf("verdict user prompt should omit notes when none provided:\n%s", up)
	}

	// Disabled patch summary: notes remain available as internal context, but
	// the verdict is told not to surface the patch-purpose assumption.
	disabledSummary := newClient()
	engine = NewEngine(stubSource{}, disabledSummary, stubRetrieval{}, config.Profile{Model: "test"})
	if _, _, err := engine.Verdict(context.Background(), sampleReviewCtx(), newInput(), VerdictOptions{ContextNotes: notes, DisablePatchSummary: true}); err != nil {
		t.Fatalf("Verdict (disabled patch summary) returned err: %v", err)
	}
	if up := disabledSummary.reqs[0].Messages[1].Content; !strings.Contains(up, `"notes"`) || !strings.Contains(up, "CONTEXT_NOTES_MARKER") {
		t.Fatalf("verdict user prompt should still carry internal notes:\n%s", up)
	}
	sys := disabledSummary.reqs[0].Messages[0].Content
	if !strings.Contains(sys, "preliminary `overall_correctness`, `overall_explanation`, and `overall_confidence_score`") {
		t.Fatalf("verdict system prompt missing preliminary field description:\n%s", sys)
	}
	if !strings.Contains(sys, "do not include the patch-purpose assumption") {
		t.Fatalf("verdict system prompt missing disabled-summary instruction:\n%s", sys)
	}
	if strings.Contains(sys, "first state the patch's intended purpose") {
		t.Fatalf("verdict system prompt should not require patch purpose when disabled:\n%s", sys)
	}
}

func TestExampleSnippetForFinalizeIncludesFinalization(t *testing.T) {
	snippet := exampleSnippetFor(llm.SchemaKindFinalize, false)
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

func TestFinalizeRestoresInputVerificationAsLastResort(t *testing.T) {
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
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d, want no validator retry for last-resort repair", len(llmClient.reqs))
	}
	got := out.Findings[0].Verification
	if got == nil {
		t.Fatalf("verification was dropped")
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
	if out.Findings[0].Title != "Issue A" || out.Findings[0].Suggestions[0].Body != "fix A" {
		t.Fatalf("findings[0] = %+v, want title=Issue A suggestion=fix A", out.Findings[0])
	}
	if out.Findings[1].Title != "Issue B" || out.Findings[1].Suggestions[0].Body != "fix B" {
		t.Fatalf("findings[1] = %+v, want title=Issue B suggestion=fix B", out.Findings[1])
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
	// Output preserves input order. Matching still uses code_location so B's
	// attempted escalation is clamped against B's own floor.
	if out.Findings[0].Title != "Issue A" || out.Findings[0].Finalization.Priority != 2 {
		t.Fatalf("findings[0] = %+v, want title=Issue A priority=2", out.Findings[0])
	}
	if out.Findings[1].Title != "Issue B" || out.Findings[1].Finalization.Priority != 2 {
		t.Fatalf("findings[1] = %+v, want title=Issue B priority=2 (clamped)", out.Findings[1])
	}
}

func TestFinalizeDropsHallucinatedFindingsWithoutInputMatch(t *testing.T) {
	hallucinated := model.Finding{
		Title: "Hallucinated", Body: "x", Priority: intPtr(0), CodeLocation: model.CodeLocation{FilePath: "ghost.go", LineRange: model.LineRange{Start: 9, End: 9}},
		Finalization: &model.FindingFinalization{Title: "Hallucinated", Body: "x", Priority: 0, ConfidenceScore: 0.5, Remarks: "new"},
	}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings:           []model.Finding{hallucinated},
				OverallCorrectness: "patch is correct",
			},
			{
				Findings:           []model.Finding{hallucinated},
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

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{MaxOutputRetries: 1})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	// Prompt forbids new findings; in-code defence ignores hallucinations
	// without allowing the finalizer to delete real input findings.
	if len(out.Findings) != 1 {
		t.Fatalf("findings = %d, want 1 preserved input finding", len(out.Findings))
	}
	if out.Findings[0].Title != "Real" {
		t.Fatalf("title = %q, want preserved input title", out.Findings[0].Title)
	}
	if out.Findings[0].Finalization == nil {
		t.Fatal("expected synthesized finalization for omitted input finding")
	}
}

func TestFinalizeRetriesWhenFindingCountDiffers(t *testing.T) {
	locA := model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}
	locB := model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{{
					Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA,
					Finalization: &model.FindingFinalization{Title: "Issue A", Body: "a", Priority: 2, Remarks: "first"},
				}},
				OverallCorrectness: "patch is correct",
			},
			{
				Findings: []model.Finding{
					{
						Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA,
						Finalization: &model.FindingFinalization{Title: "Final A", Body: "a", Priority: 2, Remarks: "retry"},
					},
					{
						Title: "Issue B", Body: "b", Priority: intPtr(2), CodeLocation: locB,
						Finalization: &model.FindingFinalization{Title: "Final B", Body: "b", Priority: 2, Remarks: "retry"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA},
			{Title: "Issue B", Body: "b", Priority: intPtr(2), CodeLocation: locB},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{MaxOutputRetries: 1})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d, want retry", len(llmClient.reqs))
	}
	if retryMessage := llmClient.reqs[1].Messages[len(llmClient.reqs[1].Messages)-1].Content; !strings.Contains(retryMessage, "2") || !strings.Contains(retryMessage, "1") {
		t.Fatalf("retry feedback missing count mismatch: %q", llmClient.reqs[1].Messages[len(llmClient.reqs[1].Messages)-1].Content)
	}
	if len(out.Findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(out.Findings))
	}
	if out.Findings[0].Finalization.Title != "Final A" || out.Findings[1].Finalization.Title != "Final B" {
		t.Fatalf("finalizations = %#v %#v, want retry output", out.Findings[0].Finalization, out.Findings[1].Finalization)
	}
}

func TestFinalizeRetriesWhenSameCountOutputMisidentifiesFinding(t *testing.T) {
	const idA = "11111111-1111-4111-8111-111111111111"
	const idB = "22222222-2222-4222-8222-222222222222"
	locA := model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}
	locB := model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{
						ID: idA, Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA,
						Finalization: &model.FindingFinalization{Title: "Final A", Body: "a", Priority: 2, Remarks: "first"},
					},
					{
						ID: idA, Title: "Issue A duplicate", Body: "a", Priority: intPtr(2), CodeLocation: locA,
						Finalization: &model.FindingFinalization{Title: "Duplicate A", Body: "a", Priority: 2, Remarks: "duplicate"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
			{
				Findings: []model.Finding{
					{
						ID: idA, Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA,
						Finalization: &model.FindingFinalization{Title: "Final A", Body: "a", Priority: 2, Remarks: "retry"},
					},
					{
						ID: idB, Title: "Issue B", Body: "b", Priority: intPtr(2), CodeLocation: locB,
						Finalization: &model.FindingFinalization{Title: "Final B", Body: "b", Priority: 2, Remarks: "retry"},
					},
				},
				OverallCorrectness: "patch is correct",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{ID: idA, Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA},
			{ID: idB, Title: "Issue B", Body: "b", Priority: intPtr(2), CodeLocation: locB},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{MaxOutputRetries: 1})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d, want retry", len(llmClient.reqs))
	}
	retryMessage := llmClient.reqs[1].Messages[len(llmClient.reqs[1].Messages)-1].Content
	for _, want := range []string{"matched 1", "omitted 1", "ignored/unmatched item"} {
		if !strings.Contains(retryMessage, want) {
			t.Fatalf("retry feedback missing %q: %q", want, retryMessage)
		}
	}
	if out.Findings[0].Finalization.Title != "Final A" || out.Findings[1].Finalization.Title != "Final B" {
		t.Fatalf("finalizations = %#v %#v, want retry output", out.Findings[0].Finalization, out.Findings[1].Finalization)
	}
}

func TestFinalizePreservesInputsWhenCountRetryExhausted(t *testing.T) {
	locA := model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}
	locB := model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{Findings: nil, OverallCorrectness: "patch is correct"},
			{Findings: nil, OverallCorrectness: "patch is correct"},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Issue A", Body: "a", Priority: intPtr(2), CodeLocation: locA},
			{Title: "Issue B", Body: "b", Priority: intPtr(1), CodeLocation: locB},
		},
	}

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{MaxOutputRetries: 1})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d, want initial + one retry", len(llmClient.reqs))
	}
	if len(out.Findings) != 2 {
		t.Fatalf("findings = %d, want preserved input findings", len(out.Findings))
	}
	for i := range out.Findings {
		if out.Findings[i].Finalization == nil {
			t.Fatalf("findings[%d] missing synthesized finalization", i)
		}
	}
	if len(out.Warnings) == 0 || !strings.Contains(out.Warnings[len(out.Warnings)-1], "Finalizer output mismatch") {
		t.Fatalf("warnings = %#v, want finalizer mismatch warning", out.Warnings)
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

func TestFinalizeWeightedConfidenceZeroesWhenNoVerification(t *testing.T) {
	loc := model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}
	out := []model.Finding{
		{
			Title: "Issue", Body: "b", Priority: intPtr(1), CodeLocation: loc,
			Finalization: &model.FindingFinalization{Title: "Issue", Body: "b", Priority: 1, ConfidenceScore: 0.9, Remarks: "keep"},
		},
	}
	in := []model.Finding{
		{
			Title: "Issue", Body: "b", ConfidenceScore: 0.65, Priority: intPtr(1), CodeLocation: loc,
		},
	}

	// Missing verification => missing confidence => 0.0; the reviewer's own
	// self-assessment is not trusted as a confidence signal.
	applyWeightedConfidence(out, in)
	got := out[0].Finalization.ConfidenceScore
	if math.Abs(got-0.0) > 1e-9 {
		t.Fatalf("confidence = %v, want 0.0 (missing verification)", got)
	}
}

// TestEnforcePriorityFloorDemotesRefutedNonFinding covers the demote: a verifier
// `refuted` finding (an affirmation / non-finding the reviewer mislabeled P0) is
// floored at the configured threshold and its confidence zeroed, so it never
// renders blocking regardless of the reviewer's priority.
func TestEnforcePriorityFloorDemotesRefutedNonFinding(t *testing.T) {
	loc := model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}
	mk := func() ([]model.Finding, []model.Finding) {
		ver := &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 0, ConfidenceScore: 0.0}
		in := []model.Finding{{Title: "No issue", Priority: intPtr(0), ConfidenceScore: 0.7, CodeLocation: loc, Verification: ver}}
		out := []model.Finding{{Title: "No issue", Priority: intPtr(0), CodeLocation: loc, Verification: ver,
			Finalization: &model.FindingFinalization{Priority: 0, ConfidenceScore: 0.9}}}
		return out, in
	}
	// p3: demote to the lowest allowed = 3 (LOW).
	out, in := mk()
	enforcePriorityFloor(out, in, model.PriorityThresholdRank("p3"))
	if got := out[0].Finalization.Priority; got != 3 {
		t.Fatalf("p3 refuted priority = %d, want 3 (demoted)", got)
	}
	// p2: demote to the lowest allowed = 2.
	out, in = mk()
	enforcePriorityFloor(out, in, model.PriorityThresholdRank("p2"))
	if got := out[0].Finalization.Priority; got != 2 {
		t.Fatalf("p2 refuted priority = %d, want 2 (demoted)", got)
	}
	// Confidence zeroed for the refuted non-finding.
	out, in = mk()
	applyWeightedConfidence(out, in)
	if got := out[0].Finalization.ConfidenceScore; got != 0.0 {
		t.Fatalf("refuted confidence = %v, want 0.0", got)
	}
}

// TestEnforcePriorityFloorKeepsConfirmedP0 guards that a genuine confirmed P0 is
// untouched by the demote path and keeps its blended confidence.
func TestEnforcePriorityFloorKeepsConfirmedP0(t *testing.T) {
	loc := model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}
	ver := &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 0, ConfidenceScore: 0.9}
	in := []model.Finding{{Title: "Real bug", Priority: intPtr(0), ConfidenceScore: 0.8, CodeLocation: loc, Verification: ver}}
	out := []model.Finding{{Title: "Real bug", Priority: intPtr(0), CodeLocation: loc, Verification: ver,
		Finalization: &model.FindingFinalization{Priority: 0, ConfidenceScore: 0.0}}}
	enforcePriorityFloor(out, in, model.PriorityThresholdRank("p3"))
	if got := out[0].Finalization.Priority; got != 0 {
		t.Fatalf("confirmed P0 priority = %d, want 0 (untouched)", got)
	}
	applyWeightedConfidence(out, in)
	// 0.6*0.9 + 0.4*0.8 = 0.86 (divergence 0.1 <= 0.3, no clamp).
	if got := out[0].Finalization.ConfidenceScore; math.Abs(got-0.86) > 1e-9 {
		t.Fatalf("confirmed P0 confidence = %v, want 0.86 (blended)", got)
	}
}

// TestEnforcePriorityFloorMissingPriorityDefaultsToThreshold proves a missing
// (nil) priority now defaults to the configured threshold rather than the
// hardcoded P3 floor of model.PriorityRank.
func TestEnforcePriorityFloorMissingPriorityDefaultsToThreshold(t *testing.T) {
	loc := model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}
	in := []model.Finding{{Title: "x", CodeLocation: loc}}
	out := []model.Finding{{Title: "x", CodeLocation: loc, Finalization: &model.FindingFinalization{Priority: 0}}}
	enforcePriorityFloor(out, in, model.PriorityThresholdRank("p2"))
	if got := out[0].Finalization.Priority; got != 2 {
		t.Fatalf("nil-priority floor = %d, want 2 (threshold), not hardcoded 3", got)
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
	// Output preserves input order. Issue A: 0.6*0.9 + 0.4*0.8 = 0.86.
	gotA := out.Findings[0].Finalization.ConfidenceScore
	if math.Abs(gotA-0.86) > 1e-9 {
		t.Fatalf("A confidence = %v, want 0.86", gotA)
	}
	// Issue B: 0.6*0.5 + 0.4*0.4 = 0.46.
	gotB := out.Findings[1].Finalization.ConfidenceScore
	if math.Abs(gotB-0.46) > 1e-9 {
		t.Fatalf("B confidence = %v, want 0.46", gotB)
	}
}

func TestFinalizeWeightedConfidenceSkipsWhenNoInputMatch(t *testing.T) {
	hallucinated := model.Finding{
		Title: "Hallucinated", Body: "x", Priority: intPtr(1),
		CodeLocation: model.CodeLocation{FilePath: "ghost.go", LineRange: model.LineRange{Start: 9, End: 9}},
		Finalization: &model.FindingFinalization{Title: "Hallucinated", Body: "x", Priority: 1, ConfidenceScore: 0.42, Remarks: "new"},
	}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings:           []model.Finding{hallucinated},
				OverallCorrectness: "patch is correct",
			},
			{
				Findings:           []model.Finding{hallucinated},
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

	out, _, err := engine.Finalize(context.Background(), sampleReviewCtx(), in, FinalizeOptions{MaxOutputRetries: 1})
	if err != nil {
		t.Fatalf("Finalize returned err: %v", err)
	}
	// Hallucinated finding has no input match → ignored. Real input finding is
	// preserved and receives synthesized finalization.
	if len(out.Findings) != 1 {
		t.Fatalf("findings = %d, want 1 preserved input finding", len(out.Findings))
	}
	if out.Findings[0].Title != "Real" {
		t.Fatalf("title = %q, want Real", out.Findings[0].Title)
	}
	if out.Findings[0].Finalization == nil {
		t.Fatal("expected synthesized finalization")
	}
}

func TestApplyFinalizerOutputMatchesByIDAndPreservesInputLocation(t *testing.T) {
	const idA = "11111111-1111-4111-8111-111111111111"
	const idB = "22222222-2222-4222-8222-222222222222"
	locA := model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}
	locB := model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}
	in := []model.Finding{
		{ID: idA, Title: "Issue A", CodeLocation: locA},
		{ID: idB, Title: "Issue B", CodeLocation: locB},
	}
	finalizer := []model.Finding{
		{
			ID: idA, Title: "Issue A refined", CodeLocation: locB,
			Finalization: &model.FindingFinalization{Title: "Final A", Body: "body", Priority: 2, Remarks: "matched by id"},
		},
	}

	stats := applyFinalizerOutput(in, finalizer)
	if stats.Matched != 1 || stats.Omitted != 1 || stats.Ignored != 0 {
		t.Fatalf("stats = %+v, want matched=1 omitted=1 ignored=0", stats)
	}
	if in[0].CodeLocation != locA {
		t.Fatalf("location = %#v, want preserved input location %#v", in[0].CodeLocation, locA)
	}
	if in[0].Finalization == nil || in[0].Finalization.Title != "Final A" {
		t.Fatalf("finalization = %#v, want applied by id", in[0].Finalization)
	}
	if in[1].Finalization == nil || !strings.Contains(in[1].Finalization.Remarks, "omitted") {
		t.Fatalf("second finalization = %#v, want synthesized omitted finalization", in[1].Finalization)
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

// Covers the three regimes of verdictConstraintsFor based on the priority
// floor = min(finding.priority, verification.priority): P0 → must be
// "patch is incorrect", P1-only → unconstrained, no critical → must be
// "patch is correct".
func TestVerdictConstraintsForPriorityFloor(t *testing.T) {
	cases := []struct {
		name string
		in   []model.Finding
		want []string // nil => unconstrained
	}{
		{
			name: "all P2/P3 → patch is correct",
			in: []model.Finding{
				{Priority: intPtr(2)},
				{Priority: intPtr(3)},
			},
			want: []string{"patch is correct"},
		},
		{
			name: "P1 reviewer only → unconstrained",
			in: []model.Finding{
				{Priority: intPtr(2)},
				{Priority: intPtr(1)},
			},
			want: nil,
		},
		{
			name: "P0 reviewer → patch is incorrect",
			in: []model.Finding{
				{Priority: intPtr(0)},
			},
			want: []string{"patch is incorrect"},
		},
		{
			name: "P3 reviewer, P0 verifier → patch is incorrect (floor)",
			in: []model.Finding{
				{Priority: intPtr(3), Verification: &model.FindingVerification{Priority: 0}},
			},
			want: []string{"patch is incorrect"},
		},
		{
			name: "P0 reviewer, P3 verifier → patch is incorrect (floor)",
			in: []model.Finding{
				{Priority: intPtr(0), Verification: &model.FindingVerification{Priority: 3}},
			},
			want: []string{"patch is incorrect"},
		},
		{
			name: "P1 reviewer, P0 verifier → patch is incorrect",
			in: []model.Finding{
				{Priority: intPtr(1), Verification: &model.FindingVerification{Priority: 0}},
			},
			want: []string{"patch is incorrect"},
		},
		{
			name: "P2 reviewer, P1 verifier → unconstrained (P1 floor)",
			in: []model.Finding{
				{Priority: intPtr(2), Verification: &model.FindingVerification{Priority: 1}},
			},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := verdictConstraintsFor(tc.in, 3).AllowedCorrectness
			if len(tc.want) == 0 {
				if len(got) != 0 {
					t.Fatalf("AllowedCorrectness = %#v, want unconstrained", got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("AllowedCorrectness = %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("AllowedCorrectness = %#v, want %#v", got, tc.want)
				}
			}
		})
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
				Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
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
