package model

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestEnsureFindingIDsPreservesUniqueValidIDs(t *testing.T) {
	findings := []Finding{
		{ID: "11111111-1111-4111-8111-111111111111"},
		{ID: "22222222-2222-4222-8222-222222222222"},
	}

	if overwrote := EnsureFindingIDs(findings); overwrote != 0 {
		t.Fatalf("overwrote = %d, want 0", overwrote)
	}

	if findings[0].ID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("first id = %q", findings[0].ID)
	}
	if findings[1].ID != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("second id = %q", findings[1].ID)
	}
}

func TestEnsureFindingIDsRegeneratesInvalidIDs(t *testing.T) {
	findings := []Finding{{ID: "not-a-uuid"}}

	if overwrote := EnsureFindingIDs(findings); overwrote != 1 {
		t.Fatalf("overwrote = %d, want 1", overwrote)
	}

	if _, err := uuid.Parse(findings[0].ID); err != nil {
		t.Fatalf("id = %q, want valid uuid", findings[0].ID)
	}
	if findings[0].ID == "not-a-uuid" {
		t.Fatalf("id was not regenerated")
	}
}

func TestEnsureFindingIDsRegeneratesDuplicateValidIDs(t *testing.T) {
	const duplicateID = "11111111-1111-4111-8111-111111111111"
	findings := []Finding{
		{ID: duplicateID},
		{ID: duplicateID},
		{ID: duplicateID},
	}

	if overwrote := EnsureFindingIDs(findings); overwrote != 0 {
		t.Fatalf("overwrote = %d, want 0", overwrote)
	}

	seen := map[string]struct{}{}
	for i, finding := range findings {
		if _, err := uuid.Parse(finding.ID); err != nil {
			t.Fatalf("finding %d id = %q, want valid uuid", i, finding.ID)
		}
		if _, ok := seen[finding.ID]; ok {
			t.Fatalf("finding %d kept duplicate id %q", i, finding.ID)
		}
		seen[finding.ID] = struct{}{}
	}
	if findings[0].ID != duplicateID {
		t.Fatalf("first id = %q, want preserved duplicate source id", findings[0].ID)
	}
}

func TestEnsureFindingIDReportsOnlyNonEmptyInvalidOverwrite(t *testing.T) {
	empty := Finding{}
	if overwrote := EnsureFindingID(&empty); overwrote {
		t.Fatal("empty ID generation should not report overwrite")
	}

	invalid := Finding{ID: "not-a-uuid"}
	if overwrote := EnsureFindingID(&invalid); !overwrote {
		t.Fatal("invalid non-empty ID should report overwrite")
	}

	valid := Finding{ID: "11111111-1111-4111-8111-111111111111"}
	if overwrote := EnsureFindingID(&valid); overwrote {
		t.Fatal("valid ID should not report overwrite")
	}
}

func TestSuggestionUnmarshalAcceptsStringShorthand(t *testing.T) {
	var got Suggestion
	if err := json.Unmarshal([]byte(`"Add a regression test."`), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Body != "Add a regression test." {
		t.Fatalf("body = %q", got.Body)
	}
	if got.LineRange != (LineRange{}) {
		t.Fatalf("line range = %+v, want zero", got.LineRange)
	}
}

func TestSuggestionUnmarshalAcceptsObject(t *testing.T) {
	var got Suggestion
	if err := json.Unmarshal([]byte(`{"body":"fix","line_range":{"start":3,"end":5}}`), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := Suggestion{Body: "fix", LineRange: LineRange{Start: 3, End: 5}}
	if got != want {
		t.Fatalf("suggestion = %+v, want %+v", got, want)
	}
}

func TestFindingVerificationMergeFromKeyAware(t *testing.T) {
	dst := FindingVerification{
		ID:              "id-1",
		Verdict:         VerdictConfirmed,
		Priority:        2,
		ConfidenceScore: 0.9,
		Remarks:         "first",
	}
	src := FindingVerification{
		ID:              "id-2",
		Verdict:         VerdictRefuted,
		Priority:        0,
		ConfidenceScore: 0.0,
		Remarks:         "second",
	}
	keys := map[string]bool{"verdict": true, "priority": true, "confidence_score": true}
	claimed, err := dst.MergeFrom(&src, keys)
	if err != nil {
		t.Fatalf("MergeFrom: %v", err)
	}
	if !claimed {
		t.Fatalf("expected claimed=true")
	}
	want := FindingVerification{
		ID:              "id-1",             // not in keys → preserved
		Verdict:         VerdictRefuted,     // in keys → overwritten with src value
		Priority:        0,                  // in keys → overwritten with src zero value
		ConfidenceScore: 0.0,                // in keys → overwritten with src zero value
		Remarks:         "first",            // not in keys → preserved
	}
	if dst != want {
		t.Fatalf("dst = %+v, want %+v", dst, want)
	}
}

func TestFindingVerificationMergeFromNoKeysReturnsUnclaimed(t *testing.T) {
	dst := FindingVerification{ID: "keep"}
	src := FindingVerification{ID: "discard"}
	claimed, err := dst.MergeFrom(&src, map[string]bool{})
	if err != nil {
		t.Fatalf("MergeFrom: %v", err)
	}
	if claimed {
		t.Fatalf("expected claimed=false for empty keys")
	}
	if dst.ID != "keep" {
		t.Fatalf("dst mutated: %+v", dst)
	}
}
