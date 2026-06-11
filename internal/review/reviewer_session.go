package review

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
)

// reviewerSession holds the per-vector reviewer state that survives across the
// initial pass and any number of nudge rounds: the conversation history, the
// pooled agent-loop budget, accumulated findings, captured reasoning-collect
// output, and the carried-forward reasoning effort. It is the abstraction shared
// by the default reviewer flow (runAgent) and the spec-driven standalone
// reasoning-extract / nudge steps, so both route through one implementation.
//
// A session is created and populated by reviewerInitial, then advanced by
// reviewerComputeExtractDelta + reviewerNudgeTurn. The default review step
// drives all of its nudges immediately; a custom spec may instead leave the
// session open (nudge_count: 0) and advance it with standalone steps later.
type reviewerSession struct {
	agent          agentSpec
	extractEnabled bool

	// extractor accumulators (reasoning collect/update run concurrently during
	// the initial pass, so these are guarded).
	mu                  sync.Mutex
	collectWG           sync.WaitGroup
	collectedLists      []string
	extractorTokens     model.TokenUsage
	extractorToolCalls  int
	extractorDuplicates int

	// running result, accumulated across the initial pass and nudge rounds.
	totalFindings   []model.Finding
	totalTokens     model.TokenUsage
	totalToolCalls  int
	totalDuplicates int
	latestResp      *llm.ReviewResponse
	latestReasoning string
	historyMessages []llm.Message
	contentMessages []string
	toolMessages    []llm.Message
	toolCallHistory []toolCallHistoryEntry

	// nudge carry state.
	nudgeState           *agentLoopState
	nudgeReasoningEffort string
	cachedUpdateDelta    string
	cachedUpdateLen      int
	nudgeErr             error
	nudgeTurns           int

	// pending extract delta produced by a standalone reasoning-extract step and
	// consumed by the next standalone nudge step.
	pendingDelta    string
	pendingDeltaSet bool

	// initLoop preserves the initial-pass telemetry so partialResult can report
	// it when the initial loop itself fails.
	initLoop agentLoopResult

	// started anchors the session's total runtime (initial pass through nudges
	// and reasoning extraction). Wall clock: spec-driven sessions advanced by
	// later standalone steps include the time between steps.
	started time.Time
}

// buildAgentLoopRequest assembles the agent-loop request and reasoning tracker
// for an agent. Model parameters come from the engine's (possibly per-step
// overridden) profile; budgets come from the request. The caller owns the
// returned section's lifetime (sec.End()).
func (e *Engine) buildAgentLoopRequest(agent agentSpec, req model.ReviewRequest) (agentLoopRequest, *logging.ReasoningSection) {
	noToolsSystem := agent.noToolsSystem
	if noToolsSystem == "" {
		noToolsSystem = agent.system
	}
	messages := []llm.Message{
		{Role: "system", Content: agent.system},
		{Role: "user", Content: agent.user},
	}
	messages = append(messages, agent.extraMessages...)
	info := e.progressInfo(agent.role, agent.name, "")
	sec := e.logger.NewReasoningTracker(info)

	tools := []llm.ToolDefinition(nil)
	if agent.hasTools {
		tools = reviewerToolDefinitions()
	}
	reviewSnippet := outputSchemaSnippetFor(agent.schemaKind, req.UseJSONSchema)
	if agent.schemaKind == llm.SchemaKindText {
		reviewSnippet = ""
	}
	loopReq := agentLoopRequest{
		AgentName:                  agent.name,
		AgentKind:                  agentLoopKind(agent.role),
		Progress:                   info,
		Messages:                   messages,
		Tools:                      tools,
		Schema:                     agent.schema,
		SchemaKind:                 agent.schemaKind,
		Constraints:                agent.constraints,
		Model:                      e.config.Model,
		MaxTokens:                  e.config.MaxTokens,
		Temperature:                e.config.Temperature,
		TopP:                       e.config.TopP,
		ExtraBody:                  e.config.ExtraBody,
		ParallelToolCalls:          !req.DisableParallelToolCalls,
		ReasoningEffort:            e.config.ReasoningEffort,
		RepoRoot:                   req.RepoRoot,
		MaxToolCalls:               req.MaxToolCalls,
		MaxDuplicateToolCalls:      req.MaxDuplicateToolCalls,
		MaxOutputRetries:           req.MaxOutputRetries,
		MaxReasoningSeconds:        req.MaxReasoningSeconds,
		MaxReasoningLoopRepeats:    req.MaxReasoningLoopRepeats,
		Section:                    sec,
		NoToolsSystem:              noToolsSystem,
		NoToolsSchemaSnippet:       reviewSnippet,
		JSONRetryExampleSnippet:    exampleSnippetFor(agent.schemaKind),
		JSONRetryProgressAgentName: agent.name,
		ValidateResponse:           agent.validateResponse,
		NoToolsMessages: func(messages []llm.Message) ([]llm.Message, error) {
			if !agent.hasTools {
				return append([]llm.Message(nil), messages...), nil
			}
			return noToolsMessagesFromRendered(noToolsSystem, messages)
		},
	}
	return loopReq, sec
}

