// Package textsan sanitizes untrusted text (LLM- or PR-sourced) before it is
// written to a terminal or a human-readable log, neutralizing control
// characters that could drive terminal escape sequences (CWE-150: terminal
// escape injection) such as cursor moves, screen clears, or alternate-buffer
// switches.
package textsan

import (
	"regexp"
	"strings"
)

var (
	sensitiveAssignmentPattern = regexp.MustCompile(`(?i)(["']?(?:(?:[a-z0-9]+[_-])*(?:api[_-]?key|access[_-]?token|token|secret|password|credential|authorization|auth))["']?\s*[:=]\s*)("(?:\\.|[^"])*"|'(?:\\.|[^'])*'|[^\s,;}]+)`)
	bearerTokenPattern         = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{8,}`)
	prefixedSecretPattern      = regexp.MustCompile(`(?i)\b(?:sk-[A-Za-z0-9_-]{8,}|gh[pousr]_[A-Za-z0-9]{8,}|glpat-[A-Za-z0-9_-]{8,}|AKIA[A-Z0-9]{16})\b`)
)

// StripControl removes control characters that a terminal could interpret as
// escape sequences: all C0 control bytes except newline and tab, the DEL
// character, and the C1 control range (U+0080–U+009F). ESC (0x1B) is a C0
// control byte, so ANSI/CSI/OSC sequences are defanged at their introducer.
// Printable text, including multi-byte UTF-8, is preserved unchanged.
func StripControl(s string) string {
	if s == "" || !strings.ContainsFunc(s, isStrippable) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isStrippable(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// RedactSecrets replaces common credential forms in untrusted text before the
// text is persisted. It handles labelled JSON/YAML/env-style assignments,
// bearer tokens, and common provider token prefixes. Unlabelled arbitrary
// values cannot be identified reliably and remain unchanged.
func RedactSecrets(s string) string {
	if s == "" {
		return s
	}
	s = sensitiveAssignmentPattern.ReplaceAllString(s, `${1}"[redacted]"`)
	s = bearerTokenPattern.ReplaceAllString(s, "Bearer [redacted]")
	return prefixedSecretPattern.ReplaceAllString(s, "[redacted]")
}

func isStrippable(r rune) bool {
	if r == '\n' || r == '\t' {
		return false
	}
	if r < 0x20 || r == 0x7f {
		return true
	}
	return r >= 0x80 && r <= 0x9f
}
