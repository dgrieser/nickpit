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
	StyleGuides               []model.StyleGuide
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

type VerifyOptions struct {
	// Limiter admits verify agent calls in spawn order; it is the same
	// run-global limiter that caps every LLM agent loop, so verify competes
	// fairly with all other steps. A nil limiter is unlimited.
	Limiter *Limiter
	// ReviewerName labels progress output when verifying a single reviewer's
	// findings (per-vector lane steps); empty for the global verify step.
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
	DropPolicy                string
	DiffFormat                model.DiffFormat
}

type verifyResult struct {
	Verification            *model.FindingVerification
	ReplacementCodeLocation *model.CodeLocation
}

func (e *Engine) Verify(ctx context.Context, req VerifyRequest) (*model.FindingVerification, model.TokenUsage, error) {
	result, usage, err := e.verifyFinding(ctx, req)
	if result == nil {
		return nil, usage, err
	}
	return result.Verification, usage, err
}

func (e *Engine) verifyFinding(ctx context.Context, req VerifyRequest) (*verifyResult, model.TokenUsage, error) {
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
	diffScopeEnabled := !req.DisableDiffScope
	systemSnippet := llm.VerifyExamplePromptSnippet()
	if diffScopeEnabled {
		systemSnippet = llm.ScopedVerifyExamplePromptSnippet()
	}
	agentKind := "verify"
	toolInstructions, err := e.renderToolInstructions(toolInstructionsConfig{
		agentRole:                agentKind,
		parallelToolCallGuidance: !req.DisableParallelToolCalls,
	})
	if err != nil {
		return nil, usage, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("verify", systemSnippet, req.DisableSuggestions)
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
		DiffScopeEnabled           bool
	}{
		OutputSchemaSnippet:        systemSnippet,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		PrioritySnippet:            commonSnippets.priority,
		ParallelToolCallGuidance:   !req.DisableParallelToolCalls,
		HasTools:                   true,
		ToolInstructions:           toolInstructions,
		StyleGuideToolchainSnippet: styleGuideToolchainSnippet,
		DiffScopeEnabled:           diffScopeEnabled,
	})
	if err != nil {
		return nil, usage, fmt.Errorf("verify: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildVerifyUserPrompt(req.ReviewCtx, req.Finding, req.DisableSuggestions, req.DisableDiffScope, req.DiffFormat)
	if err != nil {
		return nil, usage, err
	}

	var schema []byte
	if !req.DisableJSONResponseFormat {
		schema = llm.VerifySchema
		if diffScopeEnabled {
			schema = llm.ScopedVerifySchema
		}
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	progress := req.Progress
	if progress.IsZero() {
		progress = e.progressInfo(agentKind, "Verify Findings", "")
	}
	// One loop state is shared across the outer missing-verification attempts so
	// tool-dedup and retry budgets carry over instead of resetting per attempt
	// (a fresh state would let every retry re-fetch the same files).
	state := newAgentLoopState()
	for attempt := 0; ; attempt++ {
		loopResult, err := e.runAgentLoop(ctx, agentLoopRequest{
			AgentName:                         "Verify Findings",
			AgentKind:                         agentKind,
			Progress:                          progress,
			Messages:                          messages,
			Tools:                             reviewerToolDefinitions(),
			Schema:                            schema,
			SchemaKind:                        llm.SchemaKindVerify,
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
				return noToolsMessages(agentKind, systemTemplate, messages, systemSnippet, styleGuideToolchainSnippet, req.DisableSuggestions, noToolsPromptOptions{DiffScopeEnabled: diffScopeEnabled})
			},
		})
		if err != nil {
			return nil, usage, err
		}
		usage = addTokenUsage(usage, loopResult.tokensUsed)
		resp := loopResult.resp
		if resp != nil && resp.Verification != nil {
			model.EnsureVerificationID(resp.Verification, req.Finding.ID)
			return &verifyResult{
				Verification:            resp.Verification,
				ReplacementCodeLocation: resp.ReplacementCodeLocation,
			}, usage, nil
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
	results, usage, warnings, err := e.verifyAll(ctx, reviewCtx, findings, opts)
	verifications := make([]*model.FindingVerification, len(results))
	for i := range results {
		verifications[i] = results[i].Verification
	}
	return verifications, usage, warnings, err
}

func (e *Engine) verifyAll(ctx context.Context, reviewCtx *model.ReviewContext, findings []model.Finding, opts VerifyOptions) ([]verifyResult, model.TokenUsage, []string, error) {
	findings = append([]model.Finding(nil), findings...)
	if overwrote := model.EnsureFindingIDs(findings); overwrote > 0 {
		e.logf(ctx, "Verify generated replacement IDs for invalid finding IDs: count=%d", overwrote)
	}
	results := make([]verifyResult, len(findings))
	if len(findings) == 0 {
		return results, model.TokenUsage{}, nil, nil
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
		// slots. Acquire fails only when ctx is done, so one aggregate warning
		// replaces a per-finding flood and the loop stops.
		verifyCtx, release, err := opts.Limiter.Acquire(ctx)
		if err != nil {
			mu.Lock()
			warnings = append(warnings, fmt.Sprintf("Verify cancelled at finding #%d %q: %v; skipped %d remaining finding(s)", i+1, finding.Title, err, len(findings)-i))
			mu.Unlock()
			break
		}
		wg.Add(1)
		go func(idx int, f model.Finding, ctx context.Context, release func()) {
			defer wg.Done()
			defer release()
			info := e.progressInfo("verify", verifyProgressName(opts.ReviewerName, idx), truncateFindingTitle(f.Title))
			sec := e.logger.NewReasoningTracker(info)
			defer sec.End()
			req := VerifyRequest{
				ReviewCtx:                 reviewCtx,
				Finding:                   f,
				StyleGuides:               sharedStyleGuides,
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
			result, usage, err := e.verifyFinding(ctx, req)
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
			if result != nil {
				results[idx] = *result
			}
		}(i, finding, verifyCtx, release)
	}
	wg.Wait()
	for i := range results {
		if results[i].Verification == nil {
			results[i].Verification = fallbackUnverifiedVerification(findings[i])
		}
	}
	e.logProgress(logging.StageVerify, logging.StateDone, fmt.Sprintf("%sfindings=%d prompt_tokens=%s completion_tokens=%s total_tokens=%s warnings=%d runtime=%s", verifyReviewerPrefix(opts.ReviewerName), len(findings), model.HumanTokens(usageSum.PromptTokens), model.HumanTokens(usageSum.CompletionTokens), model.HumanTokens(usageSum.TotalTokens), len(warnings), model.HumanDuration(time.Since(verifyStart))))
	return results, usageSum, warnings, nil
}

func fallbackUnverifiedVerification(f model.Finding) *model.FindingVerification {
	v := &model.FindingVerification{
		ID:              f.ID,
		Verdict:         model.VerdictUnverified,
		Priority:        model.PriorityRank(f.Priority),
		ConfidenceScore: 0,
		Remarks:         "",
	}
	model.EnsureVerificationID(v, f.ID)
	return v
}

// verifyReviewerPrefix labels per-reviewer verify progress lines; the global
// verify step (no reviewer name) keeps its unprefixed format.
func verifyReviewerPrefix(reviewerName string) string {
	if reviewerName == "" {
		return ""
	}
	return fmt.Sprintf("reviewer=%q ", reviewerName)
}

// verifyConcurrencyLabel renders the run-global agent-loop cap; 0 = unlimited.
func verifyConcurrencyLabel(l *Limiter) string {
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

func (e *Engine) buildVerifyUserPrompt(reviewCtx *model.ReviewContext, finding model.Finding, disableSuggestions, disableDiffScope bool, format model.DiffFormat) (string, error) {
	payload := model.PromptPayloadFromContextWithDiffFormat(reviewCtx, format)
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
	}
	if !disableSuggestions {
		findingForVerify.Suggestions = finding.Suggestions
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
	if !disableDiffScope && reviewCtx.DiffScopeHunks != nil {
		status := "outside_diff"
		if codeLocationOverlapsDiff(finding.CodeLocation, reviewCtx.DiffScopeHunks) {
			status = "overlaps_diff"
		}
		combined["finding_diff_scope"] = status
	}

	out, err := json.MarshalIndent(combined, "", "  ")
	if err != nil {
		return "", fmt.Errorf("verify: encoding combined payload: %w", err)
	}
	return string(out), nil
}
