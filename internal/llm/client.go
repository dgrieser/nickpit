package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	openai "github.com/sashabaranov/go-openai"
)

type Client interface {
	Review(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error)
}

type OpenAIClient struct {
	baseURL            string
	apiKey             string
	model              string
	emptyMessagesLimit uint
	httpClient         *http.Client
	sdkClient          *openai.Client
	retrier            *Retrier
	logger             *logging.Logger
	transport          *capturingTransport
}

type ReviewRequest struct {
	SystemPrompt      string
	UserContent       string
	Messages          []Message
	Tools             []ToolDefinition
	Schema            json.RawMessage
	Model             string
	MaxTokens         *int
	Temperature       *float64
	ParallelToolCalls bool
	ReasoningEffort   string
}

type Message struct {
	Role       string
	Content    string
	Name       string
	ToolCallID string
	ToolCalls  []ToolCall
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type ReviewResponse struct {
	Findings               []model.Finding  `json:"findings"`
	OverallCorrectness     string           `json:"overall_correctness"`
	OverallExplanation     string           `json:"overall_explanation"`
	OverallConfidenceScore float64          `json:"overall_confidence_score"`
	ToolCalls              []ToolCall       `json:"tool_calls,omitempty"`
	RawResponse            string           `json:"raw_response,omitempty"`
	TokensUsed             model.TokenUsage `json:"tokens_used"`
}

type capture struct {
	status string
	code   int
	header http.Header
	body   []byte
}

type streamedResponse struct {
	content          string
	toolCalls        []ToolCall
	usage            model.TokenUsage
	reasoned         bool
	sawContent       bool
	sawToolCalls     bool
	sawUsage         bool
	lastFinishReason string
}

type toolCallBuilder struct {
	id        string
	name      string
	arguments strings.Builder
}

type streamReadError struct {
	err       error
	retryable bool
}

type llmHTTPStatusError struct {
	statusCode int
	status     string
	message    string
	cause      error
}

func (e *streamReadError) Error() string {
	return e.err.Error()
}

func (e *streamReadError) Unwrap() error {
	return e.err
}

func (e *llmHTTPStatusError) Error() string {
	status := formatHTTPStatus(e.statusCode, e.status)
	if e.message == "" {
		return fmt.Sprintf("llm: api returned %s", status)
	}
	return fmt.Sprintf("llm: api returned %s: %s", status, e.message)
}

func (e *llmHTTPStatusError) Unwrap() error {
	return e.cause
}

func NewOpenAIClient(baseURL, apiKey, model string) *OpenAIClient {
	transport := &capturingTransport{base: http.DefaultTransport}
	httpClient := &http.Client{
		Transport: transport,
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = strings.TrimRight(baseURL, "/")
	config.HTTPClient = httpClient
	config.EmptyMessagesLimit = 100000

	return &OpenAIClient{
		baseURL:            strings.TrimRight(baseURL, "/"),
		apiKey:             apiKey,
		model:              model,
		emptyMessagesLimit: config.EmptyMessagesLimit,
		httpClient:         httpClient,
		sdkClient:          openai.NewClientWithConfig(config),
		retrier:            NewRetrier(),
		transport:          transport,
	}
}

func (c *OpenAIClient) SetLogger(logger *logging.Logger) {
	c.logger = logger
}

func (c *OpenAIClient) Review(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("llm: nil review request")
	}

	payload := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: buildMessages(req),
		Tools:    buildTools(req.Tools),
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}
	if len(req.Tools) > 0 {
		payload.ParallelToolCalls = req.ParallelToolCalls
	}
	if payload.Model == "" {
		payload.Model = c.model
	}
	maxTokensLog := "unset"
	if req.MaxTokens != nil {
		payload.MaxTokens = *req.MaxTokens
		maxTokensLog = fmt.Sprintf("%d", *req.MaxTokens)
	}
	temperatureLog := "unset"
	if req.Temperature != nil {
		payload.Temperature = float32(*req.Temperature)
		temperatureLog = fmt.Sprintf("%.2f", *req.Temperature)
	}
	if req.ReasoningEffort != "" {
		payload.ReasoningEffort = req.ReasoningEffort
	}
	if len(req.Schema) > 0 {
		payload.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:   "review_response",
				Schema: json.RawMessage(req.Schema),
				Strict: true,
			},
		}
	}

	if _, err := json.Marshal(payload); err != nil {
		return nil, fmt.Errorf("llm: encoding request: %w", err)
	}

	c.logf(
		"LLM request prepared: model=%s endpoint=%s max_tokens=%s temperature=%s reasoning_effort=%s stream=%t messages=%d tools=%d",
		payload.Model,
		c.baseURL+"/chat/completions",
		maxTokensLog,
		temperatureLog,
		payload.ReasoningEffort,
		true,
		len(payload.Messages),
		len(payload.Tools),
	)
	c.logHighlightedJSON("LLM request payload:", payload)

	streamed, err := c.reviewStream(ctx, payload)
	if err != nil {
		return nil, err
	}
	c.logf(
		"LLM stream summary: content_chunks=%t tool_calls=%t reasoning_chunks=%t usage_chunk=%t last_finish_reason=%q raw_response_bytes=%d",
		streamed.sawContent,
		streamed.sawToolCalls,
		streamed.reasoned,
		streamed.sawUsage,
		streamed.lastFinishReason,
		len(streamed.content),
	)
	c.logRawModelResponse(streamed)

	var resp *ReviewResponse
	if len(streamed.toolCalls) > 0 {
		resp = &ReviewResponse{ToolCalls: streamed.toolCalls}
	} else {
		var err error
		resp, err = parseReviewResponse(streamed.content)
		if err != nil {
			resp = invalidJSONFallback(streamed.content)
		}
	}
	resp.RawResponse = streamed.content
	resp.TokensUsed = streamed.usage
	c.logf(
		"Parsed LLM response: findings=%d tool_calls=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d",
		len(resp.Findings),
		len(resp.ToolCalls),
		resp.TokensUsed.PromptTokens,
		resp.TokensUsed.CompletionTokens,
		resp.TokensUsed.TotalTokens,
	)
	return resp, nil
}

