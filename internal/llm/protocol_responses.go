package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"

	"github.com/dgrieser/nickpit/internal/model"
)

// ResponsesProtocolName names the OpenAI Responses protocol.
const ResponsesProtocolName = "responses"

// responsesProtocol speaks POST /responses, OpenAI's Responses API, with a
// platform API key or — in plan mode — Sign in with ChatGPT tokens. Requests
// are stateless (store=false): every call sends the full conversation, and
// the encrypted reasoning that lets a reasoning model continue after its tool
// calls travels back with the assistant turn as provider state.
type responsesProtocol struct {
	plan bool

	mu      sync.Mutex
	dropped map[string]bool
}

// ResponsesOptions configures a Responses protocol.
type ResponsesOptions struct {
	// ChatGPTPlan shapes requests for Sign in with ChatGPT plan usage, which
	// admits a narrower request than the platform API: function tools only
	// through an additional_tools input item, and no output-token limit or
	// sampling knobs.
	ChatGPTPlan bool
}

// NewResponsesProtocol returns a Responses protocol for a platform API key.
// It is stateful — it remembers optional fields the endpoint rejected — so
// each client gets its own.
func NewResponsesProtocol() Protocol {
	return NewResponsesProtocolWith(ResponsesOptions{})
}

// NewResponsesProtocolWith returns a Responses protocol configured by opts.
func NewResponsesProtocolWith(opts ResponsesOptions) Protocol {
	return &responsesProtocol{plan: opts.ChatGPTPlan, dropped: map[string]bool{}}
}

func (*responsesProtocol) Name() string { return ResponsesProtocolName }

// chatGPTPlanUnsupportedFields are the request fields Sign in with ChatGPT
// plan usage rejects (its preview limitations), so they are never sent.
var chatGPTPlanUnsupportedFields = []string{
	"background", "conversation", "max_output_tokens", "max_tool_calls", "metadata", "moderation",
	"multi_agent", "previous_response_id", "prompt", "prompt_cache_retention", "safety_identifier",
	"temperature", "top_logprobs", "top_p", "truncation", "user",
}

func (p *responsesProtocol) Capabilities() Capabilities {
	caps := Capabilities{
		// The Responses API never streams raw reasoning tokens, only the
		// summaries requested with reasoning.summary.
		Reasoning:         ReasoningSummary,
		ReasoningEffort:   true,
		Tools:             true,
		ParallelToolCalls: true,
		StructuredOutput:  true,
		OutputTokenLimit:  true,
		SamplingParams:    []string{"temperature", "top_p"},
	}
	caps.ReservedFields = p.reservedFields()
	if p.plan {
		caps.OutputTokenLimit = false
		caps.SamplingParams = nil
		caps.UnsupportedFields = chatGPTPlanUnsupportedFields
	}
	return caps
}

func (*responsesProtocol) Path() string { return "/responses" }

func (*responsesProtocol) SetHeaders(header http.Header, token string) {
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
}

// responsesOptionalParams are the request fields the protocol adds of its own
// accord. An endpoint that rejects one of them gets requests without it.
var responsesOptionalParams = []string{"reasoning.summary", "include", "temperature", "top_p"}

// responsesEncryptedReasoning asks for the reasoning items' encrypted content,
// which a stateless request needs to replay them.
const responsesEncryptedReasoning = "reasoning.encrypted_content"

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
		named := perr.Param == param || strings.HasPrefix(perr.Param, param+"[") ||
			perr.Param == "" && strings.Contains(perr.Message, "'"+param+"'")
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
	Input             []any               `json:"input"`
	Tools             []responsesTool     `json:"tools,omitempty"`
	ParallelToolCalls *bool               `json:"parallel_tool_calls,omitempty"`
	Text              *responsesText      `json:"text,omitempty"`
	Reasoning         *responsesReasoning `json:"reasoning,omitempty"`
	Include           []string            `json:"include,omitempty"`
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

