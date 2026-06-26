package review

import (
	"context"
	"reflect"
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
		Suggestions:     []model.Suggestion{{Body: "final suggestion", LineRange: model.LineRange{Start: 3, End: 4}}},
	}
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{ID: findingID, Summarization: &model.FindingSummarization{Body: "Short body.\nSecond line."}},
					{ID: overallSummaryID, Summarization: &model.FindingSummarization{Body: "Short overall."}},
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
	if !slices.Equal(got.Suggestions, fin.Suggestions) {
		t.Fatalf("summarization suggestions = %#v, want copied from finalization %#v", got.Suggestions, fin.Suggestions)
	}
	// Finalization itself is untouched.
	if out.Findings[0].Finalization == nil || !reflect.DeepEqual(out.Findings[0].Finalization, fin) {
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
					{ID: overallSummaryID, Summarization: &model.FindingSummarization{Body: "Short overall.\nVerdict line."}},
				},
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
	if up := llmClient.reqs[0].Messages[1].Content; !strings.Contains(up, overallSummaryID) || !strings.Contains(up, "LONG_OVERALL_MARKER") {
		t.Fatalf("summarize user prompt missing overall summary item:\n%s", up)
	}
	// The shortened overall_explanation is adopted.
	if out.OverallExplanation != "Short overall.\nVerdict line." {
		t.Fatalf("out.OverallExplanation = %q, want shortened", out.OverallExplanation)
	}
}

func TestSummarizeShortensProseSuggestionsInSameCall(t *testing.T) {
	const findingID = "66666666-6666-4666-8666-666666666666"
	suggestionID := summarizeSuggestionItemID(findingID, 0)
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{ID: findingID, Summarization: &model.FindingSummarization{Body: "Short body."}},
					{ID: suggestionID, Summarization: &model.FindingSummarization{Body: "Short suggestion."}},
				},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	codeSuggestion := "if err != nil {\n\treturn err\n}"
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				ID:              findingID,
				Title:           "Fix issue",
				Body:            "original body",
				ConfidenceScore: 0.6,
				Priority:        intPtr(2),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Suggestions: []model.Suggestion{
					{Body: "Replace the repeated setup with a shared helper so each test covers one clear scenario.", LineRange: model.LineRange{Start: 10, End: 12}},
					{Body: codeSuggestion, LineRange: model.LineRange{Start: 20, End: 22}},
				},
				Finalization: &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 2, ConfidenceScore: 0.7, Remarks: "keep"},
			},
		},
		OverallCorrectness: "patch is incorrect",
	}

	out, _, err := engine.Summarize(context.Background(), in, SummarizeOptions{})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d, want one shared summarize call", len(llmClient.reqs))
	}
	userPrompt := llmClient.reqs[0].Messages[1].Content
	if !strings.Contains(userPrompt, `"kind": "suggestion"`) || !strings.Contains(userPrompt, suggestionID) {
		t.Fatalf("summarize user prompt missing prose suggestion item:\n%s", userPrompt)
	}
	if strings.Contains(userPrompt, codeSuggestion) {
		t.Fatalf("code-like suggestion should not be sent for summarization:\n%s", userPrompt)
	}
	if got := out.Findings[0].Summarization.Suggestions[0].Body; got != "Short suggestion." {
		t.Fatalf("summarization suggestion body = %q, want shortened", got)
	}
	if got := out.Findings[0].Summarization.Suggestions[0].LineRange; got != (model.LineRange{Start: 10, End: 12}) {
		t.Fatalf("summarization suggestion line range = %+v, want preserved", got)
	}
	if got := out.Findings[0].Summarization.Suggestions[1].Body; got != codeSuggestion {
		t.Fatalf("code-like summarization suggestion body = %q, want unchanged", got)
	}
	if got := out.Findings[0].Suggestions[0].Body; got == "Short suggestion." {
		t.Fatalf("top-level suggestion was mutated, want reviewer suggestion preserved")
	}
}

