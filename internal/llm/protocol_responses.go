package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/dgrieser/nickpit/internal/model"
)

// ResponsesProtocolName names the OpenAI Responses protocol.
const ResponsesProtocolName = "responses"

// responsesProtocol speaks POST /responses, OpenAI's Responses API. It is the
// only route Sign in with ChatGPT admits (streamed, store=false), and works
// with a platform API key as well. Requests are stateless: every call sends
// the full conversation, so no response is stored server-side.
type responsesProtocol struct {
	mu      sync.Mutex
	dropped map[string]bool
}

// NewResponsesProtocol returns a Responses protocol. It is stateful — it
// remembers optional fields the endpoint rejected — so each client gets its
// own.
func NewResponsesProtocol() Protocol {
	return &responsesProtocol{dropped: map[string]bool{}}
}

func (*responsesProtocol) Name() string { return ResponsesProtocolName }

func (*responsesProtocol) Capabilities() Capabilities {
	return Capabilities{
		// The Responses API never streams raw reasoning tokens, only the
		// summaries requested with reasoning.summary.
		Reasoning:         ReasoningSummary,
		ReasoningEffort:   true,
		Tools:             true,
		ParallelToolCalls: true,
		StructuredOutput:  true,
		SamplingParams:    []string{"temperature", "top_p"},
	}
}

func (*responsesProtocol) Path() string { return "/responses" }

func (*responsesProtocol) SetHeaders(header http.Header, token string) {
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
}

// responsesOptionalParams are the request fields the protocol adds of its own
// accord. An endpoint that rejects one of them gets requests without it.
var responsesOptionalParams = []string{"reasoning.summary", "temperature", "top_p"}

func (p *responsesProtocol) isDropped(param string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dropped[param]
}

