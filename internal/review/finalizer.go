package review

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

type FinalizeOptions struct {
	UseJSONSchema            bool
	MaxOutputRetries         int
	MaxReasoningSeconds      int
	DisableParallelToolCalls bool
	RepoRoot                 string
}

func (e *Engine) Finalize(ctx context.Context, reviewCtx *model.ReviewContext, in *model.ReviewResult, opts FinalizeOptions) (*model.ReviewResult, model.AgentRun, error) {
	if reviewCtx == nil {
		return nil, model.AgentRun{}, fmt.Errorf("finalize: nil review context")
	}
	if in == nil {
		return nil, model.AgentRun{}, fmt.Errorf("finalize: nil review result")
	}

	systemTemplate, err := e.loadPrompt("agent_finalize_system_prompt.tmpl")
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("finalize", finalizeOutputSchemaSnippetFor(opts.UseJSONSchema))
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		PrioritySnippet     string
		OutputSchemaSnippet string
		OutputFormatSnippet string
	}{
		PrioritySnippet:     commonSnippets.priority,
		OutputSchemaSnippet: finalizeOutputSchemaSnippetFor(opts.UseJSONSchema),
		OutputFormatSnippet: commonSnippets.outputFormat,
	})
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("finalize: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildFinalizeUserPrompt(reviewCtx, in)
	if err != nil {
		return nil, model.AgentRun{}, err
	}

	var schema []byte
	if opts.UseJSONSchema {
		schema = llm.FinalizeSchema
	}

	req := model.ReviewRequest{
		RepoRoot:                 opts.RepoRoot,
		MaxOutputRetries:         opts.MaxOutputRetries,
		MaxReasoningSeconds:      opts.MaxReasoningSeconds,
		DisableParallelToolCalls: opts.DisableParallelToolCalls,
		UseJSONSchema:            opts.UseJSONSchema,
	}
	e.logProgress("Finalize", fmt.Sprintf("findings=%d", len(in.Findings)))
	result, err := e.runReviewAgent(ctx, reviewAgent{
		name:          "finalize",
		role:          "finalize",
		system:        system,
		noToolsSystem: system,
		user:          userPrompt,
		schema:        schema,
		schemaKind:    llm.SchemaKindFinalize,
		hasTools:      false,
	}, req)
	if err != nil {
		return nil, model.AgentRun{}, err
	}

	out := in.Clone()
	out.Findings = result.resp.Findings
	out.OverallCorrectness = result.resp.OverallCorrectness
	out.OverallExplanation = result.resp.OverallExplanation
	out.OverallConfidenceScore = result.resp.OverallConfidenceScore
	e.logProgress("Finalize", fmt.Sprintf("done findings_in=%d findings_out=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d", len(in.Findings), len(out.Findings), result.run.TokensUsed.PromptTokens, result.run.TokensUsed.CompletionTokens, result.run.TokensUsed.TotalTokens))
	return out, result.run, nil
}

func (e *Engine) buildFinalizeUserPrompt(reviewCtx *model.ReviewContext, in *model.ReviewResult) (string, error) {
	payload := model.PromptPayloadFromContext(reviewCtx)
	guides, err := e.styleGuidesFor(reviewCtx)
	if err != nil {
		return "", err
	}
	payload.StyleGuides = guides
	contextJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("finalize: marshalling review payload: %w", err)
	}

	findings := make([]map[string]any, 0, len(in.Findings))
	for _, finding := range in.Findings {
		entry := map[string]any{
			"title":                   finding.Title,
			"body":                    finding.Body,
			"priority":                model.PriorityRank(finding.Priority),
			"code_location":           finding.CodeLocation,
			"review_confidence_score": finding.ConfidenceScore,
		}
		if len(finding.Suggestions) > 0 {
			entry["suggestions"] = finding.Suggestions
		}
		if finding.Verification != nil {
			entry["verification"] = finding.Verification
		}
		findings = append(findings, entry)
	}

	user, err := llm.RenderJSON(map[string]any{
		"review_context":           json.RawMessage(contextJSON),
		"overall_correctness":      in.OverallCorrectness,
		"overall_explanation":      in.OverallExplanation,
		"overall_confidence_score": in.OverallConfidenceScore,
		"findings":                 findings,
	})
	if err != nil {
		return "", fmt.Errorf("finalize: rendering finalize prompt json: %w", err)
	}
	return user, nil
}

func finalizeOutputSchemaSnippetFor(useJSONSchema bool) string {
	if useJSONSchema {
		return ""
	}
	return llm.FinalizeExamplePromptSnippet()
}
