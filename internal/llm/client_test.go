package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"findings":[{"title":"[P2] Flag issue","body":"Something is wrong","confidence_score":0.9,"priority":2,"code_location":{"absolute_file_path":"/tmp/main.go","line_range":{"start":10,"end":10}}}],"overall_correctness":"patch is incorrect","overall_explanation":"summary","overall_confidence_score":0.9}`,
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "token", "model")
	resp, err := client.Review(context.Background(), &ReviewRequest{
		SystemPrompt: "system",
		UserContent:  "user",
		MaxTokens:    10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Findings) != 1 {
		t.Fatalf("findings = %d", len(resp.Findings))
	}
	if resp.TokensUsed.TotalTokens != 15 {
		t.Fatalf("total tokens = %d", resp.TokensUsed.TotalTokens)
	}
}
