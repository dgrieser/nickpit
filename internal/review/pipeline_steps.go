package review

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/dgrieser/nickpit/internal/dedupe"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
	"github.com/google/uuid"
)

// ensurePrompts builds the deterministic review prompt scaffolding (base
// template, style guides, toolchain flag, and the rendered context JSON) from
// the current Enriched context. It is idempotent; the collect step calls it
// after enrichment so reviewers see supplemental context, and standalone review
// specs (no collect) call it to build from the base context.
func (e *Engine) ensurePrompts(st *PipelineState) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.promptsReady {
		return nil
	}
	baseTemplate, err := e.loadPrompt("agent_review_general_system_prompt.tmpl")
	if err != nil {
		return err
	}
	payload := model.PromptPayloadFromContext(st.Enriched)
	payload.StyleGuides, err = e.styleGuidesFor(st.Enriched)
	if err != nil {
		return err
	}
	userPrompt, err := llm.RenderJSON(payload)
	if err != nil {
		return fmt.Errorf("review: rendering review prompt json: %w", err)
	}
	st.baseTemplate = baseTemplate
	st.enrichedPrompt = userPrompt
	st.styleGuides = payload.StyleGuides
	st.hasToolchain = len(payload.ToolchainVersions) > 0
	st.promptsReady = true
	return nil
}

// collectStepFunc runs the context agent and enriches the review context with
// the supplemental files it gathers. Context failure is soft: a warning is
// recorded and the pipeline continues with the un-enriched context.
func (e *Engine) collectStepFunc() stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		basePayload := model.PromptPayloadFromContext(st.Base)
		guides, err := sc.Engine.styleGuidesFor(st.Base)
		if err != nil {
			return err
		}
		basePayload.StyleGuides = guides
		baseUserPrompt, err := llm.RenderJSON(basePayload)
		if err != nil {
			return fmt.Errorf("review: rendering review prompt json: %w", err)
		}
		contextTemplate, err := sc.Engine.loadPrompt("agent_context_system_prompt.tmpl")
		if err != nil {
			return err
		}
		contextSystem, err := sc.Engine.renderContextSystem(contextTemplate, sc.Req)
		if err != nil {
			return err
		}
		contextResult, contextErr := sc.Engine.runContextAgent(ctx, agentSpec{
			name:          "Collect Context",
			role:          "context",
			system:        contextSystem,
			noToolsSystem: contextSystem,
			user:          baseUserPrompt,
			schemaKind:    llm.SchemaKindText,
			hasTools:      true,
		}, sc.Req)
		if contextErr != nil {
			sc.Engine.logf(ctx, "Context agent failed, continuing with degraded context: error=%v", contextErr)
			contextResult.run.Status = model.AgentRunStatusFailed
			contextResult.run.Error = contextErr.Error()
		}

		enriched, err := model.CloneContext(st.Base)
		if err != nil {
			return fmt.Errorf("review: cloning context: %w", err)
		}
		enriched.SupplementalContext = append(enriched.SupplementalContext, supplementalFromContextAgent(contextResult.toolMessages)...)

		run := contextResult.run
		st.mu.Lock()
		st.Enriched = enriched
		st.contextMessages = contextAgentMarkdownMessages(contextResult.contentMessages)
		st.contextNotes = contextAgentMarkdownContent(contextResult.contentMessages)
		st.contextRun = &run
		st.contextReasoning = contextResult.reasoningEffort
		st.contextErr = contextErr
		if contextErr != nil {
			st.warnings = append(st.warnings, fmt.Sprintf("Context agent failed: %v; continuing with degraded context", contextErr))
		}
		st.promptsReady = false
		st.mu.Unlock()
		return e.ensurePrompts(st)
	}
}

// buildReviewerAgentSpec renders the per-vector reviewer agent spec from the
// prepared prompt scaffolding in the state.
func (e *Engine) buildReviewerAgentSpec(vector reviewVector, st *PipelineState, req model.ReviewRequest) (agentSpec, error) {
	questionsSnippet, err := e.renderReviewerQuestionsSnippet(vector.questionsFile)
	if err != nil {
		return agentSpec{}, err
	}
	system, err := e.renderReviewSystemWithQuestions(st.baseTemplate, vector.focusFile, questionsSnippet, req, true, "review", st.styleGuides, st.hasToolchain)
	if err != nil {
		return agentSpec{}, err
	}
	noToolsSystem, err := e.renderReviewSystemWithQuestions(st.baseTemplate, vector.focusFile, questionsSnippet, req, false, "review", st.styleGuides, st.hasToolchain)
	if err != nil {
		return agentSpec{}, err
	}
	var schema []byte
	if req.UseJSONSchema {
		schema = llm.FindingsSchema
		if req.SkipSuggestions {
			schema = llm.FindingsSchemaWithoutSuggestions
		}
		if vector.constraints.MinPriority != nil || vector.constraints.MaxPriority != nil || len(vector.constraints.AllowedCorrectness) > 0 {
			schema = llm.FindingsSchemaWithConstraintsFor(vector.constraints, req.SkipSuggestions)
		}
	}
	return agentSpec{
		name:             vector.name,
		role:             "review",
		system:           system,
		noToolsSystem:    noToolsSystem,
		user:             st.enrichedPrompt,
		extraMessages:    st.contextMessages,
		questionsSnippet: questionsSnippet,
		schema:           schema,
		schemaKind:       llm.SchemaKindReview,
		constraints:      vector.constraints,
		hasTools:         true,
	}, nil
}

func reviewPhaseContexts(ctx context.Context, override *workflow.StepOverride, req model.ReviewRequest, collectAnyway bool) (context.Context, context.CancelFunc, context.Context, context.CancelFunc, context.Context, context.CancelFunc, context.Context, context.CancelFunc, bool) {
	noop := func() {}
	if req.SkipWorkflowTimeBudget || override == nil || !hasReviewPhaseBudget(override) {
		return ctx, noop, ctx, noop, ctx, noop, ctx, noop, false
	}
	extractEnabled := !req.DisableReasoningExtract && req.ModelEmitsReasoning && (req.NudgeCount > 0 || collectAnyway)

	type phase struct {
		name string
		tb   *workflow.TimeBudget
	}
	phases := []phase{{name: "main", tb: mainReviewTimeBudget(override.TimeBudget)}}
	mineTB := agentTimeBudget(override.MineReasoning)
	compileTB := agentTimeBudget(override.CompileFindings)
	nudgeTB := agentTimeBudget(override.Nudge)
	if extractEnabled || mineTB != nil {
		phases = append(phases, phase{name: "mine", tb: mineTB})
	}
	if (extractEnabled && req.NudgeCount > 0) || compileTB != nil {
		phases = append(phases, phase{name: "compile", tb: compileTB})
	}
	if req.NudgeCount > 0 || nudgeTB != nil {
		phases = append(phases, phase{name: "nudge", tb: nudgeTB})
	}
	budgets := make([]*workflow.TimeBudget, len(phases))
	for i := range phases {
		budgets[i] = phases[i].tb
	}
	plans := childTimePlans(ctx, budgets)

	mainCtx, mineCtx, compileCtx, nudgeCtx := ctx, ctx, ctx, ctx
	mainCancel, mineCancel, compileCancel, nudgeCancel := noop, noop, noop, noop
	mainSkipped := false
	for i, phase := range phases {
		phaseCtx, cancel, skipped := withConfiguredTimeBudget(ctx, phase.tb, plans[i])
		if skipped {
			phaseCtx, cancel = alreadyCanceledContext(ctx)
		}
		switch phase.name {
		case "main":
			mainCtx, mainCancel, mainSkipped = phaseCtx, cancel, skipped
		case "mine":
			mineCtx, mineCancel = phaseCtx, cancel
		case "compile":
			compileCtx, compileCancel = phaseCtx, cancel
		case "nudge":
			nudgeCtx, nudgeCancel = phaseCtx, cancel
		}
	}
	return mainCtx, mainCancel, mineCtx, mineCancel, compileCtx, compileCancel, nudgeCtx, nudgeCancel, mainSkipped
}

