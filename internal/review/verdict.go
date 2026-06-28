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
	SkipSuggestions          bool
	RepoRoot                 string
	ContextNotes             string
	DiffFormat               model.DiffFormat
	// PriorityThreshold is the configured "lowest currently allowed priority"
	// (p0..p3). It anchors the priority floor so a verifier-refuted non-finding is
	// classified at the threshold (not blocking) and never forces the overall
	// verdict to "patch is incorrect". Empty defaults to p3.
	PriorityThreshold string
	// ConfidenceThreshold removes low-confidence findings before verdict
	// reasoning. It uses finalized/display confidence; <=0 disables the filter.
	ConfidenceThreshold float64
}

func (e *Engine) Verdict(ctx context.Context, reviewCtx *model.ReviewContext, in *model.ReviewResult, opts VerdictOptions) (*model.ReviewResult, model.AgentRun, error) {
	if reviewCtx == nil {
		return nil, model.AgentRun{}, fmt.Errorf("verdict: nil review context")
	}
	if in == nil {
		return nil, model.AgentRun{}, fmt.Errorf("verdict: nil review result")
	}
	filtered, priorityDropped, err := filterResultByDisplayPriority(in, opts.PriorityThreshold)
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	if priorityDropped > 0 {
		e.logProgress(logging.StageVerdict, logging.StateWarn, fmt.Sprintf("priority filter dropped=%d kept=%d threshold=%s", priorityDropped, len(filtered.Findings), priorityThresholdLabel(opts.PriorityThreshold)))
		e.logf(ctx, "Verdict priority filter: dropped=%d kept=%d threshold=%s", priorityDropped, len(filtered.Findings), priorityThresholdLabel(opts.PriorityThreshold))
	}
	in = filtered
	filtered, drops, err := filterByConfidenceThreshold(in, opts.ConfidenceThreshold)
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	dropped := len(drops)
	if dropped > 0 {
		e.logProgress(logging.StageVerdict, logging.StateWarn, fmt.Sprintf("confidence filter dropped=%d kept=%d threshold=%.2f", dropped, len(filtered.Findings), opts.ConfidenceThreshold))
		e.logf(ctx, "Verdict confidence filter: dropped=%d kept=%d threshold=%.2f", dropped, len(filtered.Findings), opts.ConfidenceThreshold)
		for _, drop := range drops {
			e.logf(ctx, "Verdict confidence filter dropped finding: id=%s confidence=%.2f source=%s threshold=%.2f title=%q", drop.ID, drop.Confidence, drop.Source, opts.ConfidenceThreshold, drop.Title)
		}
	}
	in = filtered
	if opts.SkipSuggestions {
		stripped, err := in.Clone()
		if err != nil {
			return nil, model.AgentRun{}, fmt.Errorf("verdict: cloning input result: %w", err)
		}
		model.StripSuggestions(stripped.Findings)
		in = stripped
	}
	thresholdRank := model.PriorityThresholdRank(opts.PriorityThreshold)
	if len(in.Findings) == 0 {
		out, err := in.Clone()
		if err != nil {
			return nil, model.AgentRun{}, fmt.Errorf("verdict: cloning input result: %w", err)
		}
		out.OverallCorrectness = "patch is correct"
		out.OverallConfidenceScore = overallConfidenceFor("patch is correct", nil, thresholdRank)
		// Reset any pre-filter explanation rather than preserving it: with no
		// findings the verdict owns a fresh rationale, and a stale merge summary or
		// old "incorrect" explanation would contradict the empty, correct result
		// (e.g. --priority-threshold dropping every finding in the fused pipeline).
		if priorityDropped > 0 && dropped > 0 {
			out.OverallExplanation = "No findings remained after priority and confidence filtering."
		} else if priorityDropped > 0 {
			out.OverallExplanation = "No findings remained after priority filtering."
		} else if dropped > 0 {
			out.OverallExplanation = "No findings remained after confidence filtering."
		} else {
			out.OverallExplanation = "No finalized findings remained."
		}
		return out, model.AgentRun{Name: "Verdict Review", Role: "verdict", Status: model.AgentRunStatusSkipped}, nil
	}

	systemTemplate, err := e.loadPrompt("agent_verdict_system_prompt.tmpl")
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("verdict", verdictOutputSchemaSnippetFor(opts.UseJSONSchema), opts.SkipSuggestions)
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	styleGuides, err := e.styleGuidesFor(reviewCtx)
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	styleGuideToolchainSnippet, err := e.renderStyleGuideToolchainSnippet("verdict", styleGuides, len(reviewCtx.ToolchainVersions) > 0)
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet        string
		OutputFormatSnippet        string
		DisablePatchSummary        bool
		SkipSuggestions            bool
		StyleGuideToolchainSnippet string
	}{
		OutputSchemaSnippet:        verdictOutputSchemaSnippetFor(opts.UseJSONSchema),
		OutputFormatSnippet:        commonSnippets.outputFormat,
		DisablePatchSummary:        opts.DisablePatchSummary,
		SkipSuggestions:            opts.SkipSuggestions,
		StyleGuideToolchainSnippet: strings.TrimSpace(styleGuideToolchainSnippet),
	})
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("verdict: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildVerdictUserPrompt(reviewCtx, in, opts.ContextNotes, thresholdRank, opts.SkipSuggestions, opts.DiffFormat)
	if err != nil {
		return nil, model.AgentRun{}, err
	}

	var schema []byte
	constraints := verdictConstraintsFor(in.Findings, thresholdRank)
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
		SkipSuggestions:          opts.SkipSuggestions,
		UseJSONSchema:            opts.UseJSONSchema,
		DiffFormat:               opts.DiffFormat,
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
		return in, result.run, err
	}
	if result.resp == nil {
		return in, result.run, fmt.Errorf("verdict: agent returned nil response")
	}

	out, err := in.Clone()
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("verdict: cloning input result: %w", err)
	}
	out.OverallCorrectness = result.resp.OverallCorrectness
	out.OverallExplanation = result.resp.OverallExplanation
	// overall_confidence_score is computed deterministically in code (not emitted
	// by the LLM), anchored to the deciding findings' confidence.
	out.OverallConfidenceScore = overallConfidenceFor(out.OverallCorrectness, out.Findings, thresholdRank)
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
			MissingFields:   []string{"overall_correctness", "overall_explanation"},
			ReasoningEffort: reasoningEffort,
		}
	}
}

