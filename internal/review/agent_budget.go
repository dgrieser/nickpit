package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
)

// Only review pipelines opt in. In particular, update workflows also use the
// generic loop and must not acquire this behavior incidentally.
type agentBudgetEnabledKey struct{}
type agentBudgetOwnedKey struct{}
type agentBudgetTrackerKey struct{}

var errAgentSpeedup = errors.New("agent budget finalization threshold")

type agentBudgetFinalizePromptData struct {
	AgentKind string
	Notes     string
}

func agentBudgetEnabled(ctx context.Context) bool {
	enabled, _ := ctx.Value(agentBudgetEnabledKey{}).(bool)
	return enabled
}

func (e *Engine) runAgentLoop(ctx context.Context, req agentLoopRequest) (agentLoopResult, error) {
	_, budgeted := timeBudgetFromContext(ctx)
	if !agentBudgetEnabled(ctx) || !budgeted {
		return e.runAgentLoopCore(ctx, req)
	}
	switch req.AgentKind {
	case "context", "verify", "categorize", "dedupe", "merge", "finalize", "verdict", "summarize", "extract":
		return e.runBudgetAgentLoop(ctx, req)
	default:
		return e.runAgentLoopCore(ctx, req)
	}
}

// The soft deadline covers admission, all model calls, and tool execution.
// Final completion uses the still-live parent context and cannot reopen tools.
func (e *Engine) runBudgetAgentLoop(ctx context.Context, req agentLoopRequest) (result agentLoopResult, err error) {
	budget, _ := timeBudgetFromContext(ctx)
	ctx = context.WithValue(ctx, agentBudgetOwnedKey{}, true)
	if req.State == nil {
		req.State = newAgentLoopState()
	}
	if req.State.budgetStop != nil {
		return agentLoopResult{budgetStop: req.State.budgetStop}, fmt.Errorf("agent %s already stopped by time budget", req.AgentName)
	}
	finalizing := false
	defer func() {
		deadline := isTimeBudgetDeadline(ctx)
		if !finalizing && !deadline {
			return
		}
		reason := "finalized"
		if deadline {
			reason = "deadline"
			e.logTimeBudgetDeadlineIfExpired(ctx)
		}
		result.budgetStop = &model.BudgetStop{Reason: reason, Scope: budget.scope, Phase: req.AgentKind}
		req.State.budgetStop = result.budgetStop
		if tracker, _ := ctx.Value(agentBudgetTrackerKey{}).(*agentBudgetTracker); tracker != nil {
			tracker.record(result.budgetStop)
		}
	}()
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if !timeBudgetUrgentNow(ctx) {
		normalCtx := ctx
		cancel := func() {}
		if soft, ok := timeBudgetSpeedupDeadline(ctx); ok {
			normalCtx, cancel = context.WithDeadlineCause(ctx, soft, errAgentSpeedup)
		}
		result, err = e.runAgentLoopCore(normalCtx, req)
		cause := context.Cause(normalCtx)
		cancel()
		if err == nil {
			finalizing = timeBudgetUrgentNow(ctx)
			return result, nil
		}
		if cause != errAgentSpeedup || ctx.Err() != nil {
			return result, err
		}
	}
	finalizing = true
	warnTimeBudgetSpeedup(ctx, budget, true)
	notes := strings.TrimSpace(strings.Join(result.reasoningTraces, "\n\n"))
	if result.interruptedOutput != "" {
		notes = strings.TrimSpace(notes + "\n\nPartial output:\n" + result.interruptedOutput)
	}
	if notes != "" && !e.disableBudgetSummary && (req.AgentKind == "context" || req.AgentKind == "verify") {
		summary, usage, summaryErr := e.summarizeBudgetReasoning(ctx, req, notes)
		result.tokensUsed = addTokenUsage(result.tokensUsed, usage)
		if summaryErr == nil && summary != "" {
			notes = summary
		} else if summaryErr != nil {
			e.logf(ctx, "Reasoning summary failed; finalizing with captured notes: %v", summaryErr)
		}
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	messages := result.messages
	if len(messages) == 0 {
		messages = req.Messages
	}
	messages, err = agentLoopNoToolsMessages(req, messages)
	if err != nil {
		return result, err
	}
	// Also strip protocol for callers that did not provide a prompt renderer.
	if len(messages) > 0 {
		messages, err = noToolsMessagesFromRendered(messages[0].Content, messages)
		if err != nil {
			return result, err
		}
	}
	prompt, err := renderPromptFile("agent_budget_finalize_user_message.tmpl", agentBudgetFinalizePromptData{
		AgentKind: req.AgentKind,
		Notes:     notes,
	})
	if err != nil {
		return result, err
	}
	messages = append(messages, llm.Message{Role: "user", Content: prompt})
	final, finalErr := e.finalizeBudgetAgent(ctx, req, messages)
	result.tokensUsed = addTokenUsage(result.tokensUsed, final.tokensUsed)
	result.contentMessages = append(result.contentMessages, final.contentMessages...)
	result.messages = final.messages
	result.reasoningEffort = final.reasoningEffort
	if final.resp != nil {
		result.resp = final.resp
	}
	return result, finalErr
}

func (e *Engine) summarizeBudgetReasoning(ctx context.Context, parent agentLoopRequest, notes string) (string, model.TokenUsage, error) {
	override := e.budgetSummary
	if override == nil {
		override = workflow.ReasoningSummaryOverride(nil)
	}
	if *override.TimeBudget.Weight == 0 {
		return "", model.TokenUsage{}, nil
	}
	// Allocation is computed at the transition, not at the start of context
	// collection or verification. Unused time remains with the parent.
	plans := childTimePlans(ctx, []*workflow.TimeBudget{override.TimeBudget, nil})
	summaryCtx, cancel, skipped := withConfiguredTimeBudget(ctx, override.TimeBudget, plans[0], budgetScopeForSummary(ctx, parent), e.logf)
	defer cancel()
	if skipped || summaryCtx.Err() != nil {
		return "", model.TokenUsage{}, summaryCtx.Err()
	}
	// A helper's timeout must not mark its owner's successful fallback as a
	// deadline stop in the per-step aggregate.
	summaryCtx = context.WithValue(summaryCtx, agentBudgetTrackerKey{}, (*agentBudgetTracker)(nil))
	profile, helperReq := override.Resolve(e.config, model.ReviewRequest{
		MaxOutputRetries: parent.MaxOutputRetries, MaxReasoningSeconds: parent.MaxReasoningSeconds,
	})
	helper := e.withConfig(profile)
	system, err := renderPromptFile("agent_reasoning_summary_system_prompt.tmpl", nil)
	if err != nil {
		return "", model.TokenUsage{}, err
	}
	user, err := renderPromptFile("agent_reasoning_summary_user_message.tmpl", struct{ Notes string }{notes})
	if err != nil {
		return "", model.TokenUsage{}, err
	}
	name := parent.Progress.AgentName
	if name == "" {
		name = parent.AgentName
	}
	req, sec := helper.buildAgentLoopRequest(agentSpec{name: "Summarize Reasoning of " + name, role: "extract", system: system, user: user, schemaKind: llm.SchemaKindText}, helperReq)
	defer sec.End()
	req.ReasoningEffort = lowReviewerEffort(req.ReasoningEffort)
	req.Finalize = true
	req.ParallelToolCalls = false
	result, err := helper.runAgentLoop(summaryCtx, req)
	return strings.TrimSpace(strings.Join(result.contentMessages, "\n")), result.tokensUsed, err
}

func budgetScopeForSummary(ctx context.Context, req agentLoopRequest) string {
	budget, _ := timeBudgetFromContext(ctx)
	name := req.Progress.AgentName
	if name == "" {
		name = req.AgentName
	}
	return budget.scope + ":summarize_reasoning:" + name
}

// Unlike reviewer finalization, this accepts complete typed responses only.
// Reviewer-specific partial finding recovery would corrupt other schemas.
func (e *Engine) finalizeBudgetAgent(ctx context.Context, req agentLoopRequest, messages []llm.Message) (result agentLoopResult, err error) {
	ctx, release, err := LimiterFromContext(ctx).Acquire(ctx)
	if err != nil {
		return result, err
	}
	defer release()
	if e.logger != nil {
		e.logger.LiveAgentStart(ctx, req.Progress)
		defer e.logger.LiveAgentDone(req.Progress)
	}
	call := reviewerFinalRequest(req, messages)
	defer func() { result.messages = call.Messages }()
	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		resp, callErr := e.loggedReview(logging.WithProgressInfo(ctx, req.Progress), call, req.Section)
		result.tokensUsed = addTokenUsage(result.tokensUsed, reviewCallTokens(resp, callErr))
		var invalid *llm.InvalidResponseError
		if callErr != nil && !errors.As(callErr, &invalid) {
			return result, callErr
		}
		if callErr == nil {
			invalid = validateBudgetResponse(req, resp)
		}
		if invalid == nil {
			result.resp = resp
			result.reasoningEffort = resp.ReasoningEffort
			if result.reasoningEffort == "" {
				result.reasoningEffort = call.ReasoningEffort
			}
			result.contentMessages = appendResponseContent(result.contentMessages, resp)
			call.Messages = append(call.Messages, llm.Message{Role: "assistant", Content: resp.RawResponse})
			return result, nil
		}
		if attempt > 0 || ctx.Err() != nil || !outputRetriesRemaining(req.State.jsonRetries, req.MaxOutputRetries) {
			return result, invalid
		}
		req.State.jsonRetries++
		feedback, renderErr := e.renderJSONRetryFeedback(invalid, req.JSONRetryExampleSnippet)
		if renderErr != nil {
			return result, renderErr
		}
		e.logJSONRetry(ctx, req, req.State.jsonRetries, req.MaxOutputRetries, invalid)
		call.Messages = append(call.Messages, llm.Message{Role: "assistant", Content: invalid.RawContent}, llm.Message{Role: "user", Content: feedback})
	}
}

