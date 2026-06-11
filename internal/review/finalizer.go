package review

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
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
	DisablePatchSummary      bool
	RepoRoot                 string
	// ContextNotes is the context agent's markdown summary of the patch's
	// intended purpose. When set, it is sent to the finalizer as `notes` so it
	// can be merged into overall_explanation.
	ContextNotes string
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
		return out, model.AgentRun{Name: "Finalize Review", Role: "finalize"}, nil
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
		DisablePatchSummary bool
	}{
		PrioritySnippet:     commonSnippets.priority,
		OutputSchemaSnippet: finalizeOutputSchemaSnippetFor(opts.UseJSONSchema),
		OutputFormatSnippet: commonSnippets.outputFormat,
		DisablePatchSummary: opts.DisablePatchSummary,
	})
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("finalize: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildFinalizeUserPrompt(reviewCtx, in, opts.ContextNotes)
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
	finalizeStart := time.Now()
	e.logProgress(logging.StageFinalize, logging.StateStart, fmt.Sprintf("findings=%d", len(in.Findings)))
	result, err := e.runAgent(ctx, agentSpec{
		name:             "Finalize Review",
		role:             "finalize",
		system:           system,
		noToolsSystem:    system,
		user:             userPrompt,
		schema:           schema,
		schemaKind:       llm.SchemaKindFinalize,
		constraints:      constraints,
		hasTools:         false,
		validateResponse: finalizerOutputValidator(in.Findings),
	}, req)
	if err != nil {
		// Preserve partial AgentRun (tokens, tool calls accumulated before
		// the loop aborted) for telemetry parity with merge/context failures.
		return nil, result.run, err
	}
	if result.resp == nil {
		return nil, result.run, fmt.Errorf("finalize: agent returned nil response")
	}

	out, err := in.Clone()
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("finalize: cloning input result: %w", err)
	}
	stats := applyFinalizerOutput(out.Findings, result.resp.Findings)
	out.OverallCorrectness = result.resp.OverallCorrectness
	out.OverallExplanation = result.resp.OverallExplanation
	out.OverallConfidenceScore = result.resp.OverallConfidenceScore
	mergeInputSuggestions(out.Findings, in.Findings)
	// Last-resort repair after retry exhaustion or direct non-parser callers.
	// Normal schema/parser paths require `verification` in the finalizer output.
	mergeInputVerification(out.Findings, in.Findings)
	enforcePriorityFloor(out.Findings, in.Findings)
	applyWeightedConfidence(out.Findings, in.Findings)
	if stats.Omitted > 0 || stats.Ignored > 0 || stats.FinalizerFindings != len(in.Findings) {
		out.Warnings = append(out.Warnings, fmt.Sprintf("Finalizer output mismatch: findings_in=%d finalizer_findings=%d matched=%d omitted=%d ignored=%d; preserved input findings", len(in.Findings), stats.FinalizerFindings, stats.Matched, stats.Omitted, stats.Ignored))
	}
	e.logProgress(logging.StageFinalize, logging.StateDone, fmt.Sprintf("findings_in=%d finalizer_findings=%d matched=%d omitted=%d ignored=%d findings_out=%d prompt_tokens=%s completion_tokens=%s total_tokens=%s runtime=%s", len(in.Findings), stats.FinalizerFindings, stats.Matched, stats.Omitted, stats.Ignored, len(out.Findings), model.HumanTokens(result.run.TokensUsed.PromptTokens), model.HumanTokens(result.run.TokensUsed.CompletionTokens), model.HumanTokens(result.run.TokensUsed.TotalTokens), model.HumanDuration(time.Since(finalizeStart))))
	return out, result.run, nil
}

