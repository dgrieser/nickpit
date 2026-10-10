package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// planServer is a fake ChatGPT-plan Responses endpoint for a two-turn tool
// round: the first request gets a reasoning item (with encrypted content) and
// a function call, the second the final answer. It records every payload.
type planServer struct {
	t        *testing.T
	mu       sync.Mutex
	payloads []map[string]any
}

func (s *planServer) handler(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.t.Fatalf("decode request: %v", err)
	}
	s.mu.Lock()
	s.payloads = append(s.payloads, payload)
	turn := len(s.payloads)
	s.mu.Unlock()
	t := s.t
	writeResponsesEvent(t, w, map[string]any{"type": "response.created", "response": map[string]any{"id": "resp"}})
	if turn == 1 {
		writeResponsesEvent(t, w, map[string]any{"type": "response.reasoning_summary_text.delta", "delta": "Need the file list."})
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_item.done", "output_index": 0, "item": map[string]any{
			"id": "rs_1", "type": "reasoning", "status": "completed",
			"summary":           []any{map[string]any{"type": "summary_text", "text": "Need the file list."}},
			"encrypted_content": "enc-turn-1",
		}})
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_item.added", "output_index": 1, "item": map[string]any{"id": "fc_1", "type": "function_call", "call_id": "call_1", "name": "list_files", "arguments": ""}})
		writeResponsesEvent(t, w, map[string]any{"type": "response.function_call_arguments.delta", "item_id": "fc_1", "output_index": 1, "delta": `{}`})
		writeResponsesEvent(t, w, completedEvent())
		return
	}
	writeResponsesEvent(t, w, map[string]any{"type": "response.output_text.delta", "delta": "README.md is the only file."})
	writeResponsesEvent(t, w, completedEvent())
}

func TestChatGPTPlanRequestShapeAndReasoningReplay(t *testing.T) {
	fake := &planServer{t: t}
	server := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer server.Close()

	maxTokens, temperature, topP := 512, 0.2, 0.9
	client := NewAPIClient(ClientOptions{
		BaseURL:  server.URL + "/v1",
		Model:    "gpt-plan",
		Protocol: NewResponsesProtocolWith(ResponsesOptions{ChatGPTPlan: true}),
		Tokens:   StaticToken("plan-token"),
	})
	req := &ReviewRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: "You review code."},
			{Role: RoleUser, Content: "Which files exist?"},
		},
		Tools:             []ToolDefinition{{Name: "list_files", Description: "List repository files", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)}},
		ParallelToolCalls: true,
		SchemaKind:        SchemaKindText,
		ReasoningEffort:   "high",
		MaxTokens:         &maxTokens,
		Temperature:       &temperature,
		TopP:              &topP,
		ExtraBody:         map[string]any{"metadata": map[string]any{"k": "v"}, "user": "u", "truncation": "auto", "service_hint": "kept"},
	}
	first, err := client.Review(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.ToolCalls) != 1 || first.ToolCalls[0].Name != "list_files" {
		t.Fatalf("first turn tool calls = %#v", first.ToolCalls)
	}

	req.Messages = append(req.Messages,
		first.AssistantMessage(),
		Message{Role: RoleTool, ToolCallID: first.ToolCalls[0].ID, Name: "list_files", Content: "README.md"},
	)
	second, err := client.Review(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if second.RawResponse != "README.md is the only file." {
		t.Fatalf("final answer = %q", second.RawResponse)
	}

	for i, payload := range fake.payloads {
		// Plan usage rejects these outright; they must never be sent.
		for _, field := range []string{"tools", "max_output_tokens", "temperature", "top_p", "metadata", "user", "truncation"} {
			if _, ok := payload[field]; ok {
				t.Fatalf("request %d sends %q, which plan usage rejects: %#v", i+1, field, payload)
			}
		}
		if payload["service_hint"] != "kept" || payload["store"] != false || payload["stream"] != true {
			t.Fatalf("request %d = %#v", i+1, payload)
		}
		include, _ := payload["include"].([]any)
		if len(include) != 1 || include[0] != "reasoning.encrypted_content" {
			t.Fatalf("request %d include = %#v", i+1, payload["include"])
		}
		input := payload["input"].([]any)
		tools := input[0].(map[string]any)
		toolList, _ := tools["tools"].([]any)
		if tools["type"] != "additional_tools" || tools["role"] != "developer" || len(toolList) != 1 {
			t.Fatalf("request %d does not lead with an additional_tools item: %#v", i+1, input[0])
		}
		if fn := toolList[0].(map[string]any); fn["type"] != "function" || fn["name"] != "list_files" {
			t.Fatalf("request %d tool = %#v", i+1, fn)
		}
		for _, raw := range input {
			if item := raw.(map[string]any); item["role"] == "system" {
				t.Fatalf("request %d sends a system-role input item: %#v", i+1, item)
			}
		}
	}

	// The second request replays the turn in the order the model produced
	// it: encrypted reasoning, then the call, then its result.
	input := fake.payloads[1]["input"].([]any)
	var kinds []string
	for _, raw := range input {
		item := raw.(map[string]any)
		kind, _ := item["type"].(string)
		if kind == "" {
			kind = "message:" + item["role"].(string)
		}
		kinds = append(kinds, kind)
	}
	if got, want := strings.Join(kinds, ","), "additional_tools,message:user,reasoning,function_call,function_call_output"; got != want {
		t.Fatalf("second request input = %s, want %s", got, want)
	}
	reasoning := input[2].(map[string]any)
	if reasoning["encrypted_content"] != "enc-turn-1" {
		t.Fatalf("replayed reasoning = %#v", reasoning)
	}
	if _, ok := reasoning["id"]; ok {
		t.Fatalf("replayed reasoning keeps the server id, which store=false cannot resolve: %#v", reasoning)
	}
	if summary, _ := reasoning["summary"].([]any); len(summary) != 1 {
		t.Fatalf("replayed reasoning summary = %#v", reasoning["summary"])
	}
	call := input[3].(map[string]any)
	output := input[4].(map[string]any)
	if call["call_id"] != "call_1" || output["call_id"] != "call_1" || output["output"] != "README.md" {
		t.Fatalf("call/output = %#v / %#v", call, output)
	}
}