func validateBudgetResponse(req agentLoopRequest, resp *llm.ReviewResponse) *llm.InvalidResponseError {
	invalid := &llm.InvalidResponseError{Reason: "return a complete response in the original output format"}
	if resp == nil {
		return invalid
	}
	invalid.RawContent = resp.RawResponse
	if len(resp.ToolCalls) > 0 {
		invalid.Reason = "tools are disabled; return the final response directly"
		return invalid
	}
	switch req.SchemaKind {
	case llm.SchemaKindText:
		if strings.TrimSpace(resp.RawResponse) == "" {
			return invalid
		}
	case llm.SchemaKindVerify:
		if resp.Verification == nil {
			return invalid
		}
	case llm.SchemaKindCategorize:
		if resp.Categorization == nil || len(resp.Categorization.Categories) == 0 {
			return invalid
		}
	}
	if req.ValidateResponse != nil {
		return req.ValidateResponse(resp)
	}
	return nil
}

// Verification and classification aggregate several concurrent agents into one
// saved run. A deadline takes precedence over successful urgent completions.
type agentBudgetTracker struct {
	mu   sync.Mutex
	stop *model.BudgetStop
}

func (t *agentBudgetTracker) record(stop *model.BudgetStop) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stop == nil || stop.Reason == "deadline" {
		t.stop = stop
	}
}

func (t *agentBudgetTracker) result(ctx context.Context, phase string) *model.BudgetStop {
	if agentBudgetEnabled(ctx) && isTimeBudgetDeadline(ctx) {
		budget, _ := timeBudgetFromContext(ctx)
		t.record(&model.BudgetStop{Reason: "deadline", Scope: budget.scope, Phase: phase})
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stop
}
