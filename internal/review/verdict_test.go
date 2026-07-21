package review

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestVerdictEmptyFindingsResetsStaleExplanation(t *testing.T) {
	engine := NewEngine(stubSource{}, &multiAgentLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{
		OverallCorrectness: "patch is incorrect",
		OverallExplanation: "Merged 3 reviewer finding lists into 2 findings.",
	}
	out, run, err := engine.Verdict(context.Background(), sampleReviewCtx(), in, VerdictOptions{})
	if err != nil {
		t.Fatalf("Verdict returned err: %v", err)
	}
	if run.Status != model.AgentRunStatusSkipped {
		t.Fatalf("run status = %v, want skipped", run.Status)
	}
	if out.OverallCorrectness != "patch is correct" {
		t.Fatalf("correctness = %q, want \"patch is correct\"", out.OverallCorrectness)
	}
	if out.OverallExplanation != "No finalized findings remained." {
		t.Fatalf("explanation = %q, want stale text reset", out.OverallExplanation)
	}
}

func TestVerdictConfidenceThresholdFiltersPromptAndResult(t *testing.T) {
	loc := func(line int) model.CodeLocation {
		return model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: line, End: line}}
	}
	low := model.Finding{
		ID: "11111111-1111-4111-8111-111111111111", Title: "Low confidence", Body: "low", ConfidenceScore: 0.99, Priority: intPtr(1), CodeLocation: loc(1),
		Finalization: &model.FindingFinalization{Title: "Low final", Body: "low final", Priority: 1, ConfidenceScore: 0.69},
	}
	kept := model.Finding{
		ID: "22222222-2222-4222-8222-222222222222", Title: "Kept confidence", Body: "kept", ConfidenceScore: 0.1, Priority: intPtr(1), CodeLocation: loc(2),
		Finalization: &model.FindingFinalization{Title: "Kept final", Body: "kept final", Priority: 1, ConfidenceScore: 0.70},
	}
	llmClient := &capturingLLM{resps: []*llm.ReviewResponse{{
		OverallCorrectness: "patch is incorrect",
		OverallExplanation: "kept issue remains",
	}}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{Findings: []model.Finding{low, kept}, OverallCorrectness: "patch is incorrect", OverallExplanation: "pre-filter"}

	out, _, err := engine.Verdict(context.Background(), sampleReviewCtx(), in, VerdictOptions{ConfidenceThreshold: 0.7})
	if err != nil {
		t.Fatalf("Verdict returned err: %v", err)
	}
	if len(out.Findings) != 1 || out.Findings[0].ID != kept.ID {
		t.Fatalf("findings = %#v, want only kept finding", out.Findings)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("verdict requests = %d, want 1", len(llmClient.reqs))
	}
	payload := verdictPromptPayload(t, llmClient.reqs[0])
	findings, ok := payload["findings"].([]any)
	if !ok || len(findings) != 1 {
		t.Fatalf("prompt findings = %#v, want one", payload["findings"])
	}
	raw, _ := json.Marshal(payload)
	if strings.Contains(string(raw), low.ID) || !strings.Contains(string(raw), kept.ID) {
		t.Fatalf("prompt payload did not filter as expected: %s", raw)
	}
}

