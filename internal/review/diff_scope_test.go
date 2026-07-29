package review

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

func TestAllowedDiffCodeLocationsAreCompleteCodeLocationJSON(t *testing.T) {
	hunks := []model.DiffHunk{{
		FilePath: "pkg/demo.go",
		Language: "go",
		OldStart: 10,
		OldLines: 2,
		NewStart: 10,
		NewLines: 2,
		Content:  " context\n-old()\n+new()\n",
	}}

	allowed := allowedDiffCodeLocations(hunks)
	if len(allowed) != 2 {
		t.Fatalf("allowed locations = %#v, want old and new code_location", allowed)
	}
	contents := map[string]bool{}
	for _, got := range allowed {
		if got.FilePath != "pkg/demo.go" ||
			got.LineRange != (model.LineRange{Start: 10, End: 11, Count: 2}) ||
			got.Language != "go" {
			t.Fatalf("code_location = %#v", got)
		}
		contents[got.Content] = true
	}
	if !contents["context\nold()"] || !contents["context\nnew()"] {
		t.Fatalf("allowed contents = %#v, want old and new sides", contents)
	}

	encoded, err := json.Marshal(allowed)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []model.CodeLocation
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("allowed windows are not code_location JSON: %v", err)
	}
	jsonText := string(encoded)
	for _, field := range []string{`"file_path"`, `"line_range"`, `"start"`, `"end"`, `"count"`, `"language"`, `"content"`} {
		if !strings.Contains(jsonText, field) {
			t.Fatalf("allowed JSON %s missing %s", jsonText, field)
		}
	}
}

func TestAllowedDiffCodeLocationsSkipsEmptySides(t *testing.T) {
	hunks := []model.DiffHunk{
		{FilePath: "added.go", NewStart: 4, NewLines: 1, Content: "+added()\n"},
		{FilePath: "deleted.go", OldStart: 7, OldLines: 1, Content: "-deleted()\n"},
	}
	allowed := allowedDiffCodeLocations(hunks)
	if len(allowed) != 2 {
		t.Fatalf("allowed locations = %#v, want one non-empty side per hunk", allowed)
	}
}

func TestCodeLocationOverlapsDiffAcceptsAnyOldOrNewSideIntersection(t *testing.T) {
	hunks := []model.DiffHunk{
		{FilePath: "f.go", OldStart: 10, OldLines: 3, NewStart: 10, NewLines: 2},
		{FilePath: "f.go", OldStart: 30, OldLines: 2, NewStart: 29, NewLines: 3},
	}
	tests := []struct {
		name  string
		path  string
		start int
		end   int
		want  bool
	}{
		{name: "new-side line", path: "f.go", start: 11, end: 11, want: true},
		{name: "deleted-only old-side line", path: "f.go", start: 12, end: 12, want: true},
		{name: "partial overlap", path: "f.go", start: 1, end: 10, want: true},
		{name: "range spans hunk gap", path: "f.go", start: 11, end: 30, want: true},
		{name: "wholly between hunks", path: "f.go", start: 13, end: 28, want: false},
		{name: "outside after hunks", path: "f.go", start: 40, end: 50, want: false},
		{name: "wrong file", path: "other.go", start: 10, end: 10, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := model.CodeLocation{FilePath: tt.path, LineRange: model.LineRange{Start: tt.start, End: tt.end}}
			if got := codeLocationOverlapsDiff(loc, hunks); got != tt.want {
				t.Fatalf("codeLocationOverlapsDiff() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodeLocationOverlapsLegacyContentOnlyHunk(t *testing.T) {
	hunks := []model.DiffHunk{{
		FilePath: "f.go",
		OldStart: 5,
		NewStart: 5,
		Content:  " old\n-removed\n+added\n context\n",
	}}
	for _, line := range []int{5, 6, 7} {
		loc := model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: line, End: line}}
		if !codeLocationOverlapsDiff(loc, hunks) {
			t.Fatalf("line %d should overlap legacy hunk", line)
		}
	}
}

func TestPipelineAssembleAppliesFinalDiffScopeSafeguard(t *testing.T) {
	ctx := &model.ReviewContext{DiffScopeHunks: []model.DiffHunk{{
		FilePath: "f.go",
		OldStart: 10,
		OldLines: 1,
		NewStart: 10,
		NewLines: 1,
	}}}
	st := newPipelineState(ctx, nil)
	st.result = &model.ReviewResult{
		Findings: []model.Finding{
			{Title: "inside", CodeLocation: model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 10, End: 10}}},
			{Title: "outside", CodeLocation: model.CodeLocation{FilePath: "other.go", LineRange: model.LineRange{Start: 1, End: 1}}},
		},
		OverallCorrectness: "patch is incorrect",
	}
	pipeline := &Pipeline{engine: &Engine{}}
	result := pipeline.assemble(st, model.ReviewRequest{})
	if len(result.Findings) != 1 || result.Findings[0].Title != "inside" {
		t.Fatalf("findings = %#v", result.Findings)
	}
}

func TestPrepareFindingsForVerificationAggregatesOutOfDiffWarnings(t *testing.T) {
	ctx := &model.ReviewContext{DiffScopeHunks: []model.DiffHunk{{
		FilePath: "f.go",
		OldStart: 10,
		OldLines: 1,
		NewStart: 10,
		NewLines: 1,
		Content:  " changed()\n",
	}}}
	results := []agentResult{{
		run: model.AgentRun{Name: "Security"},
		resp: &llm.ReviewResponse{Findings: []model.Finding{
			{Title: "outside one", CodeLocation: model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 1, End: 1}, Content: "one()"}},
			{Title: "outside two", CodeLocation: model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 2, End: 2}, Content: "two()"}},
		}},
	}}
	engine := NewEngine(nil, nil, nil, config.Profile{})

	warnings := engine.prepareFindingsForVerification(context.Background(), ctx, results, model.ReviewRequest{})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Dropped 2 out-of-diff finding(s) from Security") {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(results[0].resp.Findings) != 0 {
		t.Fatalf("findings = %#v, want both dropped", results[0].resp.Findings)
	}
}