func finalizerOutputValidator(inputFindings []model.Finding) func(*llm.ReviewResponse) *llm.InvalidResponseError {
	return func(resp *llm.ReviewResponse) *llm.InvalidResponseError {
		expected := len(inputFindings)
		var finalizerFindings []model.Finding
		if resp != nil {
			finalizerFindings = resp.Findings
		}
		stats := finalizerOutputStats(inputFindings, finalizerFindings, nil)
		if stats.FinalizerFindings == expected && stats.Matched == expected && stats.Omitted == 0 && stats.Ignored == 0 {
			return nil
		}
		raw := ""
		reasoningEffort := ""
		if resp != nil {
			raw = resp.RawResponse
			reasoningEffort = resp.ReasoningEffort
		}
		invalid := &llm.InvalidResponseError{
			RawContent:      raw,
			Reason:          fmt.Sprintf("finalizer_output_mismatch got=%d expected=%d matched=%d omitted=%d ignored=%d", stats.FinalizerFindings, expected, stats.Matched, stats.Omitted, stats.Ignored),
			MissingFields:   []string{"findings"},
			ReasoningEffort: reasoningEffort,
		}
		if stats.FinalizerFindings != expected || stats.Matched != expected || stats.Omitted != 0 || stats.Ignored != 0 {
			invalid.RetryGuidanceTemplate = "finalizer_count_retry_guidance.tmpl"
			invalid.RetryGuidanceData = struct {
				Expected int
				Got      int
				Matched  int
				Omitted  int
				Ignored  int
			}{
				Expected: expected,
				Got:      stats.FinalizerFindings,
				Matched:  stats.Matched,
				Omitted:  stats.Omitted,
				Ignored:  stats.Ignored,
			}
		}
		return invalid
	}
}

func (e *Engine) buildFinalizeUserPrompt(reviewCtx *model.ReviewContext, in *model.ReviewResult, contextNotes string) (string, error) {
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

	payloadMap := map[string]any{
		"review_context":           json.RawMessage(contextJSON),
		"overall_correctness":      in.OverallCorrectness,
		"overall_explanation":      in.OverallExplanation,
		"overall_confidence_score": in.OverallConfidenceScore,
		"findings":                 findings,
	}
	// Only send notes when present so finalize-from-file / no-context runs do
	// not include an empty field for the model to merge.
	if strings.TrimSpace(contextNotes) != "" {
		payloadMap["notes"] = contextNotes
	}
	user, err := llm.RenderJSON(payloadMap)
	if err != nil {
		return "", fmt.Errorf("finalize: rendering finalize prompt json: %w", err)
	}
	return user, nil
}

type finalizerApplyStats struct {
	FinalizerFindings int
	Matched           int
	Omitted           int
	Ignored           int
}

// applyFinalizerOutput copies finalization-only data from finalizer output
// onto input-owned findings. The finalizer is not allowed to add, remove,
// reorder, or relocate findings; unmatched output is ignored, and omitted
// input findings receive conservative synthesized finalization.
func applyFinalizerOutput(inOut, finalizer []model.Finding) finalizerApplyStats {
	return finalizerOutputStats(inOut, finalizer, func(inIdx, outIdx int) {
		applyFinalizedFinding(&inOut[inIdx], finalizer[outIdx])
	})
}

func finalizerOutputStats(inOut, finalizer []model.Finding, onMatch func(inIdx, outIdx int)) finalizerApplyStats {
	stats := finalizerApplyStats{FinalizerFindings: len(finalizer)}
	matchedInput := make([]bool, len(inOut))
	usedOutput := make([]bool, len(finalizer))
	for outIdx := range finalizer {
		inIdx := findFinalizerInputIndex(finalizer[outIdx], inOut, matchedInput)
		if inIdx < 0 {
			continue
		}
		usedOutput[outIdx] = true
		matchedInput[inIdx] = true
		stats.Matched++
		if onMatch != nil {
			onMatch(inIdx, outIdx)
		}
	}
	for i := range finalizer {
		if !usedOutput[i] {
			stats.Ignored++
		}
	}
	for i := range inOut {
		if matchedInput[i] {
			continue
		}
		stats.Omitted++
		if onMatch != nil && inOut[i].Finalization == nil {
			inOut[i].Finalization = synthesizedFinalization(inOut[i])
		}
	}
	return stats
}

