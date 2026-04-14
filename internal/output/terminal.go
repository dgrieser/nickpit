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
	counts := map[model.Severity]int{}
	for _, finding := range result.Findings {
		counts[finding.Severity]++
	}
	_, err := fmt.Fprintf(f.w, "NickPit Review\n\n%d findings (%d critical, %d error, %d warning, %d info)\n\n",
		len(result.Findings), counts[model.SeverityCritical], counts[model.SeverityError], counts[model.SeverityWarning], counts[model.SeverityInfo],
	)
	if err != nil {
		return err
	}
	sort.Slice(result.Findings, func(i, j int) bool {
		return model.SeverityRank(result.Findings[i].Severity) > model.SeverityRank(result.Findings[j].Severity)
	})
	for _, finding := range result.Findings {
		if _, err := fmt.Fprintf(f.w, "%s %s:%d-%d [%s]\n%s\nConfidence: %.2f\n\n",
			f.colorize(strings.ToUpper(string(finding.Severity)), finding.Severity),
			finding.FilePath, finding.StartLine, max(finding.EndLine, finding.StartLine),
			finding.Category, finding.Title+"\n"+finding.Description, finding.Confidence,
		); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(f.w, "Summary: %s\nTokens: %d prompt / %d completion / %d total\n",
		result.Summary, result.TokensUsed.PromptTokens, result.TokensUsed.CompletionTokens, result.TokensUsed.TotalTokens)
	return err
}

func (f *TerminalFormatter) colorize(text string, severity model.Severity) string {
	if !f.useANSI {
		return text
	}
	code := "36"
	switch severity {
	case model.SeverityCritical:
		code = "1;31"
	case model.SeverityError:
		code = "31"
	case model.SeverityWarning:
		code = "33"
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
