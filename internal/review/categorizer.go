package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/mappings"
)

// CategorizeRequest carries one finding into the private classifier.
// Categorization judges what kind of item was submitted, not whether its claim
// is true, so it receives no patch context, style guides, tools, or routing
// outcome.
type CategorizeRequest struct {
	ReviewCtx                 *model.ReviewContext
	Finding                   model.Finding
	Section                   *logging.ReasoningSection
	Progress                  logging.ProgressInfo
	DisableJSONResponseFormat bool
	MaxOutputRetries          int
	MaxReasoningSeconds       int
	DisableSuggestions        bool
}

type CategorizeOptions struct {
	// Limiter admits categorize agent calls in spawn order; it is the same
	// run-global limiter that caps every LLM agent loop.
	Limiter *Limiter
	// ReviewerName labels progress output for one reviewer's internal
	// classification phase.
	ReviewerName              string
	DisableJSONResponseFormat bool
	MaxOutputRetries          int
	MaxReasoningSeconds       int
	DisableSuggestions        bool
	// DropPolicy is applied privately by Go after descriptive classification.
	DropPolicy string
}

type categorizeResult struct {
	Categorization *model.FindingCategorization
}

func (e *Engine) Categorize(ctx context.Context, req CategorizeRequest) (*model.FindingCategorization, model.TokenUsage, error) {
	result, usage, err := e.categorizeFinding(ctx, req)
	if result == nil {
		return nil, usage, err
	}
	return result.Categorization, usage, err
}

