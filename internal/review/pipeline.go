package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
)

// PipelineState threads the review's evolving data between steps. It holds the
// (possibly enriched) context, the per-vector finding groups, the flat merged
// result, and the accumulated telemetry. Steps mutate it; the executor assembles
// the final ReviewResult from it after all steps complete.
//
// Per-vector grouping lives only here, in groupByID — it is never serialized.
// Steps that need grouped findings (dedupe, merge) read the groups; the merge
// step is the single grouped→flat transform that sets result.
type PipelineState struct {
	mu sync.Mutex

	Base     *model.ReviewContext
	Enriched *model.ReviewContext

	baseTemplate    string
	enrichedPrompt  string
	contextMessages []llm.Message
	contextNotes    string
	styleGuides     []model.StyleGuide
	hasToolchain    bool
	promptsReady    bool

	groupOrder []string
	groupByID  map[string]*groupEntry
	injectSeq  int

	// limiter is the run-global cap on concurrent LLM agent loops, shared by
	// every step. Set once in Run before any step goroutine starts; read-only
	// afterwards. Verify steps additionally use it for ordered admission in
	// their spawn loops.
	limiter *Limiter

	// Flat result, set by the merge step, by findings injection into finalize,
	// or by materialization from groups.
	result *model.ReviewResult

	// Telemetry, aggregated into the final ReviewResult by the executor.
	contextRun       *model.AgentRun
	contextReasoning string
	contextErr       error
	dedupeRuns       []model.AgentRun
	mergeRuns        []model.AgentRun
	mergeReasoning   string
	finalizeRun      *model.AgentRun
	summarizeRun     *model.AgentRun
	verifyUsage      model.TokenUsage
	finalizeUsage    model.TokenUsage
	summarizeUsage   model.TokenUsage
	warnings         []string
}

type groupEntry struct {
	id      string
	result  agentResult
	session *reviewerSession
	filled  bool
}

func newPipelineState(reviewCtx *model.ReviewContext, reviewOrder []string) *PipelineState {
	st := &PipelineState{
		Base:      reviewCtx,
		Enriched:  reviewCtx,
		groupByID: make(map[string]*groupEntry),
	}
	for _, id := range reviewOrder {
		st.groupByID[id] = &groupEntry{id: id}
		st.groupOrder = append(st.groupOrder, id)
	}
	return st
}

func (st *PipelineState) setGroup(id string, result agentResult, session *reviewerSession) {
	st.mu.Lock()
	defer st.mu.Unlock()
	g, ok := st.groupByID[id]
	if !ok {
		g = &groupEntry{id: id}
		st.groupByID[id] = g
		st.groupOrder = append(st.groupOrder, id)
	}
	g.result = result
	if session != nil {
		g.session = session
	}
	g.filled = true
}

func (st *PipelineState) group(id string) *groupEntry {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.groupByID[id]
}

// nextInjectSeq returns a monotonically increasing sequence number so each
// injection's synthetic group ids are unique across the whole pipeline run and
// cannot overwrite a prior injection's groups.
func (st *PipelineState) nextInjectSeq() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	seq := st.injectSeq
	st.injectSeq++
	return seq
}

// vectorResults returns the filled groups in deterministic order. The returned
// agentResult values share their *llm.ReviewResponse pointers with the groups,
// so in-place finding mutations (e.g. verify) propagate automatically.
func (st *PipelineState) vectorResults() []agentResult {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.vectorResultsLocked()
}

// vectorResultsLocked is the body of vectorResults; the caller must hold st.mu.
func (st *PipelineState) vectorResultsLocked() []agentResult {
	out := make([]agentResult, 0, len(st.groupOrder))
	for _, id := range st.groupOrder {
		if g := st.groupByID[id]; g.filled {
			out = append(out, g.result)
		}
	}
	return out
}

