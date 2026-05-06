package logging

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestPrintErrorPlain(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	logger := New(&buf, false, false)

	logger.PrintError(assertErr("boom"))

	if got := buf.String(); got != "ERROR: boom\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestPrintErrorANSI(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{w: &buf, useANSI: true}

	logger.PrintError(assertErr("boom"))

	want := "\x1b[31mERROR\x1b[0m\x1b[90m:\x1b[0m \x1b[37mboom\x1b[0m\n"
	if got := buf.String(); got != want {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestPrintJSONRendersEmbeddedJSONStringStructurally(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	logger := New(&buf, true, false)

	logger.PrintJSON("", map[string]any{
		"payload": `{"nested":{"ok":true},"items":[1,2]}`,
	})

	got := buf.String()
	if !strings.Contains(got, `"payload": {`) {
		t.Fatalf("expected embedded object to render structurally:\n%s", got)
	}
	if !strings.Contains(got, `"nested": {`) {
		t.Fatalf("expected nested object:\n%s", got)
	}
	if !strings.Contains(got, `"items": [`) {
		t.Fatalf("expected embedded array:\n%s", got)
	}
	if strings.Contains(got, `"{\"nested\"`) {
		t.Fatalf("expected embedded JSON string to be parsed, not printed escaped:\n%s", got)
	}
}

func TestPrintJSONRendersMultilineStringsConsistently(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	logger := New(&buf, true, false)

	logger.PrintJSON("", map[string]any{
		"content": "line1\nline2\nline3",
	})

	got := buf.String()
	if !strings.Contains(got, `"content": "line1`) {
		t.Fatalf("expected multiline string first line:\n%s", got)
	}
	if !strings.Contains(got, "line2") || !strings.Contains(got, "line3") {
		t.Fatalf("expected multiline string continuation lines:\n%s", got)
	}
	if strings.Contains(got, `\n`) {
		t.Fatalf("expected real multiline output, not escaped newlines:\n%s", got)
	}
}

func TestPrintJSONPreservesEscapesInMultilineStrings(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	logger := New(&buf, true, false)

	logger.PrintJSON("", map[string]any{
		"content": "line1\t\"quoted\" <tag>\npath\\segment\t\"tail\"",
	})

	got := buf.String()
	if !strings.Contains(got, "\t\"quoted\" <tag>") {
		t.Fatalf("expected literal tab, quotes, and < on first line:\n%s", got)
	}
	if !strings.Contains(got, "path\\segment\t\"tail\"") {
		t.Fatalf("expected literal backslash, tab, and quotes on continuation line:\n%s", got)
	}
	for _, unwanted := range []string{`\t`, `\"`, `\u003c`} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("expected escape %q to be decoded in multiline output:\n%s", unwanted, got)
		}
	}
}

func TestReasoningRendererTTYReplaysFullReasoningAfterPreview(t *testing.T) {
	var buf bytes.Buffer
	renderer := &ReasoningRenderer{w: &buf, isTTY: true, width: 80, height: 12}

	id := renderer.Begin("review")
	var want strings.Builder
	want.WriteString("Reasoning for review...\n")
	for i := 0; i < 80; i++ {
		line := fmt.Sprintf("line %02d\n", i)
		want.WriteString(line)
		renderer.Append(id, line)
	}
	renderer.End(id)

	got := buf.String()
	if !strings.Contains(got, want.String()) {
		t.Fatalf("final reasoning output was capped or reordered:\n%s", got)
	}
	if strings.Contains(got, "... truncated") {
		t.Fatalf("final reasoning output should not be truncated:\n%s", got)
	}
}