func (e *Engine) categorizeFinding(ctx context.Context, req CategorizeRequest) (*categorizeResult, model.TokenUsage, error) {
	usage := model.TokenUsage{}
	if req.ReviewCtx == nil {
		return nil, usage, fmt.Errorf("categorize: nil review context")
	}
	if model.EnsureFindingID(&req.Finding) {
		e.logf(ctx, "Categorize generated replacement ID for invalid finding ID: title=%q", req.Finding.Title)
	}

	systemTemplate, err := e.loadPrompt("agent_categorize_system_prompt.tmpl")
	if err != nil {
		return nil, usage, err
	}
	systemSnippet := llm.CategorizeExamplePromptSnippet()
	agentKind := "categorize"
	commonSnippets, err := agentCommonSystemPromptSnippets(agentKind, systemSnippet, req.DisableSuggestions)
	if err != nil {
		return nil, usage, err
	}
	language := strings.TrimSpace(req.Finding.CodeLocation.Language)
	if language == "" {
		language = filetype.DetectLanguage(req.Finding.CodeLocation.FilePath)
	}
	promptData := categorizePromptData{
		OutputSchemaSnippet:   systemSnippet,
		OutputFormatSnippet:   commonSnippets.outputFormat,
		UnusedIdentifierKinds: strings.Join(mappings.UnusedIdentifierDiagnostics(language), " or "),
		CategoryConfirmation:  model.CategoryConfirmation,
		CategoryCompilation:   model.CategoryCompilation,
		CategoryFinding:       model.CategoryFinding,
	}
	systemPrompt, err := llm.RenderPrompt(systemTemplate, promptData)
	if err != nil {
		return nil, usage, fmt.Errorf("categorize: rendering system prompt: %w", err)
	}

	userPrompt, err := buildCategorizeUserPrompt(req.ReviewCtx, req.Finding, req.DisableSuggestions)
	if err != nil {
		return nil, usage, err
	}

	var schema []byte
	if !req.DisableJSONResponseFormat {
		schema = llm.CategorizeSchema
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	progress := req.Progress
	if progress.IsZero() {
		progress = e.progressInfo(agentKind, "Categorize Findings", "")
	}
	// One loop state is shared across the outer missing-categorization attempts
	// so tool-dedup and retry budgets carry over instead of resetting per
	// attempt, matching verifyFinding.
	state := newAgentLoopState()
	for attempt := 0; ; attempt++ {
		loopResult, err := e.runAgentLoop(ctx, agentLoopRequest{
			AgentName:               "Categorize Findings",
			AgentKind:               agentKind,
			Progress:                progress,
			Messages:                messages,
			Tools:                   nil,
			Schema:                  schema,
			SchemaKind:              llm.SchemaKindCategorize,
			Model:                   e.config.Model,
			MaxTokens:               e.config.MaxTokens,
			Temperature:             e.config.Temperature,
			TopP:                    e.config.TopP,
			TopK:                    e.config.TopK,
			PresencePenalty:         e.config.PresencePenalty,
			ExtraBody:               e.config.ExtraBody,
			ParallelToolCalls:       false,
			ReasoningEffort:         e.config.ReasoningEffort,
			MaxOutputRetries:        req.MaxOutputRetries,
			MaxReasoningSeconds:     req.MaxReasoningSeconds,
			State:                   state,
			Section:                 req.Section,
			NoToolsSystem:           systemPrompt,
			NoToolsSchemaSnippet:    systemSnippet,
			JSONRetryExampleSnippet: systemSnippet,
			NoToolsMessages: func(messages []llm.Message) ([]llm.Message, error) {
				return append([]llm.Message(nil), messages...), nil
			},
		})
		if err != nil {
			return nil, usage, err
		}
		usage = addTokenUsage(usage, loopResult.tokensUsed)
		resp := loopResult.resp
		if resp != nil && resp.Categorization != nil && len(resp.Categorization.Categories) > 0 {
			model.EnsureCategorizationID(resp.Categorization, req.Finding.ID)
			return &categorizeResult{Categorization: resp.Categorization}, usage, nil
		}
		if !outputRetriesRemaining(attempt, req.MaxOutputRetries) {
			return nil, usage, fmt.Errorf("categorize: missing categorization in response")
		}
		e.logf(ctx, "Categorize: missing categorization, retrying: attempt=%d", attempt+1)
		if len(loopResult.messages) > 0 {
			messages = loopResult.messages
		}
	}
}

// categorizePromptData keeps the classifier's descriptive labels explicit at
// the prompt boundary.
type categorizePromptData struct {
	OutputSchemaSnippet   string
	OutputFormatSnippet   string
	UnusedIdentifierKinds string
	CategoryConfirmation  string
	CategoryCompilation   string
	CategoryFinding       string
}

func buildCategorizeUserPrompt(reviewCtx *model.ReviewContext, finding model.Finding, disableSuggestions bool) (string, error) {
	submitted := struct {
		ID              string             `json:"id"`
		Title           string             `json:"title"`
		Body            string             `json:"body"`
		ConfidenceScore float64            `json:"confidence_score"`
		Priority        *int               `json:"priority,omitempty"`
		CodeLocation    model.CodeLocation `json:"code_location"`
		Suggestions     []model.Suggestion `json:"suggestions,omitempty"`
	}{
		ID:              finding.ID,
		Title:           finding.Title,
		Body:            finding.Body,
		ConfidenceScore: finding.ConfidenceScore,
		Priority:        finding.Priority,
		CodeLocation:    finding.CodeLocation,
	}
	if !disableSuggestions {
		submitted.Suggestions = finding.Suggestions
	}
	payload := struct {
		Finding           any                      `json:"finding"`
		ToolchainVersions []model.ToolchainVersion `json:"toolchain_versions,omitempty"`
	}{
		Finding: submitted,
	}
	if reviewCtx != nil {
		payload.ToolchainVersions = reviewCtx.ToolchainVersions
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("categorize: encoding input: %w", err)
	}
	return string(encoded), nil
}

func (e *Engine) CategorizeAll(ctx context.Context, reviewCtx *model.ReviewContext, findings []model.Finding, opts CategorizeOptions) ([]*model.FindingCategorization, model.TokenUsage, []string, error) {
	results, usage, warnings, err := e.categorizeAll(ctx, reviewCtx, findings, opts)
	categorizations := make([]*model.FindingCategorization, len(results))
	for i := range results {
		categorizations[i] = results[i].Categorization
	}
	return categorizations, usage, warnings, err
}

func (e *Engine) categorizeAll(ctx context.Context, reviewCtx *model.ReviewContext, findings []model.Finding, opts CategorizeOptions) ([]categorizeResult, model.TokenUsage, []string, error) {
	findings = append([]model.Finding(nil), findings...)
	if overwrote := model.EnsureFindingIDs(findings); overwrote > 0 {
		e.logf(ctx, "Categorize generated replacement IDs for invalid finding IDs: count=%d", overwrote)
	}
	results := make([]categorizeResult, len(findings))
	if len(findings) == 0 {
		return results, model.TokenUsage{}, nil, nil
	}

	var (
		mu       sync.Mutex
		usageSum model.TokenUsage
		warnings []string
		wg       sync.WaitGroup
	)
	categorizeStart := time.Now()
	e.logProgress(logging.StageCategorize, logging.StateStart, fmt.Sprintf("%sfindings=%d concurrency=%s", categorizeReviewerPrefix(opts.ReviewerName), len(findings), verifyConcurrencyLabel(opts.Limiter)))
	for i, finding := range findings {
		// Admission goes through the run-shared limiter in the spawn loop so
		// this call's findings start in order and compete fairly with every
		// other agent. Acquire fails only when ctx is done, so one aggregate
		// warning replaces a per-finding flood and the loop stops.
		categorizeCtx, release, err := opts.Limiter.Acquire(ctx)
		if err != nil {
			mu.Lock()
			warnings = append(warnings, fmt.Sprintf("Categorize cancelled at finding #%d %q: %v; skipped %d remaining finding(s)", i+1, finding.Title, err, len(findings)-i))
			mu.Unlock()
			break
		}
		wg.Add(1)
		go func(idx int, f model.Finding, ctx context.Context, release func()) {
			defer wg.Done()
			defer release()
			info := e.progressInfo("categorize", categorizeProgressName(opts.ReviewerName, idx), truncateFindingTitle(f.Title))
			sec := e.logger.NewReasoningTracker(info)
			defer sec.End()
			req := CategorizeRequest{
				ReviewCtx:                 reviewCtx,
				Finding:                   f,
				Section:                   sec,
				Progress:                  info,
				DisableJSONResponseFormat: opts.DisableJSONResponseFormat,
				MaxOutputRetries:          opts.MaxOutputRetries,
				MaxReasoningSeconds:       opts.MaxReasoningSeconds,
				DisableSuggestions:        opts.DisableSuggestions,
			}
			result, usage, err := e.categorizeFinding(ctx, req)
			mu.Lock()
			usageSum.PromptTokens += usage.PromptTokens
			usageSum.CompletionTokens += usage.CompletionTokens
			usageSum.TotalTokens += usage.TotalTokens
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Categorize failed for finding #%d %q: %v", idx+1, f.Title, err))
			}
			mu.Unlock()
			if err != nil {
				e.logf(ctx, "Categorize failed: index=%d title=%q error=%v", idx, f.Title, err)
				return
			}
			if result != nil {
				results[idx] = *result
			}
		}(i, finding, categorizeCtx, release)
	}
	wg.Wait()
	for i := range results {
		fallback := false
		if results[i].Categorization == nil {
			results[i].Categorization = fallbackFindingCategorization(findings[i])
			fallback = true
		}
		if e.logger != nil {
			categories := effectiveFindingCategories(results[i].Categorization)
			msg := fmt.Sprintf("categories=%s", strings.Join(categories, ","))
			if fallback {
				msg += " fallback=true"
			}
			e.logger.ProgressFor(
				e.progressInfo("categorize", categorizeProgressName(opts.ReviewerName, i), truncateFindingTitle(findings[i].Title)),
				logging.StageCategorize,
				logging.StateOK,
				msg,
			)
		}
	}
	e.logProgress(logging.StageCategorize, logging.StateDone, fmt.Sprintf("%sfindings=%d prompt_tokens=%s completion_tokens=%s total_tokens=%s warnings=%d runtime=%s", categorizeReviewerPrefix(opts.ReviewerName), len(findings), model.HumanTokens(usageSum.PromptTokens), model.HumanTokens(usageSum.CompletionTokens), model.HumanTokens(usageSum.TotalTokens), len(warnings), model.HumanDuration(time.Since(categorizeStart))))
	return results, usageSum, warnings, nil
}