// vectorResult returns the filled group's result for id. The agentResult shares
// its *llm.ReviewResponse pointer with the group, so in-place finding mutation
// (verify) propagates without write-back.
func (st *PipelineState) vectorResult(id string) (agentResult, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	g, ok := st.groupByID[id]
	if !ok || !g.filled {
		return agentResult{}, false
	}
	return g.result, true
}

// setVectorResponse swaps group id's response pointer; dedupe replaces the
// response rather than mutating findings in place.
func (st *PipelineState) setVectorResponse(id string, resp *llm.ReviewResponse) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if g, ok := st.groupByID[id]; ok && g.filled {
		g.result.resp = resp
	}
}

// writeBackVectorResults copies (possibly replaced) response pointers from a
// vectorResults slice back into the groups, needed after dedupe which swaps the
// response pointer rather than mutating findings in place.
func (st *PipelineState) writeBackVectorResults(vr []agentResult) {
	st.mu.Lock()
	defer st.mu.Unlock()
	i := 0
	for _, id := range st.groupOrder {
		g := st.groupByID[id]
		if !g.filled {
			continue
		}
		if i < len(vr) {
			g.result.resp = vr[i].resp
		}
		i++
	}
}

// stepContext is the per-step engine + request, with overrides applied.
type stepContext struct {
	Engine *Engine
	Req    model.ReviewRequest
}

type stepFunc func(ctx context.Context, sc *stepContext, st *PipelineState) error

type boundStep struct {
	label       string
	needsSource bool
	override    *workflow.StepOverride
	run         stepFunc
}

type planUnit struct {
	// lanes run concurrently; the steps within one lane run sequentially. A
	// plain sequential step is a single lane of length 1.
	lanes [][]boundStep
}

// Pipeline is a compiled, runnable workflow.
type Pipeline struct {
	engine      *Engine
	units       []planUnit
	reviewOrder []string
	needsSource bool
}

// NeedsSource reports whether any step requires a review source. When false, the
// caller may skip source/repo resolution entirely (e.g. a merge-from-file run).
func (p *Pipeline) NeedsSource() bool { return p.needsSource }

// BuildPipeline compiles a workflow spec into a runnable pipeline against the
// engine. Per-step overrides are resolved at run time against the engine's base
// profile and the supplied request.
func (e *Engine) BuildPipeline(spec workflow.Spec) (*Pipeline, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	manual := manualReviewVectors(spec)
	reviewOrder := reviewVectorOrder(spec)
	p := &Pipeline{engine: e, reviewOrder: reviewOrder}
	for _, entry := range spec.Steps {
		if entry.IsParallel() {
			unit := planUnit{}
			for _, sub := range entry.Parallel {
				var lane []boundStep
				for _, ls := range sub.LaneSteps() {
					bs, err := e.bindStep(ls, manual)
					if err != nil {
						return nil, err
					}
					lane = append(lane, bs)
					p.needsSource = p.needsSource || bs.needsSource
				}
				unit.lanes = append(unit.lanes, lane)
			}
			p.units = append(p.units, unit)
			continue
		}
		bs, err := e.bindStep(entry, manual)
		if err != nil {
			return nil, err
		}
		p.units = append(p.units, planUnit{lanes: [][]boundStep{{bs}}})
		p.needsSource = p.needsSource || bs.needsSource
	}
	return p, nil
}

