package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/toolchain"
	toolcatalog "github.com/dgrieser/nickpit/internal/tools"
	"github.com/dgrieser/nickpit/mappings"
	"github.com/dgrieser/nickpit/prompts"
)

type Engine struct {
	source                 model.ReviewSource
	llm                    llm.Client
	retrieval              retrieval.Engine
	config                 config.Profile
	trimmer                *Trimmer
	logger                 *logging.Logger
	searchToolOptimization bool
	toolchainCapture       func(ctx context.Context, repoRoot string, reviewCtx *model.ReviewContext) []model.ToolchainVersion
}

var searchFunctionQueryPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\((?:\))?$`)

// ErrEmptyDiff is returned when the resolved review context has no changed files
// and no diff content, meaning there is nothing meaningful to review.
var ErrEmptyDiff = errors.New("review: empty diff (no changed files and no diff content)")

const defaultMaxOutputRetries = config.DefaultMaxOutputRetries

type keyedLocker struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func (l *keyedLocker) lock(key string) func() {
	l.mu.Lock()
	if l.locks == nil {
		l.locks = make(map[string]*sync.Mutex)
	}
	lock, ok := l.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		l.locks[key] = lock
	}
	l.mu.Unlock()
	lock.Lock()
	return lock.Unlock
}

type toolRoundState struct {
	mu             sync.Mutex
	seenFiles      map[string]retrieval.FileContent
	seenFileRanges map[string][]model.LineRange
	seenToolCalls  map[string]struct{}
	fileLocks      keyedLocker
	toolLocks      keyedLocker
}

func NewEngine(source model.ReviewSource, llmClient llm.Client, retrievalEngine retrieval.Engine, profile config.Profile) *Engine {
	return &Engine{
		source:                 source,
		llm:                    llmClient,
		retrieval:              retrievalEngine,
		config:                 profile,
		searchToolOptimization: true,
		toolchainCapture:       toolchain.Capture,
	}
}

// SetToolchainCapture overrides the toolchain version detector. Intended for
// tests; production code uses the default manifest parsing capture.
func (e *Engine) SetToolchainCapture(fn func(ctx context.Context, repoRoot string, reviewCtx *model.ReviewContext) []model.ToolchainVersion) {
	e.toolchainCapture = fn
}

func (e *Engine) SetLogger(logger *logging.Logger) {
	e.logger = logger
}

func (e *Engine) SetSearchToolOptimization(enabled bool) {
	e.searchToolOptimization = enabled
}

// RunSpecPipeline executes an already-built pipeline. When the pipeline needs a
// source it resolves and trims the review context; otherwise (e.g. a
// merge/finalize-from-file workflow) it runs against a minimal synthetic context
// so no git/PR resolution is required. This is the single execution path for
// every review — the embedded DefaultSpec and any user-supplied spec alike.
func (e *Engine) RunSpecPipeline(ctx context.Context, p *Pipeline, req model.ReviewRequest) (*model.ReviewResult, *model.ReviewContext, error) {
	var reviewCtx *model.ReviewContext
	if p.NeedsSource() {
		c, err := e.resolveAndTrimContext(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		reviewCtx = c
		if reviewContextAllFiltered(reviewCtx) {
			result := &model.ReviewResult{
				Findings:               nil,
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "All changed files were omitted by filters.",
				OverallConfidenceScore: 1,
				Warnings:               []string{allChangedFilesFilteredWarning},
			}
			e.applyResultMetadata(result, req, reviewCtx)
			return result, reviewCtx, nil
		}
	} else {
		reviewCtx = &model.ReviewContext{Mode: req.Mode, CheckoutRoot: req.RepoRoot, Identifier: req.Identifier}
	}
	result, enrichedCtx, err := p.Run(ctx, reviewCtx, req)
	if err != nil {
		return nil, nil, err
	}
	e.applyResultMetadata(result, req, reviewCtx)
	return result, enrichedCtx, nil
}

// resolveAndTrimContext resolves the review source, captures toolchain versions,
// optionally inlines full files, and trims to the context budget.
func (e *Engine) resolveAndTrimContext(ctx context.Context, req model.ReviewRequest) (*model.ReviewContext, error) {
	e.logf(ctx, "Starting review: mode=%s repo=%s id=%d submode=%s repo_root=%s", req.Mode, req.Repo, req.Identifier, req.Submode, req.RepoRoot)
	contextFilter, err := newReviewContextFilter(req)
	if err != nil {
		return nil, err
	}
	reviewCtx, err := e.source.ResolveContext(ctx, req)
	if err != nil {
		return nil, err
	}
	e.logProgress(logging.StageReview, logging.StateStart, reviewContextSummary(reviewCtx, req))
	e.logf(ctx, "Resolved context: title=%q files=%d commits=%d comments=%d diff_bytes=%d", reviewCtx.Title, len(reviewCtx.ChangedFiles), len(reviewCtx.Commits), len(reviewCtx.Comments), len(reviewCtx.Diff))
	if len(reviewCtx.ChangedFiles) == 0 && len(reviewCtx.Diff) == 0 {
		return nil, ErrEmptyDiff
	}
	reviewCtx.CheckoutRoot = req.RepoRoot
	reviewCtx.Identifier = req.Identifier
	if allFiltered, err := e.applyReviewContextFilter(ctx, reviewCtx, req, contextFilter); err != nil {
		return nil, err
	} else if allFiltered {
		e.logf(ctx, "Filtered context: files=0 diff_bytes=0")
		return reviewCtx, nil
	}

	if e.toolchainCapture != nil {
		reviewCtx.ToolchainVersions = e.toolchainCapture(ctx, req.RepoRoot, reviewCtx)
		if len(reviewCtx.ToolchainVersions) > 0 {
			e.logf(ctx, "Captured toolchain versions: count=%d", len(reviewCtx.ToolchainVersions))
		}
	}

	if req.IncludeFullFiles && e.retrieval != nil && req.RepoRoot != "" {
		e.logf(ctx, "Including full files: count=%d", len(reviewCtx.ChangedFiles))
		for _, file := range reviewCtx.ChangedFiles {
			e.logf(ctx, "Retrieving file: path=%s", file.Path)
			content, err := e.retrieval.GetFile(ctx, req.RepoRoot, file.Path)
			if err != nil {
				e.logf(ctx, "Skipping file retrieval: path=%s error=%v", file.Path, err)
				continue
			}
			reviewCtx.SupplementalContext = append(reviewCtx.SupplementalContext, model.SupplementalFile{
				Path:     file.Path,
				Content:  content.Content,
				Language: content.Language,
				Kind:     "full_file",
			})
		}
	}

	trimmer := e.trimmer
	if trimmer == nil {
		trimmer = NewTrimmer(req.MaxContextTokens, model.SimpleEstimator{})
	}

	trimmed, err := trimmer.Trim(reviewCtx)
	if err != nil {
		return nil, fmt.Errorf("review: trim context: %w", err)
	}
	e.logf(ctx, "Trimmed context: files=%d supplemental=%d omitted=%d budget=%d", len(trimmed.ChangedFiles), len(trimmed.SupplementalContext), len(trimmed.OmittedSections), req.MaxContextTokens)
	return trimmed, nil
}

func (e *Engine) applyResultMetadata(result *model.ReviewResult, req model.ReviewRequest, reviewCtx *model.ReviewContext) {
	result.Mode = string(req.Mode)
	if req.Submode != "" {
		result.Mode = result.Mode + ":" + req.Submode
	}
	result.Model = e.config.Model
	result.Repo = req.Repo
	result.Identifier = req.Identifier
	result.BaseURL = e.config.BaseURL
	if reviewCtx != nil {
		result.BaseRef = reviewCtx.Repository.BaseRef
		result.HeadRef = reviewCtx.Repository.HeadRef
	}
}

func (e *Engine) reviewWithoutTools(ctx context.Context, llmReq *llm.ReviewRequest, agentRole string, systemTemplate string, messages []llm.Message, systemSnippet string, styleGuideToolchainSnippet string, maxOutputRetries int, sec *logging.ReasoningSection) (*llm.ReviewResponse, error) {
	finalMessages, err := noToolsMessages(agentRole, systemTemplate, messages, systemSnippet, styleGuideToolchainSnippet)
	if err != nil {
		return nil, err
	}
	llmReq.Messages = finalMessages
	llmReq.Tools = nil
	llmReq.ParallelToolCalls = false
	exampleSnippet := exampleSnippetFor(llmReq.SchemaKind)
	for attempt := 0; ; attempt++ {
		resp, err := e.loggedReview(ctx, llmReq, sec)
		if err == nil {
			if resp.ReasoningEffort != "" {
				llmReq.ReasoningEffort = resp.ReasoningEffort
			}
			return resp, nil
		}
		var invalidResp *llm.InvalidResponseError
		if !errors.As(err, &invalidResp) || !outputRetriesRemaining(attempt, maxOutputRetries) {
			return nil, err
		}
		if invalidResp.ReasoningEffort != "" {
			llmReq.ReasoningEffort = invalidResp.ReasoningEffort
		}
		e.logf(ctx, "Invalid JSON response in no-tools call, retrying: attempt=%d reason=%q missing=%v", attempt+1, invalidResp.Reason, invalidResp.MissingFields)
		e.logProgress(logging.StageModel, logging.StateRetry, fmt.Sprintf("invalid JSON, attempt=%d", attempt+1))
		if strings.TrimSpace(invalidResp.RawContent) != "" {
			llmReq.Messages = append(llmReq.Messages, llm.Message{Role: "assistant", Content: invalidResp.RawContent})
		}
		feedback, err := e.renderJSONRetryFeedback(invalidResp, exampleSnippet)
		if err != nil {
			return nil, err
		}
		llmReq.Messages = append(llmReq.Messages, llm.Message{Role: "user", Content: feedback})
	}
}

type agentSpec struct {
	name             string
	role             string
	system           string
	noToolsSystem    string
	user             string
	extraMessages    []llm.Message
	questionsSnippet string
	schema           []byte
	schemaKind       llm.SchemaKind
	constraints      llm.ResponseConstraints
	hasTools         bool
	// validateResponse returns the typed error so retry guidance metadata can
	// be rendered after otherwise valid JSON is parsed.
	validateResponse func(*llm.ReviewResponse) *llm.InvalidResponseError
}

type agentResult struct {
	resp               *llm.ReviewResponse
	run                model.AgentRun
	reasoningEffort    string
	contentMessages    []string
	toolMessages       []llm.Message
	toolCallHistory    []toolCallHistoryEntry
	duplicateToolCalls int
}

type contextAgentResult struct {
	run                model.AgentRun
	reasoningEffort    string
	contentMessages    []string
	toolMessages       []llm.Message
	toolCallHistory    []toolCallHistoryEntry
	duplicateToolCalls int
}

type reviewVector struct {
	// id is the stable, portable identifier used in workflow specs
	// (e.g. "security" for the "review:security" step). name is the display name.
	id            string
	name          string
	focusFile     string
	questionsFile string
	constraints   llm.ResponseConstraints
}

var reviewVectors = []reviewVector{
	{
		id:            "codequality",
		name:          "Code Quality",
		focusFile:     "agent_review_codequality_system_prompt.tmpl",
		questionsFile: "agent_review_codequality_questions.tmpl",
	},
	{
		id:            "security",
		name:          "Security",
		focusFile:     "agent_review_security_system_prompt.tmpl",
		questionsFile: "agent_review_security_questions.tmpl",
	},
	{
		id:            "architecture",
		name:          "Architecture",
		focusFile:     "agent_review_architecture_system_prompt.tmpl",
		questionsFile: "agent_review_architecture_questions.tmpl",
	},
	{
		id:            "performance",
		name:          "Performance",
		focusFile:     "agent_review_performance_system_prompt.tmpl",
		questionsFile: "agent_review_performance_questions.tmpl",
	},
	{
		id:            "testing",
		name:          "Testing",
		focusFile:     "agent_review_testing_system_prompt.tmpl",
		questionsFile: "agent_review_testing_questions.tmpl",
		constraints: llm.ResponseConstraints{
			MinPriority:        intPtr(2),
			AllowedCorrectness: []string{"patch is correct"},
		},
	},
	{
		id:            "bestpractices",
		name:          "Best Practices",
		focusFile:     "agent_review_bestpractices_system_prompt.tmpl",
		questionsFile: "agent_review_bestpractices_questions.tmpl",
	},
}

// reviewVectorByID returns the reviewVector with the given workflow id.
func reviewVectorByID(id string) (reviewVector, bool) {
	for _, v := range reviewVectors {
		if v.id == id {
			return v, true
		}
	}
	return reviewVector{}, false
}

func intPtr(v int) *int { return &v }

// withConfig returns a shallow copy of the engine whose profile is replaced.
// All reference fields (llm client, retrieval, logger, source, trimmer) are
// shared; only the value-type config differs. Used to apply per-step model
// parameter overrides without mutating the shared engine or racing concurrent
// steps, since the clone's config is read-only for the lifetime of a step.
func (e *Engine) withConfig(profile config.Profile) *Engine {
	clone := *e
	clone.config = profile
	return &clone
}

// verifyAndFilterVectorFindings runs the verifier on every finding from
// `vectorResults` and replaces each vector's `resp.Findings` in place with the
// subset that survives the drop policy. The mutation is intentional: the merge
// agent reads vectorResults downstream and must see only verified findings.
// Returns aggregated token usage, soft warnings, and any fatal error. On error,
// callers should still propagate the returned usage/warnings — they hold the
// partial-run telemetry up to the failure point.
func (e *Engine) verifyAndFilterVectorFindings(ctx context.Context, reviewCtx *model.ReviewContext, vectorResults []agentResult, req model.ReviewRequest) (model.TokenUsage, []string, error) {
	findings := make([]model.Finding, 0)
	type findingRef struct {
		vectorIdx  int
		findingIdx int
	}
	refs := make([]findingRef, 0)
	for vectorIdx, result := range vectorResults {
		if result.run.Status == model.AgentRunStatusFailed || result.resp == nil {
			continue
		}
		for findingIdx, finding := range result.resp.Findings {
			findings = append(findings, finding)
			refs = append(refs, findingRef{vectorIdx: vectorIdx, findingIdx: findingIdx})
		}
	}
	if len(findings) == 0 {
		return model.TokenUsage{}, nil, nil
	}
	if overwrote := model.EnsureFindingIDs(findings); overwrote > 0 {
		e.logf(ctx, "Review generated replacement IDs before verification: count=%d", overwrote)
	}
	opts := verifyOptionsFromReviewRequest(req)
	verifications, usage, warnings, err := e.VerifyAll(ctx, reviewCtx, findings, opts)
	if err != nil {
		return usage, warnings, err
	}
	if len(verifications) != len(refs) {
		return usage, warnings, fmt.Errorf("review: verifier returned %d results for %d findings", len(verifications), len(refs))
	}
	type dropCounts struct {
		refuted         int
		unverified      int
		belowConfidence int
	}
	keptByVector := make(map[int][]model.Finding, len(vectorResults))
	droppedIdxByVector := make(map[int]map[int]struct{}, len(vectorResults))
	dropsByVector := make(map[int]dropCounts, len(vectorResults))
	for i, verification := range verifications {
		ref := refs[i]
		if verification == nil {
			return usage, warnings, fmt.Errorf("review: verifier returned no result for finding #%d %q", i+1, findings[i].Title)
		}
		finding := vectorResults[ref.vectorIdx].resp.Findings[ref.findingIdx]
		// findings[i].ID holds the normalized ID after EnsureFindingIDs above,
		// which may have replaced an invalid or duplicate reviewer ID. Always
		// adopt it so corrected IDs survive into downstream dedupe/merge
		// validation and stay in sync with Verification.ID.
		finding.ID = findings[i].ID
		v := *verification
		model.EnsureVerificationID(&v, finding.ID)
		finding.Verification = &v
		drop, reason := shouldDropFinding(&v, opts.DropPolicy, opts.DropConfidence)
		if drop {
			if droppedIdxByVector[ref.vectorIdx] == nil {
				droppedIdxByVector[ref.vectorIdx] = make(map[int]struct{})
			}
			droppedIdxByVector[ref.vectorIdx][ref.findingIdx] = struct{}{}
			counts := dropsByVector[ref.vectorIdx]
			switch reason {
			case "refuted":
				counts.refuted++
			case "unverified":
				counts.unverified++
			}
			dropsByVector[ref.vectorIdx] = counts
			continue
		}
		if reason == "below_confidence" {
			counts := dropsByVector[ref.vectorIdx]
			counts.belowConfidence++
			dropsByVector[ref.vectorIdx] = counts
		}
		keptByVector[ref.vectorIdx] = append(keptByVector[ref.vectorIdx], finding)
	}
	for vectorIdx := range vectorResults {
		if vectorResults[vectorIdx].run.Status == model.AgentRunStatusFailed || vectorResults[vectorIdx].resp == nil {
			continue
		}
		if len(vectorResults[vectorIdx].resp.Findings) == 0 {
			continue
		}
		vectorResults[vectorIdx].resp.Findings = keptByVector[vectorIdx]
		dropped := len(droppedIdxByVector[vectorIdx])
		counts := dropsByVector[vectorIdx]
		if dropped > 0 || counts.belowConfidence > 0 {
			e.logf(ctx, "Verifier filter before merge: reviewer=%s dropped=%d refuted=%d unverified=%d below_confidence_kept=%d kept=%d policy=%s threshold=%.2f",
				vectorResults[vectorIdx].run.Name,
				dropped,
				counts.refuted,
				counts.unverified,
				counts.belowConfidence,
				len(keptByVector[vectorIdx]),
				normalizeDropPolicy(opts.DropPolicy),
				opts.DropConfidence,
			)
		}
	}
	return usage, warnings, nil
}

// shouldDropFinding returns whether the verifier's verdict is severe enough to
// drop a finding before merge, plus a label describing why (or why not).
//
// Labels:
//   - "refuted" / "unverified": verdict reason for dropping
//   - "below_confidence": verdict would drop but confidence_score is under the floor; kept
//   - "kept": verdict does not warrant dropping (e.g. "confirmed", or policy="none")
func shouldDropFinding(v *model.FindingVerification, policy string, threshold float64) (bool, string) {
	if v == nil {
		return false, "kept"
	}
	policy = normalizeDropPolicy(policy)
	if policy == DropPolicyNone {
		return false, "kept"
	}
	verdict := v.Verdict
	if verdict == "" {
		// Treat missing verdict as unverified so we never drop on schema gaps.
		verdict = model.VerdictUnverified
	}
	switch policy {
	case DropPolicyRefutedOnly:
		if verdict != model.VerdictRefuted {
			return false, "kept"
		}
	case DropPolicyRefutedAndUnverified:
		if verdict == model.VerdictConfirmed {
			return false, "kept"
		}
	default:
		return false, "kept"
	}
	if v.ConfidenceScore < threshold {
		return false, "below_confidence"
	}
	return true, verdict
}

// Drop policies for --verify-drop-policy. Defined as constants so the accepted
// set, the normalization fallback, and the drop decision all reference the same
// values and cannot drift.
const (
	DropPolicyNone                 = "none"
	DropPolicyRefutedOnly          = "refuted-only"
	DropPolicyRefutedAndUnverified = "refuted-and-unverified"
)

// Default tool-call arguments. defaultCallHierarchyDepth in particular must be
// used both where the find_callers/find_callees dedup key is computed and where
// the call is executed, otherwise the key would not match the executed depth.
const (
	defaultCallHierarchyDepth = 10
	defaultSearchContextLines = 5
)

// ValidDropPolicies lists the accepted values for --verify-drop-policy.
var ValidDropPolicies = []string{DropPolicyNone, DropPolicyRefutedOnly, DropPolicyRefutedAndUnverified}

// ValidateDropPolicy returns an error when policy is not one of the supported values.
func ValidateDropPolicy(policy string) error {
	if slices.Contains(ValidDropPolicies, policy) {
		return nil
	}
	return fmt.Errorf("invalid verify-drop-policy %q (allowed: %s)", policy, strings.Join(ValidDropPolicies, ", "))
}

func normalizeDropPolicy(policy string) string {
	switch policy {
	case DropPolicyNone, DropPolicyRefutedOnly, DropPolicyRefutedAndUnverified:
		return policy
	default:
		return DropPolicyRefutedOnly
	}
}

func verifyOptionsFromReviewRequest(req model.ReviewRequest) VerifyOptions {
	return VerifyOptions{
		Concurrency:              req.VerifyConcurrency,
		UseJSONSchema:            req.UseJSONSchema,
		MaxToolCalls:             req.MaxToolCalls,
		MaxDuplicateToolCalls:    req.MaxDuplicateToolCalls,
		MaxOutputRetries:         req.MaxOutputRetries,
		MaxReasoningSeconds:      req.MaxReasoningSeconds,
		MaxReasoningLoopRepeats:  req.MaxReasoningLoopRepeats,
		DisableParallelToolCalls: req.DisableParallelToolCalls,
		RepoRoot:                 req.RepoRoot,
		DropPolicy:               req.VerifyDropPolicy,
		DropConfidence:           req.VerifyDropConfidence,
	}
}

// allVectorsFailed reports whether every per-vector reviewer returned a
// failed status. Used to short-circuit the merge LLM call.
func allVectorsFailed(results []agentResult) bool {
	if len(results) == 0 {
		return false
	}
	for _, r := range results {
		if r.run.Status != model.AgentRunStatusFailed {
			return false
		}
	}
	return true
}

// appendAgentRunWarnings folds AgentRun-level failures into the top-level
// warnings list. Failures already surfaced via contextErr above are skipped to
// avoid duplicates.
func appendAgentRunWarnings(warnings []string, runs []model.AgentRun, contextErr error) []string {
	for _, run := range runs {
		if run.Status == model.AgentRunStatusOK {
			continue
		}
		if run.Role == "context" && contextErr != nil {
			continue
		}
		actor := "reviewer"
		if run.Role == "merge" {
			actor = "merge step"
		}
		switch run.Status {
		case model.AgentRunStatusFailed:
			warnings = append(warnings, fmt.Sprintf("%s %s failed: %s", run.Name, actor, run.Error))
		case model.AgentRunStatusPartial:
			warnings = append(warnings, fmt.Sprintf("%s %s partial result: %s", run.Name, actor, run.Error))
		}
	}
	return warnings
}

type pairwiseMergeInput struct {
	name     string
	role     string
	index    int
	response *llm.ReviewResponse
}

func mergeSchemaForDedupe(req model.ReviewRequest) []byte {
	if !req.UseJSONSchema {
		return nil
	}
	constraints := mergeConstraintsForRequest(req)
	if hasResponseConstraints(constraints) {
		return llm.MergeSchemaWithConstraints(constraints)
	}
	return llm.MergeSchema
}

func mergeConstraintsForDedupe(req model.ReviewRequest) llm.ResponseConstraints {
	if !req.UseJSONSchema {
		return llm.ResponseConstraints{}
	}
	return mergeConstraintsForRequest(req)
}

// runDedupeAgents runs a per-reviewer dedupe pass concurrently. It intentionally
// mutates vectorResults[idx].resp in place, but only when a dedupe agent returns
// a valid response. A failed or invalid dedupe leaves that reviewer's original
// findings intact and only records the dedupe run for telemetry.
func (e *Engine) runDedupeAgents(ctx context.Context, contextNotes string, vectorResults []agentResult, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest) []model.AgentRun {
	runs := make([]model.AgentRun, len(vectorResults))
	var wg sync.WaitGroup
	for i := range vectorResults {
		result := vectorResults[i]
		if result.run.Status == model.AgentRunStatusFailed || result.resp == nil || len(result.resp.Findings) < 2 {
			continue
		}
		wg.Add(1)
		go func(idx int, input agentResult) {
			defer wg.Done()
			resp, run := e.runDedupeAgent(ctx, contextNotes, input, schema, constraints, req)
			runs[idx] = run
			if resp != nil {
				vectorResults[idx].resp = resp
			}
		}(i, result)
	}
	wg.Wait()

	out := make([]model.AgentRun, 0, len(runs))
	for _, run := range runs {
		if run.Name == "" {
			continue
		}
		out = append(out, run)
	}
	return out
}

func (e *Engine) runDedupeAgent(ctx context.Context, contextNotes string, input agentResult, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest) (*llm.ReviewResponse, model.AgentRun) {
	result, err := e.callDedupeAgent(ctx, contextNotes, input, schema, constraints, req)
	run := result.run
	if err != nil {
		run = markDedupeRun(run, model.AgentRunStatusFailed, err)
		return nil, run
	}
	if result.resp == nil {
		err := fmt.Errorf("dedupe agent returned no response")
		run = markDedupeRun(run, model.AgentRunStatusFailed, err)
		return nil, run
	}
	if invalid := validateDedupeResponse(result.resp, input.resp); invalid != nil {
		run = markDedupeRun(run, model.AgentRunStatusPartial, invalid)
		return nil, run
	}
	return cloneReviewResponse(result.resp), run
}

func (e *Engine) callDedupeAgent(ctx context.Context, contextNotes string, input agentResult, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest) (agentResult, error) {
	systemTemplate, err := e.loadPrompt("agent_dedupe_system_prompt.tmpl")
	if err != nil {
		return agentResult{}, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("dedupe", mergeOutputSchemaSnippetFor(req.UseJSONSchema))
	if err != nil {
		return agentResult{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		FindingInstructionsSnippet string
		PrioritySnippet            string
		OutputFormatSnippet        string
	}{
		FindingInstructionsSnippet: commonSnippets.findingInstructions,
		PrioritySnippet:            commonSnippets.priority,
		OutputFormatSnippet:        commonSnippets.outputFormat,
	})
	if err != nil {
		return agentResult{}, fmt.Errorf("review: rendering dedupe system prompt: %w", err)
	}
	dedupeUser, err := llm.RenderJSON(map[string]any{
		"context_agent_notes": contextNotes,
		"review_findings": map[string]any{
			"name":                     input.run.Name,
			"role":                     input.run.Role,
			"findings":                 input.resp.Findings,
			"overall_correctness":      input.resp.OverallCorrectness,
			"overall_explanation":      input.resp.OverallExplanation,
			"overall_confidence_score": input.resp.OverallConfidenceScore,
		},
	})
	if err != nil {
		return agentResult{}, fmt.Errorf("review: rendering dedupe prompt json: %w", err)
	}
	return e.runAgent(ctx, agentSpec{
		name:          "Dedupe Findings",
		role:          "dedupe",
		system:        system,
		noToolsSystem: system,
		user:          dedupeUser,
		schema:        schema,
		schemaKind:    llm.SchemaKindMerge,
		constraints:   constraints,
		hasTools:      false,
		validateResponse: func(resp *llm.ReviewResponse) *llm.InvalidResponseError {
			return validateDedupeResponse(resp, input.resp)
		},
	}, req)
}

func markDedupeRun(run model.AgentRun, status string, err error) model.AgentRun {
	if run.Name == "" {
		run.Name = "Dedupe Findings"
	}
	if run.Role == "" {
		run.Role = "dedupe"
	}
	if run.Status == "" || run.Status == model.AgentRunStatusOK {
		run.Status = status
	}
	if run.Error == "" && err != nil {
		run.Error = err.Error()
	}
	return run
}

func (e *Engine) runPairwiseMergeAgents(ctx context.Context, userPrompt string, contextNotes string, inputs []pairwiseMergeInput, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest) (agentResult, []model.AgentRun) {
	if len(inputs) == 0 {
		result := emptyVerifiedMergeResult()
		return result, []model.AgentRun{result.run}
	}
	if len(inputs) == 1 {
		result := agentResult{
			resp: cloneReviewResponse(inputs[0].response),
			run: model.AgentRun{
				Name:   "Merge Findings",
				Role:   "merge",
				Status: model.AgentRunStatusSkipped,
			},
		}
		return result, []model.AgentRun{result.run}
	}

	accumulator := cloneReviewResponse(inputs[0].response)
	var runs []model.AgentRun
	var last agentResult
	for i := 1; i < len(inputs); i++ {
		stepResult, err := e.runMergeAgent(ctx, userPrompt, contextNotes, accumulator, inputs[i], schema, constraints, req)
		run := stepResult.run
		if err != nil {
			run = markMergeRun(run, model.AgentRunStatusFailed, err)
			accumulator = fallbackPairwiseMerge(accumulator, inputs[i], err)
			last = agentResult{resp: accumulator, run: run, reasoningEffort: stepResult.reasoningEffort}
			runs = append(runs, run)
			continue
		}
		if stepResult.resp == nil {
			err := fmt.Errorf("merge step returned no response")
			run = markMergeRun(run, model.AgentRunStatusFailed, err)
			accumulator = fallbackPairwiseMerge(accumulator, inputs[i], err)
			last = agentResult{resp: accumulator, run: run, reasoningEffort: stepResult.reasoningEffort}
			runs = append(runs, run)
			continue
		}
		if invalid := validatePairwiseMergeResponse(stepResult.resp, accumulator, inputs[i]); invalid != nil {
			run = markMergeRun(run, model.AgentRunStatusPartial, invalid)
			accumulator = fallbackPairwiseMerge(accumulator, inputs[i], invalid)
			last = agentResult{resp: accumulator, run: run, reasoningEffort: stepResult.reasoningEffort}
			runs = append(runs, run)
			continue
		}
		accumulator = cloneReviewResponse(stepResult.resp)
		last = agentResult{resp: accumulator, run: run, reasoningEffort: stepResult.reasoningEffort}
		runs = append(runs, run)
	}
	return last, runs
}

func markMergeRun(run model.AgentRun, status string, err error) model.AgentRun {
	if run.Name == "" {
		run.Name = "Merge Findings"
	}
	if run.Role == "" {
		run.Role = "merge"
	}
	if run.Status == "" || run.Status == model.AgentRunStatusOK {
		run.Status = status
	}
	if run.Error == "" && err != nil {
		run.Error = err.Error()
	}
	return run
}

func fallbackPairwiseMerge(finalFindings *llm.ReviewResponse, incoming pairwiseMergeInput, mergeFailure error) *llm.ReviewResponse {
	out := cloneReviewResponse(finalFindings)
	if out == nil {
		out = &llm.ReviewResponse{}
	}
	if incoming.response != nil {
		clonedIncoming := cloneReviewResponse(incoming.response)
		out.Findings = append(out.Findings, clonedIncoming.Findings...)
		remintDuplicateFindingIDs(out.Findings)
	}
	out.OverallCorrectness = "patch is incorrect"
	out.OverallExplanation = fmt.Sprintf("Merge step unavailable; kept Final findings and appended %s findings unchanged.", incoming.name)
	if mergeFailure != nil {
		out.OverallExplanation += " Error: " + mergeFailure.Error()
	}
	out.OverallConfidenceScore = fallbackMergeConfidence(finalFindings)
	return out
}

func fallbackMergeConfidence(finalFindings *llm.ReviewResponse) float64 {
	if finalFindings == nil || finalFindings.OverallConfidenceScore <= 0 {
		return 0
	}
	if finalFindings.OverallConfidenceScore < 0.3 {
		return finalFindings.OverallConfidenceScore
	}
	return 0.3
}

// remintDuplicateFindingIDs replaces colliding valid UUIDs in-place so that
// downstream verification repair can address each finding by ID.
func remintDuplicateFindingIDs(findings []model.Finding) {
	seen := make(map[string]struct{}, len(findings))
	for i := range findings {
		if findings[i].ID == "" {
			continue
		}
		if _, ok := seen[findings[i].ID]; ok {
			findings[i].ID = ""
			model.EnsureFindingID(&findings[i])
		}
		seen[findings[i].ID] = struct{}{}
	}
}

func pairwiseMergeInputs(vectorResults []agentResult) []pairwiseMergeInput {
	inputs := make([]pairwiseMergeInput, 0, len(vectorResults))
	for i, result := range vectorResults {
		if result.run.Status == model.AgentRunStatusFailed || result.resp == nil || len(result.resp.Findings) == 0 {
			continue
		}
		inputs = append(inputs, pairwiseMergeInput{
			name:     result.run.Name,
			role:     result.run.Role,
			index:    i,
			response: result.resp,
		})
	}
	sort.SliceStable(inputs, func(i, j int) bool {
		left := len(inputs[i].response.Findings)
		right := len(inputs[j].response.Findings)
		if left != right {
			return left > right
		}
		return inputs[i].index < inputs[j].index
	})
	return inputs
}

func flattenPairwiseMergeInputs(inputs []pairwiseMergeInput) []model.Finding {
	var findings []model.Finding
	for _, input := range inputs {
		if input.response == nil {
			continue
		}
		findings = append(findings, input.response.Findings...)
	}
	return findings
}

func cloneReviewResponse(resp *llm.ReviewResponse) *llm.ReviewResponse {
	if resp == nil {
		return nil
	}
	clone := *resp
	clone.Findings = make([]model.Finding, len(resp.Findings))
	for i, finding := range resp.Findings {
		clone.Findings[i] = finding
		if finding.Priority != nil {
			priority := *finding.Priority
			clone.Findings[i].Priority = &priority
		}
		if len(finding.Suggestions) > 0 {
			clone.Findings[i].Suggestions = append([]model.Suggestion(nil), finding.Suggestions...)
		}
		if finding.Verification != nil {
			verification := *finding.Verification
			clone.Findings[i].Verification = &verification
		}
		if finding.Finalization != nil {
			finalization := *finding.Finalization
			clone.Findings[i].Finalization = &finalization
		}
	}
	return &clone
}

func (e *Engine) runMergeAgent(ctx context.Context, userPrompt string, contextNotes string, finalFindings *llm.ReviewResponse, incoming pairwiseMergeInput, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest) (agentResult, error) {
	systemTemplate, err := e.loadPrompt("agent_merge_system_prompt.tmpl")
	if err != nil {
		return agentResult{}, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("merge", mergeOutputSchemaSnippetFor(req.UseJSONSchema))
	if err != nil {
		return agentResult{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		FindingInstructionsSnippet string
		PrioritySnippet            string
		OutputFormatSnippet        string
	}{
		FindingInstructionsSnippet: commonSnippets.findingInstructions,
		PrioritySnippet:            commonSnippets.priority,
		OutputFormatSnippet:        commonSnippets.outputFormat,
	})
	if err != nil {
		return agentResult{}, fmt.Errorf("review: rendering merge system prompt: %w", err)
	}
	mergeUser, err := llm.RenderJSON(map[string]any{
		"review_context":      json.RawMessage(userPrompt),
		"context_agent_notes": contextNotes,
		"final_findings":      finalFindingsPayload(finalFindings),
		"incoming_review":     pairwiseMergePayload(incoming),
	})
	if err != nil {
		return agentResult{}, fmt.Errorf("review: rendering merge prompt json: %w", err)
	}
	return e.runAgent(ctx, agentSpec{
		name:          "Merge Findings",
		role:          "merge",
		system:        system,
		noToolsSystem: system,
		user:          mergeUser,
		schema:        schema,
		schemaKind:    llm.SchemaKindMerge,
		constraints:   constraints,
		hasTools:      false,
		validateResponse: func(resp *llm.ReviewResponse) *llm.InvalidResponseError {
			return validatePairwiseMergeResponse(resp, finalFindings, incoming)
		},
	}, req)
}

func finalFindingsPayload(resp *llm.ReviewResponse) map[string]any {
	entry := map[string]any{
		"name": "Final findings",
		"role": "merge_accumulator",
	}
	if resp != nil {
		entry["findings"] = resp.Findings
		entry["overall_correctness"] = resp.OverallCorrectness
		entry["overall_explanation"] = resp.OverallExplanation
		entry["overall_confidence_score"] = resp.OverallConfidenceScore
	}
	return entry
}

func pairwiseMergePayload(input pairwiseMergeInput) map[string]any {
	entry := map[string]any{
		"name": input.name,
		"role": input.role,
	}
	if input.response != nil {
		entry["findings"] = input.response.Findings
		entry["overall_correctness"] = input.response.OverallCorrectness
		entry["overall_explanation"] = input.response.OverallExplanation
		entry["overall_confidence_score"] = input.response.OverallConfidenceScore
	}
	return entry
}

func validateDedupeResponse(resp *llm.ReviewResponse, input *llm.ReviewResponse) *llm.InvalidResponseError {
	if resp == nil {
		return &llm.InvalidResponseError{
			Reason:        "dedupe returned no response",
			MissingFields: []string{"findings"},
		}
	}
	inputCount := 0
	inputIDs := map[string]struct{}{}
	if input != nil {
		inputCount = len(input.Findings)
		for _, finding := range input.Findings {
			id := strings.TrimSpace(finding.ID)
			if id != "" {
				inputIDs[id] = struct{}{}
			}
		}
	}
	minCount := dedupeMinCount(inputCount)
	countTooLow := len(resp.Findings) < minCount
	countTooHigh := len(resp.Findings) > inputCount
	unknownIDs := 0
	duplicateIDs := 0
	verificationMismatch := 0
	seen := map[string]struct{}{}
	for _, finding := range resp.Findings {
		id := strings.TrimSpace(finding.ID)
		if id == "" {
			unknownIDs++
		} else {
			if _, ok := inputIDs[id]; !ok {
				unknownIDs++
			}
			if _, ok := seen[id]; ok {
				duplicateIDs++
			}
			seen[id] = struct{}{}
		}
		if finding.Verification == nil || strings.TrimSpace(finding.Verification.ID) != id {
			verificationMismatch++
		}
	}
	if !countTooLow && !countTooHigh && unknownIDs == 0 && duplicateIDs == 0 && verificationMismatch == 0 {
		return nil
	}
	var problems []string
	if countTooLow {
		problems = append(problems, fmt.Sprintf("count_too_low got=%d min=%d input=%d", len(resp.Findings), minCount, inputCount))
	}
	if countTooHigh {
		problems = append(problems, fmt.Sprintf("count_too_high got=%d input=%d", len(resp.Findings), inputCount))
	}
	if unknownIDs > 0 {
		problems = append(problems, fmt.Sprintf("unknown_ids count=%d", unknownIDs))
	}
	if duplicateIDs > 0 {
		problems = append(problems, fmt.Sprintf("duplicate_ids count=%d", duplicateIDs))
	}
	if verificationMismatch > 0 {
		problems = append(problems, fmt.Sprintf("verification_mismatch count=%d", verificationMismatch))
	}
	return &llm.InvalidResponseError{
		RawContent:            resp.RawResponse,
		Reason:                "dedupe_validation_failed: " + strings.Join(problems, "; "),
		MissingFields:         []string{"findings"},
		ReasoningEffort:       resp.ReasoningEffort,
		RetryGuidanceTemplate: "dedupe_validation_retry_guidance.tmpl",
		RetryGuidanceData: struct {
			CountTooLow          bool
			CountTooHigh         bool
			InputCount           int
			MinCount             int
			UnknownIDs           int
			DuplicateIDs         int
			VerificationMismatch int
		}{
			CountTooLow:          countTooLow,
			CountTooHigh:         countTooHigh,
			InputCount:           inputCount,
			MinCount:             minCount,
			UnknownIDs:           unknownIDs,
			DuplicateIDs:         duplicateIDs,
			VerificationMismatch: verificationMismatch,
		},
	}
}

func dedupeMinCount(inputCount int) int {
	if inputCount <= 0 {
		return 0
	}
	if inputCount <= 3 {
		return 1
	}
	return (inputCount + 1) / 2
}

func validatePairwiseMergeResponse(resp *llm.ReviewResponse, finalFindings *llm.ReviewResponse, incoming pairwiseMergeInput) *llm.InvalidResponseError {
	var inputFindings []model.Finding
	minCount := 0
	if finalFindings != nil {
		// The protected count is the current accumulator size, not the
		// original reviewer size. This floor intentionally grows after each
		// accepted step because pairwise merge must keep existing Final
		// findings and only merge the incoming reviewer into them.
		minCount = len(finalFindings.Findings)
		inputFindings = append(inputFindings, finalFindings.Findings...)
	}
	var incomingFindings []model.Finding
	if incoming.response != nil {
		incomingFindings = incoming.response.Findings
		inputFindings = append(inputFindings, incomingFindings...)
	}
	return validateMergeResponse(resp, mergeValidationInputs{
		minCount:         minCount,
		protectedName:    "Final findings",
		inputFindings:    inputFindings,
		accumulator:      finalFindings,
		incomingFindings: incomingFindings,
	})
}

type mergeValidationInputs struct {
	minCount      int
	protectedName string
	inputFindings []model.Finding
	// Pairwise-only. accumulator and incomingFindings must both be set to enable
	// the merge_mismatch check that detects responses where count did not grow
	// enough to absorb the incoming reviewer and none of the accumulator findings
	// were modified — i.e. the merge step silently dropped the incoming
	// reviewer's contribution.
	accumulator      *llm.ReviewResponse
	incomingFindings []model.Finding
}

func validateMergeResponse(resp *llm.ReviewResponse, in mergeValidationInputs) *llm.InvalidResponseError {
	if resp == nil {
		return &llm.InvalidResponseError{
			Reason:        "merge returned no response",
			MissingFields: []string{"findings"},
		}
	}
	var problems []string
	countMismatch := len(resp.Findings) < in.minCount
	if countMismatch {
		problems = append(problems, fmt.Sprintf("count_mismatch got=%d min=%d", len(resp.Findings), in.minCount))
	}
	unmatched := 0
	for i, finding := range resp.Findings {
		if findMergeInputMatch(finding, in.inputFindings) == nil {
			unmatched++
			problems = append(problems, fmt.Sprintf("unmatched_finding index=%d", i))
		}
	}
	mergeMismatch := false
	changed, required, growth, incomingCount := 0, 0, 0, len(in.incomingFindings)
	if in.accumulator != nil && incomingCount > 0 {
		finalCount := len(in.accumulator.Findings)
		growth = max(len(resp.Findings)-finalCount, 0)
		required = incomingCount - growth
		if required > 0 {
			changed = pairwiseMergeChangedCount(resp.Findings, in.accumulator.Findings)
			if changed < required {
				mergeMismatch = true
				problems = append(problems, fmt.Sprintf("merge_mismatch changed=%d required=%d incoming=%d growth=%d", changed, required, incomingCount, growth))
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return &llm.InvalidResponseError{
		RawContent:            resp.RawResponse,
		Reason:                "merge_validation_failed: " + strings.Join(problems, "; "),
		MissingFields:         []string{"findings"},
		ReasoningEffort:       resp.ReasoningEffort,
		RetryGuidanceTemplate: "merge_validation_retry_guidance.tmpl",
		RetryGuidanceData: struct {
			CountMismatch bool
			GotCount      int
			MinCount      int
			Unmatched     int
			ProtectedName string
			MergeMismatch bool
			Changed       int
			Required      int
			IncomingCount int
			Growth        int
		}{
			CountMismatch: countMismatch,
			GotCount:      len(resp.Findings),
			MinCount:      in.minCount,
			Unmatched:     unmatched,
			ProtectedName: in.protectedName,
			MergeMismatch: mergeMismatch,
			Changed:       changed,
			Required:      required,
			IncomingCount: incomingCount,
			Growth:        growth,
		},
	}
}

// pairwiseMergeChangedCount counts how many accumulator findings the merge
// output failed to carry through unchanged. It iterates the accumulator (the
// protected side), not the output, so it stays correct when a valid merge folds
// an accumulator finding into the incoming reviewer's finding and keeps the
// incoming ID: such a merge is invisible to an output-keyed scan, but here it
// still registers because the accumulator finding either drops out of the
// output entirely or surfaces as a materially different output finding.
//
// Each accumulator finding is attributed to an output finding via
// findMergeInputMatch (ID first, then code_location with a title tiebreak),
// which handles both the production path (parser mints UUIDs, so IDs match
// across steps) and degenerate cases where IDs are missing (location/title
// fallback still attributes correctly). An accumulator finding counts as
// changed when no output finding matches it, or when the matched output finding
// differs materially.
func pairwiseMergeChangedCount(out, accumulator []model.Finding) int {
	changed := 0
	for i := range accumulator {
		match := findMergeInputMatch(accumulator[i], out)
		if match == nil {
			changed++
			continue
		}
		if !findingMaterialEqual(accumulator[i], *match) {
			changed++
		}
	}
	return changed
}

// findingMaterialEqual compares the parts of a finding that a merge step would
// rewrite when folding in new information. Finding ID and Verification.ID are
// intentionally ignored: a merge that only reassigns an ID without touching
// content is not a merge, and Verification.ID merely mirrors the parent finding
// ID, which may legitimately change when a finding is folded into another.
func findingMaterialEqual(a, b model.Finding) bool {
	if a.Title != b.Title || a.Body != b.Body {
		return false
	}
	if a.ConfidenceScore != b.ConfidenceScore {
		return false
	}
	if model.PriorityRank(a.Priority) != model.PriorityRank(b.Priority) {
		return false
	}
	if a.CodeLocation != b.CodeLocation {
		return false
	}
	if len(a.Suggestions) != 0 || len(b.Suggestions) != 0 {
		if !reflect.DeepEqual(a.Suggestions, b.Suggestions) {
			return false
		}
	}
	if !verificationContentEqual(a.Verification, b.Verification) {
		return false
	}
	if !reflect.DeepEqual(a.Finalization, b.Finalization) {
		return false
	}
	return true
}

// verificationContentEqual compares only the content fields of two
// verifications, ignoring Verification.ID. That ID mirrors the parent finding
// ID and may legitimately change during a merge, so it must not make two
// otherwise-identical verifications compare unequal.
func verificationContentEqual(a, b *model.FindingVerification) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Verdict == b.Verdict &&
		a.Priority == b.Priority &&
		a.ConfidenceScore == b.ConfidenceScore &&
		a.Remarks == b.Remarks
}

func findMergeInputMatch(target model.Finding, in []model.Finding) *model.Finding {
	id := strings.TrimSpace(target.ID)
	if id != "" {
		for i := range in {
			if in[i].ID == id {
				return &in[i]
			}
		}
	}
	return findInputMatch(target, in)
}

func (e *Engine) runContextAgent(ctx context.Context, agent agentSpec, req model.ReviewRequest) (contextAgentResult, error) {
	result, err := e.runAgent(ctx, agent, req)
	// Always project whatever runAgent returned (even on err) so callers
	// preserve accumulated tokens, tool calls, and partial content for
	// telemetry / degraded-fallback flows.
	return contextAgentResult{
		run:                result.run,
		reasoningEffort:    result.reasoningEffort,
		contentMessages:    result.contentMessages,
		toolMessages:       result.toolMessages,
		toolCallHistory:    result.toolCallHistory,
		duplicateToolCalls: result.duplicateToolCalls,
	}, err
}

func (e *Engine) renderContextSystem(template string, req model.ReviewRequest) (string, error) {
	toolInstructions, err := e.renderToolInstructions(toolInstructionsConfig{
		agentRole:                "context",
		parallelToolCallGuidance: !req.DisableParallelToolCalls,
	})
	if err != nil {
		return "", err
	}
	systemPrompt, err := llm.RenderPrompt(template, struct {
		ToolInstructions string
	}{
		ToolInstructions: toolInstructions,
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering context system prompt: %w", err)
	}
	return systemPrompt, nil
}

// runAgent executes one agent. Reviewer agents run their full initial pass plus
// any nudge rounds through a shared reviewerSession (see reviewer_session.go);
// every other role is a single-turn agent loop. The reviewer session machinery
// is the single implementation shared with the spec-driven standalone
// nudge/reasoning-extract steps — there is no parallel reviewer code path.
func (e *Engine) runAgent(ctx context.Context, agent agentSpec, req model.ReviewRequest) (agentResult, error) {
	if agent.role == "review" {
		s := e.newReviewerSession(agent, req, false)
		if err := e.reviewerInitial(ctx, s, req); err != nil {
			return s.partialResult(req), err
		}
		if err := e.reviewerNudges(ctx, s, req); err != nil {
			return agentResult{}, err
		}
		return s.result(req), nil
	}

	loopReq, sec := e.buildAgentLoopRequest(agent, req)
	defer sec.End()
	loopResult, err := e.runAgentLoop(ctx, loopReq)
	if err != nil {
		return partialAgentResult(agent, req, loopResult), err
	}
	if loopResult.resp == nil {
		return partialAgentResult(agent, req, loopResult), fmt.Errorf("agent %s returned no response", agent.name)
	}
	return agentResult{
		resp:               loopResult.resp,
		reasoningEffort:    loopResult.reasoningEffort,
		contentMessages:    loopResult.contentMessages,
		toolMessages:       loopResult.toolMessages,
		toolCallHistory:    loopResult.toolCallHistory,
		duplicateToolCalls: loopResult.duplicateToolCalls,
		run: model.AgentRun{
			Name:                  agent.name,
			Role:                  agent.role,
			Findings:              len(loopResult.resp.Findings),
			MaxToolCalls:          req.MaxToolCalls,
			MaxDuplicateToolCalls: req.MaxDuplicateToolCalls,
			ToolCalls:             loopResult.toolCalls,
			DuplicateToolCalls:    loopResult.duplicateToolCalls,
			TokensUsed:            loopResult.tokensUsed,
		},
	}, nil
}

func (e *Engine) runReasoningCollectFindings(ctx context.Context, reasoning, parentName string, _ int, req model.ReviewRequest) (string, agentResult, error) {
	name := fmt.Sprintf("Mine Reasoning of %s", parentName)
	system, err := renderPromptFile("agent_reasoning_collect_findings_system_prompt.tmpl", nil)
	if err != nil {
		return "", agentResult{}, err
	}
	user, err := renderPromptFile("agent_reasoning_collect_findings_user_message.tmpl", struct {
		ReasoningContent string
	}{
		ReasoningContent: reasoning,
	})
	if err != nil {
		return "", agentResult{}, err
	}
	result, err := e.runAgent(ctx, agentSpec{
		name:       name,
		role:       "extract",
		system:     system,
		user:       user,
		schemaKind: llm.SchemaKindText,
		hasTools:   false,
	}, reasoningExtractRequest(req))
	out := reasoningExtractOutput(result.contentMessages)
	if err == nil {
		extractCtx := logging.WithProgressInfo(ctx, e.progressInfo("extract", name, ""))
		if out != "" {
			e.logBlock(extractCtx, "Extracted reasoning findings:", out)
		} else {
			e.logf(extractCtx, "No reasoning findings extracted")
		}
	}
	return out, result, err
}

func (e *Engine) runReasoningUpdateFindings(ctx context.Context, combinedList, findingsJSON, parentName string, req model.ReviewRequest) (string, agentResult, error) {
	system, err := renderPromptFile("agent_reasoning_update_findings_system_prompt.tmpl", nil)
	if err != nil {
		return "", agentResult{}, err
	}
	user, err := renderPromptFile("agent_reasoning_update_findings_user_message.tmpl", struct {
		FullList     string
		FindingsJSON string
	}{
		FullList:     combinedList,
		FindingsJSON: findingsJSON,
	})
	if err != nil {
		return "", agentResult{}, err
	}
	result, err := e.runAgent(ctx, agentSpec{
		name:       fmt.Sprintf("Compiling Findings to Nudge from %s", parentName),
		role:       "extract",
		system:     system,
		user:       user,
		schemaKind: llm.SchemaKindText,
		hasTools:   false,
	}, reasoningExtractRequest(req))
	return reasoningExtractOutput(result.contentMessages), result, err
}

func reasoningExtractRequest(req model.ReviewRequest) model.ReviewRequest {
	req.NudgeCount = 0
	req.DisableReasoningExtract = true
	return req
}

func reasoningExtractOutput(messages []string) string {
	out := strings.TrimSpace(strings.Join(messages, "\n"))
	if strings.EqualFold(out, "NONE") {
		return ""
	}
	return out
}

var reasoningBulletPrefix = regexp.MustCompile(`^(?:[-*+]\s*|\d+[.)]\s+)`)

func formatReasoningFindingsList(findings string) string {
	lines := strings.Split(findings, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(reasoningBulletPrefix.ReplaceAllString(line, ""))
		if line == "" {
			continue
		}
		out = append(out, "- "+line)
	}
	return strings.Join(out, "\n")
}

func reasoningFindingsJSON(findings []model.Finding) (string, error) {
	return llm.RenderJSON(struct {
		Findings []model.Finding `json:"findings"`
	}{
		Findings: findings,
	})
}

func (e *Engine) renderReviewSystemWithQuestions(template, focusName, questionsSnippet string, req model.ReviewRequest, hasTools bool, agentRole string, styleGuides []model.StyleGuide, hasToolchainVersions bool) (string, error) {
	focusSnippet, err := e.renderReviewerFocusSnippet(focusName, questionsSnippet)
	if err != nil {
		return "", err
	}
	return e.renderReviewSystemWithFocus(template, focusSnippet, req, hasTools, agentRole, styleGuides, hasToolchainVersions)
}

func (e *Engine) renderReviewerQuestionsSnippet(questionsName string) (string, error) {
	if strings.TrimSpace(questionsName) != "" {
		questionsTemplate, err := e.loadPrompt(questionsName)
		if err != nil {
			return "", err
		}
		questionsSnippet, err := llm.RenderPrompt(questionsTemplate, nil)
		if err != nil {
			return "", fmt.Errorf("review: rendering reviewer questions prompt %s: %w", questionsName, err)
		}
		return strings.TrimSpace(questionsSnippet), nil
	}
	return "", nil
}

func (e *Engine) renderReviewerFocusSnippet(focusName, questionsSnippet string) (string, error) {
	focusTemplate, err := e.loadPrompt(focusName)
	if err != nil {
		return "", err
	}
	rendered, err := llm.RenderPrompt(focusTemplate, struct {
		QuestionsSnippet string
	}{
		QuestionsSnippet: strings.TrimSpace(questionsSnippet),
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering reviewer focus prompt %s: %w", focusName, err)
	}
	return rendered, nil
}

func (e *Engine) renderReviewSystemWithFocus(template, focusSnippet string, req model.ReviewRequest, hasTools bool, agentRole string, styleGuides []model.StyleGuide, hasToolchainVersions bool) (string, error) {
	toolInstructions := ""
	if hasTools {
		var err error
		toolInstructions, err = e.renderToolInstructions(toolInstructionsConfig{
			agentRole:                agentRole,
			parallelToolCallGuidance: !req.DisableParallelToolCalls,
		})
		if err != nil {
			return "", err
		}
	}
	outputSchemaSnippet := reviewOutputSchemaSnippetFor(req.UseJSONSchema)
	commonSnippets, err := agentCommonSystemPromptSnippets(agentRole, outputSchemaSnippet)
	if err != nil {
		return "", err
	}
	styleGuideToolchainSnippet, err := e.renderStyleGuideToolchainSnippet(agentRole, styleGuides, hasToolchainVersions)
	if err != nil {
		return "", err
	}
	systemPrompt, err := llm.RenderPrompt(template, struct {
		OutputSchemaSnippet        string
		FindingInstructionsSnippet string
		PrioritySnippet            string
		OutputFormatSnippet        string
		ParallelToolCallGuidance   bool
		HasTools                   bool
		FocusSnippet               string
		ToolInstructions           string
		StyleGuideToolchainSnippet string
	}{
		OutputSchemaSnippet:        outputSchemaSnippet,
		FindingInstructionsSnippet: commonSnippets.findingInstructions,
		PrioritySnippet:            commonSnippets.priority,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		ParallelToolCallGuidance:   !req.DisableParallelToolCalls,
		HasTools:                   hasTools,
		FocusSnippet:               strings.TrimSpace(focusSnippet),
		ToolInstructions:           toolInstructions,
		StyleGuideToolchainSnippet: strings.TrimSpace(styleGuideToolchainSnippet),
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering review system prompt: %w", err)
	}
	return systemPrompt, nil
}

type toolInstructionsConfig struct {
	agentRole                string
	parallelToolCallGuidance bool
	toolNames                []string
}

func (e *Engine) renderToolInstructions(config toolInstructionsConfig) (string, error) {
	template, err := e.loadPrompt("tool_instructions.tmpl")
	if err != nil {
		return "", err
	}
	rendered, err := llm.RenderPrompt(template, struct {
		AgentRole                string
		ParallelToolCallGuidance bool
		ToolListing              string
	}{
		AgentRole:                config.agentRole,
		ParallelToolCallGuidance: config.parallelToolCallGuidance,
		ToolListing:              toolInstructionsListing(config.toolNames...),
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering tool instructions prompt: %w", err)
	}
	return rendered, nil
}

func reviewerToolDefinitions(names ...string) []llm.ToolDefinition {
	definitions, err := toolcatalog.Definitions(names...)
	if err != nil {
		panic(fmt.Sprintf("review: selecting tool definitions: %v", err))
	}
	return definitions
}

func toolInstructionsListing(names ...string) string {
	listing, err := toolcatalog.InstructionsListing(names...)
	if err != nil {
		panic(fmt.Sprintf("review: selecting tool instructions: %v", err))
	}
	return listing
}

func noToolsMessagesFromRendered(systemPrompt string, messages []llm.Message) ([]llm.Message, error) {
	finalMessages := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		switch {
		case msg.Role == "assistant" && len(msg.ToolCalls) > 0:
			if strings.TrimSpace(msg.Content) != "" {
				finalMessages = append(finalMessages, llm.Message{Role: "assistant", Content: msg.Content})
			}
		case msg.Role == "tool":
			finalMessages = append(finalMessages, llm.Message{Role: "user", Content: msg.Content})
		default:
			finalMessages = append(finalMessages, msg)
		}
	}
	if len(finalMessages) == 0 {
		return []llm.Message{{Role: "system", Content: systemPrompt}}, nil
	}
	finalMessages[0] = llm.Message{Role: "system", Content: systemPrompt}
	return finalMessages, nil
}

// partialAgentResult wraps an aborted agent loop into a agentResult
// so callers in failure branches can read accumulated tokens / tool calls /
// content even when the loop errored. resp is intentionally left as the
// loop's last response (possibly nil) — callers must check before using it.
func partialAgentResult(agent agentSpec, req model.ReviewRequest, loop agentLoopResult) agentResult {
	return agentResult{
		resp:               loop.resp,
		reasoningEffort:    loop.reasoningEffort,
		contentMessages:    loop.contentMessages,
		toolMessages:       loop.toolMessages,
		toolCallHistory:    loop.toolCallHistory,
		duplicateToolCalls: loop.duplicateToolCalls,
		run: model.AgentRun{
			Name:                  agent.name,
			Role:                  agent.role,
			MaxToolCalls:          req.MaxToolCalls,
			MaxDuplicateToolCalls: req.MaxDuplicateToolCalls,
			ToolCalls:             loop.toolCalls,
			DuplicateToolCalls:    loop.duplicateToolCalls,
			TokensUsed:            loop.tokensUsed,
		},
	}
}

func emptyVerifiedMergeResult() agentResult {
	return agentResult{
		resp: &llm.ReviewResponse{
			Findings:               nil,
			OverallCorrectness:     "patch is correct",
			OverallExplanation:     "No verified findings remained after verification.",
			OverallConfidenceScore: 1,
		},
		run: model.AgentRun{
			Name:   "Merge Findings",
			Role:   "merge",
			Status: model.AgentRunStatusSkipped,
		},
	}
}

func appendNewFindings(existing, candidates []model.Finding) []model.Finding {
	if len(candidates) == 0 {
		return existing
	}
	out := append([]model.Finding(nil), existing...)
	seenIDTitles := make(map[string]struct{}, len(out))
	seenTitleLocations := make(map[string]struct{}, len(out))
	for _, finding := range out {
		recordFindingKeys(seenIDTitles, seenTitleLocations, finding)
	}
	for _, finding := range candidates {
		idTitleKey, titleLocationKey := findingDedupKeys(finding)
		if idTitleKey != "" {
			if _, ok := seenIDTitles[idTitleKey]; ok {
				continue
			}
		}
		if titleLocationKey != "" {
			if _, ok := seenTitleLocations[titleLocationKey]; ok {
				continue
			}
		}
		out = append(out, finding)
		recordFindingKeys(seenIDTitles, seenTitleLocations, finding)
	}
	return out
}

func recordFindingKeys(seenIDTitles, seenTitleLocations map[string]struct{}, finding model.Finding) {
	idTitleKey, titleLocationKey := findingDedupKeys(finding)
	if idTitleKey != "" {
		seenIDTitles[idTitleKey] = struct{}{}
	}
	if titleLocationKey != "" {
		seenTitleLocations[titleLocationKey] = struct{}{}
	}
}

func findingDedupKeys(finding model.Finding) (string, string) {
	title := strings.ToLower(strings.TrimSpace(finding.Title))
	if title == "" {
		return "", ""
	}
	idTitleKey := ""
	if id := strings.TrimSpace(finding.ID); id != "" {
		idTitleKey = id + "\x00" + title
	}
	loc := finding.CodeLocation
	titleLocationKey := fmt.Sprintf("%s\x00%s\x00%d\x00%d", title, loc.FilePath, loc.LineRange.Start, loc.LineRange.End)
	return idTitleKey, titleLocationKey
}

func supplementalFromContextAgent(messages []llm.Message) []model.SupplementalFile {
	out := make([]model.SupplementalFile, 0, len(messages))
	for i, msg := range messages {
		path := contextToolPath(msg.Content)
		if path == "" {
			path = fmt.Sprintf("context/tool-%d", i+1)
		}
		out = append(out, model.SupplementalFile{
			Path:    path,
			Content: msg.Content,
			Kind:    "context_tool_result",
			Reason:  "tool result gathered by context agent",
		})
	}
	return out
}

func appendResponseContent(contentMessages []string, resp *llm.ReviewResponse) []string {
	if resp == nil {
		return contentMessages
	}
	if content := strings.TrimSpace(resp.RawResponse); content != "" {
		contentMessages = append(contentMessages, content)
	}
	return contentMessages
}

func messagesWithFinalResponse(messages []llm.Message, resp *llm.ReviewResponse) []llm.Message {
	out := append([]llm.Message(nil), messages...)
	if resp == nil || strings.TrimSpace(resp.RawResponse) == "" {
		return out
	}
	if len(out) > 0 {
		last := out[len(out)-1]
		if last.Role == "assistant" && last.Content == resp.RawResponse {
			return out
		}
	}
	return append(out, llm.Message{Role: "assistant", Content: resp.RawResponse})
}

func contextAgentMarkdownMessages(contentMessages []string) []llm.Message {
	content := contextAgentMarkdownContent(contentMessages)
	if content == "" {
		return nil
	}
	return []llm.Message{{
		Role:    "user",
		Content: content,
	}}
}

func contextAgentMarkdownContent(contentMessages []string) string {
	var merged []string
	for _, content := range contentMessages {
		if content = strings.TrimSpace(content); content != "" {
			merged = append(merged, content)
		}
	}
	if len(merged) == 0 {
		return ""
	}
	rendered, err := renderPromptFile("agent_context_notes_snippet.tmpl", struct {
		Content string
	}{
		Content: strings.Join(merged, "\n\n---\n\n"),
	})
	if err != nil {
		panic(fmt.Sprintf("review: rendering context agent notes prompt: %v", err))
	}
	return rendered
}

func contextToolPath(content string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return ""
	}
	if path, _ := payload["path"].(string); path != "" {
		return path
	}
	if results, ok := payload["results"].([]any); ok && len(results) > 0 {
		if first, ok := results[0].(map[string]any); ok {
			path, _ := first["path"].(string)
			return path
		}
	}
	return ""
}

func addTokenUsage(left, right model.TokenUsage) model.TokenUsage {
	return model.TokenUsage{
		PromptTokens:     left.PromptTokens + right.PromptTokens,
		CompletionTokens: left.CompletionTokens + right.CompletionTokens,
		TotalTokens:      left.TotalTokens + right.TotalTokens,
	}
}

func exampleSnippetFor(kind llm.SchemaKind) string {
	if kind == llm.SchemaKindVerify {
		return llm.VerifyExamplePromptSnippet()
	}
	if kind == llm.SchemaKindMerge {
		return llm.MergeExamplePromptSnippet()
	}
	if kind == llm.SchemaKindFinalize {
		return llm.FinalizeExamplePromptSnippet()
	}
	if kind == llm.SchemaKindSummarize {
		return llm.SummarizeExamplePromptSnippet()
	}
	return llm.FindingsExamplePromptSnippet()
}

func noToolsMessages(agentRole string, systemTemplate string, messages []llm.Message, snippet string, styleGuideToolchainSnippet string) ([]llm.Message, error) {
	commonSnippets, err := agentCommonSystemPromptSnippets(agentRole, snippet)
	if err != nil {
		return nil, err
	}
	noToolsPrompt, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet        string
		FindingInstructionsSnippet string
		PrioritySnippet            string
		OutputFormatSnippet        string
		ParallelToolCallGuidance   bool
		HasTools                   bool
		ToolInstructions           string
		StyleGuideToolchainSnippet string
	}{
		OutputSchemaSnippet:        snippet,
		FindingInstructionsSnippet: commonSnippets.findingInstructions,
		PrioritySnippet:            commonSnippets.priority,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		HasTools:                   false,
		StyleGuideToolchainSnippet: strings.TrimSpace(styleGuideToolchainSnippet),
	})
	if err != nil {
		return nil, fmt.Errorf("review: rendering no-tools system prompt: %w", err)
	}
	finalMessages := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		switch {
		case msg.Role == "assistant" && len(msg.ToolCalls) > 0:
			if strings.TrimSpace(msg.Content) != "" {
				finalMessages = append(finalMessages, llm.Message{Role: "assistant", Content: msg.Content})
			}
		case msg.Role == "tool":
			finalMessages = append(finalMessages, llm.Message{Role: "user", Content: msg.Content})
		default:
			finalMessages = append(finalMessages, msg)
		}
	}
	if len(finalMessages) == 0 {
		finalMessages = append(finalMessages, llm.Message{Role: "system", Content: noToolsPrompt})
	} else {
		finalMessages[0] = llm.Message{Role: "system", Content: noToolsPrompt}
	}
	return finalMessages, nil
}

func (e *Engine) renderJSONRetryFeedback(invalid *llm.InvalidResponseError, exampleSnippet string) (string, error) {
	if exampleSnippet == "" {
		exampleSnippet = llm.FindingsExamplePromptSnippet()
	}
	guidance := ""
	if invalid.RetryGuidanceTemplate != "" {
		renderedGuidance, err := renderPromptFile(invalid.RetryGuidanceTemplate, invalid.RetryGuidanceData)
		if err != nil {
			return "", fmt.Errorf("review: rendering JSON retry guidance prompt: %w", err)
		}
		guidance = strings.TrimSpace(renderedGuidance)
	}
	rendered, err := renderPromptFile("helper_json_snippet.tmpl", struct {
		Reason         string
		MissingFields  string
		Guidance       string
		ExampleSnippet string
	}{
		Reason:         invalid.Reason,
		MissingFields:  strings.Join(invalid.MissingFields, ", "),
		Guidance:       guidance,
		ExampleSnippet: strings.TrimSpace(exampleSnippet),
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering JSON retry feedback prompt: %w", err)
	}
	return rendered, nil
}

func (e *Engine) loadPrompt(name string) (string, error) {
	e.logf(context.Background(), "Loading prompt: source=embedded name=%s", name)
	return prompts.Load(name)
}

func renderPromptFile(name string, data any) (string, error) {
	tmpl, err := prompts.Load(name)
	if err != nil {
		return "", err
	}
	return llm.RenderPrompt(tmpl, data)
}

func agentCommonSystemPromptSnippet(agentRole string, snippet string, outputSchemaSnippet string) (string, error) {
	rendered, err := renderPromptFile("agent_common_system_prompt_snippet.tmpl", struct {
		AgentRole           string
		Snippet             string
		OutputSchemaSnippet string
	}{
		AgentRole:           agentRole,
		Snippet:             snippet,
		OutputSchemaSnippet: outputSchemaSnippet,
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering common system prompt snippet %q for %s: %w", snippet, agentRole, err)
	}
	return rendered, nil
}

type agentCommonSystemPromptSnippetSet struct {
	findingInstructions string
	priority            string
	outputFormat        string
}

func agentCommonSystemPromptSnippets(agentRole string, outputSchemaSnippet string) (agentCommonSystemPromptSnippetSet, error) {
	findingInstructions, err := agentCommonSystemPromptSnippet(agentRole, "findings", "")
	if err != nil {
		return agentCommonSystemPromptSnippetSet{}, err
	}
	priority, err := agentCommonSystemPromptSnippet(agentRole, "priority", "")
	if err != nil {
		return agentCommonSystemPromptSnippetSet{}, err
	}
	outputFormatSnippet := "output_format"
	if outputSchemaSnippet == "" {
		outputFormatSnippet = "response_format"
	}
	outputFormat, err := agentCommonSystemPromptSnippet(agentRole, outputFormatSnippet, outputSchemaSnippet)
	if err != nil {
		return agentCommonSystemPromptSnippetSet{}, err
	}
	return agentCommonSystemPromptSnippetSet{
		findingInstructions: findingInstructions,
		priority:            priority,
		outputFormat:        outputFormat,
	}, nil
}

func (e *Engine) styleGuidesFor(ctx *model.ReviewContext) ([]model.StyleGuide, error) {
	languages := changedLanguages(ctx)
	guides := make([]model.StyleGuide, 0, len(languages))
	seenFiles := make(map[string]struct{})
	for _, language := range languages {
		name, ok := mappings.StyleGuideFile(language)
		if !ok {
			continue
		}
		if _, ok := seenFiles[name]; ok {
			continue
		}
		seenFiles[name] = struct{}{}
		content, err := prompts.Load(name)
		if err != nil {
			return nil, fmt.Errorf("review: loading style guide for %s: %w", language, err)
		}
		guides = append(guides, model.StyleGuide{
			Language: language,
			Content:  content,
			Title:    styleGuideTitle(content),
		})
	}
	return guides, nil
}

func (e *Engine) renderStyleGuideToolchainSnippet(agentRole string, guides []model.StyleGuide, hasToolchainVersions bool) (string, error) {
	agentRole = strings.TrimSpace(agentRole)
	if agentRole != "review" && agentRole != "verify" {
		return "", nil
	}
	titles := make([]string, 0, len(guides))
	for _, guide := range guides {
		title := strings.TrimSpace(guide.Title)
		if title != "" {
			titles = append(titles, title)
		}
	}
	if len(titles) == 0 && !hasToolchainVersions {
		return "", nil
	}
	template, err := e.loadPrompt("agent_styleguide_toolchain_snippet.tmpl")
	if err != nil {
		return "", err
	}
	rendered, err := llm.RenderPrompt(template, struct {
		AgentRole            string
		StyleGuideTitles     []string
		HasToolchainVersions bool
	}{
		AgentRole:            agentRole,
		StyleGuideTitles:     titles,
		HasToolchainVersions: hasToolchainVersions,
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering styleguide/toolchain prompt: %w", err)
	}
	return strings.TrimSpace(rendered), nil
}

func styleGuideTitle(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func changedLanguages(ctx *model.ReviewContext) []string {
	if ctx == nil {
		return nil
	}
	seen := make(map[string]struct{})
	for _, hunk := range ctx.DiffHunks {
		language := styleGuideLanguageForPath(hunk.FilePath)
		if language == "" {
			language = hunk.Language
		}
		addLanguage(seen, language)
		addDetectorLanguages(seen, hunk.FilePath, hunk.Content)
	}
	for _, file := range ctx.ChangedFiles {
		addLanguage(seen, styleGuideLanguageForPath(file.Path))
		content := ""
		if file.Status != model.FileDeleted && mappings.StyleGuideDetectorProbePath(file.Path) {
			content = readReviewFile(ctx.CheckoutRoot, file.Path)
		}
		addDetectorLanguages(seen, file.Path, content)
	}
	for _, supplemental := range ctx.SupplementalContext {
		addDetectorLanguages(seen, supplemental.Path, supplemental.Content)
	}

	languages := make([]string, 0, len(seen))
	for _, language := range mappings.StyleGuideOrder() {
		if _, ok := seen[language]; ok {
			languages = append(languages, language)
			delete(seen, language)
		}
	}
	for language := range seen {
		languages = append(languages, language)
	}
	return languages
}

func addLanguage(seen map[string]struct{}, language string) {
	if language == "" {
		return
	}
	if _, ok := mappings.StyleGuideFile(language); !ok {
		return
	}
	seen[language] = struct{}{}
}

func styleGuideLanguageForPath(path string) string {
	return mappings.StyleGuideLanguageForPath(path, filetype.DetectLanguage)
}

func addDetectorLanguages(seen map[string]struct{}, path, content string) {
	for _, language := range mappings.StyleGuideDetectorLanguages(path, content) {
		addLanguage(seen, language)
	}
}

const maxStyleGuideProbeBytes = 1 << 20

func readReviewFile(root, path string) string {
	if root == "" || path == "" {
		return ""
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return ""
	}
	fullPath := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, fullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	file, err := openReviewFileNoFollow(fullPath)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || info.IsDir() || info.Size() > maxStyleGuideProbeBytes {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(file, maxStyleGuideProbeBytes+1))
	if err != nil || len(data) > maxStyleGuideProbeBytes {
		return ""
	}
	return string(data)
}

func filterByPriority(findings []model.Finding, threshold string) []model.Finding {
	maxPriority := model.PriorityThresholdRank(threshold)
	filtered := make([]model.Finding, 0, len(findings))
	for _, finding := range findings {
		if model.PriorityRank(finding.Priority) <= maxPriority {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

func mergeConstraintsForRequest(req model.ReviewRequest) llm.ResponseConstraints {
	maxPriority := model.PriorityThresholdRank(req.PriorityThreshold)
	if maxPriority >= 3 {
		return llm.ResponseConstraints{}
	}
	return llm.ResponseConstraints{MaxPriority: intPtr(maxPriority)}
}

func hasResponseConstraints(c llm.ResponseConstraints) bool {
	return c.MinPriority != nil || c.MaxPriority != nil || len(c.AllowedCorrectness) > 0
}

func reviewOutputSchemaSnippetFor(useJSONSchema bool) string {
	if useJSONSchema {
		return ""
	}
	return llm.FindingsExamplePromptSnippet()
}

func mergeOutputSchemaSnippetFor(useJSONSchema bool) string {
	if useJSONSchema {
		return ""
	}
	return llm.MergeExamplePromptSnippet()
}

func outputSchemaSnippetFor(kind llm.SchemaKind, useJSONSchema bool) string {
	if kind == llm.SchemaKindMerge {
		return mergeOutputSchemaSnippetFor(useJSONSchema)
	}
	if kind == llm.SchemaKindFinalize {
		return finalizeOutputSchemaSnippetFor(useJSONSchema)
	}
	if kind == llm.SchemaKindVerify {
		return verifyOutputSchemaSnippetFor(useJSONSchema)
	}
	if kind == llm.SchemaKindSummarize {
		return summarizeOutputSchemaSnippetFor(useJSONSchema)
	}
	return reviewOutputSchemaSnippetFor(useJSONSchema)
}

// agentLoopKind maps an agentSpec role to the loop kind. Roles are uniform
// identifiers (context, review, verify, dedupe, merge, finalize, summarize,
// extract), so this is the identity today; it stays as the seam where a role
// would diverge from its loop kind.
func agentLoopKind(role string) string {
	return role
}

func (e *Engine) renderSyntheticToolFollowup(history []toolCallHistoryEntry, agentRole string) (string, error) {
	items := make([]syntheticToolFollowupEntry, 0, len(history))
	for i, entry := range history {
		items = append(items, syntheticToolFollowupEntryFromHistory(i+1, entry))
	}
	lastResult := toolResultSummary{}
	if len(history) > 0 {
		lastResult = history[len(history)-1].Result
	}
	rendered, err := renderPromptFile("helper_tools_snippet.tmpl", struct {
		History       []syntheticToolFollowupEntry
		RetryLastTool bool
		AgentRole     string
	}{
		History:       items,
		RetryLastTool: lastResult.IsError && lastResult.Code != "already_requested",
		AgentRole:     agentRole,
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering tool follow-up prompt: %w", err)
	}
	return rendered, nil
}

type syntheticToolFollowupEntry struct {
	Index       int
	Name        string
	OptimizedTo string
	ToolCallID  string
	Arguments   string
	Outcome     string
}

func syntheticToolFollowupEntryFromHistory(index int, entry toolCallHistoryEntry) syntheticToolFollowupEntry {
	toolCall := entry.ToolCall
	result := entry.Result
	var args toolCallArgs
	_ = llm.LenientUnmarshal(toolCall.Arguments, &args)
	return syntheticToolFollowupEntry{
		Index:       index,
		Name:        toolCall.Name,
		OptimizedTo: entry.OptimizedTo,
		ToolCallID:  toolCall.ID,
		Arguments:   syntheticToolArguments(toolCall.Name, args),
		Outcome:     syntheticToolOutcome(toolCall.Name, result),
	}
}

type toolResultSummary struct {
	IsError     bool
	Code        string
	Message     string
	Lines       int
	Files       int
	ResultCount int
}

type toolCallHistoryEntry struct {
	ToolCall    llm.ToolCall
	Result      toolResultSummary
	OptimizedTo string // non-empty when the tool call was rewritten, e.g. "search" → "find_callers"
}

func collectToolCallHistory(toolCalls []llm.ToolCall, toolMessages []llm.Message) []toolCallHistoryEntry {
	results := make(map[string]toolResultSummary, len(toolMessages))
	rawContents := make(map[string]string, len(toolMessages))
	for _, msg := range toolMessages {
		results[msg.ToolCallID] = parseToolResultSummary(msg.Content)
		rawContents[msg.ToolCallID] = msg.Content
	}

	history := make([]toolCallHistoryEntry, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		entry := toolCallHistoryEntry{
			ToolCall: toolCall,
			Result:   results[toolCall.ID],
		}
		if toolCall.Name == "search" && isCallHierarchyResult(rawContents[toolCall.ID]) {
			entry.OptimizedTo = "find_callers"
		}
		history = append(history, entry)
	}
	return history
}

func isCallHierarchyResult(content string) bool {
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return false
	}
	_, hasRoot := payload["root"]
	return hasRoot
}

func parseToolResultSummary(content string) toolResultSummary {
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return toolResultSummary{}
	}
	summary := toolResultSummary{}
	if status, _ := payload["status"].(string); status == "error" {
		summary.IsError = true
		if errPayload, ok := payload["error"].(map[string]any); ok {
			summary.Code, _ = errPayload["code"].(string)
			summary.Message, _ = errPayload["message"].(string)
		}
		return summary
	}

	if content, _ := payload["content"].(string); content != "" {
		summary.Lines = lineCount(content)
	}
	if files, ok := payload["files"].([]any); ok {
		summary.Files = len(files)
	}
	if results, ok := payload["results"].([]any); ok {
		summary.ResultCount = len(results)
		distinct := make(map[string]struct{}, len(results))
		for _, item := range results {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			path, _ := entry["path"].(string)
			if path == "" {
				continue
			}
			distinct[path] = struct{}{}
		}
		summary.Files = len(distinct)
	}
	if resultCount, ok := payload["result_count"].(float64); ok {
		summary.ResultCount = int(resultCount)
	}
	if root, ok := payload["root"].(map[string]any); ok {
		summary.Files = countCallHierarchyFiles(root)
		summary.Lines = countCallHierarchyLines(root)
	}
	return summary
}

func countDuplicateToolCalls(toolMessages []llm.Message) int {
	count := 0
	for _, msg := range toolMessages {
		summary := parseToolResultSummary(msg.Content)
		if summary.IsError && summary.Code == "already_requested" {
			count++
		}
	}
	return count
}

func countCallHierarchyFiles(root map[string]any) int {
	distinct := make(map[string]struct{})
	walkCallHierarchy(root, func(node map[string]any) {
		path, _ := node["path"].(string)
		if path != "" {
			distinct[path] = struct{}{}
		}
	})
	return len(distinct)
}

func countCallHierarchyLines(root map[string]any) int {
	lines := 0
	walkCallHierarchy(root, func(node map[string]any) {
		source, _ := node["source"].(string)
		lines += lineCount(source)
	})
	return lines
}

func walkCallHierarchy(node map[string]any, visit func(map[string]any)) {
	visit(node)
	children, _ := node["children"].([]any)
	for _, child := range children {
		childNode, ok := child.(map[string]any)
		if !ok {
			continue
		}
		walkCallHierarchy(childNode, visit)
	}
}

// toolCallArgs is the union of arguments across the retrieval tools
// (inspect_file, list_files, search, find_callers, find_callees). A single
// named type replaces the 9-field anonymous struct that was previously
// re-declared verbatim at several call sites.
type toolCallArgs struct {
	Path          string `json:"path"`
	LineStart     int    `json:"line_start"`
	LineEnd       int    `json:"line_end"`
	Depth         int    `json:"depth"`
	Symbol        string `json:"symbol"`
	Query         string `json:"query"`
	ContextLines  int    `json:"context_lines"`
	MaxResults    int    `json:"max_results"`
	CaseSensitive bool   `json:"case_sensitive"`
}

func syntheticToolArguments(toolName string, args toolCallArgs) string {
	parts := make([]string, 0, 5)
	switch toolName {
	case "inspect_file":
		parts = append(parts, fmt.Sprintf("path=%q", syntheticPathValue(args.Path, "<path>")))
		if args.LineStart > 0 {
			parts = append(parts, fmt.Sprintf("line_start=%d", args.LineStart))
		}
		if args.LineEnd > 0 {
			parts = append(parts, fmt.Sprintf("line_end=%d", args.LineEnd))
		}
	case "list_files":
		parts = append(parts, fmt.Sprintf("path=%q", syntheticPathValue(args.Path, ".")))
		if args.Depth <= 0 {
			args.Depth = 1
		}
		parts = append(parts, fmt.Sprintf("depth=%d", args.Depth))
	case "search":
		parts = append(parts, fmt.Sprintf("path=%q", syntheticPathValue(args.Path, ".")))
		parts = append(parts, fmt.Sprintf("query=%q", args.Query))
		if args.ContextLines < 0 {
			args.ContextLines = defaultSearchContextLines
		}
		parts = append(parts, fmt.Sprintf("context_lines=%d", args.ContextLines))
		parts = append(parts, fmt.Sprintf("max_results=%d", args.MaxResults))
		parts = append(parts, fmt.Sprintf("case_sensitive=%t", args.CaseSensitive))
	case "find_callers", "find_callees":
		parts = append(parts, fmt.Sprintf("path=%q", syntheticPathValue(args.Path, ".")))
		parts = append(parts, fmt.Sprintf("symbol=%q", args.Symbol))
		if args.Depth <= 0 {
			args.Depth = defaultCallHierarchyDepth
		}
		parts = append(parts, fmt.Sprintf("depth=%d", args.Depth))
	default:
		parts = append(parts, fmt.Sprintf("path=%q", syntheticPathValue(args.Path, "<path>")))
	}
	return strings.Join(parts, ", ")
}

func syntheticToolOutcome(toolName string, result toolResultSummary) string {
	if result.IsError {
		return fmt.Sprintf("error=%q", result.Message)
	}
	parts := make([]string, 0, 2)
	if result.Lines > 0 {
		parts = append(parts, fmt.Sprintf("lines=%d", result.Lines))
	}
	if result.Files > 0 || result.ResultCount > 0 {
		parts = append(parts, fmt.Sprintf("files=%d", result.Files))
	}
	if result.ResultCount > 0 {
		parts = append(parts, fmt.Sprintf("result_count=%d", result.ResultCount))
	}
	if len(parts) == 0 {
		if toolName == "search" {
			parts = append(parts, "result_count=0")
			return fmt.Sprintf("result=[%s]", strings.Join(parts, ", "))
		}
		parts = append(parts, "ok")
	}
	return fmt.Sprintf("result=[%s]", strings.Join(parts, ", "))
}

func syntheticPathValue(path, empty string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return empty
	}
	return path
}

func normalizeToolPath(path string) string {
	return strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), "./")
}

func (e *Engine) loggedReview(ctx context.Context, req *llm.ReviewRequest, sec *logging.ReasoningSection) (*llm.ReviewResponse, error) {
	callNum := sec.IncrCallNum()
	info, ok := logging.ProgressInfoFromContext(ctx)
	turnInfo := info.WithTurn(callNum)
	if ok && e.logger != nil {
		e.logger.ProgressFor(turnInfo, logging.StageRequest, logging.StateSent, "")
		e.logger.ProgressFor(turnInfo, logging.StageReasoning, logging.StateStart, "")
	}
	previousSink := req.ReasoningSink
	callSec := e.openReviewRequestReasoningSection(info, callNum)
	req.ReasoningSink = llm.TeeReasoningSinks(callSec, previousSink)
	defer func() {
		req.ReasoningSink = previousSink
		callSec.End()
	}()
	start := time.Now()
	resp, err := e.llm.Review(ctx, req)
	elapsed := time.Since(start).Truncate(time.Second)
	if ok && e.logger != nil {
		if resp != nil && resp.Reasoned {
			e.logger.ProgressFor(turnInfo, logging.StageReasoning, logging.StateDone, elapsed.String())
		}
		e.logger.ProgressFor(turnInfo, logging.StageResponse, logging.StateDone, elapsed.String())
	}
	return resp, err
}

func (e *Engine) openReviewRequestReasoningSection(info logging.ProgressInfo, callNum int) *logging.ReasoningSection {
	if e.logger == nil || !e.logger.ShowReasoning() {
		return nil
	}
	if info.IsZero() || callNum <= 0 {
		return e.logger.OpenReasoningSection(logging.ProgressInfo{})
	}
	return e.logger.OpenReasoningSection(info.WithTurn(callNum))
}

func (e *Engine) logf(ctx context.Context, format string, args ...any) {
	if e.logger == nil {
		return
	}
	e.logger.Verbosef(ctx, format, args...)
}

func (e *Engine) logBlock(ctx context.Context, label, content string) {
	if e.logger == nil {
		return
	}
	e.logger.VerboseBlock(ctx, label, content)
}

// progressInfo builds the ctx-carried logging identity for an agent, filling
// model and effort from the engine profile.
func (e *Engine) progressInfo(role, name, detail string) logging.ProgressInfo {
	return logging.ProgressInfo{
		AgentRole: role,
		AgentName: name,
		Detail:    detail,
		Model:     e.config.Model,
		Effort:    e.config.ReasoningEffort,
	}
}

func (e *Engine) logProgress(stage logging.Stage, state logging.State, msg string) {
	if e.logger != nil {
		e.logger.ProgressFor(e.progressInfo("", "", ""), stage, state, msg)
	}
}

func (e *Engine) logToolCall(ctx context.Context, toolCall llm.ToolCall, result string) {
	if e.logger == nil {
		return
	}
	e.logger.ProgressToolCall(ctx, toolCallDisplay(toolCall), syntheticToolOutcome(toolCall.Name, parseToolResultSummary(result)))
}

func syntheticToolArgumentsForCall(toolCall llm.ToolCall) string {
	var args toolCallArgs
	_ = llm.LenientUnmarshal(toolCall.Arguments, &args)
	return syntheticToolArguments(toolCall.Name, args)
}

func toolCallDisplay(toolCall llm.ToolCall) string {
	if optimized, ok := optimizedSearchToolCallDisplay(toolCall); ok {
		return optimized
	}
	return fmt.Sprintf("%s(%s)", toolCall.Name, syntheticToolArgumentsForCall(toolCall))
}

func reviewContextSummary(ctx *model.ReviewContext, req model.ReviewRequest) string {
	if ctx == nil {
		return ""
	}
	return fmt.Sprintf("%s:%s [%s, ≥%s] on %s @ %s → %s",
		ctx.Mode, req.Submode,
		req.ProfileName, req.PriorityThreshold,
		ctx.Repository.FullName,
		ctx.Repository.HeadRef, ctx.Repository.BaseRef,
	)
}

func optimizedSearchToolCallDisplay(toolCall llm.ToolCall) (string, bool) {
	if toolCall.Name != "search" {
		return "", false
	}
	var args struct {
		Path          string `json:"path"`
		Query         string `json:"query"`
		ContextLines  int    `json:"context_lines"`
		MaxResults    int    `json:"max_results"`
		CaseSensitive bool   `json:"case_sensitive"`
	}
	if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
		return "", false
	}
	normalizedPath := normalizeToolPath(strings.TrimSpace(args.Path))
	query := strings.TrimSpace(args.Query)
	matches := searchFunctionQueryPattern.FindStringSubmatch(query)
	if len(matches) != 2 {
		return "", false
	}
	findArgs := syntheticToolArguments("find_callers", toolCallArgs{
		Path:   normalizedPath,
		Symbol: matches[1],
		Depth:  defaultCallHierarchyDepth,
	})
	return fmt.Sprintf(`find_callers(instead_of="search", %s)`, findArgs), true
}

func lineCount(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func rangeAlreadyCovered(seen []model.LineRange, requested model.LineRange) bool {
	for _, existing := range seen {
		if existing.Start <= requested.Start && existing.End >= requested.End {
			return true
		}
	}
	return false
}
