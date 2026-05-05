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
// Each active stream gets a labeled section. On TTY stderr the live area is
// redrawn in place; on non-TTY each section is flushed atomically when it ends.
type ReasoningRenderer struct {
	mu        sync.Mutex
	w         io.Writer
	fd        int // for term.GetSize; -1 when not a TTY
	useANSI   bool
	isTTY     bool
	sections  []*reasoningSection
	lastLines int // wrapped rows drawn in the last TTY redraw
}

type reasoningSection struct {
	label string
	buf   strings.Builder
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

// Begin opens a new reasoning section and returns its ID. Banner is shown
// immediately on TTY; on non-TTY Begin is a no-op visually.
func (r *ReasoningRenderer) Begin(label string) SectionID {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := SectionID(len(r.sections))
	sec := &reasoningSection{label: label}
	if label != "" {
		fmt.Fprintf(&sec.buf, "Reasoning for %s...\n", label)
	}
	r.sections = append(r.sections, sec)
	if r.isTTY {
		r.redrawLocked()
	}
	return id
}

// Append adds a delta to the section's content buffer and triggers a redraw on TTY.
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
	sec.buf.WriteString(delta)
	if r.isTTY {
		r.redrawLocked()
	}
}

// End marks the section done. When all open sections are ended the live area
// is committed (TTY) or the section is flushed (non-TTY).
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
		r.flushSectionLocked(sec)
		if r.allEndedLocked() {
			r.sections = nil
			r.lastLines = 0
		}
		return
	}

	if r.allEndedLocked() {
		r.redrawLocked()
		r.commitLocked()
	} else {
		r.redrawLocked()
	}
}

func (r *ReasoningRenderer) allEndedLocked() bool {
	for _, s := range r.sections {
		if !s.ended {
			return false
		}
	}
	return true
}

// redrawLocked erases the previous live area and redraws all sections.
// Must be called with r.mu held.
func (r *ReasoningRenderer) redrawLocked() {
	content := r.buildLiveAreaLocked()
	var out strings.Builder
	if r.lastLines > 0 {
		fmt.Fprintf(&out, "\x1b[%dA\x1b[0J", r.lastLines)
	}
	out.WriteString(content)
	r.lastLines = visibleLineCount(content, r.termWidth())
	_, _ = io.WriteString(r.w, out.String())
}

// commitLocked emits a trailing blank line and resets state after all sections ended.
// Must be called with r.mu held, after a final redrawLocked.
func (r *ReasoningRenderer) commitLocked() {
	_, _ = io.WriteString(r.w, "\n")
	r.sections = nil
	r.lastLines = 0
}

// flushSectionLocked writes a section atomically on non-TTY when it ends.
// Must be called with r.mu held.
func (r *ReasoningRenderer) flushSectionLocked(sec *reasoningSection) {
	body := sec.buf.String()
	if body != "" {
		if r.useANSI {
			_, _ = fmt.Fprintf(r.w, "\x1b[3;90m%s\x1b[0m", body)
		} else {
			_, _ = io.WriteString(r.w, body)
		}
		if !strings.HasSuffix(body, "\n") {
			_, _ = io.WriteString(r.w, "\n")
		}
	}
	_, _ = io.WriteString(r.w, "\n")
}

func (r *ReasoningRenderer) buildLiveAreaLocked() string {
	width := r.termWidth()
	// Reserve one row for the line below the live area so the cursor never
	// pushes content into the scrollback buffer.
	budget := r.termHeight() - 1
	if budget < 2 {
		budget = 2
	}

	var out strings.Builder
	for _, sec := range r.sections {
		if budget <= 0 {
			break
		}
		body := sec.buf.String()
		if body == "" {
			continue
		}
		if rows := visibleLineCount(body, width); rows > budget {
			body = capToRows(body, budget, width)
		}
		if r.useANSI {
			fmt.Fprintf(&out, "\x1b[3;90m%s\x1b[0m", body)
		} else {
			out.WriteString(body)
		}
		if !strings.HasSuffix(body, "\n") {
			out.WriteString("\n")
		}
		budget -= visibleLineCount(body, width)
	}
	return out.String()
}

// capToRows truncates plain-text content to at most maxRows visual rows,
// keeping the most-recent lines and prepending "…\n" as the first row.
func capToRows(content string, maxRows, width int) string {
	lines := strings.Split(content, "\n")
	hasTrailing := len(lines) > 0 && lines[len(lines)-1] == ""
	if hasTrailing {
		lines = lines[:len(lines)-1]
	}
	// Walk from the end, accumulating rows until budget (maxRows-1) is full.
	budget := maxRows - 1 // one row reserved for "…"
	kept := 0
	used := 0
	for i := len(lines) - 1; i >= 0; i-- {
		lr := len([]rune(lines[i]))
		lineRows := (lr + width - 1) / width
		if lineRows == 0 {
			lineRows = 1
		}
		if used+lineRows > budget {
			break
		}
		used += lineRows
		kept++
	}
	start := len(lines) - kept
	result := "…\n" + strings.Join(lines[start:], "\n")
	if hasTrailing {
		result += "\n"
	}
	return result
}


// WriteProgress writes a pre-formatted progress line. When a live area is
// active on TTY it erases the live area, writes the line, then redraws so the
// cursor accounting stays correct. Safe to call concurrently.
func (r *ReasoningRenderer) WriteProgress(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isTTY && r.lastLines > 0 {
		var out strings.Builder
		fmt.Fprintf(&out, "\x1b[%dA\x1b[0J", r.lastLines)
		r.lastLines = 0
		out.WriteString(line)
		if !strings.HasSuffix(line, "\n") {
			out.WriteString("\n")
		}
		_, _ = io.WriteString(r.w, out.String())
		r.redrawLocked()
	} else {
		_, _ = io.WriteString(r.w, line)
		if !strings.HasSuffix(line, "\n") {
			_, _ = io.WriteString(r.w, "\n")
		}
	}
}

func (r *ReasoningRenderer) termWidth() int {
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
	if r.fd < 0 {
		return 24
	}
	_, h, err := term.GetSize(r.fd)
	if err != nil || h <= 0 {
		return 24
	}
	return h
}

// visibleLineCount counts the number of terminal rows the string occupies,
// stripping ANSI escape sequences and accounting for line wrapping.
func visibleLineCount(s string, width int) int {
	if width <= 0 {
		width = 80
	}
	stripped := stripANSI(s)
	lines := strings.Split(stripped, "\n")
	// Trailing newline produces an empty last element; don't count it.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total := 0
	for _, line := range lines {
		runes := []rune(line)
		if len(runes) == 0 {
			total++
			continue
		}
		total += (len(runes) + width - 1) / width
	}
	return total
}

func stripANSI(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && !isANSIFinalByte(s[i]) {
				i++
			}
			if i < len(s) {
				i++ // consume the final byte
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
