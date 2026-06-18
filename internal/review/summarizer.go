package review

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
)

// overallSummaryID is the sentinel id for the overall-explanation text item the
// summarizer shortens alongside findings. It must be a valid UUID because the
// summarize schema constrains every item id to format: uuid; this reserved v4
// UUID is distinct from any real finding id and is stripped before findings are
// applied (it never escapes into output).
const overallSummaryID = "00000000-0000-4000-8000-0000000a11ee"

// SummarizeOptions mirrors FinalizeOptions: the per-step model/budget knobs the
// summarize pass forwards into its single batch agent run.
type SummarizeOptions struct {
	UseJSONSchema            bool
	MaxOutputRetries         int
	MaxReasoningSeconds      int
	MaxReasoningLoopRepeats  int
	DisableParallelToolCalls bool
	DisablePatchSummary      bool
	RepoRoot                 string
}

type summarizeTextItem struct {
	ID    string
	Title string
	Body  string
}

// Summarize shortens each finding's body and, when present, the overall
// explanation as one more text item. Finding metadata is copied in code.
func (e *Engine) Summarize(ctx context.Context, in *model.ReviewResult, opts SummarizeOptions) (*model.ReviewResult, model.AgentRun, error) {
	if in == nil {
		return nil, model.AgentRun{}, fmt.Errorf("summarize: nil review result")
	}
	// With no findings the overall explanation is a short static message (e.g.
	// "No finalized findings remained."), so there is nothing worth an LLM call.
	if len(in.Findings) == 0 {
		out, err := in.Clone()
		if err != nil {
			return nil, model.AgentRun{}, fmt.Errorf("summarize: cloning input result: %w", err)
		}
		return out, model.AgentRun{Name: "Summarize Review", Role: "summarize", Status: model.AgentRunStatusSkipped}, nil
	}
	items := summarizeItemsForResult(in, strings.TrimSpace(in.OverallExplanation) != "")
	if len(items) == 0 {
		out, err := in.Clone()
		if err != nil {
			return nil, model.AgentRun{}, fmt.Errorf("summarize: cloning input result: %w", err)
		}
		return out, model.AgentRun{Name: "Summarize Review", Role: "summarize"}, nil
	}
	bodies, run, err := e.summarizeTextItems(ctx, items, opts)
	if err != nil {
		return nil, run, err
	}
	out, err := in.Clone()
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("summarize: cloning input result: %w", err)
	}
	stats := applySummarizerBodies(out.Findings, bodies)
	if body := strings.TrimSpace(bodies[overallSummaryID]); body != "" {
		out.OverallExplanation = body
	}
	if stats.Omitted > 0 || stats.Ignored > 0 || stats.SummarizerFindings != len(in.Findings) {
		out.Warnings = append(out.Warnings, fmt.Sprintf("Summarizer output mismatch: findings_in=%d summarizer_findings=%d matched=%d omitted=%d ignored=%d; preserved finalized bodies", len(in.Findings), stats.SummarizerFindings, stats.Matched, stats.Omitted, stats.Ignored))
	}
	return out, run, nil
}

func (e *Engine) SummarizeOverall(ctx context.Context, overall string, opts SummarizeOptions) (string, model.AgentRun, error) {
	if strings.TrimSpace(overall) == "" {
		return overall, model.AgentRun{Name: "Summarize Review", Role: "summarize", Status: model.AgentRunStatusSkipped}, nil
	}
	bodies, run, err := e.summarizeTextItems(ctx, []summarizeTextItem{{ID: overallSummaryID, Title: "Overall explanation", Body: overall}}, opts)
	if err != nil {
		return overall, run, err
	}
	if body := strings.TrimSpace(bodies[overallSummaryID]); body != "" {
		return body, run, nil
	}
	return overall, run, nil
}

func (e *Engine) summarizeTextItems(ctx context.Context, items []summarizeTextItem, opts SummarizeOptions) (map[string]string, model.AgentRun, error) {
	systemTemplate, err := e.loadPrompt("agent_summarize_system_prompt.tmpl")
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("summarize", summarizeOutputSchemaSnippetFor(opts.UseJSONSchema), false)
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet string
		OutputFormatSnippet string
		DisablePatchSummary bool
	}{
		OutputSchemaSnippet: summarizeOutputSchemaSnippetFor(opts.UseJSONSchema),
		OutputFormatSnippet: commonSnippets.outputFormat,
		DisablePatchSummary: opts.DisablePatchSummary,
	})
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("summarize: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildSummarizeUserPrompt(items)
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
	summarizeStart := time.Now()
	e.logProgress(logging.StageSummarize, logging.StateStart, fmt.Sprintf("items=%d", len(items)))
	result, err := e.runAgent(ctx, agentSpec{
		name:             "Summarize Review",
		role:             "summarize",
		system:           system,
		noToolsSystem:    system,
		user:             userPrompt,
		schema:           schema,
		schemaKind:       llm.SchemaKindSummarize,
		hasTools:         false,
		validateResponse: summarizerOutputValidator(items),
	}, req)
	if err != nil {
		// Preserve the partial AgentRun (tokens accrued before the loop aborted)
		// for telemetry parity with the finalize/merge failure paths.
		return nil, result.run, err
	}
	if result.resp == nil {
		return nil, result.run, fmt.Errorf("summarize: agent returned nil response")
	}
	bodies := make(map[string]string, len(result.resp.Findings))
	for _, finding := range result.resp.Findings {
		if finding.Summarization == nil {
			continue
		}
		bodies[finding.ID] = finding.Summarization.Body
	}
	e.logProgress(logging.StageSummarize, logging.StateDone, fmt.Sprintf("items_in=%d items_out=%d prompt_tokens=%s completion_tokens=%s total_tokens=%s runtime=%s", len(items), len(bodies), model.HumanTokens(result.run.TokensUsed.PromptTokens), model.HumanTokens(result.run.TokensUsed.CompletionTokens), model.HumanTokens(result.run.TokensUsed.TotalTokens), model.HumanDuration(time.Since(summarizeStart))))
	return bodies, result.run, nil
}

