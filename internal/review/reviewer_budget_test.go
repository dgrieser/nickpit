package review

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
)

type budgetRoundClient struct {
	mu       sync.Mutex
	requests []llm.ReviewRequest
	respond  func(context.Context, *llm.ReviewRequest) (*llm.ReviewResponse, error)
}

func (c *budgetRoundClient) Review(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	c.mu.Lock()
	copyReq := *req
	copyReq.Messages = append([]llm.Message(nil), req.Messages...)
	c.requests = append(c.requests, copyReq)
	c.mu.Unlock()
	return c.respond(ctx, req)
}

func (c *budgetRoundClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.requests)
}

func reviewerBudgetTestContext(t *testing.T, limit time.Duration, threshold int, urgent bool) context.Context {
	t.Helper()
	now := time.Now()
	start := now.Add(-3 * limit)
	if urgent {
		start = now.Add(-5 * limit)
	}
	budget := activeTimeBudget{scope: "step:review:security", start: start, deadline: now.Add(limit), speedupThreshold: threshold}
	ctx, cancel := context.WithDeadlineCause(context.Background(), budget.deadline, &timeBudgetDeadlineCause{scope: budget.scope})
	t.Cleanup(cancel)
	return context.WithValue(ctx, timeBudgetContextKey{}, budget)
}

func budgetFinding(title string) model.Finding {
	return model.Finding{Title: title, Body: title, Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}}
}

func budgetAgent() agentSpec {
	return agentSpec{name: "Security", role: "review", system: "Review with tools", noToolsSystem: "Review supplied evidence", user: "Review patch", hasTools: true, schemaKind: llm.SchemaKindReview}
}

func assertFinalCall(t *testing.T, req *llm.ReviewRequest) {
	t.Helper()
	if !req.Finalize || req.Urgent || len(req.Tools) != 0 || req.ParallelToolCalls || req.ReasoningEffort != "low" {
		t.Fatalf("unexpected final request: finalize=%v urgent=%v tools=%d parallel=%v effort=%q", req.Finalize, req.Urgent, len(req.Tools), req.ParallelToolCalls, req.ReasoningEffort)
	}
	for _, msg := range req.Messages {
		if msg.Role == "tool" || len(msg.ToolCalls) > 0 {
			t.Fatal("final request contains tool protocol")
		}
	}
}

func budgetPrompt(req *llm.ReviewRequest) string {
	var parts []string
	for _, message := range req.Messages {
		parts = append(parts, message.Content)
	}
	return strings.Join(parts, "\n")
}