func TestSummarizeSkipSuggestionsOmitsSuggestionItems(t *testing.T) {
	const findingID = "88888888-8888-4888-8888-888888888888"
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{ID: findingID, Summarization: &model.FindingSummarization{Body: "Short body."}},
				},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				ID:           findingID,
				Title:        "Fix issue",
				Body:         "original body",
				Priority:     intPtr(2),
				CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Suggestions:  []model.Suggestion{{Body: "reviewer prose suggestion that should not be summarized", LineRange: model.LineRange{Start: 10, End: 12}}},
				Finalization: &model.FindingFinalization{
					Title:       "Final issue",
					Body:        "final body",
					Priority:    2,
					Remarks:     "keep",
					Suggestions: []model.Suggestion{{Body: "final prose suggestion that should not be summarized", LineRange: model.LineRange{Start: 10, End: 12}}},
				},
			},
		},
	}

	out, _, err := engine.Summarize(context.Background(), in, SummarizeOptions{SkipSuggestions: true})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d, want one summarize call", len(llmClient.reqs))
	}
	systemPrompt := llmClient.reqs[0].Messages[0].Content
	if strings.Contains(systemPrompt, "suggestion") {
		t.Fatalf("summarize system prompt should not mention suggestions when skipped:\n%s", systemPrompt)
	}
	userPrompt := llmClient.reqs[0].Messages[1].Content
	if strings.Contains(userPrompt, `"kind": "suggestion"`) || strings.Contains(userPrompt, "prose suggestion") {
		t.Fatalf("summarize user prompt should not carry suggestion items:\n%s", userPrompt)
	}
	if len(out.Findings[0].Suggestions) != 0 {
		t.Fatalf("top-level suggestions = %+v, want stripped", out.Findings[0].Suggestions)
	}
	if out.Findings[0].Finalization != nil && len(out.Findings[0].Finalization.Suggestions) != 0 {
		t.Fatalf("finalization suggestions = %+v, want stripped", out.Findings[0].Finalization.Suggestions)
	}
	if out.Findings[0].Summarization != nil && len(out.Findings[0].Summarization.Suggestions) != 0 {
		t.Fatalf("summarization suggestions = %+v, want stripped", out.Findings[0].Summarization.Suggestions)
	}
}

func TestSummarizeDoesNotRetryWhenOptionalSuggestionSummaryMissing(t *testing.T) {
	const findingID = "77777777-7777-4777-8777-777777777777"
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{ID: findingID, Summarization: &model.FindingSummarization{Body: "Short body."}},
				},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	originalSuggestion := "Replace the repeated setup with a shared helper so each test covers one clear scenario."
	in := &model.ReviewResult{
		Findings: []model.Finding{
			{
				ID:              findingID,
				Title:           "Fix issue",
				Body:            "original body",
				ConfidenceScore: 0.6,
				Priority:        intPtr(2),
				CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Suggestions:     []model.Suggestion{{Body: originalSuggestion, LineRange: model.LineRange{Start: 10, End: 12}}},
				Finalization:    &model.FindingFinalization{Title: "Final issue", Body: "final body", Priority: 2, ConfidenceScore: 0.7, Remarks: "keep"},
			},
		},
		OverallCorrectness: "patch is incorrect",
	}

	out, _, err := engine.Summarize(context.Background(), in, SummarizeOptions{MaxOutputRetries: 2})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d, want no retry for omitted optional suggestion", len(llmClient.reqs))
	}
	if len(out.Warnings) != 0 {
		t.Fatalf("warnings = %v, want none for omitted optional suggestion", out.Warnings)
	}
	if got := out.Findings[0].Summarization.Suggestions[0].Body; got != originalSuggestion {
		t.Fatalf("summarization suggestion body = %q, want original fallback", got)
	}
	if got := out.Findings[0].Suggestions[0].Body; got != originalSuggestion {
		t.Fatalf("top-level suggestion body = %q, want original reviewer suggestion", got)
	}
}

