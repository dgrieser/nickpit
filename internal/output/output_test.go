package output

import (
	"bytes"
	"encoding/json"
	"os"
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
			{ID: "11111111-1111-4111-8111-111111111111", Title: "Problem", Body: "Description", ConfidenceScore: 0.82, Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 10, End: 12}}},
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
		Findings: []model.Finding{},
		AgentRuns: []model.AgentRun{
			{Name: "reviewer", Role: "reviewer", MaxToolCalls: 0, DuplicateToolCalls: 0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["max_tool_calls"]; ok {
		t.Fatalf("root max_tool_calls should be omitted: %#v", payload["max_tool_calls"])
	}
	runs, ok := payload["agent_runs"].([]any)
	if !ok || len(runs) != 1 {
		t.Fatalf("agent_runs = %#v", payload["agent_runs"])
	}
	run, ok := runs[0].(map[string]any)
	if !ok {
		t.Fatalf("agent run = %#v", runs[0])
	}
	if got, ok := run["max_tool_calls"]; !ok || got != float64(0) {
		t.Fatalf("agent max_tool_calls = %#v, present=%t", got, ok)
	}
	if got, ok := run["duplicate_tool_calls"]; !ok || got != float64(0) {
		t.Fatalf("agent duplicate_tool_calls = %#v, present=%t", got, ok)
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
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.92, Remarks: "confirmed"},
			},
			{
				Title: "Stylistic", Body: "B2", ConfidenceScore: 0.7, Priority: intPtr(2),
				CodeLocation: model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}},
				Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 3, ConfidenceScore: 0.88, Remarks: "not reachable"},
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
	if !strings.Contains(out, "verified: 1 confirmed, 1 refuted, 0 unverified") {
		t.Fatalf("missing header tally:\n%s", out)
	}
	if !strings.Contains(out, "[ok] confirmed") {
		t.Fatalf("missing confirmed glyph:\n%s", out)
	}
	if !strings.Contains(out, "[bad] refuted") {
		t.Fatalf("missing refuted glyph:\n%s", out)
	}
	if !strings.Contains(out, "P2→P3") {
		t.Fatalf("missing priority arrow:\n%s", out)
	}
	if !strings.Contains(out, "remark: confirmed") {
		t.Fatalf("missing valid remark:\n%s", out)
	}
}

func TestTerminalFormatterRendersFinalizationPriorityChange(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, false)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "Bug", Body: "B", ConfidenceScore: 0.8, Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.9, Remarks: "ok"},
				Finalization: &model.FindingFinalization{Title: "Bug", Body: "B", Priority: 2, ConfidenceScore: 0.7, Remarks: "downgraded"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "finalized") {
		t.Fatalf("missing finalized marker:\n%s", out)
	}
	if !strings.Contains(out, "P1→P2") {
		t.Fatalf("missing finalize priority arrow:\n%s", out)
	}
	if !strings.Contains(out, "final remark: downgraded") {
		t.Fatalf("missing final remark:\n%s", out)
	}
}

func TestTerminalFormatterRendersFinalizationPriorityUnchanged(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, false)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "Bug", Body: "B", ConfidenceScore: 0.8, Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.9, Remarks: "ok"},
				Finalization: &model.FindingFinalization{Title: "Bug", Body: "B", Priority: 1, ConfidenceScore: 0.85, Remarks: "kept"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "finalized  P1") {
		t.Fatalf("missing finalized priority display:\n%s", out)
	}
	if strings.Contains(out, "P1→P1") {
		t.Fatalf("unwanted arrow for unchanged priority:\n%s", out)
	}
}

func TestTerminalFormatterRendersWarnings(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, false)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{},
		Warnings: []string{
			"Verify failed for finding #1 \"x\": upstream 503",
			"Finalize failed: timeout; using verified result",
		},
		OverallCorrectness: "patch is correct",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Warnings:") {
		t.Fatalf("missing Warnings header:\n%s", out)
	}
	if !strings.Contains(out, "Verify failed for finding #1") {
		t.Fatalf("missing verify warning:\n%s", out)
	}
	if !strings.Contains(out, "Finalize failed: timeout") {
		t.Fatalf("missing finalize warning:\n%s", out)
	}
}

func TestTerminalFormatterWarningsColored(t *testing.T) {
	withoutNoColor(t)
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, true)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{},
		Warnings: []string{"something failed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "\x1b[33mWarnings:\x1b[0m") {
		t.Fatalf("expected ANSI-yellow Warnings header:\n%q", out)
	}
}

func withoutNoColor(t *testing.T) {
	t.Helper()
	prev, ok := os.LookupEnv("NO_COLOR")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv("NO_COLOR", prev)
			return
		}
		_ = os.Unsetenv("NO_COLOR")
	})
}

func TestJSONFormatterIncludesVerification(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "x", Body: "y", Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.5, Remarks: "ok"},
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
	if v["verdict"] != "confirmed" {
		t.Fatalf("verification.verdict = %#v", v["verdict"])
	}
	if v["remarks"] != "ok" {
		t.Fatalf("verification.remarks = %#v", v["remarks"])
	}
}

func TestJSONFormatterIncludesFindingID(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{ID: "11111111-1111-4111-8111-111111111111", Title: "x", Body: "y", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}},
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
	if first["id"] != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("id = %#v", first["id"])
	}
}

func TestJSONFormatterIncludesFinalization(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "x", Body: "y", Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Finalization: &model.FindingFinalization{Title: "final x", Body: "final y", Priority: 1, ConfidenceScore: 0.75, Remarks: "kept after verifier feedback"},
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
	first := payload["findings"].([]any)[0].(map[string]any)
	v, ok := first["finalization"].(map[string]any)
	if !ok {
		t.Fatalf("finalization missing: %#v", first)
	}
	if v["confidence_score"] != 0.75 {
		t.Fatalf("finalization.confidence_score = %#v", v["confidence_score"])
	}
	if v["title"] != "final x" || v["body"] != "final y" {
		t.Fatalf("finalization title/body = %#v", v)
	}
	if v["remarks"] != "kept after verifier feedback" {
		t.Fatalf("finalization.remarks = %#v", v["remarks"])
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
