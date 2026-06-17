package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
)

type VerdictOptions struct {
	UseJSONSchema            bool
	MaxOutputRetries         int
	MaxReasoningSeconds      int
	MaxReasoningLoopRepeats  int
	DisableParallelToolCalls bool
	DisablePatchSummary      bool
	RepoRoot                 string
	ContextNotes             string
}

func (e *Engine) Verdict(ctx context.Context, reviewCtx *model.ReviewContext, in *model.ReviewResult, opts VerdictOptions) (*model.ReviewResult, model.AgentRun, error) {
	if reviewCtx == nil {
		return nil, model.AgentRun{}, fmt.Errorf("verdict: nil review context")
	}
	if in == nil {
		return nil, model.AgentRun{}, fmt.Errorf("verdict: nil review result")
	}
	if len(in.Findings) == 0 {
		out, err := in.Clone()
		if err != nil {
			return nil, model.AgentRun{}, fmt.Errorf("verdict: cloning input result: %w", err)
		}
		out.OverallCorrectness = "patch is correct"
		if strings.TrimSpace(out.OverallExplanation) == "" {
			out.OverallExplanation = "No finalized findings remained."
		}
		return out, model.AgentRun{Name: "Verdict Review", Role: "verdict", Status: model.AgentRunStatusSkipped}, nil
	}

	systemTemplate, err := e.loadPrompt("agent_verdict_system_prompt.tmpl")
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("verdict", verdictOutputSchemaSnippetFor(opts.UseJSONSchema))
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet string
		OutputFormatSnippet string
		DisablePatchSummary bool
	}{
		OutputSchemaSnippet: verdictOutputSchemaSnippetFor(opts.UseJSONSchema),
		OutputFormatSnippet: commonSnippets.outputFormat,
		DisablePatchSummary: opts.DisablePatchSummary,
	})
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("verdict: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildVerdictUserPrompt(reviewCtx, in, opts.ContextNotes)
	if err != nil {
		return nil, model.AgentRun{}, err
	}

	var schema []byte
	constraints := verdictConstraintsFor(in.Findings)
	if opts.UseJSONSchema {
		if hasResponseConstraints(constraints) {
			schema = llm.VerdictSchemaWithConstraints(constraints)
		} else {
			schema = llm.VerdictSchema
		}
	}
	req := model.ReviewRequest{
		RepoRoot:                 opts.RepoRoot,
		MaxOutputRetries:         opts.MaxOutputRetries,
		MaxReasoningSeconds:      opts.MaxReasoningSeconds,
		MaxReasoningLoopRepeats:  opts.MaxReasoningLoopRepeats,
		DisableParallelToolCalls: opts.DisableParallelToolCalls,
		UseJSONSchema:            opts.UseJSONSchema,
	}
	verdictStart := time.Now()
	e.logProgress(logging.StageVerdict, logging.StateStart, fmt.Sprintf("findings=%d", len(in.Findings)))
	result, err := e.runAgent(ctx, agentSpec{
		name:             "Verdict Review",
		role:             "verdict",
		system:           system,
		noToolsSystem:    system,
		user:             userPrompt,
		schema:           schema,
		schemaKind:       llm.SchemaKindVerdict,
		constraints:      constraints,
		hasTools:         false,
		validateResponse: verdictOutputValidator(),
	}, req)
	if err != nil {
		return nil, result.run, err
	}
	if result.resp == nil {
		return nil, result.run, fmt.Errorf("verdict: agent returned nil response")
	}

	out, err := in.Clone()
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("verdict: cloning input result: %w", err)
	}
	out.OverallCorrectness = result.resp.OverallCorrectness
	out.OverallExplanation = result.resp.OverallExplanation
	out.OverallConfidenceScore = result.resp.OverallConfidenceScore
	e.logProgress(logging.StageVerdict, logging.StateDone, fmt.Sprintf("findings=%d prompt_tokens=%s completion_tokens=%s total_tokens=%s runtime=%s", len(in.Findings), model.HumanTokens(result.run.TokensUsed.PromptTokens), model.HumanTokens(result.run.TokensUsed.CompletionTokens), model.HumanTokens(result.run.TokensUsed.TotalTokens), model.HumanDuration(time.Since(verdictStart))))
	return out, result.run, nil
}

