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
	Findings         []model.Finding         `json:"findings"`
	FollowUpRequests []model.FollowUpRequest `json:"follow_up_requests,omitempty"`
	Summary          string                  `json:"summary"`
	RawResponse      string                  `json:"raw_response,omitempty"`
	TokensUsed       model.TokenUsage        `json:"tokens_used"`
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

	var httpResp *http.Response
	var responseBody []byte
	for attempt := 0; ; attempt++ {
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
		if httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {
			break
		}
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
					ID:          "parse_error",
					Severity:    model.SeverityWarning,
					Category:    "parse",
					FilePath:    "",
					Title:       "Failed to parse model response",
					Description: strings.TrimSpace(content),
					Confidence:  0.2,
				},
			},
			Summary:     "Model response could not be parsed as structured JSON.",
			RawResponse: content,
		}
	}
	resp.RawResponse = content
	resp.TokensUsed = model.TokenUsage{
		PromptTokens:     envelope.Usage.PromptTokens,
		CompletionTokens: envelope.Usage.CompletionTokens,
		TotalTokens:      envelope.Usage.TotalTokens,
	}
	return resp, nil
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
