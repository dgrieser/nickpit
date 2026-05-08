package review

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
)

type agentLoopRequest struct {
	AgentName                  string
	AgentKind                  string
	Messages                   []llm.Message
	Tools                      []llm.ToolDefinition
	Schema                     []byte
	SchemaKind                 llm.SchemaKind
	Model                      string
	MaxTokens                  *int
	Temperature                *float64
	TopP                       *float64
	ExtraBody                  map[string]any
	ReasoningEffort            string
	RepoRoot                   string
	MaxToolCalls               int
	MaxDuplicateToolCalls      int
	ParallelToolCalls          bool
	Section                    *logging.ReasoningSection
	NoToolsMessages            func([]llm.Message) ([]llm.Message, error)
	NoToolsSystem              string
	NoToolsSchemaSnippet       string
	JSONRetryExampleSnippet    string
	JSONRetryProgressAgentName string
}

type agentLoopResult struct {
	resp               *llm.ReviewResponse
	tokensUsed         model.TokenUsage
	reasoningEffort    string
	contentMessages    []string
	toolMessages       []llm.Message
	toolCallHistory    []toolCallHistoryEntry
	toolCalls          int
	duplicateToolCalls int
}

func (e *Engine) runAgentLoop(ctx context.Context, req agentLoopRequest) (agentLoopResult, error) {
	llmReq := &llm.ReviewRequest{
		Messages:          req.Messages,
		Tools:             append([]llm.ToolDefinition(nil), req.Tools...),
		Schema:            req.Schema,
		SchemaKind:        req.SchemaKind,
		Model:             req.Model,
		MaxTokens:         req.MaxTokens,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		ExtraBody:         req.ExtraBody,
		ParallelToolCalls: req.ParallelToolCalls,
		ReasoningEffort:   req.ReasoningEffort,
	}

	messages := append([]llm.Message(nil), req.Messages...)
	result := agentLoopResult{reasoningEffort: req.ReasoningEffort}
	toolState := &toolRoundState{
		seenFiles:      make(map[string]retrieval.FileContent),
		seenFileRanges: make(map[string][]model.LineRange),
		seenToolCalls:  make(map[string]struct{}),
	}
	var syntheticFollowup *llm.Message
	jsonRetries := 0
	jsonRepairWithoutTools := false

	for {
		noToolsHistory, err := agentLoopNoToolsMessages(req, messages)
		if err != nil {
			return agentLoopResult{}, err
		}
		llmReq.NoToolsMessages = noToolsHistory
		llmReq.Messages = messages
		if syntheticFollowup != nil {
			llmReq.Messages = append(append([]llm.Message(nil), messages...), *syntheticFollowup)
		}

		resp, err := e.loggedReview(ctx, llmReq, req.Section)
		if err != nil {
			var invalidResp *llm.InvalidResponseError
			if errors.As(err, &invalidResp) && jsonRetries < defaultMaxJSONRetries {
				if invalidResp.ReasoningEffort != "" {
					result.reasoningEffort = invalidResp.ReasoningEffort
					llmReq.ReasoningEffort = invalidResp.ReasoningEffort
				}
				if invalidResp.ToolsOmitted || jsonRepairWithoutTools {
					jsonRepairWithoutTools = true
					messages = noToolsHistory
					llmReq.Tools = nil
					llmReq.ParallelToolCalls = false
				}
				jsonRetries++
				e.logJSONRetry(req, jsonRetries, invalidResp)
				if strings.TrimSpace(invalidResp.RawContent) != "" {
					messages = append(messages, llm.Message{Role: "assistant", Content: invalidResp.RawContent})
				}
				feedback, err := e.renderJSONRetryFeedback(invalidResp, req.JSONRetryExampleSnippet)
				if err != nil {
					return agentLoopResult{}, err
				}
				messages = append(messages, llm.Message{Role: "user", Content: feedback})
				syntheticFollowup = nil
				continue
			}
			return agentLoopResult{}, err
		}

		if resp.ReasoningEffort != "" {
			result.reasoningEffort = resp.ReasoningEffort
			llmReq.ReasoningEffort = resp.ReasoningEffort
		}
		result.tokensUsed = addTokenUsage(result.tokensUsed, resp.TokensUsed)
		result.contentMessages = appendResponseContent(result.contentMessages, resp)
		result.resp = resp

		resp.ToolCalls = validAgentToolCalls(resp.ToolCalls, req.Tools)
		if len(resp.ToolCalls) == 0 {
			break
		}
		pendingToolCalls := len(resp.ToolCalls)
		if req.MaxToolCalls > 0 && result.toolCalls+pendingToolCalls > req.MaxToolCalls {
			e.logf("Tool call limit reached, making final call without tools: agent=%s limit=%d used=%d requested=%d", req.AgentName, req.MaxToolCalls, result.toolCalls, pendingToolCalls)
			finalMessages := append([]llm.Message(nil), messages...)
			if strings.TrimSpace(resp.RawResponse) != "" {
				finalMessages = append(finalMessages, llm.Message{Role: "assistant", Content: resp.RawResponse})
			}
			resp, err = e.agentLoopReviewWithoutTools(ctx, llmReq, req, finalMessages)
			if err != nil {
				return agentLoopResult{}, err
			}
			result.tokensUsed = addTokenUsage(result.tokensUsed, resp.TokensUsed)
			result.contentMessages = appendResponseContent(result.contentMessages, resp)
			result.resp = resp
			break
		}

		e.logf("Executing tool batch: agent=%s used=%d requested=%d", req.AgentName, result.toolCalls, pendingToolCalls)
		messages = append(messages, llm.Message{Role: "assistant", Content: resp.RawResponse, ToolCalls: resp.ToolCalls})
		batch := e.executeToolCalls(ctx, req.RepoRoot, resp.ToolCalls, toolState)
		messages = append(messages, batch...)
		result.toolMessages = append(result.toolMessages, batch...)
		result.toolCallHistory = append(result.toolCallHistory, collectToolCallHistory(resp.ToolCalls, batch)...)
		result.duplicateToolCalls += countDuplicateToolCalls(batch)
		result.toolCalls += pendingToolCalls
		if req.MaxDuplicateToolCalls > 0 && result.duplicateToolCalls >= req.MaxDuplicateToolCalls {
			e.logf("Duplicate tool call limit reached, making final call without tools: agent=%s limit=%d duplicates=%d", req.AgentName, req.MaxDuplicateToolCalls, result.duplicateToolCalls)
			resp, err = e.agentLoopReviewWithoutTools(ctx, llmReq, req, messages)
			if err != nil {
				return agentLoopResult{}, err
			}
			result.tokensUsed = addTokenUsage(result.tokensUsed, resp.TokensUsed)
			result.contentMessages = appendResponseContent(result.contentMessages, resp)
			result.resp = resp
			break
		}
		content, err := e.renderSyntheticToolFollowup(result.toolCallHistory, req.AgentKind)
		if err != nil {
			return agentLoopResult{}, err
		}
		syntheticFollowup = &llm.Message{Role: "user", Content: content}
	}

	if result.resp == nil {
		return agentLoopResult{}, fmt.Errorf("agent %s returned no response", req.AgentName)
	}
	return result, nil
}

