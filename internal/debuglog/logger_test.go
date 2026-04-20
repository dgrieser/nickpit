package debuglog

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintJSONRendersEmbeddedJSONStringStructurally(t *testing.T) {
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
