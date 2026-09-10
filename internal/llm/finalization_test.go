package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFinalizationEffortCapabilities(t *testing.T) {
	for _, tt := range []struct {
		original string
		allowed  []string
		want     string
	}{
		{"high", nil, "low"},
		{"none", nil, "none"},
		{"minimal", []string{"minimal", "low", "high"}, "minimal"},
		{"high", []string{"off", "high"}, "off"},
		{"high", []string{"medium", "high"}, "medium"},
		{"low", []string{"provider-only"}, "provider-only"},
	} {
		var allowed map[string]struct{}
		if tt.allowed != nil {
			allowed = make(map[string]struct{})
			for _, effort := range tt.allowed {
				allowed[effort] = struct{}{}
			}
		}
		if got := finalizationReasoningEffort(tt.original, allowed); got != tt.want {
			t.Errorf("effort(%q,%v)=%q, want %q", tt.original, tt.allowed, got, tt.want)
		}
	}
}

func TestFinalizationDoesNotClimbOrRepeatEffort(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload["reasoning_effort"] != "low" {
			t.Errorf("effort=%v", payload["reasoning_effort"])
		}
		if _, ok := payload["tools"]; ok {
			t.Error("provider extras restored tools")
		}
		if payload["parallel_tool_calls"] == true {
			t.Error("provider extras enabled parallel tools")
		}
		writeReasoningLengthSSE(t, w)
	}))
	defer server.Close()
	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(context.Background(), &ReviewRequest{SystemPrompt: "system", UserContent: "finalize", ReasoningEffort: "high", Finalize: true,
		ExtraBody: map[string]any{"tools": []any{}, "parallel_tool_calls": true, "reasoning_effort": "high"}})
	var exhausted *ReasoningBudgetExhaustedError
	if !errors.As(err, &exhausted) || calls.Load() != 1 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
}

type cancelReasoningSink struct{ cancel context.CancelFunc }

func (s cancelReasoningSink) Append(string) { s.cancel() }
func (s cancelReasoningSink) End()          {}

func TestInterruptedReviewPreservesOutputAndUsage(t *testing.T) {
	const content = `{"findings":[{"title":"candidate","body":"evidence","priority":2}],"overall_correctness":"patch is incorrect","overall_explanation":"issue","overall_confidence_score":0.9}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEChunk(t, w, map[string]any{"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": content}}}})
		writeSSEChunk(t, w, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 4, "completion_tokens": 2, "total_tokens": 6}})
		writeSSEChunk(t, w, map[string]any{"choices": []map[string]any{{"index": 0, "delta": map[string]any{"reasoning_content": "cancel now"}}}})
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := NewOpenAIClient(server.URL, "token", "model")
	_, err := client.Review(ctx, &ReviewRequest{SystemPrompt: "system", UserContent: "review", SchemaKind: SchemaKindReview, ReasoningSink: cancelReasoningSink{cancel: cancel}})
	var interrupted *InterruptedResponseError
	if !errors.As(err, &interrupted) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if interrupted.RawContent != content || interrupted.TokensUsed.TotalTokens != 6 || interrupted.PartialResponse == nil || len(interrupted.PartialResponse.Findings) != 1 {
		t.Fatalf("diagnostic=%+v", interrupted)
	}
}
