package output

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/dgrieser/nickpit/internal/model"
)

type Formatter interface {
	FormatFindings(result *model.ReviewResult) error
}

type TerminalFormatter struct {
	w       io.Writer
	useANSI bool
}

func NewTerminalFormatter(w io.Writer, useANSI bool) *TerminalFormatter {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		useANSI = false
	}
	return &TerminalFormatter{w: w, useANSI: useANSI}
}

func (f *TerminalFormatter) FormatFindings(result *model.ReviewResult) error {
	counts := map[int]int{}
	for _, finding := range result.Findings {
		counts[model.PriorityRank(finding.Priority)]++
	}
	_, err := fmt.Fprintf(f.w, "NickPit Review\n\n%d findings (%d P0, %d P1, %d P2, %d P3)\n\n",
		len(result.Findings), counts[0], counts[1], counts[2], counts[3],
	)
	if err != nil {
		return err
	}
	sort.Slice(result.Findings, func(i, j int) bool {
		return model.PriorityRank(result.Findings[i].Priority) < model.PriorityRank(result.Findings[j].Priority)
	})
	for _, finding := range result.Findings {
		if _, err := fmt.Fprintf(f.w, "%s %s:%d-%d\n%s\n%s\nConfidence: %.2f\n\n",
			f.colorize(priorityLabel(finding.Priority), model.PriorityRank(finding.Priority)),
			finding.CodeLocation.FilePath,
			finding.CodeLocation.LineRange.Start,
			max(finding.CodeLocation.LineRange.End, finding.CodeLocation.LineRange.Start),
			finding.Title,
			finding.Body,
			finding.ConfidenceScore,
		); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(f.w, "Overall: %s\n%s\nConfidence: %.2f\nTokens: %d prompt / %d completion / %d total\n",
		result.OverallCorrectness, result.OverallExplanation, result.OverallConfidenceScore,
		result.TokensUsed.PromptTokens, result.TokensUsed.CompletionTokens, result.TokensUsed.TotalTokens)
	return err
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
