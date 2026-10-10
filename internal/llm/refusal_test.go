package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A refusal the provider reports apart from the answer ends the call: it is
// not parsed as an answer, and it is not an InvalidResponseError — the error
// the callers' output-retry loops re-prompt on.
func TestRefusalIsTerminalNotAnInvalidResponse(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol func() Protocol
		write    func(t *testing.T, w http.ResponseWriter)
	}{
		{
			name:     "responses",
			protocol: NewResponsesProtocol,
			write: func(t *testing.T, w http.ResponseWriter) {
				writeResponsesEvent(t, w, map[string]any{"type": "response.refusal.delta", "delta": "I can't help "})
				writeResponsesEvent(t, w, map[string]any{"type": "response.refusal.delta", "delta": "with that."})
				writeResponsesEvent(t, w, map[string]any{"type": "response.refusal.done", "refusal": "I can't help with that."})
				writeResponsesEvent(t, w, completedEvent())
			},
		},
		{
			name:     "responses_after_reasoning",
			protocol: NewResponsesProtocol,
			write: func(t *testing.T, w http.ResponseWriter) {
				writeResponsesEvent(t, w, map[string]any{"type": "response.reasoning_summary_text.delta", "delta": "Considering the request."})
				writeResponsesEvent(t, w, map[string]any{"type": "response.refusal.delta", "delta": "I can't help with that."})
				writeResponsesEvent(t, w, completedEvent())
			},
		},
		{
			name:     "responses_done_without_delta",
			protocol: NewResponsesProtocol,
			write: func(t *testing.T, w http.ResponseWriter) {
				writeResponsesEvent(t, w, map[string]any{"type": "response.reasoning_summary_text.delta", "delta": "Considering the request."})
				writeResponsesEvent(t, w, map[string]any{"type": "response.refusal.done", "refusal": "I can't help with that."})
				writeResponsesEvent(t, w, completedEvent())
			},
		},
		{
			name:     "chat_completions_after_reasoning",
			protocol: ChatCompletionsProtocol,
			write: func(t *testing.T, w http.ResponseWriter) {
				writeSSEChunk(t, w, map[string]any{"choices": []map[string]any{{"index": 0, "delta": map[string]any{"reasoning_content": "Considering the request."}}}})
				writeSSEChunk(t, w, map[string]any{"choices": []map[string]any{{"index": 0, "delta": map[string]any{"refusal": "I can't help with that."}, "finish_reason": "stop"}}})
				writeSSEDone(t, w)
			},
		},
		{
			name:     "chat_completions",
			protocol: ChatCompletionsProtocol,
			write: func(t *testing.T, w http.ResponseWriter) {
				writeSSEChunk(t, w, map[string]any{"choices": []map[string]any{{"index": 0, "delta": map[string]any{"refusal": "I can't help with that."}}}})
				writeSSEChunk(t, w, map[string]any{"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}})
				writeSSEDone(t, w)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				tc.write(t, w)
			}))
			defer server.Close()
			client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "m", Protocol: tc.protocol()})
			_, err := client.Review(context.Background(), &ReviewRequest{
				SystemPrompt: "s", UserContent: "u", ReasoningEffort: "high",
				SchemaKind: SchemaKindReview, Schema: json.RawMessage(`{"type":"object"}`),
			})
			var refusal *RefusalError
			if !errors.As(err, &refusal) || strings.TrimSpace(refusal.Message) != "I can't help with that." {
				t.Fatalf("err = %v, want *RefusalError with the refusal once", err)
			}
			var invalid *InvalidResponseError
			if errors.As(err, &invalid) {
				t.Fatalf("a refusal must not be an invalid response the output-retry loop re-prompts: %v", err)
			}
			if requests != 1 {
				t.Fatalf("requests = %d, want 1: no effort ladder or retry for a refusal", requests)
			}
		})
	}
}
