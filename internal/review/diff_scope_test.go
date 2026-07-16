package review

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

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
