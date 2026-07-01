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

func TestValidateResponseCodeLocationsAcceptsExactFindLinesLocations(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/demo.go", "package demo\n\nfunc Run() {\n\tfmt.Println(\"run\")\n}\n")
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})

	resp := &llm.ReviewResponse{
		Findings: []model.Finding{{
			Title:           "Fix run output",
			Body:            "body",
			ConfidenceScore: 0.85,
			Priority:        intPtr(1),
			CodeLocation: model.CodeLocation{
				FilePath:  "pkg/demo.go",
				LineRange: model.LineRange{Start: 3, End: 5, Count: 3},
				Language:  "go",
				Content:   "func Run() {\n\tfmt.Println(\"run\")\n}",
			},
			Suggestions: []model.Suggestion{{
				Body: "replace print",
				CodeLocation: model.CodeLocation{
					FilePath:  "pkg/demo.go",
					LineRange: model.LineRange{Start: 4, End: 4, Count: 1},
					Language:  "go",
					Content:   "\tfmt.Println(\"run\")",
				},
			}},
		}},
	}

	if invalid := engine.validateResponseCodeLocations(context.Background(), repoRoot, resp); invalid != nil {
		t.Fatalf("validateResponseCodeLocations returned %v, want nil", invalid)
	}
}

func TestValidateResponseCodeLocationsRejectsWrongFindingLine(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/demo.go", "package demo\n\nfunc Run() {\n\tfmt.Println(\"run\")\n}\n")
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})

	resp := &llm.ReviewResponse{
		Findings: []model.Finding{{
			Title:           "Fix run output",
			Body:            "body",
			ConfidenceScore: 0.85,
			Priority:        intPtr(1),
			CodeLocation: model.CodeLocation{
				FilePath:  "pkg/demo.go",
				LineRange: model.LineRange{Start: 4, End: 4, Count: 1},
				Language:  "go",
				Content:   "func Run() {",
			},
		}},
	}

	invalid := engine.validateResponseCodeLocations(context.Background(), repoRoot, resp)
	if invalid == nil {
		t.Fatal("validateResponseCodeLocations returned nil, want invalid response")
	}
	if !invalid.ValidationFailure {
		t.Fatal("ValidationFailure = false, want true")
	}
	if !slices.Contains(invalid.MissingFields, "findings[0].code_location") {
		t.Fatalf("MissingFields = %v, want findings[0].code_location", invalid.MissingFields)
	}
	rendered, err := renderPromptFile(invalid.RetryGuidanceTemplate, invalid.RetryGuidanceData)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "calling the `find_lines` tool") || !strings.Contains(rendered, "findings[0].code_location") {
		t.Fatalf("retry guidance missing find_lines/location details:\n%s", rendered)
	}
}

func TestValidateResponseCodeLocationsRejectsSuggestionContentMismatch(t *testing.T) {
	repoRoot := t.TempDir()
	writeRepoFile(t, repoRoot, "pkg/demo.go", "package demo\n\nfunc Run() {\n\tfmt.Println(\"run\")\n}\n")
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})

	resp := &llm.ReviewResponse{
		Findings: []model.Finding{{
			Title:           "Fix run output",
			Body:            "body",
			ConfidenceScore: 0.85,
			Priority:        intPtr(1),
			CodeLocation: model.CodeLocation{
				FilePath:  "pkg/demo.go",
				LineRange: model.LineRange{Start: 3, End: 5, Count: 3},
				Language:  "go",
				Content:   "func Run() {\n\tfmt.Println(\"run\")\n}",
			},
			Suggestions: []model.Suggestion{{
				Body: "replace print",
				CodeLocation: model.CodeLocation{
					FilePath:  "pkg/demo.go",
					LineRange: model.LineRange{Start: 4, End: 4, Count: 1},
					Language:  "go",
					Content:   "\tfmt.Println(\"run\") ",
				},
			}},
		}},
	}

	invalid := engine.validateResponseCodeLocations(context.Background(), repoRoot, resp)
	if invalid == nil {
		t.Fatal("validateResponseCodeLocations returned nil, want invalid response")
	}
	if !slices.Contains(invalid.MissingFields, "findings[0].suggestions[0].code_location") {
		t.Fatalf("MissingFields = %v, want findings[0].suggestions[0].code_location", invalid.MissingFields)
	}
	if !strings.Contains(invalid.Reason, "content differs from the find_lines match") {
		t.Fatalf("Reason = %q, want content mismatch", invalid.Reason)
	}
}
