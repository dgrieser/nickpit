package output

import (
	"bytes"
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
			{Title: "[P2] Problem", Body: "Description", ConfidenceScore: 0.82, Priority: intPtr(2), CodeLocation: model.CodeLocation{AbsoluteFilePath: "a.go", LineRange: model.LineRange{Start: 10, End: 12}}},
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

func intPtr(v int) *int {
	return &v
}