func hasReviewPhaseBudget(override *workflow.StepOverride) bool {
	if override == nil {
		return false
	}
	if tb := mainReviewTimeBudget(override.TimeBudget); tb != nil {
		return true
	}
	return agentTimeBudget(override.MineReasoning) != nil || agentTimeBudget(override.CompileFindings) != nil || agentTimeBudget(override.Nudge) != nil
}

func mainReviewTimeBudget(tb *workflow.TimeBudget) *workflow.TimeBudget {
	if tb == nil || (tb.Weight == nil && tb.SpeedupThreshold == nil) {
		return nil
	}
	return &workflow.TimeBudget{SpeedupThreshold: tb.SpeedupThreshold, Weight: tb.Weight}
}

func agentTimeBudget(override *workflow.AgentOverride) *workflow.TimeBudget {
	if override == nil {
		return nil
	}
	return override.TimeBudget
}

func alreadyCanceledContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	cancel()
	return ctx, func() {}
}

// reviewStepFunc runs one vector reviewer: its initial pass plus any configured
// nudge rounds, leaving the session open in state for standalone nudge/extract
// steps. A reviewer failure is soft (recorded as a failed group), matching the
// legacy per-vector tolerance.
func (e *Engine) reviewStepFunc(vectorID string, collectAnyway bool) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		if err := sc.Engine.ensurePrompts(st); err != nil {
			return err
		}
		vector, ok := reviewVectorByID(vectorID)
		if !ok {
			return fmt.Errorf("workflow: unknown reviewer vector %q", vectorID)
		}
		spec, err := sc.Engine.buildReviewerAgentSpec(vector, st, sc.Req)
		if err != nil {
			return err
		}
		session := sc.Engine.newReviewerSession(spec, sc.Req, collectAnyway)
		mine := internalAgentContext{Engine: sc.Engine, Req: sc.Req}
		compile := internalAgentContext{Engine: sc.Engine, Req: sc.Req}
		nudge := internalAgentContext{Engine: sc.Engine, Req: sc.Req}
		if sc.Override != nil {
			if sc.Override.MineReasoning != nil {
				mine = sc.internalAgentContext(sc.Override.MineReasoning)
			}
			if sc.Override.CompileFindings != nil {
				compile = sc.internalAgentContext(sc.Override.CompileFindings)
			}
			if sc.Override.Nudge != nil {
				nudge = sc.internalAgentContext(sc.Override.Nudge)
			}
		}
		mainCtx, mainCancel, mineCtx, mineCancel, compileCtx, compileCancel, nudgeCtx, nudgeCancel, mainSkipped := reviewPhaseContexts(ctx, sc.Override, sc.Req, collectAnyway)
		defer mainCancel()
		defer mineCancel()
		defer compileCancel()
		defer nudgeCancel()
		if mainSkipped {
			res := session.partialResult(sc.Req)
			res.run.Status = model.AgentRunStatusSkipped
			res.run.Error = "main review skipped because its time budget was exhausted"
			st.setGroup(vectorID, res, nil)
			return nil
		}
		if err := sc.Engine.reviewerInitial(mainCtx, session, sc.Req, mineCtx, mine.Engine, mine.Req); err != nil {
			// Preserve the partial telemetry (tokens/tool calls) of the failed
			// initial pass instead of discarding it with a bare failed result.
			// Store NO session: the initial pass never populated it, so a later
			// nudge/reasoning-extract step must reject the failed group rather
			// than dereference a nil response.
			sc.Engine.logf(ctx, "Vector reviewer failed, continuing with others: vector=%s error=%v", vector.name, err)
			res := session.partialResult(sc.Req)
			res.run.Status = model.AgentRunStatusFailed
			res.run.Error = err.Error()
			st.setGroup(vectorID, res, nil)
			return nil
		}
		if err := sc.Engine.reviewerNudges(ctx, session, sc.Req, compileCtx, compile.Engine, compile.Req, nudgeCtx, nudge.Engine, nudge.Req); err != nil {
			// The initial pass already produced findings; keep them (and their
			// telemetry) as a partial result rather than throwing them away.
			sc.Engine.logf(ctx, "Vector reviewer nudges failed, keeping initial findings: vector=%s error=%v", vector.name, err)
			res := session.result(sc.Req)
			res.run.Status = model.AgentRunStatusPartial
			res.run.Error = err.Error()
			st.setGroup(vectorID, res, session)
			return nil
		}
		st.setGroup(vectorID, session.result(sc.Req), session)
		return nil
	}
}

// sessionForStep returns the open reviewer session for a vector, distinguishing
// "no preceding review" from "the review ran but failed" (which leaves no
// session) so standalone nudge/extract steps fail cleanly instead of operating
// on an unpopulated session.
func sessionForStep(st *PipelineState, stepType, vectorID string) (*reviewerSession, error) {
	g := st.group(vectorID)
	if g == nil {
		return nil, fmt.Errorf("workflow: %s%s requires a preceding review:%s step", stepType, vectorID, vectorID)
	}
	if g.session == nil {
		return nil, fmt.Errorf("workflow: %s%s cannot run because review:%s did not complete successfully", stepType, vectorID, vectorID)
	}
	return g.session, nil
}

// extractStepFunc runs the standalone reasoning-extract step for a vector,
// storing the computed delta on the session for the next nudge step.
func (e *Engine) extractStepFunc(vectorID string) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		sess, err := sessionForStep(st, "reasoning-extract:", vectorID)
		if err != nil {
			return err
		}
		delta, err := sc.Engine.reviewerComputeExtractDelta(ctx, sess, sc.Req)
		if err != nil {
			return err
		}
		sess.pendingDelta = delta
		sess.pendingDeltaSet = true
		return nil
	}
}

