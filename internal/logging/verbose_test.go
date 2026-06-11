package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func ansiVerboseLogger(buf *bytes.Buffer) *Logger {
	// Construct directly so a NO_COLOR env var cannot flip useANSI.
	return &Logger{w: buf, useANSI: true, enabled: true}
}

func verboseCtx() context.Context {
	return WithProgressInfo(context.Background(), ProgressInfo{
		AgentRole: "review",
		AgentName: "Security",
		Model:     "gpt-5",
		Effort:    "high",
		Turn:      2,
	})
}

func TestVerbosefPlain(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{w: &buf, useANSI: false, enabled: true}
	l.Verbosef(verboseCtx(), "Executing tool call: name=%s path=%s", "inspect_file", "internal/llm/client.go")
	want := "+ [review: Security · gpt-5:high] #2 Executing tool call: name=inspect_file path=internal/llm/client.go\n"
	if got := buf.String(); got != want {
		t.Errorf("Verbosef plain = %q, want %q", got, want)
	}
}

func TestVerbosefPlainNoInfo(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{w: &buf, useANSI: false, enabled: true}
	l.Verbosef(context.Background(), "Loading prompt: source=embedded name=x")
	want := "+ Loading prompt: source=embedded name=x\n"
	if got := buf.String(); got != want {
		t.Errorf("Verbosef plain no-info = %q, want %q", got, want)
	}
}

func TestVerbosefGatedOnEnabled(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{w: &buf, useANSI: false, enabled: false}
	l.Verbosef(verboseCtx(), "hidden")
	l.VerboseBlock(verboseCtx(), "label:", "content")
	l.VerboseJSON(verboseCtx(), "label:", map[string]any{"a": 1})
	l.VerboseMaybeJSON(verboseCtx(), "label:", []byte(`{"a":1}`))
	if buf.Len() != 0 {
		t.Errorf("expected no output when disabled, got %q", buf.String())
	}
}

func TestVerbosefANSIHeadTailSplit(t *testing.T) {
	var buf bytes.Buffer
	l := ansiVerboseLogger(&buf)
	l.Verbosef(context.Background(), "LLM stream opened: status=ok")
	got := buf.String()
	// dim gutter, white head without the colon, two-space gap, tokenized tail
	if !strings.HasPrefix(got, "\x1b[38;5;242m+\x1b[0m ") {
		t.Errorf("missing dim gutter: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;5;255mLLM stream opened\x1b[0m  ") {
		t.Errorf("missing white head with two-space gap: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;5;116mstatus\x1b[0m") {
		t.Errorf("tail key not tokenized: %q", got)
	}
	if strings.Contains(got, "opened:") {
		t.Errorf("colon should be dropped in ANSI head: %q", got)
	}
}

func TestVerbosefANSIProseOnlyAndTrailingColon(t *testing.T) {
	var buf bytes.Buffer
	l := ansiVerboseLogger(&buf)
	l.Verbosef(context.Background(), "LLM waiting for first stream chunk")
	l.Verbosef(context.Background(), "Extracted reasoning findings:")
	got := buf.String()
	if !strings.Contains(got, "\x1b[38;5;255mLLM waiting for first stream chunk\x1b[0m\n") {
		t.Errorf("prose message should render entirely as head: %q", got)
	}
	if !strings.Contains(got, "\x1b[38;5;255mExtracted reasoning findings\x1b[0m\n") {
		t.Errorf("trailing colon should be trimmed from bare label: %q", got)
	}
}

func TestVerbosefANSIBracketMatchesProgress(t *testing.T) {
	var buf bytes.Buffer
	l := ansiVerboseLogger(&buf)
	info, _ := ProgressInfoFromContext(verboseCtx())
	l.Verbosef(verboseCtx(), "msg")
	if want := formatProgressBracket(true, info); !strings.Contains(buf.String(), want) {
		t.Errorf("verbose bracket %q not identical to progress bracket %q", buf.String(), want)
	}
	if want := formatProgressTurn(true, 2); !strings.Contains(buf.String(), want) {
		t.Errorf("verbose turn rendering missing: %q", buf.String())
	}
}

func TestVerboseBlockPlain(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{w: &buf, useANSI: false, enabled: true}
	l.VerboseBlock(context.Background(), "Extracted reasoning findings:", "one\n\r\n  \ntwo\n")
	want := "+ Extracted reasoning findings:\n+ one\n+ two\n"
	if got := buf.String(); got != want {
		t.Errorf("VerboseBlock plain = %q, want %q", got, want)
	}
}

func TestVerboseBlockEmptyContent(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{w: &buf, useANSI: false, enabled: true}
	l.VerboseBlock(context.Background(), "label:", "")
	want := "+ label:\n+ (empty)\n"
	if got := buf.String(); got != want {
		t.Errorf("VerboseBlock empty = %q, want %q", got, want)
	}
}

func TestVerboseJSONANSIPalette(t *testing.T) {
	var buf bytes.Buffer
	l := ansiVerboseLogger(&buf)
	l.VerboseJSON(context.Background(), "", map[string]any{
		"key":  "value",
		"n":    3,
		"flag": true,
		"none": nil,
	})
	got := buf.String()
	for name, want := range map[string]string{
		"key turquoise":  "\x1b[38;5;116m\"key\"\x1b[0m",
		"string green":   "\x1b[38;5;120m\"value\"\x1b[0m",
		"number green":   "\x1b[38;5;118m3\x1b[0m",
		"bool green":     "\x1b[38;5;156mtrue\x1b[0m",
		"null dark grey": "\x1b[38;5;242mnull\x1b[0m",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s (%q) in:\n%q", name, want, got)
		}
	}
}

func TestColorizeProgressTextHashTokens(t *testing.T) {
	hash := func(s string) string { return "\x1b[38;5;71m" + s + "\x1b[0m" }
	tests := []struct {
		name string
		in   string
		want string // substring that must appear
		not  string // substring that must not appear
	}{
		{
			name: "full git sha uniform dark green",
			in:   "head_sha=4953b5702dae8484fb5a30b9eac6d9dceb4b279f",
			want: hash("4953b5702dae8484fb5a30b9eac6d9dceb4b279f"),
		},
		{
			name: "short sha",
			in:   "sha=4953b57",
			want: hash("4953b57"),
		},
		{
			name: "uuid",
			in:   "id=0b51bcd9-04d7-4c2a-a4a9-3e1d6cd44b25",
			want: hash("0b51bcd9-04d7-4c2a-a4a9-3e1d6cd44b25"),
		},
		{
			name: "uppercase hex",
			in:   "sha=4953B5702DAE8484FB5A30B9EAC6D9DCEB4B279F",
			want: hash("4953B5702DAE8484FB5A30B9EAC6D9DCEB4B279F"),
		},
		{
			name: "pure number stays number green",
			in:   "tokens=12345678",
			want: "\x1b[38;5;118m12345678\x1b[0m",
			not:  hash("12345678"),
		},
		{
			name: "hex-letter word without digits stays word",
			in:   "deadbeef accede",
			want: "\x1b[38;5;252mdeadbeef\x1b[0m",
			not:  hash("deadbeef"),
		},
		{
			name: "number with unit suffix unaffected",
			in:   "budget=120k context",
			want: "\x1b[38;5;118m120\x1b[0m\x1b[38;5;71mk\x1b[0m",
		},
		{
			name: "longer word containing hex prefix not clipped",
			in:   "ref=4953b5702daextra",
			not:  hash("4953b5702dae"),
		},
		{
			name: "non-uuid dashed token unaffected",
			in:   "rate-limit-delay",
			not:  hash("rate"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := colorizeProgressText(tt.in)
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("colorizeProgressText(%q) = %q, missing %q", tt.in, got, tt.want)
			}
			if tt.not != "" && strings.Contains(got, tt.not) {
				t.Errorf("colorizeProgressText(%q) = %q, must not contain %q", tt.in, got, tt.not)
			}
		})
	}
}

