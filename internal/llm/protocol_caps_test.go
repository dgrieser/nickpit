package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// limitedProtocol speaks Chat Completions but declares an API that can only
// express plain text: no tools, no reasoning effort, no schema, no sampling.
type limitedProtocol struct{ Protocol }

func (limitedProtocol) Capabilities() Capabilities {
	return Capabilities{Reasoning: ReasoningNone}
}

func TestClientHonoursDeclaredCapabilities(t *testing.T) {
	var payloads []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		payloads = append(payloads, payload)
		writeSSEChunk(t, w, map[string]any{"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": "plain answer"}}}})
		writeSSEChunk(t, w, map[string]any{"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	temperature, topK := 0.3, 20
	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "m", Protocol: limitedProtocol{ChatCompletionsProtocol()}})
	resp, err := client.Review(context.Background(), &ReviewRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: "system"},
			{Role: RoleUser, Content: "user"},
			{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "list_files", Arguments: "{}"}}},
			{Role: RoleTool, ToolCallID: "c1", Name: "list_files", Content: "README.md"},
		},
		Tools:           []ToolDefinition{{Name: "list_files", Parameters: json.RawMessage(`{"type":"object"}`)}},
		SchemaKind:      SchemaKindText,
		Schema:          json.RawMessage(`{"type":"object"}`),
		ReasoningEffort: "high",
		Temperature:     &temperature,
		TopK:            &topK,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.ToolsOmitted {
		t.Fatal("a protocol without tool calling must mark the response ToolsOmitted")
	}
	if len(payloads) != 1 {
		t.Fatalf("requests = %d, want 1", len(payloads))
	}
	payload := payloads[0]
	for _, key := range []string{"tools", "response_format", "reasoning_effort", "temperature", "top_k"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("%s sent to a protocol that cannot express it: %#v", key, payload)
		}
	}
	for _, raw := range payload["messages"].([]any) {
		msg := raw.(map[string]any)
		if msg["role"] == RoleTool || msg["tool_calls"] != nil {
			t.Fatalf("tool history sent to a protocol without tool calling: %#v", msg)
		}
	}
}

func TestClientSkipsEffortLadderWithoutEffortCapability(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"reasoning_effort value is invalid"}}`))
	}))
	defer server.Close()

	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "m", Protocol: limitedProtocol{ChatCompletionsProtocol()}})
	if _, err := client.Review(context.Background(), &ReviewRequest{SystemPrompt: "s", UserContent: "u", ReasoningEffort: "high"}); err == nil {
		t.Fatal("expected error")
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1: the ladder has nothing to vary without an effort capability", requests)
	}
}

func fuzzyLoopText() string {
	var lines []string
	for _, cycle := range [][3]string{
		{"AddSession", "DropSession", "Close"},
		{"DropSession", "DropPod", "Close"},
		{"CreateSession", "DeleteSession", "Close"},
		{"OpenSession", "RemoveSession", "Close"},
		{"StartSession", "StopSession", "Close"},
		{"MakeSession", "ClearSession", "Close"},
	} {
		lines = append(lines, fuzzyReasoningCycle(cycle[0], cycle[1], cycle[2])...)
	}
	return strings.Join(lines, "\n") + "\n"
}

// The loop detector watches the model's own tokens. A provider summary is
// paraphrase, so the same text as a summary must not abort the stream, while
// as raw reasoning it does.
func TestLoopDetectionReadsRawReasoningOnly(t *testing.T) {
	for _, tc := range []struct {
		event        string
		wantRequests int
	}{
		{event: "response.reasoning_summary_text.delta", wantRequests: 1},
		{event: "response.reasoning_text.delta", wantRequests: 2},
	} {
		t.Run(tc.event, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if requests == 1 {
					writeResponsesEvent(t, w, map[string]any{"type": tc.event, "delta": fuzzyLoopText()})
					// Give the detector the same window in both cases: a
					// detected loop cancels the request inside it.
					select {
					case <-r.Context().Done():
						return
					case <-time.After(500 * time.Millisecond):
					}
				}
				writeResponsesEvent(t, w, map[string]any{"type": "response.output_text.delta", "delta": "done"})
				writeResponsesEvent(t, w, completedEvent())
			}))
			defer server.Close()

			client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "m", Protocol: NewResponsesProtocol()})
			resp, err := client.Review(context.Background(), &ReviewRequest{SystemPrompt: "s", UserContent: "u", SchemaKind: SchemaKindText, ReasoningEffort: "high"})
			if err != nil {
				t.Fatal(err)
			}
			if resp.RawResponse != "done" || requests != tc.wantRequests {
				t.Fatalf("response = %q requests = %d, want %d", resp.RawResponse, requests, tc.wantRequests)
			}
		})
	}
}

func TestResponsesMidStreamFailureAfterOutputIsNotAdmission(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeResponsesEvent(t, w, map[string]any{"type": "response.output_text.delta", "delta": "partial"})
		writeResponsesEvent(t, w, map[string]any{"type": "response.failed", "response": map[string]any{"error": map[string]any{"code": "subscription_sharing_usage_limit_exceeded", "message": "limit"}}})
	}))
	defer server.Close()

	client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "m", Protocol: NewResponsesProtocol()})
	_, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt:       "s",
		UserContent:        "u",
		SchemaKind:         SchemaKindText,
		CallerRetriesError: func(error) bool { return false },
	})
	if err == nil || !strings.Contains(err.Error(), "usage limit") {
		t.Fatalf("error = %v", err)
	}
	var perr *ProviderError
	if !errors.As(err, &perr) || perr.Code != "subscription_sharing_usage_limit_exceeded" {
		t.Fatalf("provider error not reachable: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestProtocolByName(t *testing.T) {
	for name, want := range map[string]string{"": ChatCompletionsProtocolName, "chat_completions": ChatCompletionsProtocolName, "responses": ResponsesProtocolName} {
		protocol, err := ProtocolByName(name)
		if err != nil || protocol.Name() != want {
			t.Fatalf("ProtocolByName(%q) = %v, %v", name, protocol, err)
		}
	}
	if _, err := ProtocolByName("carrier-pigeon"); err == nil {
		t.Fatal("unknown protocol accepted")
	}
	if caps := NewResponsesProtocol().Capabilities(); caps.Reasoning != ReasoningSummary || caps.AcceptsSampling("top_k") {
		t.Fatalf("responses capabilities = %s", caps)
	}
}