func findFinalizerInputIndex(target model.Finding, in []model.Finding, matched []bool) int {
	id := strings.TrimSpace(target.ID)
	if id != "" {
		for i := range in {
			if matched[i] {
				continue
			}
			if in[i].ID == id {
				return i
			}
		}
	}
	var locMatches []int
	for i := range in {
		if matched[i] {
			continue
		}
		if in[i].CodeLocation == target.CodeLocation {
			locMatches = append(locMatches, i)
		}
	}
	if len(locMatches) == 1 {
		return locMatches[0]
	}
	for _, i := range locMatches {
		if in[i].Title == target.Title {
			return i
		}
	}
	return -1
}

func applyFinalizedFinding(dst *model.Finding, src model.Finding) {
	if src.Finalization != nil {
		finalization := *src.Finalization
		dst.Finalization = &finalization
	}
	if len(src.Suggestions) > 0 {
		dst.Suggestions = append([]model.Suggestion(nil), src.Suggestions...)
	}
	if src.Verification != nil {
		verification := *src.Verification
		dst.Verification = &verification
	}
}

func synthesizedFinalization(finding model.Finding) *model.FindingFinalization {
	priority := model.PriorityRank(finding.Priority)
	// Verifier-driven escalation is intentional; it mirrors the finalizer floor.
	if finding.Verification != nil && finding.Verification.Priority < priority {
		priority = finding.Verification.Priority
	}
	return &model.FindingFinalization{
		Title:    finding.Title,
		Body:     finding.Body,
		Priority: priority,
		Remarks:  "Finalizer omitted or misidentified this finding; preserved original finding.",
	}
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

// mergeInputVerification is a last-resort repair for responses that survive
// retry exhaustion or direct non-parser tests despite dropping `verification`.
// Normal merge/finalize schema and parser paths require `verification`, so this
// should not run on valid model output. LLM-emitted verification wins when present.
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
// confidence, rounded to two decimals. Moved out of the LLM prompt because LLMs
// are unreliable at arithmetic. If verification is missing, the review
// confidence is used as-is. Hallucinated findings with no input match are
// skipped (no value applied).
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
			out[i].Finalization.ConfidenceScore = roundConfidenceScore(review)
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
		out[i].Finalization.ConfidenceScore = roundConfidenceScore(score)
	}
}

func roundConfidenceScore(score float64) float64 {
	return math.Round(score*100) / 100
}

// finalizeConstraintsFor returns the constraints to apply to the finalizer
// based on the verified findings. Three regimes, computed from the priority
// floor = min(finding.priority, verification.priority):
//   - any P0 floor: AllowedCorrectness = ["patch is incorrect"]. The schema
//     forces the finalizer's hand because a critical finding by definition
//     blocks the patch.
//   - any P1 floor, no P0: unconstrained. The prompt asks the finalizer to
//     default to "patch is incorrect" but lets it claim "patch is correct"
//     with strong justification — schema cannot judge justification quality.
//   - no P0/P1 floor: AllowedCorrectness = ["patch is correct"]. Without a
//     critical finding the finalizer cannot fabricate a blocker.
func finalizeConstraintsFor(in []model.Finding) llm.ResponseConstraints {
	hasP0, hasP1 := false, false
	for _, f := range in {
		floor := model.PriorityRank(f.Priority)
		if f.Verification != nil && f.Verification.Priority < floor {
			floor = f.Verification.Priority
		}
		switch floor {
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

func finalizeOutputSchemaSnippetFor(useJSONSchema bool) string {
	if useJSONSchema {
		return ""
	}
	return llm.FinalizeExamplePromptSnippet()
}