func agentLoopNoToolsMessages(req agentLoopRequest, messages []llm.Message) ([]llm.Message, error) {
	if req.NoToolsMessages == nil {
		return append([]llm.Message(nil), messages...), nil
	}
	return req.NoToolsMessages(messages)
}

func (e *Engine) agentLoopReviewWithoutTools(ctx context.Context, llmReq *llm.ReviewRequest, req agentLoopRequest, messages []llm.Message) (*llm.ReviewResponse, error) {
	noToolsReq := *llmReq
	noToolsReq.Tools = nil
	noToolsReq.ParallelToolCalls = false
	if req.NoToolsMessages != nil {
		finalMessages, err := req.NoToolsMessages(messages)
		if err != nil {
			return nil, err
		}
		noToolsReq.Messages = finalMessages
	}
	return e.reviewWithoutTools(ctx, &noToolsReq, req.NoToolsSystem, messages, req.NoToolsSchemaSnippet, req.Section)
}

func (e *Engine) logJSONRetry(req agentLoopRequest, attempt int, invalidResp *llm.InvalidResponseError) {
	if req.JSONRetryProgressAgentName == "" {
		e.logf("Verify: invalid JSON, retrying: attempt=%d reason=%q missing=%v", attempt, invalidResp.Reason, invalidResp.MissingFields)
		return
	}
	e.logf("Invalid JSON response, retrying with feedback: agent=%s attempt=%d reason=%q missing=%v", req.JSONRetryProgressAgentName, attempt, invalidResp.Reason, invalidResp.MissingFields)
	e.logProgress("Model", fmt.Sprintf("status=InvalidJsonRetry, agent=%s, attempt=%d", req.JSONRetryProgressAgentName, attempt))
}

func validAgentToolCalls(toolCalls []llm.ToolCall, tools []llm.ToolDefinition) []llm.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	knownTools := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		if tool.Name != "" {
			knownTools[tool.Name] = struct{}{}
		}
	}
	valid := make([]llm.ToolCall, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		if validAgentToolCall(toolCall, knownTools) {
			toolCall.Arguments, _ = llm.NormalizeToolCallArguments(toolCall.Arguments)
			valid = append(valid, toolCall)
		}
	}
	return valid
}

func validAgentToolCall(toolCall llm.ToolCall, knownTools map[string]struct{}) bool {
	if strings.TrimSpace(toolCall.ID) == "" || strings.TrimSpace(toolCall.Name) == "" {
		return false
	}
	if _, ok := knownTools[toolCall.Name]; !ok {
		return false
	}
	var args map[string]any
	if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
		return false
	}
	switch toolCall.Name {
	case "inspect_file":
		return nonEmptyStringArg(args, "path")
	case "search":
		return nonEmptyStringArg(args, "query")
	case "find_callers", "find_callees":
		return nonEmptyStringArg(args, "symbol")
	default:
		return true
	}
}

func nonEmptyStringArg(args map[string]any, key string) bool {
	value, ok := args[key].(string)
	return ok && strings.TrimSpace(value) != ""
}
