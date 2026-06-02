package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

// SummarizeOptions mirrors FinalizeOptions: the per-step model/budget knobs the
// summarize pass forwards into its single batch agent run.
type SummarizeOptions struct {
	UseJSONSchema            bool
	MaxOutputRetries         int
	MaxReasoningSeconds      int
	MaxReasoningLoopRepeats  int
	DisableParallelToolCalls bool
	RepoRoot                 string
}

// Summarize shortens each finding's body. It runs a single no-tools batch agent
// over the finalized result, reading each finding's finalized body as source and
// writing a concise `summarization.body`. Every other summarization field is
// copied verbatim in code from the finding's finalization (see
// applySummarizedFinding), so the LLM only ever produces the shortened body. Like
// Finalize, an empty input is a no-op and any failure is soft at the call site.
func (e *Engine) Summarize(ctx context.Context, in *model.ReviewResult, opts SummarizeOptions) (*model.ReviewResult, model.AgentRun, error) {
	if in == nil {
		return nil, model.AgentRun{}, fmt.Errorf("summarize: nil review result")
	}
	if len(in.Findings) == 0 {
		out, err := in.Clone()
		if err != nil {
			return nil, model.AgentRun{}, fmt.Errorf("summarize: cloning input result: %w", err)
		}
		return out, model.AgentRun{Name: "Summarize Review", Role: "summarize"}, nil
	}

	systemTemplate, err := e.loadPrompt("agent_summarize_system_prompt.tmpl")
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet string
	}{
		OutputSchemaSnippet: summarizeOutputSchemaSnippetFor(opts.UseJSONSchema),
	})
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("summarize: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildSummarizeUserPrompt(in)
	if err != nil {
		return nil, model.AgentRun{}, err
	}

	var schema []byte
	if opts.UseJSONSchema {
		schema = llm.SummarizeSchema
	}

	req := model.ReviewRequest{
		RepoRoot:                 opts.RepoRoot,
		MaxOutputRetries:         opts.MaxOutputRetries,
		MaxReasoningSeconds:      opts.MaxReasoningSeconds,
		MaxReasoningLoopRepeats:  opts.MaxReasoningLoopRepeats,
		DisableParallelToolCalls: opts.DisableParallelToolCalls,
		UseJSONSchema:            opts.UseJSONSchema,
	}
	e.logProgress("Summarize", fmt.Sprintf("findings=%d", len(in.Findings)))
	result, err := e.runAgent(ctx, agentSpec{
		name:             "Summarize Review",
		role:             "summarize",
		system:           system,
		noToolsSystem:    system,
		user:             userPrompt,
		schema:           schema,
		schemaKind:       llm.SchemaKindSummarize,
		hasTools:         false,
		validateResponse: summarizerOutputValidator(in.Findings, in.OverallExplanation),
	}, req)
	if err != nil {
		// Preserve the partial AgentRun (tokens accrued before the loop aborted)
		// for telemetry parity with the finalize/merge failure paths.
		return nil, result.run, err
	}
	if result.resp == nil {
		return nil, result.run, fmt.Errorf("summarize: agent returned nil response")
	}

	out, err := in.Clone()
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("summarize: cloning input result: %w", err)
	}
	stats := applySummarizerOutput(out.Findings, result.resp.Findings)
	// Adopt the shortened overall_explanation; keep the finalized one if the
	// model omitted it (soft, mirroring the per-finding body handling).
	if strings.TrimSpace(result.resp.OverallExplanation) != "" {
		out.OverallExplanation = result.resp.OverallExplanation
	}
	if stats.Omitted > 0 || stats.Ignored > 0 || stats.SummarizerFindings != len(in.Findings) {
		out.Warnings = append(out.Warnings, fmt.Sprintf("Summarizer output mismatch: findings_in=%d summarizer_findings=%d matched=%d omitted=%d ignored=%d; preserved finalized bodies", len(in.Findings), stats.SummarizerFindings, stats.Matched, stats.Omitted, stats.Ignored))
	}
	e.logProgress("Summarize", fmt.Sprintf("done findings_in=%d summarizer_findings=%d matched=%d omitted=%d ignored=%d findings_out=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d", len(in.Findings), stats.SummarizerFindings, stats.Matched, stats.Omitted, stats.Ignored, len(out.Findings), result.run.TokensUsed.PromptTokens, result.run.TokensUsed.CompletionTokens, result.run.TokensUsed.TotalTokens))
	return out, result.run, nil
}

