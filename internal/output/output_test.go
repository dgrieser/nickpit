package output

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
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

func TestTerminalFormatterRendersVerification(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, false)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "Real bug", Body: "B1", ConfidenceScore: 0.8, Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification: &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.92, Remarks: "confirmed"},
			},
			{
				Title: "Stylistic", Body: "B2", ConfidenceScore: 0.7, Priority: intPtr(2),
				CodeLocation: model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}},
				Verification: &model.FindingVerification{Valid: false, Priority: 3, ConfidenceScore: 0.88, Remarks: "not reachable"},
			},
		},
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "ok",
		OverallConfidenceScore: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "verified: 1 valid, 1 invalid") {
		t.Fatalf("missing header tally:\n%s", out)
	}
	if !strings.Contains(out, "[ok] verified") {
		t.Fatalf("missing valid glyph:\n%s", out)
	}
	if !strings.Contains(out, "[bad] invalid") {
		t.Fatalf("missing invalid glyph:\n%s", out)
	}
	if !strings.Contains(out, "P2→P3") {
		t.Fatalf("missing priority arrow:\n%s", out)
	}
	if !strings.Contains(out, "remark: confirmed") {
		t.Fatalf("missing valid remark:\n%s", out)
	}
}

func TestTerminalFormatterHideInvalid(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, false).WithHideInvalid(true)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "Real bug", Body: "B1", ConfidenceScore: 0.8, Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification: &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.92, Remarks: "ok"},
			},
			{
				Title: "Should be hidden", Body: "B2", ConfidenceScore: 0.7, Priority: intPtr(2),
				CodeLocation: model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}},
				Verification: &model.FindingVerification{Valid: false, Priority: 3, ConfidenceScore: 0.88, Remarks: "no"},
			},
		},
		OverallCorrectness: "patch is correct",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "Should be hidden") {
		t.Fatalf("invalid finding leaked:\n%s", out)
	}
	if !strings.Contains(out, "Real bug") {
		t.Fatalf("valid finding missing:\n%s", out)
	}
}

func TestJSONFormatterIncludesVerification(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "x", Body: "y", Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification: &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.5, Remarks: "ok"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	findings := payload["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("findings len = %d", len(findings))
	}
	first := findings[0].(map[string]any)
	v, ok := first["verification"].(map[string]any)
	if !ok {
		t.Fatalf("verification missing: %#v", first)
	}
	if v["valid"] != true {
		t.Fatalf("verification.valid = %#v", v["valid"])
	}
	if v["remarks"] != "ok" {
		t.Fatalf("verification.remarks = %#v", v["remarks"])
	}
}

func TestJSONFormatterOmitsVerificationWhenNil(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{Title: "x", Body: "y", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	first := payload["findings"].([]any)[0].(map[string]any)
	if _, ok := first["verification"]; ok {
		t.Fatalf("verification should be omitted: %#v", first)
	}
}