func buildMessages(req *ReviewRequest) []openai.ChatCompletionMessage {
	if len(req.Messages) > 0 {
		messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages))
		for _, msg := range req.Messages {
			messages = append(messages, toOpenAIMessage(msg))
		}
		return messages
	}
	return []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: req.SystemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: req.UserContent},
	}
}

func buildTools(tools []ToolDefinition) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}
	converted := make([]openai.Tool, 0, len(tools))
	for _, tool := range tools {
		converted = append(converted, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
				Strict:      true,
			},
		})
	}
	return converted
}

func toOpenAIMessage(msg Message) openai.ChatCompletionMessage {
	converted := openai.ChatCompletionMessage{
		Role:       msg.Role,
		Content:    msg.Content,
		Name:       msg.Name,
		ToolCallID: msg.ToolCallID,
	}
	if len(msg.ToolCalls) > 0 {
		converted.ToolCalls = make([]openai.ToolCall, 0, len(msg.ToolCalls))
		for _, call := range msg.ToolCalls {
			converted.ToolCalls = append(converted.ToolCalls, openai.ToolCall{
				ID:   call.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      call.Name,
					Arguments: call.Arguments,
				},
			})
		}
	}
	return converted
}

func (c *OpenAIClient) reviewStream(ctx context.Context, payload openai.ChatCompletionRequest) (*streamedResponse, error) {
	for attempt := 0; ; attempt++ {
		c.logf("Sending LLM request: attempt=%d", attempt+1)
		c.transport.reset()

		stream, err := c.sdkClient.CreateChatCompletionStream(ctx, payload)
		capture := c.transport.snapshot()
		if capture != nil && capture.code != 0 {
			c.logf("LLM stream opened: status=%s", capture.status)
		}
		if err != nil {
			if status := statusCodeFromError(err, capture); status > 0 {
				statusErr := newLLMHTTPStatusError(err, capture)
				c.logf("LLM request failed: attempt=%d error=%v", attempt+1, statusErr)
				if body := httpErrorBody(err, capture); len(body) > 0 {
					c.logMaybeJSON("LLM raw response body:", body)
				}
				if !c.shouldRetryHTTPStatus(status, attempt) {
					return nil, statusErr
				}
				resp := responseFromCapture(capture)
				waitFor := c.retrier.Backoff(attempt, resp)
				c.logRetryHTTPStatus(status, attempt+1, waitFor)
				c.logf("Retrying request: status=%d backoff=%s", status, waitFor)
				if waitErr := c.retrier.Wait(ctx, attempt, resp); waitErr != nil {
					return nil, fmt.Errorf("llm: retry canceled: %w", waitErr)
				}
				continue
			}
			c.logf("LLM request failed: attempt=%d error=%v", attempt+1, err)
			if !isRetryableNetworkError(err) || attempt >= c.retrier.MaxRetries {
				return nil, fmt.Errorf("llm: request failed: %w", err)
			}
			if c.logger != nil {
				c.logger.PrintStatusLine("LLM request hit a network error, retrying...")
			}
			if waitErr := c.retrier.Wait(ctx, attempt, nil); waitErr != nil {
				return nil, fmt.Errorf("llm: request canceled: %w", waitErr)
			}
			continue
		}

		resp, streamErr := c.collectStream(stream)
		closeErr := stream.Close()
		if streamErr != nil {
			if closeErr != nil {
				c.logf("LLM stream close failed after error: %v", closeErr)
			}
			var readErr *streamReadError
			if errors.As(streamErr, &readErr) && readErr.retryable && attempt < c.retrier.MaxRetries {
				if c.logger != nil {
					c.logger.PrintStatusLine("LLM stream hit a network error, retrying...")
				}
				c.logf("Retrying request: stream network error")
				if waitErr := c.retrier.Wait(ctx, attempt, nil); waitErr != nil {
					return nil, fmt.Errorf("llm: retry canceled: %w", waitErr)
				}
				continue
			}
			return nil, streamErr
		}
		if closeErr != nil {
			return nil, fmt.Errorf("llm: closing stream: %w", closeErr)
		}
		return resp, nil
	}
}

