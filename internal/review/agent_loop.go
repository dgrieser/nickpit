package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
)

type agentLoopRequest struct {
	AgentName                         string
	AgentKind                         string
	Messages                          []llm.Message
	Tools                             []llm.ToolDefinition
	Schema                            []byte
	SchemaKind                        llm.SchemaKind
	Constraints                       llm.ResponseConstraints
	Model                             string
	MaxTokens                         *int
	Temperature                       *float64
	TopP                              *float64
	ExtraBody                         map[string]any
	ReasoningEffort                   string
	ReasoningSink                     llm.ReasoningSink
	RepoRoot                          string
	MaxToolCalls                      int
	MaxDuplicateToolCalls             int
	MaxOutputRetries                  int
	MaxReasoningSeconds               int
	MaxReasoningLoopRepeats           int
	ParallelToolCalls                 bool
	State                             *agentLoopState
	Section                           *logging.ReasoningSection
	NoToolsMessages                   func([]llm.Message) ([]llm.Message, error)
	NoToolsSystem                     string
	NoToolsSchemaSnippet              string
	NoToolsStyleGuideToolchainSnippet string
	JSONRetryExampleSnippet           string
	JSONRetryProgressAgentName        string
	OnReasoningTrace                  func(agentName string, iterIdx int, reasoning string)
}

type agentLoopResult struct {
	resp               *llm.ReviewResponse
	tokensUsed         model.TokenUsage
	reasoningEffort    string
	contentMessages    []string
	toolMessages       []llm.Message
	toolCallHistory    []toolCallHistoryEntry
	messages           []llm.Message
	toolCalls          int
	duplicateToolCalls int
}

type agentLoopState struct {
	toolState              *toolRoundState
	jsonRetries            int
	jsonRepairWithoutTools bool
	toolCalls              int
	duplicateToolCalls     int
	callNum                int
}

func newAgentLoopState() *agentLoopState {
	return &agentLoopState{
		toolState: &toolRoundState{
			seenFiles:      make(map[string]retrieval.FileContent),
			seenFileRanges: make(map[string][]model.LineRange),
			seenToolCalls:  make(map[string]struct{}),
		},
	}
}

