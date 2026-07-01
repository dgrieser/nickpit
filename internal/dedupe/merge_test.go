package dedupe

import (
	"math"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

func intPtr(v int) *int { return &v }

func TestMergeFindingsBaseSelectionAndNoisyOr(t *testing.T) {
	a := finding("Weaker", "short", "a.sh", 10, 12)
	a.ID = "id-a"
	a.ConfidenceScore = 0.6
	b := finding("Stronger", "much longer body text", "a.sh", 8, 11)
	b.ID = "id-b"
	b.ConfidenceScore = 0.7

	out := MergeFindings(a, b)
	if out.ID != "id-b" || out.Title != "Stronger" {
		t.Fatalf("base = %s/%s, want higher-confidence finding id-b/Stronger", out.ID, out.Title)
	}
	want := 1 - (1-0.6)*(1-0.7)
	if math.Abs(out.ConfidenceScore-want) > 1e-9 {
		t.Fatalf("confidence = %v, want noisy-or %v", out.ConfidenceScore, want)
	}
}

func TestMergeFindingsConfidenceCap(t *testing.T) {
	a := finding("A", "body", "a.sh", 1, 1)
	a.ConfidenceScore = 0.95
	b := finding("B", "body", "a.sh", 1, 1)
	b.ConfidenceScore = 0.95
	if got := MergeFindings(a, b).ConfidenceScore; got != 0.99 {
		t.Fatalf("confidence = %v, want capped 0.99", got)
	}
}

func TestMergeFindingsExtendsRange(t *testing.T) {
	a := finding("A", "longer body", "a.sh", 10, 12)
	a.ConfidenceScore = 0.8
	b := finding("B", "short", "a.sh", 8, 11)
	b.ConfidenceScore = 0.5

	if got := MergeFindings(a, b).CodeLocation.LineRange; got != (model.LineRange{Start: 8, End: 12}) {
		t.Fatalf("range = %+v, want union 8-12", got)
	}

	b.CodeLocation.LineRange = model.LineRange{}
	if got := MergeFindings(a, b).CodeLocation.LineRange; got != (model.LineRange{Start: 10, End: 12}) {
		t.Fatalf("range = %+v, want base range when other side unknown", got)
	}
}

func TestMergeFindingsPreservesFindLinesLocation(t *testing.T) {
	a := finding("A", "longer body", "a.sh", 10, 12)
	a.ConfidenceScore = 0.8
	a.CodeLocation.Content = "exact base snippet"
	b := finding("B", "short", "a.sh", 8, 11)
	b.ConfidenceScore = 0.5
	b.CodeLocation.Content = "exact other snippet"

	got := MergeFindings(a, b).CodeLocation
	if got.LineRange != (model.LineRange{Start: 10, End: 12}) || got.Content != "exact base snippet" {
		t.Fatalf("location = %+v, want exact base find_lines location preserved", got)
	}
}

func TestMergeFindingsKeepsOtherFindLinesLocationWhenBaseIsLegacy(t *testing.T) {
	a := finding("A", "longer body", "a.sh", 10, 12)
	a.ConfidenceScore = 0.8
	b := finding("B", "short", "a.sh", 8, 11)
	b.ConfidenceScore = 0.5
	b.CodeLocation.Content = "exact other snippet"
	b.CodeLocation.Language = "sh"
	b.CodeLocation.LineRange.Count = 4

	got := MergeFindings(a, b).CodeLocation
	if got != b.CodeLocation {
		t.Fatalf("location = %+v, want other exact find_lines location %+v", got, b.CodeLocation)
	}
}

func TestMergeFindingsMostCriticalPriority(t *testing.T) {
	a := finding("A", "body", "a.sh", 1, 1)
	a.ConfidenceScore = 0.9
	a.Priority = intPtr(3)
	b := finding("B", "body", "a.sh", 1, 1)
	b.Priority = intPtr(1)

	if got := MergeFindings(a, b).Priority; got == nil || *got != 1 {
		t.Fatalf("priority = %v, want most critical 1", got)
	}

	a.Priority, b.Priority = nil, nil
	if got := MergeFindings(a, b).Priority; got != nil {
		t.Fatalf("priority = %v, want nil when both unset", got)
	}
}

func TestMergeFindingsIncludesAllSuggestions(t *testing.T) {
	a := finding("A", "body", "a.sh", 1, 1)
	a.ConfidenceScore = 0.9
	a.Suggestions = []model.Suggestion{{Body: "short fix"}}
	b := finding("B", "body", "a.sh", 1, 1)
	b.Suggestions = []model.Suggestion{{Body: "a far more detailed and actionable fix description"}}

	out := MergeFindings(a, b)
	if len(out.Suggestions) != 2 {
		t.Fatalf("suggestions = %+v, want both sides included", out.Suggestions)
	}
	if out.Suggestions[0].Body != a.Suggestions[0].Body || out.Suggestions[1].Body != b.Suggestions[0].Body {
		t.Fatalf("suggestions = %+v, want base set first then other set", out.Suggestions)
	}

	b.Suggestions = nil
	out = MergeFindings(a, b)
	if len(out.Suggestions) != 1 || out.Suggestions[0].Body != a.Suggestions[0].Body {
		t.Fatalf("suggestions = %+v, want base set untouched when other side empty", out.Suggestions)
	}
}

func TestMergeFindingsVerificationFollowsHighestConfidence(t *testing.T) {
	a := finding("A", "body", "a.sh", 1, 1)
	a.ID = "id-a"
	a.ConfidenceScore = 0.9
	a.Verification = &model.FindingVerification{
		ID: "id-a", Verdict: "confirmed", Priority: 2, ConfidenceScore: 0.8, Remarks: "base remarks",
	}
	b := finding("B", "body", "a.sh", 1, 1)
	b.Verification = &model.FindingVerification{
		ID: "id-b", Verdict: "uncertain", Priority: 1, ConfidenceScore: 0.5, Remarks: "other remarks",
	}

	v := MergeFindings(a, b).Verification
	if v == nil {
		t.Fatal("verification dropped")
		return
	}
	if v.ID != "id-a" || v.Verdict != "confirmed" || v.Remarks != "base remarks" {
		t.Fatalf("verification = %s/%s/%q, want verdict and remarks from higher-confidence side", v.ID, v.Verdict, v.Remarks)
	}
	if v.Priority != 1 {
		t.Fatalf("verification priority = %d, want most critical 1", v.Priority)
	}
	if v.ConfidenceScore != 0.8 {
		t.Fatalf("verification confidence = %v, want highest 0.8", v.ConfidenceScore)
	}

	// Other side carries the stronger verification: its verdict and remarks
	// win even though the base finding provides identity and text.
	b.Verification.ConfidenceScore = 0.95
	v = MergeFindings(a, b).Verification
	if v.ID != "id-a" {
		t.Fatalf("verification ID = %s, want surviving finding id-a", v.ID)
	}
	if v.Verdict != "uncertain" || v.Remarks != "other remarks" {
		t.Fatalf("verification = %s/%q, want higher-confidence side's verdict and remarks", v.Verdict, v.Remarks)
	}
	if v.ConfidenceScore != 0.95 {
		t.Fatalf("verification confidence = %v, want highest 0.95", v.ConfidenceScore)
	}
}

func TestMergeFindingsNilVerificationSides(t *testing.T) {
	a := finding("A", "body", "a.sh", 1, 1)
	a.ID = "id-a"
	a.ConfidenceScore = 0.9
	b := finding("B", "body", "a.sh", 1, 1)
	b.Verification = &model.FindingVerification{ID: "id-b", Verdict: "confirmed", Remarks: "only one side"}

	v := MergeFindings(a, b).Verification
	if v == nil || v.Remarks != "only one side" {
		t.Fatalf("verification = %+v, want the non-nil side", v)
	}
	if v.ID != "id-a" {
		t.Fatalf("verification ID = %s, want re-anchored to surviving finding id-a", v.ID)
	}

	b.Verification = nil
	if got := MergeFindings(a, b).Verification; got != nil {
		t.Fatalf("verification = %+v, want nil when both sides nil", got)
	}
}

func TestFoldCluster(t *testing.T) {
	if got := FoldCluster(nil); got.ID != "" {
		t.Fatalf("FoldCluster(nil) = %+v, want zero finding", got)
	}

	low := finding("Low", "low body", "a.sh", 5, 6)
	low.ID = "low"
	low.ConfidenceScore = 0.3
	mid := finding("Mid", "mid body", "a.sh", 1, 2)
	mid.ID = "mid"
	mid.ConfidenceScore = 0.5
	high := finding("High", "high body", "a.sh", 3, 4)
	high.ID = "high"
	high.ConfidenceScore = 0.8

	out := FoldCluster([]model.Finding{low, mid, high})
	if out.ID != "high" {
		t.Fatalf("base = %s, want highest-confidence finding", out.ID)
	}
	if out.CodeLocation.LineRange != (model.LineRange{Start: 1, End: 6}) {
		t.Fatalf("range = %+v, want union 1-6 across all members", out.CodeLocation.LineRange)
	}
	want := 1 - (1-0.8)*(1-0.5)*(1-0.3)
	if math.Abs(out.ConfidenceScore-want) > 1e-9 {
		t.Fatalf("confidence = %v, want folded noisy-or %v", out.ConfidenceScore, want)
	}
}