func (c *OpenAIClient) shouldRetryHTTPStatus(status, attempt int) bool {
	if !c.retrier.ShouldRetry(status) {
		return false
	}
	if status == http.StatusTooManyRequests {
		return true
	}
	return attempt < c.retrier.MaxRetries
}

func (c *OpenAIClient) logRetryHTTPStatus(status, currentAttempt int, waitFor time.Duration) {
	if c.logger == nil {
		return
	}
	if status == http.StatusTooManyRequests {
		c.logger.PrintProgress("Model", fmt.Sprintf("rate limited (429), waiting %s before retry attempt %d", waitFor, currentAttempt+1))
		return
	}
	c.logger.PrintStatusLine(fmt.Sprintf("LLM request failed with status %d, retrying in %s...", status, waitFor))
}

func (c *OpenAIClient) collectStream(stream *openai.ChatCompletionStream) (*streamedResponse, error) {
	var (
		contentBuilder   strings.Builder
		toolCalls        []*toolCallBuilder
		usage            model.TokenUsage
		reasoningStarted bool
		sawUsage         bool
		sawContent       bool
		sawToolCalls     bool
		lastFinishReason string
		receivedChunk    bool
	)
	c.logf("LLM waiting for first stream chunk")

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if !sawUsage {
					if reasoningStarted {
						c.logger.PrintBlankLine()
					}
					return nil, &streamReadError{
						err:       fmt.Errorf("llm: reading stream: interrupted before final usage chunk"),
						retryable: true,
					}
				}
				if reasoningStarted {
					c.logger.PrintBlankLine()
				}
				return &streamedResponse{
					content:          contentBuilder.String(),
					toolCalls:        finalizeToolCalls(toolCalls),
					usage:            usage,
					reasoned:         reasoningStarted,
					sawContent:       sawContent,
					sawToolCalls:     sawToolCalls,
					sawUsage:         sawUsage,
					lastFinishReason: lastFinishReason,
				}, nil
			}
			if reasoningStarted {
				c.logger.PrintBlankLine()
			}
			return nil, &streamReadError{
				err:       fmt.Errorf("llm: reading stream: %w", err),
				retryable: isRetryableNetworkError(err),
			}
		}

		if chunk.Usage != nil {
			sawUsage = true
			usage = model.TokenUsage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}
		if !receivedChunk {
			receivedChunk = true
			c.logf("LLM first stream chunk received")
		}

		for _, choice := range chunk.Choices {
			if choice.Index != 0 {
				continue
			}
			if choice.FinishReason != "" {
				lastFinishReason = string(choice.FinishReason)
			}
			if choice.Delta.ReasoningContent != "" {

				if !reasoningStarted {
					reasoningStarted = true
					if c.logger != nil {
						c.logger.PrintReasoningBanner()
					}
				}
				if c.logger != nil {
					c.logger.PrintReasoningDelta(choice.Delta.ReasoningContent)
				}
			}
			if choice.Delta.Content != "" {
				contentBuilder.WriteString(choice.Delta.Content)
				sawContent = true
			}
			if len(choice.Delta.ToolCalls) > 0 {
				sawToolCalls = true
				mergeToolCallDeltas(&toolCalls, choice.Delta.ToolCalls)
			}
		}
	}
}