func verdictOutputValidator() func(*llm.ReviewResponse) *llm.InvalidResponseError {
	return func(resp *llm.ReviewResponse) *llm.InvalidResponseError {
		if resp != nil && strings.TrimSpace(resp.OverallCorrectness) != "" && strings.TrimSpace(resp.OverallExplanation) != "" {
			return nil
		}
		raw := ""
		reasoningEffort := ""
		if resp != nil {
			raw = resp.RawResponse
			reasoningEffort = resp.ReasoningEffort
		}
		return &llm.InvalidResponseError{
			RawContent:      raw,
			Reason:          "verdict_output_mismatch",
			MissingFields:   []string{"overall_correctness", "overall_explanation", "overall_confidence_score"},
			ReasoningEffort: reasoningEffort,
		}
	}
}

func (e *Engine) buildVerdictUserPrompt(reviewCtx *model.ReviewContext, in *model.ReviewResult, contextNotes string) (string, error) {
	payload := model.PromptPayloadFromContext(reviewCtx)
	guides, err := e.styleGuidesFor(reviewCtx)
	if err != nil {
		return "", err
	}
	payload.StyleGuides = guides
	contextJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("verdict: marshalling review payload: %w", err)
	}
	findings := make([]map[string]any, 0, len(in.Findings))
	for _, finding := range in.Findings {
		entry := map[string]any{
			"id":                      finding.ID,
			"title":                   finding.Title,
			"body":                    finding.Body,
			"priority":                model.PriorityRank(finding.Priority),
			"priority_floor":          findingPriorityFloor(finding),
			"code_location":           finding.CodeLocation,
			"review_confidence_score": finding.ConfidenceScore,
		}
		if finding.Verification != nil {
			verification := *finding.Verification
			model.EnsureVerificationID(&verification, finding.ID)
			entry["verification"] = &verification
		}
		if finding.Finalization != nil {
			entry["finalization"] = finding.Finalization
		}
		findings = append(findings, entry)
	}
	payloadMap := map[string]any{
		"review_context":           json.RawMessage(contextJSON),
		"overall_correctness":      in.OverallCorrectness,
		"overall_explanation":      in.OverallExplanation,
		"overall_confidence_score": in.OverallConfidenceScore,
		"findings":                 findings,
	}
	if strings.TrimSpace(contextNotes) != "" {
		payloadMap["notes"] = contextNotes
	}
	user, err := llm.RenderJSON(payloadMap)
	if err != nil {
		return "", fmt.Errorf("verdict: rendering verdict prompt json: %w", err)
	}
	return user, nil
}

func findingPriorityFloor(finding model.Finding) int {
	floor := model.PriorityRank(finding.Priority)
	if finding.Verification != nil && finding.Verification.Priority < floor {
		floor = finding.Verification.Priority
	}
	return floor
}

// verdictConstraintsFor returns the correctness constraints implied by the
// verified finding priority floor = min(finding.priority, verification.priority).
// P0 blocks the patch, no P0/P1 cannot block it, and P1 remains prompt-judged
// because justification quality cannot be expressed in JSON schema.
func verdictConstraintsFor(in []model.Finding) llm.ResponseConstraints {
	hasP0, hasP1 := false, false
	for _, f := range in {
		switch findingPriorityFloor(f) {
		case 0:
			hasP0 = true
		case 1:
			hasP1 = true
		}
	}
	switch {
	case hasP0:
		return llm.ResponseConstraints{AllowedCorrectness: []string{"patch is incorrect"}}
	case hasP1:
		return llm.ResponseConstraints{}
	default:
		return llm.ResponseConstraints{AllowedCorrectness: []string{"patch is correct"}}
	}
}

func verdictOutputSchemaSnippetFor(useJSONSchema bool) string {
	if useJSONSchema {
		return ""
	}
	return llm.VerdictExamplePromptSnippet()
}