// newReviewerSession creates an empty session. collectAnyway forces reasoning
// collection during the initial pass even when no nudges are configured — used
// by the spec runner when later standalone nudge/extract steps will consume it.
func (e *Engine) newReviewerSession(agent agentSpec, req model.ReviewRequest, collectAnyway bool) *reviewerSession {
	extractEnabled := agent.role == "review" && !req.DisableReasoningExtract && req.ModelEmitsReasoning && (req.NudgeCount > 0 || collectAnyway)
	return &reviewerSession{
		agent:           agent,
		extractEnabled:  extractEnabled,
		cachedUpdateLen: -1,
		started:         time.Now(),
	}
}

func (s *reviewerSession) addExtractorRun(run model.AgentRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extractorTokens = addTokenUsage(s.extractorTokens, run.TokensUsed)
	s.extractorToolCalls += run.ToolCalls
	s.extractorDuplicates += run.DuplicateToolCalls
}

func (s *reviewerSession) extractorTotals() (model.TokenUsage, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.extractorTokens, s.extractorToolCalls, s.extractorDuplicates
}

func (s *reviewerSession) combinedCollectedList() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.TrimSpace(strings.Join(s.collectedLists, "\n"))
}

func (s *reviewerSession) launchCollect(ctx context.Context, e *Engine, agentName string, iterIdx int, reasoning string, req model.ReviewRequest) {
	if strings.TrimSpace(reasoning) == "" {
		return
	}
	s.collectWG.Go(func() {
		list, result, err := e.runReasoningCollectFindings(ctx, reasoning, agentName, iterIdx, req)
		s.addExtractorRun(result.run)
		if err != nil {
			e.logf(ctx, "Reasoning collect findings failed: agent=%s iter=%d error=%v", agentName, iterIdx, err)
			return
		}
		if strings.TrimSpace(list) == "" {
			return
		}
		s.mu.Lock()
		s.collectedLists = append(s.collectedLists, list)
		s.mu.Unlock()
	})
}

// reviewerInitial runs the reviewer's initial pass, wiring reasoning collection
// when enabled, and populates the session. The reasoning-collect goroutines are
// awaited before returning so collectedLists is frozen for later extraction.
func (e *Engine) reviewerInitial(ctx context.Context, s *reviewerSession, req model.ReviewRequest) error {
	loopReq, sec := e.buildAgentLoopRequest(s.agent, req)
	defer sec.End()
	if s.extractEnabled {
		loopReq.OnReasoningTrace = func(agentName string, iterIdx int, reasoning string) {
			s.launchCollect(ctx, e, agentName, iterIdx, reasoning, req)
		}
	}
	loopResult, err := e.runAgentLoop(ctx, loopReq)
	s.collectWG.Wait()
	s.initLoop = loopResult
	if err != nil {
		return err
	}
	if loopResult.resp == nil {
		return fmt.Errorf("agent %s returned no response", s.agent.name)
	}
	s.totalFindings = append([]model.Finding(nil), loopResult.resp.Findings...)
	s.totalTokens = loopResult.tokensUsed
	s.totalToolCalls = loopResult.toolCalls
	s.totalDuplicates = loopResult.duplicateToolCalls
	s.latestResp = loopResult.resp
	s.latestReasoning = loopResult.reasoningEffort
	s.historyMessages = messagesWithFinalResponse(loopResult.messages, loopResult.resp)
	s.contentMessages = append([]string(nil), loopResult.contentMessages...)
	s.toolMessages = append([]llm.Message(nil), loopResult.toolMessages...)
	s.toolCallHistory = append([]toolCallHistoryEntry(nil), loopResult.toolCallHistory...)
	return nil
}

