package review

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

func TestBuildDiscussContextIncludesReviewAndDiff(t *testing.T) {
	e := &Engine{}
	reviewCtx := &model.ReviewContext{
		Mode:       model.ModeGitLab,
		Identifier: 7,
		Repository: model.RepositoryInfo{FullName: "grp/proj", BaseRef: "main", HeadRef: "feat"},
		Diff:       "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
	}
	p := 1
	result := &model.ReviewResult{
		ReviewID:               "rev-1",
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "risky change",
		OverallConfidenceScore: 0.7,
		Findings: []model.Finding{
			{ID: "f1", Title: "Bug", Body: "explodes", Priority: &p, CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}, Suggestions: []model.Suggestion{{Body: "guard it"}}},
		},
	}

	got, err := e.buildDiscussContext(reviewCtx, result, "f1", false, model.DiffFormat("git"))
	if err != nil {
		t.Fatalf("buildDiscussContext: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("context not valid JSON: %v\n%s", err, got)
	}
	if decoded["focus_finding_id"] != "f1" {
		t.Fatalf("focus_finding_id missing: %v", decoded["focus_finding_id"])
	}
	if _, ok := decoded["diff"]; !ok {
		t.Fatalf("raw diff missing from context")
	}
	review, ok := decoded["review"].(map[string]any)
	if !ok {
		t.Fatalf("review object missing: %T", decoded["review"])
	}
	if review["overall_correctness"] != "patch is incorrect" {
		t.Fatalf("overall not carried: %v", review["overall_correctness"])
	}
	findings, ok := review["findings"].([]any)
	if !ok || len(findings) != 1 {
		t.Fatalf("findings not carried: %v", review["findings"])
	}
	f0 := findings[0].(map[string]any)
	if f0["body"] != "explodes" {
		t.Fatalf("finding body not carried: %v", f0["body"])
	}
	if _, ok := f0["suggestions"]; !ok {
		t.Fatalf("suggestions should be present when not disabled")
	}
}

func TestBuildDiscussContextDropsSuggestionsWhenDisabled(t *testing.T) {
	e := &Engine{}
	reviewCtx := &model.ReviewContext{Repository: model.RepositoryInfo{FullName: "g/p"}}
	result := &model.ReviewResult{
		Findings: []model.Finding{
			{ID: "f1", Title: "T", Suggestions: []model.Suggestion{{Body: "x"}}},
		},
	}
	got, err := e.buildDiscussContext(reviewCtx, result, "", true, model.DiffFormat("git"))
	if err != nil {
		t.Fatalf("buildDiscussContext: %v", err)
	}
	if strings.Contains(got, "\"suggestions\"") {
		t.Fatalf("suggestions should be stripped: %s", got)
	}
	// The original result must not be mutated by stripping.
	if len(result.Findings[0].Suggestions) != 1 {
		t.Fatalf("original findings were mutated")
	}
}

func TestDiscussOpener(t *testing.T) {
	p := 2
	result := &model.ReviewResult{
		Findings: []model.Finding{
			{ID: "f1", Title: "Null deref", Priority: &p, CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 10}}},
		},
	}
	opener := discussOpener(result, "f1")
	if !strings.Contains(opener, "Null deref") || !strings.Contains(opener, "a.go:10") || !strings.Contains(opener, "P2") {
		t.Fatalf("opener missing details: %q", opener)
	}
	if discussOpener(result, "missing") != "" {
		t.Fatalf("opener should be empty for unknown finding id")
	}
}
