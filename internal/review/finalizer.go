package review

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

const (
	finalizeVerificationWeight = 0.6
	finalizeReviewWeight       = 0.4
	finalizeDivergenceClamp    = 0.3
)

type FinalizeOptions struct {
	UseJSONSchema            bool
	MaxOutputRetries         int
	MaxReasoningSeconds      int
	MaxReasoningLoopRepeats  int
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
	if len(in.Findings) == 0 {
		out, err := in.Clone()
		if err != nil {
			return nil, model.AgentRun{}, fmt.Errorf("finalize: cloning input result: %w", err)
		}
		return out, model.AgentRun{Name: "finalize", Role: "finalize"}, nil
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
		MaxReasoningLoopRepeats:  opts.MaxReasoningLoopRepeats,
		DisableParallelToolCalls: opts.DisableParallelToolCalls,
		UseJSONSchema:            opts.UseJSONSchema,
	}
	constraints := finalizeConstraintsFor(in.Findings)
	if opts.UseJSONSchema && len(constraints.AllowedCorrectness) > 0 {
		schema = llm.FinalizeSchemaWithConstraints(constraints)
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
		constraints:   constraints,
		hasTools:      false,
	}, req)
	if err != nil {
		return nil, model.AgentRun{}, err
	}

	out, err := in.Clone()
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("finalize: cloning input result: %w", err)
	}
	out.Findings = dropUnmatchedFindings(result.resp.Findings, in.Findings)
	out.OverallCorrectness = result.resp.OverallCorrectness
	out.OverallExplanation = result.resp.OverallExplanation
	out.OverallConfidenceScore = result.resp.OverallConfidenceScore
	mergeInputSuggestions(out.Findings, in.Findings)
	mergeInputVerification(out.Findings, in.Findings)
	enforcePriorityFloor(out.Findings, in.Findings)
	applyWeightedConfidence(out.Findings, in.Findings)
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
			"id":                      finding.ID,
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
			verification := *finding.Verification
			model.EnsureVerificationID(&verification, finding.ID)
			entry["verification"] = &verification
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

// dropUnmatchedFindings removes finalizer-output findings whose code_location
// or ID has no input match. The finalize prompt forbids new findings; this is the
// in-code defence so hallucinated entries cannot bypass priority floor and
// weighted-confidence (which both skip on no-match) and ship with arbitrary
// LLM values.
func dropUnmatchedFindings(out, in []model.Finding) []model.Finding {
	kept := make([]model.Finding, 0, len(out))
	for i := range out {
		match := findInputMatch(out[i], in)
		if match == nil {
			continue
		}
		out[i].ID = match.ID
		kept = append(kept, out[i])
	}
	return kept
}

// mergeInputSuggestions defends against the finalizer LLM dropping `suggestions`
// by restoring them from the matching input finding when the output finding has
// none. Matching is by code_location, with finding title as a tiebreaker when
// multiple input findings share the same location.
func mergeInputSuggestions(out, in []model.Finding) {
	for i := range out {
		if len(out[i].Suggestions) > 0 {
			continue
		}
		src := findInputMatch(out[i], in)
		if src == nil || len(src.Suggestions) == 0 {
			continue
		}
		out[i].Suggestions = append([]model.Suggestion(nil), src.Suggestions...)
	}
}

// mergeInputVerification restores `verification` from the matching input
// finding when the finalizer LLM does not echo it. The finalize prompt does
// not instruct the LLM to repeat verification, so the schema does not require
// it; downstream consumers still want the verifier verdict on each finding.
// LLM-emitted verification wins when present.
func mergeInputVerification(out, in []model.Finding) {
	for i := range out {
		if out[i].Verification != nil {
			continue
		}
		src := findInputMatch(out[i], in)
		if src == nil || src.Verification == nil {
			continue
		}
		v := *src.Verification
		model.EnsureVerificationID(&v, out[i].ID)
		out[i].Verification = &v
	}
}

