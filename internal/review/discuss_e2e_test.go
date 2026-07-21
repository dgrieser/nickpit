package review

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

// discussStubLLM captures every request and answers with a fixed text reply.
type discussStubLLM struct {
	mu   sync.Mutex
	reqs []*llm.ReviewRequest
}

func (s *discussStubLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqs = append(s.reqs, req)
	return &llm.ReviewResponse{RawResponse: "the answer"}, nil
}

// End-to-end Discuss turn against a stub LLM: prompt assembly, opener,
// persisted messages, and — the regression this guards — a no-tools fallback
// transcript free of tool plumbing. A resumed session replays prior tool
// exchanges (assistant tool_calls + tool results); sending those with the
// tools field omitted is rejected by strict OpenAI-compatible backends.
func TestDiscussEndToEndWithStubLLM(t *testing.T) {
	llmClient := &discussStubLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	p := 1
	result := &model.ReviewResult{
		ReviewID:           "rev-e2e",
		OverallCorrectness: "patch is incorrect",
		OverallExplanation: "risky",
		Findings:           []model.Finding{{ID: "f1", Title: "Bug", Body: "explodes", Priority: &p}},
	}
	reviewCtx := &model.ReviewContext{
		Mode: model.ModeLocal,
		Diff: "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-old\n+new\n",
	}
	messages := []llm.Message{
		{Role: "user", Content: "why is this a bug?"},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "t1", Name: "inspect_file", Arguments: "{}"}}},
		{Role: "tool", ToolCallID: "t1", Content: "file contents"},
		{Role: "assistant", Content: "because X"},
		{Role: "user", Content: "and how do I fix it?"},
	}

	res, err := engine.Discuss(context.Background(), DiscussRequest{
		ReviewCtx:       reviewCtx,
		Result:          result,
		PinnedFindingID: "f1",
		Messages:        messages,
		Tools:           []llm.ToolDefinition{}, // disabled: no checkout in this test
	})
	if err != nil {
		t.Fatalf("Discuss: %v", err)
	}
	if res.Reply != "the answer" {
		t.Fatalf("reply = %q", res.Reply)
	}
	if !strings.Contains(res.Opener, "Bug") {
		t.Fatalf("pinned opener missing or wrong: %q", res.Opener)
	}
	if len(res.NewMessages) == 0 {
		t.Fatal("no new messages persisted")
	}
	last := res.NewMessages[len(res.NewMessages)-1]
	if last.Role != "assistant" || last.Content != "the answer" {
		t.Fatalf("reply not persisted as final assistant message: %+v", res.NewMessages)
	}

	if len(llmClient.reqs) == 0 {
		t.Fatal("no LLM request captured")
	}
	req := llmClient.reqs[0]
	if len(req.Messages) == 0 || req.Messages[0].Role != "system" || !strings.Contains(req.Messages[0].Content, "rev-e2e") {
		t.Fatal("system prompt missing or does not embed the review JSON")
	}
	// The conversation (including the replayed tool exchange) follows the
	// system prompt and pinned opener verbatim.
	if req.Messages[1].Role != "assistant" || !strings.Contains(req.Messages[1].Content, "Bug") {
		t.Fatalf("pinned opener not injected as first assistant message: %+v", req.Messages[1])
	}
	if req.SchemaKind != llm.SchemaKindText || req.Schema != nil {
		t.Fatalf("discuss must be free-form: kind=%v schema=%v", req.SchemaKind, req.Schema)
	}
	if len(req.NoToolsMessages) == 0 {
		t.Fatal("NoToolsMessages not populated")
	}
	for _, msg := range req.NoToolsMessages {
		if msg.Role == "tool" || len(msg.ToolCalls) > 0 {
			t.Fatalf("no-tools transcript leaks tool plumbing (strict backends reject it): %+v", msg)
		}
	}
}
