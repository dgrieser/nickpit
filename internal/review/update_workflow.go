package review

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
)

type UpdateWorkflowRequest struct {
	UpdateFindingsRequest
	PriorityThreshold   string
	ConfidenceThreshold float64
	DisablePatchSummary bool
}

type UpdateWorkflowResult struct {
	Review  *model.ReviewResult
	Outcome ReviewUpdateOutcome
	Publish bool
}

// RunUpdateWorkflow executes workflows/update.yaml, the built-in correction workflow:
// update -> (when warranted) verdict -> summarize. It deliberately accepts no
// workflow spec or step overrides. SCM publication belongs to the durable job,
// which checkpoints this result before writing any comments.
func (e *Engine) RunUpdateWorkflow(ctx context.Context, req UpdateWorkflowRequest) (*UpdateWorkflowResult, error) {
	spec := workflow.UpdateSpec()
	// Clone normalizes legacy location fields just as UpdateFindings does;
	// normalization alone must not turn an unchanged finding into a correction.
	before, err := req.Result.Clone()
	if err != nil {
		return nil, err
	}
	after := before
	var report FindingUpdateReport
	var changed []model.Finding
	var usage model.TokenUsage
	var verdictRun model.AgentRun
	var verdictHasFindings, publish bool
	for _, step := range spec.Steps {
		if step.Type != "update" && !publish {
			continue
		}
		sc := e.stepContext(step.Config, model.ReviewRequest{
			RepoRoot: req.RepoRoot, DisableJSONResponseFormat: e.config.DisableJSONResponseFormat,
			MaxOutputRetries: req.MaxOutputRetries, MaxReasoningSeconds: req.MaxReasoningSeconds,
			DisableParallelToolCalls: req.DisableParallelToolCalls, DisablePatchSummary: req.DisablePatchSummary,
			DisableSuggestions: req.DisableSuggestions,
		})
		switch step.Type {
		case "update":
			updateReq := req.UpdateFindingsRequest
			updateReq.MaxOutputRetries = sc.Req.MaxOutputRetries
			updateReq.MaxReasoningSeconds = sc.Req.MaxReasoningSeconds
			updateReq.DisableParallelToolCalls = sc.Req.DisableParallelToolCalls
			updateReq.DisableSuggestions = sc.Req.DisableSuggestions
			after, report, err = sc.Engine.UpdateFindings(ctx, updateReq)
			if err != nil {
				return nil, fmt.Errorf("update workflow: checking evidence: %w", err)
			}
			changed = changedUpdateFindings(before, after)
			usage = addTokenUsage(usage, report.TokensUsed)
			publish = len(changed) > 0 || report.ReviewCorrectionWarranted()
		case workflow.StepVerdict:
			verdictInput, err := after.Clone()
			if err != nil {
				return nil, err
			}
			verdictInput.OverallCorrectness, verdictInput.OverallExplanation, verdictInput.OverallConfidenceScore = "", "", 0
			var notes strings.Builder
			notes.WriteString(req.Signal.Reason)
			if report.ReviewCheck != nil {
				notes.WriteString("\n\nIndependent evidence assessment:\n")
				notes.WriteString(report.ReviewCheck.Reason)
			}
			for _, message := range slices.Backward(req.Messages) {
				if message.Role == "user" {
					notes.WriteString("\n\nLatest author message:\n")
					notes.WriteString(message.Content)
					break
				}
			}
			verdict, run, err := sc.Engine.Verdict(ctx, req.ReviewCtx, verdictInput, VerdictOptions{
				RepoRoot: req.RepoRoot, DiffFormat: req.DiffFormat, DisableSuggestions: sc.Req.DisableSuggestions,
				DisableJSONResponseFormat: sc.Req.DisableJSONResponseFormat, MaxOutputRetries: sc.Req.MaxOutputRetries,
				MaxReasoningSeconds: sc.Req.MaxReasoningSeconds, DisableParallelToolCalls: sc.Req.DisableParallelToolCalls,
				DisablePatchSummary: sc.Req.DisablePatchSummary, PriorityThreshold: req.PriorityThreshold,
				ConfidenceThreshold: req.ConfidenceThreshold, ContextNotes: notes.String(),
			})
			if err != nil {
				return nil, fmt.Errorf("update workflow: verdict: %w", err)
			}
			usage = addTokenUsage(usage, run.TokensUsed)
			// Verdict filtering must not remove stored findings or their identities.
			after.OverallCorrectness, after.OverallExplanation, after.OverallConfidenceScore = verdict.OverallCorrectness, verdict.OverallExplanation, verdict.OverallConfidenceScore
			verdictRun, verdictHasFindings = run, len(verdict.Findings) > 0
		case workflow.StepSummarize:
			var summaryUsage model.TokenUsage
			after, summaryUsage, err = summarizeUpdate(ctx, sc, after, changed, verdictRun, verdictHasFindings)
			if err != nil {
				return nil, fmt.Errorf("update workflow: summarize: %w", err)
			}
			usage = addTokenUsage(usage, summaryUsage)
			changed = changedUpdateFindings(before, after)
		}
	}
	return &UpdateWorkflowResult{Review: after, Publish: publish, Outcome: ReviewUpdateOutcome{
		TokensUsed: usage, Checks: report.Checks, ReviewCheck: report.ReviewCheck, Changed: changed,
		OverallCorrectness: after.OverallCorrectness, OverallExplanation: after.OverallExplanation,
	}}, nil
}

func changedUpdateFindings(before, after *model.ReviewResult) []model.Finding {
	byID := make(map[string]model.Finding, len(before.Findings))
	for _, finding := range before.Findings {
		byID[finding.ID] = finding
	}
	var changed []model.Finding
	for _, finding := range after.Findings {
		if !reflect.DeepEqual(byID[finding.ID], finding) {
			changed = append(changed, finding)
		}
	}
	return changed
}