func (e *Engine) runAgentLoop(ctx context.Context, req agentLoopRequest) (agentLoopResult, error) {
	llmReq := &llm.ReviewRequest{
		Messages:                req.Messages,
		Tools:                   append([]llm.ToolDefinition(nil), req.Tools...),
		Schema:                  req.Schema,
		SchemaKind:              req.SchemaKind,
		Constraints:             req.Constraints,
		Model:                   req.Model,
		MaxTokens:               req.MaxTokens,
		Temperature:             req.Temperature,
		TopP:                    req.TopP,
		ExtraBody:               req.ExtraBody,
		ParallelToolCalls:       req.ParallelToolCalls,
		ReasoningEffort:         req.ReasoningEffort,
		ReasoningSink:           req.ReasoningSink,
		MaxReasoning:            time.Duration(req.MaxReasoningSeconds) * time.Second,
		MaxReasoningLoopRepeats: req.MaxReasoningLoopRepeats,
	}

	messages := append([]llm.Message(nil), req.Messages...)
	result := agentLoopResult{reasoningEffort: req.ReasoningEffort}
	state := req.State
	if state == nil {
		state = newAgentLoopState()
	}
	var syntheticFollowup *llm.Message

	for {
		state.callNum++
		loopCtx := ctxWithAgent(ctx, agentTag{Role: req.AgentKind, Name: req.AgentName, Turn: state.callNum})
		loopCtx = llm.WithAgentLabel(loopCtx, agentLabelForLLM(loopCtx))
		noToolsHistory, err := agentLoopNoToolsMessages(req, messages)
		if err != nil {
			return result, err
		}
		llmReq.NoToolsMessages = noToolsHistory
		llmReq.Messages = messages
		if syntheticFollowup != nil {
			llmReq.Messages = append(append([]llm.Message(nil), messages...), *syntheticFollowup)
		}

		var perCallBuf *llm.BufferedReasoningSink
		if req.OnReasoningTrace != nil {
			perCallBuf = &llm.BufferedReasoningSink{}
			llmReq.ReasoningSink = llm.TeeReasoningSinks(req.ReasoningSink, perCallBuf)
		} else {
			llmReq.ReasoningSink = req.ReasoningSink
		}

		resp, err := e.loggedReview(loopCtx, llmReq, req.Section)
		if perCallBuf != nil && err == nil && resp != nil {
			if trace := strings.TrimSpace(perCallBuf.String()); trace != "" {
				req.OnReasoningTrace(req.AgentName, state.callNum, trace)
			}
		}
		if err != nil {
			var invalidResp *llm.InvalidResponseError
			if errors.As(err, &invalidResp) && outputRetriesRemaining(state.jsonRetries, req.MaxOutputRetries) {
				if invalidResp.ReasoningEffort != "" {
					result.reasoningEffort = invalidResp.ReasoningEffort
					llmReq.ReasoningEffort = invalidResp.ReasoningEffort
				}
				if invalidResp.ToolsOmitted || state.jsonRepairWithoutTools {
					state.jsonRepairWithoutTools = true
					messages = noToolsHistory
					llmReq.Tools = nil
					llmReq.ParallelToolCalls = false
				}
				state.jsonRetries++
				e.logJSONRetry(loopCtx, req, state.jsonRetries, invalidResp)
				if strings.TrimSpace(invalidResp.RawContent) != "" {
					messages = append(messages, llm.Message{Role: "assistant", Content: invalidResp.RawContent})
				}
				feedback, err := e.renderJSONRetryFeedback(invalidResp, req.JSONRetryExampleSnippet)
				if err != nil {
					return result, err
				}
				messages = append(messages, llm.Message{Role: "user", Content: feedback})
				syntheticFollowup = nil
				continue
			}
			return result, err
		}

		if resp.ReasoningEffort != "" {
			result.reasoningEffort = resp.ReasoningEffort
			llmReq.ReasoningEffort = resp.ReasoningEffort
		}
		result.tokensUsed = addTokenUsage(result.tokensUsed, resp.TokensUsed)
		result.contentMessages = appendResponseContent(result.contentMessages, resp)
		result.resp = resp

		originalToolCalls := len(resp.ToolCalls)
		resp.ToolCalls, _ = filterAgentToolCalls(resp.ToolCalls, req.Tools)
		if originalToolCalls > 0 && len(resp.ToolCalls) == 0 {
			if outputRetriesRemaining(state.jsonRetries, req.MaxOutputRetries) {
				state.jsonRetries++
				e.logfCtx(loopCtx, "Invalid tool call response, retrying without tool history: attempt=%d", state.jsonRetries)
				if strings.TrimSpace(resp.RawResponse) != "" {
					messages = append(messages, llm.Message{Role: "assistant", Content: resp.RawResponse})
				}
				continue
			}
			return result, fmt.Errorf("agent %s returned only invalid tool calls", req.AgentName)
		}
		if len(resp.ToolCalls) == 0 {
			if strings.TrimSpace(resp.RawResponse) != "" {
				messages = append(messages, llm.Message{Role: "assistant", Content: resp.RawResponse})
			}
			break
		}
		pendingToolCalls := len(resp.ToolCalls)
		if req.MaxToolCalls > 0 && state.toolCalls+pendingToolCalls > req.MaxToolCalls {
			e.logfCtx(loopCtx, "Tool call limit reached, making final call without tools: limit=%d used=%d requested=%d", req.MaxToolCalls, state.toolCalls, pendingToolCalls)
			finalMessages := append([]llm.Message(nil), messages...)
			if strings.TrimSpace(resp.RawResponse) != "" {
				finalMessages = append(finalMessages, llm.Message{Role: "assistant", Content: resp.RawResponse})
			}
			resp, err = e.agentLoopReviewWithoutTools(loopCtx, llmReq, req, finalMessages)
			if err != nil {
				return result, err
			}
			result.tokensUsed = addTokenUsage(result.tokensUsed, resp.TokensUsed)
			result.contentMessages = appendResponseContent(result.contentMessages, resp)
			result.resp = resp
			break
		}

		e.logfCtx(loopCtx, "Executing tool batch: used=%d requested=%d", state.toolCalls, pendingToolCalls)
		messages = append(messages, llm.Message{Role: "assistant", Content: resp.RawResponse, ToolCalls: resp.ToolCalls})
		batch := e.executeToolCalls(loopCtx, req.RepoRoot, resp.ToolCalls, state.toolState)
		messages = append(messages, batch...)
		result.toolMessages = append(result.toolMessages, batch...)
		result.toolCallHistory = append(result.toolCallHistory, collectToolCallHistory(resp.ToolCalls, batch)...)
		duplicates := countDuplicateToolCalls(batch)
		result.duplicateToolCalls += duplicates
		state.duplicateToolCalls += duplicates
		result.toolCalls += pendingToolCalls
		state.toolCalls += pendingToolCalls
		if req.MaxDuplicateToolCalls > 0 && state.duplicateToolCalls >= req.MaxDuplicateToolCalls {
			e.logfCtx(loopCtx, "Duplicate tool call limit reached, making final call without tools: limit=%d duplicates=%d", req.MaxDuplicateToolCalls, state.duplicateToolCalls)
			resp, err = e.agentLoopReviewWithoutTools(loopCtx, llmReq, req, messages)
			if err != nil {
				return result, err
			}
			result.tokensUsed = addTokenUsage(result.tokensUsed, resp.TokensUsed)
			result.contentMessages = appendResponseContent(result.contentMessages, resp)
			result.resp = resp
			break
		}
		content, err := e.renderSyntheticToolFollowup(result.toolCallHistory, req.AgentKind)
		if err != nil {
			return result, err
		}
		syntheticFollowup = &llm.Message{Role: "user", Content: content}
	}

	if result.resp == nil {
		return result, fmt.Errorf("agent %s returned no response", req.AgentName)
	}
	result.messages = append([]llm.Message(nil), messages...)
	return result, nil
}

