package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
)

var errReviewerSpeedup = errors.New("reviewer budget finalization threshold")

// runReviewerRound owns the soft deadline across tools and model calls. The
// original context remains live for mining and finalization until the hard cap.
func (e *Engine) runReviewerRound(ctx context.Context, s *reviewerSession, req agentLoopRequest, reviewReq model.ReviewRequest, phase string, nudge int, candidates string) (result agentLoopResult, err error) {
	budget, enabled := timeBudgetFromContext(ctx)
	if !enabled || reviewReq.DisableWorkflowTimeBudget {
		return e.runAgentLoop(ctx, req)
	}
	ctx = context.WithValue(ctx, reviewerBudgetContextKey{}, true)
	if req.State == nil {
		req.State = newAgentLoopState()
	}
	defer func() {
		if (ctx.Err() != nil || s.budgetStop != nil) && s.collectCancel != nil {
			s.collectCancel()
		}
		if err != nil {
			result.resp = e.validBudgetPartial(ctx, req, result.resp)
		}
		if isTimeBudgetDeadline(ctx) {
			s.budgetError = errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
			s.budgetStop = &model.BudgetStop{Reason: "deadline", Scope: budget.scope, Phase: phase, NudgeIndex: nudge}
			now := time.Now()
			findings := s.totalFindings
			if result.resp != nil {
				findings = appendNewFindings(append([]model.Finding(nil), findings...), result.resp.Findings)
			}
			warningsFromContext(ctx).once("time-budget-deadline:"+budget.scope,
				"Time budget deadline reached for %s: elapsed=%s limit=%s overrun=%s; %s stopped during %s; findings retained=%d",
				budget.scope, model.HumanWait(timeBudgetElapsed(budget, now)), model.HumanWait(timeBudgetLimit(budget)), model.HumanWait(timeBudgetOverrun(budget, now)), s.agent.name, reviewerBudgetPhase(phase, nudge), len(findings))
		} else if s.budgetStop != nil && err == nil {
			warningsFromContext(ctx).once("reviewer-budget-finalized:"+s.agent.name,
				"Time budget finalized %s during %s; no further nudge rounds", s.agent.name, reviewerBudgetPhase(phase, nudge))
		}
	}()
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if !timeBudgetUrgentNow(ctx) {
		roundCtx := ctx
		cancel := func() {}
		if soft, ok := timeBudgetSpeedupDeadline(ctx); ok {
			roundCtx, cancel = context.WithDeadlineCause(ctx, soft, errReviewerSpeedup)
		}
		result, err = e.runAgentLoop(roundCtx, req)
		cause := context.Cause(roundCtx)
		cancel()
		s.reasoningTraces = append(s.reasoningTraces, result.reasoningTraces...)
		if err == nil {
			if timeBudgetUrgentNow(ctx) {
				s.budgetStop = &model.BudgetStop{Reason: "finalized", Scope: budget.scope, Phase: phase, NudgeIndex: nudge}
			}
			return result, nil
		}
		if cause != errReviewerSpeedup || ctx.Err() != nil {
			return result, err
		}
	}
	s.budgetStop = &model.BudgetStop{Reason: "finalized", Scope: budget.scope, Phase: phase, NudgeIndex: nudge}
	warnTimeBudgetSpeedup(ctx, budget, true)
	if s.collectCancel != nil {
		s.collectCancel()
	}
	// Collectors honor cancellation. Do not wait for their normal phase budget
	// here: take a locked snapshot and mine anything not represented in it.
	s.mu.Lock()
	lists := append([]string(nil), s.collectedLists...)
	unmined := make([]string, 0, len(s.reasoningTraces)+1)
	for _, trace := range s.reasoningTraces {
		if !s.minedTraces[trace] {
			unmined = append(unmined, trace)
		}
	}
	s.mu.Unlock()
	lists = append(lists, candidates)
	if result.interruptedOutput != "" {
		unmined = append(unmined, "Partial answer:\n"+result.interruptedOutput)
	}
	notes := strings.TrimSpace(strings.Join(unmined, "\n\n"))
	if notes != "" && !reviewReq.DisableReasoningExtract {
		limit := min(30*time.Second, time.Until(budget.deadline)/4)
		if cfg := s.mineBudget.cfg; cfg != nil && cfg.MaxSeconds != nil {
			limit = min(limit, time.Duration(*cfg.MaxSeconds)*time.Second)
		}
		if limit > 0 {
			mineCtx, cancel := context.WithTimeout(ctx, limit)
			miner := s.mineEngine
			if miner == nil {
				miner = e
			}
			list, usage, mineErr := miner.mineBudgetNotes(mineCtx, notes, req.Progress, s.mineReq)
			cancel()
			s.addExtractorRun(model.AgentRun{TokensUsed: usage})
			if mineErr == nil {
				lists = append(lists, list)
				notes = ""
			} else {
				e.logf(ctx, "Budget reasoning mining failed; using captured notes: %v", mineErr)
			}
		}
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	existingJSON, marshalErr := reasoningFindingsJSON(s.totalFindings)
	if marshalErr != nil {
		return result, marshalErr
	}
	prompt, renderErr := renderPromptFile("agent_review_budget_finalize_user_message.tmpl", struct {
		Phase, Candidates, Notes, ExistingFindings string
	}{reviewerBudgetPhase(phase, nudge), strings.Join(lists, "\n"), notes, existingJSON})
	if renderErr != nil {
		return result, renderErr
	}
	messages := result.messages
	if len(messages) == 0 {
		messages = req.Messages
	}
	messages, err = agentLoopNoToolsMessages(req, messages)
	if err != nil {
		return result, err
	}
	messages = append(messages, llm.Message{Role: "user", Content: prompt})
	final, finalErr := e.finalizeReviewerRound(ctx, req, messages)
	result.tokensUsed = addTokenUsage(result.tokensUsed, final.tokensUsed)
	result.contentMessages = append(result.contentMessages, final.contentMessages...)
	result.messages = final.messages
	result.reasoningEffort = final.reasoningEffort
	// A locally valid interrupted answer can survive a failed finalization.
	previous := e.validBudgetPartial(ctx, req, result.resp)
	result.resp = final.resp
	if finalErr != nil && previous != nil {
		if result.resp == nil {
			result.resp = previous
		} else {
			result.resp.Findings = appendNewFindings(previous.Findings, result.resp.Findings)
		}
	}
	return result, finalErr
}

func reviewerBudgetPhase(phase string, nudge int) string {
	if nudge > 0 {
		return fmt.Sprintf("nudge %d", nudge)
	}
	return phase
}

func (s *reviewerSession) stopBeforeNudge(ctx context.Context) bool {
	if s.budgetStop != nil {
		return true
	}
	if !timeBudgetUrgentNow(ctx) && !isTimeBudgetDeadline(ctx) {
		return false
	}
	budget, _ := timeBudgetFromContext(ctx)
	reason := "finalized"
	if isTimeBudgetDeadline(ctx) {
		reason = "deadline"
	}
	s.budgetStop = &model.BudgetStop{Reason: reason, Scope: budget.scope, Phase: "between rounds", NudgeIndex: s.nudgeTurns}
	warningsFromContext(ctx).once("reviewer-budget-finalized:"+s.agent.name,
		"Time budget stopped %s between rounds; completed nudges=%d; findings retained=%d", s.agent.name, s.nudgeTurns, len(s.totalFindings))
	return true
}

func lowReviewerEffort(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "off", "none", "minimal":
		return effort
	default:
		return "low"
	}
}

