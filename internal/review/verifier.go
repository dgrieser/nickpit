package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
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
	DisableParallelToolCalls bool
}

type VerifyOptions struct {
	Concurrency              int
	UseJSONSchema            bool
	MaxToolCalls             int
	MaxDuplicateToolCalls    int
	DisableParallelToolCalls bool
	RepoRoot                 string
}

func (e *Engine) Verify(ctx context.Context, req VerifyRequest) (*model.FindingVerification, model.TokenUsage, error) {
	usage := model.TokenUsage{}
	if req.ReviewCtx == nil {
		return nil, usage, fmt.Errorf("verify: nil review context")
	}

	systemTemplate, err := e.loadPrompt("verify_system.tmpl")
	if err != nil {
		return nil, usage, err
	}
	systemSnippet := verifyOutputSchemaSnippetFor(req.UseJSONSchema)
	exampleSnippet := llm.VerifyExamplePromptSnippet()
	systemPrompt, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet      string
		ParallelToolCallGuidance bool
		HasTools                 bool
	}{
		OutputSchemaSnippet:      systemSnippet,
		ParallelToolCallGuidance: !req.DisableParallelToolCalls,
		HasTools:                 true,
	})
	if err != nil {
		return nil, usage, fmt.Errorf("verify: rendering system prompt: %w", err)
	}

	userPrompt, err := buildVerifyUserPrompt(req.ReviewCtx, req.Finding)
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
	llmReq := &llm.ReviewRequest{
		Messages:          messages,
		Tools:             reviewerToolDefinitions(),
		Schema:            schema,
		SchemaKind:        llm.SchemaKindVerify,
		Model:             e.config.Model,
		MaxTokens:         e.config.MaxTokens,
		Temperature:       e.config.Temperature,
		TopP:              e.config.TopP,
		ExtraBody:         e.config.ExtraBody,
		ParallelToolCalls: !req.DisableParallelToolCalls,
		ReasoningEffort:   e.config.ReasoningEffort,
	}

	toolState := &toolRoundState{
		seenFiles:      make(map[string]retrieval.FileContent),
		seenFileRanges: make(map[string][]model.LineRange),
		seenToolCalls:  make(map[string]struct{}),
	}
	toolCallsUsed := 0
	duplicateToolCallsUsed := 0
	var resp *llm.ReviewResponse
	var syntheticFollowup *llm.Message
	var toolCallHistory []toolCallHistoryEntry
	jsonRetries := 0
	jsonRepairWithoutTools := false

	for {
		noToolsHistory, err := noToolsMessages(systemTemplate, messages, systemSnippet)
		if err != nil {
			return nil, usage, err
		}
		llmReq.NoToolsMessages = noToolsHistory
		llmReq.Messages = messages
		if syntheticFollowup != nil {
			llmReq.Messages = append(append([]llm.Message(nil), messages...), *syntheticFollowup)
		}
		resp, err = e.loggedReview(ctx, llmReq, req.Section)
		if err != nil {
			var invalidResp *llm.InvalidResponseError
			if errors.As(err, &invalidResp) && jsonRetries < defaultMaxJSONRetries {
				if invalidResp.ToolsOmitted || jsonRepairWithoutTools {
					jsonRepairWithoutTools = true
					messages, err = noToolsMessages(systemTemplate, messages, systemSnippet)
					if err != nil {
						return nil, usage, err
					}
					llmReq.Tools = nil
					llmReq.ParallelToolCalls = false
				}
				jsonRetries++
				e.logf("Verify: invalid JSON, retrying: attempt=%d reason=%q missing=%v", jsonRetries, invalidResp.Reason, invalidResp.MissingFields)
				if strings.TrimSpace(invalidResp.RawContent) != "" {
					messages = append(messages, llm.Message{Role: "assistant", Content: invalidResp.RawContent})
				}
				messages = append(messages, llm.Message{Role: "user", Content: buildJSONRetryFeedback(invalidResp, exampleSnippet)})
				syntheticFollowup = nil
				continue
			}
			return nil, usage, err
		}
		usage.PromptTokens += resp.TokensUsed.PromptTokens
		usage.CompletionTokens += resp.TokensUsed.CompletionTokens
		usage.TotalTokens += resp.TokensUsed.TotalTokens

		if len(resp.ToolCalls) == 0 {
			break
		}
		pendingToolCalls := len(resp.ToolCalls)
		if req.MaxToolCalls > 0 && toolCallsUsed+pendingToolCalls > req.MaxToolCalls {
			finalMessages := append([]llm.Message(nil), messages...)
			if strings.TrimSpace(resp.RawResponse) != "" {
				finalMessages = append(finalMessages, llm.Message{Role: "assistant", Content: resp.RawResponse})
			}
			resp, err = e.reviewWithoutTools(ctx, llmReq, systemTemplate, finalMessages, systemSnippet, req.Section)
			if err != nil {
				return nil, usage, err
			}
			usage.PromptTokens += resp.TokensUsed.PromptTokens
			usage.CompletionTokens += resp.TokensUsed.CompletionTokens
			usage.TotalTokens += resp.TokensUsed.TotalTokens
			break
		}
		assistantMessage := llm.Message{Role: "assistant", Content: resp.RawResponse, ToolCalls: resp.ToolCalls}
		messages = append(messages, assistantMessage)
		toolMessages := e.executeToolCalls(ctx, req.RepoRoot, resp.ToolCalls, toolState)
		messages = append(messages, toolMessages...)
		toolCallHistory = append(toolCallHistory, collectToolCallHistory(resp.ToolCalls, toolMessages)...)
		duplicateToolCallsUsed += countDuplicateToolCalls(toolMessages)
		if req.MaxDuplicateToolCalls > 0 && duplicateToolCallsUsed >= req.MaxDuplicateToolCalls {
			toolCallsUsed += pendingToolCalls
			resp, err = e.reviewWithoutTools(ctx, llmReq, systemTemplate, messages, systemSnippet, req.Section)
			if err != nil {
				return nil, usage, err
			}
			usage.PromptTokens += resp.TokensUsed.PromptTokens
			usage.CompletionTokens += resp.TokensUsed.CompletionTokens
			usage.TotalTokens += resp.TokensUsed.TotalTokens
			break
		}
		syntheticFollowup = &llm.Message{
			Role:    "user",
			Content: syntheticToolFollowup(toolCallHistory),
		}
		toolCallsUsed += pendingToolCalls
	}

	if resp == nil || resp.Verification == nil {
		return nil, usage, fmt.Errorf("verify: missing verification in response")
	}
	return resp.Verification, usage, nil
}

