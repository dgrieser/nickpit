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
			{
				ID: "11111111-1111-4111-8111-111111111111", Title: "Problem", Body: "Description with `code`.\n\nSecond paragraph.",
				ConfidenceScore: 0.82, Priority: intPtr(2),
				CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 10, End: 12}},
				Suggestions: []model.Suggestion{
					{Body: "use a mutex", LineRange: model.LineRange{Start: 10, End: 12}},
					{Body: "first line\nsecond line", LineRange: model.LineRange{Start: 10, End: 12}},
				},
			},
			{
				ID: "22222222-2222-4222-8222-222222222222", Title: "Single line", Body: "Short.",
				ConfidenceScore: 0.6, Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 7, End: 7}},
			},
		},
		Warnings:               []string{"Publish failed: 404"},
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

func TestTerminalFormatterStripsControlCharsFromUntrustedText(t *testing.T) {
	var buf bytes.Buffer
	// useANSI=false: the formatter emits no escape codes of its own, so any
	// ESC byte in the output must have come from the (untrusted) finding text.
	formatter := NewTerminalFormatter(&buf, false)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "Title\x1b[31mINJECT", Body: "Body\x1b[?1049hsteal",
				ConfidenceScore: 0.8, Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "a\x1b[2J.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Suggestions:  []model.Suggestion{{Body: "sugg\x1b[31mestion"}},
			},
		},
		Warnings:           []string{"warn\x1b[31ming"},
		OverallCorrectness: "patch is correct",
		OverallExplanation: "Expl\x1b[1;5manation",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Primary security property: no ESC introducer survives, so the residual
	// "[31m"/"[?1049h" text is inert printable characters.
	if strings.ContainsRune(out, 0x1b) {
		t.Fatalf("terminal output leaked ESC from untrusted text:\n%q", out)
	}
	// The visible content is preserved (only the control byte is dropped).
	for _, want := range []string{"Title", "INJECT", "Body", "steal", "anation", "estion", "ming"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected preserved text %q in output:\n%q", want, out)
		}
	}
}

func TestPriorityBadge(t *testing.T) {
	tests := []struct {
		rank      int
		wantANSI  string
		wantPlain string
	}{
		{0, "\x1b[48;2;255;7;58m\x1b[38;2;0;0;0m BLOCKING \x1b[0m", "[BLOCKING]"},
		{1, "\x1b[48;2;251;20;139m\x1b[38;2;0;0;0m HIGH \x1b[0m", "[HIGH]"},
		{2, "\x1b[48;2;255;81;0m\x1b[38;2;0;0;0m MEDIUM \x1b[0m", "[MEDIUM]"},
		{3, "\x1b[48;2;255;234;0m\x1b[38;2;0;0;0m LOW \x1b[0m", "[LOW]"},
		{-1, "\x1b[48;2;255;7;58m\x1b[38;2;0;0;0m BLOCKING \x1b[0m", "[BLOCKING]"},
		{9, "\x1b[48;2;255;234;0m\x1b[38;2;0;0;0m LOW \x1b[0m", "[LOW]"},
	}
	for _, tt := range tests {
		if got := priorityBadge(tt.rank, true); got != tt.wantANSI {
			t.Errorf("priorityBadge(%d, true) = %q, want %q", tt.rank, got, tt.wantANSI)
		}
		if got := priorityBadge(tt.rank, false); got != tt.wantPlain {
			t.Errorf("priorityBadge(%d, false) = %q, want %q", tt.rank, got, tt.wantPlain)
		}
	}
}

func TestCorrectnessBadge(t *testing.T) {
	if got, want := correctnessBadge("patch is correct", true), "\x1b[48;2;0;255;13m\x1b[38;2;0;0;0m ✓ CORRECT \x1b[0m"; got != want {
		t.Errorf("correct ANSI badge = %q, want %q", got, want)
	}
	if got, want := correctnessBadge("patch is incorrect", true), "\x1b[48;2;255;7;58m\x1b[38;2;0;0;0m ✗ INCORRECT \x1b[0m"; got != want {
		t.Errorf("incorrect ANSI badge = %q, want %q", got, want)
	}
	if got := correctnessBadge("Patch is INCORRECT because", false); got != "[INCORRECT]" {
		t.Errorf("contains-incorrect mapping = %q, want [INCORRECT]", got)
	}
	if got := correctnessBadge("looks good", false); got != "[CORRECT]" {
		t.Errorf("default mapping = %q, want [CORRECT]", got)
	}
}

