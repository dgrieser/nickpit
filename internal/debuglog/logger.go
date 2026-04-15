package debuglog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type Logger struct {
	w       io.Writer
	useANSI bool
	enabled bool
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
	if !l.Enabled() {
		return
	}
	if label != "" {
		l.writeLines(label, "90")
	}
	if content == "" {
		l.writeLines("(empty)", "90")
		return
	}
	l.writeLines(content, "90")
}

func (l *Logger) PrintJSON(label string, value any) {
	if !l.Enabled() {
		return
	}
	if label != "" {
		l.writeLines(label, "90")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		l.writeLines(fmt.Sprintf("failed to encode json: %v", err), "90")
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		keyPrefix, contentLines := splitJSONStringValue(line)
		if contentLines == nil {
			if l.useANSI {
				_, _ = fmt.Fprintf(l.w, "\x1b[90m+ \x1b[0m%s\n", colorizeJSON(keyPrefix))
			} else {
				_, _ = fmt.Fprintf(l.w, "+ %s\n", keyPrefix)
			}
			continue
		}
		if l.useANSI {
			_, _ = fmt.Fprintf(l.w, "\x1b[90m+ \x1b[0m%s\x1b[32m%s\x1b[0m\n", colorizeJSON(keyPrefix), contentLines[0])
			for _, cl := range contentLines[1:] {
				_, _ = fmt.Fprintf(l.w, "\x1b[90m+ \x1b[32m%s\x1b[0m\n", cl)
			}
		} else {
			_, _ = fmt.Fprintf(l.w, "+ %s%s\n", keyPrefix, contentLines[0])
			for _, cl := range contentLines[1:] {
				_, _ = fmt.Fprintf(l.w, "+ %s\n", cl)
			}
		}
	}
}

// splitJSONStringValue detects a JSON line whose string value contains \n escapes and
// expands them. Returns the key prefix (without the value's opening quote, suitable for
// colorizeJSON) and the content lines. The first content line has a leading `"`, the last
// has a trailing `"`, and all continuation lines are indented to align with the value start.
// Returns (line, nil) if no expansion is needed.
func splitJSONStringValue(line string) (string, []string) {
	colonIdx := strings.Index(line, `": "`)
	if colonIdx < 0 {
		return line, nil
	}
	valueStart := colonIdx + 4
	if valueStart >= len(line) {
		return line, nil
	}
	rest := line[valueStart:]

	var raw string
	switch {
	case strings.HasSuffix(rest, `",`):
		raw = rest[:len(rest)-2]
	case strings.HasSuffix(rest, `"`):
		raw = rest[:len(rest)-1]
	default:
		return line, nil
	}

	if !strings.Contains(raw, `\n`) {
		return line, nil
	}

	unescaped := strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\\`, `\`, `\"`, `"`).Replace(raw)
	keyPrefix := line[:colonIdx+3]                    // `  "key": ` without the opening value `"`
	valueIndent := strings.Repeat(" ", len(keyPrefix)+1) // align continuation to after the `"`

	parts := strings.Split(unescaped, "\n")
	contentLines := make([]string, 0, len(parts))
	contentLines = append(contentLines, `"`+parts[0]) // opening `"` on first line
	for _, p := range parts[1:] {
		contentLines = append(contentLines, valueIndent+p)
	}
	contentLines[len(contentLines)-1] += `"` // closing `"` on last line
	return keyPrefix, contentLines
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