// A platform key takes the regular shape: top-level tools and every knob the
// Responses API accepts.
func TestResponsesAPIKeyRequestShape(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_text.delta", "delta": "ok"})
		writeResponsesEvent(t, w, completedEvent())
	}))
	defer server.Close()

	maxTokens, temperature := 256, 0.3
	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "gpt", Protocol: NewResponsesProtocol(), Tokens: StaticToken("k")})
	if _, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "s", UserContent: "u", SchemaKind: SchemaKindText,
		Tools:     []ToolDefinition{{Name: "list_files", Parameters: json.RawMessage(`{"type":"object"}`)}},
		MaxTokens: &maxTokens, Temperature: &temperature,
	}); err != nil {
		t.Fatal(err)
	}
	if tools, _ := payload["tools"].([]any); len(tools) != 1 {
		t.Fatalf("tools = %#v", payload["tools"])
	}
	if payload["max_output_tokens"] != float64(256) || payload["temperature"] != 0.3 {
		t.Fatalf("payload = %#v", payload)
	}
	if first := payload["input"].([]any)[0].(map[string]any); first["type"] == "additional_tools" {
		t.Fatalf("platform request uses the plan tool form: %#v", first)
	}
}

// Provider state goes back only to the endpoint and model that produced it.
func TestProviderStateIsNotReplayedToAnotherModel(t *testing.T) {
	var payloads []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		payloads = append(payloads, payload)
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_text.delta", "delta": "ok"})
		writeResponsesEvent(t, w, completedEvent())
	}))
	defer server.Close()

	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "model-a", Protocol: NewResponsesProtocol()})
	stale := &ProviderState{
		Origin: client.stateOrigin("model-b"),
		Items:  []json.RawMessage{json.RawMessage(`{"type":"reasoning","summary":[],"encrypted_content":"from-b"}`)},
	}
	own := &ProviderState{
		Origin: client.stateOrigin("model-a"),
		Items:  []json.RawMessage{json.RawMessage(`{"type":"reasoning","summary":[],"encrypted_content":"from-a"}`)},
	}
	for _, state := range []*ProviderState{stale, own} {
		if _, err := client.Review(context.Background(), &ReviewRequest{
			Messages: []Message{
				{Role: RoleUser, Content: "u"},
				{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c", Name: "list_files", Arguments: "{}"}}, ProviderState: state},
				{Role: RoleTool, ToolCallID: "c", Content: "x"},
			},
			Tools:      []ToolDefinition{{Name: "list_files"}},
			SchemaKind: SchemaKindText,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if body, _ := json.Marshal(payloads[0]); strings.Contains(string(body), "from-b") {
		t.Fatalf("state of another model was replayed: %s", body)
	}
	if body, _ := json.Marshal(payloads[1]); !strings.Contains(string(body), "from-a") {
		t.Fatalf("own state was not replayed: %s", body)
	}
}

// Only response.completed is a finished response. Partial output that parses
// must still be rejected when the stream reports it incomplete.
func TestIncompleteResponsesAreNotResults(t *testing.T) {
	for _, tc := range []struct {
		name   string
		events []map[string]any
		check  func(t *testing.T, err error)
	}{
		{
			name: "valid partial JSON cut at the token limit",
			events: []map[string]any{
				{"type": "response.output_text.delta", "delta": `{"answer":"ok"}`},
				{"type": "response.incomplete", "response": map[string]any{"incomplete_details": map[string]any{"reason": "max_output_tokens"}}},
			},
			check: func(t *testing.T, err error) {
				var invalid *InvalidResponseError
				if !errors.As(err, &invalid) || !strings.Contains(invalid.Reason, "cut off") {
					t.Fatalf("err = %v, want an incomplete-response error", err)
				}
			},
		},
		{
			name: "answer cut after reasoning goes down the effort ladder",
			events: []map[string]any{
				{"type": "response.reasoning_summary_text.delta", "delta": "thinking"},
				{"type": "response.output_text.delta", "delta": `{"answer":`},
				{"type": "response.incomplete", "response": map[string]any{"incomplete_details": map[string]any{"reason": "max_output_tokens"}}},
			},
			check: func(t *testing.T, err error) {
				var budget *ReasoningBudgetExhaustedError
				if !errors.As(err, &budget) || !budget.Truncated {
					t.Fatalf("err = %v, want a truncated budget error", err)
				}
			},
		},
		{
			name: "content filter is terminal",
			events: []map[string]any{
				{"type": "response.output_text.delta", "delta": `{"answer":`},
				{"type": "response.incomplete", "response": map[string]any{"incomplete_details": map[string]any{"reason": "content_filter"}}},
			},
			check: func(t *testing.T, err error) {
				var filtered *ContentFilteredError
				if !errors.As(err, &filtered) {
					t.Fatalf("err = %v, want *ContentFilteredError", err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				for _, event := range tc.events {
					writeResponsesEvent(t, w, event)
				}
			}))
			defer server.Close()
			client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "m", Protocol: NewResponsesProtocol()})
			_, err := client.Review(context.Background(), &ReviewRequest{
				SystemPrompt: "s", UserContent: "u",
				SchemaKind: SchemaKindJSON, Schema: json.RawMessage(`{"type":"object"}`),
				DisableReasoningEffortFallback: true,
				CallerRetriesError:             func(error) bool { return false },
			})
			tc.check(t, err)
		})
	}
}

