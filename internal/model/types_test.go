package model

import (
	"testing"

	"github.com/google/uuid"
)

func TestEnsureFindingIDsPreservesUniqueValidIDs(t *testing.T) {
	findings := []Finding{
		{ID: "11111111-1111-4111-8111-111111111111"},
		{ID: "22222222-2222-4222-8222-222222222222"},
	}

	EnsureFindingIDs(findings)

	if findings[0].ID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("first id = %q", findings[0].ID)
	}
	if findings[1].ID != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("second id = %q", findings[1].ID)
	}
}

func TestEnsureFindingIDsRegeneratesInvalidIDs(t *testing.T) {
	findings := []Finding{{ID: "not-a-uuid"}}

	EnsureFindingIDs(findings)

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

	EnsureFindingIDs(findings)

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