// nudgeStepFunc runs a single standalone nudge round for a vector, consuming any
// delta a preceding reasoning-extract step produced.
func (e *Engine) nudgeStepFunc(vectorID string) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		sess, err := sessionForStep(st, "nudge:", vectorID)
		if err != nil {
			return err
		}
		delta := ""
		if sess.pendingDeltaSet {
			delta = sess.pendingDelta
		} else {
			d, err := sc.Engine.reviewerComputeExtractDelta(ctx, sess, sc.Req)
			if err != nil {
				return err
			}
			delta = d
		}
		iter := sess.nudgeTurns
		nudgeName := fmt.Sprintf("%s · Nudge %d", sess.agent.name, iter+1)
		nudgeCtx := logging.WithProgressInfo(ctx, sc.Engine.progressInfo(sess.agent.role, nudgeName, ""))
		sc.Engine.reviewerNudgeTurn(nudgeCtx, sess, iter, iter+1, nudgeName, delta, sc.Req)
		sess.pendingDelta = ""
		sess.pendingDeltaSet = false
		st.setGroup(vectorID, sess.result(sc.Req), sess)
		return nil
	}
}

// verifyStepFunc verifies and filters the current groups' findings in place.
// When findings are injected, they seed a single group first. A verifier failure
// is fatal, mirroring the legacy pipeline.
func (e *Engine) verifyStepFunc(findingsFrom []string) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		if err := injectGroups(st, findingsFrom, sc.Req.SkipSuggestions); err != nil {
			return err
		}
		vr := st.vectorResults()
		usage, warnings, err := sc.Engine.verifyAndFilterVectorFindings(ctx, st.Enriched, vr, sc.Req, st.limiter, "")
		st.writeBackVectorResults(vr)
		st.mu.Lock()
		st.verifyUsage = addTokenUsage(st.verifyUsage, usage)
		st.warnings = append(st.warnings, warnings...)
		st.mu.Unlock()
		if err != nil {
			sc.Engine.logf(ctx, "Verifier failed before merge: tokens=%s warnings=%d error=%v", model.HumanTokens(usage.TotalTokens), len(warnings), err)
			return err
		}
		return nil
	}
}

// verifyVectorStepFunc verifies and filters one reviewer group's findings in
// place, admitted through the run-shared verify limiter. Verifier failure is
// fatal, matching the global verify step; a soft-failed or empty reviewer is a
// graceful no-op.
func (e *Engine) verifyVectorStepFunc(vectorID string) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		vr, ok := st.vectorResult(vectorID)
		if !ok {
			return fmt.Errorf("workflow: verify:%s requires a preceding review:%s step", vectorID, vectorID)
		}
		if vr.run.Status == model.AgentRunStatusFailed || vr.resp == nil || len(vr.resp.Findings) == 0 {
			return nil
		}
		vector, ok := reviewVectorByID(vectorID)
		if !ok {
			return fmt.Errorf("workflow: unknown reviewer vector %q", vectorID)
		}
		results := []agentResult{vr}
		usage, warnings, err := sc.Engine.verifyAndFilterVectorFindings(ctx, st.Enriched, results, sc.Req, st.limiter, vector.name)
		st.mu.Lock()
		st.verifyUsage = addTokenUsage(st.verifyUsage, usage)
		st.warnings = append(st.warnings, warnings...)
		st.mu.Unlock()
		if err != nil {
			sc.Engine.logf(ctx, "Verifier failed for reviewer: reviewer=%s tokens=%s warnings=%d error=%v", vector.name, model.HumanTokens(usage.TotalTokens), len(warnings), err)
			return err
		}
		return nil
	}
}

// dedupeVectorStepFunc runs the dedupe pass for one reviewer group. Failure is
// soft (the original findings are kept), matching the global dedupe step.
func (e *Engine) dedupeVectorStepFunc(vectorID string) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		vr, ok := st.vectorResult(vectorID)
		if !ok {
			return fmt.Errorf("workflow: dedupe:%s requires a preceding review:%s step", vectorID, vectorID)
		}
		if vr.run.Status == model.AgentRunStatusFailed || vr.resp == nil || len(vr.resp.Findings) < 2 {
			return nil
		}
		resp, run := sc.Engine.runDedupeAgent(ctx, st.contextNotes, vr, mergeSchemaForDedupe(sc.Req), mergeConstraintsForDedupe(sc.Req), sc.Req)
		if resp != nil {
			st.setVectorResponse(vectorID, resp)
		}
		st.mu.Lock()
		st.dedupeRuns = append(st.dedupeRuns, run)
		st.mu.Unlock()
		return nil
	}
}

// dedupeStepFunc runs the per-group dedupe pass.
func (e *Engine) dedupeStepFunc(findingsFrom []string) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		if err := injectGroups(st, findingsFrom, sc.Req.SkipSuggestions); err != nil {
			return err
		}
		vr := st.vectorResults()
		runs := sc.Engine.runDedupeAgents(ctx, st.contextNotes, vr, mergeSchemaForDedupe(sc.Req), mergeConstraintsForDedupe(sc.Req), sc.Req)
		st.writeBackVectorResults(vr)
		st.mu.Lock()
		st.dedupeRuns = append(st.dedupeRuns, runs...)
		st.mu.Unlock()
		return nil
	}
}

// mergeStepFunc performs the cluster merge across groups, the single
// grouped→flat transform. It sets the flat result (priority-filtered, IDs
// normalized) consumed by finalize and output.
func (e *Engine) mergeStepFunc(findingsFrom []string) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		if err := injectGroups(st, findingsFrom, sc.Req.SkipSuggestions); err != nil {
			return err
		}
		req := sc.Req
		vr := st.vectorResults()
		mergeInputs := pairwiseMergeInputs(vr)
		verifiedMergeInputs := flattenPairwiseMergeInputs(mergeInputs)

		mergeConstraints := llm.ResponseConstraints{}
		var mergeSchema []byte
		if req.UseJSONSchema {
			mergeConstraints = mergeConstraintsForRequest(req)
			if hasResponseConstraints(mergeConstraints) {
				mergeSchema = llm.MergeSchemaWithConstraintsFor(mergeConstraints, req.SkipSuggestions)
			} else if req.SkipSuggestions {
				mergeSchema = llm.MergeSchemaWithoutSuggestions
			} else {
				mergeSchema = llm.MergeSchema
			}
		}

		var (
			mergeResult agentResult
			mergeRuns   []model.AgentRun
			warnings    []string
		)
		switch {
		case allVectorsFailed(vr):
			sc.Engine.logf(ctx, "All vector reviewers failed; skipping merge agent and emitting empty result")
			warnings = append(warnings, "All vector reviewers failed; skipped merge agent and returning empty findings")
			mergeResult = emptyVerifiedMergeResult()
		case len(mergeInputs) == 0:
			sc.Engine.logf(ctx, "No verified findings remained; skipping merge agent and emitting empty result")
			warnings = append(warnings, "No verified findings remained; skipped merge agent and returning empty findings")
			mergeResult = emptyVerifiedMergeResult()
		default:
			// review_context is embedded as raw JSON in the merge prompt; a
			// source-less workflow (e.g. --step merge --findings a.json b.json)
			// has no enriched prompt yet, so fall back to an empty JSON object so
			// the merge agent actually runs instead of failing JSON rendering and
			// degrading to a plain concatenation.
			userPrompt := st.enrichedPrompt
			if strings.TrimSpace(userPrompt) == "" {
				userPrompt = "{}"
			}
			mergeResult, mergeRuns = sc.Engine.runClusterMergeAgents(ctx, userPrompt, st.contextNotes, mergeInputs, mergeSchema, mergeConstraints, req)
		}
		if mergeResult.resp != nil {
			mergeInputVerification(mergeResult.resp.Findings, verifiedMergeInputs)
		}
		if len(mergeRuns) == 0 {
			mergeRuns = []model.AgentRun{mergeResult.run}
		}

		filtered := filterByPriority(mergeResult.resp.Findings, req.PriorityThreshold)
		if overwrote := model.EnsureFindingIDs(filtered); overwrote > 0 {
			sc.Engine.logf(ctx, "Review generated replacement IDs for invalid finding IDs: count=%d", overwrote)
		}
		st.mu.Lock()
		st.mergeRuns = append(st.mergeRuns, mergeRuns...)
		st.mergeReasoning = mergeResult.reasoningEffort
		st.warnings = append(st.warnings, warnings...)
		st.result = &model.ReviewResult{
			Findings:               filtered,
			OverallCorrectness:     mergeResult.resp.OverallCorrectness,
			OverallExplanation:     mergeResult.resp.OverallExplanation,
			OverallConfidenceScore: mergeResult.resp.OverallConfidenceScore,
		}
		st.mu.Unlock()
		return nil
	}
}