func TestVerdictDisableSuggestionsOmitsFinalizationSuggestions(t *testing.T) {
	finding := model.Finding{
		ID:           "11111111-1111-4111-8111-111111111111",
		Title:        "Finding",
		Body:         "body",
		Priority:     intPtr(1),
		CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
		Verification: &model.FindingVerification{ID: "11111111-1111-4111-8111-111111111111", Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8, Remarks: "confirmed"},
		Finalization: &model.FindingFinalization{
			Title:           "Final finding",
			Body:            "final body",
			Priority:        1,
			ConfidenceScore: 0.8,
			Remarks:         "keep",
			Suggestions:     []model.Suggestion{{Body: "final prose suggestion that should not reach verdict", LineRange: model.LineRange{Start: 1, End: 1}}},
		},
	}
	llmClient := &capturingLLM{resps: []*llm.ReviewResponse{{
		OverallCorrectness: "patch is incorrect",
		OverallExplanation: "kept issue remains",
	}}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{Findings: []model.Finding{finding}, OverallCorrectness: "patch is incorrect", OverallExplanation: "pre-filter"}

	out, _, err := engine.Verdict(context.Background(), sampleReviewCtx(), in, VerdictOptions{DisableSuggestions: true})
	if err != nil {
		t.Fatalf("Verdict returned err: %v", err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("verdict requests = %d, want 1", len(llmClient.reqs))
	}
	systemPrompt := llmClient.reqs[0].Messages[0].Content
	if strings.Contains(systemPrompt, "suggestion") {
		t.Fatalf("verdict system prompt should not mention suggestions when skipped:\n%s", systemPrompt)
	}
	rawPayload := taskMessageContent(llmClient.reqs[0])
	if strings.Contains(rawPayload, "suggestion") {
		t.Fatalf("verdict user prompt should not include suggestions:\n%s", rawPayload)
	}
	payload := verdictPromptPayload(t, llmClient.reqs[0])
	findings := payload["findings"].([]any)
	finalization := findings[0].(map[string]any)["finalization"].(map[string]any)
	if _, ok := finalization["suggestions"]; ok {
		t.Fatalf("finalization suggestions should be omitted from prompt: %#v", finalization)
	}
	if out.Findings[0].Finalization != nil && len(out.Findings[0].Finalization.Suggestions) != 0 {
		t.Fatalf("output finalization suggestions = %+v, want stripped", out.Findings[0].Finalization.Suggestions)
	}
}

func TestVerdictPriorityThresholdUsesFinalizedPriority(t *testing.T) {
	finding := model.Finding{
		ID:              "11111111-1111-4111-8111-111111111111",
		Title:           "Downgraded by finalizer",
		Body:            "body",
		ConfidenceScore: 0.9,
		Priority:        intPtr(1),
		CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
		Finalization:    &model.FindingFinalization{Title: "Downgraded by finalizer", Body: "body", Priority: 2, ConfidenceScore: 0.8},
	}
	llmClient := &capturingLLM{resps: []*llm.ReviewResponse{{
		OverallCorrectness: "patch is incorrect",
		OverallExplanation: "should not be called",
	}}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	in := &model.ReviewResult{Findings: []model.Finding{finding}, OverallCorrectness: "patch is incorrect", OverallExplanation: "pre-filter"}

	out, run, err := engine.Verdict(context.Background(), sampleReviewCtx(), in, VerdictOptions{PriorityThreshold: "p1"})
	if err != nil {
		t.Fatalf("Verdict returned err: %v", err)
	}
	if len(out.Findings) != 0 {
		t.Fatalf("findings = %#v, want downgraded finding filtered", out.Findings)
	}
	if run.Status != model.AgentRunStatusSkipped {
		t.Fatalf("run status = %q, want skipped", run.Status)
	}
	if out.OverallCorrectness != "patch is correct" || out.OverallExplanation != "No findings remained after priority filtering." {
		t.Fatalf("overall = %q / %q, want priority-filtered clean verdict", out.OverallCorrectness, out.OverallExplanation)
	}
	if len(llmClient.reqs) != 0 {
		t.Fatalf("verdict requests = %d, want none after priority filter removed all findings", len(llmClient.reqs))
	}
}

func TestVerdictConfidenceThresholdZeroKeepsZeroConfidence(t *testing.T) {
	finding := model.Finding{
		ID: "11111111-1111-4111-8111-111111111111", Title: "Zero", Body: "zero", Priority: intPtr(1),
		CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
		Finalization: &model.FindingFinalization{Title: "Zero", Body: "zero", Priority: 1, ConfidenceScore: 0},
	}
	out, drops, err := filterByConfidenceThreshold(&model.ReviewResult{Findings: []model.Finding{finding}}, 0)
	if err != nil {
		t.Fatalf("filterByConfidenceThreshold returned err: %v", err)
	}
	if len(drops) != 0 || len(out.Findings) != 1 {
		t.Fatalf("dropped=%d findings=%d, want kept", len(drops), len(out.Findings))
	}
}

func TestVerdictConfidenceThresholdReportsDroppedFindings(t *testing.T) {
	loc := func(line int) model.CodeLocation {
		return model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: line, End: line}}
	}
	low := model.Finding{
		ID:              "11111111-1111-4111-8111-111111111111",
		Title:           "Low confidence",
		Body:            "low",
		ConfidenceScore: 0.99,
		Priority:        intPtr(1),
		CodeLocation:    loc(1),
		Finalization:    &model.FindingFinalization{Title: "Low final", Body: "low final", Priority: 1, ConfidenceScore: 0.69},
	}
	kept := model.Finding{
		ID:              "22222222-2222-4222-8222-222222222222",
		Title:           "Kept confidence",
		Body:            "kept",
		ConfidenceScore: 0.1,
		Priority:        intPtr(1),
		CodeLocation:    loc(2),
		Finalization:    &model.FindingFinalization{Title: "Kept final", Body: "kept final", Priority: 1, ConfidenceScore: 0.70},
	}
	llmClient := &capturingLLM{resps: []*llm.ReviewResponse{{
		OverallCorrectness: "patch is incorrect",
		OverallExplanation: "kept issue remains",
	}}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	var buf lockedTestBuffer
	engine.SetLogger(logging.New(&buf, true, false))
	in := &model.ReviewResult{Findings: []model.Finding{low, kept}, OverallCorrectness: "patch is incorrect", OverallExplanation: "pre-filter"}

	if _, _, err := engine.Verdict(context.Background(), sampleReviewCtx(), in, VerdictOptions{ConfidenceThreshold: 0.7}); err != nil {
		t.Fatalf("Verdict returned err: %v", err)
	}
	logs := buf.String()
	if !strings.Contains(logs, "Verdict confidence filter: dropped=1 kept=1 threshold=0.70") {
		t.Fatalf("verbose log missing aggregate filter line:\n%s", logs)
	}
	wantDrop := `Verdict confidence filter dropped finding: id=11111111-1111-4111-8111-111111111111 confidence=0.69 source=finalization threshold=0.70 title="Low confidence"`
	if !strings.Contains(logs, wantDrop) {
		t.Fatalf("verbose log missing dropped finding line %q:\n%s", wantDrop, logs)
	}
	if strings.Contains(logs, `id=22222222-2222-4222-8222-222222222222 confidence=0.70 source=finalization`) {
		t.Fatalf("verbose log should not include kept finding as dropped:\n%s", logs)
	}
}

func TestVerdictConfidenceThresholdDropMetadataUsesDisplayConfidenceSource(t *testing.T) {
	findings := []model.Finding{
		{ID: "review", Title: "Review", ConfidenceScore: 0.4},
		{ID: "summary", Title: "Summary", ConfidenceScore: 0.9, Summarization: &model.FindingSummarization{ConfidenceScore: 0.3}},
		{ID: "final", Title: "Final", ConfidenceScore: 0.9, Finalization: &model.FindingFinalization{ConfidenceScore: 0.2}},
	}
	out, drops, err := filterByConfidenceThreshold(&model.ReviewResult{Findings: findings}, 0.5)
	if err != nil {
		t.Fatalf("filterByConfidenceThreshold returned err: %v", err)
	}
	if len(out.Findings) != 0 {
		t.Fatalf("kept findings = %d, want none", len(out.Findings))
	}
	want := []confidenceFilterDrop{
		{ID: "review", Title: "Review", Confidence: 0.4, Source: "review"},
		{ID: "summary", Title: "Summary", Confidence: 0.3, Source: "summarization"},
		{ID: "final", Title: "Final", Confidence: 0.2, Source: "finalization"},
	}
	if len(drops) != len(want) {
		t.Fatalf("drops = %#v, want %#v", drops, want)
	}
	for i := range want {
		if drops[i] != want[i] {
			t.Fatalf("drop[%d] = %#v, want %#v", i, drops[i], want[i])
		}
	}
}

func verdictPromptPayload(t *testing.T, req *llm.ReviewRequest) map[string]any {
	t.Helper()
	content := taskMessageContent(req)
	if content == "" {
		t.Fatalf("verdict request messages = %#v", req)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("unmarshal verdict payload: %v\n%s", err, content)
	}
	return payload
}

func TestOverallConfidenceFor(t *testing.T) {
	// finalized builds a finalized finding whose priority floor and confidence are
	// both driven by the given values (priority == verification priority).
	finalized := func(priority int, conf float64) model.Finding {
		return model.Finding{
			Priority:     intPtr(priority),
			Verification: &model.FindingVerification{Priority: priority, ConfidenceScore: conf},
			Finalization: &model.FindingFinalization{Priority: priority, ConfidenceScore: conf},
		}
	}
	cases := []struct {
		name        string
		correctness string
		findings    []model.Finding
		want        float64
	}{
		{
			name:        "incorrect: max over floor-0 deciding findings",
			correctness: "patch is incorrect",
			findings:    []model.Finding{finalized(0, 0.9), finalized(0, 0.4), finalized(1, 0.99)},
			want:        0.9,
		},
		{
			name:        "incorrect: falls back to floor-1 when no floor-0",
			correctness: "patch is incorrect",
			findings:    []model.Finding{finalized(1, 0.3), finalized(1, 0.7), finalized(2, 0.95)},
			want:        0.7,
		},
		{
			name:        "correct: floor-2/3 findings never temper",
			correctness: "patch is correct",
			findings:    []model.Finding{finalized(2, 0.8), finalized(3, 0.5)},
			want:        1.0,
		},
		{
			name:        "correct: a justified P1 override tempers (not ignored)",
			correctness: "patch is correct",
			findings:    []model.Finding{finalized(1, 0.9), finalized(2, 0.3)},
			want:        0.55,
		},
		{
			name:        "correct: only the P1 tempers, not a stronger P2",
			correctness: "patch is correct",
			findings:    []model.Finding{finalized(1, 0.4), finalized(2, 0.9)},
			want:        0.8,
		},
		{
			name:        "correct: no findings is 1.0",
			correctness: "patch is correct",
			findings:    nil,
			want:        1.0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := overallConfidenceFor(tc.correctness, tc.findings, 3); got != tc.want {
				t.Fatalf("overallConfidenceFor = %.2f, want %.2f", got, tc.want)
			}
		})
	}
}

func TestApplyVerdictFallbackSynthesizesEmptyOverall(t *testing.T) {
	p1 := func(conf float64) model.Finding {
		return model.Finding{
			Priority:     intPtr(1),
			Verification: &model.FindingVerification{Priority: 1, ConfidenceScore: conf},
			Finalization: &model.FindingFinalization{Priority: 1, ConfidenceScore: conf},
		}
	}

	// Open constraint (P1, no P0), no preliminary overall fields: must synthesize a
	// valid, non-empty verdict rather than emit empty correctness/explanation.
	res := &model.ReviewResult{Findings: []model.Finding{p1(0.8)}}
	applyVerdictFallback(res, 3)
	if res.OverallCorrectness != "patch is incorrect" {
		t.Fatalf("correctness = %q, want conservative \"patch is incorrect\"", res.OverallCorrectness)
	}
	if strings.TrimSpace(res.OverallExplanation) == "" {
		t.Fatal("explanation should be synthesized, got empty")
	}
	if res.OverallConfidenceScore != 0.8 {
		t.Fatalf("confidence = %.2f, want 0.8", res.OverallConfidenceScore)
	}

	// A preliminary correctness under an open constraint is preserved, not overwritten.
	kept := &model.ReviewResult{Findings: []model.Finding{p1(0.8)}, OverallCorrectness: "patch is correct", OverallExplanation: "preliminary"}
	applyVerdictFallback(kept, 3)
	if kept.OverallCorrectness != "patch is correct" || kept.OverallExplanation != "preliminary" {
		t.Fatalf("preliminary overall fields not preserved: %q / %q", kept.OverallCorrectness, kept.OverallExplanation)
	}
}

func TestApplyVerdictFallbackReplacesStaleExplanationOnCoercion(t *testing.T) {
	fin := func(priority int, conf float64) model.Finding {
		return model.Finding{
			Priority:     intPtr(priority),
			Verification: &model.FindingVerification{Priority: priority, ConfidenceScore: conf},
			Finalization: &model.FindingFinalization{Priority: priority, ConfidenceScore: conf},
		}
	}
	const synthesized = "inferred from finding priorities"

	// P2/P3 only → forced "patch is correct"; the stale "incorrect" explanation
	// must be replaced so it doesn't contradict the coerced verdict.
	toCorrect := &model.ReviewResult{
		Findings:           []model.Finding{fin(2, 0.8)},
		OverallCorrectness: "patch is incorrect",
		OverallExplanation: "patch is incorrect because the failure remains",
	}
	applyVerdictFallback(toCorrect, 3)
	if toCorrect.OverallCorrectness != "patch is correct" {
		t.Fatalf("correctness = %q, want patch is correct", toCorrect.OverallCorrectness)
	}
	if !strings.Contains(toCorrect.OverallExplanation, synthesized) {
		t.Fatalf("explanation = %q, want stale text replaced", toCorrect.OverallExplanation)
	}

	// P0 → forced "patch is incorrect"; stale "correct" explanation must be replaced.
	toIncorrect := &model.ReviewResult{
		Findings:           []model.Finding{fin(0, 0.9)},
		OverallCorrectness: "patch is correct",
		OverallExplanation: "patch is correct; no blocking issues",
	}
	applyVerdictFallback(toIncorrect, 3)
	if toIncorrect.OverallCorrectness != "patch is incorrect" {
		t.Fatalf("correctness = %q, want patch is incorrect", toIncorrect.OverallCorrectness)
	}
	if !strings.Contains(toIncorrect.OverallExplanation, synthesized) {
		t.Fatalf("explanation = %q, want stale text replaced", toIncorrect.OverallExplanation)
	}

	// Verdict unchanged (already matches the P0 constraint): explanation preserved.
	kept := &model.ReviewResult{
		Findings:           []model.Finding{fin(0, 0.9)},
		OverallCorrectness: "patch is incorrect",
		OverallExplanation: "valid incorrect rationale",
	}
	applyVerdictFallback(kept, 3)
	if kept.OverallExplanation != "valid incorrect rationale" {
		t.Fatalf("explanation = %q, want preserved when verdict unchanged", kept.OverallExplanation)
	}
}

func TestOverallConfidenceForConfidenceSourceFallback(t *testing.T) {
	// No finalization → verifier confidence.
	verifyOnly := model.Finding{
		Priority:     intPtr(0),
		Verification: &model.FindingVerification{Priority: 0, ConfidenceScore: 0.66},
	}
	if got := overallConfidenceFor("patch is incorrect", []model.Finding{verifyOnly}, 3); got != 0.66 {
		t.Fatalf("verification-fallback confidence = %.2f, want 0.66", got)
	}
	// No finalization, no verification → reviewer confidence.
	reviewOnly := model.Finding{Priority: intPtr(0), ConfidenceScore: 0.42}
	if got := overallConfidenceFor("patch is incorrect", []model.Finding{reviewOnly}, 3); got != 0.42 {
		t.Fatalf("review-fallback confidence = %.2f, want 0.42", got)
	}
}

// TestVerdictConstraintsForDemotesRefutedNonFinding proves a refuted non-finding
// the reviewer mislabeled P0 is classified at the threshold floor, so it does not
// force the overall verdict to "patch is incorrect"; a genuine confirmed P0 still does.
func TestVerdictConstraintsForDemotesRefutedNonFinding(t *testing.T) {
	// A refuted non-finding (the "no issue" sentinel) the reviewer mislabeled P0
	// is demoted to the threshold floor, so it cannot force a blocking verdict.
	nonFinding := model.Finding{Priority: intPtr(0), Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 0, Remarks: "no issue: intentional change"}}
	got := verdictConstraintsFor([]model.Finding{nonFinding}, 3).AllowedCorrectness
	if len(got) != 1 || got[0] != "patch is correct" {
		t.Fatalf("non-finding constraints = %v, want [patch is correct]", got)
	}

	// A genuine refutation kept for review (cites code, no sentinel) keeps its P0
	// floor and still forces "patch is incorrect" — the P1 review concern.
	genuineRefuted := model.Finding{Priority: intPtr(0), Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 0, ConfidenceScore: 0.5, Remarks: "the guard at a.go:42 may not cover the empty path"}}
	got = verdictConstraintsFor([]model.Finding{genuineRefuted}, 3).AllowedCorrectness
	if len(got) != 1 || got[0] != "patch is incorrect" {
		t.Fatalf("genuine-refuted-P0 constraints = %v, want [patch is incorrect]", got)
	}

	real := model.Finding{Priority: intPtr(0), Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 0}}
	got = verdictConstraintsFor([]model.Finding{real}, 3).AllowedCorrectness
	if len(got) != 1 || got[0] != "patch is incorrect" {
		t.Fatalf("confirmed-P0 constraints = %v, want [patch is incorrect]", got)
	}
}
