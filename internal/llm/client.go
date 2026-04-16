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

	"github.com/dgrieser/nickpit/internal/debuglog"
	"github.com/dgrieser/nickpit/internal/model"
	openai "github.com/sashabaranov/go-openai"
)

type Client interface {
	Review(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error)
}

type OpenAIClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	sdkClient  *openai.Client
	retrier    *Retrier
	logger     *debuglog.Logger
	transport  *capturingTransport
}

type ReviewRequest struct {
	SystemPrompt    string
	UserContent     string
	Schema          json.RawMessage
	Model           string
	MaxTokens       int
	Temperature     float64
	ReasoningEffort string
}

type ReviewResponse struct {
	Findings               []model.Finding         `json:"findings"`
	FollowUpRequests       []model.FollowUpRequest `json:"follow_up_requests,omitempty"`
	OverallCorrectness     string                  `json:"overall_correctness"`
	OverallExplanation     string                  `json:"overall_explanation"`
	OverallConfidenceScore float64                 `json:"overall_confidence_score"`
	RawResponse            string                  `json:"raw_response,omitempty"`
	TokensUsed             model.TokenUsage        `json:"tokens_used"`
}

type capture struct {
	status string
	code   int
	header http.Header
	body   []byte
}

type streamedResponse struct {
	content  string
	usage    model.TokenUsage
	reasoned bool
}

type streamReadError struct {
	err       error
	retryable bool
}

func (e *streamReadError) Error() string {
	return e.err.Error()
}

func (e *streamReadError) Unwrap() error {
	return e.err
}

func NewOpenAIClient(baseURL, apiKey, model string) *OpenAIClient {
	transport := &capturingTransport{base: http.DefaultTransport}
	httpClient := &http.Client{
		Timeout:   90 * time.Second,
		Transport: transport,
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = strings.TrimRight(baseURL, "/")
	config.HTTPClient = httpClient

	return &OpenAIClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: httpClient,
		sdkClient:  openai.NewClientWithConfig(config),
		retrier:    NewRetrier(),
		transport:  transport,
	}
}

func (c *OpenAIClient) SetLogger(logger *debuglog.Logger) {
	c.logger = logger
}

func (c *OpenAIClient) Review(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("llm: nil review request")
	}

	payload := openai.ChatCompletionRequest{
		Model: req.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: req.SystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: req.UserContent},
		},
		MaxTokens:   req.MaxTokens,
		Temperature: float32(req.Temperature),
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}
	if payload.Model == "" {
		payload.Model = c.model
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
		"LLM request prepared: model=%s endpoint=%s max_tokens=%d temperature=%.2f reasoning_effort=%s stream=%t",
		payload.Model,
		c.baseURL+"/chat/completions",
		payload.MaxTokens,
		payload.Temperature,
		payload.ReasoningEffort,
		true,
	)
	c.logJSON("LLM request payload:", payload)

	streamed, err := c.reviewStream(ctx, payload)
	if err != nil {
		return nil, err
	}
	c.logBlock("LLM raw model response:", streamed.content)

	resp, err := parseReviewResponse(streamed.content)
	if err != nil {
		resp = invalidJSONFallback(streamed.content)
	}

	resp.RawResponse = streamed.content
	resp.TokensUsed = streamed.usage
	c.logf(
		"Parsed LLM response: findings=%d follow_up=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d",
		len(resp.Findings),
		len(resp.FollowUpRequests),
		resp.TokensUsed.PromptTokens,
		resp.TokensUsed.CompletionTokens,
		resp.TokensUsed.TotalTokens,
	)
	return resp, nil
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
			c.logf("LLM request failed: attempt=%d error=%v", attempt+1, err)
			if status := statusCodeFromError(err); status > 0 {
				if capture != nil && len(capture.body) > 0 {
					c.logMaybeJSON("LLM raw response body:", capture.body)
				}
				if attempt >= c.retrier.MaxRetries || !c.retrier.ShouldRetry(status) {
					return nil, fmt.Errorf("llm: api returned status %d: %w", status, err)
				}
				c.logf("Retrying request: status=%d", status)
				if waitErr := c.retrier.Wait(ctx, attempt, responseFromCapture(capture)); waitErr != nil {
					return nil, fmt.Errorf("llm: retry canceled: %w", waitErr)
				}
				continue
			}
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

func (c *OpenAIClient) collectStream(stream *openai.ChatCompletionStream) (*streamedResponse, error) {
	var (
		contentBuilder   strings.Builder
		usage            model.TokenUsage
		reasoningStarted bool
		sawUsage         bool
	)

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
					content:  contentBuilder.String(),
					usage:    usage,
					reasoned: reasoningStarted,
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
		for _, choice := range chunk.Choices {
			if choice.Index != 0 {
				continue
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
			}
		}
	}
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
					AbsoluteFilePath: "",
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

	if strings.Contains(req.Header.Get("Accept"), "text/event-stream") {
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

func statusCodeFromError(err error) int {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatusCode
	}

	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) {
		return reqErr.HTTPStatusCode
	}

	return 0
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

func (c *OpenAIClient) logBlock(label, content string) {
	if c.logger != nil {
		c.logger.PrintBlock(label, content)
	}
}

func (c *OpenAIClient) logJSON(label string, value any) {
	if c.logger != nil {
		c.logger.PrintJSON(label, value)
	}
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
