package debuglog

import (
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
	l.writeLines(fmt.Sprintf(format, args...))
}

func (l *Logger) PrintBlock(label, content string) {
	if !l.Enabled() {
		return
	}
	if label != "" {
		l.writeLines(label)
	}
	if content == "" {
		l.writeLines("(empty)")
		return
	}
	l.writeLines(content)
}

func (l *Logger) writeLines(text string) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, line := range lines {
		if l.useANSI {
			_, _ = fmt.Fprintf(l.w, "\x1b[90m+ %s\x1b[0m\n", line)
			continue
		}
		_, _ = fmt.Fprintf(l.w, "+ %s\n", line)
	}
}
