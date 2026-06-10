package output

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/textsan"
	"golang.org/x/term"
)

type Formatter interface {
	FormatFindings(result *model.ReviewResult) error
}

const (
	terminalDefaultWidth = 80
	terminalMinWidth     = 60
	terminalMaxWidth     = 120
)

// TerminalFormatter renders the review result the way it appears as published
// GitLab/GitHub comments: the summary comment first, then one comment per
// finding, with badge labels in place of the badge SVGs and markdown bodies
// rendered for the terminal. A dim footer keeps token usage and warnings.
type TerminalFormatter struct {
	w       io.Writer
	useANSI bool
	width   int
}

func NewTerminalFormatter(w io.Writer, useANSI bool) *TerminalFormatter {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		useANSI = false
	}
	width := terminalDefaultWidth
	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			width = min(max(w, terminalMinWidth), terminalMaxWidth)
		}
	}
	return &TerminalFormatter{w: w, useANSI: useANSI, width: width}
}

// SetWidth overrides the detected terminal width (test hook).
func (f *TerminalFormatter) SetWidth(n int) {
	if n > 0 {
		f.width = n
	}
}

func (f *TerminalFormatter) FormatFindings(result *model.ReviewResult) error {
	sortFindings(result.Findings)

	var b strings.Builder
	f.writeSummary(&b, result)
	for _, finding := range result.Findings {
		f.writeRule(&b)
		f.writeFinding(&b, finding)
	}
	f.writeRule(&b)
	f.writeFooter(&b, result)

	_, err := io.WriteString(f.w, b.String())
	return err
}

// writeSummary renders the overall verdict comment: badge, confidence, and the
// explanation as rendered markdown.
func (f *TerminalFormatter) writeSummary(b *strings.Builder, result *model.ReviewResult) {
	correctness := strings.TrimSpace(result.OverallCorrectness)
	if correctness == "" {
		// No verdict to badge; fall back to plain text like the publisher.
		b.WriteString(f.bold("review complete"))
	} else {
		b.WriteString(correctnessBadge(correctness, f.useANSI))
	}
	b.WriteString("\n")
	b.WriteString(f.dim(reviewmd.ConfidencePercent(result.OverallConfidenceScore)))
	b.WriteString("\n")
	if explanation := textsan.StripControl(strings.TrimSpace(result.OverallExplanation)); explanation != "" {
		b.WriteString("\n")
		b.WriteString(f.renderMarkdown(explanation))
		b.WriteString("\n")
	}
}

// writeFinding renders one finding comment: badge, confidence, location, and
// the title/body/suggestions markdown.
func (f *TerminalFormatter) writeFinding(b *strings.Builder, finding model.Finding) {
	_, _, rank, confidence := reviewmd.FindingDisplay(finding)
	b.WriteString(priorityBadge(rank, f.useANSI))
	b.WriteString("\n")
	b.WriteString(f.dim(reviewmd.ConfidencePercent(confidence)))
	b.WriteString("\n\n")
	b.WriteString(f.bold(findingLocation(finding)))
	b.WriteString("\n\n")
	b.WriteString(f.renderMarkdown(findingMarkdown(finding)))
	b.WriteString("\n")
}

func (f *TerminalFormatter) writeFooter(b *strings.Builder, result *model.ReviewResult) {
	for _, warning := range result.Warnings {
		b.WriteString(f.yellow("! " + textsan.StripControl(warning)))
		b.WriteString("\n")
	}
	b.WriteString(f.dim(fmt.Sprintf("Tokens: %d prompt / %d completion / %d total",
		result.TokensUsed.PromptTokens, result.TokensUsed.CompletionTokens, result.TokensUsed.TotalTokens)))
	b.WriteString("\n")
}

func (f *TerminalFormatter) writeRule(b *strings.Builder) {
	b.WriteString("\n")
	if f.useANSI {
		b.WriteString(f.dim(strings.Repeat("─", f.width)))
	} else {
		b.WriteString("---")
	}
	b.WriteString("\n\n")
}