func TestVerboseJSONMultilineStringKeepsKeyColor(t *testing.T) {
	var buf bytes.Buffer
	l := ansiVerboseLogger(&buf)
	l.VerboseJSON(context.Background(), "", map[string]any{
		"content": "# Bash Style Guide\nDefensive Bash.",
	})
	got := buf.String()
	// The key on the first line of a multiline string keeps JSON key coloring
	// instead of being painted as string content.
	if !strings.Contains(got, "\x1b[38;5;116m\"content\"\x1b[0m") {
		t.Errorf("multiline string key not turquoise:\n%q", got)
	}
	if !strings.Contains(got, "\x1b[38;5;120m\"# Bash Style Guide\x1b[0m") {
		t.Errorf("multiline string first fragment not green:\n%q", got)
	}
	if !strings.Contains(got, "\x1b[38;5;120m              Defensive Bash.\"\x1b[0m") {
		t.Errorf("continuation line not green:\n%q", got)
	}
}

func TestVerboseMaybeJSONBothBranches(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{w: &buf, useANSI: false, enabled: true}
	l.VerboseMaybeJSON(context.Background(), "body:", []byte(`{"a":1}`))
	if got := buf.String(); !strings.Contains(got, `"a": 1`) {
		t.Errorf("parseable data should render as JSON: %q", got)
	}
	buf.Reset()
	l.VerboseMaybeJSON(context.Background(), "body:", []byte("not json"))
	if got := buf.String(); !strings.Contains(got, "+ not json\n") {
		t.Errorf("unparseable data should render as block: %q", got)
	}
}

// writeCounter counts calls to the underlying writer. It implements both
// Write and WriteString against the same counter because io.WriteString
// prefers WriteString when available — counting only one of the two would
// silently measure nothing if the emit path switches methods.
type writeCounter struct {
	buf    bytes.Buffer
	writes int
}

func (w *writeCounter) Write(p []byte) (int, error) {
	w.writes++
	return w.buf.Write(p)
}

func (w *writeCounter) WriteString(s string) (int, error) {
	w.writes++
	return w.buf.WriteString(s)
}

func TestVerboseBlockSingleWrite(t *testing.T) {
	var w writeCounter
	l := &Logger{w: &w, useANSI: false, enabled: true}
	l.VerboseBlock(context.Background(), "label:", "a\nb\nc")
	if w.writes != 1 {
		t.Errorf("VerboseBlock wrote %d times, want 1 batched write", w.writes)
	}
	l.VerboseJSON(context.Background(), "payload:", map[string]any{"a": []any{1, 2}})
	if w.writes != 2 {
		t.Errorf("VerboseJSON wrote %d more times, want exactly 1", w.writes-1)
	}
}
