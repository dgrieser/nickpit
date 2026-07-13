package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/dedupe"
	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/toolchain"
	toolcatalog "github.com/dgrieser/nickpit/internal/tools"
	"github.com/dgrieser/nickpit/internal/versionmatch"
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
	// additionalStyleGuides holds user-supplied guides (files/URLs, already
	// resolved) added to a review's language styleguides. Each carries optional
	// gating metadata: an ungated guide is appended for every agent, a gated one
	// only when its language changed and (when set) the detected toolchain
	// version matches. Written once before the pipeline runs and read-only
	// afterwards, so withConfig's shallow clones and concurrent agents can share
	// it without locking.
	additionalStyleGuides []model.AdditionalStyleGuide
	// disabledStyleGuides holds built-in styleguide languages the user turned
	// off. Same write-once-before-pipeline contract as additionalStyleGuides.
	disabledStyleGuides map[string]struct{}
	// structuralSupport memoizes retrieval.SupportsStructuralAnalysis per
	// (repoRoot, path). The result is deterministic over a review's fixed
	// checkout, so caching it avoids a redundant os.Stat on every search call
	// (the check runs in both toolCallConcurrencyKey and executeSearch). It is a
	// pointer so withConfig's shallow clones share one cache (and so the Engine
	// stays copyable — a sync.Map value must not be copied). Its values are bools,
	// so it is trivially small and intentionally left uncapped.
	structuralSupport *sync.Map // repoRoot\x00path -> bool
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
		structuralSupport:      &sync.Map{},
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

// SetAdditionalStyleGuides installs user-supplied styleguides added to a
// review's language styleguides (subject to each guide's gating metadata).
// Must be called before the pipeline runs; the slice must not be mutated
// afterwards.
func (e *Engine) SetAdditionalStyleGuides(guides []model.AdditionalStyleGuide) {
	e.additionalStyleGuides = guides
}

