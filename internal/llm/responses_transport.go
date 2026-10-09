package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TokenSource supplies the bearer token for an endpoint whose credential is
// not a static API key (Sign in with ChatGPT). Token returns a currently valid
// access token, refreshing it first when it is about to expire; ForceRefresh
// renews it regardless, for a request the server rejected as unauthorized.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
	ForceRefresh(ctx context.Context) (string, error)
}

// UseResponsesAPI routes the client's chat-completion calls through the
// OpenAI Responses API, authenticated with tokens instead of the static key.
// Sign in with ChatGPT admits only POST /v1/responses (streamed, store=false),
// so the transport translates each Chat Completions request into a Responses
// request and the Responses event stream back into Chat Completions chunks;
// everything above the transport — retries, tool loops, reasoning budgets —
// stays unchanged.
func (c *OpenAIClient) UseResponsesAPI(tokens TokenSource) {
	c.transport.base = newResponsesTransport(c.transport.base, tokens)
}

const (
	// responsesPeekWindow bounds how long RoundTrip waits for the first
	// output event before handing the stream to the caller. Admission failures
	// (usage limits, unsupported capabilities) arrive as a response.failed
	// right after response.created; catching them here turns them into HTTP
	// errors the client's retry policy understands. A slow first token must
	// not hold the stream past this, or the idle watchdog would never start.
	responsesPeekWindow = 20 * time.Second
	// maxResponsesRequestAttempts bounds the optional-field fallbacks plus the
	// one forced token refresh a single request can go through.
	maxResponsesRequestAttempts = 5
)

// droppableResponsesParams are the request fields the translation adds on its
// own initiative. When the endpoint rejects one of them as unsupported, the
// transport drops it for the rest of the run instead of failing the request.
var droppableResponsesParams = []string{"reasoning.summary", "temperature", "top_p"}

type responsesTransport struct {
	base   http.RoundTripper
	tokens TokenSource

	mu      sync.Mutex
	dropped map[string]bool
}

func newResponsesTransport(base http.RoundTripper, tokens TokenSource) *responsesTransport {
	return &responsesTransport{base: base, tokens: tokens, dropped: map[string]bool{}}
}

func (t *responsesTransport) droppedParams() map[string]bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[string]bool, len(t.dropped))
	for key, value := range t.dropped {
		out[key] = value
	}
	return out
}

func (t *responsesTransport) dropParam(param string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.dropped[param] = true
}

func (t *responsesTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return t.forward(req)
	}

	var chat map[string]any
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("llm: reading chat request for responses translation: %w", err)
		}
		if err := json.Unmarshal(data, &chat); err != nil {
			return nil, fmt.Errorf("llm: decoding chat request for responses translation: %w", err)
		}
	}

	refreshed := false
	for attempt := 0; ; attempt++ {
		dropped := t.droppedParams()
		body, err := json.Marshal(chatToResponsesRequest(chat, dropped))
		if err != nil {
			return nil, fmt.Errorf("llm: encoding responses request: %w", err)
		}
		out := req.Clone(req.Context())
		out.URL.Path = strings.TrimSuffix(out.URL.Path, "/chat/completions") + "/responses"
		out.URL.RawPath = ""
		out.Body = io.NopCloser(bytes.NewReader(body))
		out.ContentLength = int64(len(body))
		out.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
		out.Header.Set("Content-Type", "application/json")
		out.Header.Set("Accept", "text/event-stream")

		resp, err := t.forward(out)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return t.translateStream(req, resp, chat)
		}

		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxCapturedBodyBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		apiErr := parseResponsesError(data)
		if attempt+1 < maxResponsesRequestAttempts {
			if resp.StatusCode == http.StatusUnauthorized && !refreshed && t.tokens != nil {
				refreshed = true
				if _, err := t.tokens.ForceRefresh(req.Context()); err == nil {
					continue
				}
			}
			if param := droppableParam(resp.StatusCode, apiErr, dropped); param != "" {
				t.dropParam(param)
				continue
			}
		}
		return errorHTTPResponse(req, resp, apiErr, data), nil
	}
}