// reviewerComputeExtractDelta runs the reasoning UpdateFindings extractor over
// the collected reasoning and current findings, returning the formatted delta to
// feed into the next nudge. The delta is cached by findings length so it is only
// recomputed when findings grew. Returns "" (no error) when extraction is
// disabled or produced nothing.
func (e *Engine) reviewerComputeExtractDelta(ctx context.Context, s *reviewerSession, req model.ReviewRequest) (string, error) {
	s.collectWG.Wait()
	reasoningFindings := ""
	if combined := s.combinedCollectedList(); combined != "" {
		if s.cachedUpdateLen == len(s.totalFindings) {
			reasoningFindings = s.cachedUpdateDelta
		} else {
			findingsJSON, err := reasoningFindingsJSON(s.totalFindings)
			if err != nil {
				return "", err
			}
			delta, result, err := e.runReasoningUpdateFindings(ctx, combined, findingsJSON, s.agent.name, req)
			s.addExtractorRun(result.run)
			if err != nil {
				e.logf(ctx, "Reasoning update findings failed, using standard nudge: error=%v", err)
			} else {
				reasoningFindings = delta
				s.cachedUpdateDelta = delta
				s.cachedUpdateLen = len(s.totalFindings)
			}
		}
	}
	formatted := formatReasoningFindingsList(reasoningFindings)
	if s.extractEnabled {
		if formatted != "" {
			e.logBlock(ctx, "Extracted reasoning findings sent to nudge:", formatted)
		} else {
			e.logf(ctx, "No extracted reasoning findings to send to nudge")
		}
	}
	return formatted, nil
}

// reviewerNudgeTurn runs one nudge round against the session, appending any new
// findings. The shared nudge agent-loop state pools the tool/JSON budgets across
// rounds; the reasoning effort baseline is reset once (lazily) then carried
// forward. Returns false (and records nudgeErr) when the round fails, so callers
// stop and keep the prior findings as a partial result.
func (e *Engine) reviewerNudgeTurn(nudgeCtx context.Context, s *reviewerSession, iterIdx, total int, nudgeName, formattedReasoningFindings string, req model.ReviewRequest) bool {
	if s.nudgeState == nil {
		s.nudgeState = newAgentLoopState()
	}
	if s.nudgeReasoningEffort == "" {
		s.nudgeReasoningEffort = e.config.ReasoningEffort
	}
	e.logf(nudgeCtx, "Nudge round: round=%d/%d", iterIdx+1, total)
	nudgeText, err := renderPromptFile("agent_review_nudge_user_message.tmpl", struct {
		HasResponseFormat bool
		QuestionsSnippet  string
		ReasoningFindings string
	}{
		HasResponseFormat: s.agent.schemaKind != llm.SchemaKindText,
		QuestionsSnippet:  strings.TrimSpace(s.agent.questionsSnippet),
		ReasoningFindings: formattedReasoningFindings,
	})
	if err != nil {
		s.nudgeErr = err
		return false
	}
	nudged := append(append([]llm.Message(nil), s.historyMessages...), llm.Message{Role: "user", Content: nudgeText})
	loopReq, sec := e.buildAgentLoopRequest(s.agent, req)
	defer sec.End()
	loopReq.AgentName = nudgeName
	loopReq.Progress.AgentName = nudgeName
	loopReq.JSONRetryProgressAgentName = nudgeName
	loopReq.Messages = nudged
	loopReq.ReasoningEffort = s.nudgeReasoningEffort
	loopReq.State = s.nudgeState
	loopReq.OnReasoningTrace = nil
	sub, err := e.runAgentLoop(nudgeCtx, loopReq)
	if err != nil {
		s.nudgeErr = fmt.Errorf("nudge %d: %w", iterIdx+1, err)
		e.logf(nudgeCtx, "Nudge failed, keeping prior findings: round=%d/%d error=%v", iterIdx+1, total, err)
		return false
	}
	if sub.resp == nil {
		s.nudgeErr = fmt.Errorf("nudge %d: agent %s returned no response", iterIdx+1, s.agent.name)
		e.logf(nudgeCtx, "Nudge returned no response, keeping prior findings: round=%d/%d", iterIdx+1, total)
		return false
	}
	prevFindings := len(s.totalFindings)
	s.totalFindings = appendNewFindings(s.totalFindings, sub.resp.Findings)
	e.logf(nudgeCtx, "Nudge findings: round=%d/%d returned=%d new=%d total=%d", iterIdx+1, total, len(sub.resp.Findings), len(s.totalFindings)-prevFindings, len(s.totalFindings))
	s.totalTokens = addTokenUsage(s.totalTokens, sub.tokensUsed)
	s.totalToolCalls += sub.toolCalls
	s.totalDuplicates += sub.duplicateToolCalls
	s.latestResp = sub.resp
	s.latestReasoning = sub.reasoningEffort
	if sub.reasoningEffort != "" {
		s.nudgeReasoningEffort = sub.reasoningEffort
	}
	s.historyMessages = messagesWithFinalResponse(sub.messages, sub.resp)
	s.contentMessages = append(s.contentMessages, sub.contentMessages...)
	s.toolMessages = append(s.toolMessages, sub.toolMessages...)
	s.toolCallHistory = append(s.toolCallHistory, sub.toolCallHistory...)
	s.nudgeTurns++
	return true
}

