package output

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/testutil"
)

func TestTerminalFormatter(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, false)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{Title: "Problem", Body: "Description", ConfidenceScore: 0.82, Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 10, End: 12}}},
		},
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "Summary text",
		OverallConfidenceScore: 0.77,
		TokensUsed: model.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 4,
			TotalTokens:      14,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGolden(t, buf.String(), filepath.Join("..", "..", "testdata", "golden", "TestTerminalFormatter.txt"))
}

func TestJSONFormatterAlwaysIncludesToolLimitsAndDuplicates(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings:           []model.Finding{},
		MaxToolCalls:       0,
		DuplicateToolCalls: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if got, ok := payload["max_tool_calls"]; !ok || got != float64(0) {
		t.Fatalf("max_tool_calls = %#v, present=%t", got, ok)
	}
	if got, ok := payload["duplicate_tool_calls"]; !ok || got != float64(0) {
		t.Fatalf("duplicate_tool_calls = %#v, present=%t", got, ok)
	}
}

func intPtr(v int) *int {
	return &v
}