func summarizerOutputValidator(inputFindings []model.Finding, inputOverall string) func(*llm.ReviewResponse) *llm.InvalidResponseError {
	return func(resp *llm.ReviewResponse) *llm.InvalidResponseError {
		expected := len(inputFindings)
		var summarizerFindings []model.Finding
		respOverall := ""
		if resp != nil {
			summarizerFindings = resp.Findings
			respOverall = resp.OverallExplanation
		}
		stats := summarizerOutputStats(inputFindings, summarizerFindings, nil)
		countOK := stats.SummarizerFindings == expected && stats.Matched == expected && stats.Omitted == 0 && stats.Ignored == 0
		// Only require a shortened overall_explanation back when we sent one to
		// shorten. A standalone summarize over findings with no overall (e.g.
		// `--step summarize --findings raw.json`) must not be forced to invent one.
		overallMissing := strings.TrimSpace(inputOverall) != "" && strings.TrimSpace(respOverall) == ""
		if countOK && !overallMissing {
			return nil
		}
		raw := ""
		reasoningEffort := ""
		if resp != nil {
			raw = resp.RawResponse
			reasoningEffort = resp.ReasoningEffort
		}
		missing := []string{"findings"}
		if overallMissing {
			missing = append(missing, "overall_explanation")
		}
		invalid := &llm.InvalidResponseError{
			RawContent:      raw,
			Reason:          fmt.Sprintf("summarizer_output_mismatch got=%d expected=%d matched=%d omitted=%d ignored=%d overall_missing=%t", stats.SummarizerFindings, expected, stats.Matched, stats.Omitted, stats.Ignored, overallMissing),
			MissingFields:   missing,
			ReasoningEffort: reasoningEffort,
		}
		invalid.RetryGuidanceTemplate = "summarizer_count_retry_guidance.tmpl"
		invalid.RetryGuidanceData = struct {
			Expected       int
			Got            int
			Matched        int
			Omitted        int
			Ignored        int
			OverallMissing bool
		}{
			Expected:       expected,
			Got:            stats.SummarizerFindings,
			Matched:        stats.Matched,
			Omitted:        stats.Omitted,
			Ignored:        stats.Ignored,
			OverallMissing: overallMissing,
		}
		return invalid
	}
}

// buildSummarizeUserPrompt sends one entry per finding: its id plus the finalized
// title/body to shorten. The finalized body is the source; when a finding has no
// finalization (e.g. a standalone --step summarize over raw findings) its own
// title/body are used instead.
func (e *Engine) buildSummarizeUserPrompt(in *model.ReviewResult) (string, error) {
	findings := make([]map[string]any, 0, len(in.Findings))
	for _, finding := range in.Findings {
		title, body := finding.Title, finding.Body
		if finding.Finalization != nil {
			title, body = finding.Finalization.Title, finding.Finalization.Body
		}
		findings = append(findings, map[string]any{
			"id":    finding.ID,
			"title": title,
			"body":  body,
		})
	}
	payloadMap := map[string]any{"findings": findings}
	// The finalized overall_explanation already folds in the context notes; send
	// it so the summarizer shortens it alongside the finding bodies.
	if strings.TrimSpace(in.OverallExplanation) != "" {
		payloadMap["overall_explanation"] = in.OverallExplanation
	}
	user, err := llm.RenderJSON(payloadMap)
	if err != nil {
		return "", fmt.Errorf("summarize: rendering summarize prompt json: %w", err)
	}
	return user, nil
}

type summarizerApplyStats struct {
	SummarizerFindings int
	Matched            int
	Omitted            int
	Ignored            int
}

// applySummarizerOutput writes a `summarization` onto each input-owned finding
// from the summarizer output. The summarizer cannot add, remove, reorder, or
// relocate findings; unmatched output is ignored, and omitted input findings
// receive a summarization synthesized from their finalization (unshortened body).
func applySummarizerOutput(inOut, summarizer []model.Finding) summarizerApplyStats {
	return summarizerOutputStats(inOut, summarizer, func(inIdx, outIdx int) {
		applySummarizedFinding(&inOut[inIdx], summarizer[outIdx])
	})
}

func summarizerOutputStats(inOut, summarizer []model.Finding, onMatch func(inIdx, outIdx int)) summarizerApplyStats {
	stats := summarizerApplyStats{SummarizerFindings: len(summarizer)}
	matchedInput := make([]bool, len(inOut))
	usedOutput := make([]bool, len(summarizer))
	for outIdx := range summarizer {
		// Reuse the finalizer's id→location→title matcher: the output carries the
		// same top-level ids/locations, so matching is identical.
		inIdx := findFinalizerInputIndex(summarizer[outIdx], inOut, matchedInput)
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
	for i := range summarizer {
		if !usedOutput[i] {
			stats.Ignored++
		}
	}
	for i := range inOut {
		if matchedInput[i] {
			continue
		}
		stats.Omitted++
		if onMatch != nil && inOut[i].Summarization == nil {
			inOut[i].Summarization = baseSummarization(&inOut[i])
		}
	}
	return stats
}

// applySummarizedFinding sets dst.Summarization to a copy of dst.Finalization
// (all fields verbatim) with the body replaced by the summarizer's shortened
// body. An empty/whitespace LLM body is ignored so the finalized body survives.
func applySummarizedFinding(dst *model.Finding, src model.Finding) {
	summary := baseSummarization(dst)
	if src.Summarization != nil && strings.TrimSpace(src.Summarization.Body) != "" {
		summary.Body = src.Summarization.Body
	}
	dst.Summarization = summary
}

// baseSummarization clones the finding's finalization into a FindingSummarization
// (every field copied verbatim). When the finding has no finalization, it falls
// back to the finding's own fields, mirroring synthesizedFinalization.
func baseSummarization(finding *model.Finding) *model.FindingSummarization {
	if finding.Finalization != nil {
		return &model.FindingSummarization{
			Title:           finding.Finalization.Title,
			Body:            finding.Finalization.Body,
			Priority:        finding.Finalization.Priority,
			ConfidenceScore: finding.Finalization.ConfidenceScore,
			Remarks:         finding.Finalization.Remarks,
		}
	}
	return &model.FindingSummarization{
		Title:           finding.Title,
		Body:            finding.Body,
		Priority:        model.PriorityRank(finding.Priority),
		ConfidenceScore: finding.ConfidenceScore,
	}
}

func summarizeOutputSchemaSnippetFor(useJSONSchema bool) string {
	if useJSONSchema {
		return ""
	}
	return llm.SummarizeExamplePromptSnippet()
}
