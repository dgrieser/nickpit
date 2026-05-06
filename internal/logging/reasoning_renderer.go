package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// ReasoningRenderer owns all reasoning output when --show-reasoning is enabled.
// Each active stream gets a bounded live preview on TTY and is replayed in full
// when it ends. On non-TTY each section is flushed atomically when it ends.
type ReasoningRenderer struct {
	mu           sync.Mutex
	w            io.Writer
	fd           int
	useANSI      bool
	isTTY        bool
	sections     []*reasoningSection
	lastLiveRows int
	width        int // test override
	height       int // test override
}

type reasoningSection struct {
	label string
	body  strings.Builder
	ended bool
}

// SectionID identifies an open reasoning section.
type SectionID int

func newReasoningRenderer(w io.Writer, useANSI bool) *ReasoningRenderer {
	isTTY := false
	fd := -1
	if f, ok := w.(*os.File); ok {
		if stat, err := f.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
			isTTY = true
			fd = int(f.Fd())
		}
	}
	return &ReasoningRenderer{
		w:       w,
		fd:      fd,
		useANSI: useANSI,
		isTTY:   isTTY,
	}
}

// Begin opens a new reasoning section and returns its ID. TTY shows a live
// preview; non-TTY buffers until End.
func (r *ReasoningRenderer) Begin(label string) SectionID {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := SectionID(len(r.sections))
	sec := &reasoningSection{label: label}
	r.sections = append(r.sections, sec)
	if r.isTTY {
		r.redrawLiveLocked()
	}
	return id
}

// Append adds a delta to the section's content buffer and updates the live preview.
func (r *ReasoningRenderer) Append(id SectionID, delta string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if int(id) >= len(r.sections) {
		return
	}
	sec := r.sections[id]
	if sec.ended {
		return
	}
	sec.body.WriteString(delta)
	if r.isTTY {
		r.redrawLiveLocked()
	}
}

// End marks the section done. TTY clears the live preview, prints the full
// section, then redraws remaining previews. Non-TTY flushes atomically.
func (r *ReasoningRenderer) End(id SectionID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if int(id) >= len(r.sections) {
		return
	}
	sec := r.sections[id]
	if sec.ended {
		return
	}
	sec.ended = true

	if !r.isTTY {
		r.writeFinalSectionLocked(sec)
		if r.allEndedLocked() {
			r.sections = nil
		}
		return
	}

	r.clearLiveLocked()
	r.writeFinalSectionLocked(sec)
	if r.allEndedLocked() {
		r.sections = nil
		return
	}
	r.redrawLiveLocked()
}

func (r *ReasoningRenderer) allEndedLocked() bool {
	for _, s := range r.sections {
		if !s.ended {
			return false
		}
	}
	return true
}

// writeFinalSectionLocked writes a complete section into scrollback.
// Must be called with r.mu held.
func (r *ReasoningRenderer) writeFinalSectionLocked(sec *reasoningSection) {
	_, _ = io.WriteString(r.w, r.formatBanner(sec.label))
	body := sec.body.String()
	if r.useANSI && body != "" {
		_, _ = fmt.Fprintf(r.w, "\x1b[3;90m%s\x1b[0m", body)
	} else if body != "" {
		_, _ = io.WriteString(r.w, body)
	}
	if body == "" || !strings.HasSuffix(body, "\n") {
		_, _ = io.WriteString(r.w, "\n")
	}
	_, _ = io.WriteString(r.w, "\n")
}

func (r *ReasoningRenderer) formatBanner(label string) string {
	if r.useANSI {
		if label != "" {
			return fmt.Sprintf("\x1b[3;90mReasoning for %s...\x1b[0m\n", label)
		}
		return "\x1b[3;90mReasoning...\x1b[0m\n"
	}
	if label != "" {
		return fmt.Sprintf("Reasoning for %s...\n", label)
	}
	return "Reasoning...\n"
}

func (r *ReasoningRenderer) redrawLiveLocked() {
	r.clearLiveLocked()
	live := r.buildLiveLocked()
	if live == "" {
		return
	}
	_, _ = io.WriteString(r.w, live)
	r.lastLiveRows = visibleLineCount(live, r.termWidth())
}

