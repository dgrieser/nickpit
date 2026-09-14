package review

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
)

func nonReviewerBudgetContext(t *testing.T, urgent bool) context.Context {
	return context.WithValue(reviewerBudgetTestContext(t, time.Second, 80, urgent), agentBudgetEnabledKey{}, true)
}

func nonReviewerBudgetRequest(kind string) agentLoopRequest {
	return agentLoopRequest{
		AgentName: kind, AgentKind: kind, Model: "primary", ReasoningEffort: "high",
		SchemaKind: llm.SchemaKindText, State: newAgentLoopState(), MaxOutputRetries: 3,
		Messages: []llm.Message{{Role: "system", Content: "Original task"}, {Role: "user", Content: "ORIGINAL_EVIDENCE"}},
	}
}

func TestAgentBudgetSummaryUsesSmallEndpointAndAccountsUsage(t *testing.T) {
	for _, kind := range []string{"context", "verify"} {
		t.Run(kind, func(t *testing.T) {
			primary := &budgetRoundClient{}
			primary.respond = func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
				if !req.Finalize {
					req.ReasoningSink.Append("CAPTURED_NOTES")
					<-ctx.Done()
					return nil, &llm.InterruptedResponseError{Err: ctx.Err(), RawContent: "PARTIAL_OUTPUT", TokensUsed: model.TokenUsage{TotalTokens: 7}}
				}
				assertFinalCall(t, req)
				if text := budgetPrompt(req); !strings.Contains(text, "CONDENSED_NOTES") || !strings.Contains(text, "ORIGINAL_EVIDENCE") || strings.Contains(text, "CAPTURED_NOTES") {
					t.Fatalf("final prompt = %s", text)
				}
				return &llm.ReviewResponse{RawResponse: "FINAL", TokensUsed: model.TokenUsage{TotalTokens: 5}}, nil
			}
			small := &budgetRoundClient{respond: func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
				if !req.Finalize || req.Model != "small" || req.ReasoningEffort != "none" || len(req.Tools) != 0 {
					t.Fatalf("summary request: %+v", req)
				}
				if text := budgetPrompt(req); !strings.Contains(text, "CAPTURED_NOTES") || !strings.Contains(text, "PARTIAL_OUTPUT") {
					t.Fatalf("summary prompt = %s", text)
				}
				if deadline, _ := ctx.Deadline(); time.Until(deadline) > 300*time.Millisecond {
					t.Fatal("summary did not use urgent remainder")
				}
				return &llm.ReviewResponse{RawResponse: "CONDENSED_NOTES", TokensUsed: model.TokenUsage{TotalTokens: 3}}, nil
			}}
			profile := config.Profile{Model: "primary", BaseURL: "http://primary.invalid", Small: config.SmallModelConfig{Model: "small", BaseURL: "http://small.invalid", ReasoningEffort: "none"}}
			e := NewEngine(stubSource{}, primary, stubRetrieval{}, profile)
			e.SetSmallClient(small, config.EffectiveSmallProfile(profile))
			ctx := WithLimiter(nonReviewerBudgetContext(t, false), NewLimiter(1))
			req := nonReviewerBudgetRequest(kind)
			result, err := e.runAgentLoop(ctx, req)
			if err != nil || result.resp.RawResponse != "FINAL" || result.tokensUsed.TotalTokens != 15 || primary.count() != 2 || small.count() != 1 || result.budgetStop == nil || result.budgetStop.Reason != "finalized" {
				t.Fatalf("result=%+v err=%v calls=%d/%d", result, err, primary.count(), small.count())
			}
			if _, err := e.runAgentLoop(ctx, req); err == nil || primary.count() != 2 {
				t.Fatal("stopped agent reopened")
			}
		})
	}
}

func TestAgentBudgetDirectFinalizationRoles(t *testing.T) {
	for _, kind := range []string{"context", "verify", "categorize", "dedupe", "merge", "finalize", "verdict", "summarize", "extract"} {
		t.Run(kind, func(t *testing.T) {
			client := &budgetRoundClient{respond: func(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
				assertFinalCall(t, req)
				return &llm.ReviewResponse{RawResponse: "FINAL"}, nil
			}}
			result, err := pipelineTestEngine(client).runAgentLoop(nonReviewerBudgetContext(t, true), nonReviewerBudgetRequest(kind))
			if err != nil || client.count() != 1 || result.budgetStop == nil {
				t.Fatalf("result=%+v err=%v calls=%d", result, err, client.count())
			}
		})
	}
}

