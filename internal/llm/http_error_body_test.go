package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// An error response whose body arrives after its headers must still be read
// in full: the provider's error decides between a refresh, a backoff, a
// dropped optional field, or giving up.
func TestHTTPErrorBodyArrivingAfterHeadersIsRead(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol func() Protocol
	}{
		{"chat_completions", ChatCompletionsProtocol},
		{"responses", NewResponsesProtocol},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.(http.Flusher).Flush()
				time.Sleep(50 * time.Millisecond)
				_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","message":"bad parameter"}}`))
			}))
			defer server.Close()
			client := NewAPIClient(ClientOptions{BaseURL: server.URL, Model: "m", Protocol: tc.protocol()})
			_, err := client.Review(context.Background(), &ReviewRequest{SystemPrompt: "s", UserContent: "u", SchemaKind: SchemaKindText})
			var statusErr *llmHTTPStatusError
			if !errors.As(err, &statusErr) {
				t.Fatalf("err = %v, want the provider's HTTP error", err)
			}
			if statusErr.statusCode != http.StatusBadRequest || !strings.Contains(statusErr.message, "bad parameter") || statusErr.code != "invalid_request" {
				t.Fatalf("status error = %+v", statusErr)
			}
		})
	}
}

// An error response whose body never completes must not hang the call: the
// body read gives up after errorBodyTimeout and the status still drives the
// retry policy — here a 503, retried into a success.
func TestStalledHTTPErrorBodyStillRetriesByStatus(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"overloa`))
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		writeSSEChunk(t, w, map[string]any{"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": "ok"}, "finish_reason": "stop"}}})
		writeSSEDone(t, w)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "k", "m")
	client.errorBodyTimeout = 100 * time.Millisecond
	client.retrier.InitialBackoff = 0
	client.retrier.MaxBackoff = 0
	done := make(chan struct{})
	var resp *ReviewResponse
	var err error
	// Cancelled before the server closes (defers run last-in first-out), so
	// a regression fails within the test's own deadline instead of hanging
	// the server's Close on the stalled handler.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		defer close(done)
		resp, err = client.Review(ctx, &ReviewRequest{SystemPrompt: "s", UserContent: "u", SchemaKind: SchemaKindText})
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		cancel()
		<-done
		t.Fatal("the call hung on an error body that never completed")
	}
	if err != nil {
		t.Fatalf("err = %v, want the 503 retried", err)
	}
	if requests != 2 || resp.RawResponse != "ok" {
		t.Fatalf("requests = %d response = %q", requests, resp.RawResponse)
	}
}