// A stream that ends without any terminal event is an interruption, retried
// like a dropped connection, not a response.
func TestResponsesStreamWithoutTerminalEventIsRetried(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_text.delta", "delta": "partial"})
		if requests == 1 {
			return
		}
		writeResponsesEvent(t, w, completedEvent())
	}))
	defer server.Close()
	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "m", Protocol: NewResponsesProtocol()})
	client.retrier.InitialBackoff = 0
	client.retrier.MaxBackoff = 0
	resp, err := client.Review(context.Background(), &ReviewRequest{SystemPrompt: "s", UserContent: "u", SchemaKind: SchemaKindText})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || resp.RawResponse != "partial" {
		t.Fatalf("requests = %d response = %q", requests, resp.RawResponse)
	}
}

// Truncation is protocol-neutral: a Chat Completions length finish with
// output that parses is not accepted either.
func TestChatCompletionsLengthFinishIsNotAResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": `{"answer":"ok"}`}, "finish_reason": "length"}}})
		writeSSEDone(t, w)
	}))
	defer server.Close()
	client := NewOpenAIClient(server.URL, "k", "m")
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "s", UserContent: "u", SchemaKind: SchemaKindJSON,
		CallerRetriesError: func(error) bool { return false },
	})
	var invalid *InvalidResponseError
	if !errors.As(err, &invalid) || !strings.Contains(invalid.Reason, "cut off") {
		t.Fatalf("err = %v, want an incomplete-response error", err)
	}
}