func summarizerOutputValidator(items []summarizeTextItem) func(*llm.ReviewResponse) *llm.InvalidResponseError {
	return func(resp *llm.ReviewResponse) *llm.InvalidResponseError {
		expected := len(items)
		var summarizerFindings []model.Finding
		if resp != nil {
			summarizerFindings = resp.Findings
		}
		stats := summarizerTextOutputStats(items, summarizerFindings)
		countOK := stats.SummarizerFindings == expected && stats.Matched == expected && stats.Omitted == 0 && stats.Ignored == 0
		if countOK {
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
			Reason:          fmt.Sprintf("summarizer_output_mismatch got=%d expected=%d matched=%d omitted=%d ignored=%d", stats.SummarizerFindings, expected, stats.Matched, stats.Omitted, stats.Ignored),
			MissingFields:   []string{"findings"},
			ReasoningEffort: reasoningEffort,
		}
		invalid.RetryGuidanceTemplate = "summarizer_count_retry_guidance.tmpl"
		invalid.RetryGuidanceData = struct {
			Expected int
			Got      int
			Matched  int
			Omitted  int
			Ignored  int
		}{
			Expected: expected,
			Got:      stats.SummarizerFindings,
			Matched:  stats.Matched,
			Omitted:  stats.Omitted,
			Ignored:  stats.Ignored,
		}
		return invalid
	}
}

func summarizeItemsForResult(in *model.ReviewResult, includeOverall bool) []summarizeTextItem {
	items := make([]summarizeTextItem, 0, len(in.Findings)+1)
	for _, finding := range in.Findings {
		title, body := sourceTitleBodyForSummary(finding)
		items = append(items, summarizeTextItem{ID: finding.ID, Title: title, Body: body})
	}
	if includeOverall && strings.TrimSpace(in.OverallExplanation) != "" {
		items = append(items, summarizeTextItem{ID: overallSummaryID, Title: "Overall explanation", Body: in.OverallExplanation})
	}
	return items
}

func sourceTitleBodyForSummary(finding model.Finding) (string, string) {
	title, body := finding.Title, finding.Body
	if finding.Finalization != nil {
		title, body = finding.Finalization.Title, finding.Finalization.Body
	}
	return title, body
}

func (e *Engine) buildSummarizeUserPrompt(items []summarizeTextItem) (string, error) {
	findings := make([]map[string]any, 0, len(items))
	for _, item := range items {
		findings = append(findings, map[string]any{
			"id":    item.ID,
			"title": item.Title,
			"body":  item.Body,
		})
	}
	payloadMap := map[string]any{"findings": findings}
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

func applySummarizerBodies(inOut []model.Finding, bodies map[string]string) summarizerApplyStats {
	matched := make([]bool, len(inOut))
	stats := summarizerApplyStats{SummarizerFindings: 0}
	for id, body := range bodies {
		if id == overallSummaryID {
			continue
		}
		stats.SummarizerFindings++
		found := false
		for i := range inOut {
			if matched[i] || inOut[i].ID != id {
				continue
			}
			matched[i] = true
			found = true
			stats.Matched++
			src := model.Finding{ID: id, Summarization: &model.FindingSummarization{Body: body}}
			applySummarizedFinding(&inOut[i], src)
			break
		}
		if !found {
			stats.Ignored++
		}
	}
	for i := range inOut {
		if !matched[i] {
			stats.Omitted++
			if inOut[i].Summarization == nil {
				inOut[i].Summarization = baseSummarization(&inOut[i])
			}
		}
	}
	return stats
}

func summarizerTextOutputStats(items []summarizeTextItem, out []model.Finding) summarizerApplyStats {
	want := make(map[string]int, len(items))
	for _, item := range items {
		want[item.ID]++
	}
	stats := summarizerApplyStats{SummarizerFindings: len(out)}
	for _, finding := range out {
		if want[finding.ID] > 0 {
			want[finding.ID]--
			stats.Matched++
		} else {
			stats.Ignored++
		}
	}
	for _, count := range want {
		stats.Omitted += count
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
