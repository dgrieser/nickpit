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
	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/textsan"
	"github.com/dgrieser/nickpit/internal/tokenestimate"
	"github.com/dgrieser/nickpit/internal/toolchain"
	"github.com/dgrieser/nickpit/internal/toollimits"
	toolcatalog "github.com/dgrieser/nickpit/internal/tools"
	"github.com/dgrieser/nickpit/internal/versionmatch"
	"github.com/dgrieser/nickpit/mappings"
	"github.com/dgrieser/nickpit/prompts"
	"github.com/google/uuid"
)

type Engine struct {
	source model.ReviewSource
	// llm is the client for THIS engine's profile. Clones made by withConfig
	// re-resolve it from clients, so the client and the model parameters of a
	// step always come from the same profile.
	llm llm.Client
	// clients resolves a profile's endpoint to its client. It holds the primary
	// endpoint and, when the profile's small model lives elsewhere, the small one.
	// Written once before the pipeline runs and read-only afterwards, like
	// additionalStyleGuides below.
	clients                *llm.ClientSet
	retrieval              retrieval.Engine
	history                git.History
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
}

// Both patterns count `$` as an identifier character, matching retrieval's
// isIdentifierByte: extracting `store` out of a matched `$store` requested
// references for a symbol the line never contains.
var (
	searchIdentifierQueryPattern = regexp.MustCompile(`^([A-Za-z_$][A-Za-z0-9_$]*)(?:\((?:\))?)?$`)
	searchIdentifierPattern      = regexp.MustCompile(`[A-Za-z_$][A-Za-z0-9_$]*`)
)

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
	// reservedToolCalls records which tool call took each dedup key, so a
	// result the context limiter later empties can hand its keys back. Without
	// it a symbol whose analysis was reduced to a bare truncation marker stayed
	// reserved, and every re-request the marker invites answered
	// already_requested.
	//
	// Keys are owned per (round, tool call ID), not per ID alone: provider IDs
	// are only unique within one response — the XML fallback synthesizes
	// xml_tool_call_1, xml_tool_call_2, … afresh each round — and releasing by
	// bare ID handed back keys an earlier round's identically-named call took.
	reservedToolCalls map[string][]string
	// round distinguishes reservation owners across tool rounds. beginToolRound
	// advances it before each batch; the calls inside one batch run
	// concurrently but never change it.
	round     int
	fileLocks keyedLocker
	toolLocks keyedLocker
}

// beginToolRound opens a new reservation scope for the next tool batch.
func (s *toolRoundState) beginToolRound() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.round++
}

// reservationOwnerLocked names the owner of a reservation: the tool call ID
// qualified by the round it ran in. Callers hold s.mu.
func (s *toolRoundState) reservationOwnerLocked(toolCallID string) string {
	return fmt.Sprintf("%d\x00%s", s.round, toolCallID)
}

// reserveToolCall records key as answered by toolCallID, reporting false when
// another call already holds it. The bookkeeping map is created here rather
// than at construction so a state assembled without it still records.
func (s *toolRoundState) reserveToolCall(toolCallID, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, duplicate := s.seenToolCalls[key]; duplicate {
		return false
	}
	s.seenToolCalls[key] = struct{}{}
	if toolCallID != "" {
		if s.reservedToolCalls == nil {
			s.reservedToolCalls = map[string][]string{}
		}
		owner := s.reservationOwnerLocked(toolCallID)
		s.reservedToolCalls[owner] = append(s.reservedToolCalls[owner], key)
	}
	return true
}

// markToolCallSeen is reserveToolCall for the callers that already checked the
// key and are recording the answer they produced.
func (s *toolRoundState) markToolCallSeen(toolCallID, key string) {
	_ = s.reserveToolCall(toolCallID, key)
}

// releaseToolCall drops the dedup keys a tool call of the current round took,
// making the same request answerable again. Earlier rounds' reservations are
// out of reach by construction: a later call whose provider ID happens to
// collide must not hand back keys it never took.
func (s *toolRoundState) releaseToolCall(toolCallID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := s.reservationOwnerLocked(toolCallID)
	for _, key := range s.reservedToolCalls[owner] {
		delete(s.seenToolCalls, key)
	}
	delete(s.reservedToolCalls, owner)
}

func NewEngine(source model.ReviewSource, llmClient llm.Client, retrievalEngine retrieval.Engine, profile config.Profile) *Engine {
	return &Engine{
		source: source,
		llm:    llmClient,
		// The profile names the endpoint this client talks to, so the resolver can
		// be built here; SetSmallClient adds a second endpoint when the profile's
		// small model lives on another host.
		clients:   llm.NewClientSet(llmClient, llm.NewEndpoint(profile.BaseURL, profile.APIKey)),
		retrieval: retrievalEngine,
		// The history tools read the same checkout the retrieval engine reads,
		// but through git; the profile tokens let a shallow remote checkout be
		// deepened on first use.
		history: git.NewExecHistory(git.HistoryAuth{
			GitHubToken:   profile.GitHubToken,
			GitLabToken:   profile.GitLabToken,
			GitLabBaseURL: profile.GitLabBaseURL,
		}),
		config:                 profile,
		searchToolOptimization: true,
		toolchainCapture:       toolchain.Capture,
	}
}

// SetSmallClient registers the client for the profile's small model endpoint, so
// steps resolved to model: "@small" run against it instead of the primary one.
// Pass the small profile that EffectiveSmallProfile produced: its base URL and
// api_key identify the endpoint. Registering an endpoint identical to the primary
// one is a no-op, so callers need not check first.
//
// Must be called before the pipeline runs: the resolver is read-only once
// concurrent steps start.
func (e *Engine) SetSmallClient(client llm.Client, smallProfile config.Profile) {
	e.clients = e.clients.With(llm.NewEndpoint(smallProfile.BaseURL, smallProfile.APIKey), client)
}