// DropUnsupported implements ParamDropper.
func (p *responsesProtocol) DropUnsupported(perr *ProviderError) bool {
	if perr == nil {
		return false
	}
	for _, param := range responsesOptionalParams {
		named := perr.Param == param || perr.Param == "" && strings.Contains(perr.Message, "'"+param+"'")
		if !named {
			continue
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.dropped[param] {
			return false
		}
		p.dropped[param] = true
		return true
	}
	return false
}

type responsesRequest struct {
	Model             string              `json:"model"`
	Instructions      string              `json:"instructions,omitempty"`
	Input             []responsesInput    `json:"input"`
	Tools             []responsesTool     `json:"tools,omitempty"`
	ParallelToolCalls *bool               `json:"parallel_tool_calls,omitempty"`
	Text              *responsesText      `json:"text,omitempty"`
	Reasoning         *responsesReasoning `json:"reasoning,omitempty"`
	MaxOutputTokens   *int                `json:"max_output_tokens,omitempty"`
	Temperature       *float64            `json:"temperature,omitempty"`
	TopP              *float64            `json:"top_p,omitempty"`
	Store             bool                `json:"store"`
	Stream            bool                `json:"stream"`
}

// responsesInput is a message or a function-call item of the input list.
type responsesInput struct {
	Type      string `json:"type,omitempty"`
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`
}

type responsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      bool            `json:"strict"`
}

type responsesText struct {
	Format responsesFormat `json:"format"`
}

type responsesFormat struct {
	Type   string          `json:"type"`
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

type responsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

func (p *responsesProtocol) EncodeRequest(req *CompletionRequest) (json.RawMessage, error) {
	instructions, input := responsesConversation(req.Messages)
	out := responsesRequest{
		Model:           req.Model,
		Instructions:    instructions,
		Input:           input,
		MaxOutputTokens: req.MaxTokens,
		Store:           false,
		Stream:          true,
	}
	for _, tool := range req.Tools {
		parameters := tool.Parameters
		if len(parameters) == 0 {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		// Strict stays off for the same reason as on Chat Completions: the
		// schemas use optional properties, which strict mode forbids.
		out.Tools = append(out.Tools, responsesTool{Type: "function", Name: tool.Name, Description: tool.Description, Parameters: parameters})
	}
	if len(out.Tools) > 0 {
		parallel := req.ParallelToolCalls
		out.ParallelToolCalls = &parallel
	}
	if len(req.Schema) > 0 {
		out.Text = &responsesText{Format: responsesFormat{Type: "json_schema", Name: req.SchemaName, Schema: req.Schema}}
	}
	if req.Temperature != nil && !p.isDropped("temperature") {
		out.Temperature = req.Temperature
	}
	if req.TopP != nil && !p.isDropped("top_p") {
		out.TopP = req.TopP
	}
	reasoning := &responsesReasoning{Effort: req.ReasoningEffort}
	if reasoning.Effort == "off" {
		reasoning.Effort = "none"
	}
	if reasoning.Effort != "none" && !p.isDropped("reasoning.summary") {
		// Without a summary the API streams no reasoning text at all, which
		// would leave the reasoning display and budget nothing to observe.
		reasoning.Summary = "auto"
	}
	if *reasoning != (responsesReasoning{}) {
		out.Reasoning = reasoning
	}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return mergeOrderedJSONObject(data, req.ExtraBody)
}

// responsesConversation maps the neutral history onto the Responses input
// list. Leading system messages become the instructions; a later one (a
// nudge) stays in place as a developer message. Assistant tool calls and tool
// results become function_call and function_call_output items.
func responsesConversation(messages []Message) (string, []responsesInput) {
	var instructions []string
	input := make([]responsesInput, 0, len(messages))
	leading := true
	for _, msg := range messages {
		switch msg.Role {
		case RoleSystem, "developer":
			if leading {
				if msg.Content != "" {
					instructions = append(instructions, msg.Content)
				}
				continue
			}
			input = append(input, responsesInput{Role: "developer", Content: msg.Content})
		case RoleAssistant:
			leading = false
			if msg.Content != "" {
				input = append(input, responsesInput{Role: RoleAssistant, Content: msg.Content})
			}
			for _, call := range msg.ToolCalls {
				arguments, ok := NormalizeToolCallArguments(call.Arguments)
				if !ok || strings.TrimSpace(call.ID) == "" || strings.TrimSpace(call.Name) == "" {
					continue
				}
				input = append(input, responsesInput{Type: "function_call", CallID: call.ID, Name: call.Name, Arguments: arguments})
			}
		case RoleTool:
			leading = false
			input = append(input, responsesInput{Type: "function_call_output", CallID: msg.ToolCallID, Output: msg.Content})
		default:
			leading = false
			input = append(input, responsesInput{Role: RoleUser, Content: msg.Content})
		}
	}
	return strings.Join(instructions, "\n\n"), input
}

func (*responsesProtocol) ParseError(status int, body []byte) *ProviderError {
	perr := responsesErrorFromBody(body)
	if perr.Message == "" && perr.Code == "" {
		perr.Message = providerErrorMessage(body)
		if perr.Message == "" {
			perr.Message = cleanHTTPErrorText(string(body))
		}
		perr.Status = status
		return perr
	}
	perr.Status = responsesErrorStatus(perr.Code, status)
	perr.Message = responsesErrorMessage(perr)
	return perr
}

// responsesErrorFromBody reads an error body: the standard error object, an
// OAuth-style string error code, or an admission {"detail": "..."}.
func responsesErrorFromBody(body []byte) *ProviderError {
	var envelope struct {
		Error  json.RawMessage `json:"error"`
		Detail json.RawMessage `json:"detail"`
	}
	perr := &ProviderError{}
	if json.Unmarshal(body, &envelope) != nil {
		return perr
	}
	if len(envelope.Error) > 0 {
		var object struct {
			Code    any    `json:"code"`
			Message string `json:"message"`
			Param   string `json:"param"`
			Type    string `json:"type"`
		}
		var code string
		if json.Unmarshal(envelope.Error, &object) == nil {
			if object.Code != nil {
				perr.Code = fmt.Sprint(object.Code)
			}
			perr.Message, perr.Param, perr.Type = object.Message, object.Param, object.Type
		} else if json.Unmarshal(envelope.Error, &code) == nil {
			perr.Code = code
		}
	}
	if perr.Message == "" && len(envelope.Detail) > 0 {
		var detail string
		if json.Unmarshal(envelope.Detail, &detail) == nil {
			perr.Message = detail
		}
	}
	return perr
}

func normalizeSubscriptionCode(code string) string {
	return strings.Replace(code, "subscription_sharing_v2_", "subscription_sharing_", 1)
}

// responsesErrorStatus maps a Responses error code onto the status the retry
// policy should see. A ChatGPT plan usage limit is reported as 403, not the
// 429 it arrives as: ChatGPT gives no reset time and the limit lasts hours, so
// the rate-limit backoff would only burn the run's wait budget before failing
// anyway.
func responsesErrorStatus(code string, fallback int) int {
	switch normalizeSubscriptionCode(code) {
	case "subscription_sharing_usage_limit_exceeded",
		"subscription_sharing_user_not_eligible",
		"subscription_sharing_route_not_supported",
		"chatpass_v2_scope_not_authorized",
		"chatpass_v2_invalid_authorization_context":
		return http.StatusForbidden
	case "subscription_sharing_invalid_user":
		return http.StatusUnauthorized
	case "subscription_sharing_unsupported_capability", "context_length_exceeded", "invalid_prompt", "model_not_found":
		return http.StatusBadRequest
	case "subscription_sharing_usage_unavailable", "subscription_sharing_user_unavailable", "server_error":
		return http.StatusServiceUnavailable
	case "rate_limit_exceeded":
		return http.StatusTooManyRequests
	}
	return fallback
}

func responsesErrorMessage(perr *ProviderError) string {
	message := perr.Message
	switch normalizeSubscriptionCode(perr.Code) {
	case "subscription_sharing_usage_limit_exceeded":
		message = "ChatGPT plan usage limit reached; check https://chatgpt.com/settings/usage"
	case "subscription_sharing_user_not_eligible":
		message = "ChatGPT plan usage is not available for this account or workspace"
	case "subscription_sharing_invalid_user":
		message = "ChatGPT did not accept the signed-in account; run `nickpit chatgpt login` again"
	case "subscription_sharing_unsupported_capability":
		if perr.Param != "" {
			message = fmt.Sprintf("the endpoint does not support %q in this request", perr.Param)
		}
	}
	if message == "" {
		message = perr.Code
	}
	if perr.Code != "" && !strings.Contains(message, perr.Code) {
		message += " (" + perr.Code + ")"
	}
	// The API names the offending field in param, not the message; keep it
	// visible so matching on the field (the reasoning-effort ladder looks
	// for "reasoning") still recognizes the rejection.
	if perr.Param != "" && !strings.Contains(message, perr.Param) {
		message += " (param: " + perr.Param + ")"
	}
	return message
}

func (*responsesProtocol) NewEventReader(body io.Reader) EventReader {
	return &responsesEventReader{
		reader:        bufio.NewReaderSize(body, 64*1024),
		callsByItem:   map[string]int{},
		callsByOutput: map[int]int{},
		argsSent:      map[int]bool{},
	}
}

// responsesEventReader decodes the Responses event stream. Text, reasoning
// summaries, and function-call fragments become chunks; response.completed
// and response.incomplete end the stream with a finish reason and usage; a
// failure becomes a *ProviderError. Every other event is a keep-alive.
type responsesEventReader struct {
	reader *bufio.Reader
	done   bool

	// callsByItem and callsByOutput map a function_call item (by item id,
	// then output index) to its tool-call index; argsSent records which ones
	// streamed argument deltas, so a .done event does not repeat them.
	callsByItem   map[string]int
	callsByOutput map[int]int
	argsSent      map[int]bool
	callCount     int

	sawSummary       bool
	lastSummaryIndex int
}

func (r *responsesEventReader) Next() (StreamChunk, error) {
	if r.done {
		return StreamChunk{}, io.EOF
	}
	data, err := r.readEvent()
	if err != nil {
		return StreamChunk{}, err
	}
	if string(data) == "[DONE]" {
		r.done = true
		return StreamChunk{}, io.EOF
	}
	return r.decode(data)
}

// readEvent returns the data of the next server-sent event.
func (r *responsesEventReader) readEvent() ([]byte, error) {
	var data bytes.Buffer
	for {
		line, err := r.reader.ReadBytes('\n')
		trimmed := bytes.TrimRight(line, "\r\n")
		if len(trimmed) == 0 && data.Len() > 0 {
			return data.Bytes(), nil
		}
		if payload, ok := bytes.CutPrefix(trimmed, []byte("data:")); ok {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.Write(bytes.TrimPrefix(payload, []byte(" ")))
		}
		if err != nil {
			if data.Len() > 0 && errors.Is(err, io.EOF) {
				return data.Bytes(), nil
			}
			if errors.Is(err, io.EOF) {
				// The stream ended without a terminal event; the client
				// reports that as an interrupted response.
				return nil, io.EOF
			}
			return nil, err
		}
	}
}

type responsesEvent struct {
	Type         string            `json:"type"`
	Delta        string            `json:"delta"`
	ItemID       string            `json:"item_id"`
	OutputIndex  int               `json:"output_index"`
	SummaryIndex int               `json:"summary_index"`
	Arguments    string            `json:"arguments"`
	Item         *responsesItem    `json:"item"`
	Response     *responsesPayload `json:"response"`
	Code         any               `json:"code"`
	Message      string            `json:"message"`
	Param        string            `json:"param"`
	Error        json.RawMessage   `json:"error"`
}

type responsesItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type responsesPayload struct {
	Error *struct {
		Code    any    `json:"code"`
		Message string `json:"message"`
		Param   string `json:"param"`
	} `json:"error"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func (r *responsesEventReader) decode(data []byte) (StreamChunk, error) {
	var event responsesEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return StreamChunk{}, fmt.Errorf("llm: decoding responses stream event: %w", err)
	}
	switch event.Type {
	case "response.output_text.delta", "response.refusal.delta":
		return StreamChunk{Text: event.Delta}, nil
	case "response.reasoning_summary_text.delta":
		delta := event.Delta
		if r.sawSummary && event.SummaryIndex != r.lastSummaryIndex && delta != "" {
			delta = "\n\n" + delta
		}
		if delta != "" {
			r.sawSummary = true
			r.lastSummaryIndex = event.SummaryIndex
		}
		return StreamChunk{Reasoning: delta, ReasoningKind: ReasoningSummary}, nil
	case "response.reasoning_text.delta":
		return StreamChunk{Reasoning: event.Delta, ReasoningKind: ReasoningRaw}, nil
	case "response.output_item.added":
		if event.Item == nil || event.Item.Type != "function_call" {
			break
		}
		index := r.callCount
		r.callCount++
		if event.Item.ID != "" {
			r.callsByItem[event.Item.ID] = index
		}
		r.callsByOutput[event.OutputIndex] = index
		if event.Item.Arguments != "" {
			r.argsSent[index] = true
		}
		return StreamChunk{ToolCalls: []ToolCallDelta{{Index: &index, ID: event.Item.CallID, Name: event.Item.Name, Arguments: event.Item.Arguments}}}, nil
	case "response.function_call_arguments.delta":
		if index, ok := r.callIndex(event.ItemID, event.OutputIndex); ok && event.Delta != "" {
			r.argsSent[index] = true
			return StreamChunk{ToolCalls: []ToolCallDelta{{Index: &index, Arguments: event.Delta}}}, nil
		}
	case "response.function_call_arguments.done":
		if index, ok := r.callIndex(event.ItemID, event.OutputIndex); ok && !r.argsSent[index] && event.Arguments != "" {
			r.argsSent[index] = true
			return StreamChunk{ToolCalls: []ToolCallDelta{{Index: &index, Arguments: event.Arguments}}}, nil
		}
	case "response.output_item.done":
		if event.Item == nil || event.Item.Type != "function_call" {
			break
		}
		if index, ok := r.callIndex(event.Item.ID, event.OutputIndex); ok && !r.argsSent[index] && event.Item.Arguments != "" {
			r.argsSent[index] = true
			return StreamChunk{ToolCalls: []ToolCallDelta{{Index: &index, Arguments: event.Item.Arguments}}}, nil
		}
	case "response.completed":
		r.done = true
		finish := FinishStop
		if r.callCount > 0 {
			finish = FinishToolCalls
		}
		return StreamChunk{FinishReason: finish, Usage: responsesUsage(event.Response)}, nil
	case "response.incomplete":
		r.done = true
		finish := FinishLength
		if event.Response != nil && event.Response.IncompleteDetails != nil && event.Response.IncompleteDetails.Reason == "content_filter" {
			finish = FinishContentFilter
		}
		return StreamChunk{FinishReason: finish, Usage: responsesUsage(event.Response)}, nil
	case "response.failed":
		r.done = true
		perr := &ProviderError{Code: "response_failed", Message: "the response failed"}
		if event.Response != nil && event.Response.Error != nil {
			perr = &ProviderError{Code: codeString(event.Response.Error.Code), Message: event.Response.Error.Message, Param: event.Response.Error.Param}
		}
		return StreamChunk{}, streamProviderError(perr)
	case "error":
		r.done = true
		perr := &ProviderError{Code: codeString(event.Code), Message: event.Message, Param: event.Param}
		if len(event.Error) > 0 {
			if nested := responsesErrorFromBody(append(append([]byte(`{"error":`), event.Error...), '}')); nested.Code != "" || nested.Message != "" {
				perr = nested
			}
		}
		return StreamChunk{}, streamProviderError(perr)
	}
	return StreamChunk{}, nil
}

// streamProviderError finishes an error reported inside the stream. It gets
// the status its code maps to, falling back to a server error, so a failure
// before any output is retried like the equivalent error response would be.
func streamProviderError(perr *ProviderError) *ProviderError {
	perr.Status = responsesErrorStatus(perr.Code, http.StatusInternalServerError)
	perr.Message = responsesErrorMessage(perr)
	return perr
}

func codeString(code any) string {
	if code == nil {
		return ""
	}
	return fmt.Sprint(code)
}

func (r *responsesEventReader) callIndex(itemID string, outputIndex int) (int, bool) {
	if itemID != "" {
		if index, ok := r.callsByItem[itemID]; ok {
			return index, true
		}
	}
	index, ok := r.callsByOutput[outputIndex]
	return index, ok
}

func responsesUsage(payload *responsesPayload) *model.TokenUsage {
	usage := &model.TokenUsage{}
	if payload == nil || payload.Usage == nil {
		return usage
	}
	usage.PromptTokens = payload.Usage.InputTokens
	usage.CompletionTokens = payload.Usage.OutputTokens
	usage.TotalTokens = payload.Usage.TotalTokens
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage
}

// ProtocolByName returns a new instance of the named protocol; an empty name
// selects Chat Completions.
func ProtocolByName(name string) (Protocol, error) {
	switch name {
	case "", ChatCompletionsProtocolName:
		return ChatCompletionsProtocol(), nil
	case ResponsesProtocolName:
		return NewResponsesProtocol(), nil
	}
	return nil, fmt.Errorf("llm: unknown protocol %q", name)
}
