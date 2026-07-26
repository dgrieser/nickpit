package review

import (
	"context"
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

// CategorizeRequest carries one finding into the categorize agent. It mirrors
// VerifyRequest minus the style guides: categorization judges what kind of item
// was submitted, not whether it obeys the project's rules, so injecting the
// styleguides would only invite the agent to start verifying.
type CategorizeRequest struct {
	ReviewCtx                 *model.ReviewContext
	Finding                   model.Finding
	RepoRoot                  string
	Section                   *logging.ReasoningSection
	Progress                  logging.ProgressInfo
	DisableJSONResponseFormat bool
	MaxToolCalls              int
	MaxDuplicateToolCalls     int
	MaxOutputRetries          int
	MaxReasoningSeconds       int
	DisableParallelToolCalls  bool
	DisableSuggestions        bool
	DisableDiffScope          bool
	DiffFormat                model.DiffFormat
}

type CategorizeOptions struct {
	// Limiter admits categorize agent calls in spawn order; it is the same
	// run-global limiter that caps every LLM agent loop.
	Limiter *Limiter
	// ReviewerName labels progress output when categorizing a single reviewer's
	// findings (per-vector lane steps); empty for the global categorize step.
	ReviewerName              string
	DisableJSONResponseFormat bool
	MaxToolCalls              int
	MaxDuplicateToolCalls     int
	MaxOutputRetries          int
	MaxReasoningSeconds       int
	DisableParallelToolCalls  bool
	DisableSuggestions        bool
	DisableDiffScope          bool
	RepoRoot                  string
	// DropPolicy is --verify-drop-policy. Categorization is the first pass
	// of verification, so the same policy applies to the verdict its
	// categories imply (see model.VerdictForCategories).
	DropPolicy string
	DiffFormat model.DiffFormat
}

type categorizeResult struct {
	Categorization          *model.FindingCategorization
	ReplacementCodeLocation *model.CodeLocation
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
	diffScopeEnabled := !req.DisableDiffScope && req.ReviewCtx.DiffScopeHunks != nil
	systemSnippet := llm.CategorizeExamplePromptSnippet()
	if diffScopeEnabled {
		systemSnippet = llm.ScopedCategorizeExamplePromptSnippet()
	}
	agentKind := "categorize"
	toolInstructions, err := e.renderToolInstructions(toolInstructionsConfig{
		agentRole:                agentKind,
		parallelToolCallGuidance: !req.DisableParallelToolCalls,
	})
	if err != nil {
		return nil, usage, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets(agentKind, systemSnippet, req.DisableSuggestions)
	if err != nil {
		return nil, usage, err
	}
	// No styleguides: the compilation category needs the toolchain versions to
	// know which compiler owns a diagnostic, but styleguide rules are evidence
	// for verification, not for classification.
	styleGuideToolchainSnippet, err := e.renderStyleGuideToolchainSnippet(agentKind, nil, len(req.ReviewCtx.ToolchainVersions) > 0)
	if err != nil {
		return nil, usage, err
	}
	// "imports or variables", "imports", "variables", or "" (bullet omitted):
	// only the kinds the finding language's default toolchain reports.
	unusedIdentifierKinds := strings.Join(mappings.UnusedIdentifierDiagnostics(filetype.DetectLanguage(req.Finding.CodeLocation.FilePath)), " or ")
	promptData := categorizePromptData{
		OutputSchemaSnippet:        systemSnippet,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		ParallelToolCallGuidance:   !req.DisableParallelToolCalls,
		HasTools:                   true,
		ToolInstructions:           toolInstructions,
		StyleGuideToolchainSnippet: styleGuideToolchainSnippet,
		DiffScopeEnabled:           diffScopeEnabled,
		UnusedIdentifierKinds:      unusedIdentifierKinds,
		CategoryConfirmation:       model.CategoryConfirmation,
		CategoryCompilation:        model.CategoryCompilation,
		CategoryOutsideDiffScope:   model.CategoryOutsideDiffScope,
		CategoryFinding:            model.CategoryFinding,
	}
	systemPrompt, err := llm.RenderPrompt(systemTemplate, promptData)
	if err != nil {
		return nil, usage, fmt.Errorf("categorize: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildFindingAgentUserPrompt(agentKind, req.ReviewCtx, req.Finding, req.DisableSuggestions, !req.DisableDiffScope, req.DiffFormat)
	if err != nil {
		return nil, usage, err
	}

	var schema []byte
	if !req.DisableJSONResponseFormat {
		schema = llm.CategorizeSchema
		if diffScopeEnabled {
			schema = llm.ScopedCategorizeSchema
		}
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
			AgentName:                         "Categorize Findings",
			AgentKind:                         agentKind,
			Progress:                          progress,
			Messages:                          messages,
			Tools:                             reviewerToolDefinitions(),
			Schema:                            schema,
			SchemaKind:                        llm.SchemaKindCategorize,
			Constraints:                       llm.ResponseConstraints{RequireReplacementCodeLocation: diffScopeEnabled},
			Model:                             e.config.Model,
			MaxTokens:                         e.config.MaxTokens,
			Temperature:                       e.config.Temperature,
			TopP:                              e.config.TopP,
			TopK:                              e.config.TopK,
			PresencePenalty:                   e.config.PresencePenalty,
			ExtraBody:                         e.config.ExtraBody,
			ParallelToolCalls:                 !req.DisableParallelToolCalls,
			ReasoningEffort:                   e.config.ReasoningEffort,
			RepoRoot:                          req.RepoRoot,
			MaxToolCalls:                      req.MaxToolCalls,
			MaxDuplicateToolCalls:             req.MaxDuplicateToolCalls,
			MaxOutputRetries:                  req.MaxOutputRetries,
			MaxReasoningSeconds:               req.MaxReasoningSeconds,
			State:                             state,
			Section:                           req.Section,
			NoToolsSystem:                     systemTemplate,
			NoToolsSchemaSnippet:              systemSnippet,
			NoToolsStyleGuideToolchainSnippet: styleGuideToolchainSnippet,
			JSONRetryExampleSnippet:           systemSnippet,
			NoToolsMessages: func(messages []llm.Message) ([]llm.Message, error) {
				return categorizeNoToolsMessages(systemTemplate, messages, promptData)
			},
		})
		if err != nil {
			return nil, usage, err
		}
		usage = addTokenUsage(usage, loopResult.tokensUsed)
		resp := loopResult.resp
		if resp != nil && resp.Categorization != nil && len(resp.Categorization.Categories) > 0 {
			model.EnsureCategorizationID(resp.Categorization, req.Finding.ID)
			return &categorizeResult{
				Categorization:          resp.Categorization,
				ReplacementCodeLocation: resp.ReplacementCodeLocation,
			}, usage, nil
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

// categorizePromptData is the render data for the categorize system prompt. It
// is a named type (unlike the other agents' anonymous structs) because the
// tools and no-tools renders must stay in sync — the no-tools variant only
// flips HasTools and drops the tool instructions.
type categorizePromptData struct {
	OutputSchemaSnippet        string
	OutputFormatSnippet        string
	ParallelToolCallGuidance   bool
	HasTools                   bool
	ToolInstructions           string
	StyleGuideToolchainSnippet string
	DiffScopeEnabled           bool
	UnusedIdentifierKinds      string
	CategoryConfirmation       string
	CategoryCompilation        string
	CategoryOutsideDiffScope   string
	CategoryFinding            string
}

// categorizeNoToolsMessages re-renders the system prompt without tools and
// rewrites the conversation the way noToolsMessages does, for models that turn
// out not to support tool calls mid-run.
func categorizeNoToolsMessages(systemTemplate string, messages []llm.Message, data categorizePromptData) ([]llm.Message, error) {
	commonSnippets, err := agentCommonSystemPromptSnippetsForTools("categorize", data.OutputSchemaSnippet, false, false)
	if err != nil {
		return nil, err
	}
	noToolsData := data
	noToolsData.HasTools = false
	noToolsData.ParallelToolCallGuidance = false
	noToolsData.ToolInstructions = ""
	noToolsData.OutputFormatSnippet = commonSnippets.outputFormat
	noToolsData.StyleGuideToolchainSnippet = strings.TrimSpace(data.StyleGuideToolchainSnippet)
	noToolsPrompt, err := llm.RenderPrompt(systemTemplate, noToolsData)
	if err != nil {
		return nil, fmt.Errorf("categorize: rendering no-tools system prompt: %w", err)
	}
	return replaceSystemPromptWithoutTools(noToolsPrompt, messages), nil
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
				RepoRoot:                  opts.RepoRoot,
				Section:                   sec,
				Progress:                  info,
				DisableJSONResponseFormat: opts.DisableJSONResponseFormat,
				MaxToolCalls:              opts.MaxToolCalls,
				MaxDuplicateToolCalls:     opts.MaxDuplicateToolCalls,
				MaxOutputRetries:          opts.MaxOutputRetries,
				MaxReasoningSeconds:       opts.MaxReasoningSeconds,
				DisableParallelToolCalls:  opts.DisableParallelToolCalls,
				DisableSuggestions:        opts.DisableSuggestions,
				DisableDiffScope:          opts.DisableDiffScope,
				DiffFormat:                opts.DiffFormat,
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
		if results[i].Categorization == nil {
			results[i].Categorization = fallbackFindingCategorization(findings[i])
		}
	}
	e.logProgress(logging.StageCategorize, logging.StateDone, fmt.Sprintf("%sfindings=%d prompt_tokens=%s completion_tokens=%s total_tokens=%s warnings=%d runtime=%s", categorizeReviewerPrefix(opts.ReviewerName), len(findings), model.HumanTokens(usageSum.PromptTokens), model.HumanTokens(usageSum.CompletionTokens), model.HumanTokens(usageSum.TotalTokens), len(warnings), model.HumanDuration(time.Since(categorizeStart))))
	return results, usageSum, warnings, nil
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

// categorizeReviewerPrefix labels per-reviewer categorize progress lines; the
// global categorize step (no reviewer name) keeps its unprefixed format.
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
		MaxToolCalls:              req.MaxToolCalls,
		MaxDuplicateToolCalls:     req.MaxDuplicateToolCalls,
		MaxOutputRetries:          req.MaxOutputRetries,
		MaxReasoningSeconds:       req.MaxReasoningSeconds,
		DisableParallelToolCalls:  req.DisableParallelToolCalls,
		DisableSuggestions:        req.DisableSuggestions,
		DisableDiffScope:          req.DisableDiffScope,
		RepoRoot:                  req.RepoRoot,
		DropPolicy:                req.VerifyDropPolicy,
		DiffFormat:                req.DiffFormat,
	}
}
