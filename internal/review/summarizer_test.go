package review

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestSummarizeShortensBodyAndCopiesFinalizationFields(t *testing.T) {
	const findingID = "11111111-1111-4111-8111-111111111111"
	fin := &model.FindingFinalization{
		Title:           "Final issue",
		Body:            "A long finalized body that explains the problem in considerable detail across several sentences.",
		Priority:        1,
		ConfidenceScore: 0.75,
		Remarks:         "keep",
	}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{ID: findingID, Summarization: &model.FindingSummarization{Body: "Short body.\nSecond line."}},
				},
				OverallExplanation: "Short overall.",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				ID:              findingID,
				Title:           "Fix issue",
				Body:            "original body",
				ConfidenceScore: 0.7,
				Priority:        intPtr(1),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification:    &model.FindingVerification{ID: findingID, Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
				Finalization:    fin,
			},
		},
		OverallCorrectness:     "patch is correct",
		OverallExplanation:     "ok",
		OverallConfidenceScore: 0.7,
	}

	out, run, err := engine.Summarize(context.Background(), in, SummarizeOptions{})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	if run.Role != "summarize" {
		t.Fatalf("run.Role = %q, want summarize", run.Role)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(llmClient.reqs))
	}
	if got := llmClient.reqs[0].SchemaKind; got != llm.SchemaKindSummarize {
		t.Fatalf("schema kind = %v, want summarize", got)
	}
	// The user prompt's source body must be the finalized body, not the original.
	if userPrompt := llmClient.reqs[0].Messages[1].Content; !strings.Contains(userPrompt, "long finalized body") || !strings.Contains(userPrompt, findingID) {
		t.Fatalf("summarize user prompt missing finalized body or id:\n%s", userPrompt)
	}
	if sys := llmClient.reqs[0].Messages[0].Content; !strings.Contains(sys, "summarization") {
		t.Fatalf("summarize system prompt missing 'summarization':\n%s", sys)
	}

	got := out.Findings[0].Summarization
	if got == nil {
		t.Fatal("summarization is nil")
	}
	if got.Body != "Short body.\nSecond line." {
		t.Fatalf("summarization.body = %q, want shortened body", got.Body)
	}
	// Every non-body field is copied verbatim from the finalization.
	if got.Title != fin.Title || got.Priority != fin.Priority || got.ConfidenceScore != fin.ConfidenceScore || got.Remarks != fin.Remarks {
		t.Fatalf("summarization fields = %#v, want copied from finalization %#v", got, fin)
	}
	// Finalization itself is untouched.
	if out.Findings[0].Finalization == nil || *out.Findings[0].Finalization != *fin {
		t.Fatalf("finalization mutated: %#v, want %#v", out.Findings[0].Finalization, fin)
	}
}

func TestSummarizeSynthesizesWhenNoFinalization(t *testing.T) {
	const findingID = "22222222-2222-4222-8222-222222222222"
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{ID: findingID, Summarization: &model.FindingSummarization{Body: "Short."}},
				},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				ID:              findingID,
				Title:           "Fix issue",
				Body:            "original body",
				ConfidenceScore: 0.6,
				Priority:        intPtr(2),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
			},
		},
		OverallCorrectness: "patch is incorrect",
	}

	out, _, err := engine.Summarize(context.Background(), in, SummarizeOptions{})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	got := out.Findings[0].Summarization
	if got == nil {
		t.Fatal("summarization is nil")
	}
	if got.Body != "Short." {
		t.Fatalf("summarization.body = %q, want shortened", got.Body)
	}
	if got.Title != "Fix issue" || got.Priority != 2 || got.ConfidenceScore != 0.6 {
		t.Fatalf("synthesized summarization = %#v, want fields from finding", got)
	}
}

func TestSummarizeShortensOverallExplanation(t *testing.T) {
	const findingID = "33333333-3333-4333-8333-333333333333"
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{ID: findingID, Summarization: &model.FindingSummarization{Body: "Short."}},
				},
				OverallExplanation: "Short overall.\nVerdict line.",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				ID:           findingID,
				Title:        "Fix issue",
				Body:         "body",
				Priority:     intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Finalization: &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 1, ConfidenceScore: 0.75, Remarks: "keep"},
			},
		},
		OverallCorrectness: "patch is correct",
		OverallExplanation: "A LONG_OVERALL_MARKER explanation describing the patch intent and the review verdict in detail.",
	}

	out, _, err := engine.Summarize(context.Background(), in, SummarizeOptions{})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	// The input overall_explanation is sent to the model to shorten.
	if up := llmClient.reqs[0].Messages[1].Content; !strings.Contains(up, `"overall_explanation"`) || !strings.Contains(up, "LONG_OVERALL_MARKER") {
		t.Fatalf("summarize user prompt missing overall_explanation:\n%s", up)
	}
	// The shortened overall_explanation is adopted.
	if out.OverallExplanation != "Short overall.\nVerdict line." {
		t.Fatalf("out.OverallExplanation = %q, want shortened", out.OverallExplanation)
	}
}