func TestReviewerBudgetInitialMinesInterruptedNotesAndStopsNudges(t *testing.T) {
	client := &budgetRoundClient{}
	client.respond = func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
		if !req.Finalize {
			req.ReasoningSink.Append("INTERRUPTED_CANDIDATE")
			<-ctx.Done()
			return nil, &llm.InterruptedResponseError{Err: ctx.Err(), RawContent: "PARTIAL_ANSWER", TokensUsed: model.TokenUsage{TotalTokens: 7}}
		}
		assertFinalCall(t, req)
		text := budgetPrompt(req)
		if req.SchemaKind == llm.SchemaKindText {
			if !strings.Contains(text, "INTERRUPTED_CANDIDATE") || !strings.Contains(text, "PARTIAL_ANSWER") {
				t.Fatalf("mining input = %s", text)
			}
			return &llm.ReviewResponse{RawResponse: "MINED_CANDIDATE", TokensUsed: model.TokenUsage{TotalTokens: 3}}, nil
		}
		if !strings.Contains(text, "MINED_CANDIDATE") || !strings.Contains(text, "OLD_CANDIDATE") || !strings.Contains(text, "ONLY this round") {
			t.Fatalf("final input = %s", text)
		}
		return &llm.ReviewResponse{Findings: []model.Finding{budgetFinding("recovered")}, TokensUsed: model.TokenUsage{TotalTokens: 5}}, nil
	}
	e := pipelineTestEngine(client)
	req := model.ReviewRequest{NudgeCount: 3, ForceAllNudges: true, ModelEmitsReasoning: true}
	s := e.newReviewerSession(budgetAgent(), req, false)
	s.collectedLists = []string{"OLD_CANDIDATE"}
	ctx := WithLimiter(reviewerBudgetTestContext(t, time.Second, 80, false), NewLimiter(1))
	b := newTimeBudgetStarter(ctx, nil, childTimePlan{}, false, "", nil)
	if err := e.reviewerInitial(ctx, s, req, b, e, req); err != nil {
		t.Fatal(err)
	}
	if err := e.reviewerNudges(ctx, s, req, b, e, req, b, e, req); err != nil {
		t.Fatal(err)
	}
	if client.count() != 3 || len(s.totalFindings) != 1 || s.budgetStop == nil || s.nudgeTurns != 0 {
		t.Fatalf("calls=%d findings=%d stop=%+v nudges=%d", client.count(), len(s.totalFindings), s.budgetStop, s.nudgeTurns)
	}
	if usage := s.result(req).run.TokensUsed.TotalTokens; usage != 15 {
		t.Fatalf("usage=%d, want 15", usage)
	}
	// A fresh standalone-step budget cannot reopen a closed reviewer.
	st := &PipelineState{groupByID: make(map[string]*groupEntry)}
	st.setGroup("security", s.result(req), s)
	if err := e.extractStepFunc("security")(context.Background(), &stepContext{Engine: e, Req: req}, st); err != nil {
		t.Fatal(err)
	}
	if err := e.nudgeStepFunc("security")(context.Background(), &stepContext{Engine: e, Req: req}, st); err != nil {
		t.Fatal(err)
	}
	if client.count() != 3 {
		t.Fatal("standalone step reopened reviewer")
	}
}

func TestReviewerBudgetFinishesOnlyActiveNudge(t *testing.T) {
	initial := budgetFinding("existing")
	client := &budgetRoundClient{}
	initialCall := true
	client.respond = func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
		if initialCall {
			initialCall = false
			return &llm.ReviewResponse{Findings: []model.Finding{initial}}, nil
		}
		if !req.Finalize {
			req.ReasoningSink.Append("NUDGE_NOTE")
			<-ctx.Done()
			return nil, ctx.Err()
		}
		assertFinalCall(t, req)
		text := budgetPrompt(req)
		if !strings.Contains(text, "NUDGE_NOTE") || !strings.Contains(text, "existing") || !strings.Contains(text, "current nudge 1") {
			t.Fatalf("nudge final input = %s", text)
		}
		return &llm.ReviewResponse{Findings: []model.Finding{initial, budgetFinding("new")}}, nil
	}
	e := pipelineTestEngine(client)
	req := model.ReviewRequest{NudgeCount: 4, ForceAllNudges: true, DisableReasoningExtract: true}
	s := e.newReviewerSession(budgetAgent(), req, false)
	b := newTimeBudgetStarter(context.Background(), nil, childTimePlan{}, false, "", nil)
	if err := e.reviewerInitial(context.Background(), s, req, b, e, req); err != nil {
		t.Fatal(err)
	}
	ctx := reviewerBudgetTestContext(t, time.Second, 80, false)
	if err := e.reviewerNudges(ctx, s, req, b, e, req, newTimeBudgetStarter(ctx, nil, childTimePlan{}, false, "", nil), e, req); err != nil {
		t.Fatal(err)
	}
	if client.count() != 3 || s.nudgeTurns != 1 || len(s.totalFindings) != 2 || s.budgetStop.NudgeIndex != 1 {
		t.Fatalf("calls=%d nudges=%d findings=%d stop=%+v", client.count(), s.nudgeTurns, len(s.totalFindings), s.budgetStop)
	}
}

