package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/logging"
	openai "github.com/sashabaranov/go-openai"
)

func TestClientReview(t *testing.T) {
	var payload map[string]any
	var path string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[{"title":"[P2] Flag issue","body":"Something is wrong","confidence_score":0.9,"priority":2,"code_location":{"file_path":"main.go","line_range":{"start":10,"end":10}}}],"overall_correctness":"patch is incorrect",`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `"overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-3",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	maxTokens := 10
	temperature := 0.25
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user",
		MaxTokens:       &maxTokens,
		Temperature:     &temperature,
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/chat/completions" {
		t.Fatalf("path = %q", path)
	}
	if len(resp.Findings) != 1 {
		t.Fatalf("findings = %d", len(resp.Findings))
	}
	if resp.TokensUsed.TotalTokens != 15 {
		t.Fatalf("total tokens = %d", resp.TokensUsed.TotalTokens)
	}
	if got := payload["reasoning_effort"]; got != "high" {
		t.Fatalf("reasoning_effort = %v", got)
	}
	if got := payload["stream"]; got != true {
		t.Fatalf("stream = %v", got)
	}
	streamOptions, ok := payload["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("stream_options = %#v", payload["stream_options"])
	}
	if streamOptions["include_usage"] != true {
		t.Fatalf("include_usage = %v", streamOptions["include_usage"])
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("response_format should be omitted, payload=%#v", payload)
	}
}

func TestClientReviewOmitsMaxTokensWhenUnset(t *testing.T) {
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["max_tokens"]; ok {
		t.Fatalf("max_tokens should be omitted, payload=%#v", payload)
	}
}

func TestClientReviewOmitsTemperatureWhenUnset(t *testing.T) {
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["temperature"]; ok {
		t.Fatalf("temperature should be omitted, payload=%#v", payload)
	}
}

func TestClientReviewIncludesTopPAndExtraBody(t *testing.T) {
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	topP := 0.9
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
		TopP:         &topP,
		ExtraBody: map[string]any{
			"chat_template_kwargs": map[string]any{
				"enable_thinking": true,
				"clear_thinking":  false,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := payload["top_p"]; got != 0.9 {
		t.Fatalf("top_p = %v", got)
	}
	chatTemplateKwargs, ok := payload["chat_template_kwargs"].(map[string]any)
	if !ok {
		t.Fatalf("chat_template_kwargs = %#v", payload["chat_template_kwargs"])
	}
	if chatTemplateKwargs["enable_thinking"] != true {
		t.Fatalf("enable_thinking = %v", chatTemplateKwargs["enable_thinking"])
	}
	if chatTemplateKwargs["clear_thinking"] != false {
		t.Fatalf("clear_thinking = %v", chatTemplateKwargs["clear_thinking"])
	}
}

func TestNewOpenAIClientDisablesHTTPClientTimeoutForStreaming(t *testing.T) {
	client := NewOpenAIClient("https://example.com", "token", "model")
	if client.httpClient.Timeout != 0 {
		t.Fatalf("http client timeout = %v", client.httpClient.Timeout)
	}
}

func TestNewOpenAIClientRaisesEmptyMessageLimitForStreaming(t *testing.T) {
	client := NewOpenAIClient("https://example.com", "token", "model")
	config := openai.DefaultConfig("token")
	if client.emptyMessagesLimit != 100000 {
		t.Fatalf("empty message limit = %d", client.emptyMessagesLimit)
	}
	if config.EmptyMessagesLimit >= 100000 {
		t.Fatalf("default empty message limit unexpectedly high: %d", config.EmptyMessagesLimit)
	}
}

func TestClientReviewRetries429WithProgressLoggingUntilSuccess(t *testing.T) {
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "Provider returned error", http.StatusTooManyRequests)
			return
		}
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	client.retrier.InitialBackoff = time.Millisecond
	client.retrier.MaxBackoff = time.Millisecond

	var logs bytes.Buffer
	logger := logging.New(&logs, true, false)
	logger.SetShowProgress(true)
	client.SetLogger(logger)

	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d", attempts)
	}
	got := logs.String()
	if want := "Model: rate limited (429), waiting 1ms before retry attempt 2"; !strings.Contains(got, want) {
		t.Fatalf("missing first retry progress log %q in:\n%s", want, got)
	}
	if want := "Model: rate limited (429), waiting 1ms before retry attempt 3"; !strings.Contains(got, want) {
		t.Fatalf("missing second retry progress log %q in:\n%s", want, got)
	}
}

func TestRetrierBackoffCapsAtConfiguredBounds(t *testing.T) {
	retrier := NewRetrier()
	retrier.InitialBackoff = time.Second
	retrier.MaxBackoff = 30 * time.Second

	if got := retrier.Backoff(0, nil); got != time.Second {
		t.Fatalf("attempt 0 backoff = %v", got)
	}
	if got := retrier.Backoff(4, nil); got != 16*time.Second {
		t.Fatalf("attempt 4 backoff = %v", got)
	}
	if got := retrier.Backoff(5, nil); got != 30*time.Second {
		t.Fatalf("attempt 5 backoff = %v", got)
	}
	if got := retrier.Backoff(6, nil); got != 30*time.Second {
		t.Fatalf("attempt 6 backoff = %v", got)
	}
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "120")
	if got := retrier.Backoff(0, resp); got != 30*time.Second {
		t.Fatalf("retry-after cap backoff = %v", got)
	}
	resp.Header.Set("Retry-After", "0")
	if got := retrier.Backoff(0, resp); got != time.Second {
		t.Fatalf("retry-after floor backoff = %v", got)
	}
}