func filterResultByDisplayPriority(in *model.ReviewResult, threshold string) (*model.ReviewResult, int, error) {
	if in == nil {
		return nil, 0, fmt.Errorf("priority filter: nil review result")
	}
	filtered := filterByDisplayPriority(in.Findings, threshold)
	if len(filtered) == len(in.Findings) {
		return in, 0, nil
	}
	out, err := in.Clone()
	if err != nil {
		return nil, 0, fmt.Errorf("priority filter: cloning input result: %w", err)
	}
	out.Findings = filterByDisplayPriority(out.Findings, threshold)
	return out, len(in.Findings) - len(filtered), nil
}

func priorityThresholdLabel(threshold string) string {
	if threshold == "" {
		return "p3"
	}
	return threshold
}

type confidenceFilterDrop struct {
	ID         string
	Title      string
	Confidence float64
	Source     string
}

func filterByConfidenceThreshold(in *model.ReviewResult, threshold float64) (*model.ReviewResult, []confidenceFilterDrop, error) {
	if in == nil {
		return nil, nil, fmt.Errorf("verdict: nil review result")
	}
	if threshold <= 0 {
		return in, nil, nil
	}
	out, err := in.Clone()
	if err != nil {
		return nil, nil, fmt.Errorf("verdict: cloning input result: %w", err)
	}
	filtered := out.Findings[:0]
	drops := make([]confidenceFilterDrop, 0)
	for _, finding := range out.Findings {
		confidence, source := verdictFilterConfidence(finding)
		if confidence >= threshold {
			filtered = append(filtered, finding)
			continue
		}
		drops = append(drops, confidenceFilterDrop{
			ID:         finding.ID,
			Title:      finding.Title,
			Confidence: confidence,
			Source:     source,
		})
	}
	out.Findings = filtered
	return out, drops, nil
}

func verdictFilterConfidence(finding model.Finding) (float64, string) {
	if finding.Finalization != nil {
		return finding.Finalization.ConfidenceScore, "finalization"
	}
	if finding.Summarization != nil {
		return finding.Summarization.ConfidenceScore, "summarization"
	}
	return finding.ConfidenceScore, "review"
}

