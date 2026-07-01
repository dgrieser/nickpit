package review

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
)

func TestRepairResponseCodeLocationsFixesWrongLineFromContent(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/demo.go", "package demo\n\nfunc Run() {\n\tfmt.Println(\"run\")\n}\n")
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})
	resp := &llm.ReviewResponse{Findings: []model.Finding{{
		Title:           "Fix run output",
		Body:            "body",
		ConfidenceScore: 0.85,
		Priority:        intPtr(1),
		CodeLocation: model.CodeLocation{
			FilePath:  "pkg/demo.go",
			LineRange: model.LineRange{Start: 4, End: 4, Count: 1},
			Language:  "python",
			Content:   "func Run() {",
		},
	}}}

	result := runCodeLocationRepair(t, engine, repoRoot, resp)

	if result.Repaired != 1 || len(result.RetryFields) != 0 {
		t.Fatalf("repair result = %+v, want one repair and no retry", result)
	}
	loc := resp.Findings[0].CodeLocation
	if loc.LineRange != (model.LineRange{Start: 3, End: 3, Count: 1}) {
		t.Fatalf("line range = %+v, want line 3", loc.LineRange)
	}
	if loc.Language != "go" || loc.Content != "func Run() {" {
		t.Fatalf("location = %+v, want repaired language/content", loc)
	}
}

func TestRepairResponseCodeLocationsRepairsSuggestionsAndLineRangeMirror(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/demo.go", "package demo\n\nfunc Run() {\n\tfmt.Println(\"run\")\n}\n")
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})
	resp := &llm.ReviewResponse{Findings: []model.Finding{{
		Title:           "Fix run output",
		Body:            "body",
		ConfidenceScore: 0.85,
		Priority:        intPtr(1),
		CodeLocation: model.CodeLocation{
			FilePath:  "pkg/demo.go",
			LineRange: model.LineRange{Start: 3, End: 5, Count: 3},
			Content:   "func Run() {\n\tfmt.Println(\"run\")\n}",
		},
		Suggestions: []model.Suggestion{{
			Body: "replace print",
			CodeLocation: model.CodeLocation{
				FilePath:  "pkg/demo.go",
				LineRange: model.LineRange{Start: 1, End: 1, Count: 1},
				Content:   "\tfmt.Println(\"run\")",
			},
		}},
	}}}

	result := runCodeLocationRepair(t, engine, repoRoot, resp)

	if result.Repaired != 2 || len(result.RetryFields) != 0 {
		t.Fatalf("repair result = %+v, want finding and suggestion repairs", result)
	}
	got := resp.Findings[0].Suggestions[0].CodeLocation.LineRange
	if got != (model.LineRange{Start: 4, End: 4, Count: 1}) {
		t.Fatalf("suggestion range = %+v, want line 4", got)
	}
	if resp.Findings[0].Suggestions[0].LineRange != got {
		t.Fatalf("legacy suggestion range = %+v, want mirror %+v", resp.Findings[0].Suggestions[0].LineRange, got)
	}
}

func TestRepairResponseCodeLocationsUsesFirstAndLastContentLines(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/demo.go", "func Run() {\n\tactual := 1\n\treturn actual\n}\n")
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})
	resp := &llm.ReviewResponse{Findings: []model.Finding{{
		Title:           "Fix run output",
		Body:            "body",
		ConfidenceScore: 0.85,
		Priority:        intPtr(1),
		CodeLocation: model.CodeLocation{
			FilePath: "pkg/demo.go",
			Content:  "func Run() {\n\twrong := 1\n}",
		},
	}}}

	result := runCodeLocationRepair(t, engine, repoRoot, resp)

	if result.Repaired != 1 || len(result.RetryFields) != 0 {
		t.Fatalf("repair result = %+v, want endpoint repair", result)
	}
	loc := resp.Findings[0].CodeLocation
	if loc.LineRange != (model.LineRange{Start: 1, End: 4, Count: 4}) {
		t.Fatalf("line range = %+v, want full function", loc.LineRange)
	}
	if !strings.Contains(loc.Content, "\tactual := 1") || !strings.Contains(loc.Content, "\treturn actual") {
		t.Fatalf("content = %q, want actual file slice", loc.Content)
	}
}

