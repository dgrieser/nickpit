package review

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
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
	var logs bytes.Buffer
	engine.SetLogger(logging.New(&logs, true, false))
	now := time.Now()
	ctx := context.WithValue(context.Background(), timeBudgetContextKey{}, activeTimeBudget{
		scope:            "unit:soft",
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
	if log := logs.String(); !strings.Contains(log, "Workflow time budget speed-up threshold reached: scope=unit:soft") || !strings.Contains(log, "retrying urgently") {
		t.Fatalf("log = %q, want soft threshold retry with scope", log)
	}
}

func TestReviewWithTimeBudgetThreshold100DoesNotRetryUrgently(t *testing.T) {
	client := &urgentRecordingLLM{wait: true}
	engine := pipelineTestEngine(client)
	var logs bytes.Buffer
	engine.SetLogger(logging.New(&logs, true, false))
	now := time.Now()
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(20*time.Millisecond))
	defer cancel()
	ctx = context.WithValue(ctx, timeBudgetContextKey{}, activeTimeBudget{
		scope:            "unit:hard",
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
	if log := logs.String(); !strings.Contains(log, "Workflow time budget deadline reached: scope=unit:hard") || !strings.Contains(log, "call aborted") {
		t.Fatalf("log = %q, want hard deadline with scope", log)
	}
}

type budgetLogRecorder struct {
	lines []string
}

func (r *budgetLogRecorder) logf(_ context.Context, format string, args ...any) {
	r.lines = append(r.lines, fmt.Sprintf(format, args...))
}

func (r *budgetLogRecorder) contains(fragment string) bool {
	for _, line := range r.lines {
		if strings.Contains(line, fragment) {
			return true
		}
	}
	return false
}

func TestTimeBudgetStartLogsScopeLimitAndThreshold(t *testing.T) {
	rec := &budgetLogRecorder{}
	ctx, cancel, skipped := withConfiguredTimeBudget(
		context.Background(),
		&workflow.TimeBudget{MaxSeconds: intPtr(2), SpeedupThreshold: intPtr(75)},
		childTimePlan{},
		"unit:start",
		rec.logf,
	)
	defer cancel()
	if skipped {
		t.Fatal("budget was skipped")
	}
	if _, ok := timeBudgetFromContext(ctx); !ok {
		t.Fatal("budget missing from context")
	}
	for _, fragment := range []string{
		"Workflow time budget started: scope=unit:start",
		"limit=",
		"deadline_in=",
		"speedup_threshold=75%",
		"speedup_in=",
	} {
		if !rec.contains(fragment) {
			t.Fatalf("logs = %#v, missing %q", rec.lines, fragment)
		}
	}
}

func TestOptionalTimeBudgetSkipLogsScope(t *testing.T) {
	rec := &budgetLogRecorder{}
	now := time.Now()
	parent := context.WithValue(context.Background(), timeBudgetContextKey{}, activeTimeBudget{
		scope:            "unit:parent",
		start:            now.Add(-time.Second),
		deadline:         now.Add(-time.Millisecond),
		speedupThreshold: 80,
	})
	_, _, skipped := withConfiguredTimeBudget(parent, nil, childTimePlan{optional: true}, "unit:optional", rec.logf)
	if !skipped {
		t.Fatal("budget was not skipped")
	}
	if !rec.contains("Workflow time budget skipped: scope=unit:optional reason=parent_deadline_exhausted") {
		t.Fatalf("logs = %#v, want skip with scope", rec.lines)
	}
}

func TestWeightedChildBudgetStartsWhenActivated(t *testing.T) {
	now := time.Now()
	parent := context.WithValue(context.Background(), timeBudgetContextKey{}, activeTimeBudget{
		start:            now,
		deadline:         now.Add(2 * time.Second),
		speedupThreshold: 80,
	})
	budget := &workflow.TimeBudget{Weight: intPtr(10)}
	plans := childTimePlans(parent, []*workflow.TimeBudget{budget})
	if plans[0].allocated == nil || *plans[0].allocated < 190*time.Millisecond || *plans[0].allocated > 210*time.Millisecond {
		t.Fatalf("allocated = %v, want about 200ms", plans[0].allocated)
	}

	time.Sleep(60 * time.Millisecond)
	activatedAt := time.Now()
	child, cancel, skipped := newTimeBudgetStarter(parent, budget, plans[0], true, "unit:child", nil).start()
	defer cancel()
	if skipped {
		t.Fatal("child budget was skipped")
	}
	active, ok := timeBudgetFromContext(child)
	if !ok {
		t.Fatal("child budget missing from context")
	}
	if active.start.Before(activatedAt.Add(-10 * time.Millisecond)) {
		t.Fatalf("child budget started at %v, before activation at %v", active.start, activatedAt)
	}
	if active.deadline.Before(activatedAt.Add(150 * time.Millisecond)) {
		t.Fatalf("child deadline = %v, want around activation+200ms, not original plan time", active.deadline)
	}
}

func TestPipelinePhaseBudgetStartsWhenPhaseStarts(t *testing.T) {
	now := time.Now()
	parent := context.WithValue(context.Background(), timeBudgetContextKey{}, activeTimeBudget{
		start:            now,
		deadline:         now.Add(2 * time.Second),
		speedupThreshold: 80,
	})
	fused := postMergeFusedSpec{
		merge:        workflow.StepEntry{Config: &workflow.StepOverride{TimeBudget: &workflow.TimeBudget{Weight: intPtr(35)}}},
		finalize:     workflow.StepEntry{Config: &workflow.StepOverride{TimeBudget: &workflow.TimeBudget{Weight: intPtr(35)}}},
		verdict:      workflow.StepEntry{Config: &workflow.StepOverride{TimeBudget: &workflow.TimeBudget{Weight: intPtr(20)}}},
		summarize:    workflow.StepEntry{Config: &workflow.StepOverride{TimeBudget: &workflow.TimeBudget{Weight: intPtr(10)}}},
		hasSummarize: true,
	}
	_, _, _, summarizeBudget := pipelinePhaseBudgets(parent, fused, false, nil)

	time.Sleep(60 * time.Millisecond)
	activatedAt := time.Now()
	summarizeCtx, cancel, skipped := summarizeBudget.start()
	defer cancel()
	if skipped {
		t.Fatal("summarize budget was skipped")
	}
	active, ok := timeBudgetFromContext(summarizeCtx)
	if !ok {
		t.Fatal("summarize budget missing from context")
	}
	if active.start.Before(activatedAt.Add(-10 * time.Millisecond)) {
		t.Fatalf("summarize budget started at %v, before activation at %v", active.start, activatedAt)
	}
	if active.deadline.Before(activatedAt.Add(150 * time.Millisecond)) {
		t.Fatalf("summarize deadline = %v, want around activation+200ms", active.deadline)
	}
}

func TestReviewSubphaseBudgetStartsWhenSubphaseStarts(t *testing.T) {
	now := time.Now()
	parent := context.WithValue(context.Background(), timeBudgetContextKey{}, activeTimeBudget{
		start:            now,
		deadline:         now.Add(2 * time.Second),
		speedupThreshold: 80,
	})
	override := &workflow.StepOverride{
		TimeBudget: &workflow.TimeBudget{Weight: intPtr(70)},
		Nudge:      &workflow.AgentOverride{TimeBudget: &workflow.TimeBudget{Weight: intPtr(10)}},
	}
	_, _, _, nudgeBudget := reviewPhaseBudgetStarters(parent, "testing", override, modelReviewRequestWithNudges(), false, nil)

	time.Sleep(60 * time.Millisecond)
	activatedAt := time.Now()
	nudgeCtx, cancel, skipped := nudgeBudget.start()
	defer cancel()
	if skipped {
		t.Fatal("nudge budget was skipped")
	}
	active, ok := timeBudgetFromContext(nudgeCtx)
	if !ok {
		t.Fatal("nudge budget missing from context")
	}
	if active.start.Before(activatedAt.Add(-10 * time.Millisecond)) {
		t.Fatalf("nudge budget started at %v, before activation at %v", active.start, activatedAt)
	}
	if active.deadline.Before(activatedAt.Add(150 * time.Millisecond)) {
		t.Fatalf("nudge deadline = %v, want around activation+200ms", active.deadline)
	}
}

func modelReviewRequestWithNudges() model.ReviewRequest {
	return model.ReviewRequest{NudgeCount: 1}
}
