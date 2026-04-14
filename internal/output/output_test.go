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
			{Severity: model.SeverityWarning, Category: "bug", FilePath: "a.go", StartLine: 10, EndLine: 12, Title: "Problem", Description: "Description", Confidence: 0.82},
		},
		Summary: "Summary text",
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
