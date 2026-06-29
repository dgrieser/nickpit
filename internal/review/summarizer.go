package review

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/google/uuid"
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
	DisableJSONResponseFormat bool
	MaxOutputRetries          int
	MaxReasoningSeconds       int
	MaxReasoningLoopRepeats   int
	DisableParallelToolCalls  bool
	DisablePatchSummary       bool
	DisableSuggestions        bool
	RepoRoot                  string
}

type summarizeItemKind string

const (
	summarizeItemFinding    summarizeItemKind = "finding"
	summarizeItemSuggestion summarizeItemKind = "suggestion"
	summarizeItemOverall    summarizeItemKind = "overall"
)

type summarizeTextItem struct {
	ID              string
	Title           string
	Body            string
	Kind            summarizeItemKind
	FindingIndex    int
	SuggestionIndex int
}

// Summarize shortens each finding's body and, when present, the overall
// explanation as one more text item. Finding metadata is copied in code.
func (e *Engine) Summarize(ctx context.Context, in *model.ReviewResult, opts SummarizeOptions) (*model.ReviewResult, model.AgentRun, error) {
	if in == nil {
		return nil, model.AgentRun{}, fmt.Errorf("summarize: nil review result")
	}
	if opts.DisableSuggestions {
		out, err := in.Clone()
		if err != nil {
			return nil, model.AgentRun{}, fmt.Errorf("summarize: cloning input result: %w", err)
		}
		model.StripSuggestions(out.Findings)
		in = out
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
	items := summarizeItemsForResult(in, strings.TrimSpace(in.OverallExplanation) != "", opts.DisableSuggestions)
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
	stats := applySummarizerBodies(out.Findings, items, bodies)
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
	bodies, run, err := e.summarizeTextItems(ctx, []summarizeTextItem{{ID: overallSummaryID, Title: "Overall explanation", Body: overall, Kind: summarizeItemOverall}}, opts)
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
	commonSnippets, err := agentCommonSystemPromptSnippets("summarize", summarizeOutputSchemaSnippetFor(opts.DisableJSONResponseFormat), opts.DisableSuggestions)
	if err != nil {
		return nil, model.AgentRun{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet string
		OutputFormatSnippet string
		DisablePatchSummary bool
		DisableSuggestions  bool
	}{
		OutputSchemaSnippet: summarizeOutputSchemaSnippetFor(opts.DisableJSONResponseFormat),
		OutputFormatSnippet: commonSnippets.outputFormat,
		DisablePatchSummary: opts.DisablePatchSummary,
		DisableSuggestions:  opts.DisableSuggestions,
	})
	if err != nil {
		return nil, model.AgentRun{}, fmt.Errorf("summarize: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildSummarizeUserPrompt(items)
	if err != nil {
		return nil, model.AgentRun{}, err
	}

	var schema []byte
	if !opts.DisableJSONResponseFormat {
		schema = llm.SummarizeSchema
	}

	req := model.ReviewRequest{
		RepoRoot:                  opts.RepoRoot,
		MaxOutputRetries:          opts.MaxOutputRetries,
		MaxReasoningSeconds:       opts.MaxReasoningSeconds,
		MaxReasoningLoopRepeats:   opts.MaxReasoningLoopRepeats,
		DisableParallelToolCalls:  opts.DisableParallelToolCalls,
		DisableSuggestions:        opts.DisableSuggestions,
		DisableJSONResponseFormat: opts.DisableJSONResponseFormat,
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
	bodies := summarizerBodiesForOutput(items, result.resp.Findings)
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
		if recovered, ok := recoverSummarizerFindingsByPosition(items, summarizerFindings); ok {
			summarizerFindings = recovered
		}
		stats := summarizerTextOutputStats(items, summarizerFindings)
		countOK := stats.Omitted == 0 && stats.Ignored == 0
		if countOK {
			return nil
		}
		ids := summarizerOutputIDDiagnostics(items, summarizerFindings)
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
			Got:        stats.SummarizerFindings,
			Matched:    stats.Matched,
			Omitted:    stats.Omitted,
			Ignored:    stats.Ignored,
			AllowedIDs: ids.AllowedIDs,
			OmittedIDs: ids.OmittedIDs,
			IgnoredIDs: ids.IgnoredIDs,
		}
		return invalid
	}
}

func summarizeItemsForResult(in *model.ReviewResult, includeOverall bool, disableSuggestions bool) []summarizeTextItem {
	items := make([]summarizeTextItem, 0, len(in.Findings)+1)
	for findingIndex, finding := range in.Findings {
		title, body := sourceTitleBodyForSummary(finding)
		items = append(items, summarizeTextItem{
			ID:           finding.ID,
			Title:        title,
			Body:         body,
			Kind:         summarizeItemFinding,
			FindingIndex: findingIndex,
		})
		if !disableSuggestions {
			for suggestionIndex, suggestion := range sourceSuggestionsForSummary(finding) {
				if !shouldSummarizeSuggestionBody(suggestion.Body) {
					continue
				}
				items = append(items, summarizeTextItem{
					ID:              summarizeSuggestionItemID(finding.ID, suggestionIndex),
					Title:           fmt.Sprintf("Suggestion for: %s", title),
					Body:            suggestion.Body,
					Kind:            summarizeItemSuggestion,
					FindingIndex:    findingIndex,
					SuggestionIndex: suggestionIndex,
				})
			}
		}
	}
	if includeOverall && strings.TrimSpace(in.OverallExplanation) != "" {
		items = append(items, summarizeTextItem{ID: overallSummaryID, Title: "Overall explanation", Body: in.OverallExplanation, Kind: summarizeItemOverall})
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

func sourceSuggestionsForSummary(finding model.Finding) []model.Suggestion {
	if finding.Finalization != nil && len(finding.Finalization.Suggestions) > 0 {
		return finding.Finalization.Suggestions
	}
	return finding.Suggestions
}

func (e *Engine) buildSummarizeUserPrompt(items []summarizeTextItem) (string, error) {
	findings := make([]map[string]any, 0, len(items))
	for _, item := range items {
		findings = append(findings, map[string]any{
			"id":    item.ID,
			"kind":  summarizeItemKindOrDefault(item),
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

type summarizerIDDiagnostics struct {
	AllowedIDs []string
	OmittedIDs []string
	IgnoredIDs []string
}

func summarizerBodiesForOutput(items []summarizeTextItem, out []model.Finding) map[string]string {
	if recovered, ok := recoverSummarizerFindingsByPosition(items, out); ok {
		out = recovered
	}
	bodies := make(map[string]string, len(out))
	for _, finding := range out {
		if finding.Summarization == nil {
			continue
		}
		bodies[strings.TrimSpace(finding.ID)] = finding.Summarization.Body
	}
	return bodies
}

func applySummarizerBodies(inOut []model.Finding, items []summarizeTextItem, bodies map[string]string) summarizerApplyStats {
	itemsByID := make(map[string]summarizeTextItem, len(items))
	for _, item := range items {
		itemsByID[strings.TrimSpace(item.ID)] = item
	}
	matched := make([]bool, len(inOut))
	stats := summarizerApplyStats{SummarizerFindings: 0}
	for id, body := range bodies {
		if id == overallSummaryID {
			continue
		}
		item, ok := itemsByID[id]
		if !ok {
			stats.Ignored++
			continue
		}
		switch summarizeItemKindOrDefault(item) {
		case summarizeItemSuggestion:
			if strings.TrimSpace(body) == "" || item.FindingIndex < 0 || item.FindingIndex >= len(inOut) {
				continue
			}
			summary := inOut[item.FindingIndex].Summarization
			if summary == nil {
				summary = baseSummarization(&inOut[item.FindingIndex])
				inOut[item.FindingIndex].Summarization = summary
			}
			if item.SuggestionIndex < 0 || item.SuggestionIndex >= len(summary.Suggestions) {
				continue
			}
			summary.Suggestions[item.SuggestionIndex].Body = body
		case summarizeItemFinding:
			stats.SummarizerFindings++
			if item.FindingIndex < 0 || item.FindingIndex >= len(inOut) || matched[item.FindingIndex] || inOut[item.FindingIndex].ID != id {
				stats.Ignored++
				continue
			}
			matched[item.FindingIndex] = true
			stats.Matched++
			src := model.Finding{ID: id, Summarization: &model.FindingSummarization{Body: body}}
			applySummarizedFinding(&inOut[item.FindingIndex], src)
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

func recoverSummarizerFindingsByPosition(items []summarizeTextItem, out []model.Finding) ([]model.Finding, bool) {
	if len(items) == 0 || len(out) != len(items) {
		return nil, false
	}
	knownIDs := make(map[string]struct{}, len(items))
	for _, item := range items {
		if id := strings.TrimSpace(item.ID); id != "" {
			knownIDs[id] = struct{}{}
		}
	}
	samePositionKnown := 0
	recovered := make([]model.Finding, len(out))
	for i := range out {
		if out[i].Summarization == nil || strings.TrimSpace(out[i].Summarization.Body) == "" {
			return nil, false
		}
		id := strings.TrimSpace(out[i].ID)
		if _, known := knownIDs[id]; known {
			if id != strings.TrimSpace(items[i].ID) {
				return nil, false
			}
			samePositionKnown++
		}
		recovered[i] = out[i]
		recovered[i].ID = items[i].ID
	}
	if len(items) > 1 && samePositionKnown == 0 {
		return nil, false
	}
	return recovered, true
}

func summarizerTextOutputStats(items []summarizeTextItem, out []model.Finding) summarizerApplyStats {
	want := make(map[string]int, len(items))
	known := make(map[string]int, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		known[id]++
		if summarizeItemRequired(item) {
			want[id]++
		}
	}
	stats := summarizerApplyStats{SummarizerFindings: len(out)}
	for _, finding := range out {
		id := strings.TrimSpace(finding.ID)
		if known[id] > 0 {
			known[id]--
			if want[id] > 0 {
				want[id]--
			}
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

func summarizerOutputIDDiagnostics(items []summarizeTextItem, out []model.Finding) summarizerIDDiagnostics {
	ids := summarizerIDDiagnostics{AllowedIDs: make([]string, 0, len(items))}
	matched := make([]bool, len(items))
	for _, item := range items {
		ids.AllowedIDs = append(ids.AllowedIDs, item.ID)
	}
	for _, finding := range out {
		found := false
		findingID := strings.TrimSpace(finding.ID)
		for i, item := range items {
			if matched[i] || findingID != item.ID {
				continue
			}
			matched[i] = true
			found = true
			break
		}
		if !found {
			ids.IgnoredIDs = append(ids.IgnoredIDs, findingID)
		}
	}
	for i, item := range items {
		if !matched[i] && summarizeItemRequired(item) {
			ids.OmittedIDs = append(ids.OmittedIDs, item.ID)
		}
	}
	return ids
}

// applySummarizedFinding sets dst.Summarization to a copy of dst.Finalization
// (all fields verbatim) with the body replaced by the summarizer's shortened
// body. An empty/whitespace LLM body is ignored so the finalized body survives.
func applySummarizedFinding(dst *model.Finding, src model.Finding) {
	existingSuggestions := []model.Suggestion(nil)
	if dst.Summarization != nil {
		existingSuggestions = dst.Summarization.Suggestions
	}
	summary := baseSummarization(dst)
	for i := range existingSuggestions {
		if i >= len(summary.Suggestions) {
			break
		}
		if strings.TrimSpace(existingSuggestions[i].Body) != "" {
			summary.Suggestions[i].Body = existingSuggestions[i].Body
		}
	}
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
			Suggestions:     cloneSuggestions(sourceSuggestionsForSummary(*finding)),
		}
	}
	return &model.FindingSummarization{
		Title:           finding.Title,
		Body:            finding.Body,
		Priority:        model.PriorityRank(finding.Priority),
		ConfidenceScore: finding.ConfidenceScore,
		Suggestions:     cloneSuggestions(finding.Suggestions),
	}
}

func summarizeOutputSchemaSnippetFor(disableJSONResponseFormat bool) string {
	if !disableJSONResponseFormat {
		return ""
	}
	return llm.SummarizeExamplePromptSnippet()
}

func summarizeItemKindOrDefault(item summarizeTextItem) summarizeItemKind {
	if item.Kind == "" {
		return summarizeItemFinding
	}
	return item.Kind
}

func summarizeItemRequired(item summarizeTextItem) bool {
	return summarizeItemKindOrDefault(item) != summarizeItemSuggestion
}

func summarizeSuggestionItemID(findingID string, suggestionIndex int) string {
	data := fmt.Appendf(nil, "nickpit:summarize:suggestion:%s:%d", strings.TrimSpace(findingID), suggestionIndex)
	return uuid.NewSHA1(uuid.NameSpaceOID, data).String()
}

func shouldSummarizeSuggestionBody(body string) bool {
	text := strings.TrimSpace(body)
	if text == "" || strings.Contains(text, "```") {
		return false
	}
	if looksCodeLikeSuggestion(text) {
		return false
	}
	return wordCount(text) >= 6
}

func looksCodeLikeSuggestion(text string) bool {
	lines := strings.Split(text, "\n")
	codeSignals := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "@@"),
			strings.HasPrefix(trimmed, "diff --git"),
			strings.HasPrefix(trimmed, "+++"),
			strings.HasPrefix(trimmed, "---"),
			strings.HasPrefix(trimmed, "+") && !strings.HasPrefix(trimmed, "+ "),
			strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "- "),
			strings.Contains(trimmed, ":="),
			strings.Contains(trimmed, "=>"),
			strings.Contains(trimmed, "&&"),
			strings.Contains(trimmed, "||"),
			strings.Contains(trimmed, "{{"),
			strings.Contains(trimmed, "}}"):
			codeSignals++
		}
	}
	if codeSignals > 0 {
		return true
	}
	return len(lines) > 1 && strings.ContainsAny(text, "{};$")
}

func wordCount(text string) int {
	words := 0
	inWord := false
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			if !inWord {
				words++
				inWord = true
			}
			continue
		}
		inWord = false
	}
	return words
}