func mergeToolCallDeltas(builders *[]*toolCallBuilder, deltas []openai.ToolCall) {
	for _, delta := range deltas {
		index := 0
		if delta.Index != nil && *delta.Index >= 0 {
			index = *delta.Index
		}
		for len(*builders) <= index {
			*builders = append(*builders, nil)
		}
		if (*builders)[index] == nil {
			(*builders)[index] = &toolCallBuilder{}
		}
		builder := (*builders)[index]
		if delta.ID != "" {
			builder.id = delta.ID
		}
		if delta.Function.Name != "" {
			builder.name = delta.Function.Name
		}
		if delta.Function.Arguments != "" {
			builder.arguments.WriteString(delta.Function.Arguments)
		}
	}
}

func finalizeToolCalls(builders []*toolCallBuilder) []ToolCall {
	if len(builders) == 0 {
		return nil
	}
	toolCalls := make([]ToolCall, 0, len(builders))
	for _, builder := range builders {
		if builder == nil {
			continue
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:        builder.id,
			Name:      builder.name,
			Arguments: builder.arguments.String(),
		})
	}
	return toolCalls
}

func invalidJSONFallback(content string) *ReviewResponse {
	return &ReviewResponse{
		Findings: []model.Finding{
			{
				Title:           "[P2] Return valid review JSON",
				Body:            "The model response could not be parsed as the configured review schema, so the review output is unusable until the prompt or schema alignment is fixed.",
				ConfidenceScore: 0.2,
				Priority:        intPtr(2),
				CodeLocation: model.CodeLocation{
					FilePath: "",
					LineRange: model.LineRange{
						Start: 1,
						End:   1,
					},
				},
			},
		},
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     strings.TrimSpace(content),
		OverallConfidenceScore: 0.2,
		RawResponse:            content,
	}
}

type capturingTransport struct {
	base http.RoundTripper
	last *capture
}

func (t *capturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		t.last = nil
		return nil, err
	}

	captured := &capture{
		status: resp.Status,
		code:   resp.StatusCode,
		header: resp.Header.Clone(),
	}

	if strings.Contains(req.Header.Get("Accept"), "text/event-stream") &&
		resp.StatusCode >= http.StatusOK &&
		resp.StatusCode < http.StatusBadRequest {
		t.last = captured
		return resp, nil
	}

	data, readErr := readAndRestoreBody(resp)
	captured.body = data
	t.last = captured
	if readErr != nil {
		return nil, readErr
	}

	return resp, nil
}

func (t *capturingTransport) reset() {
	t.last = nil
}

func (t *capturingTransport) snapshot() *capture {
	if t.last == nil {
		return nil
	}

	cloned := *t.last
	if cloned.header != nil {
		cloned.header = cloned.header.Clone()
	}
	if cloned.body != nil {
		cloned.body = append([]byte(nil), cloned.body...)
	}
	return &cloned
}

func readAndRestoreBody(resp *http.Response) ([]byte, error) {
	if resp.Body == nil {
		return nil, nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	if err := resp.Body.Close(); err != nil {
		return nil, err
	}

	resp.Body = io.NopCloser(bytes.NewReader(data))
	return data, nil
}

func responseFromCapture(c *capture) *http.Response {
	if c == nil || c.code == 0 {
		return nil
	}
	return &http.Response{
		Status:     c.status,
		StatusCode: c.code,
		Header:     c.header.Clone(),
	}
}

func statusCodeFromError(err error, c *capture) int {
	var statusErr *llmHTTPStatusError
	if errors.As(err, &statusErr) {
		if statusErr.statusCode > 0 {
			return statusErr.statusCode
		}
	}

	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		if apiErr.HTTPStatusCode > 0 {
			return apiErr.HTTPStatusCode
		}
	}

	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) {
		if reqErr.HTTPStatusCode > 0 {
			return reqErr.HTTPStatusCode
		}
	}

	if c != nil {
		return c.code
	}

	return 0
}

func newLLMHTTPStatusError(err error, c *capture) *llmHTTPStatusError {
	statusCode := statusCodeFromError(err, c)
	status := ""
	if c != nil {
		status = c.status
	}
	message := ""

	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		if apiErr.HTTPStatus != "" {
			status = apiErr.HTTPStatus
		}
		if apiErr.HTTPStatusCode > 0 {
			statusCode = apiErr.HTTPStatusCode
		}
		message = apiErr.Message
	}

	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) {
		if reqErr.HTTPStatus != "" {
			status = reqErr.HTTPStatus
		}
		if reqErr.HTTPStatusCode > 0 {
			statusCode = reqErr.HTTPStatusCode
		}
		if message == "" {
			message = providerErrorMessage(reqErr.Body)
		}
		if message == "" {
			message = cleanHTTPErrorText(string(reqErr.Body))
		}
		if message == "" && reqErr.Err != nil {
			message = cleanHTTPErrorText(reqErr.Err.Error())
		}
	}

	if message == "" && c != nil {
		message = providerErrorMessage(c.body)
		if message == "" {
			message = cleanHTTPErrorText(string(c.body))
		}
	}
	message = cleanHTTPErrorText(message)

	return &llmHTTPStatusError{
		statusCode: statusCode,
		status:     status,
		message:    message,
		cause:      err,
	}
}