func (e *Engine) buildVerdictUserPrompt(reviewCtx *model.ReviewContext, in *model.ReviewResult, contextNotes string, thresholdRank int, skipSuggestions bool, format model.DiffFormat) (string, error) {
	payload := model.PromptPayloadFromContextWithDiffFormat(reviewCtx, format)
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
			"priority_floor":          priorityFloor(finding, thresholdRank),
			"code_location":           finding.CodeLocation,
			"review_confidence_score": finding.ConfidenceScore,
		}
		if finding.Verification != nil {
			verification := *finding.Verification
			model.EnsureVerificationID(&verification, finding.ID)
			entry["verification"] = &verification
		}
		if finding.Finalization != nil {
			finalization := *finding.Finalization
			if skipSuggestions {
				finalization.Suggestions = nil
			}
			entry["finalization"] = &finalization
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

// verdictConstraintsFor returns the correctness constraints implied by the
// verified finding priority floor (see priorityFloor). P0 blocks the patch, no
// P0/P1 cannot block it, and P1 remains prompt-judged because justification
// quality cannot be expressed in JSON schema. A verifier-refuted non-finding is
// demoted to the threshold floor, so it cannot force a blocking verdict.
func verdictConstraintsFor(in []model.Finding, thresholdRank int) llm.ResponseConstraints {
	hasP0, hasP1 := false, false
	for _, f := range in {
		switch priorityFloor(f, thresholdRank) {
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

// applyVerdictFallback fixes up a result's overall fields when the verdict agent
// did not produce them (failure or skip fallback). It first coerces overall
// correctness to the priority-derived constraint: the merge-derived value can
// disagree with the constraint — "patch is incorrect" for findings whose floor is
// only P2/P3, or the reverse for a P0 — so a transient verdict failure must not
// emit a blocking verdict for non-blocking findings (or a non-blocking one when a
// P0 is present). When the constraint is open (a P1 with no P0) the merge-derived
// correctness is kept. It then recomputes overall confidence from the (possibly
// coerced) correctness so the fallback matches the success path rather than
// carrying the merge-derived maxOverallConfidence.
func applyVerdictFallback(result *model.ReviewResult, thresholdRank int) {
	if result == nil {
		return
	}
	before := result.OverallCorrectness
	if allowed := verdictConstraintsFor(result.Findings, thresholdRank).AllowedCorrectness; len(allowed) == 1 {
		result.OverallCorrectness = allowed[0]
	} else if strings.TrimSpace(result.OverallCorrectness) == "" {
		// Open constraint (a P1 with no P0) and no preliminary correctness to keep
		// — e.g. verdict run directly on a bare findings array. Default
		// conservatively to "patch is incorrect" (the prompt's P1 default) so the
		// soft-failure path never emits an empty verdict.
		result.OverallCorrectness = "patch is incorrect"
	}
	// Replace the explanation whenever the fallback changed the verdict — a kept
	// stale explanation would state the opposite of the coerced correctness — or
	// when none was provided.
	if result.OverallCorrectness != before || strings.TrimSpace(result.OverallExplanation) == "" {
		result.OverallExplanation = "Verdict agent unavailable; overall correctness inferred from finding priorities."
	}
	result.OverallConfidenceScore = overallConfidenceFor(result.OverallCorrectness, result.Findings, thresholdRank)
}

// overallConfidenceFor computes the top-line verdict confidence deterministically
// from the finalized findings, anchored to the priority floor that drove the
// correctness decision. Like the per-finding finalization.confidence_score, this
// is computed in code rather than emitted by the LLM.
//
//   - "patch is incorrect": max confidence over the deciding findings — those at
//     floor 0, or (if none) floor 1. Defensive fallback to all findings.
//   - "patch is correct":   1 - 0.5*max(confidence over all findings); no findings
//     => 1.0.
func overallConfidenceFor(correctness string, findings []model.Finding, thresholdRank int) float64 {
	if correctness == "patch is incorrect" {
		deciding := findingsAtFloor(findings, 0, thresholdRank)
		if len(deciding) == 0 {
			deciding = findingsAtFloor(findings, 1, thresholdRank)
		}
		if len(deciding) == 0 {
			deciding = findings
		}
		return roundConfidenceScore(maxFindingConfidence(deciding))
	}
	// "patch is correct": temper by the strongest finding the verdict chose NOT to
	// treat as blocking. A floor-0 finding cannot occur here (the constraint forces
	// "incorrect"), but a floor-1 (P1) can — the prompt allows a justified "correct"
	// verdict over a P1 — so it must temper too, not be filtered out. No findings
	// => max is 0 => 1.0.
	return roundConfidenceScore(1 - 0.5*maxFindingConfidence(findings))
}

func findingsAtFloor(findings []model.Finding, floor, thresholdRank int) []model.Finding {
	var out []model.Finding
	for _, f := range findings {
		if priorityFloor(f, thresholdRank) == floor {
			out = append(out, f)
		}
	}
	return out
}

func maxFindingConfidence(findings []model.Finding) float64 {
	max := 0.0
	for _, f := range findings {
		if c := findingConfidence(f); c > max {
			max = c
		}
	}
	return max
}

// findingConfidence reads a finding's authoritative confidence: the code-computed
// finalization score, falling back to the verifier's, then the reviewer's.
func findingConfidence(f model.Finding) float64 {
	if f.Finalization != nil {
		return f.Finalization.ConfidenceScore
	}
	if f.Verification != nil {
		return f.Verification.ConfidenceScore
	}
	return f.ConfidenceScore
}

func verdictOutputSchemaSnippetFor(useJSONSchema bool) string {
	if useJSONSchema {
		return ""
	}
	return llm.VerdictExamplePromptSnippet()
}