func TestRepairResponseCodeLocationsUsesIndividualLineOffset(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/demo.go", "package demo\n\nfunc Run() {\n\tactual := 1\n\treturn actual\n}\n")
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})
	resp := &llm.ReviewResponse{Findings: []model.Finding{{
		Title:           "Fix run output",
		Body:            "body",
		ConfidenceScore: 0.85,
		Priority:        intPtr(1),
		CodeLocation: model.CodeLocation{
			FilePath: "pkg/demo.go",
			Content:  "not first\n\treturn actual\nnot last",
		},
	}}}

	result := runCodeLocationRepair(t, engine, repoRoot, resp)

	if result.Repaired != 1 || len(result.RetryFields) != 0 {
		t.Fatalf("repair result = %+v, want line-offset repair", result)
	}
	loc := resp.Findings[0].CodeLocation
	if loc.LineRange != (model.LineRange{Start: 4, End: 6, Count: 3}) {
		t.Fatalf("line range = %+v, want inferred 4-6", loc.LineRange)
	}
	if loc.Content != "\tactual := 1\n\treturn actual\n}" {
		t.Fatalf("content = %q, want inferred file slice", loc.Content)
	}
}

func TestRepairResponseCodeLocationsFillsContentFromLineRange(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/demo.go", "line 1\nline 2\nline 3\n")
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})
	resp := &llm.ReviewResponse{Findings: []model.Finding{{
		Title:           "Fix run output",
		Body:            "body",
		ConfidenceScore: 0.85,
		Priority:        intPtr(1),
		CodeLocation: model.CodeLocation{
			FilePath:  "pkg/demo.go",
			LineRange: model.LineRange{Start: 2, End: 99, Count: 98},
		},
	}}}

	result := runCodeLocationRepair(t, engine, repoRoot, resp)

	if result.Repaired != 1 || len(result.RetryFields) != 0 {
		t.Fatalf("repair result = %+v, want range repair", result)
	}
	loc := resp.Findings[0].CodeLocation
	if loc.LineRange != (model.LineRange{Start: 2, End: 3, Count: 2}) {
		t.Fatalf("line range = %+v, want clamped 2-3", loc.LineRange)
	}
	if loc.Content != "line 2\nline 3" {
		t.Fatalf("content = %q, want clamped content", loc.Content)
	}
}

func TestRepairResponseCodeLocationsDefaultsMissingEndToStart(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/demo.go", "line 1\nline 2\nline 3\n")
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})
	resp := &llm.ReviewResponse{Findings: []model.Finding{{
		Title:           "Fix run output",
		Body:            "body",
		ConfidenceScore: 0.85,
		Priority:        intPtr(1),
		CodeLocation: model.CodeLocation{
			FilePath:  "pkg/demo.go",
			LineRange: model.LineRange{Start: 2},
		},
	}}}

	result := runCodeLocationRepair(t, engine, repoRoot, resp)

	if result.Repaired != 1 || len(result.RetryFields) != 0 {
		t.Fatalf("repair result = %+v, want range repair", result)
	}
	loc := resp.Findings[0].CodeLocation
	if loc.LineRange != (model.LineRange{Start: 2, End: 2, Count: 1}) {
		t.Fatalf("line range = %+v, want single line 2", loc.LineRange)
	}
	if loc.Content != "line 2" {
		t.Fatalf("content = %q, want single-line content", loc.Content)
	}
}

func TestRepairResponseCodeLocationsRequestsRetryForMissingAnchors(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/demo.go", "line 1\n")
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})
	resp := &llm.ReviewResponse{Findings: []model.Finding{
		{
			Title:           "Missing path",
			Body:            "body",
			ConfidenceScore: 0.85,
			Priority:        intPtr(1),
			CodeLocation:    model.CodeLocation{Content: "line 1"},
		},
		{
			Title:           "Missing anchor",
			Body:            "body",
			ConfidenceScore: 0.85,
			Priority:        intPtr(1),
			CodeLocation:    model.CodeLocation{FilePath: "pkg/demo.go"},
		},
	}}

	result := runCodeLocationRepair(t, engine, repoRoot, resp)

	if result.Repaired != 0 {
		t.Fatalf("repaired = %d, want 0", result.Repaired)
	}
	for _, field := range []string{"findings[0].code_location.file_path", "findings[1].code_location.content_or_line_range"} {
		if !slices.Contains(result.RetryFields, field) {
			t.Fatalf("retry fields = %v, want %s", result.RetryFields, field)
		}
	}
}

func runCodeLocationRepair(t *testing.T, engine *Engine, repoRoot string, resp *llm.ReviewResponse) codeLocationRepairResult {
	t.Helper()
	repair := engine.responseCodeLocationRepairer(repoRoot)
	if repair == nil {
		t.Fatal("repairer is nil")
	}
	return repair(context.Background(), resp)
}