func httpErrorBody(err error, c *capture) []byte {
	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) && len(reqErr.Body) > 0 {
		return reqErr.Body
	}
	if c != nil && len(c.body) > 0 {
		return c.body
	}
	return nil
}

func formatHTTPStatus(code int, status string) string {
	status = strings.TrimSpace(status)
	if status != "" {
		return status
	}
	if code <= 0 {
		return "unknown status"
	}
	if text := http.StatusText(code); text != "" {
		return fmt.Sprintf("%d %s", code, text)
	}
	return fmt.Sprintf("%d", code)
}

func providerErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return ""
	}
	return cleanHTTPErrorText(providerErrorMessageValue(value))
}

func providerErrorMessageValue(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"message", "detail", "error_description", "error"} {
			if message := providerErrorMessageValue(typed[key]); message != "" {
				return message
			}
		}
	case []any:
		var parts []string
		for _, item := range typed {
			if message := providerErrorMessageValue(item); message != "" {
				parts = append(parts, message)
			}
		}
		return strings.Join(parts, ", ")
	case string:
		return typed
	}
	return ""
}

func cleanHTTPErrorText(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	const maxErrorTextRunes = 1024
	runes := []rune(text)
	if len(runes) > maxErrorTextRunes {
		return string(runes[:maxErrorTextRunes]) + "..."
	}
	return text
}

func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isRetryableNetworkError(urlErr.Err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	return false
}

func intPtr(v int) *int {
	return &v
}

func (c *OpenAIClient) logf(format string, args ...any) {
	if c.logger != nil {
		c.logger.Printf(format, args...)
	}
}

func (c *OpenAIClient) logRawModelResponse(streamed *streamedResponse) {
	if c.logger == nil || streamed == nil {
		return
	}
	c.logHighlightedJSON("LLM raw model response:", rawModelResponseForLog(streamed))
}

func (c *OpenAIClient) logJSON(label string, value any) {
	if c.logger != nil {
		c.logger.PrintJSON(label, value)
	}
}

func (c *OpenAIClient) logHighlightedJSON(label string, value any) {
	if c.logger == nil {
		return
	}
	c.logger.Printf("%s", label)
	c.logger.PrintJSON("", value)
}

func (c *OpenAIClient) logMaybeJSON(label string, data []byte) {
	if c.logger == nil {
		return
	}
	var value any
	if err := json.Unmarshal(data, &value); err == nil {
		c.logger.PrintJSON(label, value)
		return
	}
	c.logger.PrintBlock(label, string(data))
}

func rawModelResponseForLog(streamed *streamedResponse) any {
	if streamed == nil {
		return nil
	}
	return struct {
		Content          string           `json:"content"`
		ToolCalls        []ToolCall       `json:"tool_calls,omitempty"`
		Usage            model.TokenUsage `json:"usage"`
		Reasoned         bool             `json:"reasoned"`
		SawContent       bool             `json:"saw_content"`
		SawToolCalls     bool             `json:"saw_tool_calls"`
		SawUsage         bool             `json:"saw_usage"`
		LastFinishReason string           `json:"last_finish_reason,omitempty"`
	}{
		Content:          streamed.content,
		ToolCalls:        streamed.toolCalls,
		Usage:            streamed.usage,
		Reasoned:         streamed.reasoned,
		SawContent:       streamed.sawContent,
		SawToolCalls:     streamed.sawToolCalls,
		SawUsage:         streamed.sawUsage,
		LastFinishReason: streamed.lastFinishReason,
	}
}

func parseReviewResponse(content string) (*ReviewResponse, error) {
	var parsed ReviewResponse
	if err := json.Unmarshal([]byte(content), &parsed); err == nil {
		return &parsed, nil
	}
	re := regexp.MustCompile("(?s)```json\\s*(\\{.*\\})\\s*```")
	matches := re.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid JSON response")
	}
	if err := json.Unmarshal([]byte(matches[1]), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}