func effectiveFindingCategories(categorization *model.FindingCategorization) []string {
	if categorization == nil {
		return []string{model.CategoryFinding}
	}
	categories := model.NormalizeFindingCategories(categorization.Categories)
	if len(categories) == 0 {
		return []string{model.CategoryFinding}
	}
	return categories
}

// fallbackFindingCategorization fails open. A categorize agent that errored, or
// that never produced a usable category list even after its retries, must not
// cost the run a real finding — so the finding is treated as a plain finding
// and continues to verification, where it still faces a full verdict.
func fallbackFindingCategorization(f model.Finding) *model.FindingCategorization {
	c := &model.FindingCategorization{
		ID:         f.ID,
		Categories: []string{model.CategoryFinding},
	}
	model.EnsureCategorizationID(c, f.ID)
	return c
}

// categorizeReviewerPrefix labels per-reviewer categorize progress lines.
// An unnamed classification phase keeps its unprefixed format.
func categorizeReviewerPrefix(reviewerName string) string {
	if reviewerName == "" {
		return ""
	}
	return fmt.Sprintf("reviewer=%q ", reviewerName)
}

// categorizeProgressName names a finding's categorize agent, scoped to its
// reviewer when categorizing a single reviewer's findings, e.g.
// "Categorize Code Quality #2".
func categorizeProgressName(reviewerName string, idx int) string {
	if reviewerName == "" {
		return fmt.Sprintf("Categorize #%d", idx+1)
	}
	return fmt.Sprintf("Categorize %s #%d", reviewerName, idx+1)
}

func categorizeOptionsFromReviewRequest(req model.ReviewRequest) CategorizeOptions {
	return CategorizeOptions{
		DisableJSONResponseFormat: req.DisableJSONResponseFormat,
		MaxOutputRetries:          req.MaxOutputRetries,
		MaxReasoningSeconds:       req.MaxReasoningSeconds,
		DisableSuggestions:        req.DisableSuggestions,
		DropPolicy:                req.VerifyDropPolicy,
	}
}