func TestClientReviewUsesJSONSchemaWhenProvided(t *testing.T) {
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
		Schema:       FindingsSchema,
	})
	if err != nil {
		t.Fatal(err)
	}

	responseFormat, ok := payload["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format = %#v", payload["response_format"])
	}
	if responseFormat["type"] != "json_schema" {
		t.Fatalf("response_format.type = %v", responseFormat["type"])
	}
	jsonSchema, ok := responseFormat["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("json_schema = %#v", responseFormat["json_schema"])
	}
	if jsonSchema["name"] != "review_response" {
		t.Fatalf("json_schema.name = %v", jsonSchema["name"])
	}
	if jsonSchema["strict"] != true {
		t.Fatalf("json_schema.strict = %v", jsonSchema["strict"])
	}
}

func TestClientReviewReassemblesStreamedToolCalls(t *testing.T) {
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"tool_calls": []map[string]any{
							{
								"index": 0,
								"id":    "call_1",
								"type":  "function",
								"function": map[string]any{
									"name":      "inspect_file",
									"arguments": "{\"path\":\"ex",
								},
							},
						},
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"tool_calls": []map[string]any{
							{
								"index": 0,
								"function": map[string]any{
									"arguments": "tra.go\"}",
								},
							},
						},
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-3",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		Messages: []Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "user"},
		},
		Tools: []ToolDefinition{
			{
				Name:        "inspect_file",
				Description: "Retrieve a file",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
		ParallelToolCalls: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "inspect_file" {
		t.Fatalf("tool name = %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Arguments != `{"path":"extra.go"}` {
		t.Fatalf("arguments = %q", resp.ToolCalls[0].Arguments)
	}

	tools, ok := payload["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v", payload["tools"])
	}
	if payload["parallel_tool_calls"] != true {
		t.Fatalf("parallel_tool_calls = %#v", payload["parallel_tool_calls"])
	}
}

func TestClientReviewRecoversXMLToolCallsFromContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `I'll inspect both files.<tool_call>inspect_file<arg_key>path</arg_key><arg_value>pkg/server/handler/multichannelhandler.go</arg_value></tool_call>`,
						"tool_calls": []map[string]any{
							{
								"index": 0,
								"id":    "call_1",
								"type":  "function",
								"function": map[string]any{
									"name":      "inspect_file",
									"arguments": `{"path":"pkg/projects/session_cache_redis.go"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		Messages: []Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "user"},
		},
		Tools: []ToolDefinition{
			{
				Name:        "inspect_file",
				Description: "Retrieve a file",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
		ParallelToolCalls: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.ToolCalls) != 2 {
		t.Fatalf("tool calls = %#v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].ID != "call_1" || resp.ToolCalls[0].Name != "inspect_file" || resp.ToolCalls[0].Arguments != `{"path":"pkg/projects/session_cache_redis.go"}` {
		t.Fatalf("structured tool call = %#v", resp.ToolCalls[0])
	}
	if resp.ToolCalls[1].ID != "xml_tool_call_1" || resp.ToolCalls[1].Name != "inspect_file" || resp.ToolCalls[1].Arguments != `{"path":"pkg/server/handler/multichannelhandler.go"}` {
		t.Fatalf("xml tool call = %#v", resp.ToolCalls[1])
	}
	if strings.Contains(resp.RawResponse, "<tool_call>") {
		t.Fatalf("raw response should remove XML tool call markup: %q", resp.RawResponse)
	}
	if resp.RawResponse != "I'll inspect both files." {
		t.Fatalf("raw response = %q", resp.RawResponse)
	}
}

func TestClientReviewDeduplicatesXMLToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `I'll inspect the file.<tool_call>inspect_file<arg_key>path</arg_key><arg_value>extra.go</arg_value></tool_call>`,
						"tool_calls": []map[string]any{
							{
								"index": 0,
								"id":    "call_1",
								"type":  "function",
								"function": map[string]any{
									"name":      "inspect_file",
									"arguments": `{"path":"extra.go"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		Messages: []Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "user"},
		},
		Tools: []ToolDefinition{
			{
				Name:        "inspect_file",
				Description: "Retrieve a file",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
		ParallelToolCalls: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls = %#v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].ID != "call_1" {
		t.Fatalf("dedup should preserve structured tool call, got %#v", resp.ToolCalls[0])
	}
	if resp.ToolCalls[0].Arguments != `{"path":"extra.go"}` {
		t.Fatalf("arguments = %q", resp.ToolCalls[0].Arguments)
	}
	if strings.Contains(resp.RawResponse, "<tool_call>") {
		t.Fatalf("raw response should remove XML tool call markup: %q", resp.RawResponse)
	}
}

func TestClientReviewCanDisableParallelToolCalls(t *testing.T) {
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:      "system",
		UserContent:       "user",
		ParallelToolCalls: false,
		Tools: []ToolDefinition{
			{Name: "inspect_file", Description: "desc", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if payload["parallel_tool_calls"] != false {
		t.Fatalf("parallel_tool_calls = %#v", payload["parallel_tool_calls"])
	}
}

func TestClientReviewStreamsReasoningToLogger(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"reasoning_content": "Thinking through the diff.",
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-3",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	var buf bytes.Buffer
	client := NewOpenAIClient(server.URL, "token", "model")
	logger := logging.New(&buf, true, false)
	logger.SetShowReasoning(true)
	client.SetLogger(logger)

	if _, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	}); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "Reasoning...\n") {
		t.Fatalf("reasoning banner missing: %q", got)
	}
	if !strings.Contains(got, "Thinking through the diff.") {
		t.Fatalf("reasoning delta missing: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected trailing newline, got %q", got)
	}
}

func TestClientReviewDoesNotStreamReasoningWithoutFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"reasoning_content": "Thinking through the diff.",
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-3",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	var buf bytes.Buffer
	client := NewOpenAIClient(server.URL, "token", "model")
	client.SetLogger(logging.New(&buf, true, false))

	if _, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	}); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if strings.Contains(got, "Reasoning...\n") {
		t.Fatalf("reasoning banner should be hidden: %q", got)
	}
	if strings.Contains(got, "Thinking through the diff.") {
		t.Fatalf("reasoning delta should be hidden: %q", got)
	}
}

func TestClientReviewLogsToolCallOnlyRawResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"tool_calls": []map[string]any{
							{
								"index": 0,
								"id":    "call_1",
								"type":  "function",
								"function": map[string]any{
									"name":      "inspect_file",
									"arguments": "{\"path\":\"extra.go\"}",
								},
							},
						},
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	var buf bytes.Buffer
	client := NewOpenAIClient(server.URL, "token", "model")
	client.SetLogger(logging.New(&buf, true, false))

	if _, err := client.Review(context.Background(), &ReviewRequest{
		Messages: []Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "user"},
		},
		Tools: []ToolDefinition{
			{
				Name:        "inspect_file",
				Description: "Retrieve a file",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "LLM raw model response:") {
		t.Fatalf("missing raw response banner: %q", got)
	}
	if !strings.Contains(got, "\"content\": \"\"") {
		t.Fatalf("raw response should include content field even when empty: %q", got)
	}
	if !strings.Contains(got, "\"tool_calls\": [") {
		t.Fatalf("raw response should include tool calls: %q", got)
	}
	if !strings.Contains(got, "\"usage\": {") {
		t.Fatalf("raw response should include usage: %q", got)
	}
	if !strings.Contains(got, "\"saw_tool_calls\": true") {
		t.Fatalf("raw response should include saw_tool_calls: %q", got)
	}
	if strings.Contains(got, "(empty)") {
		t.Fatalf("raw response should not print empty for tool calls: %q", got)
	}
}

func TestClientReviewReturnsErrInvalidJSONOnParseFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": "not valid review json",
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     7,
				"completion_tokens": 3,
				"total_tokens":      10,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	})
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestClientReviewParsesJSONWrappedInProse(t *testing.T) {
	wrapped := "Sure! Here's my review:\n\n" +
		`{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"looks fine","overall_confidence_score":0.42}` +
		"\n\nLet me know if you need anything else."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": wrapped,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     7,
				"completion_tokens": 3,
				"total_tokens":      10,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	})
	if err != nil {
		t.Fatalf("expected lenient parse to succeed, got %v", err)
	}
	if resp.OverallCorrectness != "patch is correct" {
		t.Fatalf("overall_correctness = %q", resp.OverallCorrectness)
	}
	if resp.OverallExplanation != "looks fine" {
		t.Fatalf("overall_explanation = %q", resp.OverallExplanation)
	}
	if resp.OverallConfidenceScore != 0.42 {
		t.Fatalf("overall_confidence_score = %v", resp.OverallConfidenceScore)
	}
}

func TestClientReviewReportsMissingFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[]}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     7,
				"completion_tokens": 3,
				"total_tokens":      10,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	})
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
	var invalidResp *InvalidResponseError
	if !errors.As(err, &invalidResp) {
		t.Fatalf("expected *InvalidResponseError, got %T", err)
	}
	if invalidResp.RawContent != `{"findings":[]}` {
		t.Fatalf("raw content = %q", invalidResp.RawContent)
	}
	hasField := func(name string) bool {
		for _, f := range invalidResp.MissingFields {
			if strings.HasPrefix(f, name) {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"overall_correctness", "overall_explanation", "overall_confidence_score"} {
		if !hasField(want) {
			t.Fatalf("missing field %q not reported, got %v", want, invalidResp.MissingFields)
		}
	}
}

func TestClientReviewReportsPlainTextHTTPErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte("404 page not found\n")); err != nil {
			t.Fatalf("write error body: %v", err)
		}
	}))
	defer server.Close()

	var logs bytes.Buffer
	client := NewOpenAIClient(server.URL, "token", "missing-model")
	client.SetLogger(logging.New(&logs, true, false))

	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "llm: api returned 404 Not Found: 404 page not found"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("error should not expose SDK JSON parse failure: %q", err)
	}
	gotLogs := logs.String()
	if strings.Contains(gotLogs, "invalid character") {
		t.Fatalf("logs should not expose SDK JSON parse failure:\n%s", gotLogs)
	}
	if !strings.Contains(gotLogs, "LLM raw response body:") || !strings.Contains(gotLogs, "404 page not found") {
		t.Fatalf("logs should include provider response body:\n%s", gotLogs)
	}
}

func TestFallbackReasoningEfforts(t *testing.T) {
	tests := []struct {
		name   string
		effort string
		want   []string
	}{
		{name: "known", effort: "high", want: []string{"medium", "low", "minimal", "none", "off"}},
		{name: "empty", effort: "", want: []string{"low", "minimal", "none", "off"}},
		{name: "unknown", effort: "provider-max", want: []string{"low", "minimal", "none", "off"}},
		{name: "case insensitive", effort: "XHIGH", want: []string{"high", "medium", "low", "minimal", "none", "off"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fallbackReasoningEfforts(tt.effort)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("fallbackReasoningEfforts(%q) = %v, want %v", tt.effort, got, tt.want)
			}
		})
	}
}

func TestCloneReviewRequestIsolatesReferenceFields(t *testing.T) {
	maxTokens := 10
	temperature := 0.25
	topP := 0.9
	req := &ReviewRequest{
		Messages: []Message{
			{
				Role:    "assistant",
				Content: "original content",
				ToolCalls: []ToolCall{
					{ID: "call-1", Name: "inspect_file", Arguments: `{"path":"a.go"}`},
				},
			},
		},
		NoToolsMessages: []Message{
			{
				Role:    "assistant",
				Content: "original no-tools content",
				ToolCalls: []ToolCall{
					{ID: "call-nt", Name: "inspect_file", Arguments: `{"path":"nt.go"}`},
				},
			},
		},
		Tools: []ToolDefinition{
			{
				Name:       "inspect_file",
				Parameters: json.RawMessage(`{"type":"object"}`),
			},
		},
		Schema:      json.RawMessage(`{"type":"object"}`),
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
		TopP:        &topP,
		ExtraBody: map[string]any{
			"nested": map[string]any{"value": "original"},
			"list":   []any{"original"},
			"raw":    json.RawMessage(`{"value":"original"}`),
			"bytes":  []byte("original"),
		},
	}

	cloned := cloneReviewRequest(req)
	cloned.Messages[0].Content = "changed content"
	cloned.Messages[0].ToolCalls[0].Arguments = `{"path":"b.go"}`
	cloned.NoToolsMessages[0].Content = "changed no-tools content"
	cloned.NoToolsMessages[0].ToolCalls[0].Arguments = `{"path":"changed-nt.go"}`
	cloned.Tools[0].Parameters[0] = '['
	cloned.Schema[0] = '['
	*cloned.MaxTokens = 20
	*cloned.Temperature = 0.5
	*cloned.TopP = 0.7
	cloned.ExtraBody["nested"].(map[string]any)["value"] = "changed"
	cloned.ExtraBody["list"].([]any)[0] = "changed"
	cloned.ExtraBody["raw"].(json.RawMessage)[0] = '['
	cloned.ExtraBody["bytes"].([]byte)[0] = 'X'

	if req.Messages[0].Content != "original content" {
		t.Fatalf("message content was mutated: %q", req.Messages[0].Content)
	}
	if req.Messages[0].ToolCalls[0].Arguments != `{"path":"a.go"}` {
		t.Fatalf("tool call arguments were mutated: %q", req.Messages[0].ToolCalls[0].Arguments)
	}
	if req.NoToolsMessages[0].Content != "original no-tools content" {
		t.Fatalf("no-tools message content was mutated: %q", req.NoToolsMessages[0].Content)
	}
	if req.NoToolsMessages[0].ToolCalls[0].Arguments != `{"path":"nt.go"}` {
		t.Fatalf("no-tools tool call arguments were mutated: %q", req.NoToolsMessages[0].ToolCalls[0].Arguments)
	}
	if got, want := string(req.Tools[0].Parameters), `{"type":"object"}`; got != want {
		t.Fatalf("tool parameters = %q, want %q", got, want)
	}
	if got, want := string(req.Schema), `{"type":"object"}`; got != want {
		t.Fatalf("schema = %q, want %q", got, want)
	}
	if *req.MaxTokens != 10 {
		t.Fatalf("max tokens = %d", *req.MaxTokens)
	}
	if *req.Temperature != 0.25 {
		t.Fatalf("temperature = %f", *req.Temperature)
	}
	if *req.TopP != 0.9 {
		t.Fatalf("top_p = %f", *req.TopP)
	}
	if got := req.ExtraBody["nested"].(map[string]any)["value"]; got != "original" {
		t.Fatalf("nested extra body = %v", got)
	}
	if got := req.ExtraBody["list"].([]any)[0]; got != "original" {
		t.Fatalf("extra body list = %v", got)
	}
	if got, want := string(req.ExtraBody["raw"].(json.RawMessage)), `{"value":"original"}`; got != want {
		t.Fatalf("extra body raw = %q, want %q", got, want)
	}
	if got, want := string(req.ExtraBody["bytes"].([]byte)), "original"; got != want {
		t.Fatalf("extra body bytes = %q, want %q", got, want)
	}
}

func TestClientReviewFallsBackAfterReasoningBudgetExhausted(t *testing.T) {
	var efforts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		effort, _ := payload["reasoning_effort"].(string)
		efforts = append(efforts, effort)
		if len(efforts) == 1 {
			writeReasoningLengthSSE(t, w)
			return
		}
		writeValidReviewSSE(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user",
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(efforts, ","), "high,medium"; got != want {
		t.Fatalf("reasoning efforts = %s, want %s", got, want)
	}
	if resp.ReasoningEffort != "medium" {
		t.Fatalf("effective reasoning effort = %q", resp.ReasoningEffort)
	}
}

func TestClientReviewAddsConciseReasoningHintAfterBudgetExhaustion(t *testing.T) {
	var userMessages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		messages, ok := payload["messages"].([]any)
		if !ok {
			t.Fatalf("messages missing or wrong type: %#v", payload["messages"])
		}
		lastUser := ""
		for _, raw := range messages {
			msg, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("message has wrong type: %#v", raw)
			}
			if msg["role"] == "user" {
				lastUser, _ = msg["content"].(string)
			}
		}
		userMessages = append(userMessages, lastUser)

		effort, _ := payload["reasoning_effort"].(string)
		if effort == "high" {
			writeReasoningLengthSSE(t, w)
			return
		}
		if effort != "medium" {
			t.Fatalf("unexpected effort %q", effort)
		}
		writeValidReviewSSE(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user",
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(userMessages) != 2 {
		t.Fatalf("user messages = %d, want 2", len(userMessages))
	}
	if strings.Contains(userMessages[0], reasoningBudgetRetryHint) {
		t.Fatalf("first request should not include retry hint: %q", userMessages[0])
	}
	if !strings.Contains(userMessages[1], reasoningBudgetRetryHint) {
		t.Fatalf("retry request missing retry hint: %q", userMessages[1])
	}
}

func TestClientReviewAppendsConciseReasoningHintToSyntheticUserMessage(t *testing.T) {
	var retryMessages []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		effort, _ := payload["reasoning_effort"].(string)
		if effort == "high" {
			writeReasoningLengthSSE(t, w)
			return
		}
		if effort != "medium" {
			t.Fatalf("unexpected effort %q", effort)
		}
		rawMessages, ok := payload["messages"].([]any)
		if !ok {
			t.Fatalf("messages missing or wrong type: %#v", payload["messages"])
		}
		retryMessages = make([]map[string]any, 0, len(rawMessages))
		for _, raw := range rawMessages {
			msg, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("message has wrong type: %#v", raw)
			}
			retryMessages = append(retryMessages, msg)
		}
		writeValidReviewSSE(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{
		Messages: []Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "original review request"},
			{Role: "assistant", Content: "tool call"},
			{Role: "user", Content: "synthetic tool followup"},
		},
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retryMessages) != 4 {
		t.Fatalf("retry messages = %d, want 4", len(retryMessages))
	}
	originalUser, _ := retryMessages[1]["content"].(string)
	if strings.Contains(originalUser, reasoningBudgetRetryHint) {
		t.Fatalf("original user message should not include retry hint: %q", originalUser)
	}
	syntheticUser, _ := retryMessages[3]["content"].(string)
	if !strings.Contains(syntheticUser, "synthetic tool followup\n\n"+reasoningBudgetRetryHint) {
		t.Fatalf("synthetic user message missing retry hint: %q", syntheticUser)
	}
}

func TestClientReviewTreatsReasoningOnlyPeerInternalStreamErrorAsBudgetExhausted(t *testing.T) {
	var efforts []string
	client := NewOpenAIClient("http://example.test", "token", "model")
	client.transport.base = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if err := req.Body.Close(); err != nil {
			t.Fatalf("close request body: %v", err)
		}
		effort, _ := payload["reasoning_effort"].(string)
		efforts = append(efforts, effort)

		var body io.ReadCloser
		if len(efforts) == 1 {
			body = &errorAfterReader{
				reader: bytes.NewReader([]byte(sseChunk(t, map[string]any{
					"id":      "chunk-1",
					"object":  "chat.completion.chunk",
					"created": 1,
					"model":   "model",
					"choices": []map[string]any{
						{
							"index": 0,
							"delta": map[string]any{
								"reasoning_content": "thinking",
							},
						},
					},
				}))),
				err: errors.New("stream error: stream ID 5; INTERNAL_ERROR; received from peer"),
			}
		} else {
			body = io.NopCloser(strings.NewReader(validReviewStream(t)))
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       body,
			Request:    req,
		}, nil
	})

	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user",
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(efforts, ","), "high,medium"; got != want {
		t.Fatalf("reasoning efforts = %s, want %s", got, want)
	}
	if resp.ReasoningEffort != "medium" {
		t.Fatalf("effective reasoning effort = %q", resp.ReasoningEffort)
	}
}

func TestClientReviewNoToolsFallbackInvalidJSONIncludesMetadata(t *testing.T) {
	var attempts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		effort, _ := payload["reasoning_effort"].(string)
		hasTools := false
		if tools, ok := payload["tools"].([]any); ok && len(tools) > 0 {
			hasTools = true
		}
		attempts = append(attempts, fmt.Sprintf("%s:%t", effort, hasTools))
		if effort == "off" && !hasTools {
			writeSSEChunk(t, w, map[string]any{
				"id":      "chunk-1",
				"object":  "chat.completion.chunk",
				"created": 1,
				"model":   "model",
				"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": "not json"}}},
			})
			writeSSEChunk(t, w, map[string]any{
				"id":      "chunk-2",
				"object":  "chat.completion.chunk",
				"created": 1,
				"model":   "model",
				"choices": []map[string]any{},
				"usage": map[string]any{
					"prompt_tokens":     1,
					"completion_tokens": 1,
					"total_tokens":      2,
				},
			})
			writeSSEDone(t, w)
			return
		}
		if effort == "low" {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte("Failed to deserialize the JSON body into the target type: unknown variant `low`, expected one of `medium`, `high`")); err != nil {
				t.Fatalf("write error: %v", err)
			}
			return
		}
		writeReasoningLengthSSE(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user",
		ReasoningEffort: "high",
		Tools: []ToolDefinition{
			{
				Name:        "inspect_file",
				Description: "Retrieve a file",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
		ParallelToolCalls: true,
	})
	var invalidResp *InvalidResponseError
	if !errors.As(err, &invalidResp) {
		t.Fatalf("expected InvalidResponseError, got %T: %v", err, err)
	}
	if got, want := strings.Join(attempts, ","), "high:true,medium:true,low:true,minimal:true,none:true,off:true,off:false"; got != want {
		t.Fatalf("attempts = %s, want %s", got, want)
	}
	if invalidResp.ReasoningEffort != "off" {
		t.Fatalf("invalid response reasoning effort = %q", invalidResp.ReasoningEffort)
	}
	if !invalidResp.ToolsOmitted {
		t.Fatal("expected invalid response to record omitted tools")
	}
}

func TestClientReviewRetriesLastBudgetExhaustedEffortWithoutToolsAfterFallbacks(t *testing.T) {
	var attempts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		effort, _ := payload["reasoning_effort"].(string)
		hasTools := false
		if tools, ok := payload["tools"].([]any); ok && len(tools) > 0 {
			hasTools = true
		}
		attempts = append(attempts, fmt.Sprintf("%s:%t", effort, hasTools))
		if effort == "off" && !hasTools {
			writeValidReviewSSE(t, w)
			return
		}
		if effort == "low" {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte("Failed to deserialize the JSON body into the target type: unknown variant `low`, expected one of `medium`, `high`")); err != nil {
				t.Fatalf("write error: %v", err)
			}
			return
		}
		writeReasoningLengthSSE(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user",
		ReasoningEffort: "high",
		Tools: []ToolDefinition{
			{
				Name:        "inspect_file",
				Description: "Retrieve a file",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
		ParallelToolCalls: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(attempts, ","), "high:true,medium:true,low:true,minimal:true,none:true,off:true,off:false"; got != want {
		t.Fatalf("attempts = %s, want %s", got, want)
	}
	if resp.ReasoningEffort != "off" {
		t.Fatalf("effective reasoning effort = %q", resp.ReasoningEffort)
	}
	if !resp.ToolsOmitted {
		t.Fatal("expected response to record omitted tools")
	}
}

func TestClientReviewNoToolsRetryIncludesHintWhenBudgetPreviouslyExhausted(t *testing.T) {
	var attempts []struct {
		effort   string
		hasTools bool
		user     string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		effort, _ := payload["reasoning_effort"].(string)
		hasTools := false
		if tools, ok := payload["tools"].([]any); ok && len(tools) > 0 {
			hasTools = true
		}
		user := ""
		if msgs, ok := payload["messages"].([]any); ok {
			for _, raw := range msgs {
				msg, _ := raw.(map[string]any)
				if msg["role"] == "user" {
					user, _ = msg["content"].(string)
				}
			}
		}
		attempts = append(attempts, struct {
			effort   string
			hasTools bool
			user     string
		}{effort, hasTools, user})
		if hasTools && effort == "high" {
			writeReasoningLengthSSE(t, w)
			return
		}
		if hasTools {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte(`{"error":{"message":"reasoning_effort value is invalid"}}`)); err != nil {
				t.Fatalf("write error: %v", err)
			}
			return
		}
		writeValidReviewSSE(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user content",
		ReasoningEffort: "high",
		Tools: []ToolDefinition{
			{
				Name:        "inspect_file",
				Description: "Retrieve a file",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(attempts), 7; got != want {
		t.Fatalf("attempt count = %d, want %d", got, want)
	}
	if got, want := attemptSummary(attempts), "high:true,medium:true,low:true,minimal:true,none:true,off:true,high:false"; got != want {
		t.Fatalf("attempts = %s, want %s", got, want)
	}
	if got := strings.Count(attempts[len(attempts)-1].user, reasoningBudgetRetryHint); got != 1 {
		t.Fatalf("no-tools retry hint count = %d in %q", got, attempts[len(attempts)-1].user)
	}
	if resp.ReasoningEffort != "high" {
		t.Fatalf("effective reasoning effort = %q", resp.ReasoningEffort)
	}
	if !resp.ToolsOmitted {
		t.Fatal("expected response to record omitted tools")
	}
}

func TestClientReviewNoToolsRetryUsesProvidedNoToolsMessages(t *testing.T) {
	var attempts []string
	var noToolsMessages []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		effort, _ := payload["reasoning_effort"].(string)
		hasTools := false
		if tools, ok := payload["tools"].([]any); ok && len(tools) > 0 {
			hasTools = true
		}
		attempts = append(attempts, fmt.Sprintf("%s:%t", effort, hasTools))
		if hasTools && effort == "high" {
			writeReasoningLengthSSE(t, w)
			return
		}
		if hasTools {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte(`{"error":{"message":"reasoning_effort value is invalid"}}`)); err != nil {
				t.Fatalf("write error: %v", err)
			}
			return
		}
		rawMessages, ok := payload["messages"].([]any)
		if !ok {
			t.Fatalf("messages missing or wrong type: %#v", payload["messages"])
		}
		noToolsMessages = make([]map[string]any, 0, len(rawMessages))
		for _, raw := range rawMessages {
			msg, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("message has wrong type: %#v", raw)
			}
			noToolsMessages = append(noToolsMessages, msg)
		}
		writeValidReviewSSE(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		Messages: []Message{
			{Role: "system", Content: "tool system"},
			{Role: "user", Content: "review request"},
			{
				Role:    "assistant",
				Content: "I'll inspect a.go.",
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"a.go"}`},
				},
			},
			{Role: "tool", ToolCallID: "call_1", Content: `{"path":"a.go","content":"package a"}`},
			{Role: "user", Content: "synthetic tool followup"},
		},
		NoToolsMessages: []Message{
			{Role: "system", Content: "no-tools system"},
			{Role: "user", Content: "review request"},
			{Role: "assistant", Content: "I'll inspect a.go."},
			{Role: "user", Content: `{"path":"a.go","content":"package a"}`},
		},
		ReasoningEffort: "high",
		Tools: []ToolDefinition{
			{
				Name:        "inspect_file",
				Description: "Retrieve a file",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
		ParallelToolCalls: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(attempts, ","), "high:true,medium:true,low:true,minimal:true,none:true,off:true,high:false"; got != want {
		t.Fatalf("attempts = %s, want %s", got, want)
	}
	if !resp.ToolsOmitted {
		t.Fatal("expected response to record omitted tools")
	}
	if len(noToolsMessages) != 4 {
		t.Fatalf("no-tools messages = %d, want 4: %#v", len(noToolsMessages), noToolsMessages)
	}
	if content, _ := noToolsMessages[0]["content"].(string); content != "no-tools system" {
		t.Fatalf("system content = %q", content)
	}
	if content, _ := noToolsMessages[3]["content"].(string); strings.Contains(content, "synthetic tool followup") {
		t.Fatalf("no-tools messages should use provided converted history, got %q", content)
	}
	if content, _ := noToolsMessages[3]["content"].(string); strings.Count(content, reasoningBudgetRetryHint) != 1 {
		t.Fatalf("no-tools retry hint count wrong in %q", content)
	}
	for _, msg := range noToolsMessages {
		if msg["role"] == "tool" {
			t.Fatalf("no-tools request sent tool role: %#v", noToolsMessages)
		}
		if _, ok := msg["tool_calls"]; ok {
			t.Fatalf("no-tools request sent tool_calls: %#v", noToolsMessages)
		}
		if _, ok := msg["tool_call_id"]; ok {
			t.Fatalf("no-tools request sent tool_call_id: %#v", noToolsMessages)
		}
	}
}

func TestNoToolsFallbackMessagesSanitizesToolTranscript(t *testing.T) {
	got := noToolsFallbackMessages(&ReviewRequest{
		Messages: []Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "review"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_empty", Name: "inspect_file", Arguments: `{"path":"empty.go"}`},
				},
			},
			{
				Role:    "assistant",
				Content: "I inspected a.go.",
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"a.go"}`},
				},
			},
			{Role: "tool", Name: "inspect_file", ToolCallID: "call_1", Content: `{"path":"a.go"}`},
		},
	})

	if len(got) != 4 {
		t.Fatalf("sanitized messages = %d, want 4: %#v", len(got), got)
	}
	if got[2].Role != "assistant" || got[2].Content != "I inspected a.go." {
		t.Fatalf("assistant content was not preserved without tool calls: %#v", got[2])
	}
	if got[3].Role != "user" || got[3].ToolCallID != "" || got[3].Name != "" {
		t.Fatalf("tool result was not converted to plain user message: %#v", got[3])
	}
	for _, msg := range got {
		if msg.Role == "tool" {
			t.Fatalf("sanitized messages contain tool role: %#v", got)
		}
		if len(msg.ToolCalls) > 0 {
			t.Fatalf("sanitized messages contain tool calls: %#v", got)
		}
	}
}

func TestAddReasoningBudgetRetryHintIsIdempotent(t *testing.T) {
	req := &ReviewRequest{
		Messages: []Message{
			{Role: "system", Content: "system"},
			{Role: "user", Content: "review request\n\n" + reasoningBudgetRetryHint},
		},
	}

	addReasoningBudgetRetryHint(req)
	addReasoningBudgetRetryHint(req)

	if got := strings.Count(req.Messages[1].Content, reasoningBudgetRetryHint); got != 1 {
		t.Fatalf("retry hint count = %d in %q", got, req.Messages[1].Content)
	}
}

func attemptSummary(attempts []struct {
	effort   string
	hasTools bool
	user     string
}) string {
	parts := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		parts = append(parts, fmt.Sprintf("%s:%t", attempt.effort, attempt.hasTools))
	}
	return strings.Join(parts, ",")
}

func TestClientReviewFallbackStartsAtLowForEmptyAndUnknownEffort(t *testing.T) {
	tests := []struct {
		name   string
		effort string
		want   string
	}{
		{name: "empty", effort: "", want: ",low"},
		{name: "unknown", effort: "provider-high", want: "provider-high,low"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var efforts []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				effort, _ := payload["reasoning_effort"].(string)
				efforts = append(efforts, effort)
				if len(efforts) == 1 {
					writeReasoningLengthSSE(t, w)
					return
				}
				writeValidReviewSSE(t, w)
			}))
			defer server.Close()

			client := NewOpenAIClient(server.URL, "token", "model")
			resp, err := client.Review(context.Background(), &ReviewRequest{
				SystemPrompt:    "system",
				UserContent:     "user",
				ReasoningEffort: tt.effort,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(efforts, ","); got != tt.want {
				t.Fatalf("reasoning efforts = %s, want %s", got, tt.want)
			}
			if resp.ReasoningEffort != "low" {
				t.Fatalf("effective reasoning effort = %q", resp.ReasoningEffort)
			}
		})
	}
}

func TestClientReviewContinuesAfterRejectedLowerReasoningEffort(t *testing.T) {
	var efforts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		effort, _ := payload["reasoning_effort"].(string)
		efforts = append(efforts, effort)
		switch effort {
		case "high":
			writeReasoningLengthSSE(t, w)
		case "medium":
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte(`{"error":{"message":"reasoning_effort value is invalid"}}`)); err != nil {
				t.Fatalf("write error: %v", err)
			}
		case "low":
			writeValidReviewSSE(t, w)
		default:
			t.Fatalf("unexpected effort %q", effort)
		}
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user",
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(efforts, ","), "high,medium,low"; got != want {
		t.Fatalf("reasoning efforts = %s, want %s", got, want)
	}
	if resp.ReasoningEffort != "low" {
		t.Fatalf("effective reasoning effort = %q", resp.ReasoningEffort)
	}
}

func TestClientReviewAttemptsAllLowerEffortsBeforeExhausting(t *testing.T) {
	var efforts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		effort, _ := payload["reasoning_effort"].(string)
		efforts = append(efforts, effort)
		if effort == "minimal" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			if _, err := w.Write([]byte(`{"detail":"reasoning effort is not supported"}`)); err != nil {
				t.Fatalf("write error: %v", err)
			}
			return
		}
		writeReasoningLengthSSE(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user",
		ReasoningEffort: "medium",
	})
	if err == nil {
		t.Fatal("expected reasoning budget error")
	}
	var budgetErr *ReasoningBudgetExhaustedError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected ReasoningBudgetExhaustedError, got %T: %v", err, err)
	}
	if got, want := strings.Join(efforts, ","), "medium,low,minimal,none,off"; got != want {
		t.Fatalf("reasoning efforts = %s, want %s", got, want)
	}
}

func TestClientReviewRetriesMinimalWithoutToolsWhenNoneIsUnknownVariant(t *testing.T) {
	var attempts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		effort, _ := payload["reasoning_effort"].(string)
		hasTools := false
		if tools, ok := payload["tools"].([]any); ok && len(tools) > 0 {
			hasTools = true
		}
		attempts = append(attempts, fmt.Sprintf("%s:%t", effort, hasTools))
		if effort == "none" || effort == "off" {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte(fmt.Sprintf("Failed to deserialize the JSON body into the target type: unknown variant `%s`, expected one of `minimal`, `low`, `medium`, `high` at line 1 column 46461", effort))); err != nil {
				t.Fatalf("write error: %v", err)
			}
			return
		}
		if effort == "minimal" && !hasTools {
			writeValidReviewSSE(t, w)
			return
		}
		writeReasoningLengthSSE(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user",
		ReasoningEffort: "medium",
		Tools: []ToolDefinition{
			{
				Name:        "inspect_file",
				Description: "Retrieve a file",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
		ParallelToolCalls: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(attempts, ","), "medium:true,low:true,minimal:true,none:true,off:true,minimal:false"; got != want {
		t.Fatalf("attempts = %s, want %s", got, want)
	}
	if resp.ReasoningEffort != "minimal" {
		t.Fatalf("effective reasoning effort = %q", resp.ReasoningEffort)
	}
}

func TestClientReviewInvalidJSONIncludesEffectiveReasoningEffort(t *testing.T) {
	var efforts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		effort, _ := payload["reasoning_effort"].(string)
		efforts = append(efforts, effort)
		if len(efforts) == 1 {
			writeReasoningLengthSSE(t, w)
			return
		}
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": "not json"}}},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:    "system",
		UserContent:     "user",
		ReasoningEffort: "high",
	})
	var invalidResp *InvalidResponseError
	if !errors.As(err, &invalidResp) {
		t.Fatalf("expected InvalidResponseError, got %T: %v", err, err)
	}
	if invalidResp.ReasoningEffort != "medium" {
		t.Fatalf("invalid response reasoning effort = %q", invalidResp.ReasoningEffort)
	}
}

func TestClientReviewRetriesRetryableStatus(t *testing.T) {
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "rate limited",
					"type":    "rate_limit_error",
				},
			})
			return
		}

		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	client.retrier.InitialBackoff = 4 * time.Nanosecond
	client.retrier.MaxBackoff = 4 * time.Nanosecond

	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
	if resp.TokensUsed.TotalTokens != 6 {
		t.Fatalf("total tokens = %d", resp.TokensUsed.TotalTokens)
	}
}

func TestClientReviewRetriesNetworkErrorWhileReadingStream(t *testing.T) {
	attempts := 0
	var logBuf bytes.Buffer

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "text/event-stream")
		if attempts == 1 {
			if _, err := w.Write([]byte("data: ")); err != nil {
				t.Fatalf("write partial chunk: %v", err)
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	client.SetLogger(logging.New(&logBuf, false, false))
	client.retrier.InitialBackoff = 4 * time.Nanosecond
	client.retrier.MaxBackoff = 4 * time.Nanosecond

	if _, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	}); err != nil {
		t.Fatal(err)
	}

	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
	if !strings.Contains(logBuf.String(), "LLM stream hit a network error, retrying...") {
		t.Fatalf("retry notice missing: %q", logBuf.String())
	}
}

func TestClientReviewRetriesNetworkErrorOpeningStream(t *testing.T) {
	attempts := 0
	var logBuf bytes.Buffer

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("response writer does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.Close()
			return
		}

		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
		})
		writeSSEChunk(t, w, map[string]any{
			"id":      "chunk-2",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "model",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	client.SetLogger(logging.New(&logBuf, false, false))
	client.retrier.InitialBackoff = 4 * time.Nanosecond
	client.retrier.MaxBackoff = 4 * time.Nanosecond

	if _, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
	}); err != nil {
		t.Fatal(err)
	}

	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
	if !strings.Contains(logBuf.String(), "LLM request hit a network error, retrying...") {
		t.Fatalf("network retry notice missing: %q", logBuf.String())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errorAfterReader struct {
	reader *bytes.Reader
	err    error
}

func (r *errorAfterReader) Read(p []byte) (int, error) {
	if r.reader.Len() > 0 {
		return r.reader.Read(p)
	}
	return 0, r.err
}

func (r *errorAfterReader) Close() error {
	return nil
}

func validReviewStream(t *testing.T) string {
	t.Helper()
	return sseChunk(t, map[string]any{
		"id":      "chunk-1",
		"object":  "chat.completion.chunk",
		"created": 1,
		"model":   "model",
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]any{
					"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
				},
			},
		},
	}) + sseChunk(t, map[string]any{
		"id":      "chunk-2",
		"object":  "chat.completion.chunk",
		"created": 1,
		"model":   "model",
		"choices": []map[string]any{},
		"usage": map[string]any{
			"prompt_tokens":     4,
			"completion_tokens": 2,
			"total_tokens":      6,
		},
	}) + "data: [DONE]\n\n"
}

func writeSSEChunk(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "text/event-stream")
	if _, err := w.Write([]byte(sseChunk(t, payload))); err != nil {
		t.Fatalf("write chunk: %v", err)
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func sseChunk(t *testing.T, payload any) string {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal chunk: %v", err)
	}
	return "data: " + string(data) + "\n\n"
}

func writeReasoningLengthSSE(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	writeSSEChunk(t, w, map[string]any{
		"id":      "chunk-1",
		"object":  "chat.completion.chunk",
		"created": 1,
		"model":   "model",
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]any{
					"reasoning_content": "thinking",
				},
				"finish_reason": "length",
			},
		},
	})
	writeSSEChunk(t, w, map[string]any{
		"id":      "chunk-2",
		"object":  "chat.completion.chunk",
		"created": 1,
		"model":   "model",
		"choices": []map[string]any{},
		"usage": map[string]any{
			"prompt_tokens":     4,
			"completion_tokens": 2,
			"total_tokens":      6,
		},
	})
	writeSSEDone(t, w)
}

func writeValidReviewSSE(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	writeSSEChunk(t, w, map[string]any{
		"id":      "chunk-1",
		"object":  "chat.completion.chunk",
		"created": 1,
		"model":   "model",
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]any{
					"content": `{"findings":[],"overall_correctness":"patch is correct","overall_explanation":"summary","overall_confidence_score":0.9}`,
				},
			},
		},
	})
	writeSSEChunk(t, w, map[string]any{
		"id":      "chunk-2",
		"object":  "chat.completion.chunk",
		"created": 1,
		"model":   "model",
		"choices": []map[string]any{},
		"usage": map[string]any{
			"prompt_tokens":     4,
			"completion_tokens": 2,
			"total_tokens":      6,
		},
	})
	writeSSEDone(t, w)
}

func writeSSEDone(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	if _, err := w.Write([]byte("data: [DONE]\n\n")); err != nil {
		t.Fatalf("write done: %v", err)
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