// reviewerNudges drives req.NudgeCount rounds of extract+nudge. It returns an
// error only for the rare reasoning-findings marshalling failure; a failed nudge
// round stops the loop and is reported via the session's nudgeErr (a partial,
// not a hard error), matching the legacy behavior.
func (e *Engine) reviewerNudges(ctx context.Context, s *reviewerSession, req model.ReviewRequest) error {
	for i := 0; i < req.NudgeCount; i++ {
		nudgeName := fmt.Sprintf("%s · Nudge %d/%d", s.agent.name, i+1, req.NudgeCount)
		nudgeCtx := logging.WithProgressInfo(ctx, e.progressInfo(s.agent.role, nudgeName, ""))
		delta, err := e.reviewerComputeExtractDelta(nudgeCtx, s, req)
		if err != nil {
			return err
		}
		if !e.reviewerNudgeTurn(nudgeCtx, s, i, req.NudgeCount, nudgeName, delta, req) {
			break
		}
	}
	return nil
}

// result assembles the reviewer's agentResult from the accumulated session
// state, folding in the reasoning-extractor telemetry.
func (s *reviewerSession) result(req model.ReviewRequest) agentResult {
	// Defensive: a session whose initial pass failed never set latestResp.
	// Callers (the review step) avoid this by not exposing failed sessions, but
	// guard against a nil dereference regardless.
	if s.latestResp == nil {
		return s.partialResult(req)
	}
	tokens, toolCalls, duplicates := s.extractorTotals()
	latest := *s.latestResp
	latest.Findings = s.totalFindings
	run := model.AgentRun{
		Name:                  s.agent.name,
		Role:                  s.agent.role,
		Findings:              len(latest.Findings),
		MaxToolCalls:          req.MaxToolCalls,
		MaxDuplicateToolCalls: req.MaxDuplicateToolCalls,
		ToolCalls:             s.totalToolCalls + toolCalls,
		DuplicateToolCalls:    s.totalDuplicates + duplicates,
		TokensUsed:            addTokenUsage(s.totalTokens, tokens),
	}
	if s.nudgeErr != nil {
		run.Status = model.AgentRunStatusPartial
		run.Error = s.nudgeErr.Error()
	}
	run.RuntimeSeconds = model.RuntimeSeconds(time.Since(s.started))
	return agentResult{
		resp:               &latest,
		reasoningEffort:    s.latestReasoning,
		contentMessages:    s.contentMessages,
		toolMessages:       s.toolMessages,
		toolCallHistory:    s.toolCallHistory,
		duplicateToolCalls: s.totalDuplicates + duplicates,
		run:                run,
	}
}

// partialResult builds the partial agentResult used when the initial reviewer
// loop fails before the session was populated.
func (s *reviewerSession) partialResult(req model.ReviewRequest) agentResult {
	result := partialAgentResult(s.agent, req, s.initLoop)
	result.run.RuntimeSeconds = model.RuntimeSeconds(time.Since(s.started))
	return result
}
