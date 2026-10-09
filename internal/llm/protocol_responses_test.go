package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type staticTokens struct {
	mu        sync.Mutex
	token     string
	refreshed int
}

func (s *staticTokens) Token(context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token, nil
}

func (s *staticTokens) ForceRefresh(context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshed++
	s.token = fmt.Sprintf("refreshed-%d", s.refreshed)
	return s.token, nil
}

func writeResponsesEvent(t *testing.T, w http.ResponseWriter, event map[string]any) {
	t.Helper()
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("encode event: %v", err)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data); err != nil {
		t.Fatalf("write event: %v", err)
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func completedEvent() map[string]any {
	return map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"id":    "resp_1",
			"model": "gpt-test",
			"usage": map[string]any{
				"input_tokens":          12,
				"output_tokens":         7,
				"total_tokens":          19,
				"output_tokens_details": map[string]any{"reasoning_tokens": 3},
			},
		},
	}
}

func TestResponsesProtocolTranslatesToolCallRound(t *testing.T) {
	var (
		path    string
		auth    string
		payload map[string]any
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		auth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeResponsesEvent(t, w, map[string]any{"type": "response.created", "response": map[string]any{"id": "resp_1", "model": "gpt-test"}})
		writeResponsesEvent(t, w, map[string]any{"type": "response.reasoning_summary_text.delta", "item_id": "rs_1", "summary_index": 0, "delta": "Looking at the file."})
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_item.added", "output_index": 1, "item": map[string]any{"id": "fc_1", "type": "function_call", "call_id": "call_1", "name": "inspect_file", "arguments": ""}})
		writeResponsesEvent(t, w, map[string]any{"type": "response.function_call_arguments.delta", "item_id": "fc_1", "output_index": 1, "delta": `{"path":"ex`})
		writeResponsesEvent(t, w, map[string]any{"type": "response.function_call_arguments.delta", "item_id": "fc_1", "output_index": 1, "delta": `tra.go"}`})
		writeResponsesEvent(t, w, map[string]any{"type": "response.function_call_arguments.done", "item_id": "fc_1", "output_index": 1, "arguments": `{"path":"extra.go"}`})
		writeResponsesEvent(t, w, completedEvent())
	}))
	defer server.Close()

	client := NewAPIClient(ClientOptions{BaseURL: server.URL + "/v1", Model: "gpt-test", Protocol: NewResponsesProtocol(), Tokens: &staticTokens{token: "access-1"}})
	resp, err := client.Review(context.Background(), &ReviewRequest{
		Messages: []Message{
			{Role: "system", Content: "system prompt"},
			{Role: "user", Content: "review this"},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "call_0", Name: "search", Arguments: `{"q":"x"}`}}},
			{Role: "tool", ToolCallID: "call_0", Name: "search", Content: "result"},
		},
		Tools:             []ToolDefinition{{Name: "inspect_file", Description: "Retrieve a file", Parameters: json.RawMessage(`{"type":"object"}`)}},
		ParallelToolCalls: true,
		ReasoningEffort:   "high",
		TopK:              new(int),
	})
	if err != nil {
		t.Fatal(err)
	}

	if path != "/v1/responses" {
		t.Fatalf("path = %q", path)
	}
	if auth != "Bearer access-1" {
		t.Fatalf("authorization = %q", auth)
	}
	if payload["store"] != false || payload["stream"] != true {
		t.Fatalf("store/stream = %#v/%#v", payload["store"], payload["stream"])
	}
	if payload["instructions"] != "system prompt" {
		t.Fatalf("instructions = %#v", payload["instructions"])
	}
	if _, ok := payload["top_k"]; ok {
		t.Fatalf("top_k must not reach the Responses API: %#v", payload)
	}
	reasoning, _ := payload["reasoning"].(map[string]any)
	if reasoning["effort"] != "high" || reasoning["summary"] != "auto" {
		t.Fatalf("reasoning = %#v", payload["reasoning"])
	}
	input, _ := payload["input"].([]any)
	if len(input) != 3 {
		t.Fatalf("input = %#v", payload["input"])
	}
	call, _ := input[1].(map[string]any)
	output, _ := input[2].(map[string]any)
	if call["type"] != "function_call" || call["call_id"] != "call_0" || call["name"] != "search" {
		t.Fatalf("function_call item = %#v", call)
	}
	if output["type"] != "function_call_output" || output["call_id"] != "call_0" || output["output"] != "result" {
		t.Fatalf("function_call_output item = %#v", output)
	}
	tools, _ := payload["tools"].([]any)
	tool, _ := tools[0].(map[string]any)
	if len(tools) != 1 || tool["type"] != "function" || tool["name"] != "inspect_file" {
		t.Fatalf("tools = %#v", payload["tools"])
	}

	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "call_1" || resp.ToolCalls[0].Name != "inspect_file" {
		t.Fatalf("tool calls = %#v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].Arguments != `{"path":"extra.go"}` {
		t.Fatalf("arguments = %q", resp.ToolCalls[0].Arguments)
	}
	if !resp.Reasoned {
		t.Fatal("reasoning summary was not surfaced as reasoning")
	}
	if resp.TokensUsed.PromptTokens != 12 || resp.TokensUsed.CompletionTokens != 7 || resp.TokensUsed.TotalTokens != 19 {
		t.Fatalf("usage = %#v", resp.TokensUsed)
	}
}

