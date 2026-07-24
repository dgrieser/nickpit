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
	DisableJSONResponseFormat bool
	MaxOutputRetries          int
	MaxReasoningSeconds       int
	DisableParallelToolCalls  bool
	DisablePatchSummary       bool
	DisableSuggestions        bool
	RepoRoot                  string
	DiffFormat                model.DiffFormat
	// PriorityThreshold is the configured "lowest currently allowed priority"
	// (p0..p3). It is the floor a missing priority defaults to and the level a
	// verifier-refuted non-finding is demoted to, so an affirmation never renders
	// blocking. Empty defaults to p3 via model.PriorityThresholdRank.
	PriorityThreshold string
	// ContextNotes is kept for option-shape compatibility with the post-merge
	// pipeline, but final correctness/explanation notes are handled by Verdict.
	ContextNotes string
	// ShardLabel, when set (e.g. "#2"), distinguishes a per-cluster finalize
	// shard's live-progress bar; it never affects the telemetry run name.
	ShardLabel string
}

func (e *Engine) Finalize(ctx context.Context, reviewCtx *model.ReviewContext, in *model.ReviewResult, opts FinalizeOptions) (*model.ReviewResult, model.AgentRun, error) {
	if reviewCtx == nil {
		return nil, model.AgentRun{}, fmt.Errorf("finalize: nil review context")
	}
	if in == nil {
		return nil, model.AgentRun{}, fmt.Errorf("finalize: nil review result")
	}
	prepared, err := in.Clone()
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("finalize: cloning input result: %w", err)
	}
	model.KeepFirstSuggestion(prepared.Findings)
	in = prepared
	if len(in.Findings) == 0 {
		return in, model.AgentRun{Name: "Finalize Review", Role: "finalize"}, nil
	}

	systemTemplate, err := e.loadPrompt("agent_finalize_system_prompt.tmpl")
	if err != nil {
		return in, model.AgentRun{}, err
	}
	outputSchemaSnippet := exampleSnippetFor(llm.SchemaKindFinalize, opts.DisableSuggestions)
	commonSnippets, err := agentCommonSystemPromptSnippets("finalize", outputSchemaSnippet, opts.DisableSuggestions)
	if err != nil {
		return in, model.AgentRun{}, err
	}
	styleGuides, err := e.styleGuidesFor(reviewCtx)
	if err != nil {
		return in, model.AgentRun{}, err
	}
	styleGuideToolchainSnippet, err := e.renderStyleGuideToolchainSnippet("finalize", styleGuides, len(reviewCtx.ToolchainVersions) > 0)
	if err != nil {
		return in, model.AgentRun{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		PrioritySnippet            string
		OutputSchemaSnippet        string
		OutputFormatSnippet        string
		DisablePatchSummary        bool
		DisableSuggestions         bool
		StyleGuideToolchainSnippet string
	}{
		PrioritySnippet:            commonSnippets.priority,
		OutputSchemaSnippet:        outputSchemaSnippet,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		DisablePatchSummary:        opts.DisablePatchSummary,
		DisableSuggestions:         opts.DisableSuggestions,
		StyleGuideToolchainSnippet: strings.TrimSpace(styleGuideToolchainSnippet),
	})
	if err != nil {
		return in, model.AgentRun{}, fmt.Errorf("finalize: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildFinalizeUserPrompt(reviewCtx, in, opts.ContextNotes, opts.DisableSuggestions, opts.DiffFormat)
	if err != nil {
		return in, model.AgentRun{}, err
	}

	var schema []byte
	if !opts.DisableJSONResponseFormat {
		schema = llm.FinalizeSchema
		if opts.DisableSuggestions {
			schema = llm.FinalizeSchemaWithoutSuggestions
		}
	}

	req := model.ReviewRequest{
		RepoRoot:                  opts.RepoRoot,
		MaxOutputRetries:          opts.MaxOutputRetries,
		MaxReasoningSeconds:       opts.MaxReasoningSeconds,
		DisableParallelToolCalls:  opts.DisableParallelToolCalls,
		DisableSuggestions:        opts.DisableSuggestions,
		DisableJSONResponseFormat: opts.DisableJSONResponseFormat,
		DiffFormat:                opts.DiffFormat,
	}
	finalizeStart := time.Now()
	e.logProgress(logging.StageFinalize, logging.StateStart, fmt.Sprintf("findings=%d", len(in.Findings)))
	result, err := e.runAgent(ctx, agentSpec{
		name:             "Finalize Review",
		progressName:     shardProgressName("Finalize", opts.ShardLabel),
		role:             "finalize",
		system:           system,
		noToolsSystem:    system,
		user:             userPrompt,
		schema:           schema,
		schemaKind:       llm.SchemaKindFinalize,
		hasTools:         false,
		validateResponse: finalizerOutputValidator(in.Findings),
	}, req)
	if err != nil {
		// Preserve partial AgentRun (tokens, tool calls accumulated before
		// the loop aborted) for telemetry parity with merge/context failures.
		return in, result.run, err
	}
	if result.resp == nil {
		return in, result.run, fmt.Errorf("finalize: agent returned nil response")
	}

	out, err := in.Clone()
	if err != nil {
		return in, model.AgentRun{}, fmt.Errorf("finalize: cloning input result: %w", err)
	}
	stats := applyFinalizerOutput(out.Findings, result.resp.Findings)
	if opts.DisableSuggestions {
		model.StripSuggestions(out.Findings)
	} else {
		normalizeFinalizedSuggestions(out.Findings, in.Findings)
	}
	// Last-resort repair after retry exhaustion or direct non-parser callers.
	// Normal schema/parser paths require `verification` in the finalizer output.
	mergeInputVerification(out.Findings, in.Findings)
	enforcePriorityFloor(out.Findings, in.Findings, model.PriorityThresholdRank(opts.PriorityThreshold))
	applyWeightedConfidence(out.Findings, in.Findings)
	preserveUnverifiedReviewFinalizations(out.Findings, in.Findings)
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
		ids := finalizerOutputIDDiagnostics(inputFindings, finalizerFindings)
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
				Expected   int
				Got        int
				Matched    int
				Omitted    int
				Ignored    int
				AllowedIDs []string
				OmittedIDs []string
				IgnoredIDs []string
			}{
				Expected:   expected,
				Got:        stats.FinalizerFindings,
				Matched:    stats.Matched,
				Omitted:    stats.Omitted,
				Ignored:    stats.Ignored,
				AllowedIDs: ids.AllowedIDs,
				OmittedIDs: ids.OmittedIDs,
				IgnoredIDs: ids.IgnoredIDs,
			}
		}
		return invalid
	}
}