func TestTerminalFormatterSortsFindings(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, false)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "Refuted high", Body: "B", ConfidenceScore: 0.9, Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "refuted.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 1, ConfidenceScore: 0.99},
			},
			{
				Title: "Low prio", Body: "B", ConfidenceScore: 0.9, Priority: intPtr(3),
				CodeLocation: model.CodeLocation{FilePath: "low.go", LineRange: model.LineRange{Start: 1, End: 1}},
			},
			{
				Title: "Confirmed high", Body: "B", ConfidenceScore: 0.9, Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "confirmed.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.8},
			},
		},
		OverallCorrectness: "patch is incorrect",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	confirmed := strings.Index(out, "confirmed.go")
	refuted := strings.Index(out, "refuted.go")
	low := strings.Index(out, "low.go")
	if !(confirmed < refuted && refuted < low) {
		t.Fatalf("sort order wrong (confirmed=%d refuted=%d low=%d):\n%s", confirmed, refuted, low, out)
	}
}

func TestTerminalFormatterFinalizedPriorityDrivesBadge(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, false)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "Bug", Body: "B", ConfidenceScore: 0.8, Priority: intPtr(1),
				CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Finalization: &model.FindingFinalization{Title: "Bug", Body: "B", Priority: 2, ConfidenceScore: 0.7},
			},
		},
		OverallCorrectness: "patch is incorrect",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "[MEDIUM]") {
		t.Fatalf("finalized priority should drive the badge:\n%s", out)
	}
	if strings.Contains(out, "[HIGH]") {
		t.Fatalf("original priority should not be shown:\n%s", out)
	}
}

func TestTerminalFormatterPrefersSummarizationBody(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, false)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "Bug", Body: "original", ConfidenceScore: 0.8, Priority: intPtr(1),
				CodeLocation:  model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}},
				Finalization:  &model.FindingFinalization{Title: "Bug", Body: "long finalized body", Priority: 1, ConfidenceScore: 0.85, Remarks: "kept"},
				Summarization: &model.FindingSummarization{Title: "Bug", Body: "short summary", Priority: 1, ConfidenceScore: 0.85, Remarks: "kept"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "short summary") {
		t.Fatalf("missing summarized body:\n%s", out)
	}
	if strings.Contains(out, "long finalized body") {
		t.Fatalf("finalized body should be replaced by the summary:\n%s", out)
	}
}

func TestTerminalFormatterEmptyFindings(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, false)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings:               []model.Finding{},
		Warnings:               []string{"Verify failed for finding #1 \"x\": upstream 503"},
		OverallCorrectness:     "patch is correct",
		OverallExplanation:     "All good.",
		OverallConfidenceScore: 0.9,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "[CORRECT]") {
		t.Fatalf("missing correctness badge:\n%s", out)
	}
	if !strings.Contains(out, "All good.") {
		t.Fatalf("missing summary explanation:\n%s", out)
	}
	if got := strings.Count(out, "---"); got != 1 {
		t.Fatalf("rules = %d, want exactly 1 (summary|footer):\n%s", got, out)
	}
	if !strings.Contains(out, "! Verify failed for finding #1") {
		t.Fatalf("missing footer warning:\n%s", out)
	}
	if !strings.Contains(out, "Tokens: 0 prompt / 0 completion / 0 total") {
		t.Fatalf("missing tokens footer:\n%s", out)
	}
}

func TestTerminalFormatterANSI(t *testing.T) {
	withoutNoColor(t)
	var buf bytes.Buffer
	formatter := NewTerminalFormatter(&buf, true)
	formatter.SetWidth(80)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{
			{
				Title: "Race in stream retry", Body: "Body text.", ConfidenceScore: 0.81, Priority: intPtr(2),
				CodeLocation: model.CodeLocation{FilePath: "internal/llm/client.go", LineRange: model.LineRange{Start: 120, End: 124}},
			},
		},
		Warnings:               []string{"something failed"},
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "Explanation.",
		OverallConfidenceScore: 0.77,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Badge escapes (exact bytes covered by badge unit tests).
	if !strings.Contains(out, "\x1b[48;2;255;81;0m") {
		t.Fatalf("missing P2 badge background:\n%q", out)
	}
	if !strings.Contains(out, "\x1b[48;2;255;7;58m") {
		t.Fatalf("missing incorrect badge background:\n%q", out)
	}
	if !strings.Contains(out, "\x1b[1minternal/llm/client.go:120-124\x1b[0m") {
		t.Fatalf("missing bold location:\n%q", out)
	}
	if !strings.Contains(out, "Race in stream retry") {
		t.Fatalf("missing glamour-rendered title text:\n%q", out)
	}
	if !strings.Contains(out, "\x1b[33m! something failed\x1b[0m") {
		t.Fatalf("missing yellow footer warning:\n%q", out)
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

func intPtr(v int) *int {
	return &v
}

func TestJSONFormatterAlwaysIncludesToolLimitsAndDuplicates(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewJSONFormatter(&buf)
	err := formatter.FormatFindings(&model.ReviewResult{
		Findings: []model.Finding{},
		AgentRuns: []model.AgentRun{
			{Name: "review", Role: "review", MaxToolCalls: 0, DuplicateToolCalls: 0},
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
