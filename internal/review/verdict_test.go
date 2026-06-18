package review

import (
	"context"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
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
			name:        "correct: tempered by strongest non-blocking finding",
			correctness: "patch is correct",
			findings:    []model.Finding{finalized(2, 0.8), finalized(3, 0.5)},
			want:        0.6,
		},
		{
			name:        "correct: a justified P1 override tempers (not ignored)",
			correctness: "patch is correct",
			findings:    []model.Finding{finalized(1, 0.9), finalized(2, 0.3)},
			want:        0.55,
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
			if got := overallConfidenceFor(tc.correctness, tc.findings); got != tc.want {
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
	applyVerdictFallback(res)
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
	applyVerdictFallback(kept)
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
	applyVerdictFallback(toCorrect)
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
	applyVerdictFallback(toIncorrect)
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
	applyVerdictFallback(kept)
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
	if got := overallConfidenceFor("patch is incorrect", []model.Finding{verifyOnly}); got != 0.66 {
		t.Fatalf("verification-fallback confidence = %.2f, want 0.66", got)
	}
	// No finalization, no verification → reviewer confidence.
	reviewOnly := model.Finding{Priority: intPtr(0), ConfidenceScore: 0.42}
	if got := overallConfidenceFor("patch is incorrect", []model.Finding{reviewOnly}); got != 0.42 {
		t.Fatalf("review-fallback confidence = %.2f, want 0.42", got)
	}
}