type clusterMergeOutcome struct {
	index    int
	findings []model.Finding
	run      model.AgentRun
	hasRun   bool
}

func pipelinePhaseContexts(ctx context.Context, fused postMergeFusedSpec, skip bool) (context.Context, context.CancelFunc, context.Context, context.CancelFunc, context.Context, context.CancelFunc, context.Context, context.CancelFunc) {
	noop := func() {}
	if skip {
		return ctx, noop, ctx, noop, ctx, noop, ctx, noop
	}
	budgets := []*workflow.TimeBudget{
		timeBudgetOf(fused.merge.Config),
		timeBudgetOf(fused.finalize.Config),
		timeBudgetOf(fused.verdict.Config),
	}
	if fused.hasSummarize {
		budgets = append(budgets, timeBudgetOf(fused.summarize.Config))
	}
	plans := childTimePlans(ctx, budgets)
	contexts := make([]context.Context, len(budgets))
	cancels := make([]context.CancelFunc, len(budgets))
	for i, budget := range budgets {
		phaseCtx, cancel, skipped := withConfiguredTimeBudget(ctx, budget, plans[i])
		if skipped {
			phaseCtx, cancel = alreadyCanceledContext(ctx)
		}
		contexts[i] = phaseCtx
		cancels[i] = cancel
	}
	summarizeCtx, summarizeCancel := ctx, noop
	if fused.hasSummarize {
		summarizeCtx, summarizeCancel = contexts[3], cancels[3]
	}
	return contexts[0], cancels[0], contexts[1], cancels[1], contexts[2], cancels[2], summarizeCtx, summarizeCancel
}

