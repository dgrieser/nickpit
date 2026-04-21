package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Logger struct {
	w             io.Writer
	useANSI       bool
	enabled       bool
	showReasoning bool
	showProgress  bool
}

func New(w io.Writer, enabled bool, useANSI bool) *Logger {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		useANSI = false
	}
	return &Logger{
		w:       w,
		useANSI: useANSI,
		enabled: enabled,
	}
}

func (l *Logger) SetShowReasoning(enabled bool) {
	if l == nil {
		return
	}
	l.showReasoning = enabled
}

func (l *Logger) SetShowProgress(enabled bool) {
	if l == nil {
		return
	}
	l.showProgress = enabled
}

func (l *Logger) Enabled() bool {
	return l != nil && l.enabled
}

func (l *Logger) Printf(format string, args ...any) {
	if !l.Enabled() {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if l.useANSI {
		if idx := strings.IndexByte(msg, ':'); idx >= 0 {
			_, _ = fmt.Fprintf(l.w, "\x1b[90m+\x1b[0m \x1b[1;33m%s\x1b[0m\x1b[90m%s\x1b[0m\n", msg[:idx], msg[idx:])
			return
		}
		_, _ = fmt.Fprintf(l.w, "\x1b[90m+\x1b[0m \x1b[1;33m%s\x1b[0m\n", msg)
		return
	}
	_, _ = fmt.Fprintf(l.w, "+ %s\n", msg)
}

func (l *Logger) PrintBlock(label, content string) {
	l.PrintBlockColor(label, content, "90")
}

func (l *Logger) PrintBlockColor(label, content, color string) {
	if !l.Enabled() {
		return
	}
	if label != "" {
		l.writeLines(label, color)
	}
	if content == "" {
		l.writeLines("(empty)", color)
		return
	}
	l.writeLines(content, color)
}

func (l *Logger) PrintJSON(label string, value any) {
	l.printJSON(label, value, false)
}

func (l *Logger) PrintProgress(label, summary string) {
	if l == nil || !l.showProgress {
		return
	}
	if l.useANSI {
		coloredSummary := colorizeProgressSummary(summary)
		if label == "Model" {
			coloredSummary = colorizeModelSummary(summary)
		} else if label == "Review" {
			coloredSummary = colorizeReviewSummary(summary)
		} else if label == "Result" || label == "Tool" {
			coloredSummary = colorizeResultSummary(summary)
		}
		_, _ = fmt.Fprintf(l.w, "\x1b[33m%s\x1b[0m\x1b[90m: \x1b[0m%s\n", label, coloredSummary)
		return
	}
	_, _ = fmt.Fprintf(l.w, "%s: %s\n", label, summary)
}

func (l *Logger) PrintProgressToolCall(call, result string) {
	if l == nil || !l.showProgress {
		return
	}
	if l.useANSI {
		_, _ = fmt.Fprintf(
			l.w,
			"\x1b[33mTool\x1b[0m\x1b[90m: \x1b[0m%s \x1b[90m→\x1b[0m %s\n",
			colorizeToolCallCall(call),
			colorizeToolCallResult(result),
		)
		return
	}
	_, _ = fmt.Fprintf(l.w, "Tool: %s → %s\n", call, result)
}

func (l *Logger) printJSON(label string, value any, force bool) {
	if !force && !l.Enabled() {
		return
	}
	if label != "" {
		l.writeLines(label, "90")
	}
	data, err := json.Marshal(value)
	if err != nil {
		l.writeLines(fmt.Sprintf("failed to encode json: %v", err), "90")
		return
	}
	var normalized any
	if err := json.Unmarshal(data, &normalized); err != nil {
		l.writeLines(fmt.Sprintf("failed to decode json: %v", err), "90")
		return
	}
	normalized = normalizeEmbeddedJSON(normalized)
	for _, line := range renderJSONLines(normalized, 0, "", false) {
		l.writeRenderedJSONLine(line)
	}
}

func (l *Logger) PrintReasoningBanner() {
	if l == nil || !l.showReasoning {
		return
	}
	if l.useANSI {
		_, _ = fmt.Fprint(l.w, "\x1b[33mReasoning\x1b[0m\x1b[90m...\x1b[0m\n")
		return
	}
	_, _ = fmt.Fprintln(l.w, "Reasoning...")
}

func (l *Logger) PrintReasoningDelta(delta string) {
	if l == nil || !l.showReasoning || delta == "" {
		return
	}
	if l.useANSI {
		_, _ = fmt.Fprintf(l.w, "\x1b[3;90m%s\x1b[0m", delta)
		return
	}
	_, _ = fmt.Fprint(l.w, delta)
}

func (l *Logger) PrintBlankLine() {
	if l == nil || !l.showReasoning {
		return
	}
	_, _ = fmt.Fprintln(l.w)
}

func (l *Logger) PrintStatusLine(text string) {
	if l == nil || text == "" {
		return
	}
	if l.useANSI {
		_, _ = fmt.Fprintf(l.w, "\x1b[90m%s\x1b[0m\n", text)
		return
	}
	_, _ = fmt.Fprintln(l.w, text)
}

func (l *Logger) writeLines(text, color string) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if l.useANSI {
			_, _ = fmt.Fprintf(l.w, "\x1b[%sm+ %s\x1b[0m\n", color, line)
			continue
		}
		_, _ = fmt.Fprintf(l.w, "+ %s\n", line)
	}
}