func TestResponsesProtocolTranslatesJSONSchemaText(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeResponsesEvent(t, w, map[string]any{"type": "response.created", "response": map[string]any{"id": "resp_2"}})
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_text.delta", "delta": `{"answer":`})
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_text.delta", "delta": `"ok"}`})
		writeResponsesEvent(t, w, completedEvent())
	}))
	defer server.Close()

	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "gpt-test", Protocol: NewResponsesProtocol(), Tokens: &staticTokens{token: "t"}})
	maxTokens := 100
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
		Schema:       json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}}}`),
		SchemaKind:   SchemaKindJSON,
		MaxTokens:    &maxTokens,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.RawResponse != `{"answer":"ok"}` {
		t.Fatalf("raw response = %q", resp.RawResponse)
	}
	text, _ := payload["text"].(map[string]any)
	format, _ := text["format"].(map[string]any)
	if format["type"] != "json_schema" || format["name"] != "json_response" || format["schema"] == nil {
		t.Fatalf("text.format = %#v", payload["text"])
	}
	if payload["max_output_tokens"] != float64(100) {
		t.Fatalf("max_output_tokens = %#v", payload["max_output_tokens"])
	}
}

func TestResponsesProtocolReportsUsageLimitWithoutRetrying(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeResponsesEvent(t, w, map[string]any{"type": "response.created", "response": map[string]any{"id": "resp_3"}})
		writeResponsesEvent(t, w, map[string]any{"type": "response.failed", "response": map[string]any{
			"id":    "resp_3",
			"error": map[string]any{"code": "subscription_sharing_usage_limit_exceeded", "message": "limit"},
		}})
	}))
	defer server.Close()

	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "gpt-test", Protocol: NewResponsesProtocol(), Tokens: &staticTokens{token: "t"}})
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:       "system",
		UserContent:        "user",
		CallerRetriesError: func(error) bool { return false },
	})
	if err == nil {
		t.Fatal("expected usage-limit error")
	}
	if !strings.Contains(err.Error(), "usage limit") || !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1 (a plan usage limit must not be retried)", requests)
	}
}

func TestResponsesProtocolDropsRejectedOptionalParam(t *testing.T) {
	var summaries []any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		reasoning, _ := payload["reasoning"].(map[string]any)
		summaries = append(summaries, reasoning["summary"])
		if reasoning["summary"] != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"subscription_sharing_unsupported_capability","param":"reasoning.summary","message":"unsupported"}}`))
			return
		}
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_text.delta", "delta": "hello"})
		writeResponsesEvent(t, w, completedEvent())
	}))
	defer server.Close()

	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "gpt-test", Protocol: NewResponsesProtocol(), Tokens: &staticTokens{token: "t"}})
	for range 2 {
		resp, err := client.Review(context.Background(), &ReviewRequest{
			SystemPrompt:    "system",
			UserContent:     "user",
			SchemaKind:      SchemaKindText,
			ReasoningEffort: "low",
		})
		if err != nil {
			t.Fatal(err)
		}
		if resp.RawResponse != "hello" {
			t.Fatalf("raw response = %q", resp.RawResponse)
		}
	}
	// The second call goes straight out without the rejected field.
	if len(summaries) != 3 || summaries[0] != "auto" || summaries[1] != nil || summaries[2] != nil {
		t.Fatalf("summaries = %#v", summaries)
	}
}