// postMergeFusedStepFunc optimizes the normal post-review tail:
// merge -> finalize -> verdict -> summarize. Cluster merge is still the
// grouped->flat boundary, but each resolved cluster is immediately finalized
// and summarized while other merge clusters are still running. The verdict waits
// for every finalize shard so it always sees the complete finding set; overall
// summarization starts as soon as the verdict is ready and can overlap with
// remaining finding summaries.
func (e *Engine) postMergeFusedStepFunc(fused postMergeFusedSpec) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		mergeSC := e.stepContext(fused.merge.Config, sc.Req)
		finalizeSC := e.stepContext(fused.finalize.Config, sc.Req)
		verdictSC := e.stepContext(fused.verdict.Config, sc.Req)
		var summarizeSC *stepContext
		if fused.hasSummarize {
			summarizeSC = e.stepContext(fused.summarize.Config, sc.Req)
		}
		mergeCtx, mergeCancel, finalizeCtx, finalizeCancel, verdictCtx, verdictCancel, summarizeCtx, summarizeCancel := pipelinePhaseContexts(ctx, fused, sc.Req.SkipWorkflowTimeBudget)
		defer mergeCancel()
		defer finalizeCancel()
		defer verdictCancel()
		defer summarizeCancel()

		if err := injectGroups(st, fused.merge.FindingsFrom, mergeSC.Req.SkipSuggestions); err != nil {
			return err
		}
		vr := st.vectorResults()
		mergeInputs := pairwiseMergeInputs(vr)
		verifiedMergeInputs := flattenPairwiseMergeInputs(mergeInputs)

		if allVectorsFailed(vr) || len(mergeInputs) == 0 {
			mergeResult := emptyVerifiedMergeResult()
			warning := "No verified findings remained; skipped merge agent and returning empty findings"
			if allVectorsFailed(vr) {
				mergeSC.Engine.logf(ctx, "All vector reviewers failed; skipping merge agent and emitting empty result")
				warning = "All vector reviewers failed; skipped merge agent and returning empty findings"
			} else {
				mergeSC.Engine.logf(ctx, "No verified findings remained; skipping merge agent and emitting empty result")
			}
			st.mu.Lock()
			st.mergeRuns = append(st.mergeRuns, mergeResult.run)
			st.warnings = append(st.warnings, warning)
			st.result = &model.ReviewResult{
				Findings:               nil,
				OverallCorrectness:     mergeResult.resp.OverallCorrectness,
				OverallExplanation:     mergeResult.resp.OverallExplanation,
				OverallConfidenceScore: mergeResult.resp.OverallConfidenceScore,
			}
			st.mu.Unlock()
			if err := e.verdictStepFunc(nil)(verdictCtx, verdictSC, st); err != nil {
				return err
			}
			if fused.hasSummarize {
				return e.summarizeStepFunc(nil)(summarizeCtx, summarizeSC, st)
			}
			return nil
		}

		mergeConstraints, mergeSchema := mergeSchemaForStep(mergeSC.Req)
		userPrompt := st.enrichedPrompt
		if strings.TrimSpace(userPrompt) == "" {
			userPrompt = "{}"
		}

		findings, reviewerByID := flattenMergeMembers(mergeInputs)
		clusters := dedupe.Clusters(findings, dedupe.Possible)
		if len(mergeInputs) == 1 {
			clusters = singletonFindingClusters(len(findings))
		}
		outcomes := make(chan clusterMergeOutcome, len(clusters))
		absorbed := 0
		llmClusters := 0
		var mergeWG sync.WaitGroup
		for ci, cluster := range clusters {
			clusterFindings := make([]model.Finding, 0, len(cluster))
			for _, idx := range cluster {
				clusterFindings = append(clusterFindings, findings[idx])
			}
			reduced, folded := mechanicallyDedupeFindings(clusterFindings)
			absorbed += folded
			if len(reduced) == 1 {
				outcomes <- clusterMergeOutcome{index: ci, findings: reduced}
				continue
			}
			llmClusters++
			mergeWG.Add(1)
			go func(ci int, reduced []model.Finding) {
				defer mergeWG.Done()
				merged, run := mergeSC.Engine.runClusterMergeAgent(mergeCtx, userPrompt, st.contextNotes, reduced, reviewerByID, mergeSchema, mergeConstraints, mergeSC.Req)
				outcomes <- clusterMergeOutcome{index: ci, findings: merged, run: run, hasRun: run.Name != ""}
			}(ci, reduced)
		}
		go func() {
			mergeWG.Wait()
			close(outcomes)
		}()

		rawMergedByCluster := make([][]model.Finding, len(clusters))
		finalizedByCluster := make([][]model.Finding, len(clusters))
		summarizedByCluster := make([][]model.Finding, len(clusters))
		mergeRunsByCluster := make([]*model.AgentRun, len(clusters))
		finalizeRunsByCluster := make([]*model.AgentRun, len(clusters))
		summarizeRunsByCluster := make([]*model.AgentRun, len(clusters))
		finalizeWarningsByCluster := make([][]string, len(clusters))
		summarizeWarningsByCluster := make([][]string, len(clusters))
		var resultMu sync.Mutex
		var finalizeWG sync.WaitGroup
		var summarizeWG sync.WaitGroup
		seenFindingIDs := make(map[string]struct{}, len(findings))

		for outcome := range outcomes {
			merged := append([]model.Finding(nil), outcome.findings...)
			if overwrote := normalizeFindingIDsWithSeen(merged, seenFindingIDs); overwrote > 0 {
				mergeSC.Engine.logf(ctx, "Review generated replacement IDs for invalid finding IDs: count=%d", overwrote)
			}
			mergeInputVerification(merged, verifiedMergeInputs)
			rawMergedByCluster[outcome.index] = merged
			filtered := filterByPriority(merged, mergeSC.Req.PriorityThreshold)
			if outcome.hasRun {
				run := outcome.run
				mergeRunsByCluster[outcome.index] = &run
			}
			if len(filtered) == 0 {
				finalizedByCluster[outcome.index] = filtered
				if fused.hasSummarize {
					summarizedByCluster[outcome.index] = filtered
				}
				continue
			}

			finalizeWG.Add(1)
			if fused.hasSummarize {
				summarizeWG.Add(1)
			}
			go func(idx int, shard []model.Finding) {
				shardResult := &model.ReviewResult{Findings: append([]model.Finding(nil), shard...)}
				finalized, finalizeRun, finalizeWarnings := runFinalizeShard(finalizeCtx, finalizeSC, st, shardResult)

				resultMu.Lock()
				finalizedByCluster[idx] = append([]model.Finding(nil), finalized.Findings...)
				if finalizeRun != nil {
					finalizeRunsByCluster[idx] = finalizeRun
				}
				finalizeWarningsByCluster[idx] = finalizeWarnings
				resultMu.Unlock()

				// Snapshot the summarize input before releasing the finalize barrier.
				// After the barrier the verdict path normalizes finalized finding IDs
				// (which mutates the shared Verification pointers), and that runs
				// concurrently with this summary — so summarize must read an
				// independent deep copy, not the finalized findings themselves.
				var summarizeInput *model.ReviewResult
				if fused.hasSummarize {
					if clone, err := finalized.Clone(); err == nil {
						summarizeInput = clone
					} else {
						// Clone of an in-memory result effectively never fails, but if
						// it does, fall back to a manual deep copy rather than sharing
						// the finalized findings — the post-barrier ID normalization
						// mutates Verification.ID through those pointers concurrently.
						mergeSC.Engine.logf(ctx, "Summarize input clone failed; manual-copying to avoid a data race: error=%v", err)
						summarizeInput = &model.ReviewResult{Findings: deepCopyFindingsForSummarize(finalized.Findings)}
					}
				}

				// Release the finalize barrier now (not deferred to after summarize)
				// so the verdict and overall summary overlap the remaining per-cluster
				// summaries. Reached on every path — runFinalizeShard never returns
				// early — so the count stays balanced.
				finalizeWG.Done()

				if fused.hasSummarize {
					defer summarizeWG.Done()
					summarized, summarizeRun, summarizeWarnings := runSummarizeShard(summarizeCtx, summarizeSC, summarizeInput)
					resultMu.Lock()
					summarizedByCluster[idx] = append([]model.Finding(nil), summarized.Findings...)
					if summarizeRun != nil {
						summarizeRunsByCluster[idx] = summarizeRun
					}
					summarizeWarningsByCluster[idx] = summarizeWarnings
					resultMu.Unlock()
				}
			}(outcome.index, filtered)
		}

		finalizeWG.Wait()
		finalizedFindings := appendClusterFindings(finalizedByCluster)
		if overwrote := normalizeFindingIDsWithSeen(finalizedFindings, nil); overwrote > 0 {
			mergeSC.Engine.logf(ctx, "Review generated replacement IDs for invalid finding IDs: count=%d", overwrote)
		}
		rawMergedCount := len(appendClusterFindings(rawMergedByCluster))

		base := &model.ReviewResult{
			Findings:               finalizedFindings,
			OverallCorrectness:     aggregateOverallCorrectness(mergeInputs, rawMergedCount),
			OverallExplanation:     fmt.Sprintf("Merged %d reviewer finding lists (%d findings) into %d findings: %d absorbed mechanically, %d clusters judged by merge agents.", len(mergeInputs), len(findings), rawMergedCount, absorbed, llmClusters),
			OverallConfidenceScore: maxOverallConfidence(mergeInputs),
		}
		verdict, verdictRun, verdictWarnings := runVerdictShard(verdictCtx, verdictSC, st, base)

		var overallSummarizeRun *model.AgentRun
		overallSummarizeWarnings := []string(nil)
		// With no finalized findings the verdict's overall explanation is a short
		// static message, so skip the overall-summary LLM call entirely.
		if fused.hasSummarize && len(finalizedFindings) > 0 {
			overall, run, warnings := runOverallSummarize(summarizeCtx, summarizeSC, verdict.OverallExplanation)
			verdict.OverallExplanation = overall
			overallSummarizeRun = run
			overallSummarizeWarnings = warnings
		}
		summarizeWG.Wait()

		finalFindings := finalizedFindings
		if fused.hasSummarize {
			finalFindings = appendClusterFindings(summarizedByCluster)
			// Summarize ran on a pre-normalization clone, so adopt the IDs already
			// assigned to finalizedFindings (same findings, same per-cluster order)
			// rather than re-normalizing — re-normalizing would generate fresh random
			// IDs and drift from the verdict's view. Fall back to normalization only
			// if the counts somehow disagree.
			if len(finalFindings) == len(finalizedFindings) {
				for i := range finalFindings {
					finalFindings[i].ID = finalizedFindings[i].ID
					if finalFindings[i].Verification != nil {
						finalFindings[i].Verification.ID = finalizedFindings[i].ID
					}
				}
			} else {
				normalizeFindingIDsWithSeen(finalFindings, nil)
			}
		}
		verdict.Findings = finalFindings

		mergeRuns := orderedRuns(mergeRunsByCluster)
		if len(mergeRuns) == 0 {
			mergeRuns = append(mergeRuns, model.AgentRun{Name: "Merge Findings", Role: "merge", Status: model.AgentRunStatusSkipped})
		}
		finalizeRuns := orderedRuns(finalizeRunsByCluster)
		summarizeRuns := orderedRuns(summarizeRunsByCluster)
		if overallSummarizeRun != nil {
			summarizeRuns = append(summarizeRuns, *overallSummarizeRun)
		}
		finalizeWarnings := appendClusterWarnings(finalizeWarningsByCluster)
		summarizeWarnings := appendClusterWarnings(summarizeWarningsByCluster)
		finalizeUsage := tokenUsageForRuns(finalizeRuns)
		summarizeUsage := tokenUsageForRuns(summarizeRuns)

		mergeSC.Engine.logf(ctx, "Mechanical merge: findings=%d clusters=%d llm_clusters=%d absorbed=%d merged=%d",
			len(findings), len(clusters), llmClusters, absorbed, rawMergedCount)

		st.mu.Lock()
		st.mergeRuns = append(st.mergeRuns, mergeRuns...)
		st.finalizeRuns = append(st.finalizeRuns, finalizeRuns...)
		if verdictRun != nil {
			st.verdictRun = verdictRun
			st.verdictUsage = verdictRun.TokensUsed
		}
		st.summarizeRuns = append(st.summarizeRuns, summarizeRuns...)
		st.finalizeUsage = addTokenUsage(st.finalizeUsage, finalizeUsage)
		st.summarizeUsage = addTokenUsage(st.summarizeUsage, summarizeUsage)
		st.warnings = append(st.warnings, finalizeWarnings...)
		st.warnings = append(st.warnings, summarizeWarnings...)
		st.warnings = append(st.warnings, verdictWarnings...)
		st.warnings = append(st.warnings, overallSummarizeWarnings...)
		st.result = verdict
		st.mu.Unlock()
		return nil
	}
}