func (e *Engine) mineBudgetNotes(ctx context.Context, notes string, info logging.ProgressInfo, reviewReq model.ReviewRequest) (string, model.TokenUsage, error) {
	ctx, release, err := LimiterFromContext(ctx).Acquire(ctx)
	if err != nil {
		return "", model.TokenUsage{}, err
	}
	defer release()
	system, err := renderPromptFile("agent_reasoning_collect_findings_system_prompt.tmpl", nil)
	if err != nil {
		return "", model.TokenUsage{}, err
	}
	user, err := renderPromptFile("agent_reasoning_collect_findings_user_message.tmpl", struct{ ReasoningContent string }{notes})
	if err != nil {
		return "", model.TokenUsage{}, err
	}
	req, sec := e.buildAgentLoopRequest(agentSpec{name: "Mine Reasoning of " + info.AgentName, role: "extract", system: system, user: user, schemaKind: llm.SchemaKindText}, reviewReq)
	defer sec.End()
	call := reviewerFinalRequest(req, req.Messages)
	resp, err := e.loggedReview(logging.WithProgressInfo(ctx, req.Progress), call, sec)
	usage := reviewCallTokens(resp, err)
	if err != nil {
		return "", usage, err
	}
	if resp == nil || len(resp.ToolCalls) > 0 {
		return "", usage, fmt.Errorf("reasoning miner returned no text answer")
	}
	return reasoningExtractOutput([]string{resp.RawResponse}), usage, nil
}