// findingLocation renders the file:line anchor shown above each finding — the
// terminal equivalent of the platform inline anchor (and identical in shape to
// the publisher's general-comment location prefix).
func findingLocation(finding model.Finding) string {
	path := textsan.StripControl(finding.CodeLocation.FilePath)
	start := finding.CodeLocation.LineRange.Start
	end := finding.CodeLocation.LineRange.End
	if end > start {
		return fmt.Sprintf("%s:%d-%d", path, start, end)
	}
	return fmt.Sprintf("%s:%d", path, start)
}

// findingMarkdown builds the markdown one would see in the published comment:
// title heading, body, and the suggestions block. Markers and hard breaks are
// publishing concerns and are deliberately absent; the renderer wraps at a
// known width.
func findingMarkdown(finding model.Finding) string {
	title, body, _, _ := reviewmd.FindingDisplay(finding)
	var b strings.Builder
	if title = strings.TrimSpace(title); title != "" {
		b.WriteString("### ")
		b.WriteString(textsan.StripControl(title))
		b.WriteString("\n\n")
	}
	b.WriteString(textsan.StripControl(strings.TrimSpace(body)))
	suggestions := make([]string, 0, len(finding.Suggestions))
	for _, suggestion := range finding.Suggestions {
		text := strings.TrimSpace(suggestion.Body)
		if text == "" {
			continue
		}
		suggestions = append(suggestions, strings.ReplaceAll(textsan.StripControl(text), "\n", "\n  "))
	}
	if len(suggestions) > 0 {
		b.WriteString("\n\n**Suggestions**\n")
		for _, suggestion := range suggestions {
			b.WriteString("\n- ")
			b.WriteString(suggestion)
		}
	}
	return b.String()
}

// renderMarkdown renders markdown for the terminal. Plain mode returns the raw
// markdown (deterministic, pipe-friendly); rendering errors fall back to the
// raw markdown — never fail the review output over styling.
func (f *TerminalFormatter) renderMarkdown(md string) string {
	if !f.useANSI {
		return md
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(f.width),
	)
	if err != nil {
		return md
	}
	rendered, err := renderer.Render(md)
	if err != nil {
		return md
	}
	return strings.Trim(rendered, "\n")
}

// sortFindings orders findings by priority rank, refuted-last within a rank,
// then by verification confidence descending. Verification is no longer shown
// but still drives a useful reading order.
func sortFindings(findings []model.Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		ri := displayRank(findings[i])
		rj := displayRank(findings[j])
		if ri != rj {
			return ri < rj
		}
		invI := findings[i].Verification != nil && findings[i].Verification.Verdict == model.VerdictRefuted
		invJ := findings[j].Verification != nil && findings[j].Verification.Verdict == model.VerdictRefuted
		if invI != invJ {
			return !invI
		}
		ci := 0.0
		if findings[i].Verification != nil {
			ci = findings[i].Verification.ConfidenceScore
		}
		cj := 0.0
		if findings[j].Verification != nil {
			cj = findings[j].Verification.ConfidenceScore
		}
		return ci > cj
	})
}

func displayRank(finding model.Finding) int {
	_, _, rank, _ := reviewmd.FindingDisplay(finding)
	return rank
}

func (f *TerminalFormatter) dim(text string) string {
	if !f.useANSI || text == "" {
		return text
	}
	return "\x1b[2m" + text + "\x1b[0m"
}

func (f *TerminalFormatter) bold(text string) string {
	if !f.useANSI || text == "" {
		return text
	}
	return "\x1b[1m" + text + "\x1b[0m"
}

func (f *TerminalFormatter) yellow(text string) string {
	if !f.useANSI || text == "" {
		return text
	}
	return "\x1b[33m" + text + "\x1b[0m"
}
