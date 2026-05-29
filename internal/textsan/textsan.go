// Package textsan sanitizes untrusted text (LLM- or PR-sourced) before it is
// written to a terminal or a human-readable log, neutralizing control
// characters that could drive terminal escape sequences (CWE-150: terminal
// escape injection) such as cursor moves, screen clears, or alternate-buffer
// switches.
package textsan

import "strings"

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

func isStrippable(r rune) bool {
	if r == '\n' || r == '\t' {
		return false
	}
	if r < 0x20 || r == 0x7f {
		return true
	}
	return r >= 0x80 && r <= 0x9f
}