func reviewerFinalRequest(req agentLoopRequest, messages []llm.Message) *llm.ReviewRequest {
	return &llm.ReviewRequest{
		Messages: messages, Schema: req.Schema, SchemaKind: req.SchemaKind, Constraints: req.Constraints,
		Model: req.Model, MaxTokens: req.MaxTokens, Temperature: req.Temperature, TopP: req.TopP,
		TopK: req.TopK, MinP: req.MinP, PresencePenalty: req.PresencePenalty,
		RepetitionPenalty: req.RepetitionPenalty, ExtraBody: req.ExtraBody,
		ReasoningEffort: lowReviewerEffort(req.ReasoningEffort), Finalize: true,
		DisableReasoningEffortFallback: true, MaxReasoning: time.Duration(req.MaxReasoningSeconds) * time.Second,
		CallerRetriesError: func(err error) bool { var invalid *llm.InvalidResponseError; return errors.As(err, &invalid) },
	}
}

func (e *Engine) finalizeReviewerRound(ctx context.Context, req agentLoopRequest, messages []llm.Message) (result agentLoopResult, err error) {
	ctx, release, err := LimiterFromContext(ctx).Acquire(ctx)
	if err != nil {
		return result, err
	}
	defer release()
	call := reviewerFinalRequest(req, messages)
	defer func() { result.messages = call.Messages }()
	for attempt := 0; ; attempt++ {
		resp, callErr := e.loggedReview(logging.WithProgressInfo(ctx, req.Progress), call, req.Section)
		result.tokensUsed = addTokenUsage(result.tokensUsed, reviewCallTokens(resp, callErr))
		var invalid *llm.InvalidResponseError
		var interrupted *llm.InterruptedResponseError
		if errors.As(callErr, &interrupted) {
			resp = interrupted.PartialResponse
		} else if errors.As(callErr, &invalid) {
			resp = invalid.PartialResponse
		}
		if resp != nil {
			if len(resp.ToolCalls) > 0 {
				invalid = &llm.InvalidResponseError{Reason: "tools are disabled; return findings directly", RawContent: resp.RawResponse}
				resp = nil
			} else if repaired := e.repairResponseOrRetry(ctx, req, resp); repaired != nil {
				invalid = repaired
			} else if req.ValidateResponse != nil {
				if validation := req.ValidateResponse(resp); validation != nil {
					invalid = validation
				}
			}
			if recovered := e.validBudgetPartial(ctx, req, resp); recovered != nil {
				if result.resp != nil {
					recovered.Findings = appendNewFindings(result.resp.Findings, recovered.Findings)
				}
				result.resp = recovered
			}
		}
		if callErr == nil && invalid == nil && resp != nil {
			result.resp = resp
			result.reasoningEffort = resp.ReasoningEffort
			if result.reasoningEffort == "" {
				result.reasoningEffort = call.ReasoningEffort
			}
			result.contentMessages = appendResponseContent(result.contentMessages, resp)
			call.Messages = append(call.Messages, llm.Message{Role: "assistant", Content: resp.RawResponse})
			return result, nil
		}
		if invalid == nil {
			if callErr == nil {
				callErr = fmt.Errorf("urgent finalization returned no response")
			}
			return result, callErr
		}
		if attempt > 0 || ctx.Err() != nil || !outputRetriesRemaining(req.State.jsonRetries, req.MaxOutputRetries) {
			if callErr != nil {
				return result, callErr
			}
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

// Keep only findings that independently pass local validation. Recovery never
// treats an interrupted tool request or an unvalidated fragment as a finding.
func (e *Engine) validBudgetPartial(ctx context.Context, req agentLoopRequest, resp *llm.ReviewResponse) *llm.ReviewResponse {
	if resp == nil || len(resp.ToolCalls) > 0 {
		return nil
	}
	copyResp := *resp
	copyResp.Findings = nil
	for _, finding := range resp.Findings {
		if !llm.ValidRecoveredReviewFinding(finding, req.Constraints) {
			continue
		}
		candidate := *resp
		candidate.Findings = []model.Finding{finding}
		if e.repairResponseOrRetry(ctx, req, &candidate) != nil {
			continue
		}
		if req.ValidateResponse != nil && req.ValidateResponse(&candidate) != nil {
			continue
		}
		copyResp.Findings = append(copyResp.Findings, candidate.Findings...)
	}
	if len(copyResp.Findings) == 0 {
		return nil
	}
	model.EnsureFindingIDs(copyResp.Findings)
	return &copyResp
}