// Run executes the pipeline against the given context, returning the assembled
// result and the (possibly enriched) context.
func (p *Pipeline) Run(ctx context.Context, reviewCtx *model.ReviewContext, req model.ReviewRequest) (*model.ReviewResult, *model.ReviewContext, error) {
	st := newPipelineState(reviewCtx, p.reviewOrder)
	st.limiter = NewLimiter(req.Concurrency)
	// Every agent loop in this run acquires admission from the same limiter,
	// capping LLM concurrency globally (reviewers, verify, dedupe, merge, ...).
	ctx = WithLimiter(ctx, st.limiter)
	var segments []model.SegmentRuntime
	for _, unit := range p.units {
		unitStart := time.Now()
		errs := make([]error, len(unit.lanes))
		var wg sync.WaitGroup
		for i, lane := range unit.lanes {
			wg.Add(1)
			go func(i int, lane []boundStep) {
				defer wg.Done()
				for _, bs := range lane {
					if err := bs.run(ctx, p.engine.stepContext(bs.override, req), st); err != nil {
						// Abort this lane on its first error; sibling lanes run
						// to the barrier below.
						errs[i] = err
						return
					}
				}
			}(i, lane)
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				return nil, nil, err
			}
		}
		segments = append(segments, unitSegment(unit, unitStart))
		if len(unit.lanes) > 1 {
			// One line for the whole concurrent group: its wall-clock span (the
			// slowest lane), which individual step lines cannot show.
			p.engine.logProgress(logging.StageReview, logging.StateDone,
				fmt.Sprintf("lanes=%d runtime=%s", len(unit.lanes), model.HumanDuration(time.Since(unitStart))))
		}
	}
	result := p.assemble(st, req)
	result.SegmentRuntimes = segments
	return result, st.Enriched, nil
}

// unitSegment records the wall-clock span of one executed pipeline unit. Each
// lane becomes one entry; a multi-step lane's labels are joined with "→" so
// single-step segments keep their plain step label.
func unitSegment(unit planUnit, start time.Time) model.SegmentRuntime {
	steps := make([]string, len(unit.lanes))
	for i, lane := range unit.lanes {
		labels := make([]string, len(lane))
		for j, bs := range lane {
			labels[j] = bs.label
		}
		steps[i] = strings.Join(labels, "→")
	}
	return model.SegmentRuntime{
		Steps:          steps,
		RuntimeSeconds: model.RuntimeSeconds(time.Since(start)),
	}
}

func (e *Engine) stepContext(override *workflow.StepOverride, req model.ReviewRequest) *stepContext {
	profile, effReq := override.Resolve(e.config, req)
	return &stepContext{Engine: e.withConfig(profile), Req: effReq}
}

// assemble builds the final ReviewResult: the flat findings/overall from the
// merge/finalize/injection, plus the aggregated telemetry.
func (p *Pipeline) assemble(st *PipelineState, req model.ReviewRequest) *model.ReviewResult {
	res := st.result
	if res == nil {
		res = st.materializeFromGroups(req)
	}
	allRuns, usage, toolCalls, reasoning := st.aggregateTelemetry()
	res.AgentRuns = allRuns
	res.Warnings = appendAgentRunWarnings(st.warnings, allRuns, st.contextErr)
	res.TokensUsed = usage
	res.VerifyTokensUsed = st.verifyUsage
	res.FinalizeTokensUsed = st.finalizeUsage
	res.SummarizeTokensUsed = st.summarizeUsage
	res.TotalToolCalls = toolCalls
	res.ReasoningEffort = reasoning
	return res
}

func (st *PipelineState) aggregateTelemetry() ([]model.AgentRun, model.TokenUsage, int, string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	var runs []model.AgentRun
	usage := model.TokenUsage{}
	toolCalls := 0
	reasoning := st.contextReasoning
	if st.contextRun != nil {
		runs = append(runs, *st.contextRun)
		usage = addTokenUsage(usage, st.contextRun.TokensUsed)
		toolCalls += st.contextRun.ToolCalls
	}
	for _, id := range st.groupOrder {
		g := st.groupByID[id]
		if !g.filled {
			continue
		}
		runs = append(runs, g.result.run)
		usage = addTokenUsage(usage, g.result.run.TokensUsed)
		toolCalls += g.result.run.ToolCalls
		if g.result.reasoningEffort != "" {
			reasoning = g.result.reasoningEffort
		}
	}
	for _, run := range st.dedupeRuns {
		runs = append(runs, run)
		usage = addTokenUsage(usage, run.TokensUsed)
		toolCalls += run.ToolCalls
	}
	for _, run := range st.mergeRuns {
		runs = append(runs, run)
		usage = addTokenUsage(usage, run.TokensUsed)
		toolCalls += run.ToolCalls
	}
	if st.mergeReasoning != "" {
		reasoning = st.mergeReasoning
	}
	if st.finalizeRun != nil {
		runs = append(runs, *st.finalizeRun)
	}
	if st.summarizeRun != nil {
		runs = append(runs, *st.summarizeRun)
	}
	return runs, usage, toolCalls, reasoning
}