type renderedJSONLine struct {
	text       string
	stringOnly bool
}

func (l *Logger) writeRenderedJSONLine(line renderedJSONLine) {
	if l.useANSI {
		if line.stringOnly {
			_, _ = fmt.Fprintf(l.w, "\x1b[90m+ \x1b[0m\x1b[32m%s\x1b[0m\n", line.text)
			return
		}
		_, _ = fmt.Fprintf(l.w, "\x1b[90m+ \x1b[0m%s\n", colorizeJSON(line.text))
		return
	}
	_, _ = fmt.Fprintf(l.w, "+ %s\n", line.text)
}

func normalizeEmbeddedJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = normalizeEmbeddedJSON(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = normalizeEmbeddedJSON(item)
		}
		return out
	case string:
		trimmed := strings.TrimSpace(typed)
		if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
			(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
			var embedded any
			if err := json.Unmarshal([]byte(trimmed), &embedded); err == nil {
				return normalizeEmbeddedJSON(embedded)
			}
		}
		return strings.ReplaceAll(typed, "\r\n", "\n")
	default:
		return typed
	}
}

func renderJSONLines(value any, indent int, prefix string, trailingComma bool) []renderedJSONLine {
	switch typed := value.(type) {
	case map[string]any:
		return renderJSONObjectLines(typed, indent, prefix, trailingComma)
	case []any:
		return renderJSONArrayLines(typed, indent, prefix, trailingComma)
	case string:
		return renderJSONStringLines(typed, prefix, trailingComma)
	default:
		return []renderedJSONLine{{
			text: prefix + marshalJSONScalar(typed) + trailingCommaSuffix(trailingComma),
		}}
	}
}

func renderJSONObjectLines(value map[string]any, indent int, prefix string, trailingComma bool) []renderedJSONLine {
	if len(value) == 0 {
		return []renderedJSONLine{{
			text: prefix + "{}" + trailingCommaSuffix(trailingComma),
		}}
	}

	lines := []renderedJSONLine{{text: prefix + "{"}}
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for i, key := range keys {
		childPrefix := strings.Repeat("  ", indent+1) + marshalJSONString(key) + ": "
		lines = append(lines, renderJSONLines(value[key], indent+1, childPrefix, i < len(keys)-1)...)
	}
	lines = append(lines, renderedJSONLine{
		text: strings.Repeat("  ", indent) + "}" + trailingCommaSuffix(trailingComma),
	})
	return lines
}

func renderJSONArrayLines(value []any, indent int, prefix string, trailingComma bool) []renderedJSONLine {
	if len(value) == 0 {
		return []renderedJSONLine{{
			text: prefix + "[]" + trailingCommaSuffix(trailingComma),
		}}
	}

	lines := []renderedJSONLine{{text: prefix + "["}}
	for i, item := range value {
		childPrefix := strings.Repeat("  ", indent+1)
		lines = append(lines, renderJSONLines(item, indent+1, childPrefix, i < len(value)-1)...)
	}
	lines = append(lines, renderedJSONLine{
		text: strings.Repeat("  ", indent) + "]" + trailingCommaSuffix(trailingComma),
	})
	return lines
}