// The summarize prompt no longer carries patch-summary handling — overall
// explanation (and the DisablePatchSummary instruction) moved to the verdict
// agent (see the verdict prompt test in finalizer_test.go). Summarize-side
// overall-explanation shortening is covered by TestSummarizeShortensOverallExplanation.

// When the input carries an overall_explanation but the model omits its
// synthetic item, the validator forces a retry to try to get it. If the model
// still omits it after retries, the pass soft-accepts and the finalized
// overall_explanation is kept (apply-guard) — so the shortening is attempted,
// never silently skipped.
func TestSummarizeRetriesThenFallsBackWhenOverallMissing(t *testing.T) {
	const findingID = "44444444-4444-4444-8444-444444444444"
	// Two responses, both omitting overall_explanation: the first triggers a
	// retry, the second exhausts retries and is soft-accepted.
	resp := func() *llm.ReviewResponse {
		return &llm.ReviewResponse{
			Findings: []model.Finding{
				{ID: findingID, Summarization: &model.FindingSummarization{Body: "Short."}},
			},
			// No synthetic overall summary item emitted.
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
	// Missing synthetic overall item forced a retry: 1 initial call + 1 retry.
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d, want 2 (overall-missing should trigger a retry)", len(llmClient.reqs))
	}
	// After retries exhaust, the finalized overall is preserved by the apply-guard.
	if out.OverallExplanation != "finalized overall explanation" {
		t.Fatalf("out.OverallExplanation = %q, want finalized preserved", out.OverallExplanation)
	}
}

// The overall summary item requirement is conditional: a standalone summarize
// over findings with no input overall must not be forced to invent one.
func TestSummarizerOutputValidatorOverallRequirementIsConditional(t *testing.T) {
	const findingID = "55555555-5555-4555-8555-555555555555"
	inputItems := []summarizeTextItem{{ID: findingID, Title: "T", Body: "body"}}
	inputWithOverall := append([]summarizeTextItem(nil), inputItems...)
	inputWithOverall = append(inputWithOverall, summarizeTextItem{ID: overallSummaryID, Title: "Overall explanation", Body: "long overall"})
	respWithOverall := &llm.ReviewResponse{
		Findings: []model.Finding{
			{ID: findingID, Summarization: &model.FindingSummarization{Body: "x"}},
			{ID: overallSummaryID, Summarization: &model.FindingSummarization{Body: "short overall"}},
		},
	}
	respNoOverall := &llm.ReviewResponse{
		Findings: []model.Finding{{ID: findingID, Summarization: &model.FindingSummarization{Body: "x"}}},
	}

	// Input has an overall item + response omits it -> invalid.
	got := summarizerOutputValidator(inputWithOverall)(respNoOverall)
	if got == nil {
		t.Fatal("want invalid when input overall present but response omits it")
	}
	if !slices.Contains(got.MissingFields, "findings") {
		t.Fatalf("MissingFields = %v, want findings", got.MissingFields)
	}

	// Input has an overall + response provides it -> valid.
	if got := summarizerOutputValidator(inputWithOverall)(respWithOverall); got != nil {
		t.Fatalf("want valid when response carries overall, got %q", got.Reason)
	}

	// Input has NO overall -> not required even when the response omits it.
	if got := summarizerOutputValidator(inputItems)(respNoOverall); got != nil {
		t.Fatalf("want valid when input has no overall, got %q", got.Reason)
	}
}

func TestSummarizeRecoversMutatedIDByPosition(t *testing.T) {
	const (
		id1       = "11111111-1111-4111-8111-111111111111"
		id2       = "22222222-2222-4222-8222-222222222222"
		id3       = "33333333-3333-4333-8333-333333333333"
		mutatedID = "af5fc1a4-fd98-40a3-95ad-ba44f9852efd"
	)
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{ID: id1, Summarization: &model.FindingSummarization{Body: "short 1"}},
					{ID: id2, Summarization: &model.FindingSummarization{Body: "short 2"}},
					{ID: mutatedID, Summarization: &model.FindingSummarization{Body: "short 3"}},
				},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			summarizerTestFinding(id1, "final 1"),
			summarizerTestFinding(id2, "final 2"),
			summarizerTestFinding(id3, "final 3"),
		},
		OverallCorrectness: "patch is incorrect",
	}

	out, _, err := engine.Summarize(context.Background(), in, SummarizeOptions{MaxOutputRetries: 2})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d, want 1 (position recovery should avoid retry)", len(llmClient.reqs))
	}
	for i, want := range []string{"short 1", "short 2", "short 3"} {
		if got := out.Findings[i].Summarization.Body; got != want {
			t.Fatalf("finding %d summarization.body = %q, want %q", i, got, want)
		}
	}
	if len(out.Warnings) != 0 {
		t.Fatalf("warnings = %v, want none", out.Warnings)
	}
}

