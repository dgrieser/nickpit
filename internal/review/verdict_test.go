package review

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

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