func TestReviewerBudgetMiningFallback(t *testing.T) {
	for _, mode := range []string{"error", "timeout", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			client := &budgetRoundClient{}
			client.respond = func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
				assertFinalCall(t, req)
				if req.SchemaKind == llm.SchemaKindText {
					if mode == "timeout" {
						<-ctx.Done()
						return nil, ctx.Err()
					}
					return nil, errors.New("miner unavailable")
				}
				if ctx.Err() != nil || !strings.Contains(budgetPrompt(req), "UNMINED_NOTE") {
					t.Fatal("fallback lost notes or finalization budget")
				}
				return &llm.ReviewResponse{}, nil
			}
			e := pipelineTestEngine(client)
			req := model.ReviewRequest{DisableReasoningExtract: mode == "disabled"}
			s := e.newReviewerSession(budgetAgent(), req, false)
			s.reasoningTraces = []string{"UNMINED_NOTE"}
			loop, sec := e.buildAgentLoopRequest(s.agent, req)
			defer sec.End()
			if _, err := e.runReviewerRound(reviewerBudgetTestContext(t, time.Second, 80, true), s, loop, req, "review", 0, ""); err != nil {
				t.Fatal(err)
			}
			want := 2
			if mode == "disabled" {
				want = 1
			}
			if client.count() != want {
				t.Fatalf("calls=%d, want %d", client.count(), want)
			}
		})
	}
}

func TestReviewerBudgetFinalMiningHonorsPhaseAllocation(t *testing.T) {
	for _, tt := range []struct {
		name       string
		mainWeight *int
		mineWeight *int
		maxSeconds *int
		wantMining bool
	}{
		{name: "small weight", mineWeight: intPtr(1), wantMining: true},
		{name: "optional weight zero", mineWeight: intPtr(0)},
		{name: "optional with absolute cap", mineWeight: intPtr(0), maxSeconds: intPtr(1)},
		{name: "no remaining allocation", mainWeight: intPtr(100)},
		{name: "absolute cap tighter", mineWeight: intPtr(100), maxSeconds: intPtr(1), wantMining: true},
		{name: "final response reserve tighter", mineWeight: intPtr(100), wantMining: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := reviewerBudgetTestContext(t, 8*time.Second, 80, true)
			cfg := &workflow.StepOverride{
				TimeBudget: &workflow.TimeBudget{Weight: tt.mainWeight},
				MineReasoning: &workflow.AgentOverride{TimeBudget: &workflow.TimeBudget{
					Weight: tt.mineWeight, MaxSeconds: tt.maxSeconds,
				}},
			}
			req := model.ReviewRequest{}
			_, mine, _, _ := reviewPhaseBudgetStarters(ctx, "security", cfg, req, false, nil)
			bound := 2 * time.Second // quarter of the round's remaining time
			if mine.plan.allocated != nil {
				bound = min(bound, *mine.plan.allocated)
			}
			if tt.maxSeconds != nil {
				bound = min(bound, time.Duration(*tt.maxSeconds)*time.Second)
			}
			miningCalls := 0
			client := &budgetRoundClient{respond: func(callCtx context.Context, call *llm.ReviewRequest) (*llm.ReviewResponse, error) {
				assertFinalCall(t, call)
				if call.SchemaKind == llm.SchemaKindText {
					miningCalls++
					deadline, ok := callCtx.Deadline()
					if remaining := time.Until(deadline); !ok || remaining <= 0 || remaining > bound {
						t.Fatalf("mining deadline remaining=%s, want 0 < remaining <= %s", remaining, bound)
					}
					return nil, errors.New("use raw notes")
				}
				if callCtx.Err() != nil || !strings.Contains(budgetPrompt(call), "UNMINED_NOTE") {
					t.Fatal("final response lost its budget or unmined notes")
				}
				return &llm.ReviewResponse{}, nil
			}}
			e := pipelineTestEngine(client)
			s := e.newReviewerSession(budgetAgent(), req, false)
			s.mineBudget = mine
			s.reasoningTraces = []string{"UNMINED_NOTE"}
			loop, sec := e.buildAgentLoopRequest(s.agent, req)
			defer sec.End()
			if _, err := e.runReviewerRound(ctx, s, loop, req, "review", 0, ""); err != nil {
				t.Fatal(err)
			}
			wantMining := 0
			if tt.wantMining {
				wantMining = 1
			}
			if miningCalls != wantMining || client.count() != wantMining+1 {
				t.Fatalf("mining calls=%d total=%d, want mining=%d plus one final response", miningCalls, client.count(), wantMining)
			}
		})
	}
}

