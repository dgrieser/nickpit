package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
)

type VerifyRequest struct {
	ReviewCtx *model.ReviewContext
	Finding   model.Finding
	// StyleGuides, when non-nil, are reused instead of being recomputed from
	// ReviewCtx. VerifyAll resolves them once and shares them across findings so
	// the style-guide probe files are not stat+read once per finding.
	StyleGuides              []model.StyleGuide
	RepoRoot                 string
	Section                  *logging.ReasoningSection
	Progress                 logging.ProgressInfo
	UseJSONSchema            bool
	MaxToolCalls             int
	MaxDuplicateToolCalls    int
	MaxOutputRetries         int
	MaxReasoningSeconds      int
	MaxReasoningLoopRepeats  int
	DisableParallelToolCalls bool
}

type VerifyOptions struct {
	// Limiter admits verify agent calls; it is shared across the whole pipeline
	// run so the concurrency cap is global, not per call. A nil limiter is
	// unlimited.
	Limiter *VerifyLimiter
	// ReviewerName labels progress output when verifying a single reviewer's
	// findings (per-vector lane steps); empty for the global verify step.
	ReviewerName             string
	UseJSONSchema            bool
	MaxToolCalls             int
	MaxDuplicateToolCalls    int
	MaxOutputRetries         int
	MaxReasoningSeconds      int
	MaxReasoningLoopRepeats  int
	DisableParallelToolCalls bool
	RepoRoot                 string
	DropPolicy               string
	DropConfidence           float64
}

