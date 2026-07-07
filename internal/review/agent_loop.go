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
	Progress                          logging.ProgressInfo
	Messages                          []llm.Message
	Tools                             []llm.ToolDefinition
	Schema                            []byte
	SchemaKind                        llm.SchemaKind
	Constraints                       llm.ResponseConstraints
	Model                             string
	MaxTokens                         *int
	Temperature                       *float64
	TopP                              *float64
	TopK                              *int
	PresencePenalty                   *float64
	ExtraBody                         map[string]any
	ReasoningEffort                   string
	ReasoningSink                     llm.ReasoningSink
	RepoRoot                          string
	MaxToolCalls                      int
	MaxDuplicateToolCalls             int
	MaxOutputRetries                  int
	MaxReasoningSeconds               int
	ParallelToolCalls                 bool
	State                             *agentLoopState
	Section                           *logging.ReasoningSection
	NoToolsMessages                   func([]llm.Message) ([]llm.Message, error)
	NoToolsSystem                     string
	NoToolsSchemaSnippet              string
	NoToolsStyleGuideToolchainSnippet string
	DisableSuggestions                bool
	JSONRetryExampleSnippet           string
	JSONRetryProgressAgentName        string
	OnReasoningTrace                  func(agentName string, iterIdx int, reasoning string)
	RepairResponse                    func(context.Context, *llm.ReviewResponse) codeLocationRepairResult
	ValidateResponse                  func(*llm.ReviewResponse) *llm.InvalidResponseError
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
	codeLocationRetried    bool
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
	// Admission through the run-global limiter caps concurrent LLM agent
	// loops across the whole pipeline. Chains admitted upstream (verify's
	// ordered spawn loop) carry the admission in ctx and pass through.
	ctx, release, err := LimiterFromContext(ctx).Acquire(ctx)
	if err != nil {
		return agentLoopResult{}, err
	}
	defer release()

	llmReq := &llm.ReviewRequest{
		Messages:          req.Messages,
		Tools:             append([]llm.ToolDefinition(nil), req.Tools...),
		Schema:            req.Schema,
		SchemaKind:        req.SchemaKind,
		Constraints:       req.Constraints,
		Model:             req.Model,
		MaxTokens:         req.MaxTokens,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		TopK:              req.TopK,
		PresencePenalty:   req.PresencePenalty,
		ExtraBody:         req.ExtraBody,
		ParallelToolCalls: req.ParallelToolCalls,
		ReasoningEffort:   req.ReasoningEffort,
		ReasoningSink:     req.ReasoningSink,
		MaxReasoning:      time.Duration(req.MaxReasoningSeconds) * time.Second,
	}

	messages := append([]llm.Message(nil), req.Messages...)
	result := agentLoopResult{reasoningEffort: req.ReasoningEffort}
	state := req.State
	if state == nil {
		state = newAgentLoopState()
	}
	var syntheticFollowup *llm.Message
	recordInvalidResponseTokens := func(invalidResp *llm.InvalidResponseError) {
		if invalidResp == nil {
			return
		}
		usage := invalidResp.TokensUsed
		if usage == (model.TokenUsage{}) && invalidResp.PartialResponse != nil {
			usage = invalidResp.PartialResponse.TokensUsed
		}
		result.tokensUsed = addTokenUsage(result.tokensUsed, usage)
	}

	for {
		state.callNum++
		loopCtx := logging.WithProgressInfo(ctx, req.Progress.WithTurn(state.callNum))
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
		repairedFromPartial := false
		if err != nil {
			var invalidResp *llm.InvalidResponseError
			if !errors.As(err, &invalidResp) {
				return result, err
			}
			if partialResp, retryInvalid, handled := e.tryRepairPartialResponse(loopCtx, req, invalidResp); handled {
				repairedFromPartial = true
				if retryInvalid != nil {
					queued, err := e.tryQueueCodeLocationRetry(loopCtx, req, state, retryInvalid, &messages, &syntheticFollowup, llmReq, true)
					if err != nil {
						return result, err
					}
					if queued {
						recordInvalidResponseTokens(invalidResp)
						continue
					}
					e.logf(loopCtx, "Code location repair needed retry but retry budget is exhausted; using partial parsed response: missing=%v", retryInvalid.MissingFields)
				}
				resp = partialResp
				err = nil
			}
			if err != nil && outputRetriesRemaining(state.jsonRetries, req.MaxOutputRetries) {
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
				} else {
					// Keep the history alternating for strict-role providers
					// when the invalid response carried no appendable content.
					messages = append(messages, llm.Message{Role: "assistant", Content: "[invalid response]"})
				}
				feedback, err := e.renderJSONRetryFeedback(invalidResp, req.JSONRetryExampleSnippet)
				if err != nil {
					return result, err
				}
				messages = append(messages, llm.Message{Role: "user", Content: feedback})
				syntheticFollowup = nil
				recordInvalidResponseTokens(invalidResp)
				continue
			}
			if err != nil && invalidResp.PartialResponse != nil {
				e.logf(loopCtx, "Invalid JSON response after retries exhausted; using partial parsed response: reason=%q missing=%v", invalidResp.Reason, invalidResp.MissingFields)
				resp = invalidResp.PartialResponse
			} else if err != nil {
				return result, err
			}
		}

		result.tokensUsed = addTokenUsage(result.tokensUsed, resp.TokensUsed)
		if !repairedFromPartial {
			if retryInvalid := e.repairResponseOrRetry(loopCtx, req, resp); retryInvalid != nil {
				queued, err := e.tryQueueCodeLocationRetry(loopCtx, req, state, retryInvalid, &messages, &syntheticFollowup, llmReq, true)
				if err != nil {
					return result, err
				}
				if queued {
					continue
				}
				e.logf(loopCtx, "Code location repair needed retry but retry budget is exhausted; keeping response as-is: missing=%v", retryInvalid.MissingFields)
			}
		}

		if resp.ReasoningEffort != "" {
			result.reasoningEffort = resp.ReasoningEffort
			llmReq.ReasoningEffort = resp.ReasoningEffort
		}
		result.contentMessages = appendResponseContent(result.contentMessages, resp)
		result.resp = resp
		if req.ValidateResponse != nil {
			if invalidResp := req.ValidateResponse(resp); invalidResp != nil {
				if outputRetriesRemaining(state.jsonRetries, req.MaxOutputRetries) {
					if invalidResp.ReasoningEffort != "" {
						result.reasoningEffort = invalidResp.ReasoningEffort
						llmReq.ReasoningEffort = invalidResp.ReasoningEffort
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
				if invalidResp.PartialResponse != nil {
					e.logf(loopCtx, "Response validation failed after retries exhausted; using partial validated response: reason=%q missing=%v", invalidResp.Reason, invalidResp.MissingFields)
					resp = invalidResp.PartialResponse
					result.resp = resp
				} else {
					e.logf(loopCtx, "Response validation failed after retries exhausted: reason=%q missing=%v", invalidResp.Reason, invalidResp.MissingFields)
				}
			}
		}

		invalidToolCalls := resp.ToolCalls
		originalToolCalls := len(resp.ToolCalls)
		resp.ToolCalls, _ = filterAgentToolCalls(resp.ToolCalls, req.Tools)
		if originalToolCalls > 0 && len(resp.ToolCalls) == 0 {
			if outputRetriesRemaining(state.jsonRetries, req.MaxOutputRetries) {
				state.jsonRetries++
				e.logf(loopCtx, "Invalid tool call response, retrying without tool history: attempt=%d", state.jsonRetries)
				if strings.TrimSpace(resp.RawResponse) != "" {
					messages = append(messages, llm.Message{Role: "assistant", Content: resp.RawResponse})
				} else {
					// Without raw content the retry would resend byte-identical
					// messages; inject a short corrective note describing the
					// invalid calls so the retry request can differ. The
					// placeholder assistant turn keeps the history alternating
					// for strict-role providers (the failed turn produced no
					// appendable content of its own).
					messages = append(messages, llm.Message{Role: "assistant", Content: "[invalid tool calls]"})
					messages = append(messages, llm.Message{Role: "user", Content: invalidToolCallFeedback(invalidToolCalls)})
					syntheticFollowup = nil
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
			e.logf(loopCtx, "Tool call limit reached, making final call without tools: limit=%d used=%d requested=%d", req.MaxToolCalls, state.toolCalls, pendingToolCalls)
			finalMessages := append([]llm.Message(nil), messages...)
			if strings.TrimSpace(resp.RawResponse) != "" {
				finalMessages = append(finalMessages, llm.Message{Role: "assistant", Content: resp.RawResponse})
			}
			resp, err = e.agentLoopReviewWithoutTools(loopCtx, llmReq, req, finalMessages, state)
			if err != nil {
				return result, err
			}
			result.tokensUsed = addTokenUsage(result.tokensUsed, resp.TokensUsed)
			result.contentMessages = appendResponseContent(result.contentMessages, resp)
			result.resp = resp
			break
		}

		e.logf(loopCtx, "Executing tool batch: used=%d requested=%d", state.toolCalls, pendingToolCalls)
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
			e.logf(loopCtx, "Duplicate tool call limit reached, making final call without tools: limit=%d duplicates=%d", req.MaxDuplicateToolCalls, state.duplicateToolCalls)
			resp, err = e.agentLoopReviewWithoutTools(loopCtx, llmReq, req, messages, state)
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

func (e *Engine) tryRepairPartialResponse(ctx context.Context, req agentLoopRequest, invalidResp *llm.InvalidResponseError) (*llm.ReviewResponse, *llm.InvalidResponseError, bool) {
	if invalidResp == nil || invalidResp.PartialResponse == nil || req.RepairResponse == nil {
		return nil, nil, false
	}
	if !onlyCodeLocationMissingFields(invalidResp.MissingFields) {
		return nil, nil, false
	}
	resp := invalidResp.PartialResponse
	retryInvalid := e.repairResponseOrRetry(ctx, req, resp)
	if retryInvalid == nil {
		e.logf(ctx, "Code location repair accepted partial parsed response: missing=%v", invalidResp.MissingFields)
	}
	return resp, retryInvalid, true
}

func onlyCodeLocationMissingFields(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		if !strings.Contains(field, "code_location") {
			return false
		}
	}
	return true
}

func (e *Engine) repairResponseOrRetry(ctx context.Context, req agentLoopRequest, resp *llm.ReviewResponse) *llm.InvalidResponseError {
	if req.RepairResponse == nil || resp == nil {
		return nil
	}
	result := req.RepairResponse(ctx, resp)
	if len(result.RetryFields) == 0 {
		return nil
	}
	return &llm.InvalidResponseError{
		RawContent:            resp.RawResponse,
		Reason:                "code_location needs file_path plus content or line_range",
		MissingFields:         result.RetryFields,
		ReasoningEffort:       resp.ReasoningEffort,
		ValidationFailure:     true,
		RetryGuidanceTemplate: "",
		PartialResponse:       resp,
	}
}

func (e *Engine) canRetryCodeLocation(state *agentLoopState, maxRetries int) bool {
	if state == nil || state.codeLocationRetried {
		return false
	}
	return outputRetriesRemaining(state.jsonRetries, maxRetries)
}

func (e *Engine) tryQueueCodeLocationRetry(ctx context.Context, req agentLoopRequest, state *agentLoopState, invalidResp *llm.InvalidResponseError, messages *[]llm.Message, syntheticFollowup **llm.Message, llmReq *llm.ReviewRequest, retriesAvailable bool) (bool, error) {
	if !retriesAvailable || !e.canRetryCodeLocation(state, req.MaxOutputRetries) {
		return false, nil
	}
	if err := e.queueCodeLocationRetry(ctx, req, state, invalidResp, messages, syntheticFollowup, llmReq); err != nil {
		return false, err
	}
	return true, nil
}

func (e *Engine) queueCodeLocationRetry(ctx context.Context, req agentLoopRequest, state *agentLoopState, invalidResp *llm.InvalidResponseError, messages *[]llm.Message, syntheticFollowup **llm.Message, llmReq *llm.ReviewRequest) error {
	if state == nil || invalidResp == nil || messages == nil {
		return nil
	}
	if invalidResp.ReasoningEffort != "" && llmReq != nil {
		llmReq.ReasoningEffort = invalidResp.ReasoningEffort
	}
	state.codeLocationRetried = true
	state.jsonRetries++
	e.logJSONRetry(ctx, req, state.jsonRetries, invalidResp)
	if strings.TrimSpace(invalidResp.RawContent) != "" {
		*messages = append(*messages, llm.Message{Role: "assistant", Content: invalidResp.RawContent})
	}
	feedback, err := e.renderJSONRetryFeedback(invalidResp, req.JSONRetryExampleSnippet)
	if err != nil {
		return err
	}
	*messages = append(*messages, llm.Message{Role: "user", Content: feedback})
	if syntheticFollowup != nil {
		*syntheticFollowup = nil
	}
	return nil
}

func (e *Engine) agentLoopReviewWithoutTools(ctx context.Context, llmReq *llm.ReviewRequest, req agentLoopRequest, messages []llm.Message, state *agentLoopState) (*llm.ReviewResponse, error) {
	noToolsReq := *llmReq
	noToolsReq.Tools = nil
	noToolsReq.ParallelToolCalls = false
	// reviewWithoutTools computes the no-tools transcript itself, preferring
	// req.NoToolsMessages when set (which is always the case for loop agents).
	return e.reviewWithoutTools(ctx, &noToolsReq, req.AgentKind, req.NoToolsSystem, messages, req.NoToolsSchemaSnippet, req.NoToolsStyleGuideToolchainSnippet, req.DisableSuggestions, req.MaxOutputRetries, req.Section, req, state)
}

func (e *Engine) logJSONRetry(ctx context.Context, req agentLoopRequest, attempt int, invalidResp *llm.InvalidResponseError) {
	if req.JSONRetryProgressAgentName == "" {
		e.logf(ctx, "Verify: invalid JSON, retrying: attempt=%d reason=%q missing=%v", attempt, invalidResp.Reason, invalidResp.MissingFields)
		return
	}
	e.logf(ctx, "Invalid JSON response, retrying with feedback: attempt=%d reason=%q missing=%v", attempt, invalidResp.Reason, invalidResp.MissingFields)
	if e.logger != nil {
		e.logger.Progress(ctx, logging.StageModel, logging.StateRetry, fmt.Sprintf("invalid JSON, attempt=%d", attempt))
	}
}

// invalidToolCallFeedback describes a batch of rejected tool calls so a retry
// after an all-invalid tool response carries a corrective user message instead
// of resending a byte-identical request.
func invalidToolCallFeedback(toolCalls []llm.ToolCall) string {
	names := make([]string, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		name := strings.TrimSpace(toolCall.Name)
		if name == "" {
			name = "<unnamed>"
		}
		names = append(names, name)
	}
	return fmt.Sprintf(
		"Your previous response contained only invalid tool calls (%s). Every tool call needs a known tool name, a call id, and valid JSON arguments including the required fields. Either issue corrected tool calls or answer directly in the required output format.",
		strings.Join(names, ", "),
	)
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