func TestReviewerBudgetRepairBoundAndPartialRecovery(t *testing.T) {
	for _, repairAllowed := range []bool{true, false} {
		t.Run(map[bool]string{true: "one repair", false: "allowance spent"}[repairAllowed], func(t *testing.T) {
			client := &budgetRoundClient{respond: func(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
				assertFinalCall(t, req)
				return nil, &llm.InvalidResponseError{Reason: "missing priority", RawContent: "{}", PartialResponse: &llm.ReviewResponse{Findings: []model.Finding{budgetFinding("valid"), {Title: "invalid"}}}}
			}}
			e := pipelineTestEngine(client)
			req := model.ReviewRequest{MaxOutputRetries: 1}
			s := e.newReviewerSession(budgetAgent(), req, false)
			loop, sec := e.buildAgentLoopRequest(s.agent, req)
			defer sec.End()
			loop.State = newAgentLoopState()
			if !repairAllowed {
				loop.State.jsonRetries = 1
			}
			result, err := e.runReviewerRound(reviewerBudgetTestContext(t, time.Second, 80, true), s, loop, req, "review", 0, "")
			want := 1
			if repairAllowed {
				want = 2
			}
			if err == nil || client.count() != want || result.resp == nil || len(result.resp.Findings) != 1 {
				t.Fatalf("err=%v calls=%d response=%+v", err, client.count(), result.resp)
			}
		})
	}
}

func TestReviewerBudgetDeadlineHasNoDuplicateReviewWarning(t *testing.T) {
	for _, threshold := range []int{80, 100} {
		t.Run(model.HumanTokens(threshold), func(t *testing.T) {
			client := &budgetRoundClient{respond: func(ctx context.Context, _ *llm.ReviewRequest) (*llm.ReviewResponse, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			}}
			e := pipelineTestEngine(client)
			req := model.ReviewRequest{}
			s := e.newReviewerSession(budgetAgent(), req, false)
			warnings := &warningLog{}
			ctx := withWarnings(reviewerBudgetTestContext(t, 30*time.Millisecond, threshold, true), warnings)
			b := newTimeBudgetStarter(ctx, nil, childTimePlan{}, false, "", nil)
			if err := e.reviewerInitial(ctx, s, req, b, e, req); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("err=%v", err)
			}
			run := s.partialResult(req).run
			run.Status = model.AgentRunStatusFailed
			if !s.budgetError || run.BudgetStop == nil || run.BudgetStop.Reason != "deadline" || agentRunWarning(run) != "" {
				t.Fatalf("run=%+v budgetError=%v", run, s.budgetError)
			}
			count := 0
			for _, w := range appendAgentRunWarnings(warnings.list(), []model.AgentRun{run}, nil) {
				if strings.Contains(w, "deadline reached") {
					count++
				}
				if strings.Contains(w, "reviewer failed") {
					t.Fatal(w)
				}
			}
			if count != 1 || client.count() != 1 {
				t.Fatalf("deadline warnings=%d calls=%d", count, client.count())
			}
			run.Error = "independent validation failure"
			if agentRunWarning(run) == "" {
				t.Fatal("budget metadata hid independent failure")
			}
		})
	}
}

func TestReviewerBudgetProviderTimeoutAndUserCancelStayDistinct(t *testing.T) {
	for _, cancelParent := range []bool{false, true} {
		client := &budgetRoundClient{respond: func(_ context.Context, _ *llm.ReviewRequest) (*llm.ReviewResponse, error) {
			return nil, context.DeadlineExceeded
		}}
		e := pipelineTestEngine(client)
		req := model.ReviewRequest{}
		s := e.newReviewerSession(budgetAgent(), req, false)
		loop, sec := e.buildAgentLoopRequest(s.agent, req)
		ctx, cancel := context.WithCancel(reviewerBudgetTestContext(t, time.Second, 80, false))
		if cancelParent {
			cancel()
		}
		_, err := e.runReviewerRound(ctx, s, loop, req, "review", 0, "")
		cancel()
		sec.End()
		if err == nil || s.budgetStop != nil || s.budgetError {
			t.Fatalf("err=%v stop=%+v", err, s.budgetStop)
		}
	}
}