func mergeSchemaForStep(req model.ReviewRequest) (llm.ResponseConstraints, []byte) {
	constraints := llm.ResponseConstraints{}
	var schema []byte
	if req.UseJSONSchema {
		constraints = mergeConstraintsForRequest(req)
		if hasResponseConstraints(constraints) {
			schema = llm.MergeSchemaWithConstraintsFor(constraints, req.SkipSuggestions)
		} else if req.SkipSuggestions {
			schema = llm.MergeSchemaWithoutSuggestions
		} else {
			schema = llm.MergeSchema
		}
	}
	return constraints, schema
}

func runFinalizeShard(ctx context.Context, sc *stepContext, st *PipelineState, in *model.ReviewResult) (*model.ReviewResult, *model.AgentRun, []string) {
	opts := FinalizeOptions{
		UseJSONSchema:            sc.Req.UseJSONSchema,
		MaxOutputRetries:         sc.Req.MaxOutputRetries,
		MaxReasoningSeconds:      sc.Req.MaxReasoningSeconds,
		MaxReasoningLoopRepeats:  sc.Req.MaxReasoningLoopRepeats,
		DisableParallelToolCalls: sc.Req.DisableParallelToolCalls,
		DisablePatchSummary:      sc.Req.DisablePatchSummary,
		RepoRoot:                 sc.Req.RepoRoot,
		PriorityThreshold:        sc.Req.PriorityThreshold,
		ContextNotes:             st.contextNotes,
	}
	finalized, run, err := sc.Engine.Finalize(ctx, st.Enriched, in, opts)
	if err != nil {
		sc.Engine.logf(ctx, "Finalize failed for shard, using verified shard: error=%v", err)
		run.Name = "finalize"
		run.Role = "finalize"
		run.Status = model.AgentRunStatusFailed
		run.Error = err.Error()
		return in, &run, []string{fmt.Sprintf("Finalize failed: %v; using verified result", err)}
	}
	warnings := append([]string(nil), finalized.Warnings...)
	finalized.Warnings = nil
	return finalized, &run, warnings
}

func runVerdictShard(ctx context.Context, sc *stepContext, st *PipelineState, in *model.ReviewResult) (*model.ReviewResult, *model.AgentRun, []string) {
	opts := VerdictOptions{
		UseJSONSchema:            sc.Req.UseJSONSchema,
		MaxOutputRetries:         sc.Req.MaxOutputRetries,
		MaxReasoningSeconds:      sc.Req.MaxReasoningSeconds,
		MaxReasoningLoopRepeats:  sc.Req.MaxReasoningLoopRepeats,
		DisableParallelToolCalls: sc.Req.DisableParallelToolCalls,
		DisablePatchSummary:      sc.Req.DisablePatchSummary,
		RepoRoot:                 sc.Req.RepoRoot,
		PriorityThreshold:        sc.Req.PriorityThreshold,
		ContextNotes:             st.contextNotes,
	}
	verdict, run, err := sc.Engine.Verdict(ctx, st.Enriched, in, opts)
	if err != nil {
		sc.Engine.logf(ctx, "Verdict failed, using merged overall fields: error=%v", err)
		applyVerdictFallback(in, model.PriorityThresholdRank(sc.Req.PriorityThreshold))
		run.Name = "verdict"
		run.Role = "verdict"
		run.Status = model.AgentRunStatusFailed
		run.Error = err.Error()
		return in, &run, []string{fmt.Sprintf("Verdict failed: %v; using merged overall fields", err)}
	}
	warnings := append([]string(nil), verdict.Warnings...)
	verdict.Warnings = nil
	return verdict, &run, warnings
}

func runSummarizeShard(ctx context.Context, sc *stepContext, in *model.ReviewResult) (*model.ReviewResult, *model.AgentRun, []string) {
	opts := SummarizeOptions{
		UseJSONSchema:            sc.Req.UseJSONSchema,
		MaxOutputRetries:         sc.Req.MaxOutputRetries,
		MaxReasoningSeconds:      sc.Req.MaxReasoningSeconds,
		MaxReasoningLoopRepeats:  sc.Req.MaxReasoningLoopRepeats,
		DisableParallelToolCalls: sc.Req.DisableParallelToolCalls,
		DisablePatchSummary:      sc.Req.DisablePatchSummary,
		RepoRoot:                 sc.Req.RepoRoot,
	}
	summarized, run, err := sc.Engine.Summarize(ctx, in, opts)
	if err != nil {
		sc.Engine.logf(ctx, "Summarize failed for shard, using finalized shard: error=%v", err)
		run.Name = "summarize"
		run.Role = "summarize"
		run.Status = model.AgentRunStatusFailed
		run.Error = err.Error()
		return in, &run, []string{fmt.Sprintf("Summarize failed: %v; using finalized result", err)}
	}
	warnings := append([]string(nil), summarized.Warnings...)
	summarized.Warnings = nil
	return summarized, &run, warnings
}