func TestAgentBudgetSummaryFallback(t *testing.T) {
	for _, mode := range []string{"disabled", "zero", "error", "timeout", "empty", "direct"} {
		t.Run(mode, func(t *testing.T) {
			helperCalls := 0
			client := &budgetRoundClient{respond: func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
				if !req.Finalize {
					req.ReasoningSink.Append("RAW_NOTES")
					<-ctx.Done()
					return nil, &llm.InterruptedResponseError{Err: ctx.Err(), RawContent: "PARTIAL"}
				}
				if strings.HasPrefix(req.Messages[0].Content, "Summarize an engineer") {
					helperCalls++
					if mode == "timeout" {
						<-ctx.Done()
						return nil, &llm.InterruptedResponseError{Err: ctx.Err(), TokensUsed: model.TokenUsage{TotalTokens: 3}}
					}
					if mode == "empty" {
						return &llm.ReviewResponse{}, nil
					}
					return nil, errors.New("summary unavailable")
				}
				assertFinalCall(t, req)
				if text := budgetPrompt(req); !strings.Contains(text, "RAW_NOTES") || !strings.Contains(text, "PARTIAL") {
					t.Fatalf("fallback lost notes: %s", text)
				}
				return &llm.ReviewResponse{RawResponse: "FINAL", TokensUsed: model.TokenUsage{TotalTokens: 5}}, nil
			}}
			e := pipelineTestEngine(client)
			e.disableBudgetSummary = mode == "disabled"
			if mode == "zero" {
				zero := 0
				e.budgetSummary = workflow.ReasoningSummaryOverride(&workflow.StepOverride{SummarizeReasoning: &workflow.AgentOverride{TimeBudget: &workflow.TimeBudget{Weight: &zero}}})
			}
			kind := "context"
			if mode == "direct" {
				kind = "merge"
			}
			result, err := e.runAgentLoop(nonReviewerBudgetContext(t, false), nonReviewerBudgetRequest(kind))
			wantHelpers := 1
			if mode == "disabled" || mode == "zero" || mode == "direct" {
				wantHelpers = 0
			}
			if err != nil || helperCalls != wantHelpers || result.budgetStop.Reason != "finalized" {
				t.Fatalf("result=%+v err=%v helpers=%d", result, err, helperCalls)
			}
			if mode == "timeout" && result.tokensUsed.TotalTokens != 8 {
				t.Fatalf("usage=%+v", result.tokensUsed)
			}
		})
	}
}

func TestAgentBudgetRepairAllowanceAndValidation(t *testing.T) {
	for _, exhausted := range []bool{false, true} {
		client := &budgetRoundClient{respond: func(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
			assertFinalCall(t, req)
			return &llm.ReviewResponse{RawResponse: "incomplete", TokensUsed: model.TokenUsage{TotalTokens: 2}}, nil
		}}
		req := nonReviewerBudgetRequest("verify")
		req.SchemaKind = llm.SchemaKindVerify
		if exhausted {
			req.State.jsonRetries = req.MaxOutputRetries
		}
		result, err := pipelineTestEngine(client).runAgentLoop(nonReviewerBudgetContext(t, true), req)
		want := 2
		if exhausted {
			want = 1
		}
		var invalid *llm.InvalidResponseError
		if !errors.As(err, &invalid) || client.count() != want || result.tokensUsed.TotalTokens != want*2 {
			t.Fatalf("exhausted=%v result=%+v err=%v calls=%d", exhausted, result, err, client.count())
		}
	}
	for _, req := range []agentLoopRequest{
		{SchemaKind: llm.SchemaKindText}, {SchemaKind: llm.SchemaKindVerify}, {SchemaKind: llm.SchemaKindCategorize},
		{ValidateResponse: func(*llm.ReviewResponse) *llm.InvalidResponseError {
			return &llm.InvalidResponseError{Reason: "custom"}
		}},
	} {
		if validateBudgetResponse(req, &llm.ReviewResponse{}) == nil {
			t.Fatalf("accepted invalid %+v", req)
		}
		if validateBudgetResponse(req, &llm.ReviewResponse{RawResponse: "text", ToolCalls: []llm.ToolCall{{Name: "tool"}}}) == nil {
			t.Fatal("accepted tool calls")
		}
	}
}