func TestSummarizerPositionRecoveryIgnoresBlankInputIDs(t *testing.T) {
	const (
		id2       = "22222222-2222-4222-8222-222222222222"
		mutatedID = "af5fc1a4-fd98-40a3-95ad-ba44f9852efd"
	)
	items := []summarizeTextItem{
		{ID: "  ", Title: "blank", Body: "body 1"},
		{ID: id2, Title: "two", Body: "body 2"},
	}
	out := []model.Finding{
		{ID: "", Summarization: &model.FindingSummarization{Body: "short 1"}},
		{ID: mutatedID, Summarization: &model.FindingSummarization{Body: "short 2"}},
	}

	if recovered, ok := recoverSummarizerFindingsByPosition(items, out); ok {
		t.Fatalf("recoverSummarizerFindingsByPosition = %#v, want no recovery from blank ID anchor", recovered)
	}
}

func TestSummarizeAcceptsWhitespaceWrappedIDsWithoutPositionRecovery(t *testing.T) {
	const (
		id1 = "11111111-1111-4111-8111-111111111111"
		id2 = "22222222-2222-4222-8222-222222222222"
	)
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				Findings: []model.Finding{
					{ID: "  " + id2 + "\n", Summarization: &model.FindingSummarization{Body: "short 2"}},
					{ID: "\t" + id1 + "  ", Summarization: &model.FindingSummarization{Body: "short 1"}},
				},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			summarizerTestFinding(id1, "final 1"),
			summarizerTestFinding(id2, "final 2"),
		},
		OverallCorrectness: "patch is incorrect",
	}

	out, _, err := engine.Summarize(context.Background(), in, SummarizeOptions{MaxOutputRetries: 1})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(llmClient.reqs))
	}
	for i, want := range []string{"short 1", "short 2"} {
		if got := out.Findings[i].Summarization.Body; got != want {
			t.Fatalf("finding %d summarization.body = %q, want %q", i, got, want)
		}
	}
	if len(out.Warnings) != 0 {
		t.Fatalf("warnings = %v, want none", out.Warnings)
	}
}

func TestSummarizerRetryGuidanceListsAllowedOmittedAndIgnoredIDs(t *testing.T) {
	const (
		id1       = "11111111-1111-4111-8111-111111111111"
		id2       = "22222222-2222-4222-8222-222222222222"
		mutatedID = "af5fc1a4-fd98-40a3-95ad-ba44f9852efd"
	)
	inputItems := []summarizeTextItem{
		{ID: id1, Title: "one", Body: "body 1"},
		{ID: id2, Title: "two", Body: "body 2"},
	}
	resp := &llm.ReviewResponse{
		Findings: []model.Finding{
			{ID: "  " + mutatedID + "\n", Summarization: &model.FindingSummarization{Body: "short"}},
		},
	}

	invalid := summarizerOutputValidator(inputItems)(resp)
	if invalid == nil {
		t.Fatal("want invalid response")
	}
	rendered, err := renderPromptFile(invalid.RetryGuidanceTemplate, invalid.RetryGuidanceData)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Allowed input IDs, in order:",
		"`" + id1 + "`",
		"`" + id2 + "`",
		"Omitted input IDs:",
		"Ignored output IDs:",
		"`" + mutatedID + "`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("retry guidance missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "code_location") {
		t.Fatalf("retry guidance should not mention code_location:\n%s", rendered)
	}
	if strings.Contains(rendered, "  "+mutatedID) || strings.Contains(rendered, mutatedID+"\n`") {
		t.Fatalf("retry guidance should trim ignored IDs:\n%s", rendered)
	}
}