func TestReasoningRendererTTYProgressDoesNotCorruptFinalReasoning(t *testing.T) {
	var buf bytes.Buffer
	renderer := &ReasoningRenderer{w: &buf, isTTY: true, width: 80, height: 12}

	id := renderer.Begin("review")
	renderer.Append(id, "partial")
	renderer.WriteProgress("Progress: done")
	renderer.End(id)

	got := buf.String()
	if !strings.Contains(got, "Progress: done\n") {
		t.Fatalf("progress should remain in scrollback: %q", got)
	}
	if want := "Reasoning for review...\npartial\n\n"; !strings.Contains(got, want) {
		t.Fatalf("final reasoning block missing, want %q in %q", want, got)
	}
}

func TestReasoningRendererANSIBannerIsGreyItalic(t *testing.T) {
	var buf bytes.Buffer
	renderer := &ReasoningRenderer{w: &buf, useANSI: true}

	id := renderer.Begin("review")
	renderer.End(id)

	got := buf.String()
	if !strings.Contains(got, "\x1b[3;90mReasoning for review...\x1b[0m\n") {
		t.Fatalf("reasoning banner should be grey italic: %q", got)
	}
	if strings.Contains(got, "\x1b[33m") {
		t.Fatalf("reasoning banner should not use yellow: %q", got)
	}
}

func TestReasoningRendererTTYConcurrentSectionsReplaySeparateBlocks(t *testing.T) {
	var buf bytes.Buffer
	renderer := &ReasoningRenderer{w: &buf, isTTY: true, width: 80, height: 24}

	first := renderer.Begin("first")
	second := renderer.Begin("second")
	renderer.Append(first, "first line 1\n")
	renderer.Append(second, "second line 1\n")
	renderer.Append(first, "first line 2\n")
	renderer.Append(second, "second line 2\n")

	renderer.End(second)
	renderer.End(first)

	got := buf.String()
	for _, want := range []string{
		"Reasoning for second...\nsecond line 1\nsecond line 2\n\n",
		"Reasoning for first...\nfirst line 1\nfirst line 2\n\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("final reasoning block %q missing in %q", want, got)
		}
	}
	if strings.Contains(got, "second line 1\nfirst line 2\nsecond line 2\n") {
		t.Fatalf("final reasoning blocks should not interleave:\n%s", got)
	}
}

func TestReasoningRendererLivePreviewShrinksToFitTerminal(t *testing.T) {
	var buf bytes.Buffer
	renderer := &ReasoningRenderer{w: &buf, isTTY: true, width: 80, height: 13}

	for _, label := range []string{"one", "two", "three"} {
		id := renderer.Begin(label)
		for i := 0; i < 20; i++ {
			renderer.Append(id, fmt.Sprintf("%s line %02d\n", label, i))
		}
	}

	live := renderer.buildLiveLocked()
	if rows := visibleLineCount(live, renderer.termWidth()); rows > renderer.termHeight()-1 {
		t.Fatalf("live preview rows=%d exceed available rows=%d:\n%s", rows, renderer.termHeight()-1, live)
	}
	for _, want := range []string{"Reasoning for one...\n", "Reasoning for two...\n", "Reasoning for three...\n"} {
		if !strings.Contains(live, want) {
			t.Fatalf("live preview should keep header %q in:\n%s", want, live)
		}
	}
}

func TestReasoningRendererLivePreviewPrefixesHeadersWithEmptyLine(t *testing.T) {
	var buf bytes.Buffer
	renderer := &ReasoningRenderer{w: &buf, isTTY: true, width: 80, height: 12}

	id := renderer.Begin("review")
	renderer.Append(id, "line\n")

	live := renderer.buildLiveLocked()
	if !strings.Contains(live, "\nReasoning for review...\n") {
		t.Fatalf("live preview should prefix header with empty line: %q", live)
	}
	renderer.End(id)
	if strings.Contains(buf.String(), "\n\nReasoning for review...\nline\n\n") {
		t.Fatalf("final replay should not add prefixed empty line: %q", buf.String())
	}
}

func assertErr(msg string) error {
	return testError(msg)
}

type testError string

func (e testError) Error() string {
	return string(e)
}
