package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Verbosef emits one verbose line: a dim "+" gutter, the identity bracket
// from ctx (same rendering as progress lines), the turn counter, and the
// message. With ANSI, the message head (text before the first ": ") is
// emphasized and the tail is tokenized like progress messages.
func (l *Logger) Verbosef(ctx context.Context, format string, args ...any) {
	if !l.Enabled() {
		return
	}
	info, _ := ProgressInfoFromContext(ctx)
	l.emitVerbose(formatVerboseLine(l.useANSI, info, fmt.Sprintf(format, args...)))
}

// VerboseBlock emits a labeled block of free-form content: one identity line
// for the label, then one dim content line per non-blank input line. The whole
// block is written in a single Write so concurrent agents cannot interleave it.
func (l *Logger) VerboseBlock(ctx context.Context, label, content string) {
	if !l.Enabled() {
		return
	}
	info, _ := ProgressInfoFromContext(ctx)
	var b strings.Builder
	if label != "" {
		b.WriteString(formatVerboseLine(l.useANSI, info, label))
	}
	lines := blockContentLines(content)
	if len(lines) == 0 {
		lines = []string{"(empty)"}
	}
	gutter := verboseGutter(l.useANSI)
	for _, line := range lines {
		if l.useANSI {
			line = progressStyle(progressColorGrey, line)
		}
		b.WriteString(gutter + line + "\n")
	}
	l.emitVerbose(b.String())
}

// VerboseJSON emits a labeled, pretty-printed, syntax-colored JSON value.
// The whole rendering is written in a single Write.
func (l *Logger) VerboseJSON(ctx context.Context, label string, value any) {
	if !l.Enabled() {
		return
	}
	info, _ := ProgressInfoFromContext(ctx)
	var b strings.Builder
	if label != "" {
		b.WriteString(formatVerboseLine(l.useANSI, info, label))
	}
	gutter := verboseGutter(l.useANSI)
	for _, line := range renderVerboseJSONLines(value) {
		text := line.text
		if l.useANSI {
			if line.stringOnly {
				text = progressStyle(progressColorStringGreen, text)
			} else {
				text = colorizeJSON(text)
			}
		}
		b.WriteString(gutter + text + "\n")
	}
	l.emitVerbose(b.String())
}

// VerboseMaybeJSON emits data as colored JSON when it parses, and as a plain
// content block otherwise.
func (l *Logger) VerboseMaybeJSON(ctx context.Context, label string, data []byte) {
	if !l.Enabled() {
		return
	}
	var value any
	if err := json.Unmarshal(data, &value); err == nil {
		l.VerboseJSON(ctx, label, value)
		return
	}
	l.VerboseBlock(ctx, label, string(data))
}

// emitVerbose is the single write path for verbose output.
func (l *Logger) emitVerbose(batch string) {
	if batch == "" {
		return
	}
	l.writeRaw(batch)
}

func renderVerboseJSONLines(value any) []renderedJSONLine {
	data, err := json.Marshal(value)
	if err != nil {
		return []renderedJSONLine{{text: fmt.Sprintf("failed to encode json: %v", err)}}
	}
	var normalized any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return []renderedJSONLine{{text: fmt.Sprintf("failed to decode json: %v", err)}}
	}
	normalized = normalizeEmbeddedJSON(normalized)
	return renderJSONLines(normalized, 0, "", false)
}

func blockContentLines(content string) []string {
	var out []string
	for line := range strings.SplitSeq(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func formatVerboseLine(useANSI bool, info ProgressInfo, msg string) string {
	var b strings.Builder
	b.WriteString(verboseGutter(useANSI))
	if bracket := formatProgressBracket(useANSI, info); bracket != "" {
		b.WriteString(bracket)
		b.WriteString(" ")
	}
	if info.Turn > 0 {
		b.WriteString(formatProgressTurn(useANSI, info.Turn))
		b.WriteString(" ")
	}
	if useANSI {
		b.WriteString(formatVerboseMessage(msg))
	} else {
		b.WriteString(msg)
	}
	b.WriteString("\n")
	return b.String()
}

// formatVerboseMessage renders "Action phrase: key=value tail" as an
// emphasized head plus a tokenized tail. Messages without a tail render
// entirely as the head, with a bare trailing colon trimmed.
func formatVerboseMessage(msg string) string {
	head, tail, found := strings.Cut(msg, ": ")
	if found && tail != "" {
		return progressStyle(progressColorWhite, head) + "  " + colorizeProgressText(tail)
	}
	return progressStyle(progressColorWhite, strings.TrimSuffix(msg, ":"))
}

func verboseGutter(useANSI bool) string {
	if useANSI {
		return progressStyle(progressColorDarkGrey, "+") + " "
	}
	return "+ "
}