func renderJSONStringLines(value, prefix string, trailingComma bool) []renderedJSONLine {
	if !strings.Contains(value, "\n") {
		return []renderedJSONLine{{
			text: prefix + marshalJSONString(value) + trailingCommaSuffix(trailingComma),
		}}
	}

	parts := strings.Split(value, "\n")
	continuation := strings.Repeat(" ", len(prefix)+1)
	lines := make([]renderedJSONLine, 0, len(parts))
	lines = append(lines, renderedJSONLine{
		text:       prefix + `"` + escapeJSONStringFragment(parts[0]),
		stringOnly: true,
	})
	for _, part := range parts[1 : len(parts)-1] {
		lines = append(lines, renderedJSONLine{
			text:       continuation + escapeJSONStringFragment(part),
			stringOnly: true,
		})
	}
	lines = append(lines, renderedJSONLine{
		text:       continuation + escapeJSONStringFragment(parts[len(parts)-1]) + `"` + trailingCommaSuffix(trailingComma),
		stringOnly: true,
	})
	return lines
}

func colorizeToolCallArguments(text string) string {
	return colorizeKeyValueSummary(text)
}

func colorizeToolCallCall(text string) string {
	var b strings.Builder
	segments := strings.Split(text, " replaced with ")
	for i, segment := range segments {
		if i > 0 {
			b.WriteString("\x1b[90m replaced with \x1b[0m")
		}
		open := strings.IndexByte(segment, '(')
		close := strings.LastIndexByte(segment, ')')
		if open <= 0 || close < open {
			b.WriteString(colorizeKeyValueSummary(text))
			return b.String()
		}
		b.WriteString("\x1b[37m")
		b.WriteString(segment[:open])
		b.WriteString("\x1b[0m")
		b.WriteString("\x1b[90m(\x1b[0m")
		b.WriteString(colorizeToolCallArguments(segment[open+1 : close]))
		b.WriteString("\x1b[90m)\x1b[0m")
	}
	return b.String()
}

func colorizeToolCallResult(text string) string {
	if strings.HasPrefix(text, "result") {
		return "\x1b[37mresult\x1b[0m" + colorizeKeyValueSummary(strings.TrimPrefix(text, "result"))
	}
	return colorizeKeyValueSummary(text)
}

func colorizeProgressSummary(text string) string {
	return colorizeKeyValueSummary(text)
}

func colorizeModelSummary(text string) string {
	// format: model:effort [flags] @ url
	modelName, rest, ok := strings.Cut(text, ":")
	if !ok {
		return colorizeProgressSummary(text)
	}
	effortAndFlags, urlPart, ok := strings.Cut(rest, " @ ")
	if !ok {
		return colorizeProgressSummary(text)
	}
	effort, flags, hasFlags := strings.Cut(effortAndFlags, " [")
	out := "\x1b[34m" + modelName + "\x1b[0m" +
		"\x1b[90m:\x1b[0m" +
		"\x1b[32m" + effort + "\x1b[0m"
	if hasFlags {
		out += " \x1b[90m[\x1b[0m" + strings.TrimSuffix(flags, "]") + "\x1b[90m]\x1b[0m"
	}
	return out + " \x1b[90m@\x1b[0m \x1b[35m" + urlPart + "\x1b[0m"
}

func colorizeReviewSummary(text string) string {
	// format: mode:submode @ profile ≥threshold on repo @ head → base
	mode, rest, ok := strings.Cut(text, ":")
	if !ok {
		return colorizeProgressSummary(text)
	}
	submode, rest, ok := strings.Cut(rest, " @ ")
	if !ok {
		return colorizeProgressSummary(text)
	}
	profileThreshold, repoRefs, ok := strings.Cut(rest, " on ")
	if !ok {
		return colorizeProgressSummary(text)
	}
	profile, threshold, _ := strings.Cut(profileThreshold, " ")
	repo, refs, ok := strings.Cut(repoRefs, " @ ")
	if !ok {
		return colorizeProgressSummary(text)
	}
	head, base, ok := strings.Cut(refs, " → ")
	if !ok {
		return colorizeProgressSummary(text)
	}
	return "\x1b[34m" + mode + "\x1b[0m" +
		"\x1b[90m:\x1b[0m" +
		"\x1b[32m" + submode + "\x1b[0m" +
		" \x1b[90m@\x1b[0m " +
		profile +
		" \x1b[33m" + threshold + "\x1b[0m" +
		" \x1b[90mon\x1b[0m " +
		"\x1b[34m" + repo + "\x1b[0m" +
		" \x1b[90m@\x1b[0m " +
		"\x1b[32m" + head + "\x1b[0m" +
		" \x1b[90m→\x1b[0m " +
		"\x1b[35m" + base + "\x1b[0m"
}