func runOverallSummarize(ctx context.Context, sc *stepContext, overall string) (string, *model.AgentRun, []string) {
	opts := SummarizeOptions{
		UseJSONSchema:            sc.Req.UseJSONSchema,
		MaxOutputRetries:         sc.Req.MaxOutputRetries,
		MaxReasoningSeconds:      sc.Req.MaxReasoningSeconds,
		MaxReasoningLoopRepeats:  sc.Req.MaxReasoningLoopRepeats,
		DisableParallelToolCalls: sc.Req.DisableParallelToolCalls,
		DisablePatchSummary:      sc.Req.DisablePatchSummary,
		RepoRoot:                 sc.Req.RepoRoot,
	}
	summary, run, err := sc.Engine.SummarizeOverall(ctx, overall, opts)
	if err != nil {
		sc.Engine.logf(ctx, "Summarize failed for overall explanation, using verdict text: error=%v", err)
		run.Name = "summarize"
		run.Role = "summarize"
		run.Status = model.AgentRunStatusFailed
		run.Error = err.Error()
		return overall, &run, []string{fmt.Sprintf("Summarize failed: %v; using finalized result", err)}
	}
	return summary, &run, nil
}

func appendClusterFindings(clusters [][]model.Finding) []model.Finding {
	var out []model.Finding
	for _, findings := range clusters {
		out = append(out, findings...)
	}
	return out
}

// deepCopyFindingsForSummarize copies findings with their Verification and
// Finalization pointers detached, so a concurrent post-finalize ID normalization
// (which writes Verification.ID through those pointers) cannot race the summarize
// pass reading this copy. Fallback for the effectively-impossible case where
// ReviewResult.Clone fails.
func deepCopyFindingsForSummarize(in []model.Finding) []model.Finding {
	out := make([]model.Finding, len(in))
	for i, f := range in {
		out[i] = f
		if f.Verification != nil {
			v := *f.Verification
			out[i].Verification = &v
		}
		if f.Finalization != nil {
			fin := *f.Finalization
			out[i].Finalization = &fin
		}
	}
	return out
}

func singletonFindingClusters(n int) [][]int {
	clusters := make([][]int, n)
	for i := range n {
		clusters[i] = []int{i}
	}
	return clusters
}

func normalizeFindingIDsWithSeen(findings []model.Finding, seen map[string]struct{}) int {
	if seen == nil {
		seen = make(map[string]struct{}, len(findings))
	}
	overwrote := 0
	for i := range findings {
		if model.EnsureFindingID(&findings[i]) {
			overwrote++
		}
		if _, ok := seen[findings[i].ID]; ok {
			findings[i].ID = newReviewUUID(seen)
		}
		seen[findings[i].ID] = struct{}{}
		if findings[i].Verification != nil {
			findings[i].Verification.ID = findings[i].ID
		}
	}
	return overwrote
}

func newReviewUUID(seen map[string]struct{}) string {
	for {
		id := uuid.NewString()
		if _, ok := seen[id]; !ok {
			return id
		}
	}
}

func orderedRuns(runs []*model.AgentRun) []model.AgentRun {
	out := make([]model.AgentRun, 0, len(runs))
	for _, run := range runs {
		if run != nil {
			out = append(out, *run)
		}
	}
	return out
}

func appendClusterWarnings(clusters [][]string) []string {
	var out []string
	for _, warnings := range clusters {
		out = append(out, warnings...)
	}
	return out
}

func tokenUsageForRuns(runs []model.AgentRun) model.TokenUsage {
	usage := model.TokenUsage{}
	for _, run := range runs {
		usage = addTokenUsage(usage, run.TokensUsed)
	}
	return usage
}

// finalizeStepFunc runs the finalizer over the flat result. When findings are
// injected, they become the flat result first; otherwise the merged result (or a
// materialized fallback) is used. Finalize failure is soft.
func (e *Engine) finalizeStepFunc(findingsFrom []string) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		if len(findingsFrom) > 0 {
			groups, err := loadFindingsFiles(findingsFrom)
			if err != nil {
				return err
			}
			flat := flattenInjectedGroups(groups)
			// Apply the priority threshold to imported findings, matching what
			// merge and materializeFromGroups do, so --priority-threshold stays
			// effective for finalize-from-file workflows.
			findings := filterByPriority(flat.findings, sc.Req.PriorityThreshold)
			if sc.Req.SkipSuggestions {
				model.StripSuggestions(findings)
			}
			st.mu.Lock()
			st.result = &model.ReviewResult{
				Findings:               findings,
				OverallCorrectness:     flat.overallCorrectness,
				OverallExplanation:     flat.overallExplanation,
				OverallConfidenceScore: flat.overallConfidence,
			}
			st.mu.Unlock()
		}
		st.mu.Lock()
		if st.result == nil {
			st.result = st.materializeFromGroupsLocked(sc.Req)
		}
		in := st.result
		contextNotes := st.contextNotes
		st.mu.Unlock()

		if in == nil || len(in.Findings) == 0 {
			return nil
		}
		opts := FinalizeOptions{
			UseJSONSchema:            sc.Req.UseJSONSchema,
			MaxOutputRetries:         sc.Req.MaxOutputRetries,
			MaxReasoningSeconds:      sc.Req.MaxReasoningSeconds,
			MaxReasoningLoopRepeats:  sc.Req.MaxReasoningLoopRepeats,
			DisableParallelToolCalls: sc.Req.DisableParallelToolCalls,
			DisablePatchSummary:      sc.Req.DisablePatchSummary,
			SkipSuggestions:          sc.Req.SkipSuggestions,
			RepoRoot:                 sc.Req.RepoRoot,
			PriorityThreshold:        sc.Req.PriorityThreshold,
			ContextNotes:             contextNotes,
		}
		finalized, finalizeRun, err := sc.Engine.Finalize(ctx, st.Enriched, in, opts)
		st.mu.Lock()
		defer st.mu.Unlock()
		if err != nil {
			sc.Engine.logf(ctx, "Finalize failed, using verified result: error=%v", err)
			st.warnings = append(st.warnings, fmt.Sprintf("Finalize failed: %v; using verified result", err))
			finalizeRun.Name = "finalize"
			finalizeRun.Role = "finalize"
			finalizeRun.Status = model.AgentRunStatusFailed
			finalizeRun.Error = err.Error()
			st.finalizeRuns = append(st.finalizeRuns, finalizeRun)
			st.finalizeUsage = finalizeRun.TokensUsed
			return nil
		}
		// Finalize may surface a mismatch warning on the cloned result; fold it
		// into the pipeline warnings (the executor owns Warnings assembly).
		if len(finalized.Warnings) > 0 {
			st.warnings = append(st.warnings, finalized.Warnings...)
			finalized.Warnings = nil
		}
		st.result = finalized
		st.finalizeRuns = append(st.finalizeRuns, finalizeRun)
		st.finalizeUsage = finalizeRun.TokensUsed
		return nil
	}
}