func TestReviewerBudgetCancelsQueuedCollectorsAtConcurrencyOne(t *testing.T) {
	var normalCalls atomic.Int32
	client := &budgetRoundClient{respond: func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
		if req.SchemaKind == llm.SchemaKindText {
			if !req.Finalize {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			if text := budgetPrompt(req); !strings.Contains(text, "FIRST_TRACE") || !strings.Contains(text, "LAST_TRACE") {
				t.Fatalf("unmined traces missing: %s", text)
			}
			return &llm.ReviewResponse{RawResponse: "NONE"}, nil
		}
		if req.Finalize {
			assertFinalCall(t, req)
			return &llm.ReviewResponse{}, nil
		}
		if normalCalls.Add(1) == 1 {
			req.ReasoningSink.Append("FIRST_TRACE")
			return &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "read", Name: "inspect_file", Arguments: `{"path":"main.go"}`}}}, nil
		}
		req.ReasoningSink.Append("LAST_TRACE")
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	e := pipelineTestEngine(client)
	req := model.ReviewRequest{RepoRoot: ".", NudgeCount: 2, ModelEmitsReasoning: true}
	s := e.newReviewerSession(budgetAgent(), req, false)
	ctx := WithLimiter(reviewerBudgetTestContext(t, time.Second, 80, false), NewLimiter(1))
	b := newTimeBudgetStarter(ctx, nil, childTimePlan{}, false, "", nil)
	if err := e.reviewerInitial(ctx, s, req, b, e, req); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != nil || s.budgetStop == nil || normalCalls.Load() != 2 {
		t.Fatalf("deadline=%v stop=%+v calls=%d", ctx.Err(), s.budgetStop, normalCalls.Load())
	}
}

func TestReviewerBudgetRepairSuccessAndDisabledTools(t *testing.T) {
	var calls int
	client := &budgetRoundClient{respond: func(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
		assertFinalCall(t, req)
		calls++
		if !strings.Contains(budgetPrompt(req), "TOOL_EVIDENCE") {
			t.Fatal("tool evidence lost")
		}
		if calls == 1 {
			return &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "forbidden", Name: "inspect_file", Arguments: `{"path":"main.go"}`}}}, nil
		}
		return &llm.ReviewResponse{Findings: []model.Finding{budgetFinding("repaired")}}, nil
	}}
	e := pipelineTestEngine(client)
	req := model.ReviewRequest{MaxOutputRetries: 1}
	s := e.newReviewerSession(budgetAgent(), req, false)
	loop, sec := e.buildAgentLoopRequest(s.agent, req)
	defer sec.End()
	loop.Messages = append(loop.Messages, llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "old", Name: "inspect_file"}}}, llm.Message{Role: "tool", ToolCallID: "old", Content: "TOOL_EVIDENCE"})
	result, err := e.runReviewerRound(reviewerBudgetTestContext(t, time.Second, 80, true), s, loop, req, "review", 0, "")
	if err != nil || calls != 2 || result.resp == nil || len(result.resp.Findings) != 1 || result.toolCalls != 0 {
		t.Fatalf("err=%v calls=%d result=%+v", err, calls, result)
	}
}

