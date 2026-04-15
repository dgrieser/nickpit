package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/debuglog"
	"github.com/dgrieser/nickpit/internal/model"
)

type Client interface {
	Review(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error)
}

type OpenAIClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	retrier    *Retrier
	logger     *debuglog.Logger
}

type ReviewRequest struct {
	SystemPrompt string
	UserContent  string
	Schema       json.RawMessage
	Model        string
	MaxTokens    int
	Temperature  float64
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

type chatCompletionRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
	Temperature    float64        `json:"temperature,omitempty"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func NewOpenAIClient(baseURL, apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		retrier: NewRetrier(),
	}
}

func (c *OpenAIClient) SetLogger(logger *debuglog.Logger) {
	c.logger = logger
}

func (c *OpenAIClient) Review(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("llm: nil review request")
	}

	payload := chatCompletionRequest{
		Model: req.Model,
		Messages: []chatMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserContent},
		},
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		ResponseFormat: map[string]any{
			"type": "json_object",
		},
	}
	if payload.Model == "" {
		payload.Model = c.model
	}
	if len(req.Schema) > 0 {
		payload.ResponseFormat = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "review_response",
				"schema": json.RawMessage(req.Schema),
			},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("llm: encoding request: %w", err)
	}
	c.logf("LLM request prepared: model=%s endpoint=%s max_tokens=%d temperature=%.2f", payload.Model, c.baseURL+"/chat/completions", payload.MaxTokens, payload.Temperature)
	c.logJSON("LLM request payload:", payload)

	var httpResp *http.Response
	var responseBody []byte
	for attempt := 0; ; attempt++ {
		c.logf("Sending LLM request: attempt=%d", attempt+1)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("llm: building request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		httpResp, err = c.httpClient.Do(httpReq)
		if err != nil {
			c.logf("LLM request failed: attempt=%d error=%v", attempt+1, err)
			if attempt >= c.retrier.MaxRetries {
				return nil, fmt.Errorf("llm: request failed: %w", err)
			}
			if waitErr := c.retrier.Wait(ctx, attempt, nil); waitErr != nil {
				return nil, fmt.Errorf("llm: request canceled: %w", waitErr)
			}
			continue
		}

		responseBody, err = io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("llm: reading response: %w", err)
		}
		c.logf("LLM response received: status=%s", httpResp.Status)
		c.logMaybeJSON("LLM raw response body:", responseBody)
		if httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {
			break
		}
		c.logf("Retrying request: status=%d", httpResp.StatusCode)
		if attempt >= c.retrier.MaxRetries || !c.retrier.ShouldRetry(httpResp.StatusCode) {
			return nil, fmt.Errorf("llm: api returned status %d", httpResp.StatusCode)
		}
		if waitErr := c.retrier.Wait(ctx, attempt, httpResp); waitErr != nil {
			return nil, fmt.Errorf("llm: retry canceled: %w", waitErr)
		}
	}

	var envelope chatCompletionResponse
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return nil, fmt.Errorf("llm: parsing response envelope: %w", err)
	}
	if len(envelope.Choices) == 0 {
		return nil, fmt.Errorf("llm: response contained no choices")
	}

	content := envelope.Choices[0].Message.Content
	resp, err := parseReviewResponse(content)
	if err != nil {
		resp = &ReviewResponse{
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
	resp.RawResponse = content
	resp.TokensUsed = model.TokenUsage{
		PromptTokens:     envelope.Usage.PromptTokens,
		CompletionTokens: envelope.Usage.CompletionTokens,
		TotalTokens:      envelope.Usage.TotalTokens,
	}
	c.logf("Parsed LLM response: findings=%d follow_up=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d",
		len(resp.Findings), len(resp.FollowUpRequests), resp.TokensUsed.PromptTokens, resp.TokensUsed.CompletionTokens, resp.TokensUsed.TotalTokens)
	return resp, nil
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