func colorizeResultSummary(text string) string {
	statusPart, rest, ok := strings.Cut(text, ", ")
	if !ok {
		return colorizeProgressSummary(text)
	}
	key, value, ok := strings.Cut(statusPart, "=")
	if !ok {
		return colorizeProgressSummary(text)
	}
	valueColor := "\x1b[34m"
	switch value {
	case "ok":
		valueColor = "\x1b[32m"
	case "error", "LimitReached":
		valueColor = "\x1b[31m"
	}
	return "\x1b[34m" + key + "\x1b[0m" + "\x1b[90m=\x1b[0m" + valueColor + value + "\x1b[0m" + "\x1b[90m, \x1b[0m" + colorizeProgressSummary(rest)
}

func colorizeKeyValueSummary(text string) string {
	var b strings.Builder
	inString := false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '"' {
			if inString {
				b.WriteString(string(ch))
				b.WriteString("\x1b[0m")
				inString = false
			} else {
				b.WriteString("\x1b[32m")
				b.WriteString(string(ch))
				inString = true
			}
			continue
		}
		if inString {
			b.WriteByte(ch)
			continue
		}
		switch ch {
		case '=':
			b.WriteString("\x1b[90m=\x1b[0m")
		case ',', '(', ')', '[', ']':
			b.WriteString("\x1b[90m")
			b.WriteByte(ch)
			b.WriteString("\x1b[0m")
		case ' ':
			b.WriteByte(ch)
		default:
			if isToolCallKeyStart(text, i) {
				j := i
				for j < len(text) && isToolCallKeyChar(rune(text[j])) {
					j++
				}
				if j < len(text) && text[j] == '=' {
					b.WriteString("\x1b[34m")
					b.WriteString(text[i:j])
					b.WriteString("\x1b[0m")
					i = j - 1
					continue
				}
			}
			if isNumberStart(text, i) {
				j := i + 1
				for j < len(text) && isNumberChar(rune(text[j])) {
					j++
				}
				b.WriteString("\x1b[32m")
				b.WriteString(text[i:j])
				b.WriteString("\x1b[0m")
				i = j - 1
				continue
			}
			b.WriteByte(ch)
		}
	}
	if inString {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

func isToolCallKeyStart(text string, i int) bool {
	if !isToolCallKeyChar(rune(text[i])) {
		return false
	}
	if i > 0 {
		prev := rune(text[i-1])
		if isToolCallKeyChar(prev) {
			return false
		}
	}
	return true
}

func isToolCallKeyChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isNumberStart(text string, i int) bool {
	ch := rune(text[i])
	if !unicode.IsDigit(ch) {
		return false
	}
	if i > 0 {
		prev := rune(text[i-1])
		if unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '"' {
			return false
		}
	}
	return true
}

func isNumberChar(r rune) bool {
	return unicode.IsDigit(r) || r == '.'
}

func marshalJSONString(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(data)
}

func escapeJSONStringFragment(value string) string {
	marshaled := marshalJSONString(value)
	unquoted, err := strconv.Unquote(marshaled)
	if err != nil {
		return strings.TrimSuffix(strings.TrimPrefix(marshaled, `"`), `"`)
	}
	return unquoted
}

func marshalJSONScalar(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(data)
}

func trailingCommaSuffix(enabled bool) string {
	if enabled {
		return ","
	}
	return ""
}

func colorizeJSON(line string) string {
	var out strings.Builder
	inString := false
	escaped := false
	token := &bytes.Buffer{}

	flushToken := func() {
		if token.Len() == 0 {
			return
		}
		text := token.String()
		switch text {
		case "true", "false":
			out.WriteString("\x1b[35m" + text + "\x1b[0m")
		case "null":
			out.WriteString("\x1b[1;30m" + text + "\x1b[0m")
		default:
			out.WriteString("\x1b[36m" + text + "\x1b[0m")
		}
		token.Reset()
	}

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inString {
			token.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
				j := i + 1
				for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
					j++
				}
				color := "32"
				if j < len(line) && line[j] == ':' {
					color = "34"
				}
				out.WriteString("\x1b[" + color + "m" + token.String() + "\x1b[0m")
				token.Reset()
			}
			continue
		}
		switch {
		case ch == '"':
			flushToken()
			inString = true
			token.WriteByte(ch)
		case (ch >= '0' && ch <= '9') || ch == '-' || ch == '.' || (ch >= 'a' && ch <= 'z'):
			token.WriteByte(ch)
		default:
			flushToken()
			out.WriteByte(ch)
		}
	}
	flushToken()
	return out.String()
}