func (r *ReasoningRenderer) clearLiveLocked() {
	if r.lastLiveRows <= 0 {
		return
	}
	_, _ = fmt.Fprintf(r.w, "\x1b[%dA\x1b[0J", r.lastLiveRows)
	r.lastLiveRows = 0
}

func (r *ReasoningRenderer) buildLiveLocked() string {
	active := make([]*reasoningSection, 0, len(r.sections))
	for _, sec := range r.sections {
		if !sec.ended {
			active = append(active, sec)
		}
	}
	if len(active) == 0 {
		return ""
	}

	availableRows := r.termHeight() - 1
	if availableRows < len(active)*3 {
		return ""
	}
	rowsPerSection := 10
	if maxRows := availableRows/len(active) - 1; maxRows < rowsPerSection {
		rowsPerSection = maxRows
	}
	if rowsPerSection < 2 {
		return ""
	}

	bodyRows := rowsPerSection - 1
	width := r.termWidth()
	var out strings.Builder
	for _, sec := range active {
		out.WriteByte('\n')
		out.WriteString(r.formatBanner(sec.label))
		body := latestRows(sec.body.String(), bodyRows, width)
		if r.useANSI {
			fmt.Fprintf(&out, "\x1b[3;90m%s\x1b[0m", body)
		} else {
			out.WriteString(body)
		}
		if body == "" || !strings.HasSuffix(body, "\n") {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func latestRows(content string, maxRows, width int) string {
	if maxRows <= 0 || content == "" {
		return ""
	}
	if width <= 0 {
		width = 80
	}
	lines := strings.Split(content, "\n")
	trailingNewline := len(lines) > 0 && lines[len(lines)-1] == ""
	if trailingNewline {
		lines = lines[:len(lines)-1]
	}

	used := 0
	start := len(lines)
	for i := len(lines) - 1; i >= 0; i-- {
		rows := wrappedRows(lines[i], width)
		if used+rows > maxRows {
			break
		}
		used += rows
		start = i
	}
	if start == len(lines) {
		return tailRunes(lines[len(lines)-1], maxRows*width)
	}
	result := strings.Join(lines[start:], "\n")
	if trailingNewline {
		result += "\n"
	}
	return result
}

func tailRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[len(runes)-max:])
}

func wrappedRows(line string, width int) int {
	if width <= 0 {
		width = 80
	}
	n := len([]rune(line))
	if n == 0 {
		return 1
	}
	return (n + width - 1) / width
}

func visibleLineCount(s string, width int) int {
	if width <= 0 {
		width = 80
	}
	stripped := stripANSI(s)
	lines := strings.Split(stripped, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total := 0
	for _, line := range lines {
		total += wrappedRows(line, width)
	}
	return total
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && !isANSIFinalByte(s[i]) {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func isANSIFinalByte(b byte) bool {
	return b >= 0x40 && b <= 0x7E
}

func (r *ReasoningRenderer) termWidth() int {
	if r.width > 0 {
		return r.width
	}
	if r.fd < 0 {
		return 80
	}
	w, _, err := term.GetSize(r.fd)
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

func (r *ReasoningRenderer) termHeight() int {
	if r.height > 0 {
		return r.height
	}
	if r.fd < 0 {
		return 24
	}
	_, h, err := term.GetSize(r.fd)
	if err != nil || h <= 0 {
		return 24
	}
	return h
}

// WriteProgress writes a pre-formatted progress line outside the live preview.
func (r *ReasoningRenderer) WriteProgress(line string) {
	r.WriteOutside(line)
}

// WriteOutside writes normal logger output without corrupting the live preview.
func (r *ReasoningRenderer) WriteOutside(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isTTY {
		r.clearLiveLocked()
	}
	_, _ = io.WriteString(r.w, text)
	if !strings.HasSuffix(text, "\n") {
		_, _ = io.WriteString(r.w, "\n")
	}
	if r.isTTY {
		r.redrawLiveLocked()
	}
}