// materializeFromGroups flattens the current groups into a flat result. Used by
// custom specs that have no merge step but still need a flat result for output
// or finalize.
func (st *PipelineState) materializeFromGroups(req model.ReviewRequest) *model.ReviewResult {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.materializeFromGroupsLocked(req)
}

// materializeFromGroupsLocked is the body of materializeFromGroups; the caller
// must hold st.mu. The finalize step uses this while already holding the lock,
// avoiding a self-deadlock on the non-reentrant mutex.
func (st *PipelineState) materializeFromGroupsLocked(req model.ReviewRequest) *model.ReviewResult {
	var findings []model.Finding
	for _, vr := range st.vectorResultsLocked() {
		if vr.resp != nil {
			findings = append(findings, vr.resp.Findings...)
		}
	}
	findings = filterByPriority(findings, req.PriorityThreshold)
	model.EnsureFindingIDs(findings)
	return &model.ReviewResult{Findings: findings}
}

// --- step binding ---

func (e *Engine) bindStep(entry workflow.StepEntry, manual map[string]bool) (boundStep, error) {
	t := entry.Type
	switch t {
	case workflow.StepCollectContext:
		return boundStep{label: t, needsSource: true, override: entry.Config, run: e.collectStepFunc()}, nil
	case workflow.StepVerify:
		return boundStep{label: t, override: entry.Config, run: e.verifyStepFunc(entry.FindingsFrom)}, nil
	case workflow.StepDedupe:
		return boundStep{label: t, override: entry.Config, run: e.dedupeStepFunc(entry.FindingsFrom)}, nil
	case workflow.StepMerge:
		return boundStep{label: t, override: entry.Config, run: e.mergeStepFunc(entry.FindingsFrom)}, nil
	case workflow.StepFinalize:
		return boundStep{label: t, override: entry.Config, run: e.finalizeStepFunc(entry.FindingsFrom)}, nil
	case workflow.StepSummarize:
		return boundStep{label: t, override: entry.Config, run: e.summarizeStepFunc(entry.FindingsFrom)}, nil
	}
	if id, ok := stepVector(t, workflow.StepReviewPrefix); ok {
		return boundStep{label: t, needsSource: true, override: entry.Config, run: e.reviewStepFunc(id, manual[id])}, nil
	}
	if id, ok := stepVector(t, workflow.StepVerifyPrefix); ok {
		return boundStep{label: t, override: entry.Config, run: e.verifyVectorStepFunc(id)}, nil
	}
	if id, ok := stepVector(t, workflow.StepDedupePrefix); ok {
		return boundStep{label: t, override: entry.Config, run: e.dedupeVectorStepFunc(id)}, nil
	}
	if id, ok := stepVector(t, workflow.StepExtractPrefix); ok {
		return boundStep{label: t, override: entry.Config, run: e.extractStepFunc(id)}, nil
	}
	if id, ok := stepVector(t, workflow.StepNudgePrefix); ok {
		return boundStep{label: t, override: entry.Config, run: e.nudgeStepFunc(id)}, nil
	}
	return boundStep{}, fmt.Errorf("workflow: unknown step type %q", t)
}

func stepVector(t, prefix string) (string, bool) {
	if len(t) > len(prefix) && t[:len(prefix)] == prefix {
		return t[len(prefix):], true
	}
	return "", false
}

