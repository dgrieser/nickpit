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
