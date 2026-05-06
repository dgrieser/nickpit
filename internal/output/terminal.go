package output

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

type Formatter interface {
	FormatFindings(result *model.ReviewResult) error
}

type TerminalFormatter struct {
	w           io.Writer
	useANSI     bool
	hideInvalid bool
}

func NewTerminalFormatter(w io.Writer, useANSI bool) *TerminalFormatter {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		useANSI = false
	}
	return &TerminalFormatter{w: w, useANSI: useANSI}
}

func (f *TerminalFormatter) WithHideInvalid(hide bool) *TerminalFormatter {
	f.hideInvalid = hide
	return f
}

func (f *TerminalFormatter) FormatFindings(result *model.ReviewResult) error {
	counts := map[int]int{}
	verifiedCount := 0
	validCount := 0
	invalidCount := 0
	for _, finding := range result.Findings {
		counts[model.PriorityRank(finding.Priority)]++
		if finding.Verification != nil {
			verifiedCount++
			if finding.Verification.Valid {
				validCount++
			} else {
				invalidCount++
			}
		}
	}
	header := fmt.Sprintf("NickPit Review\n\n%d findings (%d P0, %d P1, %d P2, %d P3)",
		len(result.Findings), counts[0], counts[1], counts[2], counts[3],
	)
	if verifiedCount > 0 {
		header += fmt.Sprintf(" · verified: %d valid, %d invalid", validCount, invalidCount)
	}
	if _, err := fmt.Fprintf(f.w, "%s\n\n", header); err != nil {
		return err
	}
	sort.SliceStable(result.Findings, func(i, j int) bool {
		ri := model.PriorityRank(result.Findings[i].Priority)
		rj := model.PriorityRank(result.Findings[j].Priority)
		if ri != rj {
			return ri < rj
		}
		invI := result.Findings[i].Verification != nil && !result.Findings[i].Verification.Valid
		invJ := result.Findings[j].Verification != nil && !result.Findings[j].Verification.Valid
		if invI != invJ {
			return !invI
		}
		ci := 0.0
		if result.Findings[i].Verification != nil {
			ci = result.Findings[i].Verification.ConfidenceScore
		}
		cj := 0.0
		if result.Findings[j].Verification != nil {
			cj = result.Findings[j].Verification.ConfidenceScore
		}
		return ci > cj
	})
	for _, finding := range result.Findings {
		if f.hideInvalid && finding.Verification != nil && !finding.Verification.Valid {
			continue
		}
		header := fmt.Sprintf("%s %s:%d-%d",
			f.colorize(priorityLabel(finding.Priority), model.PriorityRank(finding.Priority)),
			finding.CodeLocation.FilePath,
			finding.CodeLocation.LineRange.Start,
			max(finding.CodeLocation.LineRange.End, finding.CodeLocation.LineRange.Start),
		)
		if _, err := fmt.Fprintln(f.w, header); err != nil {
			return err
		}
		if line := f.renderVerification(finding.Verification, model.PriorityRank(finding.Priority)); line != "" {
			if _, err := fmt.Fprintln(f.w, line); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(f.w, "%s\n%s\nConfidence: %.2f\n\n",
			finding.Title, finding.Body, finding.ConfidenceScore,
		); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(f.w, "Overall: %s\n%s\nConfidence: %.2f\nTokens: %d prompt / %d completion / %d total\n",
		result.OverallCorrectness, result.OverallExplanation, result.OverallConfidenceScore,
		result.TokensUsed.PromptTokens, result.TokensUsed.CompletionTokens, result.TokensUsed.TotalTokens)
	return err
}

func (f *TerminalFormatter) renderVerification(v *model.FindingVerification, originalRank int) string {
	if v == nil {
		return ""
	}
	glyphValid := "✓ verified"
	glyphInvalid := "✗ invalid"
	if !f.useANSI {
		glyphValid = "[ok] verified"
		glyphInvalid = "[bad] invalid"
	}
	var b strings.Builder
	b.WriteString("  ")
	if v.Valid {
		b.WriteString(f.colorVerifyValid(glyphValid))
	} else {
		b.WriteString(f.colorVerifyInvalid(glyphInvalid, v.ConfidenceScore))
	}
	b.WriteString(fmt.Sprintf("  conf %.2f", v.ConfidenceScore))
	if v.Priority != originalRank {
		arrow := fmt.Sprintf("  P%d→P%d", originalRank, v.Priority)
		b.WriteString(f.colorPriorityArrow(arrow))
	}
	if remarks := strings.TrimSpace(v.Remarks); remarks != "" {
		b.WriteString("\n  remark: ")
		b.WriteString(truncateRemark(remarks, 200))
	}
	return b.String()
}

func truncateRemark(text string, max int) string {
	if len(text) <= max {
		return text
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "…"
}

func (f *TerminalFormatter) colorVerifyValid(text string) string {
	if !f.useANSI {
		return text
	}
	return "\x1b[2;32m" + text + "\x1b[0m"
}

func (f *TerminalFormatter) colorVerifyInvalid(text string, confidence float64) string {
	if !f.useANSI {
		return text
	}
	code := "31"
	if confidence >= 0.8 {
		code = "1;31"
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (f *TerminalFormatter) colorPriorityArrow(text string) string {
	if !f.useANSI {
		return text
	}
	return "\x1b[33m" + text + "\x1b[0m"
}

func (f *TerminalFormatter) colorize(text string, priority int) string {
	if !f.useANSI {
		return text
	}
	code := "36"
	switch priority {
	case 0:
		code = "1;31"
	case 1:
		code = "31"
	case 2:
		code = "33"
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func priorityLabel(priority *int) string {
	return fmt.Sprintf("P%d", model.PriorityRank(priority))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
