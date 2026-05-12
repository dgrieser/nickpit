package modelcheck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
)

type scriptedClient struct {
	responses []scriptedResponse
	reqs      []*llm.ReviewRequest
}

type scriptedResponse struct {
	resp *llm.ReviewResponse
	err  error
}

func (s *scriptedClient) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	cloned := *req
	cloned.Messages = append([]llm.Message(nil), req.Messages...)
	cloned.Tools = append([]llm.ToolDefinition(nil), req.Tools...)
	s.reqs = append(s.reqs, &cloned)
	if len(s.responses) == 0 {
		return &llm.ReviewResponse{RawResponse: finalSentinel}, nil
	}
	next := s.responses[0]
	s.responses = s.responses[1:]
	return next.resp, next.err
}

func TestCheckerRunsToolProbeWithInMemoryFixture(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_list", Name: "list_files", Arguments: `{}`}}}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{
				{ID: "call_readme", Name: "inspect_file", Arguments: `{"path":"README.md"}`},
				{ID: "call_app", Name: "inspect_file", Arguments: `{"path":"internal/app.go"}`},
			}}},
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
		},
	}
	result := New(client, config.Profile{Model: "model", ReasoningEffort: "high"}).Run(context.Background())
	if result.ConfiguredNoTools().Status != StatusOK {
		t.Fatalf("no-tools status = %s", result.ConfiguredNoTools().Status)
	}
	if result.ConfiguredTools().Status != StatusOK {
		t.Fatalf("tools status = %s error=%s", result.ConfiguredTools().Status, result.ConfiguredTools().Error)
	}
	if len(client.reqs) < 4 {
		t.Fatalf("requests = %d, want at least 4", len(client.reqs))
	}
	toolResponseCount := 0
	for _, msg := range client.reqs[3].Messages {
		if msg.Role == "tool" {
			toolResponseCount++
		}
	}
	if toolResponseCount != 3 {
		t.Fatalf("tool responses before final request = %d, want 3", toolResponseCount)
	}
}

func TestCheckerClassifiesGenericFailure(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{{err: errors.New("boom")}},
	}
	result := New(client, config.Profile{Model: "model", ReasoningEffort: "high"}).Run(context.Background())
	if result.ConfiguredNoTools().Status != StatusFailed {
		t.Fatalf("status = %s, want failed", result.ConfiguredNoTools().Status)
	}
}

func TestCheckerClassifiesUnsupportedReasoningEffort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "reasoning_effort unsupported"},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := llm.NewOpenAIClient(server.URL, "token", "model")
	result := New(client, config.Profile{Model: "model", BaseURL: server.URL, APIKey: "token", ReasoningEffort: "high"}).Run(context.Background())
	if result.ConfiguredNoTools().Status != StatusUnsupported {
		t.Fatalf("status = %s error=%s, want unsupported", result.ConfiguredNoTools().Status, result.ConfiguredNoTools().Error)
	}
}

func TestCheckerRequiresToolSequence(t *testing.T) {
	client := &scriptedClient{
		responses: []scriptedResponse{
			{resp: &llm.ReviewResponse{RawResponse: finalSentinel}},
			{resp: &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "call_file", Name: "inspect_file", Arguments: `{"path":"README.md"}`}}}},
		},
	}
	result := New(client, config.Profile{Model: "model", ReasoningEffort: "high"}).Run(context.Background())
	if result.ConfiguredTools().Status != StatusFailed {
		t.Fatalf("tools status = %s, want failed", result.ConfiguredTools().Status)
	}
}
