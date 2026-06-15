package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
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
		if vector.constraints.MinPriority != nil || vector.constraints.MaxPriority != nil || len(vector.constraints.AllowedCorrectness) > 0 {
			schema = llm.FindingsSchemaWithConstraints(vector.constraints)
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
		mine := sc.internalAgentContext(nil)
		compile := sc.internalAgentContext(nil)
		nudge := sc.internalAgentContext(nil)
		if sc.Override != nil {
			mine = sc.internalAgentContext(sc.Override.MineReasoning)
			compile = sc.internalAgentContext(sc.Override.CompileFindings)
			nudge = sc.internalAgentContext(sc.Override.Nudge)
		}
		if err := sc.Engine.reviewerInitial(ctx, session, sc.Req, mine.Engine, mine.Req); err != nil {
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
		if err := sc.Engine.reviewerNudges(ctx, session, sc.Req, compile.Engine, compile.Req, nudge.Engine, nudge.Req); err != nil {
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
		if err := injectGroups(st, findingsFrom); err != nil {
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
		if err := injectGroups(st, findingsFrom); err != nil {
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
		if err := injectGroups(st, findingsFrom); err != nil {
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
				mergeSchema = llm.MergeSchemaWithConstraints(mergeConstraints)
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
			RepoRoot:                 sc.Req.RepoRoot,
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
			st.finalizeRun = &finalizeRun
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
		st.finalizeRun = &finalizeRun
		st.finalizeUsage = finalizeRun.TokensUsed
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
			st.summarizeRun = &summarizeRun
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
		st.summarizeRun = &summarizeRun
		st.summarizeUsage = summarizeRun.TokensUsed
		return nil
	}
}

// injectGroups loads findings files (one group per file) and registers them as
// synthetic reviewer groups, used to seed verify/dedupe/merge from disk.
func injectGroups(st *PipelineState, findingsFrom []string) error {
	if len(findingsFrom) == 0 {
		return nil
	}
	groups, err := loadFindingsFiles(findingsFrom)
	if err != nil {
		return err
	}
	seq := st.nextInjectSeq()
	for i, g := range groups {
		st.setGroup(fmt.Sprintf("injected-%d-%d", seq, i), injectedAgentResult(g), nil)
	}
	return nil
}
