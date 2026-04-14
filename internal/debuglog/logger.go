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
	l.writeLines(fmt.Sprintf(format, args...), "33")
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
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if l.useANSI {
			_, _ = fmt.Fprintf(l.w, "\x1b[90m+ \x1b[0m%s\n", colorizeJSON(line))
			continue
		}
		_, _ = fmt.Fprintf(l.w, "+ %s\n", line)
	}
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