func TestSummarizeDisablePatchSummaryInstruction(t *testing.T) {
	const findingID = "44444444-4444-4444-8444-444444444444"
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{ID: findingID, Summarization: &model.FindingSummarization{Body: "Short."}},
				},
				OverallExplanation: "Patch is incorrect because the failure remains.",
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				ID:           findingID,
				Title:        "Fix issue",
				Body:         "body",
				Priority:     intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Finalization: &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 1, ConfidenceScore: 0.75, Remarks: "keep"},
			},
		},
		OverallCorrectness: "patch is incorrect",
		OverallExplanation: "The patch appears intended to update auth. Patch is incorrect because the failure remains.",
	}

	out, _, err := engine.Summarize(context.Background(), in, SummarizeOptions{DisablePatchSummary: true})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	sys := llmClient.reqs[0].Messages[0].Content
	if !strings.Contains(sys, "do NOT include an assumption about what the patch is intended to do") {
		t.Fatalf("summarize system prompt missing disabled-summary instruction:\n%s", sys)
	}
	if out.OverallExplanation != "Patch is incorrect because the failure remains." {
		t.Fatalf("out.OverallExplanation = %q, want response adopted", out.OverallExplanation)
	}
}

// When the input carries an overall_explanation but the model omits it, the
// validator forces a retry to try to get it. If the model still omits it after
// retries, the pass soft-accepts and the finalized overall_explanation is kept
// (apply-guard) — so the shortening is attempted, never silently skipped.
func TestSummarizeRetriesThenFallsBackWhenOverallMissing(t *testing.T) {
	const findingID = "44444444-4444-4444-8444-444444444444"
	// Two responses, both omitting overall_explanation: the first triggers a
	// retry, the second exhausts retries and is soft-accepted.
	resp := func() *llm.ReviewResponse {
		return &llm.ReviewResponse{
			Findings: []model.Finding{
				{ID: findingID, Summarization: &model.FindingSummarization{Body: "Short."}},
			},
			// No OverallExplanation emitted.
		}
	}
	llmClient := &capturingLLM{resps: []*llm.ReviewResponse{resp(), resp()}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				ID:           findingID,
				Title:        "Fix issue",
				Body:         "body",
				Priority:     intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Finalization: &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 1, ConfidenceScore: 0.75, Remarks: "keep"},
			},
		},
		OverallCorrectness: "patch is correct",
		OverallExplanation: "finalized overall explanation",
	}

	out, _, err := engine.Summarize(context.Background(), in, SummarizeOptions{MaxOutputRetries: 1})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	// Missing overall forced a retry: 1 initial call + 1 retry.
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d, want 2 (overall-missing should trigger a retry)", len(llmClient.reqs))
	}
	// After retries exhaust, the finalized overall is preserved by the apply-guard.
	if out.OverallExplanation != "finalized overall explanation" {
		t.Fatalf("out.OverallExplanation = %q, want finalized preserved", out.OverallExplanation)
	}
}

// The overall_explanation requirement is conditional: a standalone summarize
// over findings with no input overall must not be forced to invent one.
func TestSummarizerOutputValidatorOverallRequirementIsConditional(t *testing.T) {
	const findingID = "55555555-5555-4555-8555-555555555555"
	inputFindings := []model.Finding{
		{ID: findingID, CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}},
	}
	respWithOverall := &llm.ReviewResponse{
		Findings:           []model.Finding{{ID: findingID, Summarization: &model.FindingSummarization{Body: "x"}}},
		OverallExplanation: "short overall",
	}
	respNoOverall := &llm.ReviewResponse{
		Findings: []model.Finding{{ID: findingID, Summarization: &model.FindingSummarization{Body: "x"}}},
	}

	// Input has an overall + response omits it -> invalid, flags overall_explanation.
	got := summarizerOutputValidator(inputFindings, "long overall")(respNoOverall)
	if got == nil {
		t.Fatal("want invalid when input overall present but response omits it")
	}
	if !slices.Contains(got.MissingFields, "overall_explanation") {
		t.Fatalf("MissingFields = %v, want overall_explanation", got.MissingFields)
	}

	// Input has an overall + response provides it -> valid.
	if got := summarizerOutputValidator(inputFindings, "long overall")(respWithOverall); got != nil {
		t.Fatalf("want valid when response carries overall, got %q", got.Reason)
	}

	// Input has NO overall -> not required even when the response omits it.
	if got := summarizerOutputValidator(inputFindings, "")(respNoOverall); got != nil {
		t.Fatalf("want valid when input has no overall, got %q", got.Reason)
	}
}

func TestSummarizeEmptyFindingsIsNoOp(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{OverallCorrectness: "patch is correct"}

	out, run, err := engine.Summarize(context.Background(), in, SummarizeOptions{})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	if len(llmClient.reqs) != 0 {
		t.Fatalf("requests = %d, want 0 for empty findings", len(llmClient.reqs))
	}
	if run.Role != "summarize" {
		t.Fatalf("run.Role = %q, want summarize", run.Role)
	}
	if out == nil || len(out.Findings) != 0 {
		t.Fatalf("out = %#v, want empty clone", out)
	}
}

func TestApplySummarizedFindingIgnoresEmptyLLMBody(t *testing.T) {
	dst := &model.Finding{
		Finalization: &model.FindingFinalization{Title: "T", Body: "finalized body", Priority: 1, ConfidenceScore: 0.5, Remarks: "r"},
	}
	applySummarizedFinding(dst, model.Finding{Summarization: &model.FindingSummarization{Body: "   "}})
	if dst.Summarization == nil {
		t.Fatal("summarization is nil")
	}
	if dst.Summarization.Body != "finalized body" {
		t.Fatalf("body = %q, want finalized body retained when LLM body is blank", dst.Summarization.Body)
	}
}

func TestExampleSnippetForSummarizeIncludesSummarization(t *testing.T) {
	snippet := exampleSnippetFor(llm.SchemaKindSummarize)
	if !strings.Contains(snippet, "summarization") {
		t.Fatalf("summarize retry example missing summarization: %s", snippet)
	}
}
