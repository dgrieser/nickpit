package review

import (
	"context"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
)

// SummarizeUpdate uses the default workflow's finding-shard and overall passes,
// including @small routing and warning-only fallback. Unchanged and resolved
// findings are not rewritten as a side effect of correcting another finding.
func (e *Engine) SummarizeUpdate(ctx context.Context, in *model.ReviewResult, changed []model.Finding, verdictRun model.AgentRun, verdictHasFindings bool, req model.ReviewRequest) (*model.ReviewResult, model.TokenUsage, error) {
	spec := workflow.UpdateSpec()
	return summarizeUpdate(ctx, e.stepContext(spec.Steps[2].Config, req), in, changed, verdictRun, verdictHasFindings)
}

func summarizeUpdate(ctx context.Context, sc *stepContext, in *model.ReviewResult, changed []model.Finding, verdictRun model.AgentRun, verdictHasFindings bool) (*model.ReviewResult, model.TokenUsage, error) {
	out, err := in.Clone()
	if err != nil {
		return nil, model.TokenUsage{}, err
	}
	var usage model.TokenUsage
	selected := &model.ReviewResult{}
	for _, finding := range changed {
		if finding.Resolution == nil {
			selected.Findings = append(selected.Findings, finding)
		}
	}
	if len(selected.Findings) > 0 {
		summarized, run, warnings := runSummarizeShard(ctx, sc, selected, "")
		if run != nil {
			usage = addTokenUsage(usage, run.TokensUsed)
		}
		out.Warnings = append(out.Warnings, warnings...)
		byID := make(map[string]model.Finding, len(summarized.Findings))
		for _, finding := range summarized.Findings {
			byID[finding.ID] = finding
		}
		for i, finding := range out.Findings {
			if replacement, ok := byID[finding.ID]; ok {
				out.Findings[i] = replacement
			}
		}
	}
	if verdictHasFindings || verdictAgentWroteOverall(&verdictRun) {
		overall, run, warnings := runOverallSummarize(ctx, sc, out.OverallExplanation)
		out.OverallExplanation = overall
		if run != nil {
			usage = addTokenUsage(usage, run.TokensUsed)
		}
		out.Warnings = append(out.Warnings, warnings...)
	}
	return out, usage, nil
}