// responsesAdditionalTools declares function tools from inside the input
// list, the form ChatGPT plan usage admits them in. The tools are available
// from this item's position on, so it leads the input.
type responsesAdditionalTools struct {
	Type  string          `json:"type"`
	Role  string          `json:"role"`
	Tools []responsesTool `json:"tools"`
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
		Model:        req.Model,
		Instructions: instructions,
		Input:        input,
		Store:        false,
		Stream:       true,
	}
	var tools []responsesTool
	for _, tool := range req.Tools {
		parameters := tool.Parameters
		if len(parameters) == 0 {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		// Strict stays off for the same reason as on Chat Completions: the
		// schemas use optional properties, which strict mode forbids.
		tools = append(tools, responsesTool{Type: "function", Name: tool.Name, Description: tool.Description, Parameters: parameters})
	}
	if len(tools) > 0 {
		if p.plan {
			// Plan usage rejects function tools in the top-level list.
			out.Input = append([]any{responsesAdditionalTools{Type: "additional_tools", Role: "developer", Tools: tools}}, out.Input...)
		} else {
			out.Tools = tools
		}
		out.ParallelToolCalls = req.ParallelToolCalls
	}
	if len(req.Schema) > 0 {
		out.Text = &responsesText{Format: responsesFormat{Type: "json_schema", Name: req.SchemaName, Schema: req.Schema}}
	}
	// The core only passes what Capabilities admits; the guards keep plan
	// requests clean even for a caller that bypasses it.
	if !p.plan {
		out.MaxOutputTokens = req.MaxTokens
		if req.Temperature != nil && !p.isDropped("temperature") {
			out.Temperature = req.Temperature
		}
		if req.TopP != nil && !p.isDropped("top_p") {
			out.TopP = req.TopP
		}
	}
	reasoning := &responsesReasoning{Effort: req.ReasoningEffort}
	if reasoning.Effort == "off" {
		reasoning.Effort = "none"
	}
	if reasoning.Effort != "none" {
		if !p.isDropped("reasoning.summary") {
			// Without a summary the API streams no reasoning text at all,
			// which would leave the reasoning display and budget nothing to
			// observe.
			reasoning.Summary = "auto"
		}
		if !p.isDropped("include") {
			out.Include = []string{responsesEncryptedReasoning}
		}
	}
	if *reasoning != (responsesReasoning{}) {
		out.Reasoning = reasoning
	}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	// The core already strips reserved fields from extra_body; dropping them
	// here as well keeps the invariants — a streamed, stateless request, its
	// conversation, and plan mode's tool form — whoever builds the request.
	extra := maps.Clone(req.ExtraBody)
	for _, field := range p.reservedFields() {
		delete(extra, field)
	}
	return mergeOrderedJSONObject(data, extra)
}

// reservedFields are the request fields this protocol owns. Plan mode adds
// store (plan usage requires store=false) and the top-level tools list (plan
// usage admits function tools only through additional_tools).
func (p *responsesProtocol) reservedFields() []string {
	fields := []string{"stream", "input", "instructions", "include"}
	if p.plan {
		fields = append(fields, "store", "tools")
	}
	return fields
}

// responsesConversation maps the neutral history onto the Responses input
// list. Leading system messages become the instructions; a later one (a
// nudge) stays in place as a developer message, since system-role input items
// are rejected on the plan route. An assistant turn replays its provider
// state (the encrypted reasoning that produced it) ahead of its text and
// function_call items, the order the model emitted them in; tool results
// become function_call_output items.
func responsesConversation(messages []Message) (string, []any) {
	var instructions []string
	input := make([]any, 0, len(messages))
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
			if msg.ProviderState != nil {
				for _, item := range msg.ProviderState.Items {
					input = append(input, item)
				}
			}
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
	case "response.output_text.delta":
		return StreamChunk{Text: event.Delta}, nil
	case "response.refusal.delta":
		return StreamChunk{Refusal: event.Delta}, nil
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
		if event.Item != nil && event.Item.Type == "reasoning" {
			if item := replayableReasoningItem(data); item != nil {
				return StreamChunk{StateItem: item}, nil
			}
			break
		}
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

// replayableReasoningItem extracts a finished reasoning item in the form a
// stateless request replays it: type, summary, and the encrypted content.
// The server-side id and status are left out — with store=false there is no
// stored item for an id to refer to. An item without encrypted content holds
// nothing the model could resume from, so it is not kept.
func replayableReasoningItem(event []byte) json.RawMessage {
	var envelope struct {
		Item struct {
			Type             string          `json:"type"`
			Summary          json.RawMessage `json:"summary"`
			EncryptedContent string          `json:"encrypted_content"`
		} `json:"item"`
	}
	if json.Unmarshal(event, &envelope) != nil || envelope.Item.EncryptedContent == "" {
		return nil
	}
	summary := envelope.Item.Summary
	if len(summary) == 0 || string(summary) == "null" {
		summary = json.RawMessage(`[]`)
	}
	item, err := json.Marshal(struct {
		Type             string          `json:"type"`
		Summary          json.RawMessage `json:"summary"`
		EncryptedContent string          `json:"encrypted_content"`
	}{Type: "reasoning", Summary: summary, EncryptedContent: envelope.Item.EncryptedContent})
	if err != nil {
		return nil
	}
	return item
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
