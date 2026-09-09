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
	InitiatingMessage   string
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
	stages := workflow.UpdateStages()
	// Clone normalizes legacy location fields just as UpdateFindings does;
	// normalization alone must not turn an unchanged finding into a correction.
	before, err := req.Result.Clone()
	if err != nil {
		return nil, err
	}
	baseReq := model.ReviewRequest{
		RepoRoot: req.RepoRoot, DisableJSONResponseFormat: e.config.DisableJSONResponseFormat,
		MaxOutputRetries: req.MaxOutputRetries, MaxReasoningSeconds: req.MaxReasoningSeconds,
		DisableParallelToolCalls: req.DisableParallelToolCalls, DisablePatchSummary: req.DisablePatchSummary,
		DisableSuggestions: req.DisableSuggestions,
	}
	updateStep := e.stepContext(stages.Update.Config, baseReq)
	updateReq := req.UpdateFindingsRequest
	updateReq.MaxOutputRetries = updateStep.Req.MaxOutputRetries
	updateReq.MaxReasoningSeconds = updateStep.Req.MaxReasoningSeconds
	updateReq.DisableParallelToolCalls = updateStep.Req.DisableParallelToolCalls
	updateReq.DisableSuggestions = updateStep.Req.DisableSuggestions
	after, report, err := updateStep.Engine.UpdateFindings(ctx, updateReq)
	if err != nil {
		return nil, fmt.Errorf("update workflow: checking evidence: %w", err)
	}
	changed := changedUpdateFindings(before, after)
	var usage model.TokenUsage
	usage = addTokenUsage(usage, report.TokensUsed)
	publish := len(changed) > 0 || report.ReviewCorrectionWarranted()
	if !publish {
		return updateWorkflowResult(after, report, changed, usage, false), nil
	}

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
	if req.InitiatingMessage != "" {
		notes.WriteString("\n\nInitiating author message:\n")
		notes.WriteString(req.InitiatingMessage)
	} else {
		for _, message := range slices.Backward(req.Messages) {
			if message.Role == "user" {
				notes.WriteString("\n\nLatest author message:\n")
				notes.WriteString(message.Content)
				break
			}
		}
	}
	verdictStep := e.stepContext(stages.Verdict.Config, baseReq)
	verdict, verdictRun, err := verdictStep.Engine.Verdict(ctx, req.ReviewCtx, verdictInput, VerdictOptions{
		RepoRoot: req.RepoRoot, DiffFormat: req.DiffFormat, DisableSuggestions: verdictStep.Req.DisableSuggestions,
		DisableJSONResponseFormat: verdictStep.Req.DisableJSONResponseFormat, MaxOutputRetries: verdictStep.Req.MaxOutputRetries,
		MaxReasoningSeconds: verdictStep.Req.MaxReasoningSeconds, DisableParallelToolCalls: verdictStep.Req.DisableParallelToolCalls,
		DisablePatchSummary: verdictStep.Req.DisablePatchSummary, PriorityThreshold: req.PriorityThreshold,
		ConfidenceThreshold: req.ConfidenceThreshold, ContextNotes: notes.String(),
		ContextMessages: req.Messages,
	})
	if err != nil {
		return nil, fmt.Errorf("update workflow: verdict: %w", err)
	}
	usage = addTokenUsage(usage, verdictRun.TokensUsed)
	// Verdict filtering must not remove stored findings or their identities.
	after.OverallCorrectness, after.OverallExplanation, after.OverallConfidenceScore = verdict.OverallCorrectness, verdict.OverallExplanation, verdict.OverallConfidenceScore

	summaryStep := e.stepContext(stages.Summarize.Config, baseReq)
	after, summaryUsage, err := summarizeUpdate(ctx, summaryStep, after, changed, verdictRun, len(verdict.Findings) > 0)
	if err != nil {
		return nil, fmt.Errorf("update workflow: summarize: %w", err)
	}
	usage = addTokenUsage(usage, summaryUsage)
	changed = changedUpdateFindings(before, after)
	return updateWorkflowResult(after, report, changed, usage, true), nil
}

func updateWorkflowResult(after *model.ReviewResult, report FindingUpdateReport, changed []model.Finding, usage model.TokenUsage, publish bool) *UpdateWorkflowResult {
	return &UpdateWorkflowResult{Review: after, Publish: publish, Outcome: ReviewUpdateOutcome{
		TokensUsed: usage, Checks: report.Checks, ReviewCheck: report.ReviewCheck, Changed: changed,
		OverallCorrectness: after.OverallCorrectness, OverallExplanation: after.OverallExplanation,
	}}
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
