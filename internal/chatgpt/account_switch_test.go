package chatgpt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
)

// A long-running process signed in as account A produces encrypted reasoning;
// another process then signs in as account B. The next request goes out
// under B's token and must not carry A's reasoning — but B's own reasoning
// is replayed on the turn after.
func TestAccountSwitchDropsPreviousAccountsReasoning(t *testing.T) {
	type request struct {
		auth string
		body string
	}
	var (
		mu       sync.Mutex
		requests []request
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, request{auth: r.Header.Get("Authorization"), body: string(body)})
		turn := len(requests)
		mu.Unlock()
		account := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer access-")
		w.Header().Set("Content-Type", "text/event-stream")
		event := func(v map[string]any) {
			data, _ := json.Marshal(v)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", v["type"], data)
		}
		event(map[string]any{"type": "response.output_item.done", "output_index": 0, "item": map[string]any{
			"type": "reasoning", "summary": []any{}, "encrypted_content": "reasoning-of-" + account,
		}})
		event(map[string]any{"type": "response.output_item.added", "output_index": 1, "item": map[string]any{
			"id": fmt.Sprintf("fc_%d", turn), "type": "function_call", "call_id": fmt.Sprintf("call_%d", turn), "name": "list_files", "arguments": "{}",
		}})
		event(map[string]any{"type": "response.completed", "response": map[string]any{"usage": map[string]any{"input_tokens": 1, "output_tokens": 1}}})
	}))
	defer server.Close()

	store := validStore(t, "access-a", "user-a")
	tokens := &LazySession{Store: store, Provider: NewProvider()}
	client := llm.NewAPIClient(llm.ClientOptions{
		BaseURL:  server.URL,
		Model:    "gpt-plan",
		Protocol: llm.NewResponsesProtocolWith(llm.ResponsesOptions{ChatGPTPlan: true}),
		Tokens:   tokens,
	})
	req := &llm.ReviewRequest{
		Messages:        []llm.Message{{Role: llm.RoleUser, Content: "list the files"}},
		Tools:           []llm.ToolDefinition{{Name: "list_files"}},
		SchemaKind:      llm.SchemaKindText,
		ReasoningEffort: "high",
	}
	turn := func() {
		t.Helper()
		resp, err := client.Review(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		req.Messages = append(req.Messages, resp.AssistantMessage(),
			llm.Message{Role: llm.RoleTool, ToolCallID: resp.ToolCalls[0].ID, Name: "list_files", Content: "README.md"})
	}

	turn() // account A
	if err := store.Replace(&Credentials{
		ClientID: "oaiapp_123", Subject: "user-b", AccessToken: "access-b", RefreshToken: "refresh-b",
		TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour), Scopes: strings.Fields(Scopes),
	}); err != nil {
		t.Fatal(err)
	}
	turn() // account B, with A's turn in the history
	turn() // account B, with B's turn in the history

	if requests[0].auth != "Bearer access-a" || requests[1].auth != "Bearer access-b" || requests[2].auth != "Bearer access-b" {
		t.Fatalf("authorizations = %q, %q, %q", requests[0].auth, requests[1].auth, requests[2].auth)
	}
	if strings.Contains(requests[1].body, "reasoning-of-a") || strings.Contains(requests[2].body, "reasoning-of-a") {
		t.Fatalf("account A's reasoning was sent under account B's token:\n%s\n%s", requests[1].body, requests[2].body)
	}
	if !strings.Contains(requests[2].body, "reasoning-of-b") {
		t.Fatalf("account B's own reasoning was not replayed:\n%s", requests[2].body)
	}
}