// verdictStepFunc runs the verdict agent over all finalized findings. Verdict
// failure is soft: merge-derived overall fields are kept, coerced to satisfy the
// priority-derived correctness constraint so a transient failure never emits a
// blocking verdict for non-blocking findings.
func (e *Engine) verdictStepFunc(findingsFrom []string) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		if len(findingsFrom) > 0 {
			groups, err := loadFindingsFiles(findingsFrom)
			if err != nil {
				return err
			}
			flat := flattenInjectedGroups(groups)
			findings := filterByPriority(flat.findings, sc.Req.PriorityThreshold)
			if sc.Req.SkipSuggestions {
				model.StripSuggestions(findings)
			}
			st.mu.Lock()
			st.result = &model.ReviewResult{
				Findings:               findings,
				OverallCorrectness:     flat.overallCorrectness,
				OverallExplanation:     flat.overallExplanation,
				OverallConfidenceScore: flat.overallConfidence,
			}
			st.mu.Unlock()
		}
		st.mu.Lock()
		if st.result == nil {
			st.result = st.materializeFromGroupsLocked(sc.Req)
		}
		in := st.result
		contextNotes := st.contextNotes
		st.mu.Unlock()

		if in == nil {
			return nil
		}
		opts := VerdictOptions{
			UseJSONSchema:            sc.Req.UseJSONSchema,
			MaxOutputRetries:         sc.Req.MaxOutputRetries,
			MaxReasoningSeconds:      sc.Req.MaxReasoningSeconds,
			MaxReasoningLoopRepeats:  sc.Req.MaxReasoningLoopRepeats,
			DisableParallelToolCalls: sc.Req.DisableParallelToolCalls,
			DisablePatchSummary:      sc.Req.DisablePatchSummary,
			RepoRoot:                 sc.Req.RepoRoot,
			PriorityThreshold:        sc.Req.PriorityThreshold,
			ContextNotes:             contextNotes,
		}
		verdict, verdictRun, err := sc.Engine.Verdict(ctx, st.Enriched, in, opts)
		st.mu.Lock()
		defer st.mu.Unlock()
		if err != nil {
			sc.Engine.logf(ctx, "Verdict failed, using merged overall fields: error=%v", err)
			st.warnings = append(st.warnings, fmt.Sprintf("Verdict failed: %v; using merged overall fields", err))
			applyVerdictFallback(in, model.PriorityThresholdRank(sc.Req.PriorityThreshold))
			verdictRun.Name = "verdict"
			verdictRun.Role = "verdict"
			verdictRun.Status = model.AgentRunStatusFailed
			verdictRun.Error = err.Error()
			st.verdictRun = &verdictRun
			st.verdictUsage = verdictRun.TokensUsed
			return nil
		}
		if len(verdict.Warnings) > 0 {
			st.warnings = append(st.warnings, verdict.Warnings...)
			verdict.Warnings = nil
		}
		st.result = verdict
		st.verdictRun = &verdictRun
		st.verdictUsage = verdictRun.TokensUsed
		return nil
	}
}

// summarizeStepFunc runs the summarizer over the flat result, after finalize.
// Like finalize it accepts injected findings (so `--step summarize --findings
// finalized.json` works) and falls back to the merged/materialized result
// otherwise. Summarize failure is soft: the finalized bodies are kept.
func (e *Engine) summarizeStepFunc(findingsFrom []string) stepFunc {
	return func(ctx context.Context, sc *stepContext, st *PipelineState) error {
		if len(findingsFrom) > 0 {
			groups, err := loadFindingsFiles(findingsFrom)
			if err != nil {
				return err
			}
			flat := flattenInjectedGroups(groups)
			findings := filterByPriority(flat.findings, sc.Req.PriorityThreshold)
			if sc.Req.SkipSuggestions {
				model.StripSuggestions(findings)
			}
			st.mu.Lock()
			st.result = &model.ReviewResult{
				Findings:               findings,
				OverallCorrectness:     flat.overallCorrectness,
				OverallExplanation:     flat.overallExplanation,
				OverallConfidenceScore: flat.overallConfidence,
			}
			st.mu.Unlock()
		}
		st.mu.Lock()
		if st.result == nil {
			st.result = st.materializeFromGroupsLocked(sc.Req)
		}
		in := st.result
		st.mu.Unlock()

		if in == nil || len(in.Findings) == 0 {
			return nil
		}
		opts := SummarizeOptions{
			UseJSONSchema:            sc.Req.UseJSONSchema,
			MaxOutputRetries:         sc.Req.MaxOutputRetries,
			MaxReasoningSeconds:      sc.Req.MaxReasoningSeconds,
			MaxReasoningLoopRepeats:  sc.Req.MaxReasoningLoopRepeats,
			DisableParallelToolCalls: sc.Req.DisableParallelToolCalls,
			DisablePatchSummary:      sc.Req.DisablePatchSummary,
			RepoRoot:                 sc.Req.RepoRoot,
		}
		summarized, summarizeRun, err := sc.Engine.Summarize(ctx, in, opts)
		st.mu.Lock()
		defer st.mu.Unlock()
		if err != nil {
			sc.Engine.logf(ctx, "Summarize failed, using finalized result: error=%v", err)
			st.warnings = append(st.warnings, fmt.Sprintf("Summarize failed: %v; using finalized result", err))
			summarizeRun.Name = "summarize"
			summarizeRun.Role = "summarize"
			summarizeRun.Status = model.AgentRunStatusFailed
			summarizeRun.Error = err.Error()
			st.summarizeRuns = append(st.summarizeRuns, summarizeRun)
			st.summarizeUsage = summarizeRun.TokensUsed
			return nil
		}
		// Fold any mismatch warning the summarizer surfaced on the cloned result
		// into the pipeline warnings (the executor owns Warnings assembly).
		if len(summarized.Warnings) > 0 {
			st.warnings = append(st.warnings, summarized.Warnings...)
			summarized.Warnings = nil
		}
		st.result = summarized
		st.summarizeRuns = append(st.summarizeRuns, summarizeRun)
		st.summarizeUsage = summarizeRun.TokensUsed
		return nil
	}
}

// injectGroups loads findings files (one group per file) and registers them as
// synthetic reviewer groups, used to seed verify/dedupe/merge from disk.
func injectGroups(st *PipelineState, findingsFrom []string, skipSuggestions bool) error {
	if len(findingsFrom) == 0 {
		return nil
	}
	groups, err := loadFindingsFiles(findingsFrom)
	if err != nil {
		return err
	}
	if skipSuggestions {
		stripInjectedGroupSuggestions(groups)
	}
	seq := st.nextInjectSeq()
	for i, g := range groups {
		st.setGroup(fmt.Sprintf("injected-%d-%d", seq, i), injectedAgentResult(g), nil)
	}
	return nil
}

func stripInjectedGroupSuggestions(groups []injectedGroup) {
	for gi := range groups {
		model.StripSuggestions(groups[gi].findings)
	}
}