// reviewVectorOrder returns the vector ids that appear in review:<id> steps, in
// spec order, so the grouped representation is deterministic regardless of
// parallel execution.
func reviewVectorOrder(spec workflow.Spec) []string {
	var order []string
	seen := map[string]bool{}
	for _, entry := range spec.FlatSteps() {
		if id, ok := stepVector(entry.Type, workflow.StepReviewPrefix); ok && !seen[id] {
			seen[id] = true
			order = append(order, id)
		}
	}
	return order
}

// manualReviewVectors returns the set of vectors that have standalone
// reasoning-extract / nudge steps, so their review step enables reasoning
// collection during the initial pass even when nudge_count is 0.
func manualReviewVectors(spec workflow.Spec) map[string]bool {
	manual := map[string]bool{}
	for _, entry := range spec.FlatSteps() {
		for _, prefix := range []string{workflow.StepExtractPrefix, workflow.StepNudgePrefix} {
			if id, ok := stepVector(entry.Type, prefix); ok {
				manual[id] = true
			}
		}
	}
	return manual
}

// --- findings injection ---

type injectedGroup struct {
	findings           []model.Finding
	overallCorrectness string
	overallExplanation string
	overallConfidence  float64
}

// loadFindingsFiles reads each path as the existing findings JSON (a ReviewResult
// object or a bare findings array). IDs are normalized globally across all files
// so injected findings are uniquely addressable downstream. Each file becomes
// one group, preserving order.
func loadFindingsFiles(paths []string) ([]injectedGroup, error) {
	groups := make([]injectedGroup, 0, len(paths))
	var all []model.Finding
	for _, path := range paths {
		g, err := loadFindingsFile(path)
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
		all = append(all, g.findings...)
	}
	if model.EnsureFindingIDs(all) > 0 {
		// Reflect normalized/unique IDs back into per-group slices.
		idx := 0
		for gi := range groups {
			for fi := range groups[gi].findings {
				groups[gi].findings[fi] = all[idx]
				idx++
			}
		}
	}
	return groups, nil
}

func loadFindingsFile(path string) (injectedGroup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return injectedGroup{}, fmt.Errorf("workflow: reading findings %s: %w", path, err)
	}
	trimmed := trimLeadingSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var findings []model.Finding
		if err := json.Unmarshal(data, &findings); err != nil {
			return injectedGroup{}, fmt.Errorf("workflow: parsing findings %s: %w", path, err)
		}
		return injectedGroup{findings: findings}, nil
	}
	var rr model.ReviewResult
	if err := json.Unmarshal(data, &rr); err != nil {
		return injectedGroup{}, fmt.Errorf("workflow: parsing findings %s: %w", path, err)
	}
	return injectedGroup{
		findings:           rr.Findings,
		overallCorrectness: rr.OverallCorrectness,
		overallExplanation: rr.OverallExplanation,
		overallConfidence:  rr.OverallConfidenceScore,
	}, nil
}

func trimLeadingSpace(data []byte) []byte {
	for i, b := range data {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return data[i:]
		}
	}
	return nil
}

func injectedAgentResult(g injectedGroup) agentResult {
	return agentResult{
		resp: &llm.ReviewResponse{
			Findings:               g.findings,
			OverallCorrectness:     g.overallCorrectness,
			OverallExplanation:     g.overallExplanation,
			OverallConfidenceScore: g.overallConfidence,
		},
		run: model.AgentRun{
			Name:     "Injected Findings",
			Role:     "review",
			Findings: len(g.findings),
			Status:   model.AgentRunStatusSkipped,
		},
	}
}

func flattenInjectedGroups(groups []injectedGroup) injectedGroup {
	out := injectedGroup{}
	for i, g := range groups {
		out.findings = append(out.findings, g.findings...)
		if i == 0 {
			out.overallCorrectness = g.overallCorrectness
			out.overallExplanation = g.overallExplanation
			out.overallConfidence = g.overallConfidence
		}
	}
	return out
}