// SetDisabledStyleGuides turns off built-in styleguides for the given
// (already validated, lowercased) languages. Must be called before the
// pipeline runs.
func (e *Engine) SetDisabledStyleGuides(languages []string) {
	if len(languages) == 0 {
		e.disabledStyleGuides = nil
		return
	}
	disabled := make(map[string]struct{}, len(languages))
	for _, language := range languages {
		disabled[language] = struct{}{}
	}
	e.disabledStyleGuides = disabled
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
	stampGeneratedFlags(reviewCtx)
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
		maxTokens := req.MaxContextTokens
		if maxTokens <= 0 {
			maxTokens = config.DefaultMaxContextToken
		}
		estimator := model.SimpleEstimator{}
		headroom := promptOverheadTokens(estimator, reviewCtx, req.DiffFormat, maxTokens)
		trimmer = NewTrimmer(maxTokens, estimator, WithHeadroomTokens(headroom))
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

func (e *Engine) reviewWithoutTools(ctx context.Context, llmReq *llm.ReviewRequest, agentRole string, systemTemplate string, messages []llm.Message, systemSnippet string, styleGuideToolchainSnippet string, disableSuggestions bool, maxOutputRetries int, sec *logging.ReasoningSection, loopReq agentLoopRequest, state *agentLoopState, recordTokens func(model.TokenUsage)) (*llm.ReviewResponse, error) {
	// When the loop request already knows how to build the no-tools transcript,
	// use it. Review/context agents carry an already-RENDERED system prompt in
	// systemTemplate; re-parsing it as a Go template breaks on prompts that embed
	// template-looking content (e.g. a helm style guide with `{{ default ... }}`).
	// Only the fallback path renders: it receives a raw template (verify-style).
	var finalMessages []llm.Message
	var err error
	if loopReq.NoToolsMessages != nil {
		finalMessages, err = loopReq.NoToolsMessages(messages)
	} else {
		finalMessages, err = noToolsMessages(agentRole, systemTemplate, messages, systemSnippet, styleGuideToolchainSnippet, disableSuggestions)
	}
	if err != nil {
		return nil, err
	}
	llmReq.Messages = finalMessages
	llmReq.Tools = nil
	llmReq.ParallelToolCalls = false
	exampleSnippet := exampleSnippetFor(llmReq.SchemaKind, disableSuggestions)
	for attempt := 0; ; attempt++ {
		resp, err := e.loggedReview(ctx, llmReq, sec)
		if recordTokens != nil {
			recordTokens(reviewCallTokens(resp, err))
		}
		if err == nil {
			if retryInvalid := e.repairResponseOrRetry(ctx, loopReq, resp); retryInvalid != nil {
				retryMessages := append([]llm.Message(nil), llmReq.Messages...)
				var synthetic *llm.Message
				queued, err := e.tryQueueCodeLocationRetry(ctx, loopReq, state, retryInvalid, &retryMessages, &synthetic, llmReq, outputRetriesRemaining(attempt, maxOutputRetries))
				if err != nil {
					return nil, err
				}
				if queued {
					llmReq.Messages = retryMessages
					continue
				}
				e.logf(ctx, "Code location repair needed retry but retry budget is exhausted; keeping no-tools response as-is: missing=%v", retryInvalid.MissingFields)
			}
			if resp.ReasoningEffort != "" {
				llmReq.ReasoningEffort = resp.ReasoningEffort
			}
			return resp, nil
		}
		var invalidResp *llm.InvalidResponseError
		if errors.As(err, &invalidResp) {
			if partialResp, retryInvalid, handled := e.tryRepairPartialResponse(ctx, loopReq, invalidResp); handled {
				if retryInvalid != nil {
					retryMessages := append([]llm.Message(nil), llmReq.Messages...)
					var synthetic *llm.Message
					queued, err := e.tryQueueCodeLocationRetry(ctx, loopReq, state, retryInvalid, &retryMessages, &synthetic, llmReq, outputRetriesRemaining(attempt, maxOutputRetries))
					if err != nil {
						return nil, err
					}
					if queued {
						llmReq.Messages = retryMessages
						continue
					}
					e.logf(ctx, "Code location repair needed retry but retry budget is exhausted; using partial no-tools response: missing=%v", retryInvalid.MissingFields)
				}
				return partialResp, nil
			}
		}
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
	// reviewSessionValidateResponse validates reviewer responses against
	// findings already accumulated by the same reviewer session.
	reviewSessionValidateResponse func([]model.Finding, *llm.ReviewResponse) *llm.InvalidResponseError
	// reviewSessionEnforceResponse repairs a reviewer response after retry
	// exhaustion. It may mutate resp and returns a partial-run message.
	reviewSessionEnforceResponse func(string, []model.Finding, *llm.ReviewResponse) string
	// maxFindings caps the findings the reviewer session may accumulate across
	// its initial pass and nudges; 0 = unlimited. Enforced by a one-retry
	// validator per turn, then by cutting the weakest findings.
	maxFindings int
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
	id               string
	name             string
	focusFile        string
	questionsFile    string
	constraints      llm.ResponseConstraints
	validateResponse func([]model.Finding, *llm.ReviewResponse) *llm.InvalidResponseError
	enforceResponse  func(string, []model.Finding, *llm.ReviewResponse) string
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
		validateResponse: validateTestingDuplicateFileResponse,
		enforceResponse:  enforceTestingDuplicateFileResponse,
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
func (e *Engine) verifyAndFilterVectorFindings(ctx context.Context, reviewCtx *model.ReviewContext, vectorResults []agentResult, req model.ReviewRequest, limiter *Limiter, reviewerName string) (model.TokenUsage, []string, error) {
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
	opts.Limiter = limiter
	opts.ReviewerName = reviewerName
	verifications, usage, warnings, err := e.VerifyAll(ctx, reviewCtx, findings, opts)
	if err != nil {
		return usage, warnings, err
	}
	if len(verifications) != len(refs) {
		return usage, warnings, fmt.Errorf("review: verifier returned %d results for %d findings", len(verifications), len(refs))
	}
	type dropCounts struct {
		refuted    int
		unverified int
	}
	keptByVector := make(map[int][]model.Finding, len(vectorResults))
	droppedIdxByVector := make(map[int]map[int]struct{}, len(vectorResults))
	dropsByVector := make(map[int]dropCounts, len(vectorResults))
	for i, verification := range verifications {
		ref := refs[i]
		finding := vectorResults[ref.vectorIdx].resp.Findings[ref.findingIdx]
		// findings[i].ID holds the normalized ID after EnsureFindingIDs above,
		// which may have replaced an invalid or duplicate reviewer ID. Always
		// adopt it so corrected IDs survive into downstream dedupe/merge
		// validation and stay in sync with Verification.ID.
		finding.ID = findings[i].ID
		if verification == nil {
			verification = fallbackUnverifiedVerification(finding)
			verifications[i] = verification
		}
		v := *verification
		model.EnsureVerificationID(&v, finding.ID)
		drop, reason := shouldDropFinding(&v, opts.DropPolicy)
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
		finding.Verification = &v
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
		if dropped > 0 {
			e.logf(ctx, "Verifier filter before merge: reviewer=%s dropped=%d refuted=%d unverified=%d kept=%d policy=%s",
				vectorResults[vectorIdx].run.Name,
				dropped,
				counts.refuted,
				counts.unverified,
				len(keptByVector[vectorIdx]),
				normalizeDropPolicy(opts.DropPolicy),
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
//   - "kept": verdict does not warrant dropping (e.g. "confirmed", or policy="none")
func shouldDropFinding(v *model.FindingVerification, policy string) (bool, string) {
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
		DisableJSONResponseFormat: req.DisableJSONResponseFormat,
		MaxToolCalls:              req.MaxToolCalls,
		MaxDuplicateToolCalls:     req.MaxDuplicateToolCalls,
		MaxOutputRetries:          req.MaxOutputRetries,
		MaxReasoningSeconds:       req.MaxReasoningSeconds,
		DisableParallelToolCalls:  req.DisableParallelToolCalls,
		DisableSuggestions:        req.DisableSuggestions,
		RepoRoot:                  req.RepoRoot,
		DropPolicy:                req.VerifyDropPolicy,
		DiffFormat:                req.DiffFormat,
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
	if req.DisableJSONResponseFormat {
		return nil
	}
	constraints := mergeConstraintsForRequest(req)
	if hasResponseConstraints(constraints) {
		return llm.MergeSchemaWithConstraintsFor(constraints, req.DisableSuggestions)
	}
	if req.DisableSuggestions {
		return llm.MergeSchemaWithoutSuggestions
	}
	return llm.MergeSchema
}

func mergeConstraintsForDedupe(req model.ReviewRequest) llm.ResponseConstraints {
	if req.DisableJSONResponseFormat {
		return llm.ResponseConstraints{}
	}
	return mergeConstraintsForRequest(req)
}

// mechanicallyDedupeFindings folds clusters of mechanically-detectable
// duplicates (dedupe.Duplicate or stronger) into single findings, so LLM
// dedupe/merge agents only ever judge the ambiguous remainder. Returns the
// reduced list and how many findings were absorbed; zero absorbed returns the
// input slice untouched.
func mechanicallyDedupeFindings(findings []model.Finding) ([]model.Finding, int) {
	clusters := dedupe.Clusters(findings, dedupe.Duplicate)
	if len(clusters) == len(findings) {
		return findings, 0
	}
	out := make([]model.Finding, 0, len(clusters))
	for _, cluster := range clusters {
		if len(cluster) == 1 {
			out = append(out, findings[cluster[0]])
			continue
		}
		members := make([]model.Finding, 0, len(cluster))
		for _, idx := range cluster {
			members = append(members, findings[idx])
		}
		out = append(out, dedupe.FoldCluster(members))
	}
	return out, len(findings) - len(out)
}

// runDedupeAgents runs a per-reviewer dedupe pass concurrently. It intentionally
// mutates vectorResults[idx].resp in place, but only when a dedupe agent returns
// a valid response. A failed or invalid dedupe leaves that reviewer's original
// findings intact and only records the dedupe run for telemetry. Mechanically
// detectable duplicates are folded in code first; the LLM agent only sees the
// reduced set.
func (e *Engine) runDedupeAgents(ctx context.Context, contextNotes string, vectorResults []agentResult, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest) []model.AgentRun {
	runs := make([]model.AgentRun, len(vectorResults))
	var wg sync.WaitGroup
	for i := range vectorResults {
		result := vectorResults[i]
		if result.run.Status == model.AgentRunStatusFailed || result.resp == nil || len(result.resp.Findings) < 2 {
			continue
		}
		if reduced, absorbed := mechanicallyDedupeFindings(result.resp.Findings); absorbed > 0 {
			resp := cloneReviewResponse(result.resp)
			resp.Findings = reduced
			vectorResults[i].resp = resp
			result.resp = resp
			e.logf(ctx, "Mechanical dedupe: reviewer=%q absorbed=%d findings=%d", result.run.Name, absorbed, len(reduced))
		}
		if len(result.resp.Findings) < 2 {
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
	resp := cloneReviewResponse(result.resp)
	// The dedupe agent shares the merge output schema, so a model may emit
	// merged_from provenance here as well; it must not leak downstream.
	stripMergedFrom(resp.Findings)
	return resp, run
}

func (e *Engine) callDedupeAgent(ctx context.Context, contextNotes string, input agentResult, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest) (agentResult, error) {
	systemTemplate, err := e.loadPrompt("agent_dedupe_system_prompt.tmpl")
	if err != nil {
		return agentResult{}, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("dedupe", exampleSnippetFor(llm.SchemaKindMerge, req.DisableSuggestions), req.DisableSuggestions)
	if err != nil {
		return agentResult{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		FindingInstructionsSnippet string
		PrioritySnippet            string
		OutputFormatSnippet        string
		DisableSuggestions         bool
	}{
		FindingInstructionsSnippet: commonSnippets.findingInstructions,
		PrioritySnippet:            commonSnippets.priority,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		DisableSuggestions:         req.DisableSuggestions,
	})
	if err != nil {
		return agentResult{}, fmt.Errorf("review: rendering dedupe system prompt: %w", err)
	}
	dedupeUser, err := llm.RenderJSON(map[string]any{
		"context_agent_notes": contextNotes,
		"review_findings": map[string]any{
			"name":                     input.run.Name,
			"role":                     input.run.Role,
			"findings":                 findingsPromptPayload(input.resp.Findings, req.DisableSuggestions),
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

// runClusterMergeAgents merges all reviewer finding lists into one. Findings
// are clustered mechanically (dedupe.Clusters at Possible): clear duplicates
// inside a cluster are folded in code, singleton clusters pass through
// untouched, and only ambiguous clusters are judged by small merge agents that
// run concurrently. A failed or invalid micro-merge keeps its cluster's
// findings unmerged — bias toward inclusion: a rare surviving near-duplicate
// beats silently losing a reviewer's finding.
func (e *Engine) runClusterMergeAgents(ctx context.Context, userPrompt string, contextNotes string, inputs []pairwiseMergeInput, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest) (agentResult, []model.AgentRun) {
	return e.runClusterMergeAgentsWithStyleGuides(ctx, userPrompt, contextNotes, inputs, schema, constraints, req, nil, false)
}

func (e *Engine) runClusterMergeAgentsWithStyleGuides(ctx context.Context, userPrompt string, contextNotes string, inputs []pairwiseMergeInput, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest, styleGuides []model.StyleGuide, hasToolchainVersions bool) (agentResult, []model.AgentRun) {
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

	findings, reviewerByID := flattenMergeMembers(inputs)
	clusters := dedupe.Clusters(findings, dedupe.Possible)

	outcomes := make([][]model.Finding, len(clusters))
	runs := make([]model.AgentRun, len(clusters))
	absorbed := 0
	llmClusters := 0
	var wg sync.WaitGroup
	for ci, cluster := range clusters {
		clusterFindings := make([]model.Finding, 0, len(cluster))
		for _, idx := range cluster {
			clusterFindings = append(clusterFindings, findings[idx])
		}
		reduced, folded := mechanicallyDedupeFindings(clusterFindings)
		absorbed += folded
		if len(reduced) == 1 {
			outcomes[ci] = reduced
			continue
		}
		llmClusters++
		wg.Add(1)
		go func(ci int, reduced []model.Finding) {
			defer wg.Done()
			outcomes[ci], runs[ci] = e.runClusterMergeAgentWithStyleGuides(ctx, userPrompt, contextNotes, reduced, reviewerByID, schema, constraints, req, styleGuides, hasToolchainVersions)
		}(ci, reduced)
	}
	wg.Wait()

	var merged []model.Finding
	mergeRuns := make([]model.AgentRun, 0, llmClusters)
	for ci := range clusters {
		merged = append(merged, outcomes[ci]...)
		if runs[ci].Name != "" {
			mergeRuns = append(mergeRuns, runs[ci])
		}
	}
	e.logf(ctx, "Mechanical merge: findings=%d clusters=%d llm_clusters=%d absorbed=%d merged=%d",
		len(findings), len(clusters), llmClusters, absorbed, len(merged))

	resp := cloneReviewResponse(&llm.ReviewResponse{
		Findings:               merged,
		OverallCorrectness:     aggregateOverallCorrectness(inputs, len(merged)),
		OverallExplanation:     fmt.Sprintf("Merged %d reviewer finding lists (%d findings) into %d findings: %d absorbed mechanically, %d clusters judged by merge agents.", len(inputs), len(findings), len(merged), absorbed, llmClusters),
		OverallConfidenceScore: maxOverallConfidence(inputs),
	})
	if len(mergeRuns) == 0 {
		mergeRuns = append(mergeRuns, model.AgentRun{
			Name:   "Merge Findings",
			Role:   "merge",
			Status: model.AgentRunStatusSkipped,
		})
	}
	return agentResult{resp: resp, run: mergeRuns[len(mergeRuns)-1]}, mergeRuns
}

// flattenMergeMembers flattens all reviewer responses into one finding list
// and records which reviewer produced each finding (keyed by finding ID) for
// prompt provenance. The parser (EnsureFindingIDs) only dedupes IDs within a
// single response, so two reviewers can legitimately emit the same ID; the
// flattened list is re-normalized here so reviewerByID stays collision-free
// and validateClusterMergeResponse cannot mark two distinct inputs covered by
// one output finding.
func flattenMergeMembers(inputs []pairwiseMergeInput) ([]model.Finding, map[string]string) {
	var findings []model.Finding
	var reviewers []string
	for _, input := range inputs {
		if input.response == nil {
			continue
		}
		for _, f := range input.response.Findings {
			// Clone the verification so the ID normalization below cannot
			// mutate the reviewer's original response through the shared
			// pointer when a colliding ID is reminted.
			if f.Verification != nil {
				verification := *f.Verification
				f.Verification = &verification
			}
			findings = append(findings, f)
			reviewers = append(reviewers, input.name)
		}
	}
	normalizeFindingIDsWithSeen(findings, nil)
	reviewerByID := make(map[string]string, len(findings))
	for i := range findings {
		reviewerByID[findings[i].ID] = reviewers[i]
	}
	return findings, reviewerByID
}

// runClusterMergeAgentWithStyleGuides judges one ambiguous cluster. Any failure
// path returns the cluster unmerged so reviewer findings are never lost.
func (e *Engine) runClusterMergeAgentWithStyleGuides(ctx context.Context, userPrompt string, contextNotes string, cluster []model.Finding, reviewerByID map[string]string, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest, styleGuides []model.StyleGuide, hasToolchainVersions bool) ([]model.Finding, model.AgentRun) {
	result, err := e.callClusterMergeAgentWithStyleGuides(ctx, userPrompt, contextNotes, cluster, reviewerByID, schema, constraints, req, styleGuides, hasToolchainVersions)
	run := result.run
	if err != nil {
		return cluster, markMergeRun(run, model.AgentRunStatusFailed, err)
	}
	if result.resp == nil {
		return cluster, markMergeRun(run, model.AgentRunStatusFailed, fmt.Errorf("merge step returned no response"))
	}
	if repaired := repairClusterMergeProvenance(result.resp, cluster); repaired > 0 {
		e.logf(ctx, "Merge provenance repair: repaired=%d", repaired)
	}
	if invalid := validateClusterMergeResponse(result.resp, cluster); invalid != nil {
		return cluster, markMergeRun(run, model.AgentRunStatusPartial, invalid)
	}
	findings := cloneReviewResponse(result.resp).Findings
	stripMergedFrom(findings)
	return findings, markMergeRun(run, model.AgentRunStatusOK, nil)
}

// stripMergedFrom clears the merge-step provenance once validation consumed
// it, so merged_from never leaks into results or posted reviews.
func stripMergedFrom(findings []model.Finding) {
	for i := range findings {
		findings[i].MergedFrom = nil
	}
}

// aggregateOverallCorrectness derives the merged verdict mechanically: any
// input saying "patch is incorrect" wins, otherwise the first explicit input
// verdict carries (an explicit "patch is correct" alongside findings is
// legitimate — e.g. the Testing vector is constrained to it). Only when no
// input carries a verdict at all (source-less merge over bare findings files)
// is the default derived from the merged findings: preserved findings with a
// "patch is correct" default would contradict the emitted result.
func aggregateOverallCorrectness(inputs []pairwiseMergeInput, mergedFindings int) string {
	out := ""
	for _, input := range inputs {
		if input.response == nil {
			continue
		}
		if input.response.OverallCorrectness == "patch is incorrect" {
			return "patch is incorrect"
		}
		if out == "" {
			out = input.response.OverallCorrectness
		}
	}
	if out == "" {
		if mergedFindings > 0 {
			return "patch is incorrect"
		}
		return "patch is correct"
	}
	return out
}

func maxOverallConfidence(inputs []pairwiseMergeInput) float64 {
	out := 0.0
	for _, input := range inputs {
		if input.response != nil && input.response.OverallConfidenceScore > out {
			out = input.response.OverallConfidenceScore
		}
	}
	return out
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
		if len(finding.MergedFrom) > 0 {
			clone.Findings[i].MergedFrom = append([]string(nil), finding.MergedFrom...)
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

func (e *Engine) callClusterMergeAgentWithStyleGuides(ctx context.Context, userPrompt string, contextNotes string, cluster []model.Finding, reviewerByID map[string]string, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest, styleGuides []model.StyleGuide, hasToolchainVersions bool) (agentResult, error) {
	systemTemplate, err := e.loadPrompt("agent_cluster_merge_system_prompt.tmpl")
	if err != nil {
		return agentResult{}, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("merge", exampleSnippetFor(llm.SchemaKindMerge, req.DisableSuggestions), req.DisableSuggestions)
	if err != nil {
		return agentResult{}, err
	}
	styleGuideToolchainSnippet, err := e.renderStyleGuideToolchainSnippet("merge", styleGuides, hasToolchainVersions)
	if err != nil {
		return agentResult{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		FindingInstructionsSnippet string
		PrioritySnippet            string
		OutputFormatSnippet        string
		DisableSuggestions         bool
		StyleGuideToolchainSnippet string
	}{
		FindingInstructionsSnippet: commonSnippets.findingInstructions,
		PrioritySnippet:            commonSnippets.priority,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		DisableSuggestions:         req.DisableSuggestions,
		StyleGuideToolchainSnippet: strings.TrimSpace(styleGuideToolchainSnippet),
	})
	if err != nil {
		return agentResult{}, fmt.Errorf("review: rendering merge system prompt: %w", err)
	}
	mergeUser, err := llm.RenderJSON(map[string]any{
		"review_context":      json.RawMessage(userPrompt),
		"context_agent_notes": contextNotes,
		"cluster_signals":     clusterMergeSignals(cluster),
		"cluster_findings":    clusterMergePayload(cluster, reviewerByID, req.DisableSuggestions),
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
			if repaired := repairClusterMergeProvenance(resp, cluster); repaired > 0 {
				e.logf(ctx, "Merge provenance repair: repaired=%d", repaired)
			}
			return validateClusterMergeResponse(resp, cluster)
		},
	}, req)
}

func clusterMergePayload(cluster []model.Finding, reviewerByID map[string]string, disableSuggestions bool) []map[string]any {
	out := make([]map[string]any, 0, len(cluster))
	for _, f := range cluster {
		out = append(out, map[string]any{
			"reviewer": reviewerByID[f.ID],
			"finding":  findingPromptPayload(f, disableSuggestions),
		})
	}
	return out
}

func findingsPromptPayload(findings []model.Finding, disableSuggestions bool) []model.Finding {
	if !disableSuggestions {
		return findings
	}
	out := make([]model.Finding, len(findings))
	for i := range findings {
		out[i] = findingPromptPayload(findings[i], true)
	}
	return out
}

func findingPromptPayload(finding model.Finding, disableSuggestions bool) model.Finding {
	if !disableSuggestions {
		return finding
	}
	finding.Suggestions = nil
	if finding.Finalization != nil {
		finalization := *finding.Finalization
		finalization.Suggestions = nil
		finding.Finalization = &finalization
	}
	if finding.Summarization != nil {
		summarization := *finding.Summarization
		summarization.Suggestions = nil
		finding.Summarization = &summarization
	}
	return finding
}

func clusterMergeSignals(cluster []model.Finding) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for i := range cluster {
		for j := i + 1; j < len(cluster); j++ {
			match := dedupe.Compare(cluster[i], cluster[j])
			if match.Verdict < dedupe.Possible || match.Reason == "" {
				continue
			}
			if _, ok := seen[match.Reason]; ok {
				continue
			}
			seen[match.Reason] = struct{}{}
			out = append(out, match.Reason)
		}
	}
	if len(out) == 0 && len(cluster) > 1 {
		return []string{"possible duplicate cluster"}
	}
	return out
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
	verifiedInputIDs := map[string]struct{}{}
	var allowedIDs []string
	if input != nil {
		inputCount = len(input.Findings)
		inputIDs = make(map[string]struct{}, inputCount)
		verifiedInputIDs = make(map[string]struct{}, inputCount)
		allowedIDs = make([]string, 0, inputCount)
		for _, finding := range input.Findings {
			id := strings.TrimSpace(finding.ID)
			if id != "" {
				allowedIDs = append(allowedIDs, id)
				inputIDs[id] = struct{}{}
				if finding.Verification != nil {
					verifiedInputIDs[id] = struct{}{}
				}
			}
		}
	}
	minCount := dedupeMinCount(inputCount)
	countTooLow := len(resp.Findings) < minCount
	countTooHigh := len(resp.Findings) > inputCount
	unknownIDs := 0
	var unknownIDValues []string
	duplicateIDs := 0
	verificationMismatch := 0
	seen := map[string]struct{}{}
	for _, finding := range resp.Findings {
		id := strings.TrimSpace(finding.ID)
		if id == "" {
			unknownIDs++
			unknownIDValues = append(unknownIDValues, "")
		} else {
			if _, ok := inputIDs[id]; !ok {
				unknownIDs++
				unknownIDValues = append(unknownIDValues, id)
			}
			if _, ok := seen[id]; ok {
				duplicateIDs++
			}
			seen[id] = struct{}{}
		}
		// Only require the verification echo when the corresponding input
		// finding actually carries a verification. Custom specs may dedupe
		// before verification (or over raw findings_from JSON); those inputs
		// have nothing to echo, and demanding one makes validation impossible.
		if _, inputVerified := verifiedInputIDs[id]; inputVerified {
			if finding.Verification == nil || strings.TrimSpace(finding.Verification.ID) != id {
				verificationMismatch++
			}
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
			AllowedIDs           []string
			UnknownIDValues      []string
			DuplicateIDs         int
			VerificationMismatch int
		}{
			CountTooLow:          countTooLow,
			CountTooHigh:         countTooHigh,
			InputCount:           inputCount,
			MinCount:             minCount,
			UnknownIDs:           unknownIDs,
			AllowedIDs:           allowedIDs,
			UnknownIDValues:      unknownIDValues,
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

// validateClusterMergeResponse checks a micro-merge response against its
// cluster: the output must keep between 1 and len(cluster) findings, every
// output finding must be attributable to a cluster finding (ID first, then
// code location with a title tiebreak), every output finding's id must be one
// of the cluster ids and appear at most once, and every cluster finding must
// be accounted for — surviving in the output, listed in an output finding's
// merged_from provenance, or content-matching an output finding. Absorbing a
// duplicate without touching the surviving finding's text is a valid merge,
// so unlike the old pairwise validator there is no "accumulator must change"
// heuristic — that heuristic deadlocked on exact duplicates.
//
// The merged_from accounting is deliberately lenient, matching the rest of
// the LLM output handling: entries are trimmed, unknown ids, duplicates, and
// a finding's own id are ignored rather than rejected — stray entries cannot
// fake coverage of a real input, so only genuinely lost findings fail.
func validateClusterMergeResponse(resp *llm.ReviewResponse, cluster []model.Finding) *llm.InvalidResponseError {
	if resp == nil {
		return &llm.InvalidResponseError{
			Reason:        "merge returned no response",
			MissingFields: []string{"findings"},
		}
	}
	inputIDs := make(map[string]struct{}, len(cluster))
	allowedIDs := make([]string, 0, len(cluster))
	for _, finding := range cluster {
		id := strings.TrimSpace(finding.ID)
		if id == "" {
			continue
		}
		inputIDs[id] = struct{}{}
		allowedIDs = append(allowedIDs, id)
	}
	var problems []string
	countMismatch := len(resp.Findings) < 1 || len(resp.Findings) > len(cluster)
	if countMismatch {
		problems = append(problems, fmt.Sprintf("count_mismatch got=%d min=1 max=%d", len(resp.Findings), len(cluster)))
	}
	unmatched := 0
	var unknownOutputIDs []string
	duplicateIDs := 0
	covered := make(map[string]struct{})
	seenOutputIDs := make(map[string]struct{}, len(resp.Findings))
	for i, finding := range resp.Findings {
		if findMergeInputMatch(finding, cluster) == nil {
			unmatched++
			problems = append(problems, fmt.Sprintf("unmatched_finding index=%d", i))
		}
		if id := strings.TrimSpace(finding.ID); id != "" {
			if _, ok := inputIDs[id]; !ok {
				unknownOutputIDs = append(unknownOutputIDs, id)
			}
			if _, ok := seenOutputIDs[id]; ok {
				duplicateIDs++
			}
			seenOutputIDs[id] = struct{}{}
			covered[id] = struct{}{}
		} else {
			unknownOutputIDs = append(unknownOutputIDs, "")
		}
		for _, src := range finding.MergedFrom {
			if src = strings.TrimSpace(src); src != "" {
				covered[src] = struct{}{}
			}
		}
	}
	if len(unknownOutputIDs) > 0 {
		problems = append(problems, fmt.Sprintf("unknown_ids count=%d", len(unknownOutputIDs)))
	}
	if duplicateIDs > 0 {
		problems = append(problems, fmt.Sprintf("duplicate_ids count=%d", duplicateIDs))
	}
	var droppedIDs []string
	var droppedTitles []string
	for _, in := range cluster {
		id := strings.TrimSpace(in.ID)
		if id != "" {
			if _, ok := covered[id]; ok {
				continue
			}
		}
		// Lenient fallback: a model that reminted the id but kept the finding
		// (or absorbed an exact duplicate without declaring provenance) still
		// accounts for the input via location+title attribution.
		if findMergeInputMatch(in, resp.Findings) != nil {
			continue
		}
		if id != "" {
			droppedIDs = append(droppedIDs, id)
		}
		droppedTitles = append(droppedTitles, in.Title)
	}
	if len(droppedTitles) > 0 {
		problems = append(problems, fmt.Sprintf("dropped_findings count=%d", len(droppedTitles)))
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
			MaxCount      int
			Unmatched     int
			AllowedIDs    []string
			UnknownIDs    []string
			Dropped       int
			DroppedIDs    []string
			DroppedTitles string
		}{
			CountMismatch: countMismatch,
			GotCount:      len(resp.Findings),
			MaxCount:      len(cluster),
			Unmatched:     unmatched,
			AllowedIDs:    allowedIDs,
			UnknownIDs:    unknownOutputIDs,
			Dropped:       len(droppedTitles),
			DroppedIDs:    droppedIDs,
			DroppedTitles: strings.Join(droppedTitles, "; "),
		},
	}
}

// repairClusterMergeProvenance patches the common case where a merge response
// correctly absorbs an input finding but forgets to list that input id in
// merged_from. It only repairs missing provenance when exactly one output is a
// strong semantic match; ambiguous or unrelated drops still fail validation and
// take the normal retry path.
func repairClusterMergeProvenance(resp *llm.ReviewResponse, cluster []model.Finding) int {
	if resp == nil || len(resp.Findings) < 1 || len(resp.Findings) > len(cluster) {
		return 0
	}
	covered := make(map[string]struct{})
	for _, finding := range resp.Findings {
		if id := strings.TrimSpace(finding.ID); id != "" {
			covered[id] = struct{}{}
		}
		for _, src := range finding.MergedFrom {
			if src = strings.TrimSpace(src); src != "" {
				covered[src] = struct{}{}
			}
		}
	}
	repaired := 0
	for _, in := range cluster {
		id := strings.TrimSpace(in.ID)
		if id == "" {
			continue
		}
		if _, ok := covered[id]; ok {
			continue
		}
		if findMergeInputMatch(in, resp.Findings) != nil {
			continue
		}
		match := -1
		for i := range resp.Findings {
			if findMergeInputMatch(resp.Findings[i], cluster) == nil {
				continue
			}
			if !mergeProvenanceRepairMatch(dedupe.Compare(in, resp.Findings[i])) {
				continue
			}
			if match >= 0 {
				match = -1
				break
			}
			match = i
		}
		if match < 0 {
			continue
		}
		resp.Findings[match].MergedFrom = append(resp.Findings[match].MergedFrom, id)
		covered[id] = struct{}{}
		repaired++
	}
	return repaired
}

func mergeProvenanceRepairMatch(match dedupe.Match) bool {
	return match.Verdict >= dedupe.Duplicate ||
		match.TitleSim >= dedupe.TitleStrong ||
		(match.TitleSim >= dedupe.TitleModerate && match.BodySim >= dedupe.BodyModerate) ||
		match.RootCauseSim >= dedupe.RootCauseStrong
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

func (e *Engine) renderContextSystem(template string, req model.ReviewRequest, styleGuides []model.StyleGuide, hasToolchainVersions bool) (string, error) {
	toolInstructions, err := e.renderToolInstructions(toolInstructionsConfig{
		agentRole:                "context",
		parallelToolCallGuidance: !req.DisableParallelToolCalls,
	})
	if err != nil {
		return "", err
	}
	styleGuideToolchainSnippet, err := e.renderStyleGuideToolchainSnippet("context", styleGuides, hasToolchainVersions)
	if err != nil {
		return "", err
	}
	systemPrompt, err := llm.RenderPrompt(template, struct {
		ToolInstructions           string
		StyleGuideToolchainSnippet string
	}{
		ToolInstructions:           toolInstructions,
		StyleGuideToolchainSnippet: strings.TrimSpace(styleGuideToolchainSnippet),
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
	start := time.Now()
	result, err := e.runAgentOnce(ctx, agent, req)
	if req.DisableSuggestions && result.resp != nil {
		model.StripSuggestions(result.resp.Findings)
	}
	// Reviewer sessions stamp their own runtime (anchored at session start);
	// every other role is timed here.
	if result.run.RuntimeSeconds == 0 {
		result.run.RuntimeSeconds = model.RuntimeSeconds(time.Since(start))
	}
	return result, err
}

func (e *Engine) runAgentOnce(ctx context.Context, agent agentSpec, req model.ReviewRequest) (agentResult, error) {
	if agent.role == "review" {
		s := e.newReviewerSession(agent, req, false)
		budget := newTimeBudgetStarter(ctx, nil, childTimePlan{}, false, "", nil)
		if err := e.reviewerInitial(ctx, s, req, budget, e, req); err != nil {
			return s.partialResult(req), err
		}
		if err := e.reviewerNudges(ctx, s, req, budget, e, req, budget, e, req); err != nil {
			// Return the session's accumulated result so telemetry and the
			// findings gathered before the failure survive alongside the error.
			return s.result(req), err
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
	raw := strings.Join(messages, "\n")
	kept := make([]string, 0, strings.Count(raw, "\n")+1)
	for line := range strings.SplitSeq(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		// Strip a leading bullet/number prefix before the NONE check so
		// bulleted placeholders like "+ NONE" are also dropped.
		bare := strings.TrimSpace(reasoningBulletPrefix.ReplaceAllString(trimmed, ""))
		if strings.EqualFold(bare, "NONE") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
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
	outputSchemaSnippet := exampleSnippetFor(llm.SchemaKindReview, req.DisableSuggestions)
	commonSnippets, err := agentCommonSystemPromptSnippetsForTools(agentRole, outputSchemaSnippet, req.DisableSuggestions, hasTools)
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
		MaxFindings                int
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
		MaxFindings:                req.MaxFindings,
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
		// Errored tool results (status:error payloads, including
		// already_requested duplicates) carry no reviewable content; embedding
		// them as supplemental context would only waste prompt budget.
		if parseToolResultSummary(msg.Content).IsError {
			continue
		}
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

func exampleSnippetFor(kind llm.SchemaKind, disableSuggestions bool) string {
	switch kind {
	case llm.SchemaKindVerify:
		return llm.VerifyExamplePromptSnippet()
	case llm.SchemaKindMerge:
		return llm.MergeExamplePromptSnippetFor(disableSuggestions)
	case llm.SchemaKindFinalize:
		return llm.FinalizeExamplePromptSnippetFor(disableSuggestions)
	case llm.SchemaKindVerdict:
		return llm.VerdictExamplePromptSnippet()
	case llm.SchemaKindSummarize:
		return llm.SummarizeExamplePromptSnippet()
	case llm.SchemaKindReview:
		return llm.FindingsExamplePromptSnippetFor(disableSuggestions)
	default:
		return ""
	}
}

func noToolsMessages(agentRole string, systemTemplate string, messages []llm.Message, snippet string, styleGuideToolchainSnippet string, disableSuggestions bool) ([]llm.Message, error) {
	commonSnippets, err := agentCommonSystemPromptSnippetsForTools(agentRole, snippet, disableSuggestions, false)
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
		Reason            string
		MissingFields     string
		Guidance          string
		ExampleSnippet    string
		ValidationFailure bool
	}{
		Reason:            invalid.Reason,
		MissingFields:     strings.Join(invalid.MissingFields, ", "),
		Guidance:          guidance,
		ExampleSnippet:    strings.TrimSpace(exampleSnippet),
		ValidationFailure: invalid.ValidationFailure,
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

func agentCommonSystemPromptSnippet(agentRole string, snippet string, outputSchemaSnippet string, disableSuggestions bool, hasTools bool) (string, error) {
	rendered, err := renderPromptFile("agent_common_system_prompt_snippet.tmpl", struct {
		AgentRole           string
		Snippet             string
		OutputSchemaSnippet string
		DisableSuggestions  bool
		HasTools            bool
	}{
		AgentRole:           agentRole,
		Snippet:             snippet,
		OutputSchemaSnippet: outputSchemaSnippet,
		DisableSuggestions:  disableSuggestions,
		HasTools:            hasTools,
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

func agentCommonSystemPromptSnippets(agentRole string, outputSchemaSnippet string, disableSuggestions bool) (agentCommonSystemPromptSnippetSet, error) {
	return agentCommonSystemPromptSnippetsForTools(agentRole, outputSchemaSnippet, disableSuggestions, true)
}

func agentCommonSystemPromptSnippetsForTools(agentRole string, outputSchemaSnippet string, disableSuggestions bool, hasTools bool) (agentCommonSystemPromptSnippetSet, error) {
	findingInstructions, err := agentCommonSystemPromptSnippet(agentRole, "findings", "", disableSuggestions, hasTools)
	if err != nil {
		return agentCommonSystemPromptSnippetSet{}, err
	}
	priority, err := agentCommonSystemPromptSnippet(agentRole, "priority", "", disableSuggestions, hasTools)
	if err != nil {
		return agentCommonSystemPromptSnippetSet{}, err
	}
	var outputFormat string
	if outputSchemaSnippet != "" {
		outputFormat, err = agentCommonSystemPromptSnippet(agentRole, "output_format", outputSchemaSnippet, disableSuggestions, hasTools)
		if err != nil {
			return agentCommonSystemPromptSnippetSet{}, err
		}
	}
	return agentCommonSystemPromptSnippetSet{
		findingInstructions: findingInstructions,
		priority:            priority,
		outputFormat:        outputFormat,
	}, nil
}

func (e *Engine) styleGuidesFor(ctx *model.ReviewContext) ([]model.StyleGuide, error) {
	languages := changedLanguages(ctx)
	changed := make(map[string]struct{}, len(languages))
	for _, language := range languages {
		changed[language] = struct{}{}
	}
	guides := make([]model.StyleGuide, 0, len(languages)+len(e.additionalStyleGuides))
	seenFiles := make(map[string]struct{})
	for _, language := range languages {
		if _, off := e.disabledStyleGuides[language]; off {
			continue
		}
		name, ok := mappings.StyleGuideFile(language, detectedVersionsFor(ctx, language))
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
		})
	}
	guides = append(guides, e.gatedAdditionalStyleGuides(ctx, changed)...)
	return guides, nil
}

// gatedAdditionalStyleGuides selects which user-supplied guides apply to this
// review, preserving their configured order. An ungated guide always applies;
// a language-gated guide applies when that language changed; a version-gated
// guide applies only for the version that wins its language (most
// authoritative source, lowest version within it — matching the built-in
// selection rule).
func (e *Engine) gatedAdditionalStyleGuides(ctx *model.ReviewContext, changed map[string]struct{}) []model.StyleGuide {
	if len(e.additionalStyleGuides) == 0 {
		return nil
	}
	// Resolve the winning version per language for version-gated guides.
	versionKeysByLang := make(map[string][]string)
	for _, g := range e.additionalStyleGuides {
		if g.GateLanguage != "" && g.GateVersion != "" {
			versionKeysByLang[g.GateLanguage] = append(versionKeysByLang[g.GateLanguage], g.GateVersion)
		}
	}
	winningByLang := make(map[string]string, len(versionKeysByLang))
	for language, keys := range versionKeysByLang {
		if _, ok := changed[language]; !ok {
			continue
		}
		if key, matched := versionmatch.SelectLowest(detectedVersionsFor(ctx, language), keys); matched {
			winningByLang[language] = key
		}
	}
	out := make([]model.StyleGuide, 0, len(e.additionalStyleGuides))
	for _, g := range e.additionalStyleGuides {
		switch {
		case g.GateLanguage == "":
			// unconditional (back-compat scalar spec)
		case g.GateVersion == "":
			if _, ok := changed[g.GateLanguage]; !ok {
				continue
			}
		default:
			if winningByLang[g.GateLanguage] != g.GateVersion {
				continue
			}
		}
		out = append(out, g.StyleGuide)
	}
	return out
}

// detectedVersionsFor returns the usable toolchain versions detected for a
// language, skipping Unavailable/Error/empty entries. A language can carry
// several (go.mod go directive, toolchain directive, Dockerfile, CI); only the
// most authoritative source tier (mappings.VersionSourceRank, e.g. go.mod over
// Dockerfile for Go) is returned, so a stale lower-priority source cannot
// override the version the code is actually built against. Within that tier
// the selection rules pick the lowest version. A source whose entry errored
// (e.g. an unparseable go.mod) yields nothing, letting the next tier take
// over.
func detectedVersionsFor(ctx *model.ReviewContext, language string) []string {
	if ctx == nil {
		return nil
	}
	bestRank := 0
	var out []string
	for _, tv := range ctx.ToolchainVersions {
		if tv.Language != language || tv.Unavailable || tv.Error != "" {
			continue
		}
		version := strings.TrimSpace(tv.Version)
		if version == "" {
			continue
		}
		rank := mappings.VersionSourceRank(language, tv.Source)
		switch {
		case len(out) == 0 || rank < bestRank:
			bestRank = rank
			out = append(out[:0], version)
		case rank == bestRank:
			out = append(out, version)
		}
	}
	return out
}

// mergeStyleGuides returns the styleguides for merge prompts. A source-less
// merge workflow (e.g. --step merge --findings a.json) never runs
// ensurePrompts, so st.styleGuides stays unset; fall back to resolving
// directly — a nil context yields no language guides but still carries the
// user-supplied additional guides.
func (e *Engine) mergeStyleGuides(st *PipelineState) ([]model.StyleGuide, error) {
	st.mu.Lock()
	ready, guides, enriched := st.promptsReady, st.styleGuides, st.Enriched
	st.mu.Unlock()
	if ready {
		return guides, nil
	}
	return e.styleGuidesFor(enriched)
}

func (e *Engine) renderStyleGuideToolchainSnippet(agentRole string, guides []model.StyleGuide, hasToolchainVersions bool) (string, error) {
	agentRole = strings.TrimSpace(agentRole)
	if len(guides) == 0 && !hasToolchainVersions {
		return "", nil
	}
	template, err := e.loadPrompt("agent_styleguide_toolchain_snippet.tmpl")
	if err != nil {
		return "", err
	}
	rendered, err := llm.RenderPrompt(template, struct {
		AgentRole            string
		StyleGuides          []model.StyleGuide
		HasToolchainVersions bool
	}{
		AgentRole:            agentRole,
		StyleGuides:          guides,
		HasToolchainVersions: hasToolchainVersions,
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering styleguide/toolchain prompt: %w", err)
	}
	return strings.TrimSpace(rendered), nil
}

func changedLanguages(ctx *model.ReviewContext) []string {
	if ctx == nil {
		return nil
	}
	seen := make(map[string]struct{})
	for _, file := range ctx.DiffFiles {
		language := styleGuideLanguageForPath(file.FilePath)
		if language == "" {
			language = file.Language
		}
		addLanguage(seen, language)
		addDetectorLanguages(seen, file.FilePath, file.Content)
	}
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
	if !mappings.HasStyleGuide(language) {
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

func filterByDisplayPriority(findings []model.Finding, threshold string) []model.Finding {
	maxPriority := model.PriorityThresholdRank(threshold)
	filtered := make([]model.Finding, 0, len(findings))
	for _, finding := range findings {
		if displayPriorityRank(finding) <= maxPriority {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

func displayPriorityRank(finding model.Finding) int {
	if finding.Summarization != nil {
		priority := finding.Summarization.Priority
		return model.PriorityRank(&priority)
	}
	if finding.Finalization != nil {
		priority := finding.Finalization.Priority
		return model.PriorityRank(&priority)
	}
	return model.PriorityRank(finding.Priority)
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

// agentLoopKind maps an agentSpec role to the loop kind. Roles are uniform
// identifiers (context, review, verify, dedupe, merge, finalize, verdict,
// summarize, extract), so this is the identity today; it stays as the seam where
// a role would diverge from its loop kind.
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
	IsError        bool
	Code           string
	Message        string
	Lines          int
	Files          int
	ResultCount    int
	HasResultCount bool
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
		summary.HasResultCount = true
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
		summary.HasResultCount = true
		summary.ResultCount = int(resultCount)
	}
	if matchCount, ok := payload["match_count"].(float64); ok {
		summary.HasResultCount = true
		summary.ResultCount = int(matchCount)
	}
	if matches, ok := payload["matches"].([]any); ok {
		distinct := make(map[string]struct{}, len(matches))
		for _, item := range matches {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			loc, ok := entry["code_location"].(map[string]any)
			if !ok {
				continue
			}
			if path, _ := loc["file_path"].(string); path != "" {
				distinct[path] = struct{}{}
			}
		}
		summary.Files = len(distinct)
	}
	if code, ok := payload["code"].(string); ok && code != "" {
		summary.Lines = retrieval.FindLinesCount(code)
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
// (inspect_file, find_lines, list_files, search, find_callers, find_callees). A single
// named type replaces the 9-field anonymous struct that was previously
// re-declared verbatim at several call sites.
type toolCallArgs struct {
	Path          string `json:"path"`
	Code          string `json:"code"`
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
	case "find_lines":
		parts = append(parts, fmt.Sprintf("path=%q", syntheticPathValue(args.Path, "<path>")))
		parts = append(parts, fmt.Sprintf("code_line_count=%d", retrieval.FindLinesCount(args.Code)))
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
	if result.HasResultCount || result.ResultCount > 0 {
		parts = append(parts, fmt.Sprintf("result_count=%d", result.ResultCount))
	}
	if len(parts) == 0 {
		if toolName == "search" || toolName == "find_lines" {
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
	resp, err := e.reviewWithTimeBudget(ctx, req)
	elapsed := time.Since(start).Truncate(time.Second)
	if ok && e.logger != nil {
		if resp != nil && resp.Reasoned {
			e.logger.ProgressFor(turnInfo, logging.StageReasoning, logging.StateDone, elapsed.String())
		}
		e.logger.ProgressFor(turnInfo, logging.StageResponse, logging.StateDone, elapsed.String())
	}
	return resp, err
}

func (e *Engine) reviewWithTimeBudget(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	if timeBudgetUrgentNow(ctx) && !req.Urgent {
		urgentReq := *req
		urgentReq.Urgent = true
		e.logTimeBudgetUrgentNow(ctx)
		return e.reviewLLMWithTimeBudgetLog(ctx, &urgentReq)
	}
	softDeadline, ok := timeBudgetSpeedupDeadline(ctx)
	if !ok || req.Urgent {
		return e.reviewLLMWithTimeBudgetLog(ctx, req)
	}
	softCtx, cancel := context.WithDeadline(ctx, softDeadline)
	resp, err := e.llm.Review(softCtx, req)
	softErr := softCtx.Err()
	cancel()
	if err == nil {
		return resp, nil
	}
	if softErr != context.DeadlineExceeded || ctx.Err() != nil {
		e.logTimeBudgetDeadlineIfExpired(ctx)
		return resp, err
	}
	urgentReq := *req
	urgentReq.Urgent = true
	e.logTimeBudgetRetry(ctx, err, softErr)
	return e.reviewLLMWithTimeBudgetLog(ctx, &urgentReq)
}

func (e *Engine) reviewLLMWithTimeBudgetLog(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	resp, err := e.llm.Review(ctx, req)
	if err != nil {
		e.logTimeBudgetDeadlineIfExpired(ctx)
	}
	return resp, err
}

func (e *Engine) logTimeBudgetUrgentNow(ctx context.Context) {
	budget, ok := timeBudgetFromContext(ctx)
	if !ok {
		e.logf(ctx, "Workflow time budget speed-up threshold already reached; sending urgent request")
		return
	}
	now := time.Now()
	e.logf(ctx, "Workflow time budget speed-up threshold already reached: scope=%s elapsed=%s limit=%s; sending urgent request",
		budget.scope, budgetDuration(timeBudgetElapsed(budget, now)), budgetDuration(timeBudgetLimit(budget)))
}

func (e *Engine) logTimeBudgetRetry(ctx context.Context, firstErr error, softErr error) {
	budget, ok := timeBudgetFromContext(ctx)
	if !ok {
		e.logf(ctx, "Workflow time budget speed-up threshold reached; retrying urgently first_error=%v soft_err=%v", firstErr, softErr)
		return
	}
	now := time.Now()
	e.logf(ctx, "Workflow time budget speed-up threshold reached: scope=%s elapsed=%s limit=%s remaining=%s; retrying urgently first_error=%v soft_err=%v",
		budget.scope, budgetDuration(timeBudgetElapsed(budget, now)), budgetDuration(timeBudgetLimit(budget)), budgetDuration(timeBudgetRemaining(budget, now)), firstErr, softErr)
}

func (e *Engine) logTimeBudgetDeadlineIfExpired(ctx context.Context) {
	if ctx.Err() == nil {
		return
	}
	budget, ok := timeBudgetFromContext(ctx)
	if !ok {
		return
	}
	now := time.Now()
	if budget.deadline.After(now) {
		return
	}
	e.logf(ctx, "Workflow time budget deadline reached: scope=%s elapsed=%s limit=%s overrun=%s; call aborted",
		budget.scope, budgetDuration(timeBudgetElapsed(budget, now)), budgetDuration(timeBudgetLimit(budget)), budgetDuration(timeBudgetOverrun(budget, now)))
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