func (e *Engine) buildFinalizeUserPrompt(reviewCtx *model.ReviewContext, in *model.ReviewResult, contextNotes string, disableSuggestions bool, format model.DiffFormat) (string, error) {
	payload := model.PromptPayloadFromContextWithDiffFormat(reviewCtx, format)
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
		if !disableSuggestions && len(finding.Suggestions) > 0 {
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
		"review_context": json.RawMessage(contextJSON),
		"findings":       findings,
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

type finalizerIDDiagnostics struct {
	AllowedIDs []string
	OmittedIDs []string
	IgnoredIDs []string
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

func finalizerOutputIDDiagnostics(inOut, finalizer []model.Finding) finalizerIDDiagnostics {
	ids := finalizerIDDiagnostics{AllowedIDs: make([]string, 0, len(inOut))}
	for _, finding := range inOut {
		if id := strings.TrimSpace(finding.ID); id != "" {
			ids.AllowedIDs = append(ids.AllowedIDs, id)
		}
	}
	matchedInput := make([]bool, len(inOut))
	for outIdx := range finalizer {
		inIdx := findFinalizerInputIndex(finalizer[outIdx], inOut, matchedInput)
		if inIdx < 0 {
			ids.IgnoredIDs = append(ids.IgnoredIDs, strings.TrimSpace(finalizer[outIdx].ID))
			continue
		}
		matchedInput[inIdx] = true
	}
	for i := range inOut {
		if matchedInput[i] {
			continue
		}
		if id := strings.TrimSpace(inOut[i].ID); id != "" {
			ids.OmittedIDs = append(ids.OmittedIDs, id)
		}
	}
	return ids
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
		if sameCodeLocationAnchor(in[i].CodeLocation, target.CodeLocation) {
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
		finalization.Suggestions = cloneSuggestions(src.Finalization.Suggestions)
		dst.Finalization = &finalization
	}
	if src.Verification != nil {
		verification := *src.Verification
		dst.Verification = &verification
	}
}

func synthesizedFinalization(finding model.Finding) *model.FindingFinalization {
	priority := model.PriorityRank(finding.Priority)
	// Verifier-driven escalation is intentional; it mirrors the finalizer floor.
	if finding.Verification != nil && !isUnverifiedVerification(finding.Verification) && finding.Verification.Priority < priority {
		priority = finding.Verification.Priority
	}
	return &model.FindingFinalization{
		Title:       finding.Title,
		Body:        finding.Body,
		Priority:    priority,
		Remarks:     "Finalizer omitted or misidentified this finding; preserved original finding.",
		Suggestions: cloneSuggestions(finding.Suggestions),
	}
}

// normalizeFinalizedSuggestions lets the finalizer polish the sole input
// suggestion while preserving its line range. Top-level suggestions stay as
// reviewer-provenance input. If the finalizer omits or adds suggestions, the
// input suggestion fills the gap and extras are discarded.
func normalizeFinalizedSuggestions(out, in []model.Finding) {
	for i := range out {
		src := findInputMatch(out[i], in)
		if src == nil {
			continue
		}
		keptSuggestionIndexes := distinctSuggestionIndexes(src.Suggestions)
		out[i].Suggestions = cloneSuggestionsAt(src.Suggestions, keptSuggestionIndexes)
		if len(keptSuggestionIndexes) == 0 {
			if out[i].Finalization != nil {
				out[i].Finalization.Suggestions = nil
			}
			continue
		}
		if out[i].Finalization == nil {
			out[i].Finalization = synthesizedFinalization(out[i])
		}
		candidates := out[i].Finalization.Suggestions
		normalized := make([]model.Suggestion, len(keptSuggestionIndexes))
		for outIdx, srcIdx := range keptSuggestionIndexes {
			normalized[outIdx] = src.Suggestions[srcIdx]
			candidateIdx := srcIdx
			if candidateIdx >= len(candidates) {
				candidateIdx = outIdx
			}
			if candidateIdx < len(candidates) && strings.TrimSpace(candidates[candidateIdx].Body) != "" {
				normalized[outIdx].Body = candidates[candidateIdx].Body
			}
		}
		out[i].Finalization.Suggestions = normalized
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
			if in[i].ID == target.ID && sameCodeLocationAnchor(in[i].CodeLocation, target.CodeLocation) {
				return &in[i]
			}
		}
	}
	var locMatches []*model.Finding
	for i := range in {
		if sameCodeLocationAnchor(in[i].CodeLocation, target.CodeLocation) {
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

func sameCodeLocationAnchor(a, b model.CodeLocation) bool {
	return a.FilePath == b.FilePath && a.LineRange.SameAnchor(b.LineRange)
}

// enforcePriorityFloor ensures finalization.priority is not more critical (lower number)
// than the finding's priority floor (see priorityFloor). "Floor" refers to the integer
// value: lower numbers = more critical, so the floor is the minimum integer the finalizer
// is allowed to emit. Matching is by code_location, with finding title as a tiebreaker when
// multiple input findings share the same location, so reordering or dropping by the LLM does
// not misalign the floor.
func enforcePriorityFloor(out, in []model.Finding, thresholdRank int) {
	for i := range out {
		if out[i].Finalization == nil {
			continue
		}
		orig := findInputMatch(out[i], in)
		if orig == nil {
			continue
		}
		floor := priorityFloor(*orig, thresholdRank)
		if out[i].Finalization.Priority < floor {
			out[i].Finalization.Priority = floor
		}
	}
}

// nonFindingRemark is the sentinel the verify prompt requires in a verification's
// remarks when a "finding" is actually a non-finding (an affirmation / "no issue"
// item) returned as `refuted`. It separates those from a genuine refutation, which
// instead cites the contradicting code.
const nonFindingRemark = "no issue"

// isNonFindingVerification reports whether a verification marks its finding as a
// non-finding: `refuted` with the "no issue" sentinel leading the remarks. Only
// these are demoted/zeroed. A real finding kept as `refuted` under
// --verify-drop-policy=none cites code instead, so it keeps its priority and
// confidence and can still force a blocking verdict. The sentinel must START the
// remark (the prompt demands exactly "no issue"; models still append variants
// like "no issue: code is sound") — a genuine refutation such as "the guard at
// foo.go:12 prevents this, so no issue arises" merely contains the phrase
// mid-sentence and must not be misclassified as a non-finding.
func isNonFindingVerification(v *model.FindingVerification) bool {
	return v != nil && v.Verdict == model.VerdictRefuted &&
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(v.Remarks)), nonFindingRemark)
}

func isUnverifiedVerification(v *model.FindingVerification) bool {
	return v != nil && v.Verdict == model.VerdictUnverified
}

// priorityFloor computes a finding's priority floor: the most-critical (lowest) integer it is
// allowed to take. A non-finding (a verifier-refuted affirmation carrying the "no issue"
// sentinel; see isNonFindingVerification) is demoted to thresholdRank — the lowest currently
// allowed priority — so it never renders blocking, ignoring the reviewer's (often template)
// priority. A missing priority also defaults to thresholdRank rather than the hardcoded floor
// of model.PriorityRank. Otherwise the floor is the most critical of the reviewer and verifier
// priorities, as before — so a genuine refutation kept for review keeps its severity. An
// unverified verifier result is non-authoritative and cannot change priority.
func priorityFloor(f model.Finding, thresholdRank int) int {
	if isNonFindingVerification(f.Verification) {
		return thresholdRank
	}
	floor := priorityRankOrThreshold(f.Priority, thresholdRank)
	if f.Verification != nil && !isUnverifiedVerification(f.Verification) && f.Verification.Priority < floor {
		floor = f.Verification.Priority
	}
	return floor
}

// priorityRankOrThreshold ranks a finding priority, defaulting a missing (nil) priority to
// thresholdRank (the lowest currently allowed priority) instead of model.PriorityRank's
// hardcoded 3.
func priorityRankOrThreshold(priority *int, thresholdRank int) int {
	if priority == nil {
		return thresholdRank
	}
	return model.PriorityRank(priority)
}

// applyWeightedConfidence overwrites finalization.confidence_score with a
// deterministic weighted average of the review confidence and the verifier's
// confidence, rounded to two decimals. Moved out of the LLM prompt because LLMs
// are unreliable at arithmetic. Missing confidence defaults to 0.0: a non-finding
// (a verifier-refuted "no issue" affirmation; see isNonFindingVerification) carries
// no real confidence, and a finding with no verification has no confidence signal to
// trust — neither is padded from the reviewer's self-assessment. A genuine refutation
// kept for review keeps its blended confidence. Hallucinated findings with no input
// match are skipped (no value applied).
func applyWeightedConfidence(out, in []model.Finding) {
	for i := range out {
		if out[i].Finalization == nil {
			continue
		}
		orig := findInputMatch(out[i], in)
		if orig == nil {
			continue
		}
		if orig.Verification == nil {
			out[i].Finalization.ConfidenceScore = 0.0
			continue
		}
		if isNonFindingVerification(orig.Verification) {
			out[i].Finalization.ConfidenceScore = 0.0
			continue
		}
		review := orig.ConfidenceScore
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

func preserveUnverifiedReviewFinalizations(out, in []model.Finding) {
	for i := range out {
		orig := findInputMatch(out[i], in)
		if orig == nil || !isUnverifiedVerification(orig.Verification) {
			continue
		}
		out[i].Finalization = reviewFinalization(*orig)
	}
}

func reviewFinalization(finding model.Finding) *model.FindingFinalization {
	return &model.FindingFinalization{
		Title:           finding.Title,
		Body:            finding.Body,
		Priority:        model.PriorityRank(finding.Priority),
		ConfidenceScore: finding.ConfidenceScore,
		Suggestions:     distinctSuggestions(finding.Suggestions),
	}
}

func cloneSuggestions(src []model.Suggestion) []model.Suggestion {
	if len(src) == 0 {
		return nil
	}
	out := make([]model.Suggestion, len(src))
	copy(out, src)
	return out
}

func distinctSuggestions(src []model.Suggestion) []model.Suggestion {
	return cloneSuggestionsAt(src, distinctSuggestionIndexes(src))
}

func cloneSuggestionsAt(src []model.Suggestion, indexes []int) []model.Suggestion {
	if len(indexes) == 0 {
		return nil
	}
	out := make([]model.Suggestion, 0, len(indexes))
	for _, idx := range indexes {
		if idx >= 0 && idx < len(src) {
			out = append(out, src[idx])
		}
	}
	return out
}

func distinctSuggestionIndexes(src []model.Suggestion) []int {
	if len(src) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(src))
	for i := range src {
		duplicate := false
		for _, keptIdx := range indexes {
			if duplicateSuggestion(src[i], src[keptIdx]) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func duplicateSuggestion(a, b model.Suggestion) bool {
	aBody := normalizeSuggestionBody(a.Body)
	bBody := normalizeSuggestionBody(b.Body)
	if aBody == "" || bBody == "" {
		return false
	}
	if !sameSuggestionAnchor(a, b) && !unanchoredSuggestion(a, b) {
		return false
	}
	if aBody == bBody {
		return true
	}
	return sameSuggestionAnchor(a, b) && suggestionTokenSimilarity(a.Body, b.Body) >= 0.75
}

func normalizeSuggestionBody(body string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(body))), " ")
}

func sameSuggestionAnchor(a, b model.Suggestion) bool {
	aLoc := suggestionLocation(a)
	bLoc := suggestionLocation(b)
	aPath := strings.TrimSpace(aLoc.FilePath)
	bPath := strings.TrimSpace(bLoc.FilePath)
	if aPath != "" || bPath != "" {
		return aPath != "" && bPath != "" && sameCodeLocationAnchor(aLoc, bLoc)
	}
	return aLoc.LineRange != (model.LineRange{}) && aLoc.LineRange.SameAnchor(bLoc.LineRange)
}

func unanchoredSuggestion(a, b model.Suggestion) bool {
	aLoc := suggestionLocation(a)
	bLoc := suggestionLocation(b)
	return strings.TrimSpace(aLoc.FilePath) == "" &&
		strings.TrimSpace(bLoc.FilePath) == "" &&
		aLoc.LineRange == (model.LineRange{}) &&
		bLoc.LineRange == (model.LineRange{})
}

func suggestionLocation(s model.Suggestion) model.CodeLocation {
	loc := s.CodeLocation
	if loc.LineRange == (model.LineRange{}) {
		loc.LineRange = s.LineRange
	}
	return loc
}

func suggestionTokenSimilarity(a, b string) float64 {
	aTokens := suggestionTokenSet(a)
	bTokens := suggestionTokenSet(b)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return 0
	}
	intersection := 0
	for token := range aTokens {
		if _, ok := bTokens[token]; ok {
			intersection++
		}
	}
	union := len(aTokens) + len(bTokens) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func suggestionTokenSet(body string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range strings.FieldsFunc(strings.ToLower(body), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_'
	}) {
		if len(token) < 2 {
			continue
		}
		out[token] = struct{}{}
	}
	return out
}

func roundConfidenceScore(score float64) float64 {
	return math.Round(score*100) / 100
}