func TestAgentBudgetCancellationAndOptIn(t *testing.T) {
	client := &budgetRoundClient{respond: func(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
		if req.Finalize {
			t.Fatal("unexpected finalization outside review pipeline")
		}
		return &llm.ReviewResponse{RawResponse: "normal"}, nil
	}}
	e := pipelineTestEngine(client)
	ctx, cancel := context.WithCancel(nonReviewerBudgetContext(t, true))
	cancel()
	if _, err := e.runAgentLoop(ctx, nonReviewerBudgetRequest("context")); !errors.Is(err, context.Canceled) || client.count() != 0 {
		t.Fatalf("cancel: %v calls=%d", err, client.count())
	}
	for _, ctx := range []context.Context{context.Background(), context.WithValue(nonReviewerBudgetContext(t, false), agentBudgetEnabledKey{}, false)} {
		if _, err := e.runAgentLoop(ctx, nonReviewerBudgetRequest("context")); err != nil {
			t.Fatal(err)
		}
	}
	ctx = context.WithValue(reviewerBudgetTestContext(t, time.Second, 100, false), agentBudgetEnabledKey{}, true)
	if _, err := e.runAgentLoop(ctx, nonReviewerBudgetRequest("context")); err != nil {
		t.Fatal(err)
	}
}

func TestAgentBudgetTrackerDeadlinePrecedence(t *testing.T) {
	tracker := &agentBudgetTracker{}
	tracker.record(&model.BudgetStop{Reason: "finalized"})
	tracker.record(&model.BudgetStop{Reason: "deadline"})
	tracker.record(&model.BudgetStop{Reason: "finalized"})
	if got := tracker.result(context.Background(), "verify"); got.Reason != "deadline" {
		t.Fatalf("stop=%+v", got)
	}
}

func TestAgentBudgetCancelsToolsAndPreservesCompletedEvidence(t *testing.T) {
	client := &budgetRoundClient{respond: func(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
		if !req.Finalize {
			req.ReasoningSink.Append("TOOL_PLAN")
			return &llm.ReviewResponse{ToolCalls: []llm.ToolCall{
				{ID: "one", Name: "completed", Arguments: `{}`},
				{ID: "two", Name: "blocked", Arguments: `{}`},
				{ID: "three", Name: "queued", Arguments: `{}`},
			}}, nil
		}
		assertFinalCall(t, req)
		if text := budgetPrompt(req); !strings.Contains(text, "COMPLETED_EVIDENCE") || !strings.Contains(text, "TOOL_PLAN") {
			t.Fatalf("lost evidence: %s", text)
		}
		return &llm.ReviewResponse{RawResponse: "FINAL"}, nil
	}}
	req := nonReviewerBudgetRequest("merge")
	req.Tools = []llm.ToolDefinition{{Name: "completed"}, {Name: "blocked"}, {Name: "queued"}}
	req.ToolHandlers = map[string]func(context.Context, llm.ToolCall) (string, error){
		"completed": func(context.Context, llm.ToolCall) (string, error) { return "COMPLETED_EVIDENCE", nil },
		"blocked":   func(ctx context.Context, _ llm.ToolCall) (string, error) { <-ctx.Done(); return "", ctx.Err() },
		"queued": func(context.Context, llm.ToolCall) (string, error) {
			t.Fatal("queued tool dispatched after cancellation")
			return "", nil
		},
	}
	result, err := pipelineTestEngine(client).runAgentLoop(nonReviewerBudgetContext(t, false), req)
	if err != nil || client.count() != 2 || len(result.toolMessages) != 3 {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, client.count())
	}
}