func (e *Engine) VerifyAll(ctx context.Context, reviewCtx *model.ReviewContext, findings []model.Finding, opts VerifyOptions) ([]model.FindingVerification, model.TokenUsage, error) {
	verifications := make([]model.FindingVerification, len(findings))
	if len(findings) == 0 {
		return verifications, model.TokenUsage{}, nil
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
				DisableParallelToolCalls: opts.DisableParallelToolCalls,
			}
			verification, usage, err := e.Verify(ctx, req)
			mu.Lock()
			usageSum.PromptTokens += usage.PromptTokens
			usageSum.CompletionTokens += usage.CompletionTokens
			usageSum.TotalTokens += usage.TotalTokens
			mu.Unlock()
			if err != nil {
				e.logf("Verify failed: index=%d title=%q error=%v", idx, f.Title, err)
				verifications[idx] = model.FindingVerification{
					Valid:           true,
					Priority:        model.PriorityRank(f.Priority),
					ConfidenceScore: 0,
					Remarks:         fmt.Sprintf("verification failed: %v", err),
				}
				return
			}
			verifications[idx] = *verification
		}(i, finding)
	}
	wg.Wait()
	e.logProgress("Verify", fmt.Sprintf("done findings=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d", len(findings), usageSum.PromptTokens, usageSum.CompletionTokens, usageSum.TotalTokens))
	return verifications, usageSum, nil
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

func buildVerifyUserPrompt(reviewCtx *model.ReviewContext, finding model.Finding) (string, error) {
	payload := model.PromptPayloadFromContext(reviewCtx)
	base, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("verify: marshalling review payload: %w", err)
	}
	var combined map[string]any
	if err := json.Unmarshal(base, &combined); err != nil {
		return "", fmt.Errorf("verify: re-decoding review payload: %w", err)
	}

	findingForVerify := struct {
		Title        string             `json:"title"`
		Body         string             `json:"body"`
		Priority     int                `json:"priority"`
		CodeLocation model.CodeLocation `json:"code_location"`
		Suggestion   *model.Suggestion  `json:"suggestion,omitempty"`
	}{
		Title:        finding.Title,
		Body:         finding.Body,
		Priority:     model.PriorityRank(finding.Priority),
		CodeLocation: finding.CodeLocation,
		Suggestion:   finding.Suggestion,
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