// SetHistory overrides the commit-history provider. Intended for tests;
// production code uses the git-backed provider built in NewEngine.
func (e *Engine) SetHistory(history git.History) {
	e.history = history
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

// PrepareContext resolves and prepares a review context exactly as a review does:
// it applies the request's path/content filters, generated-file stamping,
// toolchain capture, optional full-file inlining, and context-budget trimming.
// The discussion agent uses this so a chat sees the same, filtered, trimmed
// context the reviewers saw — never files the review deliberately withheld, and
// never a patch larger than the model context budget.
func (e *Engine) PrepareContext(ctx context.Context, req model.ReviewRequest) (*model.ReviewContext, error) {
	// Labeled as chat: this path is only used by the discussion agent, and
	// "Starting review" progress from a chat command would mislead.
	return e.resolveAndTrimContextAs(ctx, req, "chat", logging.StageChat)
}

// resolveAndTrimContext resolves the review source, captures toolchain versions,
// optionally inlines full files, and trims to the context budget.
func (e *Engine) resolveAndTrimContext(ctx context.Context, req model.ReviewRequest) (*model.ReviewContext, error) {
	return e.resolveAndTrimContextAs(ctx, req, "review", logging.StageReview)
}

// resolveAndTrimContextAs is resolveAndTrimContext with the operation label and
// progress stage of the caller (review pipeline vs discussion agent).
func (e *Engine) resolveAndTrimContextAs(ctx context.Context, req model.ReviewRequest, operation string, stage logging.Stage) (*model.ReviewContext, error) {
	e.logf(ctx, "Starting %s: mode=%s repo=%s id=%d submode=%s repo_root=%s", operation, req.Mode, req.Repo, req.Identifier, req.Submode, req.RepoRoot)
	contextFilter, err := newReviewContextFilter(req)
	if err != nil {
		return nil, err
	}
	reviewCtx, err := e.source.ResolveContext(ctx, req)
	if err != nil {
		return nil, err
	}
	e.logProgress(stage, logging.StateStart, reviewContextSummary(reviewCtx, req))
	if e.logger != nil {
		// Show the same resolved repo/branch the show-progress context line uses.
		e.logger.SetLiveTarget(reviewTargetSummary(reviewCtx))
	}
	e.logf(ctx, "Resolved context: title=%q files=%d commits=%d comments=%d diff_bytes=%d", reviewCtx.Title, len(reviewCtx.ChangedFiles), len(reviewCtx.Commits), len(reviewCtx.Comments), len(reviewCtx.Diff))
	if len(reviewCtx.ChangedFiles) == 0 && len(reviewCtx.Diff) == 0 {
		return nil, ErrEmptyDiff
	}
	reviewCtx.CheckoutRoot = req.RepoRoot
	reviewCtx.Identifier = req.Identifier
	stampGeneratedFlags(reviewCtx)
	stampSymlinkFlags(ctx, reviewCtx, git.ExecRunner{RepoRoot: reviewCtx.CheckoutRoot})
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
		e.appendFullFiles(ctx, reviewCtx, req.RepoRoot)
	}

	// Scope checks must use the complete filtered diff, not the prompt-budget
	// view produced by the trimmer below. Use append with a non-nil empty base so
	// an empty hunk list still records that a source diff was resolved.
	diffScopeHunks := append([]model.DiffHunk{}, reviewCtx.DiffHunks...)

	trimmer := e.trimmer
	if trimmer == nil {
		maxTokens := req.MaxContextTokens
		if maxTokens <= 0 {
			maxTokens = config.DefaultMaxContextToken
		}
		estimator := tokenestimate.SimpleEstimator{}
		headroom := promptOverheadTokens(estimator, reviewCtx, req.DiffFormat, maxTokens)
		trimmer = NewTrimmer(maxTokens, estimator, WithHeadroomTokens(headroom))
	}

	trimmed, err := trimmer.Trim(reviewCtx)
	if err != nil {
		return nil, fmt.Errorf("review: trim context: %w", err)
	}
	trimmed.DiffScopeHunks = diffScopeHunks
	e.logf(ctx, "Trimmed context: files=%d supplemental=%d omitted=%d budget=%d", len(trimmed.ChangedFiles), len(trimmed.SupplementalContext), len(trimmed.OmittedSections), req.MaxContextTokens)
	return trimmed, nil
}

func (e *Engine) applyResultMetadata(result *model.ReviewResult, req model.ReviewRequest, reviewCtx *model.ReviewContext) {
	if result.ReviewID == "" {
		result.ReviewID = uuid.NewString()
	}
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now().UTC()
	}
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
	if loopReq.JSONRetryExampleSnippet != "" {
		exampleSnippet = loopReq.JSONRetryExampleSnippet
	}
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
		if !errors.As(err, &invalidResp) {
			return nil, err
		}
		if !outputRetriesRemaining(attempt, maxOutputRetries) {
			// Retry budget exhausted: salvage a partial parsed response the same
			// way the main agent loop does instead of discarding it with the error.
			if invalidResp.PartialResponse != nil {
				e.logf(ctx, "Invalid JSON response after retries exhausted; using partial parsed response: reason=%q missing=%v", invalidResp.Reason, invalidResp.MissingFields)
				return invalidResp.PartialResponse, nil
			}
			return nil, err
		}
		if invalidResp.ReasoningEffort != "" {
			llmReq.ReasoningEffort = invalidResp.ReasoningEffort
		}
		e.logf(ctx, "Invalid JSON response in no-tools call, retrying: attempt=%d reason=%q missing=%v", attempt+1, invalidResp.Reason, invalidResp.MissingFields)
		e.logProgress(logging.StageModel, logging.StateRetry, fmt.Sprintf("invalid JSON, attempt=%d", attempt+1))
		if strings.TrimSpace(invalidResp.RawContent) != "" {
			llmReq.Messages = append(llmReq.Messages, llm.Message{Role: "assistant", Content: invalidResp.RawContent})
		} else {
			// Keep the history alternating for strict-role providers when the
			// invalid response carried no appendable content — the duplicate-tool-
			// call-limit path arrives here with a trailing user turn (the rewritten
			// tool results), so skipping the assistant turn entirely would produce
			// two consecutive user messages.
			llmReq.Messages = append(llmReq.Messages, llm.Message{Role: "assistant", Content: "[invalid response]"})
		}
		feedback, err := e.renderJSONRetryFeedback(invalidResp, exampleSnippet)
		if err != nil {
			return nil, err
		}
		llmReq.Messages = append(llmReq.Messages, llm.Message{Role: "user", Content: feedback})
	}
}

