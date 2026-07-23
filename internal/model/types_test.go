package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAgentRunInvalidResponseDiagnosticJSONRoundTrip(t *testing.T) {
	run := AgentRun{
		Name:   "verdict",
		Role:   "verdict",
		Status: AgentRunStatusFailed,
		Error:  "model returned invalid JSON",
		InvalidResponse: &InvalidResponseDiagnostic{
			Reason:     "candidate repair failed",
			RawContent: "å malformed response",
		},
	}
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	var got AgentRun
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.InvalidResponse, run.InvalidResponse) {
		t.Fatalf("invalid response = %#v, want %#v", got.InvalidResponse, run.InvalidResponse)
	}
}

func TestReviewPromptPayloadJSONFieldOrder(t *testing.T) {
	cases := []struct {
		name    string
		format  DiffFormat
		diffKey string
		omitKey string
	}{
		{name: "git", format: DiffFormatGit, diffKey: "diff_files", omitKey: "diff_hunks"},
		{name: "git_json", format: DiffFormatGitJson, diffKey: "diff_hunks", omitKey: "diff_files"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := PromptPayloadFromContextWithDiffFormat(&ReviewContext{
				Identifier:  42,
				Repository:  RepositoryInfo{FullName: "owner/repo"},
				Title:       "Review title",
				Description: "Review description",
				Commits: []CommitSummary{{
					SHA:     "abc123",
					Message: "commit message",
					Author:  "author",
				}},
				ChangedFiles: []ChangedFile{{
					Path:      "main.go",
					Status:    FileModified,
					Additions: 1,
					Deletions: 1,
				}},
				DiffFiles: []DiffFile{{
					FilePath: "main.go",
					Content:  "diff --git a/main.go b/main.go",
				}},
				DiffHunks: []DiffHunk{{
					FilePath: "main.go",
					NewStart: 1,
					NewLines: 1,
					Content:  "@@ -1 +1 @@",
				}},
				Comments: []Comment{{
					Author: "reviewer",
					Body:   "comment body",
				}},
				SupplementalContext: []SupplementalFile{{
					Path:    "README.md",
					Content: "context",
				}},
				ToolchainVersions: []ToolchainVersion{{
					Language: "go",
					Version:  "1.25",
				}},
				OmittedSections: []string{"large_diff"},
			}, tc.format)

			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got := string(data)
			assertJSONKeyOrder(t, got, []string{
				"repository",
				"toolchain_versions",
				"changed_files",
				tc.diffKey,
				"commits",
				"identifier",
				"title",
				"description",
				"comments",
				"supplemental_context",
				"omitted_sections",
			})
			if strings.Contains(got, `"`+tc.omitKey+`":`) {
				t.Fatalf("payload should omit %q: %s", tc.omitKey, got)
			}
		})
	}
}

func assertJSONKeyOrder(t *testing.T, data string, keys []string) {
	t.Helper()
	last := -1
	for _, key := range keys {
		pattern := `"` + key + `":`
		idx := strings.Index(data, pattern)
		if idx < 0 {
			t.Fatalf("payload missing key %q: %s", key, data)
		}
		if idx <= last {
			t.Fatalf("key %q out of order in payload: %s", key, data)
		}
		last = idx
	}
}

func TestNormalizeConfidence(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0.85, 0.85},
		{0, 0},
		{1, 1},
		{95, 0.95},
		{100, 1},
		{150, 1},
		{-0.5, 0},
	}
	for _, c := range cases {
		if got := NormalizeConfidence(c.in); got != c.want {
			t.Errorf("NormalizeConfidence(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestKeepFirstSuggestionCapsEveryRepresentation(t *testing.T) {
	findings := []Finding{{
		Suggestions: []Suggestion{{Body: "top first"}, {Body: "top second"}},
		Finalization: &FindingFinalization{
			Suggestions: []Suggestion{{Body: "final first"}, {Body: "final second"}},
		},
		Summarization: &FindingSummarization{
			Suggestions: []Suggestion{{Body: "summary first"}, {Body: "summary second"}},
		},
	}}

	KeepFirstSuggestion(findings)

	if len(findings[0].Suggestions) != 1 || findings[0].Suggestions[0].Body != "top first" {
		t.Fatalf("top-level suggestions = %+v", findings[0].Suggestions)
	}
	if len(findings[0].Finalization.Suggestions) != 1 || findings[0].Finalization.Suggestions[0].Body != "final first" {
		t.Fatalf("finalization suggestions = %+v", findings[0].Finalization.Suggestions)
	}
	if len(findings[0].Summarization.Suggestions) != 1 || findings[0].Summarization.Suggestions[0].Body != "summary first" {
		t.Fatalf("summarization suggestions = %+v", findings[0].Summarization.Suggestions)
	}
}

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
		{ID: duplicateID, Verification: &FindingVerification{ID: duplicateID}},
		{ID: duplicateID},
	}

	// Reminted duplicates count as overwrites so callers gating a write-back
	// on the return value do not skip the deduplicated slice.
	if overwrote := EnsureFindingIDs(findings); overwrote != 2 {
		t.Fatalf("overwrote = %d, want 2", overwrote)
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
	if findings[1].Verification.ID != findings[1].ID {
		t.Fatalf("verification id = %q, want resynced to finding id %q",
			findings[1].Verification.ID, findings[1].ID)
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

func TestSuggestionUnmarshalSalvagesStringShorthand(t *testing.T) {
	var got Suggestion
	if err := json.Unmarshal([]byte(`"Add a regression test."`), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Body != "Add a regression test." {
		t.Fatalf("body = %q", got.Body)
	}
	if got.CodeLocation != (CodeLocation{}) || got.LineRange != (LineRange{}) {
		t.Fatalf("location = %+v line_range = %+v, want zero", got.CodeLocation, got.LineRange)
	}
}

func TestSuggestionUnmarshalSalvagesLegacyLineRangeObject(t *testing.T) {
	var got Suggestion
	if err := json.Unmarshal([]byte(`{"body":"fix","line_range":{"start":3,"end":5}}`), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := Suggestion{Body: "fix", LineRange: LineRange{Start: 3, End: 5, Count: 3}}
	if got != want {
		t.Fatalf("suggestion = %+v, want %+v", got, want)
	}
}

func TestLineRangeUnmarshalPopulatesMissingCount(t *testing.T) {
	var got LineRange
	if err := json.Unmarshal([]byte(`{"start":3,"end":5}`), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != (LineRange{Start: 3, End: 5, Count: 3}) {
		t.Fatalf("line range = %+v, want effective count", got)
	}
}

func TestSuggestionUnmarshalAcceptsCodeLocationObject(t *testing.T) {
	var got Suggestion
	data := `{"body":"fix","code_location":{"file_path":"f.go","line_range":{"start":3,"end":5,"count":3},"content":"old code"}}`
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := Suggestion{
		Body: "fix",
		CodeLocation: CodeLocation{
			FilePath:  "f.go",
			LineRange: LineRange{Start: 3, End: 5, Count: 3},
			Content:   "old code",
		},
		LineRange: LineRange{Start: 3, End: 5, Count: 3},
	}
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
		ID:              "id-1",         // not in keys → preserved
		Verdict:         VerdictRefuted, // in keys → overwritten with src value
		Priority:        0,              // in keys → overwritten with src zero value
		ConfidenceScore: 0.0,            // in keys → overwritten with src zero value
		Remarks:         "first",        // not in keys → preserved
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