func (e *Engine) Verify(ctx context.Context, req VerifyRequest) (*model.FindingVerification, model.TokenUsage, error) {
	usage := model.TokenUsage{}
	if req.ReviewCtx == nil {
		return nil, usage, fmt.Errorf("verify: nil review context")
	}
	if model.EnsureFindingID(&req.Finding) {
		e.logf(ctx, "Verify generated replacement ID for invalid finding ID: title=%q", req.Finding.Title)
	}

	systemTemplate, err := e.loadPrompt("agent_verify_system_prompt.tmpl")
	if err != nil {
		return nil, usage, err
	}
	systemSnippet := verifyOutputSchemaSnippetFor(req.UseJSONSchema)
	exampleSnippet := llm.VerifyExamplePromptSnippet()
	agentKind := "verify"
	toolInstructions, err := e.renderToolInstructions(toolInstructionsConfig{
		agentRole:                agentKind,
		parallelToolCallGuidance: !req.DisableParallelToolCalls,
	})
	if err != nil {
		return nil, usage, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("verify", systemSnippet)
	if err != nil {
		return nil, usage, err
	}
	styleGuides := req.StyleGuides
	if styleGuides == nil {
		styleGuides, err = e.styleGuidesFor(req.ReviewCtx)
		if err != nil {
			return nil, usage, err
		}
	}
	styleGuideToolchainSnippet, err := e.renderStyleGuideToolchainSnippet(agentKind, styleGuides, len(req.ReviewCtx.ToolchainVersions) > 0)
	if err != nil {
		return nil, usage, err
	}
	systemPrompt, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet        string
		OutputFormatSnippet        string
		PrioritySnippet            string
		ParallelToolCallGuidance   bool
		HasTools                   bool
		ToolInstructions           string
		StyleGuideToolchainSnippet string
	}{
		OutputSchemaSnippet:        systemSnippet,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		PrioritySnippet:            commonSnippets.priority,
		ParallelToolCallGuidance:   !req.DisableParallelToolCalls,
		HasTools:                   true,
		ToolInstructions:           toolInstructions,
		StyleGuideToolchainSnippet: styleGuideToolchainSnippet,
	})
	if err != nil {
		return nil, usage, fmt.Errorf("verify: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildVerifyUserPrompt(req.ReviewCtx, req.Finding, styleGuides)
	if err != nil {
		return nil, usage, err
	}

	var schema []byte
	if req.UseJSONSchema {
		schema = llm.VerifySchema
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	progress := req.Progress
	if progress.IsZero() {
		progress = e.progressInfo(agentKind, "Verify Findings", "")
	}
	for attempt := 0; ; attempt++ {
		loopResult, err := e.runAgentLoop(ctx, agentLoopRequest{
			AgentName:                         "Verify Findings",
			AgentKind:                         agentKind,
			Progress:                          progress,
			Messages:                          messages,
			Tools:                             reviewerToolDefinitions(),
			Schema:                            schema,
			SchemaKind:                        llm.SchemaKindVerify,
			Model:                             e.config.Model,
			MaxTokens:                         e.config.MaxTokens,
			Temperature:                       e.config.Temperature,
			TopP:                              e.config.TopP,
			ExtraBody:                         e.config.ExtraBody,
			ParallelToolCalls:                 !req.DisableParallelToolCalls,
			ReasoningEffort:                   e.config.ReasoningEffort,
			RepoRoot:                          req.RepoRoot,
			MaxToolCalls:                      req.MaxToolCalls,
			MaxDuplicateToolCalls:             req.MaxDuplicateToolCalls,
			MaxOutputRetries:                  req.MaxOutputRetries,
			MaxReasoningSeconds:               req.MaxReasoningSeconds,
			MaxReasoningLoopRepeats:           req.MaxReasoningLoopRepeats,
			Section:                           req.Section,
			NoToolsSystem:                     systemTemplate,
			NoToolsSchemaSnippet:              systemSnippet,
			NoToolsStyleGuideToolchainSnippet: styleGuideToolchainSnippet,
			JSONRetryExampleSnippet:           exampleSnippet,
			NoToolsMessages: func(messages []llm.Message) ([]llm.Message, error) {
				return noToolsMessages(agentKind, systemTemplate, messages, systemSnippet, styleGuideToolchainSnippet)
			},
		})
		if err != nil {
			return nil, usage, err
		}
		usage = addTokenUsage(usage, loopResult.tokensUsed)
		resp := loopResult.resp
		if resp != nil && resp.Verification != nil {
			model.EnsureVerificationID(resp.Verification, req.Finding.ID)
			return resp.Verification, usage, nil
		}
		if !outputRetriesRemaining(attempt, req.MaxOutputRetries) {
			return nil, usage, fmt.Errorf("verify: missing verification in response")
		}
		e.logf(ctx, "Verify: missing verification, retrying: attempt=%d", attempt+1)
		if len(loopResult.messages) > 0 {
			messages = loopResult.messages
		}
	}
}

func (e *Engine) VerifyAll(ctx context.Context, reviewCtx *model.ReviewContext, findings []model.Finding, opts VerifyOptions) ([]*model.FindingVerification, model.TokenUsage, []string, error) {
	findings = append([]model.Finding(nil), findings...)
	if overwrote := model.EnsureFindingIDs(findings); overwrote > 0 {
		e.logf(ctx, "Verify generated replacement IDs for invalid finding IDs: count=%d", overwrote)
	}
	verifications := make([]*model.FindingVerification, len(findings))
	if len(findings) == 0 {
		return verifications, model.TokenUsage{}, nil, nil
	}

	// Resolve style guides once: the result depends only on reviewCtx, which is
	// constant across findings, so this avoids re-reading the style-guide probe
	// files once per concurrent verifier. Normalize to a non-nil slice so Verify
	// treats it as "provided" even when the repo has no matching guides.
	sharedStyleGuides, err := e.styleGuidesFor(reviewCtx)
	if err != nil {
		return nil, model.TokenUsage{}, nil, err
	}
	if sharedStyleGuides == nil {
		sharedStyleGuides = []model.StyleGuide{}
	}

	var (
		mu       sync.Mutex
		usageSum model.TokenUsage
		warnings []string
		wg       sync.WaitGroup
	)
	verifyStart := time.Now()
	e.logProgress(logging.StageVerify, logging.StateStart, fmt.Sprintf("%sfindings=%d concurrency=%s", verifyReviewerPrefix(opts.ReviewerName), len(findings), verifyConcurrencyLabel(opts.Limiter)))
	for i, finding := range findings {
		// Admission goes through the run-shared limiter in the spawn loop so
		// this call's findings start in order; concurrent VerifyAll calls (one
		// per reviewer lane) block only their own loop and compete fairly for
		// slots.
		if err := opts.Limiter.Acquire(ctx); err != nil {
			mu.Lock()
			warnings = append(warnings, fmt.Sprintf("Verify failed for finding #%d %q: %v", i+1, finding.Title, err))
			mu.Unlock()
			continue
		}
		wg.Add(1)
		go func(idx int, f model.Finding) {
			defer wg.Done()
			defer opts.Limiter.Release()
			info := e.progressInfo("verify", verifyProgressName(opts.ReviewerName, idx), truncateFindingTitle(f.Title))
			sec := e.logger.NewReasoningTracker(info)
			defer sec.End()
			req := VerifyRequest{
				ReviewCtx:                reviewCtx,
				Finding:                  f,
				StyleGuides:              sharedStyleGuides,
				RepoRoot:                 opts.RepoRoot,
				Section:                  sec,
				Progress:                 info,
				UseJSONSchema:            opts.UseJSONSchema,
				MaxToolCalls:             opts.MaxToolCalls,
				MaxDuplicateToolCalls:    opts.MaxDuplicateToolCalls,
				MaxOutputRetries:         opts.MaxOutputRetries,
				MaxReasoningSeconds:      opts.MaxReasoningSeconds,
				MaxReasoningLoopRepeats:  opts.MaxReasoningLoopRepeats,
				DisableParallelToolCalls: opts.DisableParallelToolCalls,
			}
			verification, usage, err := e.Verify(ctx, req)
			mu.Lock()
			usageSum.PromptTokens += usage.PromptTokens
			usageSum.CompletionTokens += usage.CompletionTokens
			usageSum.TotalTokens += usage.TotalTokens
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Verify failed for finding #%d %q: %v", idx+1, f.Title, err))
			}
			mu.Unlock()
			if err != nil {
				e.logf(ctx, "Verify failed: index=%d title=%q error=%v", idx, f.Title, err)
				return
			}
			verifications[idx] = verification
		}(i, finding)
	}
	wg.Wait()
	e.logProgress(logging.StageVerify, logging.StateDone, fmt.Sprintf("%sfindings=%d prompt_tokens=%s completion_tokens=%s total_tokens=%s warnings=%d runtime=%s", verifyReviewerPrefix(opts.ReviewerName), len(findings), model.HumanTokens(usageSum.PromptTokens), model.HumanTokens(usageSum.CompletionTokens), model.HumanTokens(usageSum.TotalTokens), len(warnings), model.HumanDuration(time.Since(verifyStart))))
	return verifications, usageSum, warnings, nil
}

// verifyReviewerPrefix labels per-reviewer verify progress lines; the global
// verify step (no reviewer name) keeps its unprefixed format.
func verifyReviewerPrefix(reviewerName string) string {
	if reviewerName == "" {
		return ""
	}
	return fmt.Sprintf("reviewer=%q ", reviewerName)
}

// verifyConcurrencyLabel renders the run-global verify cap; 0 = unlimited.
func verifyConcurrencyLabel(l *VerifyLimiter) string {
	if limit := l.Limit(); limit > 0 {
		return fmt.Sprintf("%d", limit)
	}
	return "∞"
}

// verifyProgressName scopes a finding's progress name to its reviewer when
// verifying a single reviewer's findings, e.g. "Code Quality #2".
func verifyProgressName(reviewerName string, idx int) string {
	if reviewerName == "" {
		return fmt.Sprintf("#%d", idx+1)
	}
	return fmt.Sprintf("%s #%d", reviewerName, idx+1)
}

func truncateFindingTitle(title string) string {
	title = strings.TrimSpace(title)
	if len([]rune(title)) > 60 {
		title = string([]rune(title)[:57]) + "..."
	}
	return title
}

func verifyOutputSchemaSnippetFor(useJSONSchema bool) string {
	if useJSONSchema {
		return ""
	}
	return llm.VerifyExamplePromptSnippet()
}

func (e *Engine) buildVerifyUserPrompt(reviewCtx *model.ReviewContext, finding model.Finding, styleGuides []model.StyleGuide) (string, error) {
	payload := model.PromptPayloadFromContext(reviewCtx)
	payload.StyleGuides = styleGuides
	base, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("verify: marshalling review payload: %w", err)
	}
	var combined map[string]any
	if err := json.Unmarshal(base, &combined); err != nil {
		return "", fmt.Errorf("verify: re-decoding review payload: %w", err)
	}

	findingForVerify := struct {
		ID           string             `json:"id"`
		Title        string             `json:"title"`
		Body         string             `json:"body"`
		Priority     int                `json:"priority"`
		CodeLocation model.CodeLocation `json:"code_location"`
		Suggestions  []model.Suggestion `json:"suggestions,omitempty"`
	}{
		ID:           finding.ID,
		Title:        finding.Title,
		Body:         finding.Body,
		Priority:     model.PriorityRank(finding.Priority),
		CodeLocation: finding.CodeLocation,
		Suggestions:  finding.Suggestions,
	}
	encoded, err := json.Marshal(findingForVerify)
	if err != nil {
		return "", fmt.Errorf("verify: marshalling finding: %w", err)
	}
	var findingMap map[string]any
	if err := json.Unmarshal(encoded, &findingMap); err != nil {
		return "", fmt.Errorf("verify: re-decoding finding: %w", err)
	}
	combined["finding"] = findingMap

	out, err := json.MarshalIndent(combined, "", "  ")
	if err != nil {
		return "", fmt.Errorf("verify: encoding combined payload: %w", err)
	}
	return string(out), nil
}