func TestAgentBudgetToolLimitFallbackCapturesInterruptedNotes(t *testing.T) {
	client := &budgetRoundClient{respond: func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
		if len(req.Tools) > 0 {
			return &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "one", Name: "custom", Arguments: `{}`}, {ID: "two", Name: "custom", Arguments: `{}`}}, TokensUsed: model.TokenUsage{TotalTokens: 2}}, nil
		}
		if !req.Finalize {
			req.ReasoningSink.Append("FALLBACK_REASONING")
			<-ctx.Done()
			return nil, &llm.InterruptedResponseError{Err: ctx.Err(), RawContent: "FALLBACK_PARTIAL", TokensUsed: model.TokenUsage{TotalTokens: 3}}
		}
		assertFinalCall(t, req)
		if text := budgetPrompt(req); !strings.Contains(text, "FALLBACK_REASONING") || !strings.Contains(text, "FALLBACK_PARTIAL") {
			t.Fatalf("lost fallback: %s", text)
		}
		return &llm.ReviewResponse{RawResponse: "FINAL", TokensUsed: model.TokenUsage{TotalTokens: 5}}, nil
	}}
	req := nonReviewerBudgetRequest("merge")
	req.Tools = []llm.ToolDefinition{{Name: "custom"}}
	req.MaxToolCalls = 1
	result, err := pipelineTestEngine(client).runAgentLoop(WithLimiter(nonReviewerBudgetContext(t, false), NewLimiter(1)), req)
	if err != nil || client.count() != 3 || result.tokensUsed.TotalTokens != 10 {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, client.count())
	}
}

func TestAgentBudgetHardDeadlineDoesNotStartCompletion(t *testing.T) {
	client := &budgetRoundClient{respond: func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
		if req.Finalize {
			t.Fatal("completion started at hard deadline")
		}
		<-ctx.Done()
		return nil, &llm.InterruptedResponseError{Err: ctx.Err(), TokensUsed: model.TokenUsage{TotalTokens: 7}}
	}}
	ctx := context.WithValue(reviewerBudgetTestContext(t, 20*time.Millisecond, 100, false), agentBudgetEnabledKey{}, true)
	result, err := pipelineTestEngine(client).runAgentLoop(ctx, nonReviewerBudgetRequest("context"))
	if !errors.Is(err, context.DeadlineExceeded) || client.count() != 1 || result.budgetStop == nil || result.budgetStop.Reason != "deadline" || result.tokensUsed.TotalTokens != 7 {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, client.count())
	}
}

func TestReasoningSummaryOwnSpeedupDoesNotRecurse(t *testing.T) {
	client := &budgetRoundClient{}
	client.respond = func(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
		assertFinalCall(t, req)
		if client.count() == 1 {
			req.ReasoningSink.Append("HELPER_NOTES")
			<-ctx.Done()
			return nil, &llm.InterruptedResponseError{Err: ctx.Err()}
		}
		if client.count() != 2 || !strings.Contains(budgetPrompt(req), "HELPER_NOTES") {
			t.Fatal("helper recursed or lost notes")
		}
		return &llm.ReviewResponse{RawResponse: "SUMMARY"}, nil
	}
	e := pipelineTestEngine(client)
	threshold := 50
	e.budgetSummary = workflow.ReasoningSummaryOverride(&workflow.StepOverride{SummarizeReasoning: &workflow.AgentOverride{TimeBudget: &workflow.TimeBudget{SpeedupThreshold: &threshold}}})
	ctx := context.WithValue(nonReviewerBudgetContext(t, true), agentBudgetOwnedKey{}, true)
	text, _, err := e.summarizeBudgetReasoning(ctx, nonReviewerBudgetRequest("context"), "ORIGINAL_NOTES")
	if err != nil || text != "SUMMARY" || client.count() != 2 {
		t.Fatalf("text=%s err=%v calls=%d", text, err, client.count())
	}
}

func TestContextNoToolsPromptOmitsExplorationInstructions(t *testing.T) {
	e := pipelineTestEngine(nil)
	template, err := e.loadPrompt("agent_context_system_prompt.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := e.renderContextSystemForTools(template, model.ReviewRequest{}, nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(prompt, "### TOOLS") || strings.Contains(prompt, "Use the following tools") || !strings.Contains(prompt, "purpose is an assumption") {
		t.Fatalf("no-tools prompt=%s", prompt)
	}
}
