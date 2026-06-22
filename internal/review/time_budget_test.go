package review

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
)

type urgentRecordingLLM struct {
	mu     sync.Mutex
	calls  []bool
	wait   bool
	result *llm.ReviewResponse
}

func (u *urgentRecordingLLM) Review(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	u.mu.Lock()
	u.calls = append(u.calls, req.Urgent)
	u.mu.Unlock()
	if u.wait && !req.Urgent {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if u.result != nil {
		return u.result, nil
	}
	return &llm.ReviewResponse{}, nil
}

func (u *urgentRecordingLLM) snapshot() []bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]bool(nil), u.calls...)
}

func TestReviewWithTimeBudgetRetriesUrgentlyAtThreshold(t *testing.T) {
	client := &urgentRecordingLLM{wait: true}
	engine := pipelineTestEngine(client)
	now := time.Now()
	ctx := context.WithValue(context.Background(), timeBudgetContextKey{}, activeTimeBudget{
		start:            now,
		deadline:         now.Add(80 * time.Millisecond),
		speedupThreshold: 50,
	})

	if _, err := engine.reviewWithTimeBudget(ctx, &llm.ReviewRequest{}); err != nil {
		t.Fatal(err)
	}
	got := client.snapshot()
	if len(got) != 2 || got[0] || !got[1] {
		t.Fatalf("urgent calls = %v, want [false true]", got)
	}
}

func TestReviewWithTimeBudgetThreshold100DoesNotRetryUrgently(t *testing.T) {
	client := &urgentRecordingLLM{wait: true}
	engine := pipelineTestEngine(client)
	now := time.Now()
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(20*time.Millisecond))
	defer cancel()
	ctx = context.WithValue(ctx, timeBudgetContextKey{}, activeTimeBudget{
		start:            now,
		deadline:         now.Add(20 * time.Millisecond),
		speedupThreshold: 100,
	})

	if _, err := engine.reviewWithTimeBudget(ctx, &llm.ReviewRequest{}); err == nil {
		t.Fatal("expected hard deadline error")
	}
	got := client.snapshot()
	if len(got) != 1 || got[0] {
		t.Fatalf("urgent calls = %v, want [false]", got)
	}
}