func TestSummarizeRetriesThenFallsBackWhenFindingMissing(t *testing.T) {
	const (
		id1 = "11111111-1111-4111-8111-111111111111"
		id2 = "22222222-2222-4222-8222-222222222222"
	)
	resp := func() *llm.ReviewResponse {
		return &llm.ReviewResponse{
			Findings: []model.Finding{
				{ID: id1, Summarization: &model.FindingSummarization{Body: "short 1"}},
			},
		}
	}
	llmClient := &capturingLLM{resps: []*llm.ReviewResponse{resp(), resp()}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		Findings: []model.Finding{
			summarizerTestFinding(id1, "final 1"),
			summarizerTestFinding(id2, "final 2"),
		},
		OverallCorrectness: "patch is incorrect",
	}

	out, _, err := engine.Summarize(context.Background(), in, SummarizeOptions{MaxOutputRetries: 1})
	if err != nil {
		t.Fatalf("Summarize returned err: %v", err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d, want 2 (count mismatch should retry once)", len(llmClient.reqs))
	}
	if got := out.Findings[0].Summarization.Body; got != "short 1" {
		t.Fatalf("first body = %q, want shortened body", got)
	}
	if got := out.Findings[1].Summarization.Body; got != "final 2" {
		t.Fatalf("second body = %q, want finalized fallback", got)
	}
	if len(out.Warnings) != 1 || !strings.Contains(out.Warnings[0], "Summarizer output mismatch") {
		t.Fatalf("warnings = %v, want summarizer mismatch warning", out.Warnings)
	}
}

func TestSummarizerPositionRecoveryRequiresAnchorForMultipleItems(t *testing.T) {
	inputItems := []summarizeTextItem{
		{ID: "11111111-1111-4111-8111-111111111111", Title: "one", Body: "body 1"},
		{ID: "22222222-2222-4222-8222-222222222222", Title: "two", Body: "body 2"},
	}
	resp := &llm.ReviewResponse{
		Findings: []model.Finding{
			{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Summarization: &model.FindingSummarization{Body: "short 1"}},
			{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Summarization: &model.FindingSummarization{Body: "short 2"}},
		},
	}

	if got := summarizerOutputValidator(inputItems)(resp); got == nil {
		t.Fatal("want invalid response when no multi-item output ID anchors input order")
	}
}

func TestSummarizerPositionRecoveryAllowsSingleUnknownID(t *testing.T) {
	inputItems := []summarizeTextItem{{ID: "11111111-1111-4111-8111-111111111111", Title: "one", Body: "body"}}
	resp := &llm.ReviewResponse{
		Findings: []model.Finding{
			{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Summarization: &model.FindingSummarization{Body: "short"}},
		},
	}

	if got := summarizerOutputValidator(inputItems)(resp); got != nil {
		t.Fatalf("want single-item unknown ID to recover by position, got %q", got.Reason)
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
	snippet := exampleSnippetFor(llm.SchemaKindSummarize, false)
	if !strings.Contains(snippet, "summarization") {
		t.Fatalf("summarize retry example missing summarization: %s", snippet)
	}
}

func summarizerTestFinding(id, finalBody string) model.Finding {
	return model.Finding{
		ID:              id,
		Title:           "Fix issue",
		Body:            "original body",
		ConfidenceScore: 0.7,
		Priority:        intPtr(1),
		CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
		Finalization:    &model.FindingFinalization{Title: "Final issue", Body: finalBody, Priority: 1, ConfidenceScore: 0.75, Remarks: "keep"},
	}
}
