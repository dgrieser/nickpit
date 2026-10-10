package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// The Protocol refactor moved Chat Completions encoding out of go-openai's
// HTTP client into protocol_chat.go. Every OpenAI-compatible server keeps
// receiving the exact bytes it did before: the golden bodies were captured
// from the pre-refactor client (commit e09a4cf) for the same requests, and
// are compared with key order intact.
func TestChatCompletionsBodiesMatchPreRefactorClient(t *testing.T) {
	data, err := os.ReadFile("testdata/chat_completions_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]json.RawMessage
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	cases := chatGoldenCases()
	if len(golden) != len(cases) {
		t.Fatalf("golden bodies = %d, cases = %d", len(golden), len(cases))
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			var captured []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if captured == nil {
					captured, _ = io.ReadAll(r.Body)
				}
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"message":"nope"}}`))
			}))
			defer server.Close()
			client := NewOpenAIClient(server.URL, "k", "default-model")
			req.CallerRetriesError = func(error) bool { return false }
			_, _ = client.Review(context.Background(), req)

			var want, got bytes.Buffer
			if err := json.Compact(&want, golden[name]); err != nil {
				t.Fatal(err)
			}
			if err := json.Compact(&got, captured); err != nil {
				t.Fatalf("captured body is not JSON: %v\n%s", err, captured)
			}
			if !bytes.Equal(want.Bytes(), got.Bytes()) {
				t.Fatalf("request body changed\nwant %s\n got %s", want.Bytes(), got.Bytes())
			}
		})
	}
}