// forward sends req upstream with the current access token, replacing the
// static key the SDK attached and any organization/project headers that do
// not apply to a ChatGPT account.
func (t *responsesTransport) forward(req *http.Request) (*http.Response, error) {
	if t.tokens != nil {
		token, err := t.tokens.Token(req.Context())
		if err != nil {
			return nil, err
		}
		if req.Header.Get("Authorization") != "Bearer "+token {
			req = req.Clone(req.Context())
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Del("OpenAI-Organization")
		req.Header.Del("OpenAI-Project")
	}
	return t.base.RoundTrip(req)
}

func droppableParam(status int, apiErr responsesAPIError, dropped map[string]bool) string {
	if status != http.StatusBadRequest && status != http.StatusUnprocessableEntity {
		return ""
	}
	for _, param := range droppableResponsesParams {
		if dropped[param] {
			continue
		}
		if apiErr.Param == param || (apiErr.Param == "" && strings.Contains(apiErr.Message, "'"+param+"'")) {
			return param
		}
	}
	return ""
}

// chatToResponsesRequest maps a Chat Completions request body onto the
// Responses API. Fields the Responses API has no equivalent for (top_k,
// min_p, penalties, provider template kwargs) are dropped: they are
// open-weight sampling knobs that OpenAI's endpoint would reject.
func chatToResponsesRequest(chat map[string]any, dropped map[string]bool) map[string]any {
	out := map[string]any{
		"model":  chat["model"],
		"stream": true,
		"store":  false,
	}

	instructions, input := chatMessagesToResponsesInput(chat["messages"])
	if instructions != "" {
		out["instructions"] = instructions
	}
	out["input"] = input

	if tools := chatToolsToResponsesTools(chat["tools"]); len(tools) > 0 {
		out["tools"] = tools
		if choice := chatToolChoiceToResponses(chat["tool_choice"]); choice != nil {
			out["tool_choice"] = choice
		}
		if parallel, ok := chat["parallel_tool_calls"].(bool); ok {
			out["parallel_tool_calls"] = parallel
		}
	}
	if format := chatResponseFormatToResponses(chat["response_format"]); format != nil {
		out["text"] = map[string]any{"format": format}
	}
	for _, key := range []string{"max_completion_tokens", "max_tokens"} {
		if value, ok := chat[key].(float64); ok && value > 0 {
			out["max_output_tokens"] = int(value)
			break
		}
	}
	for _, key := range []string{"temperature", "top_p"} {
		if value, ok := chat[key]; ok && !dropped[key] {
			out[key] = value
		}
	}
	for _, key := range []string{"metadata", "user", "truncation", "include", "prompt_cache_key"} {
		if value, ok := chat[key]; ok {
			out[key] = value
		}
	}

	reasoning := map[string]any{}
	if effort, ok := chat["reasoning_effort"].(string); ok && effort != "" {
		if effort == "off" {
			effort = "none"
		}
		reasoning["effort"] = effort
	}
	if reasoning["effort"] != "none" && !dropped["reasoning.summary"] {
		// Without a summary the Responses API streams no reasoning text at
		// all, which would leave the reasoning sinks, loop detector, and
		// reasoning budget with nothing to observe.
		reasoning["summary"] = "auto"
	}
	if len(reasoning) > 0 {
		out["reasoning"] = reasoning
	}
	return out
}

func chatMessagesToResponsesInput(raw any) (string, []any) {
	messages, _ := raw.([]any)
	var instructions []string
	input := make([]any, 0, len(messages))
	leading := true
	for _, item := range messages {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		content := chatContentText(msg["content"])
		switch role {
		case "system", "developer":
			if leading {
				if content != "" {
					instructions = append(instructions, content)
				}
				continue
			}
			input = append(input, map[string]any{"role": "developer", "content": content})
		case "assistant":
			leading = false
			if content != "" {
				input = append(input, map[string]any{"role": "assistant", "content": content})
			}
			calls, _ := msg["tool_calls"].([]any)
			for _, rawCall := range calls {
				call, _ := rawCall.(map[string]any)
				fn, _ := call["function"].(map[string]any)
				id, _ := call["id"].(string)
				name, _ := fn["name"].(string)
				arguments, _ := fn["arguments"].(string)
				if id == "" || name == "" {
					continue
				}
				input = append(input, map[string]any{
					"type":      "function_call",
					"call_id":   id,
					"name":      name,
					"arguments": arguments,
				})
			}
		case "tool", "function":
			leading = false
			callID, _ := msg["tool_call_id"].(string)
			input = append(input, map[string]any{
				"type":    "function_call_output",
				"call_id": callID,
				"output":  content,
			})
		default:
			leading = false
			input = append(input, map[string]any{"role": "user", "content": content})
		}
	}
	return strings.Join(instructions, "\n\n"), input
}

func chatContentText(raw any) string {
	switch typed := raw.(type) {
	case string:
		return typed
	case []any:
		var parts []string
		for _, item := range typed {
			part, _ := item.(map[string]any)
			if text, ok := part["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func chatToolsToResponsesTools(raw any) []any {
	tools, _ := raw.([]any)
	out := make([]any, 0, len(tools))
	for _, item := range tools {
		tool, _ := item.(map[string]any)
		fn, _ := tool["function"].(map[string]any)
		name, _ := fn["name"].(string)
		if name == "" {
			continue
		}
		converted := map[string]any{"type": "function", "name": name, "strict": false}
		if description, ok := fn["description"].(string); ok && description != "" {
			converted["description"] = description
		}
		if parameters, ok := fn["parameters"]; ok && parameters != nil {
			converted["parameters"] = parameters
		} else {
			converted["parameters"] = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, converted)
	}
	return out
}

func chatToolChoiceToResponses(raw any) any {
	switch typed := raw.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return typed
	case map[string]any:
		fn, _ := typed["function"].(map[string]any)
		if name, ok := fn["name"].(string); ok && name != "" {
			return map[string]any{"type": "function", "name": name}
		}
	}
	return nil
}

func chatResponseFormatToResponses(raw any) map[string]any {
	format, _ := raw.(map[string]any)
	switch format["type"] {
	case "json_schema":
		spec, _ := format["json_schema"].(map[string]any)
		out := map[string]any{"type": "json_schema", "strict": false}
		if name, ok := spec["name"].(string); ok && name != "" {
			out["name"] = name
		} else {
			out["name"] = "response"
		}
		if schema, ok := spec["schema"]; ok {
			out["schema"] = schema
		}
		if strict, ok := spec["strict"].(bool); ok {
			out["strict"] = strict
		}
		return out
	case "json_object":
		return map[string]any{"type": "json_object"}
	}
	return nil
}

// responsesAPIError is the error object of a Responses API failure, whether it
// came as an HTTP error body, a response.failed event, or an error event.
type responsesAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param"`
	Type    string `json:"type"`
}

func parseResponsesError(body []byte) responsesAPIError {
	var envelope struct {
		Error  json.RawMessage `json:"error"`
		Detail json.RawMessage `json:"detail"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return responsesAPIError{}
	}
	var apiErr responsesAPIError
	if len(envelope.Error) > 0 {
		if json.Unmarshal(envelope.Error, &apiErr) != nil {
			var code string
			if json.Unmarshal(envelope.Error, &code) == nil {
				apiErr.Code = code
			}
		}
	}
	if apiErr.Message == "" && len(envelope.Detail) > 0 {
		var detail string
		if json.Unmarshal(envelope.Detail, &detail) == nil {
			apiErr.Message = detail
		}
	}
	return apiErr
}

// subscriptionErrorStatus maps the Sign in with ChatGPT error codes onto the
// HTTP status the client's retry policy should see. A plan usage limit is
// reported as 403, not the 429 it arrives as: ChatGPT gives no reset time and
// the limit lasts hours, so the rate-limit backoff would only burn the run's
// wait budget before failing anyway.
func subscriptionErrorStatus(code string, fallback int) int {
	code = strings.Replace(code, "subscription_sharing_v2_", "subscription_sharing_", 1)
	switch code {
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
	case "subscription_sharing_usage_unavailable", "subscription_sharing_user_unavailable":
		return http.StatusServiceUnavailable
	case "rate_limit_exceeded":
		return http.StatusTooManyRequests
	}
	return fallback
}

func subscriptionErrorMessage(apiErr responsesAPIError) string {
	message := apiErr.Message
	switch strings.Replace(apiErr.Code, "subscription_sharing_v2_", "subscription_sharing_", 1) {
	case "subscription_sharing_usage_limit_exceeded":
		message = "ChatGPT plan usage limit reached; check https://chatgpt.com/settings/usage"
	case "subscription_sharing_user_not_eligible":
		message = "ChatGPT plan usage is not available for this account or workspace"
	case "subscription_sharing_invalid_user":
		message = "ChatGPT did not accept the signed-in account; run `nickpit chatgpt login` again"
	case "subscription_sharing_unsupported_capability":
		if apiErr.Param != "" {
			message = fmt.Sprintf("ChatGPT plan usage does not support %q in this request", apiErr.Param)
		}
	}
	if message == "" {
		message = apiErr.Code
	}
	// The Responses API names the offending field in param, not the message;
	// keep it visible so callers matching on the field (the reasoning-effort
	// fallback ladder looks for "reasoning") still recognize the rejection.
	if apiErr.Param != "" && !strings.Contains(message, apiErr.Param) {
		message += " (param: " + apiErr.Param + ")"
	}
	if apiErr.Code != "" && !strings.Contains(message, apiErr.Code) {
		message += " (" + apiErr.Code + ")"
	}
	return message
}

func errorHTTPResponse(req *http.Request, upstream *http.Response, apiErr responsesAPIError, raw []byte) *http.Response {
	status := upstream.StatusCode
	body := raw
	if apiErr.Code != "" || apiErr.Message != "" {
		status = subscriptionErrorStatus(apiErr.Code, status)
		body = encodeChatError(apiErr)
	}
	header := upstream.Header.Clone()
	header.Set("Content-Type", "application/json")
	header.Del("Content-Length")
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode:    status,
		Proto:         upstream.Proto,
		ProtoMajor:    upstream.ProtoMajor,
		ProtoMinor:    upstream.ProtoMinor,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}

func encodeChatError(apiErr responsesAPIError) []byte {
	errType := apiErr.Type
	if errType == "" {
		errType = "invalid_request_error"
	}
	payload := map[string]any{
		"message": subscriptionErrorMessage(apiErr),
		"type":    errType,
	}
	if apiErr.Code != "" {
		payload["code"] = apiErr.Code
	}
	if apiErr.Param != "" {
		payload["param"] = apiErr.Param
	}
	data, _ := json.Marshal(map[string]any{"error": payload})
	return data
}

// streamItem is one translated piece of the stream: a chat chunk to send, or
// the failure that ends it. keepAlive marks chunks that carry nothing but
// prove the upstream is alive; they reset the client's idle watchdog.
type streamItem struct {
	data      []byte
	keepAlive bool
	fail      *responsesAPIError
	err       error
}

func (t *responsesTransport) translateStream(req *http.Request, resp *http.Response, chat map[string]any) (*http.Response, error) {
	ctx := req.Context()
	modelName, _ := chat["model"].(string)
	items := make(chan streamItem, 32)
	upstream := resp.Body
	go newResponsesStreamTranslator(modelName).run(upstream, items)

	// Peek for an admission failure before handing the stream over.
	var first []streamItem
	timer := time.NewTimer(responsesPeekWindow)
	defer timer.Stop()
peek:
	for {
		select {
		case item, ok := <-items:
			if !ok {
				break peek
			}
			if item.keepAlive {
				continue
			}
			if item.fail != nil {
				_ = upstream.Close()
				go drainItems(items)
				return errorHTTPResponse(req, resp, *item.fail, nil), nil
			}
			first = append(first, item)
			break peek
		case <-timer.C:
			break peek
		case <-ctx.Done():
			_ = upstream.Close()
			go drainItems(items)
			return nil, ctx.Err()
		}
	}

	reader, writer := io.Pipe()
	go func() {
		write := func(item streamItem) bool {
			if item.err != nil {
				_ = writer.CloseWithError(item.err)
				return false
			}
			if _, err := writer.Write(item.data); err != nil {
				return false
			}
			return true
		}
		ok := true
		for _, item := range first {
			if ok = write(item); !ok {
				break
			}
		}
		for item := range items {
			if ok {
				ok = write(item)
			}
			if !ok {
				_ = upstream.Close()
			}
		}
		_ = writer.Close()
	}()

	header := resp.Header.Clone()
	header.Set("Content-Type", "text/event-stream")
	header.Del("Content-Length")
	return &http.Response{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		Proto:         resp.Proto,
		ProtoMajor:    resp.ProtoMajor,
		ProtoMinor:    resp.ProtoMinor,
		Header:        header,
		Body:          &translatedBody{reader: reader, upstream: upstream},
		ContentLength: -1,
		Request:       req,
	}, nil
}

func drainItems(items <-chan streamItem) {
	for range items {
	}
}

type translatedBody struct {
	reader   *io.PipeReader
	upstream io.Closer
}

func (b *translatedBody) Read(p []byte) (int, error) { return b.reader.Read(p) }

func (b *translatedBody) Close() error {
	_ = b.upstream.Close()
	return b.reader.Close()
}

type responsesStreamTranslator struct {
	id      string
	model   string
	created int64

	// calls maps a function_call item (by item id, then output index) to its
	// chat tool-call index; argsSent records which ones already streamed
	// argument deltas so the .done events do not repeat them.
	callsByItem   map[string]int
	callsByOutput map[int]int
	argsSent      map[int]bool
	callCount     int

	lastSummaryIndex int
	sawSummary       bool
}

func newResponsesStreamTranslator(modelName string) *responsesStreamTranslator {
	return &responsesStreamTranslator{
		id:            "chatcmpl-responses",
		model:         modelName,
		created:       time.Now().Unix(),
		callsByItem:   map[string]int{},
		callsByOutput: map[int]int{},
		argsSent:      map[int]bool{},
	}
}

func (s *responsesStreamTranslator) run(body io.Reader, items chan<- streamItem) {
	defer close(items)
	reader := bufio.NewReaderSize(body, 64*1024)
	var data bytes.Buffer
	for {
		line, err := reader.ReadBytes('\n')
		trimmed := bytes.TrimRight(line, "\r\n")
		if len(trimmed) == 0 && data.Len() > 0 {
			if done := s.dispatch(data.Bytes(), items); done {
				return
			}
			data.Reset()
		} else if payload, ok := bytes.CutPrefix(trimmed, []byte("data:")); ok {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.Write(bytes.TrimPrefix(payload, []byte(" ")))
		}
		if err != nil {
			if data.Len() > 0 {
				if done := s.dispatch(data.Bytes(), items); done {
					return
				}
			}
			if !errors.Is(err, io.EOF) {
				items <- streamItem{err: err}
			}
			// An EOF without a terminal event ends the translated stream without
			// [DONE] or usage, which the client treats as a retryable interruption.
			return
		}
	}
}

type responsesEvent struct {
	Type         string          `json:"type"`
	Delta        string          `json:"delta"`
	ItemID       string          `json:"item_id"`
	OutputIndex  int             `json:"output_index"`
	SummaryIndex int             `json:"summary_index"`
	Arguments    string          `json:"arguments"`
	Item         *responsesItem  `json:"item"`
	Response     *responsesBody  `json:"response"`
	Code         string          `json:"code"`
	Message      string          `json:"message"`
	Param        string          `json:"param"`
	Error        json.RawMessage `json:"error"`
}

type responsesItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type responsesBody struct {
	ID                string             `json:"id"`
	Model             string             `json:"model"`
	Error             *responsesAPIError `json:"error"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Usage *struct {
		InputTokens         int `json:"input_tokens"`
		OutputTokens        int `json:"output_tokens"`
		TotalTokens         int `json:"total_tokens"`
		OutputTokensDetails *struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
	} `json:"usage"`
}

// dispatch translates one Responses event and reports whether it ended the
// stream.
func (s *responsesStreamTranslator) dispatch(data []byte, items chan<- streamItem) bool {
	if string(data) == "[DONE]" {
		return true
	}
	var event responsesEvent
	if err := json.Unmarshal(data, &event); err != nil {
		items <- streamItem{err: fmt.Errorf("llm: decoding responses stream event: %w", err)}
		return true
	}
	if event.Response != nil {
		if event.Response.ID != "" {
			s.id = event.Response.ID
		}
		if event.Response.Model != "" {
			s.model = event.Response.Model
		}
	}

	switch event.Type {
	case "response.output_text.delta", "response.refusal.delta":
		if event.Delta != "" {
			items <- streamItem{data: s.chunk(map[string]any{"content": event.Delta}, nil)}
			return false
		}
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		if event.Delta != "" {
			delta := event.Delta
			if event.Type == "response.reasoning_summary_text.delta" {
				if s.sawSummary && event.SummaryIndex != s.lastSummaryIndex {
					delta = "\n\n" + delta
				}
				s.sawSummary = true
				s.lastSummaryIndex = event.SummaryIndex
			}
			items <- streamItem{data: s.chunk(map[string]any{"reasoning_content": delta}, nil)}
			return false
		}
	case "response.output_item.added":
		if event.Item != nil && event.Item.Type == "function_call" {
			index := s.callCount
			s.callCount++
			if event.Item.ID != "" {
				s.callsByItem[event.Item.ID] = index
			}
			s.callsByOutput[event.OutputIndex] = index
			call := map[string]any{
				"index":    index,
				"id":       event.Item.CallID,
				"type":     "function",
				"function": map[string]any{"name": event.Item.Name, "arguments": event.Item.Arguments},
			}
			if event.Item.Arguments != "" {
				s.argsSent[index] = true
			}
			items <- streamItem{data: s.chunk(map[string]any{"tool_calls": []any{call}}, nil)}
			return false
		}
	case "response.function_call_arguments.delta":
		if index, ok := s.callIndex(event.ItemID, event.OutputIndex); ok && event.Delta != "" {
			s.argsSent[index] = true
			items <- streamItem{data: s.argumentsChunk(index, event.Delta)}
			return false
		}
	case "response.function_call_arguments.done":
		if index, ok := s.callIndex(event.ItemID, event.OutputIndex); ok && !s.argsSent[index] && event.Arguments != "" {
			s.argsSent[index] = true
			items <- streamItem{data: s.argumentsChunk(index, event.Arguments)}
			return false
		}
	case "response.output_item.done":
		if event.Item != nil && event.Item.Type == "function_call" {
			if index, ok := s.callIndex(event.Item.ID, event.OutputIndex); ok && !s.argsSent[index] && event.Item.Arguments != "" {
				s.argsSent[index] = true
				items <- streamItem{data: s.argumentsChunk(index, event.Item.Arguments)}
				return false
			}
		}
	case "response.completed":
		finish := "stop"
		if s.callCount > 0 {
			finish = "tool_calls"
		}
		s.finish(finish, event.Response, items)
		return true
	case "response.incomplete":
		finish := "length"
		if event.Response != nil && event.Response.IncompleteDetails != nil && event.Response.IncompleteDetails.Reason == "content_filter" {
			finish = "content_filter"
		}
		s.finish(finish, event.Response, items)
		return true
	case "response.failed":
		apiErr := responsesAPIError{Code: "response_failed", Message: "the response failed"}
		if event.Response != nil && event.Response.Error != nil {
			apiErr = *event.Response.Error
		}
		items <- streamItem{fail: &apiErr, data: streamErrorLine(apiErr)}
		return true
	case "error":
		apiErr := responsesAPIError{Code: event.Code, Message: event.Message, Param: event.Param}
		if len(event.Error) > 0 {
			var nested responsesAPIError
			if json.Unmarshal(event.Error, &nested) == nil && (nested.Code != "" || nested.Message != "") {
				apiErr = nested
			}
		}
		items <- streamItem{fail: &apiErr, data: streamErrorLine(apiErr)}
		return true
	}
	items <- streamItem{data: s.chunk(nil, nil), keepAlive: true}
	return false
}

func (s *responsesStreamTranslator) callIndex(itemID string, outputIndex int) (int, bool) {
	if itemID != "" {
		if index, ok := s.callsByItem[itemID]; ok {
			return index, true
		}
	}
	index, ok := s.callsByOutput[outputIndex]
	return index, ok
}

func (s *responsesStreamTranslator) argumentsChunk(index int, arguments string) []byte {
	call := map[string]any{"index": index, "function": map[string]any{"arguments": arguments}}
	return s.chunk(map[string]any{"tool_calls": []any{call}}, nil)
}

func (s *responsesStreamTranslator) finish(reason string, body *responsesBody, items chan<- streamItem) {
	items <- streamItem{data: s.chunk(map[string]any{}, &reason)}
	usage := map[string]any{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
	if body != nil && body.Usage != nil {
		usage["prompt_tokens"] = body.Usage.InputTokens
		usage["completion_tokens"] = body.Usage.OutputTokens
		total := body.Usage.TotalTokens
		if total == 0 {
			total = body.Usage.InputTokens + body.Usage.OutputTokens
		}
		usage["total_tokens"] = total
		if body.Usage.OutputTokensDetails != nil {
			usage["completion_tokens_details"] = map[string]any{"reasoning_tokens": body.Usage.OutputTokensDetails.ReasoningTokens}
		}
	}
	items <- streamItem{data: s.encode(map[string]any{
		"id":      s.id,
		"object":  "chat.completion.chunk",
		"created": s.created,
		"model":   s.model,
		"choices": []any{},
		"usage":   usage,
	})}
	items <- streamItem{data: []byte("data: [DONE]\n\n")}
}

// chunk renders a chat.completion.chunk with one choice carrying delta. A nil
// delta yields a choice-less keep-alive chunk.
func (s *responsesStreamTranslator) chunk(delta map[string]any, finishReason *string) []byte {
	choices := []any{}
	if delta != nil {
		choice := map[string]any{"index": 0, "delta": delta, "finish_reason": nil}
		if finishReason != nil {
			choice["finish_reason"] = *finishReason
		}
		choices = append(choices, choice)
	}
	return s.encode(map[string]any{
		"id":      s.id,
		"object":  "chat.completion.chunk",
		"created": s.created,
		"model":   s.model,
		"choices": choices,
	})
}

func (s *responsesStreamTranslator) encode(payload map[string]any) []byte {
	data, _ := json.Marshal(payload)
	return append(append([]byte("data: "), data...), '\n', '\n')
}

// streamErrorLine renders a mid-stream failure the way the chat SDK expects
// one: a data line holding an error object, which it surfaces as an API error.
func streamErrorLine(apiErr responsesAPIError) []byte {
	return append(append([]byte("data: "), encodeChatError(apiErr)...), '\n', '\n')
}