type agentSpec struct {
	name string
	// progressName overrides the name shown in progress output (the live bar and
	// reasoning banner) without changing the telemetry run name — used to give
	// per-cluster synthesis shards distinct bars, e.g. "Finalize #2".
	progressName     string
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
	maxFindings       int
	enforceDiffScope  bool
	allowedDiffScopes []model.CodeLocation
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
	run             model.AgentRun
	reasoningEffort string
	contentMessages []string
	toolMessages    []llm.Message
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
// All reference fields (retrieval, logger, source, trimmer, the endpoint→client
// resolver) are shared; only the value-type config and the client resolved from
// it differ. Used to apply per-step model parameter overrides without mutating
// the shared engine or racing concurrent steps, since the clone's config is
// read-only for the lifetime of a step.
//
// Resolving the client here — rather than at the call site — is what keeps a
// step's client and its model parameters in sync: both come from this clone's
// profile, so a step routed to model: "@small" cannot end up sending the small
// model's name to the primary endpoint. Profiles whose endpoint is not
// registered (every profile in a single-endpoint run) resolve to the primary
// client, so there is one code path either way.
func (e *Engine) withConfig(profile config.Profile) *Engine {
	clone := *e
	clone.config = profile
	clone.llm = e.clients.For(llm.NewEndpoint(profile.BaseURL, profile.APIKey))
	return &clone
}

// findingRef locates a flattened finding back in its vector's response so the
// per-finding filters can write survivors back in place.
type findingRef struct {
	vectorIdx  int
	findingIdx int
}

// flattenVectorFindings collects every finding from the vector results that are
// eligible for a per-finding agent pass, together with the position each came
// from. Failed or empty reviewers contribute nothing.
func flattenVectorFindings(vectorResults []agentResult) ([]model.Finding, []findingRef) {
	findings := make([]model.Finding, 0)
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
	return findings, refs
}

// categorizeAndFilterVectorFindings performs the private classification half of
// verification. The model only emits descriptive labels; Go applies the drop
// policy without exposing its routing consequences to either model.
func (e *Engine) categorizeAndFilterVectorFindings(ctx context.Context, reviewCtx *model.ReviewContext, vectorResults []agentResult, req model.ReviewRequest, limiter *Limiter, reviewerName string) (model.TokenUsage, []string, error) {
	findings, refs := flattenVectorFindings(vectorResults)
	if len(findings) == 0 {
		return model.TokenUsage{}, nil, nil
	}
	if overwrote := model.EnsureFindingIDs(findings); overwrote > 0 {
		e.logf(ctx, "Review generated replacement IDs before categorization: count=%d", overwrote)
	}
	opts := categorizeOptionsFromReviewRequest(req)
	opts.Limiter = limiter
	opts.ReviewerName = reviewerName
	categorizeResults, usage, warnings, err := e.categorizeAll(ctx, reviewCtx, findings, opts)
	if err != nil {
		return usage, warnings, err
	}
	if len(categorizeResults) != len(refs) {
		return usage, warnings, fmt.Errorf("review: categorizer returned %d results for %d findings", len(categorizeResults), len(refs))
	}
	type dropCounts struct {
		confirmation int
		compilation  int
		unverified   int
	}
	keptByVector := make(map[int][]model.Finding, len(vectorResults))
	droppedByVector := make(map[int]int, len(vectorResults))
	dropsByVector := make(map[int]dropCounts, len(vectorResults))
	for i, result := range categorizeResults {
		ref := refs[i]
		finding := vectorResults[ref.vectorIdx].resp.Findings[ref.findingIdx]
		// findings[i].ID holds the normalized ID after EnsureFindingIDs above,
		// which may have replaced an invalid or duplicate reviewer ID. Always
		// adopt it so corrected IDs survive into verification and downstream
		// dedupe/merge validation.
		finding.ID = findings[i].ID
		categorization := result.Categorization
		if categorization == nil {
			categorization = fallbackFindingCategorization(finding)
		}
		categories := effectiveFindingCategories(categorization)
		counts := dropsByVector[ref.vectorIdx]
		dropCategorized, reason := shouldDropCategories(categories, opts.DropPolicy)
		if dropCategorized {
			if e.logger != nil {
				e.logger.ProgressFor(
					e.progressInfo("categorize", categorizeProgressName(reviewerName, i), truncateFindingTitle(finding.Title)),
					logging.StageCategorize,
					logging.StateSkip,
					fmt.Sprintf("dropped categories=%s reason=%s policy=%s", strings.Join(categories, ","), reason, model.NormalizeDropPolicy(opts.DropPolicy)),
				)
			}
			switch {
			case reason == model.VerdictUnverified:
				counts.unverified++
			case slices.Contains(categories, model.CategoryConfirmation):
				counts.confirmation++
			default:
				counts.compilation++
			}
			droppedByVector[ref.vectorIdx]++
			dropsByVector[ref.vectorIdx] = counts
			continue
		}
		dropsByVector[ref.vectorIdx] = counts
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
		dropped := droppedByVector[vectorIdx]
		counts := dropsByVector[vectorIdx]
		if e.logger != nil {
			// Categorize drops are not verdicts, so they count as filtered
			// rather than refuted.
			e.logger.LiveFindings(logging.FindingUpdate{Filtered: dropped})
		}
		if dropped > 0 {
			e.logf(ctx, "Categorize filter before verify: reviewer=%s dropped=%d confirmation=%d compilation=%d unverified=%d kept=%d policy=%s",
				vectorResults[vectorIdx].run.Name,
				dropped,
				counts.confirmation,
				counts.compilation,
				counts.unverified,
				len(keptByVector[vectorIdx]),
				model.NormalizeDropPolicy(opts.DropPolicy),
			)
		}
	}
	return usage, warnings, nil
}

func shouldDropCategories(categories []string, policy string) (bool, string) {
	if len(categories) == 0 {
		return false, "kept"
	}
	policy = model.NormalizeDropPolicy(policy)
	if policy == model.DropPolicyNone {
		return false, "kept"
	}
	hasFinding := slices.Contains(categories, model.CategoryFinding)
	if !hasFinding {
		return true, model.VerdictRefuted
	}
	if len(categories) > 1 && policy == model.DropPolicyRefutedAndUnverified {
		return true, model.VerdictUnverified
	}
	return false, "kept"
}

type verificationTelemetry struct {
	CategorizeUsage model.TokenUsage
	VerifyUsage     model.TokenUsage
	VerifyToolCalls int
}

// verifyAndFilterVectorFindings is the atomic workflow operation: deterministic
// scope handling, blind classification and routing, then blind truth
// verification for the survivors.
// categorize carries the classifier's own engine clone and request, which
// differ from the verifier's when the verify step configures a categorize
// override (e.g. model: "@small"); the zero value means "same as the verifier".
func (e *Engine) verifyAndFilterVectorFindings(ctx context.Context, reviewCtx *model.ReviewContext, vectorResults []agentResult, req model.ReviewRequest, limiter *Limiter, reviewerName string, categorize internalAgentContext) (verificationTelemetry, []string, error) {
	telemetry := verificationTelemetry{}
	categorizeEngine, categorizeReq := e, req
	if categorize.Engine != nil {
		categorizeEngine, categorizeReq = categorize.Engine, categorize.Req
	}
	scopeWarnings := e.prepareFindingsForVerification(ctx, reviewCtx, vectorResults, req)
	categorizeUsage, categorizeWarnings, err := categorizeEngine.categorizeAndFilterVectorFindings(ctx, reviewCtx, vectorResults, categorizeReq, limiter, reviewerName)
	telemetry.CategorizeUsage = categorizeUsage
	warnings := append(scopeWarnings, categorizeWarnings...)
	if err != nil {
		return telemetry, warnings, err
	}

	findings, refs := flattenVectorFindings(vectorResults)
	if len(findings) == 0 {
		return telemetry, warnings, nil
	}
	if overwrote := model.EnsureFindingIDs(findings); overwrote > 0 {
		e.logf(ctx, "Review generated replacement IDs before verification: count=%d", overwrote)
	}
	opts := verifyOptionsFromReviewRequest(req)
	opts.Limiter = limiter
	opts.ReviewerName = reviewerName
	verifyResults, usage, verifyCalls, verifyWarnings, err := e.verifyAll(ctx, reviewCtx, findings, opts)
	telemetry.VerifyUsage = usage
	telemetry.VerifyToolCalls = verifyCalls
	warnings = append(warnings, verifyWarnings...)
	if err != nil {
		return telemetry, warnings, err
	}
	if len(verifyResults) != len(refs) {
		return telemetry, warnings, fmt.Errorf("review: verifier returned %d results for %d findings", len(verifyResults), len(refs))
	}
	type dropCounts struct {
		refuted    int
		unverified int
	}
	keptByVector := make(map[int][]model.Finding, len(vectorResults))
	droppedIdxByVector := make(map[int]map[int]struct{}, len(vectorResults))
	dropsByVector := make(map[int]dropCounts, len(vectorResults))
	for i, verifyResult := range verifyResults {
		verification := verifyResult.Verification
		ref := refs[i]
		finding := vectorResults[ref.vectorIdx].resp.Findings[ref.findingIdx]
		// findings[i].ID holds the normalized ID after EnsureFindingIDs above,
		// which may have replaced an invalid or duplicate reviewer ID. Always
		// adopt it so corrected IDs survive into downstream dedupe/merge
		// validation and stay in sync with Verification.ID.
		finding.ID = findings[i].ID
		if verification == nil {
			verification = fallbackUnverifiedVerification(finding)
			verifyResults[i].Verification = verification
		}
		v := *verification
		model.EnsureVerificationID(&v, finding.ID)
		drop, reason := shouldDropFinding(&v, opts.DropPolicy)
		if drop {
			if e.logger != nil {
				e.logger.ProgressFor(
					e.progressInfo("verify", verifyProgressName(reviewerName, i), truncateFindingTitle(finding.Title)),
					logging.StageVerify,
					logging.StateSkip,
					fmt.Sprintf("dropped verdict=%s policy=%s", reason, model.NormalizeDropPolicy(opts.DropPolicy)),
				)
			}
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
		if e.logger != nil {
			e.logger.LiveFindings(logging.FindingUpdate{
				Refuted:  counts.refuted,
				Filtered: max(dropped-counts.refuted, 0),
			})
		}
		if dropped > 0 {
			e.logf(ctx, "Verifier filter before merge: reviewer=%s dropped=%d refuted=%d unverified=%d kept=%d policy=%s",
				vectorResults[vectorIdx].run.Name,
				dropped,
				counts.refuted,
				counts.unverified,
				len(keptByVector[vectorIdx]),
				model.NormalizeDropPolicy(opts.DropPolicy),
			)
		}
	}
	return telemetry, warnings, nil
}

func (e *Engine) prepareFindingsForVerification(ctx context.Context, reviewCtx *model.ReviewContext, vectorResults []agentResult, req model.ReviewRequest) []string {
	if req.DisableDiffScope || reviewCtx == nil || reviewCtx.DiffScopeHunks == nil {
		return nil
	}
	allowed := allowedDiffCodeLocations(reviewCtx.DiffScopeHunks, reviewCtx.ChangedFiles)
	relocate := e.diffScopeCodeLocationRelocator(req.RepoRoot, allowed)
	var warnings []string
	for i := range vectorResults {
		resp := vectorResults[i].resp
		if vectorResults[i].run.Status == model.AgentRunStatusFailed || resp == nil || len(resp.Findings) == 0 {
			continue
		}
		kept := make([]model.Finding, 0, len(resp.Findings))
		for findingIdx, finding := range resp.Findings {
			if codeLocationOverlapsAllowed(finding.CodeLocation, allowed) {
				kept = append(kept, finding)
				continue
			}
			if relocate != nil {
				relocate(ctx, &finding.CodeLocation)
			}
			if codeLocationOverlapsAllowed(finding.CodeLocation, allowed) {
				kept = append(kept, finding)
				continue
			}
			if e.logger != nil {
				e.logger.ProgressFor(
					e.progressInfo("verify", verifyProgressName(vectorResults[i].run.Name, findingIdx), truncateFindingTitle(finding.Title)),
					logging.StageVerify,
					logging.StateSkip,
					"dropped reason=out-of-diff",
				)
			}
		}
		dropped := len(resp.Findings) - len(kept)
		resp.Findings = kept
		if dropped > 0 {
			reviewer := strings.TrimSpace(vectorResults[i].run.Name)
			if reviewer == "" {
				reviewer = "injected findings"
			}
			warnings = append(warnings, fmt.Sprintf("Dropped %d out-of-diff finding(s) from %s after deterministic location repair found no unique allowed code_location", dropped, reviewer))
			if e.logger != nil {
				e.logger.LiveFindings(logging.FindingUpdate{Filtered: dropped})
			}
			e.logf(ctx, "Diff-scope filter before categorization: reviewer=%s dropped=%d kept=%d", vectorResults[i].run.Name, dropped, len(kept))
		}
	}
	return warnings
}

// shouldDropVerdict returns whether a verifier verdict is severe enough to drop
// a finding before merge, plus a label describing why (or why not).
//
// Labels:
//   - "refuted" / "unverified": verdict reason for dropping
//   - "kept": verdict does not warrant dropping (e.g. "confirmed", or policy="none")
func shouldDropVerdict(verdict string, policy string) (bool, string) {
	policy = model.NormalizeDropPolicy(policy)
	if policy == model.DropPolicyNone {
		return false, "kept"
	}
	if verdict == "" {
		// Treat missing verdict as unverified so we never drop on schema gaps.
		verdict = model.VerdictUnverified
	}
	switch policy {
	case model.DropPolicyRefutedOnly:
		if verdict != model.VerdictRefuted {
			return false, "kept"
		}
	case model.DropPolicyRefutedAndUnverified:
		if verdict == model.VerdictConfirmed {
			return false, "kept"
		}
	default:
		return false, "kept"
	}
	return true, verdict
}

// shouldDropFinding applies shouldDropVerdict to a verifier result.
func shouldDropFinding(v *model.FindingVerification, policy string) (bool, string) {
	if v == nil {
		return false, "kept"
	}
	return shouldDropVerdict(v.Verdict, policy)
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
// dedupe/merge agents only ever judge the ambiguous remainder. Clusters with
// multiple suggestion candidates stay unfolded so the agent can select the
// single best suggestion. Returns the reduced list and how many findings were
// absorbed; zero absorbed returns the input slice untouched.
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
		if suggestionCandidateCount(members) > 1 {
			out = append(out, members...)
			continue
		}
		out = append(out, dedupe.FoldCluster(members))
	}
	absorbed := len(findings) - len(out)
	if absorbed == 0 {
		return findings, 0
	}
	return out, absorbed
}

func suggestionCandidateCount(findings []model.Finding) int {
	count := 0
	for _, finding := range findings {
		count += len(finding.Suggestions)
		if count > 1 {
			return count
		}
	}
	return count
}

// runDedupeAgents runs a per-reviewer dedupe pass concurrently. It intentionally
// mutates vectorResults[idx].resp in place, but only when a dedupe agent returns
// a valid response. A failed or invalid dedupe leaves that reviewer's original
// findings intact and only records the dedupe run for telemetry. Mechanically
// detectable duplicates are folded in code first; the LLM agent only sees the
// reduced set.
func (e *Engine) runDedupeAgents(ctx context.Context, userPrompt string, contextNotes string, vectorResults []agentResult, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest, styleGuides []model.StyleGuide, hasToolchainVersions bool) []model.AgentRun {
	runs := make([]model.AgentRun, len(vectorResults))
	var wg sync.WaitGroup
	for i := range vectorResults {
		result := vectorResults[i]
		if result.run.Status == model.AgentRunStatusFailed || result.resp == nil || len(result.resp.Findings) < 2 {
			continue
		}
		originalCount := len(result.resp.Findings)
		if reduced, absorbed := mechanicallyDedupeFindings(result.resp.Findings); absorbed > 0 {
			resp := cloneReviewResponse(result.resp)
			resp.Findings = reduced
			vectorResults[i].resp = resp
			result.resp = resp
			e.logf(ctx, "Mechanical dedupe: reviewer=%q absorbed=%d findings=%d", result.run.Name, absorbed, len(reduced))
		}
		if len(result.resp.Findings) < 2 {
			if e.logger != nil {
				e.logger.LiveFindings(logging.FindingUpdate{
					Duplicate: originalCount - len(result.resp.Findings),
				})
			}
			continue
		}
		wg.Add(1)
		go func(idx int, input agentResult, before int) {
			defer wg.Done()
			resp, run := e.runDedupeAgent(ctx, userPrompt, contextNotes, input, schema, constraints, req, styleGuides, hasToolchainVersions)
			runs[idx] = run
			after := len(input.resp.Findings)
			if resp != nil {
				vectorResults[idx].resp = resp
				after = len(resp.Findings)
			}
			if e.logger != nil {
				e.logger.LiveFindings(logging.FindingUpdate{
					Duplicate: max(before-after, 0),
				})
			}
		}(i, result, originalCount)
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

func (e *Engine) runDedupeAgent(ctx context.Context, userPrompt string, contextNotes string, input agentResult, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest, styleGuides []model.StyleGuide, hasToolchainVersions bool) (*llm.ReviewResponse, model.AgentRun) {
	result, err := e.callDedupeAgent(ctx, userPrompt, contextNotes, input, schema, constraints, req, styleGuides, hasToolchainVersions)
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

func (e *Engine) callDedupeAgent(ctx context.Context, userPrompt string, contextNotes string, input agentResult, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest, styleGuides []model.StyleGuide, hasToolchainVersions bool) (agentResult, error) {
	systemTemplate, err := e.loadPrompt("agent_dedupe_system_prompt.tmpl")
	if err != nil {
		return agentResult{}, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("dedupe", exampleSnippetFor(llm.SchemaKindMerge, req.DisableSuggestions), req.DisableSuggestions)
	if err != nil {
		return agentResult{}, err
	}
	styleGuideToolchainSnippet, err := e.renderStyleGuideToolchainSnippet("dedupe", styleGuides, hasToolchainVersions)
	if err != nil {
		return agentResult{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		FindingInstructionsSnippet string
		PrioritySnippet            string
		OutputFormatSnippet        string
		DisableSuggestions         bool
		DisableDiffScope           bool
		StyleGuideToolchainSnippet string
	}{
		FindingInstructionsSnippet: commonSnippets.findingInstructions,
		PrioritySnippet:            commonSnippets.priority,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		DisableSuggestions:         req.DisableSuggestions,
		DisableDiffScope:           req.DisableDiffScope,
		StyleGuideToolchainSnippet: strings.TrimSpace(styleGuideToolchainSnippet),
	})
	if err != nil {
		return agentResult{}, fmt.Errorf("review: rendering dedupe system prompt: %w", err)
	}
	dedupeUser, err := llm.RenderJSON(map[string]any{
		"review_context":      json.RawMessage(userPrompt),
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
func (e *Engine) runClusterMergeAgents(ctx context.Context, userPrompt string, contextNotes string, inputs []pairwiseMergeInput, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest, styleGuides []model.StyleGuide, hasToolchainVersions bool) (agentResult, []model.AgentRun) {
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
			outcomes[ci], runs[ci] = e.runClusterMergeAgent(ctx, userPrompt, contextNotes, reduced, reviewerByID, schema, constraints, req, styleGuides, hasToolchainVersions, fmt.Sprintf("#%d", ci+1))
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

// runClusterMergeAgent judges one ambiguous cluster. Any failure path returns
// the cluster unmerged so reviewer findings are never lost.
func (e *Engine) runClusterMergeAgent(ctx context.Context, userPrompt string, contextNotes string, cluster []model.Finding, reviewerByID map[string]string, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest, styleGuides []model.StyleGuide, hasToolchainVersions bool, shardLabel string) ([]model.Finding, model.AgentRun) {
	result, err := e.callClusterMergeAgent(ctx, userPrompt, contextNotes, cluster, reviewerByID, schema, constraints, req, styleGuides, hasToolchainVersions, shardLabel)
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

func (e *Engine) callClusterMergeAgent(ctx context.Context, userPrompt string, contextNotes string, cluster []model.Finding, reviewerByID map[string]string, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest, styleGuides []model.StyleGuide, hasToolchainVersions bool, shardLabel string) (agentResult, error) {
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
		DisableDiffScope           bool
		StyleGuideToolchainSnippet string
	}{
		FindingInstructionsSnippet: commonSnippets.findingInstructions,
		PrioritySnippet:            commonSnippets.priority,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		DisableSuggestions:         req.DisableSuggestions,
		DisableDiffScope:           req.DisableDiffScope,
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
		progressName:  shardProgressName("Merge", shardLabel),
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
	tooManySuggestions := 0
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
		if len(finding.Suggestions) > 1 {
			tooManySuggestions++
		}
	}
	if !countTooLow && !countTooHigh && unknownIDs == 0 && duplicateIDs == 0 && verificationMismatch == 0 && tooManySuggestions == 0 {
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
	if tooManySuggestions > 0 {
		problems = append(problems, fmt.Sprintf("too_many_suggestions count=%d", tooManySuggestions))
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
			TooManySuggestions   int
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
			TooManySuggestions:   tooManySuggestions,
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
	tooManySuggestions := 0
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
		if len(finding.Suggestions) > 1 {
			tooManySuggestions++
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
	if tooManySuggestions > 0 {
		problems = append(problems, fmt.Sprintf("too_many_suggestions count=%d", tooManySuggestions))
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
			CountMismatch      bool
			GotCount           int
			MaxCount           int
			Unmatched          int
			AllowedIDs         []string
			UnknownIDs         []string
			Dropped            int
			DroppedIDs         []string
			DroppedTitles      string
			TooManySuggestions int
		}{
			CountMismatch:      countMismatch,
			GotCount:           len(resp.Findings),
			MaxCount:           len(cluster),
			Unmatched:          unmatched,
			AllowedIDs:         allowedIDs,
			UnknownIDs:         unknownOutputIDs,
			Dropped:            len(droppedTitles),
			DroppedIDs:         droppedIDs,
			DroppedTitles:      strings.Join(droppedTitles, "; "),
			TooManySuggestions: tooManySuggestions,
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
		run:             result.run,
		reasoningEffort: result.reasoningEffort,
		contentMessages: result.contentMessages,
		toolMessages:    result.toolMessages,
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
	var invalidResp *llm.InvalidResponseError
	if errors.As(err, &invalidResp) && (invalidResp.Reason != "" || invalidResp.RawContent != "") {
		result.run.InvalidResponse = &model.InvalidResponseDiagnostic{
			Reason:     textsan.RedactSecrets(textsan.StripControl(invalidResp.Reason)),
			RawContent: textsan.RedactSecrets(textsan.StripControl(invalidResp.RawContent)),
		}
	}
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
	// History results are keyed by a revision range rather than a file path, so
	// label them with it instead of falling back to a bare call index.
	if revisions, _ := payload["range"].(string); revisions != "" {
		return "git/" + revisions
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
	case llm.SchemaKindCategorize:
		return llm.CategorizeExamplePromptSnippet()
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

type noToolsPromptOptions struct {
	DiffScopeEnabled      bool
	UnusedIdentifierKinds string
}

func noToolsMessages(agentRole string, systemTemplate string, messages []llm.Message, snippet string, styleGuideToolchainSnippet string, disableSuggestions bool, options ...noToolsPromptOptions) ([]llm.Message, error) {
	var promptOptions noToolsPromptOptions
	if len(options) > 0 {
		promptOptions = options[0]
	}
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
		DiffScopeEnabled           bool
		UnusedIdentifierKinds      string
	}{
		OutputSchemaSnippet:        snippet,
		FindingInstructionsSnippet: commonSnippets.findingInstructions,
		PrioritySnippet:            commonSnippets.priority,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		HasTools:                   false,
		StyleGuideToolchainSnippet: strings.TrimSpace(styleGuideToolchainSnippet),
		DiffScopeEnabled:           promptOptions.DiffScopeEnabled,
		UnusedIdentifierKinds:      promptOptions.UnusedIdentifierKinds,
	})
	if err != nil {
		return nil, fmt.Errorf("review: rendering no-tools system prompt: %w", err)
	}
	return replaceSystemPromptWithoutTools(noToolsPrompt, messages), nil
}

// replaceSystemPromptWithoutTools rewrites a tool-using conversation for a model
// that cannot call tools: tool-call assistant turns keep only their prose, tool
// results become user turns, and the system prompt is swapped for the no-tools
// render.
func replaceSystemPromptWithoutTools(noToolsPrompt string, messages []llm.Message) []llm.Message {
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
	return finalMessages
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

// stepStyleGuides returns the styleguides for dedupe and merge prompts. A
// source-less workflow (e.g. --step merge --findings a.json) never runs
// ensurePrompts, so st.styleGuides stays unset; fall back to resolving
// directly — a nil context yields no language guides but still carries the
// user-supplied additional guides.
func (e *Engine) stepStyleGuides(st *PipelineState) ([]model.StyleGuide, error) {
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

// appendFullFiles inlines the current content of every changed file as
// supplemental context.
//
// Symlinks are skipped: reading their path follows the link and would inline the
// TARGET's text under the symlink's own path, which both misrepresents the file
// and invites findings about unrelated content. A symlink's real content is its
// target path, which the diff already shows.
//
// Deleted entries are skipped too. There is nothing to read for an ordinary
// deletion, and replacing a file with a symlink produces two entries for one path
// — a deleted regular file plus an added symlink — where reading the deleted entry
// would land on the symlink that now occupies the path and inline its target.
func (e *Engine) appendFullFiles(ctx context.Context, reviewCtx *model.ReviewContext, repoRoot string) {
	e.logf(ctx, "Including full files: count=%d", len(reviewCtx.ChangedFiles))
	for _, file := range reviewCtx.ChangedFiles {
		if file.Status == model.FileDeleted {
			e.logf(ctx, "Skipping file retrieval: path=%s reason=deleted", file.Path)
			continue
		}
		if file.Symlink {
			e.logf(ctx, "Skipping file retrieval: path=%s reason=symlink", file.Path)
			continue
		}
		e.logf(ctx, "Retrieving file: path=%s", file.Path)
		content, err := e.retrieval.GetFile(ctx, repoRoot, file.Path)
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
	OptimizedTo string // non-empty when the result was structurally replaced, e.g. "search" → "find_callers"
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
		if toolCall.Name == "search" {
			entry.OptimizedTo = optimizedSearchResultTool(rawContents[toolCall.ID])
		}
		history = append(history, entry)
	}
	return history
}

func optimizedSearchResultTool(content string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return ""
	}
	switch classifyToolResult(payload) {
	case shapeCallHierarchy:
		return "find_callers"
	case shapeReferences:
		return "find_references"
	case shapeGroupedSearch:
		_, hasCallers := payload["call_hierarchies"]
		_, hasReferences := payload["reference_results"]
		switch {
		case hasCallers && hasReferences:
			return "find_callers/find_references"
		case hasCallers:
			return "find_callers"
		default:
			return "find_references"
		}
	}
	return ""
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
			if path := nodeCodeLocationString(entry, "file_path"); path != "" {
				distinct[path] = struct{}{}
			}
		}
		summary.Files = len(distinct)
	}
	if commits, ok := payload["commits"].([]any); ok {
		// git_log/git_show results are commit-shaped: report how many commits
		// came back and across how many distinct files they changed.
		summary.HasResultCount = true
		summary.ResultCount = len(commits)
		distinct := make(map[string]struct{})
		for _, item := range commits {
			commit, ok := item.(map[string]any)
			if !ok {
				continue
			}
			files, ok := commit["files"].([]any)
			if !ok {
				continue
			}
			for _, file := range files {
				entry, ok := file.(map[string]any)
				if !ok {
					continue
				}
				if path, _ := entry["path"].(string); path != "" {
					distinct[path] = struct{}{}
				}
			}
		}
		summary.Files = len(distinct)
	}
	if resultCount, ok := payload["result_count"].(float64); ok {
		summary.HasResultCount = true
		summary.ResultCount = int(resultCount)
	}
	// One search can return several structural payloads at once, so the
	// structural shapes are summarized over every payload the result carries —
	// a single top-level one, or the grouped call_hierarchies/reference_results
	// arrays — and the file sets are unioned rather than overwritten.
	structural := structuralSummaryPayloads(payload)
	literals, hasLiterals := payload["literal_results"].([]any)
	if len(structural) > 0 || hasLiterals {
		distinct := map[string]struct{}{}
		for _, item := range structural {
			accumulateStructuralSummary(item, &summary, distinct)
		}
		if hasLiterals {
			summary.HasResultCount = true
			summary.ResultCount += len(literals)
			for _, item := range literals {
				entry, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if path := nodeCodeLocationString(entry, "file_path"); path != "" {
					distinct[path] = struct{}{}
				}
			}
		}
		summary.Files = len(distinct)
	}
	return summary
}

// structuralSummaryPayloads returns every call-hierarchy or reference payload a
// tool result carries: the result itself when it is one, or each entry of the
// grouped arrays a multi-declaration search produces.
func structuralSummaryPayloads(payload map[string]any) []map[string]any {
	switch classifyToolResult(payload) {
	case shapeReferences, shapeCallHierarchy:
		return []map[string]any{payload}
	case shapeGroupedSearch:
		return groupedStructuralPayloads(payload)
	}
	return nil
}

// accumulateStructuralSummary folds one structural payload into summary,
// adding the files it touches to distinct.
func accumulateStructuralSummary(payload map[string]any, summary *toolResultSummary, distinct map[string]struct{}) {
	if root, ok := payload["root"].(map[string]any); ok {
		walkCallHierarchy(root, func(node map[string]any) {
			if path := nodeCodeLocationString(node, "file_path"); path != "" {
				distinct[path] = struct{}{}
			}
			summary.Lines += lineCount(nodeCodeLocationString(node, "content"))
		})
	}
	target, ok := payload["target"].(map[string]any)
	if !ok {
		return
	}
	if definition, ok := target["definition"].(map[string]any); ok {
		if path, _ := definition["file_path"].(string); path != "" {
			distinct[path] = struct{}{}
		}
		if content, _ := definition["content"].(string); content != "" {
			summary.Lines += lineCount(content)
		}
	}
	for _, field := range []string{"functions", "outside_functions"} {
		contexts, _ := payload[field].([]any)
		for _, item := range contexts {
			context, _ := item.(map[string]any)
			if path := nodeCodeLocationString(context, "file_path"); path != "" {
				distinct[path] = struct{}{}
			}
			summary.Lines += lineCount(nodeCodeLocationString(context, "content"))
		}
	}
	for _, field := range []string{"exact_reference_count", "possible_reference_count"} {
		if count, ok := payload[field].(float64); ok {
			summary.ResultCount += int(count)
			summary.HasResultCount = true
		}
	}
}

// nodeCodeLocationString reads a string field from a node's nested
// code_location object, the location shape shared by search results and call
// hierarchy nodes.
func nodeCodeLocationString(node map[string]any, field string) string {
	loc, ok := node["code_location"].(map[string]any)
	if !ok {
		return ""
	}
	value, _ := loc[field].(string)
	return value
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

// toolCallArgs is the union of arguments across the agent tools (inspect_file,
// list_files, search, find_callers, find_callees, find_references, git_log, and
// git_show). A single named type replaces the anonymous struct previously re-declared
// verbatim at several call sites. ContextLines is a pointer so an omitted
// search value renders as its query-dependent default.
type toolCallArgs struct {
	Path          string `json:"path"`
	LineStart     int    `json:"line_start"`
	LineEnd       int    `json:"line_end"`
	Depth         int    `json:"depth"`
	Line          int    `json:"line"`
	Symbol        string `json:"symbol"`
	Query         string `json:"query"`
	ContextLines  *int   `json:"context_lines"`
	MaxResults    int    `json:"max_results"`
	CaseSensitive bool   `json:"case_sensitive"`
	Commit        string `json:"commit"`
	To            string `json:"to"`
	Since         string `json:"since"`
	Until         string `json:"until"`
	Author        string `json:"author"`
	Paths         string `json:"paths"`
	Message       string `json:"message"`
	MessageRegex  bool   `json:"message_regex"`
	Limit         int    `json:"limit"`
	MaxCommits    int    `json:"max_commits"`
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
			args.Depth = toollimits.DefaultListFilesDepth
		}
		parts = append(parts, fmt.Sprintf("depth=%d", args.Depth))
	case "search":
		query := retrieval.NormalizeSearchQuery(args.Query)
		parts = append(parts, fmt.Sprintf("path=%q", syntheticPathValue(args.Path, ".")))
		parts = append(parts, fmt.Sprintf("query=%q", query))
		if queryLines := retrieval.FindLinesCount(query); queryLines > 1 {
			parts = append(parts, fmt.Sprintf("query_line_count=%d", queryLines))
		}
		parts = append(parts, fmt.Sprintf("context_lines=%d", resolveSearchContextLines(args.ContextLines, query)))
		parts = append(parts, fmt.Sprintf("max_results=%d", args.MaxResults))
		parts = append(parts, fmt.Sprintf("case_sensitive=%t", args.CaseSensitive))
	case "find_callers", "find_callees":
		parts = append(parts, fmt.Sprintf("path=%q", syntheticPathValue(args.Path, ".")))
		parts = append(parts, fmt.Sprintf("symbol=%q", args.Symbol))
		if args.Depth <= 0 {
			args.Depth = toollimits.DefaultCallHierarchyDepth
		}
		parts = append(parts, fmt.Sprintf("depth=%d", args.Depth))
	case "find_references":
		parts = append(parts, fmt.Sprintf("path=%q", syntheticPathValue(args.Path, ".")))
		parts = append(parts, fmt.Sprintf("symbol=%q", args.Symbol))
		if args.Line > 0 {
			// Without the pin, a retry that disambiguated an ambiguous symbol
			// renders identically to the call that failed, and the model cannot
			// tell from its own history which one it already tried.
			parts = append(parts, fmt.Sprintf("line=%d", args.Line))
		}
	case "git_log":
		parts = append(parts, fmt.Sprintf("commit=%q", syntheticPathValue(args.Commit, "HEAD")))
		parts = appendOptionalToolArgs(parts, [][2]string{
			{"since", args.Since},
			{"until", args.Until},
			{"author", args.Author},
			{"paths", args.Paths},
			{"message", args.Message},
		})
		if args.MessageRegex {
			parts = append(parts, "message_regex=true")
		}
		if args.CaseSensitive {
			parts = append(parts, "case_sensitive=true")
		}
		if args.Limit > 0 {
			parts = append(parts, fmt.Sprintf("limit=%d", args.Limit))
		}
	case "git_show":
		parts = append(parts, fmt.Sprintf("commit=%q", syntheticPathValue(args.Commit, "<commit>")))
		parts = appendOptionalToolArgs(parts, [][2]string{
			{"to", args.To},
			{"paths", args.Paths},
		})
		if args.MaxCommits > 0 {
			parts = append(parts, fmt.Sprintf("max_commits=%d", args.MaxCommits))
		}
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
		if toolName == "search" {
			parts = append(parts, "result_count=0")
			return fmt.Sprintf("result=[%s]", strings.Join(parts, ", "))
		}
		parts = append(parts, "ok")
	}
	return fmt.Sprintf("result=[%s]", strings.Join(parts, ", "))
}

// appendOptionalToolArgs renders only the filter arguments an agent actually
// passed, in a fixed order so log lines and synthetic tool histories stay
// stable across calls.
func appendOptionalToolArgs(parts []string, values [][2]string) []string {
	for _, value := range values {
		if strings.TrimSpace(value[1]) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%q", value[0], value[1]))
	}
	return parts
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
	if callNum == 0 {
		callNum = info.Turn
	}
	turnInfo := info.WithTurn(callNum)
	if ok && e.logger != nil {
		turnCtx := logging.WithProgressInfo(ctx, turnInfo)
		e.logger.Progress(turnCtx, logging.StageRequest, logging.StateSent, "")
		e.logger.Progress(turnCtx, logging.StageReasoning, logging.StateStart, "")
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
		turnCtx := logging.WithProgressInfo(ctx, turnInfo)
		if resp != nil && resp.Reasoned {
			e.logger.Progress(turnCtx, logging.StageReasoning, logging.StateDone, elapsed.String())
		}
		e.logger.Progress(turnCtx, logging.StageResponse, logging.StateDone, elapsed.String())
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
	return fmt.Sprintf("%s(%s)", toolCall.Name, syntheticToolArgumentsForCall(toolCall))
}

func reviewContextSummary(ctx *model.ReviewContext, req model.ReviewRequest) string {
	if ctx == nil {
		return ""
	}
	return fmt.Sprintf("%s:%s [%s, ≥%s] on %s",
		ctx.Mode, req.Submode,
		req.ProfileName, req.PriorityThreshold,
		reviewTargetSummary(ctx),
	)
}

// reviewTargetSummary is the "repo @ head → base" tail of the context summary —
// shared with the live dashboard so a review target reads the same everywhere.
func reviewTargetSummary(ctx *model.ReviewContext) string {
	if ctx == nil {
		return ""
	}
	return fmt.Sprintf("%s @ %s → %s",
		ctx.Repository.FullName, ctx.Repository.HeadRef, ctx.Repository.BaseRef)
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
