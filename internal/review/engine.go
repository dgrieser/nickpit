package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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
	multiAgentReview       bool
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

func (e *Engine) SetMultiAgentReview(enabled bool) {
	e.multiAgentReview = enabled
}

func (e *Engine) Run(ctx context.Context, req model.ReviewRequest) (*model.ReviewResult, error) {
	result, _, err := e.RunWithContext(ctx, req)
	return result, err
}

func (e *Engine) RunWithContext(ctx context.Context, req model.ReviewRequest) (*model.ReviewResult, *model.ReviewContext, error) {
	e.logf("Starting review: mode=%s repo=%s id=%d submode=%s repo_root=%s", req.Mode, req.Repo, req.Identifier, req.Submode, req.RepoRoot)
	reviewCtx, err := e.source.ResolveContext(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	e.logProgress("Review", reviewContextSummary(reviewCtx, req))
	e.logf("Resolved context: title=%q files=%d commits=%d comments=%d diff_bytes=%d", reviewCtx.Title, len(reviewCtx.ChangedFiles), len(reviewCtx.Commits), len(reviewCtx.Comments), len(reviewCtx.Diff))
	if len(reviewCtx.ChangedFiles) == 0 && len(reviewCtx.Diff) == 0 {
		return nil, nil, ErrEmptyDiff
	}
	reviewCtx.CheckoutRoot = req.RepoRoot
	reviewCtx.Identifier = req.Identifier

	if e.toolchainCapture != nil {
		reviewCtx.ToolchainVersions = e.toolchainCapture(ctx, req.RepoRoot, reviewCtx)
		if len(reviewCtx.ToolchainVersions) > 0 {
			e.logf("Captured toolchain versions: count=%d", len(reviewCtx.ToolchainVersions))
		}
	}

	if req.IncludeFullFiles && e.retrieval != nil && req.RepoRoot != "" {
		e.logf("Including full files: count=%d", len(reviewCtx.ChangedFiles))
		for _, file := range reviewCtx.ChangedFiles {
			e.logf("Retrieving file: path=%s", file.Path)
			content, err := e.retrieval.GetFile(ctx, req.RepoRoot, file.Path)
			if err != nil {
				e.logf("Skipping file retrieval: path=%s error=%v", file.Path, err)
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
		return nil, nil, fmt.Errorf("review: trim context: %w", err)
	}
	e.logf("Trimmed context: files=%d supplemental=%d omitted=%d budget=%d", len(trimmed.ChangedFiles), len(trimmed.SupplementalContext), len(trimmed.OmittedSections), req.MaxContextTokens)
	var result *model.ReviewResult
	var enrichedCtx *model.ReviewContext
	if e.multiAgentReview {
		result, enrichedCtx, err = e.runMultiAgentReview(ctx, trimmed, req)
	} else {
		result, enrichedCtx, err = e.runSingleAgentReview(ctx, trimmed, req)
	}
	if err != nil {
		return nil, nil, err
	}
	result.Mode = string(req.Mode)
	if req.Submode != "" {
		result.Mode = result.Mode + ":" + req.Submode
	}
	result.Model = e.config.Model
	result.Repo = req.Repo
	result.Identifier = req.Identifier
	result.BaseURL = e.config.BaseURL
	result.BaseRef = reviewCtx.Repository.BaseRef
	result.HeadRef = reviewCtx.Repository.HeadRef
	return result, enrichedCtx, nil
}

func (e *Engine) reviewWithoutTools(ctx context.Context, llmReq *llm.ReviewRequest, systemTemplate string, messages []llm.Message, systemSnippet string, styleGuideToolchainSnippet string, maxOutputRetries int, sec *logging.ReasoningSection) (*llm.ReviewResponse, error) {
	finalMessages, err := noToolsMessages(systemTemplate, messages, systemSnippet, styleGuideToolchainSnippet)
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
		e.logfCtx(ctx, "Invalid JSON response in no-tools call, retrying: attempt=%d reason=%q missing=%v", attempt+1, invalidResp.Reason, invalidResp.MissingFields)
		if tag, ok := agentTagFromContext(ctx); ok && tag.Name != "" {
			e.logProgress("Model", fmt.Sprintf("status=InvalidJsonRetry, agent=%s, attempt=%d", tag.Name, attempt+1))
		} else {
			e.logProgress("Model", fmt.Sprintf("status=InvalidJsonRetry, attempt=%d", attempt+1))
		}
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

func (e *Engine) runSingleAgentReview(ctx context.Context, reviewCtx *model.ReviewContext, req model.ReviewRequest) (*model.ReviewResult, *model.ReviewContext, error) {
	systemTemplate, err := e.loadPrompt("agent_review_general_system_prompt.tmpl")
	if err != nil {
		return nil, nil, err
	}
	payload := model.PromptPayloadFromContext(reviewCtx)
	payload.StyleGuides, err = e.styleGuidesFor(reviewCtx)
	if err != nil {
		return nil, nil, err
	}
	styleGuideToolchainSnippet, err := e.renderStyleGuideToolchainSnippet("reviewing", payload.StyleGuides, len(payload.ToolchainVersions) > 0)
	if err != nil {
		return nil, nil, err
	}
	systemPrompt, err := e.renderReviewSystemWithFocus(systemTemplate, "", req, true, styleGuideToolchainSnippet)
	if err != nil {
		return nil, nil, err
	}
	noToolsSystem, err := e.renderReviewSystemWithFocus(systemTemplate, "", req, false, styleGuideToolchainSnippet)
	if err != nil {
		return nil, nil, err
	}
	userPrompt, err := llm.RenderJSON(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("review: rendering review prompt json: %w", err)
	}
	e.logf("Rendered review context JSON: lines=%d chars=%d", lineCount(userPrompt), len(userPrompt))
	var schema []byte
	if req.UseJSONSchema {
		schema = llm.FindingsSchema
	}
	run, err := e.runAgent(ctx, agentSpec{
		name:          fmt.Sprintf("#1: %s@%s", reviewCtx.Repository.FullName, reviewCtx.Repository.HeadRef),
		role:          "reviewer",
		system:        systemPrompt,
		noToolsSystem: noToolsSystem,
		user:          userPrompt,
		schema:        schema,
		schemaKind:    llm.SchemaKindReview,
		hasTools:      true,
	}, req)
	if err != nil {
		return nil, nil, err
	}
	filtered := filterByPriority(run.resp.Findings, req.PriorityThreshold)
	if overwrote := model.EnsureFindingIDs(filtered); overwrote > 0 {
		e.logf("Review generated replacement IDs for invalid finding IDs: count=%d", overwrote)
	}
	e.logf(
		"Review complete: findings=%d filtered=%d threshold=%s tool_calls=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d",
		len(run.resp.Findings),
		len(filtered),
		req.PriorityThreshold,
		run.run.ToolCalls,
		run.run.TokensUsed.PromptTokens,
		run.run.TokensUsed.CompletionTokens,
		run.run.TokensUsed.TotalTokens,
	)
	return &model.ReviewResult{
		Findings:               filtered,
		OverallCorrectness:     run.resp.OverallCorrectness,
		OverallExplanation:     run.resp.OverallExplanation,
		OverallConfidenceScore: run.resp.OverallConfidenceScore,
		AgentRuns:              []model.AgentRun{run.run},
		TokensUsed:             run.run.TokensUsed,
		TotalToolCalls:         run.run.ToolCalls,
		ReasoningEffort:        run.reasoningEffort,
	}, reviewCtx, nil
}

type reviewVector struct {
	name          string
	focusFile     string
	questionsFile string
	constraints   llm.ResponseConstraints
}

var reviewVectors = []reviewVector{
	{
		name:          "Code Quality",
		focusFile:     "agent_review_codequality_system_prompt.tmpl",
		questionsFile: "agent_review_codequality_questions.tmpl",
	},
	{
		name:          "Security",
		focusFile:     "agent_review_security_system_prompt.tmpl",
		questionsFile: "agent_review_security_questions.tmpl",
	},
	{
		name:          "Architecture",
		focusFile:     "agent_review_architecture_system_prompt.tmpl",
		questionsFile: "agent_review_architecture_questions.tmpl",
	},
	{
		name:          "Performance",
		focusFile:     "agent_review_performance_system_prompt.tmpl",
		questionsFile: "agent_review_performance_questions.tmpl",
	},
	{
		name:          "Testing",
		focusFile:     "agent_review_testing_system_prompt.tmpl",
		questionsFile: "agent_review_testing_questions.tmpl",
		constraints: llm.ResponseConstraints{
			MinPriority:        intPtr(2),
			AllowedCorrectness: []string{"patch is correct"},
		},
	},
	{
		name:          "Best Practices",
		focusFile:     "agent_review_bestpractices_system_prompt.tmpl",
		questionsFile: "agent_review_bestpractices_questions.tmpl",
	},
}

func intPtr(v int) *int { return &v }

func (e *Engine) runMultiAgentReview(ctx context.Context, reviewCtx *model.ReviewContext, req model.ReviewRequest) (*model.ReviewResult, *model.ReviewContext, error) {
	baseTemplate, err := e.loadPrompt("agent_review_general_system_prompt.tmpl")
	if err != nil {
		return nil, nil, err
	}
	contextTemplate, err := e.loadPrompt("agent_context_system_prompt.tmpl")
	if err != nil {
		return nil, nil, err
	}
	contextSystem, err := e.renderContextSystem(contextTemplate, req)
	if err != nil {
		return nil, nil, err
	}
	payload := model.PromptPayloadFromContext(reviewCtx)
	payload.StyleGuides, err = e.styleGuidesFor(reviewCtx)
	if err != nil {
		return nil, nil, err
	}
	styleGuideToolchainSnippet, err := e.renderStyleGuideToolchainSnippet("reviewing", payload.StyleGuides, len(payload.ToolchainVersions) > 0)
	if err != nil {
		return nil, nil, err
	}
	userPrompt, err := llm.RenderJSON(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("review: rendering review prompt json: %w", err)
	}
	e.logf("Rendered review context JSON: lines=%d chars=%d", lineCount(userPrompt), len(userPrompt))

	var warnings []string
	contextResult, contextErr := e.runContextAgent(ctx, agentSpec{
		name:          "Collect Context",
		role:          "context",
		system:        contextSystem,
		noToolsSystem: contextSystem,
		user:          userPrompt,
		schemaKind:    llm.SchemaKindText,
		hasTools:      true,
	}, req)
	if contextErr != nil {
		e.logf("Context agent failed, continuing with degraded context: error=%v", contextErr)
		warnings = append(warnings, fmt.Sprintf("Context agent failed: %v; continuing with degraded context", contextErr))
		contextResult.run.Status = model.AgentRunStatusFailed
		contextResult.run.Error = contextErr.Error()
	}

	enriched, err := model.CloneContext(reviewCtx)
	if err != nil {
		return nil, nil, fmt.Errorf("review: cloning context: %w", err)
	}
	enriched.SupplementalContext = append(enriched.SupplementalContext, supplementalFromContextAgent(contextResult.toolMessages)...)
	payload = model.PromptPayloadFromContext(enriched)
	payload.StyleGuides, err = e.styleGuidesFor(enriched)
	if err != nil {
		return nil, nil, err
	}
	styleGuideToolchainSnippet, err = e.renderStyleGuideToolchainSnippet("reviewing", payload.StyleGuides, len(payload.ToolchainVersions) > 0)
	if err != nil {
		return nil, nil, err
	}
	enrichedPrompt, err := llm.RenderJSON(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("review: rendering enriched review prompt json: %w", err)
	}

	var schema []byte
	if req.UseJSONSchema {
		schema = llm.FindingsSchema
	}
	contextMessages := contextAgentMarkdownMessages(contextResult.contentMessages)
	vectorResults, err := e.runVectorAgents(ctx, baseTemplate, enrichedPrompt, contextMessages, schema, req, styleGuideToolchainSnippet)
	if err != nil {
		return nil, nil, err
	}

	mergeConstraints := llm.ResponseConstraints{}
	var mergeSchema []byte
	if req.UseJSONSchema {
		mergeConstraints = mergeConstraintsForRequest(req)
		if hasResponseConstraints(mergeConstraints) {
			mergeSchema = llm.FindingsWithIDSchemaWithConstraints(mergeConstraints)
		} else {
			mergeSchema = llm.FindingsWithIDSchema
		}
	}
	var (
		mergeResult agentResult
		mergeErr    error
	)
	if allVectorsFailed(vectorResults) {
		// Every vector reviewer failed — calling the merge LLM on an
		// all-failed payload risks hallucinated findings. Short-circuit
		// to the local synthesizer (empty findings) instead.
		e.logf("All vector reviewers failed; skipping merge agent and emitting empty result")
		warnings = append(warnings, "All vector reviewers failed; skipped merge agent and returning empty findings")
		mergeResult = synthesizedMergeFromVectors(vectorResults, nil)
	} else {
		mergeResult, mergeErr = e.runMergeAgent(ctx, enrichedPrompt, contextAgentMarkdownContent(contextResult.contentMessages), vectorResults, mergeSchema, mergeConstraints, req)
		if mergeErr != nil {
			e.logf("Merge agent failed, falling back to deduped vector union: error=%v", mergeErr)
			warnings = append(warnings, fmt.Sprintf("Merge agent failed: %v; falling back to deduped vector findings", mergeErr))
			partialMergeTokens := mergeResult.run.TokensUsed
			partialMergeToolCalls := mergeResult.run.ToolCalls
			mergeResult = synthesizedMergeFromVectors(vectorResults, mergeErr)
			mergeResult.run.TokensUsed = partialMergeTokens
			mergeResult.run.ToolCalls = partialMergeToolCalls
		}
	}
	allRuns := make([]model.AgentRun, 0, 2+len(vectorResults))
	allRuns = append(allRuns, contextResult.run)
	totalUsage := contextResult.run.TokensUsed
	toolCalls := contextResult.run.ToolCalls
	effectiveReasoningEffort := contextResult.reasoningEffort
	for _, result := range vectorResults {
		allRuns = append(allRuns, result.run)
		totalUsage = addTokenUsage(totalUsage, result.run.TokensUsed)
		toolCalls += result.run.ToolCalls
		if result.reasoningEffort != "" {
			effectiveReasoningEffort = result.reasoningEffort
		}
	}
	allRuns = append(allRuns, mergeResult.run)
	totalUsage = addTokenUsage(totalUsage, mergeResult.run.TokensUsed)
	if mergeResult.reasoningEffort != "" {
		effectiveReasoningEffort = mergeResult.reasoningEffort
	}
	warnings = appendAgentRunWarnings(warnings, allRuns, contextErr, mergeErr)

	filtered := filterByPriority(mergeResult.resp.Findings, req.PriorityThreshold)
	if overwrote := model.EnsureFindingIDs(filtered); overwrote > 0 {
		e.logf("Review generated replacement IDs for invalid finding IDs: count=%d", overwrote)
	}
	e.logf(
		"Review complete: findings=%d filtered=%d threshold=%s tool_calls=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d warnings=%d",
		len(mergeResult.resp.Findings),
		len(filtered),
		req.PriorityThreshold,
		toolCalls,
		totalUsage.PromptTokens,
		totalUsage.CompletionTokens,
		totalUsage.TotalTokens,
		len(warnings),
	)
	return &model.ReviewResult{
		Findings:               filtered,
		OverallCorrectness:     mergeResult.resp.OverallCorrectness,
		OverallExplanation:     mergeResult.resp.OverallExplanation,
		OverallConfidenceScore: mergeResult.resp.OverallConfidenceScore,
		AgentRuns:              allRuns,
		Warnings:               warnings,
		TokensUsed:             totalUsage,
		TotalToolCalls:         toolCalls,
		ReasoningEffort:        effectiveReasoningEffort,
	}, enriched, nil
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
// warnings list. Failures already surfaced via contextErr / mergeErr above are
// skipped to avoid duplicates — the dedicated warning messages there are more
// informative than the bare agent error string.
func appendAgentRunWarnings(warnings []string, runs []model.AgentRun, contextErr, mergeErr error) []string {
	for _, run := range runs {
		if run.Status == model.AgentRunStatusOK {
			continue
		}
		if run.Role == "context" && contextErr != nil {
			continue
		}
		if run.Role == "merge" && mergeErr != nil {
			continue
		}
		switch run.Status {
		case model.AgentRunStatusFailed:
			warnings = append(warnings, fmt.Sprintf("%s reviewer failed: %s", run.Name, run.Error))
		case model.AgentRunStatusPartial:
			warnings = append(warnings, fmt.Sprintf("%s reviewer partial result: %s", run.Name, run.Error))
		}
	}
	return warnings
}

func (e *Engine) runVectorAgents(ctx context.Context, baseTemplate, userPrompt string, contextMessages []llm.Message, schema []byte, req model.ReviewRequest, styleGuideToolchainSnippet string) ([]agentResult, error) {
	results := make([]agentResult, len(reviewVectors))
	errs := make([]error, len(reviewVectors))
	var wg sync.WaitGroup
	for i, vector := range reviewVectors {
		wg.Add(1)
		go func(idx int, vector reviewVector) {
			defer wg.Done()
			questionsSnippet, err := e.renderReviewerQuestionsSnippet(vector.questionsFile)
			if err != nil {
				errs[idx] = err
				return
			}
			system, err := e.renderReviewSystemWithQuestions(baseTemplate, vector.focusFile, questionsSnippet, req, true, styleGuideToolchainSnippet)
			if err != nil {
				errs[idx] = err
				return
			}
			noToolsSystem, err := e.renderReviewSystemWithQuestions(baseTemplate, vector.focusFile, questionsSnippet, req, false, styleGuideToolchainSnippet)
			if err != nil {
				errs[idx] = err
				return
			}
			agentSchema := schema
			if req.UseJSONSchema && (vector.constraints.MinPriority != nil || vector.constraints.MaxPriority != nil || len(vector.constraints.AllowedCorrectness) > 0) {
				agentSchema = llm.FindingsSchemaWithConstraints(vector.constraints)
			}
			result, err := e.runAgent(ctx, agentSpec{
				name:             vector.name,
				role:             "reviewer",
				system:           system,
				noToolsSystem:    noToolsSystem,
				user:             userPrompt,
				extraMessages:    contextMessages,
				questionsSnippet: questionsSnippet,
				schema:           agentSchema,
				schemaKind:       llm.SchemaKindReview,
				constraints:      vector.constraints,
				hasTools:         true,
			}, req)
			results[idx] = result
			errs[idx] = err
		}(i, vector)
	}
	wg.Wait()
	for i, err := range errs {
		if err == nil {
			continue
		}
		e.logf("Vector reviewer failed, continuing with others: vector=%s error=%v", reviewVectors[i].name, err)
		results[i] = failedVectorResult(reviewVectors[i], err)
	}
	return results, nil
}

func (e *Engine) runMergeAgent(ctx context.Context, userPrompt string, contextNotes string, vectorResults []agentResult, schema []byte, constraints llm.ResponseConstraints, req model.ReviewRequest) (agentResult, error) {
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
		"vector_reviews":      vectorReviewPayloads(vectorResults),
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
	}, req)
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
		kind:                     "context",
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

func (e *Engine) runAgent(ctx context.Context, agent agentSpec, req model.ReviewRequest) (agentResult, error) {
	noToolsSystem := agent.noToolsSystem
	if noToolsSystem == "" {
		noToolsSystem = agent.system
	}
	messages := []llm.Message{
		{Role: "system", Content: agent.system},
		{Role: "user", Content: agent.user},
	}
	messages = append(messages, agent.extraMessages...)
	label := fmt.Sprintf("%s: %s", agent.role, agent.name)
	if agent.role == "reviewer" && strings.HasPrefix(agent.name, "#") {
		label = "reviewer " + agent.name
	}
	sec := e.logger.NewReasoningTracker(label)
	defer sec.End()

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
		NoToolsMessages: func(messages []llm.Message) ([]llm.Message, error) {
			if !agent.hasTools {
				return append([]llm.Message(nil), messages...), nil
			}
			return noToolsMessagesFromRendered(noToolsSystem, messages)
		},
	}
	extractEnabled := agent.role == "reviewer" && req.NudgeCount > 0 && !req.DisableReasoningExtract && req.ModelEmitsReasoning
	var (
		extractMu           sync.Mutex
		collectWG           sync.WaitGroup
		collectedLists      []string
		extractorTokens     model.TokenUsage
		extractorToolCalls  int
		extractorDuplicates int
	)
	addExtractorRun := func(run model.AgentRun) {
		extractMu.Lock()
		defer extractMu.Unlock()
		extractorTokens = addTokenUsage(extractorTokens, run.TokensUsed)
		extractorToolCalls += run.ToolCalls
		extractorDuplicates += run.DuplicateToolCalls
	}
	extractorTotals := func() (model.TokenUsage, int, int) {
		extractMu.Lock()
		defer extractMu.Unlock()
		return extractorTokens, extractorToolCalls, extractorDuplicates
	}
	totalTokensWithExtractors := func(base model.TokenUsage) model.TokenUsage {
		tokens, _, _ := extractorTotals()
		return addTokenUsage(base, tokens)
	}
	totalToolCallsWithExtractors := func(base int) int {
		_, toolCalls, _ := extractorTotals()
		return base + toolCalls
	}
	totalDuplicatesWithExtractors := func(base int) int {
		_, _, duplicates := extractorTotals()
		return base + duplicates
	}
	combinedCollectedList := func() string {
		extractMu.Lock()
		defer extractMu.Unlock()
		return strings.TrimSpace(strings.Join(collectedLists, "\n"))
	}
	waitCollect := func() {
		collectWG.Wait()
	}
	launchCollect := func(agentName string, iterIdx int, reasoning string) {
		if strings.TrimSpace(reasoning) == "" {
			return
		}
		collectWG.Add(1)
		go func() {
			defer collectWG.Done()
			list, result, err := e.runReasoningCollectFindings(ctx, reasoning, agentName, iterIdx, req)
			addExtractorRun(result.run)
			if err != nil {
				e.logf("Reasoning collect findings failed: agent=%s iter=%d error=%v", agentName, iterIdx, err)
				return
			}
			if strings.TrimSpace(list) == "" {
				return
			}
			extractMu.Lock()
			collectedLists = append(collectedLists, list)
			extractMu.Unlock()
		}()
	}
	if extractEnabled {
		loopReq.OnReasoningTrace = func(agentName string, iterIdx int, reasoning string) {
			launchCollect(agentName, iterIdx, reasoning)
		}
		defer waitCollect()
	}

	loopResult, err := e.runAgentLoop(ctx, loopReq)
	if err != nil {
		return partialAgentResult(agent, req, loopResult), err
	}
	if loopResult.resp == nil {
		return partialAgentResult(agent, req, loopResult), fmt.Errorf("agent %s returned no response", agent.name)
	}
	totalFindings := append([]model.Finding(nil), loopResult.resp.Findings...)
	totalTokens := loopResult.tokensUsed
	totalToolCalls := loopResult.toolCalls
	totalDuplicates := loopResult.duplicateToolCalls
	latestResp := loopResult.resp
	latestReasoningEffort := loopResult.reasoningEffort
	historyMessages := messagesWithFinalResponse(loopResult.messages, loopResult.resp)
	contentMessages := append([]string(nil), loopResult.contentMessages...)
	toolMessages := append([]llm.Message(nil), loopResult.toolMessages...)
	toolCallHistory := append([]toolCallHistoryEntry(nil), loopResult.toolCallHistory...)

	if agent.role == "reviewer" && req.NudgeCount > 0 {
		// One shared agentLoopState across all nudge rounds: tool-call, duplicate,
		// and JSON-retry budgets are pooled for the entire nudge phase rather than
		// reset per round. Intentional — prevents a chatty model from multiplying
		// its budget by NudgeCount.
		nudgeState := newAgentLoopState()
		// Reset reasoning effort to the configured baseline at the start of the
		// nudge phase. Intentional: a back-off triggered during the initial round
		// (e.g. JSON repair forcing high → low) should not permanently degrade
		// the nudges. Subsequent rounds still carry their own back-offs forward
		// via the sub.reasoningEffort update below.
		nudgeReasoningEffort := e.config.ReasoningEffort
		var nudgeErr error
		// UpdateFindings inputs: combinedCollectedList (frozen after initial
		// run) + totalFindings (append-only via appendNewFindings). Cache the
		// delta keyed by len(totalFindings); recompute only when findings grew.
		cachedUpdateDelta := ""
		cachedUpdateFindingsLen := -1
		for i := 0; i < req.NudgeCount; i++ {
			nudgeName := fmt.Sprintf("%s - Nudge %d/%d", agent.name, i+1, req.NudgeCount)
			nudgeCtx := ctxWithAgent(ctx, agentTag{Role: agent.role, Name: nudgeName})
			e.logfCtx(nudgeCtx, "Nudge round: round=%d/%d", i+1, req.NudgeCount)
			waitCollect()
			reasoningFindings := ""
			if combined := combinedCollectedList(); combined != "" {
				if cachedUpdateFindingsLen == len(totalFindings) {
					reasoningFindings = cachedUpdateDelta
				} else {
					findingsJSON, err := reasoningFindingsJSON(totalFindings)
					if err != nil {
						return agentResult{}, err
					}
					delta, result, err := e.runReasoningUpdateFindings(nudgeCtx, combined, findingsJSON, agent.name, req)
					addExtractorRun(result.run)
					if err != nil {
						e.logfCtx(nudgeCtx, "Reasoning update findings failed, using standard nudge: round=%d/%d error=%v", i+1, req.NudgeCount, err)
					} else {
						reasoningFindings = delta
						cachedUpdateDelta = delta
						cachedUpdateFindingsLen = len(totalFindings)
					}
				}
			}
			formattedReasoningFindings := formatReasoningFindingsList(reasoningFindings)
			if extractEnabled {
				if formattedReasoningFindings != "" {
					e.logBlockCtx(nudgeCtx, "Extracted reasoning findings sent to nudge:", formattedReasoningFindings)
				} else {
					e.logfCtx(nudgeCtx, "No extracted reasoning findings to send to nudge")
				}
			}
			nudgeText, err := renderPromptFile("agent_review_nudge_user_message.tmpl", struct {
				HasResponseFormat bool
				QuestionsSnippet  string
				ReasoningFindings string
			}{
				HasResponseFormat: agent.schemaKind != llm.SchemaKindText,
				QuestionsSnippet:  strings.TrimSpace(agent.questionsSnippet),
				ReasoningFindings: formattedReasoningFindings,
			})
			if err != nil {
				return agentResult{}, err
			}
			nudged := append(append([]llm.Message(nil), historyMessages...), llm.Message{Role: "user", Content: nudgeText})
			nudgeReq := loopReq
			nudgeReq.AgentName = nudgeName
			nudgeReq.JSONRetryProgressAgentName = nudgeName
			nudgeReq.Messages = nudged
			nudgeReq.ReasoningEffort = nudgeReasoningEffort
			nudgeReq.State = nudgeState
			nudgeReq.OnReasoningTrace = nil
			sub, err := e.runAgentLoop(nudgeCtx, nudgeReq)
			if err != nil {
				nudgeErr = fmt.Errorf("nudge %d: %w", i+1, err)
				e.logfCtx(nudgeCtx, "Nudge failed, keeping prior findings: round=%d/%d error=%v", i+1, req.NudgeCount, err)
				break
			}
			if sub.resp == nil {
				nudgeErr = fmt.Errorf("nudge %d: agent %s returned no response", i+1, agent.name)
				e.logfCtx(nudgeCtx, "Nudge returned no response, keeping prior findings: round=%d/%d", i+1, req.NudgeCount)
				break
			}
			prevFindings := len(totalFindings)
			totalFindings = appendNewFindings(totalFindings, sub.resp.Findings)
			e.logfCtx(nudgeCtx, "Nudge findings: round=%d/%d returned=%d new=%d total=%d", i+1, req.NudgeCount, len(sub.resp.Findings), len(totalFindings)-prevFindings, len(totalFindings))
			totalTokens = addTokenUsage(totalTokens, sub.tokensUsed)
			totalToolCalls += sub.toolCalls
			totalDuplicates += sub.duplicateToolCalls
			latestResp = sub.resp
			latestReasoningEffort = sub.reasoningEffort
			if sub.reasoningEffort != "" {
				nudgeReasoningEffort = sub.reasoningEffort
			}
			historyMessages = messagesWithFinalResponse(sub.messages, sub.resp)
			contentMessages = append(contentMessages, sub.contentMessages...)
			toolMessages = append(toolMessages, sub.toolMessages...)
			toolCallHistory = append(toolCallHistory, sub.toolCallHistory...)
		}
		// Findings are the merged set across all nudge rounds, but RawResponse
		// is only the last round's raw payload. Do not re-parse RawResponse and
		// expect to recover the merged findings — use Findings directly.
		latest := *latestResp
		latest.Findings = totalFindings
		latestResp = &latest
		if nudgeErr != nil {
			run := model.AgentRun{
				Name:                  agent.name,
				Role:                  agent.role,
				Findings:              len(latestResp.Findings),
				MaxToolCalls:          req.MaxToolCalls,
				MaxDuplicateToolCalls: req.MaxDuplicateToolCalls,
				ToolCalls:             totalToolCallsWithExtractors(totalToolCalls),
				DuplicateToolCalls:    totalDuplicatesWithExtractors(totalDuplicates),
				TokensUsed:            totalTokensWithExtractors(totalTokens),
				Status:                model.AgentRunStatusPartial,
				Error:                 nudgeErr.Error(),
			}
			return agentResult{
				resp:               latestResp,
				reasoningEffort:    latestReasoningEffort,
				contentMessages:    contentMessages,
				toolMessages:       toolMessages,
				toolCallHistory:    toolCallHistory,
				duplicateToolCalls: totalDuplicatesWithExtractors(totalDuplicates),
				run:                run,
			}, nil
		}
	}
	return agentResult{
		resp:               latestResp,
		reasoningEffort:    latestReasoningEffort,
		contentMessages:    contentMessages,
		toolMessages:       toolMessages,
		toolCallHistory:    toolCallHistory,
		duplicateToolCalls: totalDuplicatesWithExtractors(totalDuplicates),
		run: model.AgentRun{
			Name:                  agent.name,
			Role:                  agent.role,
			Findings:              len(latestResp.Findings),
			MaxToolCalls:          req.MaxToolCalls,
			MaxDuplicateToolCalls: req.MaxDuplicateToolCalls,
			ToolCalls:             totalToolCallsWithExtractors(totalToolCalls),
			DuplicateToolCalls:    totalDuplicatesWithExtractors(totalDuplicates),
			TokensUsed:            totalTokensWithExtractors(totalTokens),
		},
	}, nil
}

func (e *Engine) runReasoningCollectFindings(ctx context.Context, reasoning, parentName string, turnIdx int, req model.ReviewRequest) (string, agentResult, error) {
	name := fmt.Sprintf("reasoning-extract:%s:collect:turn-%d", parentName, turnIdx)
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
		role:       "reasoning_extract",
		system:     system,
		user:       user,
		schemaKind: llm.SchemaKindText,
		hasTools:   false,
	}, reasoningExtractRequest(req))
	out := reasoningExtractOutput(result.contentMessages)
	if err == nil {
		extractCtx := ctxWithAgent(ctx, agentTag{Role: "reasoning_extract", Name: name})
		if out != "" {
			e.logBlockCtx(extractCtx, "Extracted reasoning findings:", out)
		} else {
			e.logfCtx(extractCtx, "No reasoning findings extracted")
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
		name:       fmt.Sprintf("reasoning-extract:%s:update", parentName),
		role:       "reasoning_extract",
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

func (e *Engine) renderReviewSystemWithQuestions(template, focusName, questionsSnippet string, req model.ReviewRequest, hasTools bool, styleGuideToolchainSnippet string) (string, error) {
	focusSnippet, err := e.renderReviewerFocusSnippet(focusName, questionsSnippet)
	if err != nil {
		return "", err
	}
	return e.renderReviewSystemWithFocus(template, focusSnippet, req, hasTools, styleGuideToolchainSnippet)
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

func (e *Engine) renderReviewSystemWithFocus(template, focusSnippet string, req model.ReviewRequest, hasTools bool, styleGuideToolchainSnippet string) (string, error) {
	toolInstructions := ""
	if hasTools {
		var err error
		toolInstructions, err = e.renderToolInstructions(toolInstructionsConfig{
			kind:                     "review",
			parallelToolCallGuidance: !req.DisableParallelToolCalls,
		})
		if err != nil {
			return "", err
		}
	}
	outputSchemaSnippet := reviewOutputSchemaSnippetFor(req.UseJSONSchema)
	commonSnippets, err := agentCommonSystemPromptSnippets("review", outputSchemaSnippet)
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
	kind                     string
	parallelToolCallGuidance bool
	toolNames                []string
}

func (e *Engine) renderToolInstructions(config toolInstructionsConfig) (string, error) {
	template, err := e.loadPrompt("tool_instructions.tmpl")
	if err != nil {
		return "", err
	}
	rendered, err := llm.RenderPrompt(template, struct {
		Kind                     string
		ParallelToolCallGuidance bool
		ToolListing              string
	}{
		Kind:                     config.kind,
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

func failedVectorResult(vector reviewVector, err error) agentResult {
	return agentResult{
		resp: &llm.ReviewResponse{},
		run: model.AgentRun{
			Name:   vector.name,
			Role:   "reviewer",
			Status: model.AgentRunStatusFailed,
			Error:  err.Error(),
		},
	}
}

// synthesizedMergeFromVectors produces a merge fallback when the merge agent
// itself fails. It concatenates findings from every successful vector reviewer
// and deduplicates via appendNewFindings (same id-title + title-location keys
// the nudge loop uses), so downstream aggregation code can treat it like any
// merge result.
func synthesizedMergeFromVectors(vectorResults []agentResult, mergeErr error) agentResult {
	var merged []model.Finding
	for _, result := range vectorResults {
		if result.run.Status == model.AgentRunStatusFailed || result.resp == nil {
			continue
		}
		merged = appendNewFindings(merged, result.resp.Findings)
	}
	// Re-mint IDs locally: two vectors may emit distinct findings sharing the
	// same valid UUID, which appendNewFindings keeps because dedup runs on
	// title-location keys. EnsureFindingIDs upstream only replaces invalid
	// UUIDs, so collisions would otherwise survive into the verifier.
	remintDuplicateFindingIDs(merged)
	resp := &llm.ReviewResponse{
		Findings:           merged,
		OverallCorrectness: "",
		OverallExplanation: "Merge agent unavailable; findings are the union of per-vector reviewers, deduped locally.",
	}
	status := model.AgentRunStatusFailed
	errStr := ""
	if mergeErr != nil {
		errStr = mergeErr.Error()
	} else {
		status = model.AgentRunStatusSkipped
	}
	return agentResult{
		resp: resp,
		run: model.AgentRun{
			Name:   "Merge Findings",
			Role:   "merge",
			Status: status,
			Error:  errStr,
		},
	}
}

// remintDuplicateFindingIDs replaces colliding valid UUIDs in-place so that
// downstream verifiers can address each finding by ID.
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

func vectorReviewPayloads(results []agentResult) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, result := range results {
		entry := map[string]any{
			"name": result.run.Name,
			"role": result.run.Role,
		}
		if result.run.Status == model.AgentRunStatusFailed {
			entry["status"] = model.AgentRunStatusFailed
			if result.run.Error != "" {
				entry["error"] = result.run.Error
			}
			out = append(out, entry)
			continue
		}
		if result.resp != nil {
			entry["findings"] = result.resp.Findings
			entry["overall_correctness"] = result.resp.OverallCorrectness
			entry["overall_explanation"] = result.resp.OverallExplanation
			entry["overall_confidence_score"] = result.resp.OverallConfidenceScore
		}
		if result.run.Status != "" {
			entry["status"] = result.run.Status
		}
		out = append(out, entry)
	}
	return out
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
		return llm.FindingsWithIDExamplePromptSnippet()
	}
	if kind == llm.SchemaKindFinalize {
		return llm.FinalizeExamplePromptSnippet()
	}
	return llm.FindingsExamplePromptSnippet()
}

func noToolsMessages(systemTemplate string, messages []llm.Message, snippet string, styleGuideToolchainSnippet string) ([]llm.Message, error) {
	commonSnippets, err := agentCommonSystemPromptSnippets("review", snippet)
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
	rendered, err := renderPromptFile("helper_json_snippet.tmpl", struct {
		Reason         string
		MissingFields  string
		ExampleSnippet string
	}{
		Reason:         invalid.Reason,
		MissingFields:  strings.Join(invalid.MissingFields, ", "),
		ExampleSnippet: strings.TrimSpace(exampleSnippet),
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering JSON retry feedback prompt: %w", err)
	}
	return rendered, nil
}

func (e *Engine) loadPrompt(name string) (string, error) {
	e.logf("Loading prompt: source=embedded name=%s", name)
	return prompts.Load(name)
}

func renderPromptFile(name string, data any) (string, error) {
	tmpl, err := prompts.Load(name)
	if err != nil {
		return "", err
	}
	return llm.RenderPrompt(tmpl, data)
}

func agentCommonSystemPromptSnippet(kind string, snippet string, outputSchemaSnippet string) (string, error) {
	rendered, err := renderPromptFile("agent_common_system_prompt_snippet.tmpl", struct {
		Kind                string
		Snippet             string
		OutputSchemaSnippet string
	}{
		Kind:                kind,
		Snippet:             snippet,
		OutputSchemaSnippet: outputSchemaSnippet,
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering common system prompt snippet %q for %s: %w", snippet, kind, err)
	}
	return rendered, nil
}

type agentCommonSystemPromptSnippetSet struct {
	findingInstructions string
	priority            string
	outputFormat        string
}

func agentCommonSystemPromptSnippets(kind string, outputSchemaSnippet string) (agentCommonSystemPromptSnippetSet, error) {
	findingInstructions, err := agentCommonSystemPromptSnippet(kind, "findings", "")
	if err != nil {
		return agentCommonSystemPromptSnippetSet{}, err
	}
	priority, err := agentCommonSystemPromptSnippet(kind, "priority", "")
	if err != nil {
		return agentCommonSystemPromptSnippetSet{}, err
	}
	outputFormatSnippet := "output_format"
	if outputSchemaSnippet == "" {
		outputFormatSnippet = "response_format"
	}
	outputFormat, err := agentCommonSystemPromptSnippet(kind, outputFormatSnippet, outputSchemaSnippet)
	if err != nil {
		return agentCommonSystemPromptSnippetSet{}, err
	}
	return agentCommonSystemPromptSnippetSet{
		findingInstructions: findingInstructions,
		priority:            priority,
		outputFormat:        outputFormat,
	}, nil
}

func mustRenderPromptFile(name string, data any) string {
	rendered, err := renderPromptFile(name, data)
	if err != nil {
		panic(fmt.Sprintf("review: rendering prompt %s: %v", name, err))
	}
	return rendered
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

func (e *Engine) renderStyleGuideToolchainSnippet(action string, guides []model.StyleGuide, hasToolchainVersions bool) (string, error) {
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
		Action               string
		StyleGuideTitles     []string
		HasToolchainVersions bool
	}{
		Action:               strings.TrimSpace(action),
		StyleGuideTitles:     titles,
		HasToolchainVersions: hasToolchainVersions,
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering styleguide/toolchain prompt: %w", err)
	}
	return strings.TrimSpace(rendered), nil
}

func styleGuideTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
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
	}
	for _, file := range ctx.ChangedFiles {
		addLanguage(seen, styleGuideLanguageForPath(file.Path))
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
	return llm.FindingsWithIDExamplePromptSnippet()
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
	return reviewOutputSchemaSnippetFor(useJSONSchema)
}

func (e *Engine) executeToolCalls(ctx context.Context, repoRoot string, toolCalls []llm.ToolCall, state *toolRoundState) []llm.Message {
	results := make([]llm.Message, 0, len(toolCalls))
	if len(toolCalls) == 0 {
		return results
	}
	results = make([]llm.Message, len(toolCalls))
	groups := make(map[string][]int, len(toolCalls))
	groupOrder := make([]string, 0, len(toolCalls))
	for i, toolCall := range toolCalls {
		key := toolCallConcurrencyKey(toolCall, i)
		if _, ok := groups[key]; !ok {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], i)
	}
	var wg sync.WaitGroup
	wg.Add(len(groupOrder))
	for _, key := range groupOrder {
		indexes := append([]int(nil), groups[key]...)
		go func(indexes []int) {
			defer wg.Done()
			for _, i := range indexes {
				toolCall := toolCalls[i]
				result := e.executeToolCall(ctx, repoRoot, toolCall, state)
				results[i] = llm.Message{
					Role:       "tool",
					ToolCallID: toolCall.ID,
					Name:       toolCall.Name,
					Content:    result,
				}
				e.logToolCall(toolCall, result)
			}
		}(indexes)
	}
	wg.Wait()
	return results
}

func toolCallConcurrencyKey(toolCall llm.ToolCall, index int) string {
	uniqueKey := fmt.Sprintf("unique\x00%d\x00%s", index, toolCall.ID)
	switch toolCall.Name {
	case "inspect_file":
		var args struct {
			Path string `json:"path"`
		}
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		return fmt.Sprintf("inspect_file\x00%s", normalizeToolPath(args.Path))
	case "list_files":
		var args struct {
			Path  string `json:"path"`
			Depth int    `json:"depth"`
		}
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		if args.Depth <= 0 {
			args.Depth = 1
		}
		return fmt.Sprintf("list_files\x00%s\x00%d", normalizeToolPath(args.Path), args.Depth)
	case "find_callers", "find_callees":
		var args struct {
			Path   string `json:"path"`
			Symbol string `json:"symbol"`
			Depth  int    `json:"depth"`
		}
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		if args.Depth <= 0 {
			args.Depth = 10
		}
		return callHierarchyDedupKey(toolCall.Name, normalizeToolPath(args.Path), strings.TrimSpace(args.Symbol), args.Depth)
	case "search":
		var args struct {
			Path  string `json:"path"`
			Query string `json:"query"`
		}
		if err := llm.LenientUnmarshal(toolCall.Arguments, &args); err != nil {
			return uniqueKey
		}
		query := strings.TrimSpace(args.Query)
		if matches := searchFunctionQueryPattern.FindStringSubmatch(query); len(matches) == 2 {
			return callHierarchyDedupKey("find_callers", normalizeToolPath(args.Path), matches[1], 10)
		}
		return uniqueKey
	default:
		return uniqueKey
	}
}

func (e *Engine) executeToolCall(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {
	if e.retrieval == nil {
		return toolError("", "retrieval_unavailable", toolErrorMessage(toolErrorData{Code: "retrieval_unavailable"}))
	}
	switch toolCall.Name {
	case "inspect_file":
		return e.executeInspectFile(ctx, repoRoot, toolCall, state)
	case "list_files":
		return e.executeListFiles(ctx, repoRoot, toolCall, state)
	case "search":
		return e.executeSearch(ctx, repoRoot, toolCall, state)
	case "find_callers":
		return e.executeCallHierarchy(ctx, repoRoot, toolCall, true, state)
	case "find_callees":
		return e.executeCallHierarchy(ctx, repoRoot, toolCall, false, state)
	default:
		return toolError("", "unsupported_tool", toolErrorMessage(toolErrorData{Code: "unsupported_tool", ToolName: toolCall.Name}))
	}
}

func (e *Engine) executeInspectFile(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {

	var args struct {
		Path      string `json:"path"`
		LineStart int    `json:"line_start"`
		LineEnd   int    `json:"line_end"`
	}
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	args.Path = strings.TrimSpace(args.Path)
	if args.Path == "" {
		return toolError("", "missing_argument", missingToolArgumentMessage(toolCall.Name, "path"))
	}
	normalizedPath := normalizeToolPath(args.Path)
	unlock := state.fileLocks.lock(normalizedPath)
	defer unlock()
	state.mu.Lock()
	seenContent, ok := state.seenFiles[normalizedPath]
	state.mu.Unlock()
	if ok {
		e.logfCtx(ctx, "Skipping duplicate tool call: name=%s path=%s", toolCall.Name, normalizedPath)
		return toolError(seenContent.Path, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_file"}))
	}

	if args.LineStart > 0 || args.LineEnd > 0 {
		e.logfCtx(ctx, "Executing tool call: name=%s path=%s line_start=%d line_end=%d", toolCall.Name, normalizedPath, args.LineStart, args.LineEnd)
		content, err := e.retrieval.GetFileSlice(ctx, repoRoot, normalizedPath, args.LineStart, args.LineEnd)
		if err != nil {
			return toolError(normalizedPath, "retrieval_failed", err.Error())
		}
		requested := model.LineRange{Start: content.StartLine, End: content.EndLine}
		state.mu.Lock()
		covered := rangeAlreadyCovered(state.seenFileRanges[normalizedPath], requested)
		if !covered {
			state.seenFileRanges[normalizedPath] = append(state.seenFileRanges[normalizedPath], requested)
		}
		state.mu.Unlock()
		if covered {
			e.logfCtx(ctx, "Skipping duplicate tool call: name=%s path=%s line_start=%d line_end=%d", toolCall.Name, normalizedPath, requested.Start, requested.End)
			return toolError(content.Path, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_file"}))
		}
		return mustToolResultJSON(map[string]any{
			"path":       content.Path,
			"start_line": content.StartLine,
			"end_line":   content.EndLine,
			"language":   content.Language,
			"content":    content.Content,
		})
	}

	e.logfCtx(ctx, "Executing tool call: name=%s path=%s", toolCall.Name, normalizedPath)
	content, err := e.retrieval.GetFile(ctx, repoRoot, normalizedPath)
	if err != nil {
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}
	payload := mustToolResultJSON(map[string]any{
		"path":     content.Path,
		"language": content.Language,
		"content":  content.Content,
	})
	state.mu.Lock()
	state.seenFiles[normalizedPath] = *content
	state.mu.Unlock()
	return payload
}

func (e *Engine) executeListFiles(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {
	var args struct {
		Path  string `json:"path"`
		Depth int    `json:"depth"`
	}
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	args.Path = strings.TrimSpace(args.Path)
	if args.Depth <= 0 {
		args.Depth = 1
	}
	normalizedPath := normalizeToolPath(args.Path)
	key := fmt.Sprintf("list_files\x00%s\x00%d", normalizedPath, args.Depth)
	unlock := state.toolLocks.lock(key)
	defer unlock()
	state.mu.Lock()
	_, ok := state.seenToolCalls[key]
	state.mu.Unlock()
	if ok {
		e.logfCtx(ctx, "Skipping duplicate tool call: name=%s path=%s depth=%d", toolCall.Name, normalizedPath, args.Depth)
		return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
	}
	e.logfCtx(ctx, "Executing tool call: name=%s path=%s depth=%d", toolCall.Name, normalizedPath, args.Depth)
	listing, err := e.retrieval.ListFiles(ctx, repoRoot, normalizedPath, args.Depth)
	if err != nil {
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}
	state.mu.Lock()
	state.seenToolCalls[key] = struct{}{}
	state.mu.Unlock()
	return mustToolResultJSON(map[string]any{
		"path":  listing.Path,
		"depth": args.Depth,
		"files": listing.Files,
	})
}

func (e *Engine) executeSearch(ctx context.Context, repoRoot string, toolCall llm.ToolCall, state *toolRoundState) string {
	var args struct {
		Path          string `json:"path"`
		Query         string `json:"query"`
		ContextLines  int    `json:"context_lines"`
		MaxResults    int    `json:"max_results"`
		CaseSensitive bool   `json:"case_sensitive"`
	}
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	args.Path = strings.TrimSpace(args.Path)
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return toolError(normalizeToolPath(args.Path), "missing_argument", missingToolArgumentMessage(toolCall.Name, "query"))
	}
	if args.ContextLines < 0 {
		args.ContextLines = 5
	}
	normalizedPath := normalizeToolPath(args.Path)
	if e.searchToolOptimization {
		if matches := searchFunctionQueryPattern.FindStringSubmatch(args.Query); len(matches) == 2 {
			symbol := matches[1]
			key := callHierarchyDedupKey("find_callers", normalizedPath, symbol, 10)
			state.mu.Lock()
			_, ok := state.seenToolCalls[key]
			state.mu.Unlock()
			if ok {
				e.logfCtx(ctx, "Skipping duplicate optimized tool call: name=%s path=%s query=%q rewritten=find_callers symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Query, symbol, 10)
				return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
			}
			e.logfCtx(ctx, "Rewriting tool call: name=%s path=%s query=%q rewritten=find_callers symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Query, symbol, 10)
			return e.executeCallHierarchy(ctx, repoRoot, llm.ToolCall{
				ID:   toolCall.ID,
				Name: "find_callers",
				Arguments: mustToolResultJSON(map[string]any{
					"path":   normalizedPath,
					"symbol": symbol,
					"depth":  10,
				}),
			}, true, state)
		}
	}
	e.logfCtx(ctx, "Executing tool call: name=%s path=%s query=%q context_lines=%d max_results=%d case_sensitive=%t", toolCall.Name, normalizedPath, args.Query, args.ContextLines, args.MaxResults, args.CaseSensitive)
	results, err := e.retrieval.Search(ctx, repoRoot, normalizedPath, args.Query, args.ContextLines, args.MaxResults, args.CaseSensitive)
	if err != nil {
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}

	if hasRegexMetachar(args.Query) {
		regexPattern := args.Query
		if !args.CaseSensitive {
			regexPattern = "(?i)" + regexPattern
		}
		if compiled, compileErr := regexp.Compile(regexPattern); compileErr == nil {
			e.logfCtx(ctx, "Executing regex search: name=%s path=%s pattern=%q context_lines=%d max_results=%d", toolCall.Name, normalizedPath, compiled.String(), args.ContextLines, args.MaxResults)
			regexResults, err := e.retrieval.SearchRegex(ctx, repoRoot, normalizedPath, compiled, args.ContextLines, args.MaxResults)
			if err != nil {
				return toolError(normalizedPath, "retrieval_failed", err.Error())
			}
			merged := mergeSearchResults(results.Results, regexResults.Results, args.MaxResults)
			results.Results = merged
			results.ResultCount = len(merged)
		} else {
			e.logfCtx(ctx, "Skipping regex search: name=%s path=%s pattern=%q error=%v", toolCall.Name, normalizedPath, regexPattern, compileErr)
		}
	}

	return mustToolResultJSON(map[string]any{
		"path":           results.Path,
		"query":          results.Query,
		"context_lines":  results.ContextLines,
		"max_results":    results.MaxResults,
		"case_sensitive": results.CaseSensitive,
		"result_count":   results.ResultCount,
		"results":        results.Results,
	})
}

func hasRegexMetachar(query string) bool {
	return strings.ContainsAny(query, `\.+*?()|[]{}^$`)
}

func mergeSearchResults(literal, regex []retrieval.SearchResult, maxResults int) []retrieval.SearchResult {
	merged := make([]retrieval.SearchResult, 0, len(literal)+len(regex))
	seen := make(map[string]struct{}, len(literal)+len(regex))
	key := func(r retrieval.SearchResult) string {
		return fmt.Sprintf("%s:%d:%d", r.Path, r.StartLine, r.EndLine)
	}
	for _, r := range literal {
		k := key(r)
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		merged = append(merged, r)
	}
	for _, r := range regex {
		k := key(r)
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		merged = append(merged, r)
	}
	if maxResults > 0 && len(merged) > maxResults {
		merged = merged[:maxResults]
	}
	return merged
}

func (e *Engine) executeCallHierarchy(ctx context.Context, repoRoot string, toolCall llm.ToolCall, callers bool, state *toolRoundState) string {
	var args struct {
		Symbol string `json:"symbol"`
		Path   string `json:"path"`
		Depth  int    `json:"depth"`
	}
	if err := parseToolArguments(toolCall.Name, toolCall.Arguments, &args); err != nil {
		return toolError("", "invalid_arguments", err.Error())
	}
	args.Symbol = strings.TrimSpace(args.Symbol)
	args.Path = strings.TrimSpace(args.Path)
	if args.Symbol == "" {
		return toolError(normalizeToolPath(args.Path), "missing_argument", missingToolArgumentMessage(toolCall.Name, "symbol"))
	}
	if args.Depth <= 0 {
		args.Depth = 10
	}
	normalizedPath := normalizeToolPath(args.Path)
	key := callHierarchyDedupKey(toolCall.Name, normalizedPath, args.Symbol, args.Depth)
	unlock := state.toolLocks.lock(key)
	defer unlock()
	state.mu.Lock()
	_, ok := state.seenToolCalls[key]
	state.mu.Unlock()
	if ok {
		e.logfCtx(ctx, "Skipping duplicate tool call: name=%s path=%s symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Symbol, args.Depth)
		return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
	}
	e.logfCtx(ctx, "Executing tool call: name=%s path=%s symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Symbol, args.Depth)

	symbol := retrieval.SymbolRef{Name: args.Symbol, Path: normalizedPath}
	var (
		hierarchy *retrieval.CallHierarchy
		err       error
	)
	if callers {
		hierarchy, err = e.retrieval.FindCallers(ctx, repoRoot, symbol, args.Depth)
	} else {
		hierarchy, err = e.retrieval.FindCallees(ctx, repoRoot, symbol, args.Depth)
	}
	if err != nil {
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}
	state.mu.Lock()
	state.seenToolCalls[key] = struct{}{}
	state.mu.Unlock()
	return mustToolResultJSON(map[string]any{
		"symbol": args.Symbol,
		"path":   normalizedPath,
		"mode":   hierarchy.Mode,
		"depth":  hierarchy.Depth,
		"root":   hierarchy.Root,
	})
}

func callHierarchyDedupKey(name, path, symbol string, depth int) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%d", name, path, symbol, depth)
}

func mustToolResultJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return mustToolResultJSON(map[string]any{
			"status": "error",
			"error": map[string]any{
				"code":    "encoding_failed",
				"message": toolErrorMessage(toolErrorData{Code: "encoding_failed"}),
			},
		})
	}
	return string(data)
}

type toolErrorData = toolcatalog.ErrorData

func toolErrorMessage(data toolErrorData) string {
	return toolcatalog.ErrorMessage(data)
}

func toolArgumentSchema(name string) string {
	return toolcatalog.ArgumentSchema(name)
}

func missingToolArgumentMessage(toolName, argument string) string {
	return toolErrorMessage(toolErrorData{
		Code:     "missing_argument",
		Argument: argument,
		Schema:   toolArgumentSchema(toolName),
	})
}

func parseToolArguments(toolName string, raw string, dst any) error {
	if err := llm.LenientUnmarshal(raw, dst); err != nil {
		schema := toolArgumentSchema(toolName)
		if schema == "" {
			return fmt.Errorf("invalid tool arguments for %s: %v; received: %s", toolName, err, raw)
		}
		return fmt.Errorf("invalid tool arguments for %s: %v; expected %s; received: %s", toolName, err, schema, raw)
	}
	return nil
}

func toolError(path, code, message string) string {
	payload := map[string]any{
		"status": "error",
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	if path != "" {
		payload["path"] = path
	}
	return mustToolResultJSON(payload)
}

func agentLoopKind(role string) string {
	switch role {
	case "context":
		return "context"
	case "reviewer":
		return "review"
	case "verify":
		return "verify"
	default:
		return role
	}
}

func (e *Engine) renderSyntheticToolFollowup(history []toolCallHistoryEntry, kind string) (string, error) {
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
		Kind          string
	}{
		History:       items,
		RetryLastTool: lastResult.IsError && lastResult.Code != "already_requested",
		Kind:          kind,
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
	var args struct {
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

func syntheticToolArguments(toolName string, args struct {
	Path          string `json:"path"`
	LineStart     int    `json:"line_start"`
	LineEnd       int    `json:"line_end"`
	Depth         int    `json:"depth"`
	Symbol        string `json:"symbol"`
	Query         string `json:"query"`
	ContextLines  int    `json:"context_lines"`
	MaxResults    int    `json:"max_results"`
	CaseSensitive bool   `json:"case_sensitive"`
}) string {
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
			args.ContextLines = 5
		}
		parts = append(parts, fmt.Sprintf("context_lines=%d", args.ContextLines))
		parts = append(parts, fmt.Sprintf("max_results=%d", args.MaxResults))
		parts = append(parts, fmt.Sprintf("case_sensitive=%t", args.CaseSensitive))
	case "find_callers", "find_callees":
		parts = append(parts, fmt.Sprintf("path=%q", syntheticPathValue(args.Path, ".")))
		parts = append(parts, fmt.Sprintf("symbol=%q", args.Symbol))
		if args.Depth <= 0 {
			args.Depth = 10
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
	label := sec.Label()
	if label != "" {
		e.logProgress("Request", fmt.Sprintf("[%s] #%d", label, callNum))
		e.logProgress("Reasoning", fmt.Sprintf("[%s] #%d", label, callNum))
	}
	previousSink := req.ReasoningSink
	callSec := e.openReviewRequestReasoningSection(label, callNum)
	req.ReasoningSink = llm.TeeReasoningSinks(callSec, previousSink)
	defer func() {
		req.ReasoningSink = previousSink
		callSec.End()
	}()
	start := time.Now()
	resp, err := e.llm.Review(ctx, req)
	elapsed := time.Since(start).Truncate(time.Second)
	if label != "" {
		if resp != nil && resp.Reasoned {
			e.logProgress("Reasoning", fmt.Sprintf("[%s] #%d Done %s", label, callNum, elapsed))
		}
		e.logProgress("Response", fmt.Sprintf("[%s] #%d After %s", label, callNum, elapsed))
	}
	return resp, err
}

func (e *Engine) openReviewRequestReasoningSection(label string, callNum int) *logging.ReasoningSection {
	if e.logger == nil || !e.logger.ShowReasoning() {
		return nil
	}
	if label == "" || callNum <= 0 {
		return e.logger.OpenReasoningSection("")
	}
	return e.logger.OpenReasoningSection(fmt.Sprintf("%s #%d", label, callNum))
}

func (e *Engine) logf(format string, args ...any) {
	if e.logger != nil {
		e.logger.Printf(format, args...)
	}
}

func (e *Engine) logfCtx(ctx context.Context, format string, args ...any) {
	if e.logger == nil {
		return
	}
	e.logger.Printf("%s%s", agentLogPrefix(ctx), fmt.Sprintf(format, args...))
}

func (e *Engine) logBlock(label, content string) {
	if e.logger != nil {
		e.logger.PrintBlock(label, content)
	}
}

func (e *Engine) logBlockCtx(ctx context.Context, label, content string) {
	if e.logger != nil {
		e.logger.PrintBlock(agentLogPrefix(ctx)+label, content)
	}
}

func (e *Engine) logJSON(label string, value any) {
	if e.logger != nil {
		e.logger.PrintJSON(label, value)
	}
}

type agentTag struct {
	Role string
	Name string
	Turn int
}

type agentTagKey struct{}

func ctxWithAgent(ctx context.Context, tag agentTag) context.Context {
	return context.WithValue(ctx, agentTagKey{}, tag)
}

func agentTagFromContext(ctx context.Context) (agentTag, bool) {
	if ctx == nil {
		return agentTag{}, false
	}
	tag, ok := ctx.Value(agentTagKey{}).(agentTag)
	if !ok || (tag.Role == "" && tag.Name == "") {
		return agentTag{}, false
	}
	return tag, true
}

func agentLogPrefix(ctx context.Context) string {
	tag, ok := agentTagFromContext(ctx)
	if !ok {
		return ""
	}
	return "[" + formatAgentTag(tag) + "] "
}

func agentLabelForLLM(ctx context.Context) string {
	tag, ok := agentTagFromContext(ctx)
	if !ok {
		return ""
	}
	return formatAgentTag(tag)
}

func formatAgentTag(tag agentTag) string {
	head := fmt.Sprintf("%s: %s", tag.Role, tag.Name)
	if tag.Turn > 0 {
		head = fmt.Sprintf("%s, turn: #%d", head, tag.Turn)
	}
	return head
}

func (e *Engine) logProgress(label, summary string) {
	if e.logger != nil {
		e.logger.PrintProgress(label, summary)
	}
}

func (e *Engine) logToolCall(toolCall llm.ToolCall, result string) {
	if e.logger == nil {
		return
	}
	e.logger.PrintProgressToolCall(toolCallDisplay(toolCall), syntheticToolOutcome(toolCall.Name, parseToolResultSummary(result)))
}

func syntheticToolArgumentsForCall(toolCall llm.ToolCall) string {
	var args struct {
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
	findArgs := syntheticToolArguments("find_callers", struct {
		Path          string `json:"path"`
		LineStart     int    `json:"line_start"`
		LineEnd       int    `json:"line_end"`
		Depth         int    `json:"depth"`
		Symbol        string `json:"symbol"`
		Query         string `json:"query"`
		ContextLines  int    `json:"context_lines"`
		MaxResults    int    `json:"max_results"`
		CaseSensitive bool   `json:"case_sensitive"`
	}{
		Path:   normalizedPath,
		Symbol: matches[1],
		Depth:  10,
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