func TestResponsesProtocolRefreshesTokenOnUnauthorized(t *testing.T) {
	var auths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "Bearer stale" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"detail":"token expired"}`))
			return
		}
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_text.delta", "delta": "hi"})
		writeResponsesEvent(t, w, completedEvent())
	}))
	defer server.Close()

	tokens := &staticTokens{token: "stale"}
	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "gpt-test", Protocol: NewResponsesProtocol(), Tokens: tokens})
	if _, err := client.Review(context.Background(), &ReviewRequest{SystemPrompt: "s", UserContent: "u", SchemaKind: SchemaKindText}); err != nil {
		t.Fatal(err)
	}
	if len(auths) != 2 || auths[1] != "Bearer refreshed-1" {
		t.Fatalf("authorizations = %#v", auths)
	}
}

func TestResponsesProtocolIncompleteMapsToLengthFinish(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeResponsesEvent(t, w, map[string]any{"type": "response.reasoning_summary_text.delta", "delta": "thinking"})
		writeResponsesEvent(t, w, map[string]any{"type": "response.incomplete", "response": map[string]any{
			"incomplete_details": map[string]any{"reason": "max_output_tokens"},
		}})
	}))
	defer server.Close()

	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "gpt-test", Protocol: NewResponsesProtocol(), Tokens: &staticTokens{token: "t"}})
	_, _, err := client.reviewOnce(context.Background(), &ReviewRequest{SystemPrompt: "s", UserContent: "u", ReasoningEffort: "high"}, &retryProgress{})
	var budgetErr *ReasoningBudgetExhaustedError
	if err == nil || !errors.As(err, &budgetErr) {
		t.Fatalf("error = %v, want reasoning budget exhausted", err)
	}
}

func TestResponsesEncodeKeepsLaterSystemMessagesAsDeveloper(t *testing.T) {
	data, err := NewResponsesProtocol().EncodeRequest(&CompletionRequest{
		Model: "m",
		Messages: []Message{
			{Role: RoleSystem, Content: "a"},
			{Role: RoleSystem, Content: "b"},
			{Role: RoleUser, Content: "u"},
			{Role: RoleSystem, Content: "nudge"},
		},
		ReasoningEffort: "off",
	})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out["instructions"] != "a\n\nb" {
		t.Fatalf("instructions = %#v", out["instructions"])
	}
	input := out["input"].([]any)
	last := input[len(input)-1].(map[string]any)
	if last["role"] != "developer" || last["content"] != "nudge" {
		t.Fatalf("last input = %#v", last)
	}
	reasoning := out["reasoning"].(map[string]any)
	if reasoning["effort"] != "none" || reasoning["summary"] != nil {
		t.Fatalf("reasoning = %#v", reasoning)
	}
	if out["store"] != false || out["stream"] != true {
		t.Fatalf("store/stream = %#v/%#v", out["store"], out["stream"])
	}
}

func TestResponsesProtocolEffortRejectionStaysRecognizable(t *testing.T) {
	var efforts []any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		reasoning, _ := payload["reasoning"].(map[string]any)
		efforts = append(efforts, reasoning["effort"])
		if reasoning["effort"] == "max" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","code":"unsupported_value","param":"reasoning.effort","message":"Unsupported value: 'max' is not supported with this model."}}`))
			return
		}
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_text.delta", "delta": "ok"})
		writeResponsesEvent(t, w, completedEvent())
	}))
	defer server.Close()

	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "gpt-test", Protocol: NewResponsesProtocol(), Tokens: &staticTokens{token: "t"}})
	resp, err := client.Review(context.Background(), &ReviewRequest{SystemPrompt: "s", UserContent: "u", SchemaKind: SchemaKindText, ReasoningEffort: "max"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ReasoningEffort == "max" || len(efforts) < 2 {
		t.Fatalf("effort ladder did not fall back: efforts=%v final=%q", efforts, resp.ReasoningEffort)
	}
}