func findInputMatch(target model.Finding, in []model.Finding) *model.Finding {
	if target.ID != "" {
		for i := range in {
			// ID matches still need a location cross-check so swapped IDs cannot
			// attach one finding's ID to another finding's code location.
			if in[i].ID == target.ID && in[i].CodeLocation == target.CodeLocation {
				return &in[i]
			}
		}
	}
	var locMatches []*model.Finding
	for i := range in {
		if in[i].CodeLocation == target.CodeLocation {
			locMatches = append(locMatches, &in[i])
		}
	}
	switch len(locMatches) {
	case 0:
		return nil
	case 1:
		return locMatches[0]
	}
	for _, m := range locMatches {
		if m.Title == target.Title {
			return m
		}
	}
	return locMatches[0]
}

// enforcePriorityFloor ensures finalization.priority is not more critical (lower number)
// than the most critical value among the original finding and its verifier. "Floor" refers
// to the integer value: lower numbers = more critical, so the floor is the minimum integer
// the finalizer is allowed to emit. Matching is by code_location, with finding title as a
// tiebreaker when multiple input findings share the same location, so reordering or dropping
// by the LLM does not misalign the floor.
func enforcePriorityFloor(out, in []model.Finding) {
	for i := range out {
		if out[i].Finalization == nil {
			continue
		}
		orig := findInputMatch(out[i], in)
		if orig == nil {
			continue
		}
		floor := model.PriorityRank(orig.Priority)
		if orig.Verification != nil && orig.Verification.Priority < floor {
			floor = orig.Verification.Priority
		}
		if out[i].Finalization.Priority < floor {
			out[i].Finalization.Priority = floor
		}
	}
}

// applyWeightedConfidence overwrites finalization.confidence_score with a
// deterministic weighted average of the review confidence and the verifier's
// confidence. Moved out of the LLM prompt because LLMs are unreliable at
// arithmetic. If verification is missing, the review confidence is used as-is.
// Hallucinated findings with no input match are skipped (no value applied).
func applyWeightedConfidence(out, in []model.Finding) {
	for i := range out {
		if out[i].Finalization == nil {
			continue
		}
		orig := findInputMatch(out[i], in)
		if orig == nil {
			continue
		}
		review := orig.ConfidenceScore
		if orig.Verification == nil {
			out[i].Finalization.ConfidenceScore = review
			continue
		}
		verify := orig.Verification.ConfidenceScore
		score := finalizeVerificationWeight*verify + finalizeReviewWeight*review
		if math.Abs(verify-review) > finalizeDivergenceClamp {
			lower := math.Min(verify, review)
			if score > lower {
				score = lower
			}
		}
		out[i].Finalization.ConfidenceScore = score
	}
}

// finalizeConstraintsFor returns the constraints to apply to the finalizer
// based on the verified findings. If no input finding is P0 or P1, the
// finalizer cannot flip overall_correctness to "patch is incorrect" — that
// outcome must be driven by at least one critical finding.
func finalizeConstraintsFor(in []model.Finding) llm.ResponseConstraints {
	hasCritical := false
	for _, f := range in {
		if model.PriorityRank(f.Priority) < 2 {
			hasCritical = true
			break
		}
	}
	if hasCritical {
		return llm.ResponseConstraints{}
	}
	return llm.ResponseConstraints{AllowedCorrectness: []string{"patch is correct"}}
}

// DropInvalidFindings removes findings the verifier marked as invalid.
func DropInvalidFindings(findings []model.Finding) []model.Finding {
	out := make([]model.Finding, 0, len(findings))
	for _, f := range findings {
		if f.Verification != nil && !f.Verification.Valid {
			continue
		}
		out = append(out, f)
	}
	return out
}

func finalizeOutputSchemaSnippetFor(useJSONSchema bool) string {
	if useJSONSchema {
		return ""
	}
	return llm.FinalizeExamplePromptSnippet()
}