func TestReviewerBudgetKeepsValidFindingsWhenRepairHitsHardDeadline(t *testing.T) {
	var calls int
	client := &budgetRoundClient{respond: func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
		calls++
		if calls == 1 {
			return nil, &llm.InvalidResponseError{Reason: "incomplete answer", RawContent: "{}", PartialResponse: &llm.ReviewResponse{Findings: []model.Finding{budgetFinding("retained")}}}
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	e := pipelineTestEngine(client)
	req := model.ReviewRequest{MaxOutputRetries: 1}
	s := e.newReviewerSession(budgetAgent(), req, false)
	ctx := reviewerBudgetTestContext(t, 40*time.Millisecond, 80, true)
	b := newTimeBudgetStarter(ctx, nil, childTimePlan{}, false, "", nil)
	if err := e.reviewerInitial(ctx, s, req, b, e, req); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	result := s.result(req)
	if len(result.resp.Findings) != 1 || result.run.Status != model.AgentRunStatusPartial || result.run.Error != "" || result.run.BudgetStop.Reason != "deadline" {
		t.Fatalf("result=%+v", result)
	}
}

func TestReviewerBudgetStopsBetweenRoundsAndHonorsParentDeadline(t *testing.T) {
	client := &budgetRoundClient{respond: func(context.Context, *llm.ReviewRequest) (*llm.ReviewResponse, error) {
		t.Fatal("unexpected model call")
		return nil, nil
	}}
	e := pipelineTestEngine(client)
	req := model.ReviewRequest{NudgeCount: 3}
	s := e.newReviewerSession(budgetAgent(), req, false)
	s.latestResp = &llm.ReviewResponse{}
	ctx := reviewerBudgetTestContext(t, time.Second, 80, true)
	b := newTimeBudgetStarter(ctx, nil, childTimePlan{}, false, "", nil)
	if err := e.reviewerNudges(ctx, s, req, b, e, req, b, e, req); err != nil {
		t.Fatal(err)
	}
	if s.budgetStop == nil || client.count() != 0 {
		t.Fatal("started nudge beyond threshold")
	}
	parent := reviewerBudgetTestContext(t, 20*time.Millisecond, 100, false)
	child, cancel, skipped := withConfiguredTimeBudget(parent, nil, childTimePlan{}, "child", nil)
	defer cancel()
	if skipped {
		t.Fatal("unexpected skip")
	}
	parentDeadline, _ := parent.Deadline()
	childDeadline, _ := child.Deadline()
	if !childDeadline.Equal(parentDeadline) {
		t.Fatal("child extended parent deadline")
	}
	<-child.Done()
	if !isTimeBudgetDeadline(child) {
		t.Fatal("lost typed budget cause")
	}
}

func TestReviewerBudgetWorkflowKeepsClosedSessionOutcome(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "recovered", true: "provider failure"}[failure], func(t *testing.T) {
			client := &budgetRoundClient{respond: func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
				if !req.Finalize {
					req.ReasoningSink.Append("candidate")
					<-ctx.Done()
					return nil, ctx.Err()
				}
				if failure {
					return nil, errors.New("provider unavailable")
				}
				return &llm.ReviewResponse{Findings: []model.Finding{budgetFinding("recovered")}}, nil
			}}
			e := pipelineTestEngine(client)
			spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{
				{Type: "review:security", Config: &workflow.StepOverride{NudgeCount: intPtr(3), TimeBudget: &workflow.TimeBudget{MaxSeconds: intPtr(1)}}},
				{Type: "reasoning-extract:security"}, {Type: "nudge:security"},
			}}
			pipeline, err := e.BuildPipeline(spec)
			if err != nil {
				t.Fatal(err)
			}
			result, _, err := e.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal, RepoRoot: ".", DisableDiffScope: true, DisableReasoningExtract: true})
			if err != nil {
				t.Fatal(err)
			}
			if client.count() != 2 || len(result.AgentRuns) != 1 {
				t.Fatalf("calls=%d runs=%+v", client.count(), result.AgentRuns)
			}
			run := result.AgentRuns[0]
			if run.BudgetStop == nil {
				t.Fatal("budget metadata lost")
			}
			if failure {
				if run.Status != model.AgentRunStatusFailed || !strings.Contains(run.Error, "provider unavailable") || agentRunWarning(run) == "" {
					t.Fatalf("failure hidden: %+v", run)
				}
			} else if run.Status != model.AgentRunStatusPartial || len(result.Findings) != 1 || run.Error != "" || agentRunWarning(run) != "" {
				t.Fatalf("recovery lost: %+v", run)
			}
		})
	}
}
