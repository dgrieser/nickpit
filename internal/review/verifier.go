package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
)

const defaultVerifyConcurrency = 4

type VerifyRequest struct {
	ReviewCtx                *model.ReviewContext
	Finding                  model.Finding
	RepoRoot                 string
	Section                  *logging.ReasoningSection
	UseJSONSchema            bool
	MaxToolCalls             int
	MaxDuplicateToolCalls    int
	MaxOutputRetries         int
	MaxReasoningSeconds      int
	MaxReasoningLoopRepeats  int
	DisableParallelToolCalls bool
}

type VerifyOptions struct {
	Concurrency              int
	UseJSONSchema            bool
	MaxToolCalls             int
	MaxDuplicateToolCalls    int
	MaxOutputRetries         int
	MaxReasoningSeconds      int
	MaxReasoningLoopRepeats  int
	DisableParallelToolCalls bool
	RepoRoot                 string
}

func (e *Engine) Verify(ctx context.Context, req VerifyRequest) (*model.FindingVerification, model.TokenUsage, error) {
	usage := model.TokenUsage{}
	if req.ReviewCtx == nil {
		return nil, usage, fmt.Errorf("verify: nil review context")
	}
	if model.EnsureFindingID(&req.Finding) {
		e.logf("Verify generated replacement ID for invalid finding ID: title=%q", req.Finding.Title)
	}

	systemTemplate, err := e.loadPrompt("agent_verify_system_prompt.tmpl")
	if err != nil {
		return nil, usage, err
	}
	systemSnippet := verifyOutputSchemaSnippetFor(req.UseJSONSchema)
	exampleSnippet := llm.VerifyExamplePromptSnippet()
	toolInstructions, err := e.renderToolInstructions(toolInstructionsConfig{
		kind:                     "verify",
		parallelToolCallGuidance: !req.DisableParallelToolCalls,
	})
	if err != nil {
		return nil, usage, err
	}
	prioritySnippet, err := agentCommonSystemPromptSnippet("verify", "priority", "")
	if err != nil {
		return nil, usage, err
	}
	systemPrompt, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet      string
		PrioritySnippet          string
		ParallelToolCallGuidance bool
		HasTools                 bool
		ToolInstructions         string
	}{
		OutputSchemaSnippet:      systemSnippet,
		PrioritySnippet:          prioritySnippet,
		ParallelToolCallGuidance: !req.DisableParallelToolCalls,
		HasTools:                 true,
		ToolInstructions:         toolInstructions,
	})
	if err != nil {
		return nil, usage, fmt.Errorf("verify: rendering system prompt: %w", err)
	}

	userPrompt, err := e.buildVerifyUserPrompt(req.ReviewCtx, req.Finding)
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

	for attempt := 0; ; attempt++ {
		loopResult, err := e.runAgentLoop(ctx, agentLoopRequest{
			AgentName:               "verify",
			AgentKind:               "verify",
			Messages:                messages,
			Tools:                   reviewerToolDefinitions(),
			Schema:                  schema,
			SchemaKind:              llm.SchemaKindVerify,
			Model:                   e.config.Model,
			MaxTokens:               e.config.MaxTokens,
			Temperature:             e.config.Temperature,
			TopP:                    e.config.TopP,
			ExtraBody:               e.config.ExtraBody,
			ParallelToolCalls:       !req.DisableParallelToolCalls,
			ReasoningEffort:         e.config.ReasoningEffort,
			RepoRoot:                req.RepoRoot,
			MaxToolCalls:            req.MaxToolCalls,
			MaxDuplicateToolCalls:   req.MaxDuplicateToolCalls,
			MaxOutputRetries:        req.MaxOutputRetries,
			MaxReasoningSeconds:     req.MaxReasoningSeconds,
			MaxReasoningLoopRepeats: req.MaxReasoningLoopRepeats,
			Section:                 req.Section,
			NoToolsSystem:           systemTemplate,
			NoToolsSchemaSnippet:    systemSnippet,
			JSONRetryExampleSnippet: exampleSnippet,
			NoToolsMessages: func(messages []llm.Message) ([]llm.Message, error) {
				return noToolsMessages(systemTemplate, messages, systemSnippet)
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
		e.logf("Verify: missing verification, retrying: attempt=%d", attempt+1)
		if len(loopResult.messages) > 0 {
			messages = loopResult.messages
		}
	}
}

func (e *Engine) VerifyAll(ctx context.Context, reviewCtx *model.ReviewContext, findings []model.Finding, opts VerifyOptions) ([]*model.FindingVerification, model.TokenUsage, []string, error) {
	findings = append([]model.Finding(nil), findings...)
	if overwrote := model.EnsureFindingIDs(findings); overwrote > 0 {
		e.logf("Verify generated replacement IDs for invalid finding IDs: count=%d", overwrote)
	}
	verifications := make([]*model.FindingVerification, len(findings))
	if len(findings) == 0 {
		return verifications, model.TokenUsage{}, nil, nil
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = defaultVerifyConcurrency
	}
	if concurrency > len(findings) {
		concurrency = len(findings)
	}

	var (
		mu        sync.Mutex
		usageSum  model.TokenUsage
		warnings  []string
		semaphore = make(chan struct{}, concurrency)
		wg        sync.WaitGroup
	)
	e.logProgress("Verify", fmt.Sprintf("findings=%d concurrency=%d", len(findings), concurrency))
	for i, finding := range findings {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(idx int, f model.Finding) {
			defer wg.Done()
			defer func() { <-semaphore }()
			sec := e.logger.NewReasoningTracker(labelForFinding(idx, f))
			defer sec.End()
			req := VerifyRequest{
				ReviewCtx:                reviewCtx,
				Finding:                  f,
				RepoRoot:                 opts.RepoRoot,
				Section:                  sec,
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
				e.logf("Verify failed: index=%d title=%q error=%v", idx, f.Title, err)
				return
			}
			verifications[idx] = verification
		}(i, finding)
	}
	wg.Wait()
	e.logProgress("Verify", fmt.Sprintf("done findings=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d warnings=%d", len(findings), usageSum.PromptTokens, usageSum.CompletionTokens, usageSum.TotalTokens, len(warnings)))
	return verifications, usageSum, warnings, nil
}

func labelForFinding(idx int, f model.Finding) string {
	title := strings.TrimSpace(f.Title)
	if title == "" {
		return fmt.Sprintf("verifier #%d", idx+1)
	}
	if len([]rune(title)) > 60 {
		title = string([]rune(title)[:57]) + "..."
	}
	return fmt.Sprintf("verifier #%d: %s", idx+1, title)
}

func verifyOutputSchemaSnippetFor(useJSONSchema bool) string {
	if useJSONSchema {
		return ""
	}
	return llm.VerifyExamplePromptSnippet()
}

func (e *Engine) buildVerifyUserPrompt(reviewCtx *model.ReviewContext, finding model.Finding) (string, error) {
	payload := model.PromptPayloadFromContext(reviewCtx)
	var err error
	payload.StyleGuides, err = e.styleGuidesFor(reviewCtx)
	if err != nil {
		return "", err
	}
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