func outputRetriesRemaining(used, max int) bool {
	return max == 0 || used < max
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
	return e.reviewWithoutTools(ctx, &noToolsReq, req.NoToolsSystem, messages, req.NoToolsSchemaSnippet, req.NoToolsStyleGuideToolchainSnippet, req.MaxOutputRetries, req.Section)
}

func (e *Engine) logJSONRetry(ctx context.Context, req agentLoopRequest, attempt int, invalidResp *llm.InvalidResponseError) {
	if req.JSONRetryProgressAgentName == "" {
		e.logfCtx(ctx, "Verify: invalid JSON, retrying: attempt=%d reason=%q missing=%v", attempt, invalidResp.Reason, invalidResp.MissingFields)
		return
	}
	e.logfCtx(ctx, "Invalid JSON response, retrying with feedback: attempt=%d reason=%q missing=%v", attempt, invalidResp.Reason, invalidResp.MissingFields)
	e.logProgress("Model", fmt.Sprintf("status=InvalidJsonRetry, agent=%s, attempt=%d", req.JSONRetryProgressAgentName, attempt))
}

func validAgentToolCalls(toolCalls []llm.ToolCall, tools []llm.ToolDefinition) []llm.ToolCall {
	valid, _ := filterAgentToolCalls(toolCalls, tools)
	return valid
}

func filterAgentToolCalls(toolCalls []llm.ToolCall, tools []llm.ToolDefinition) ([]llm.ToolCall, int) {
	if len(toolCalls) == 0 {
		return nil, 0
	}
	knownTools := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		if tool.Name != "" {
			knownTools[tool.Name] = struct{}{}
		}
	}
	valid := make([]llm.ToolCall, 0, len(toolCalls))
	dropped := 0
	for _, toolCall := range toolCalls {
		if validAgentToolCall(toolCall, knownTools) {
			toolCall.Arguments, _ = llm.NormalizeToolCallArguments(toolCall.Arguments)
			valid = append(valid, toolCall)
		} else {
			dropped++
		}
	}
	return valid, dropped
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
