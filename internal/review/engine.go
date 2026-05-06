package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
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
}

var searchFunctionQueryPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\((?:\))?$`)

const defaultMaxJSONRetries = 3

var inspectFileToolParameters = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Repo-relative file path"
    },
    "line_start": {
      "type": "integer",
      "description": "Optional starting line number for partial file retrieval",
      "minimum": 1
    },
    "line_end": {
      "type": "integer",
      "description": "Optional ending line number for partial file retrieval",
      "minimum": 1
    }
  },
  "required": ["path"],
  "additionalProperties": false
}`)

var listFilesToolParameters = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Repo-relative folder path; omit or pass an empty string to list the repo root"
    },
    "depth": {
      "type": "integer",
      "description": "Optional traversal depth for nested folders; defaults to 1",
      "minimum": 1
    }
  },
  "additionalProperties": false
}`)

var searchToolParameters = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Repo-relative file or folder path; omit or pass an empty string to search from the repo root"
    },
    "query": {
      "type": "string",
      "description": "Search string to find"
    },
    "context_lines": {
      "type": "integer",
      "description": "Optional number of surrounding lines to include before and after each match; defaults to 5",
      "minimum": 0
    },
    "max_results": {
      "type": "integer",
      "description": "Optional maximum number of matches to return; omit or pass 0 for unlimited",
      "minimum": 1
    },
    "case_sensitive": {
      "type": "boolean",
      "description": "Optional case-sensitive match mode; defaults to false"
    }
  },
  "required": ["query"],
  "additionalProperties": false
}`)

var callHierarchyToolParameters = json.RawMessage(`{
  "type": "object",
  "properties": {
    "symbol": {
      "type": "string",
      "description": "Function name to inspect"
    },
    "path": {
      "type": "string",
      "description": "Optional repo-relative file or folder path containing the function; omit or pass an empty string to search from the repo root"
    },
    "depth": {
      "type": "integer",
      "description": "Optional traversal depth for the call hierarchy; defaults to 10",
      "minimum": 1
    }
  },
  "required": ["symbol"],
  "additionalProperties": false
}`)

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
	}
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

func reviewerToolDefinitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Name:        "inspect_file",
			Description: "Retrieve content of repo-relative file",
			Parameters:  inspectFileToolParameters,
		},
		{
			Name:        "list_files",
			Description: "List files of repo-relative folder",
			Parameters:  listFilesToolParameters,
		},
		{
			Name:        "search",
			Description: "Search recursively inside repo-relative file or folder",
			Parameters:  searchToolParameters,
		},
		{
			Name:        "find_callers",
			Description: "Resolve function by symbol name and return caller hierarchy and method bodies",
			Parameters:  callHierarchyToolParameters,
		},
		{
			Name:        "find_callees",
			Description: "Resolve function by symbol name and return its callee hierarchy and method bodies",
			Parameters:  callHierarchyToolParameters,
		},
	}
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
	reviewCtx.CheckoutRoot = req.RepoRoot
	reviewCtx.Identifier = req.Identifier

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

	trimmed := trimmer.Trim(reviewCtx)
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
	result.MaxToolCalls = req.MaxToolCalls
	result.MaxDuplicateToolCalls = req.MaxDuplicateToolCalls
	result.BaseURL = e.config.BaseURL
	result.BaseRef = reviewCtx.Repository.BaseRef
	result.HeadRef = reviewCtx.Repository.HeadRef
	return result, enrichedCtx, nil
}

func (e *Engine) reviewWithoutTools(ctx context.Context, llmReq *llm.ReviewRequest, systemTemplate string, messages []llm.Message, systemSnippet string, sec *logging.ReasoningSection) (*llm.ReviewResponse, error) {
	finalMessages, err := noToolsMessages(systemTemplate, messages, systemSnippet)
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
		if !errors.As(err, &invalidResp) || attempt >= defaultMaxJSONRetries {
			return nil, err
		}
		if invalidResp.ReasoningEffort != "" {
			llmReq.ReasoningEffort = invalidResp.ReasoningEffort
		}
		e.logf("Invalid JSON response in no-tools call, retrying: attempt=%d reason=%q missing=%v", attempt+1, invalidResp.Reason, invalidResp.MissingFields)
		e.logProgress("Model", fmt.Sprintf("status=InvalidJsonRetry, attempt=%d", attempt+1))
		if strings.TrimSpace(invalidResp.RawContent) != "" {
			llmReq.Messages = append(llmReq.Messages, llm.Message{Role: "assistant", Content: invalidResp.RawContent})
		}
		llmReq.Messages = append(llmReq.Messages, llm.Message{Role: "user", Content: buildJSONRetryFeedback(invalidResp, exampleSnippet)})
	}
}

type reviewAgent struct {
	name          string
	role          string
	system        string
	noToolsSystem string
	user          string
	schema        []byte
	schemaKind    llm.SchemaKind
	hasTools      bool
}

type reviewAgentResult struct {
	resp               *llm.ReviewResponse
	run                model.AgentRun
	reasoningEffort    string
	toolMessages       []llm.Message
	toolCallHistory    []toolCallHistoryEntry
	duplicateToolCalls int
}

func (e *Engine) runSingleAgentReview(ctx context.Context, reviewCtx *model.ReviewContext, req model.ReviewRequest) (*model.ReviewResult, *model.ReviewContext, error) {
	systemTemplate, err := e.loadPrompt("review_system.tmpl")
	if err != nil {
		return nil, nil, err
	}
	systemPrompt, err := e.renderReviewSystem(systemTemplate, req, true)
	if err != nil {
		return nil, nil, err
	}
	noToolsSystem, err := e.renderReviewSystem(systemTemplate, req, false)
	if err != nil {
		return nil, nil, err
	}
	payload := model.PromptPayloadFromContext(reviewCtx)
	payload.StyleGuides, err = e.styleGuidesFor(reviewCtx)
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
	run, err := e.runReviewAgent(ctx, reviewAgent{
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
		TokensUsed:             run.run.TokensUsed,
		ToolCalls:              run.run.ToolCalls,
		DuplicateToolCalls:     run.run.DuplicateToolCalls,
		ReasoningEffort:        run.reasoningEffort,
	}, reviewCtx, nil
}

var reviewVectors = []struct {
	name        string
	description string
}{
	{"Code Quality", "Focus only on code quality and correctness, including logic errors, broken behavior, maintainability risks, and concrete edge cases."},
	{"Security", "Focus only on security issues, including trust boundaries, injection, secrets, authentication, authorization, unsafe parsing, and data exposure."},
	{"Architecture", "Focus only on architecture, boundaries, API shape, data flow, coupling, compatibility, and whether the design fits this codebase."},
	{"Performance", "Focus only on performance issues, including algorithmic cost, allocation, I/O, concurrency, caching, and avoidable remote or LLM calls."},
	{"Testing", "Focus only on test coverage gaps that materially affect confidence in changed behavior, failure modes, and regressions."},
	{"Best Practices", "Focus only on project conventions, language best practices, idioms, error handling, and avoidable complexity that is actionable."},
}

func (e *Engine) runMultiAgentReview(ctx context.Context, reviewCtx *model.ReviewContext, req model.ReviewRequest) (*model.ReviewResult, *model.ReviewContext, error) {
	baseTemplate, err := e.loadPrompt("review_system.tmpl")
	if err != nil {
		return nil, nil, err
	}
	baseSystemWithTools, err := e.renderReviewSystem(baseTemplate, req, true)
	if err != nil {
		return nil, nil, err
	}
	baseSystemNoTools, err := e.renderReviewSystem(baseTemplate, req, false)
	if err != nil {
		return nil, nil, err
	}
	payload := model.PromptPayloadFromContext(reviewCtx)
	payload.StyleGuides, err = e.styleGuidesFor(reviewCtx)
	if err != nil {
		return nil, nil, err
	}
	userPrompt, err := llm.RenderJSON(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("review: rendering review prompt json: %w", err)
	}
	e.logf("Rendered review context JSON: lines=%d chars=%d", lineCount(userPrompt), len(userPrompt))

	collectorSystem := baseSystemWithTools + "\n\n" + collectorPromptSuffix()
	collectorNoToolsSystem := baseSystemNoTools + "\n\n" + collectorPromptSuffix()
	collector, err := e.runReviewAgent(ctx, reviewAgent{
		name:          "collector",
		role:          "collector",
		system:        collectorSystem,
		noToolsSystem: collectorNoToolsSystem,
		user:          userPrompt,
		schemaKind:    llm.SchemaKindText,
		hasTools:      true,
	}, req)
	if err != nil {
		return nil, nil, err
	}

	enriched := model.CloneContext(reviewCtx)
	enriched.SupplementalContext = append(enriched.SupplementalContext, supplementalFromCollector(collector.resp.RawResponse, collector.toolMessages)...)
	payload = model.PromptPayloadFromContext(enriched)
	payload.StyleGuides, err = e.styleGuidesFor(enriched)
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
	vectorResults, err := e.runVectorAgents(ctx, baseSystemWithTools, baseSystemNoTools, enrichedPrompt, schema, req)
	if err != nil {
		return nil, nil, err
	}

	mergeResult, err := e.runMergeAgent(ctx, enrichedPrompt, collector, vectorResults, schema, req)
	if err != nil {
		return nil, nil, err
	}
	allRuns := make([]model.AgentRun, 0, 2+len(vectorResults))
	allRuns = append(allRuns, collector.run)
	totalUsage := collector.run.TokensUsed
	toolCalls := collector.run.ToolCalls
	duplicateToolCalls := collector.run.DuplicateToolCalls
	effectiveReasoningEffort := collector.reasoningEffort
	for _, result := range vectorResults {
		allRuns = append(allRuns, result.run)
		totalUsage = addTokenUsage(totalUsage, result.run.TokensUsed)
		toolCalls += result.run.ToolCalls
		duplicateToolCalls += result.run.DuplicateToolCalls
		if result.reasoningEffort != "" {
			effectiveReasoningEffort = result.reasoningEffort
		}
	}
	allRuns = append(allRuns, mergeResult.run)
	totalUsage = addTokenUsage(totalUsage, mergeResult.run.TokensUsed)
	if mergeResult.reasoningEffort != "" {
		effectiveReasoningEffort = mergeResult.reasoningEffort
	}

	filtered := filterByPriority(mergeResult.resp.Findings, req.PriorityThreshold)
	e.logf(
		"Review complete: findings=%d filtered=%d threshold=%s tool_calls=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d",
		len(mergeResult.resp.Findings),
		len(filtered),
		req.PriorityThreshold,
		toolCalls,
		totalUsage.PromptTokens,
		totalUsage.CompletionTokens,
		totalUsage.TotalTokens,
	)
	return &model.ReviewResult{
		Findings:               filtered,
		OverallCorrectness:     mergeResult.resp.OverallCorrectness,
		OverallExplanation:     mergeResult.resp.OverallExplanation,
		OverallConfidenceScore: mergeResult.resp.OverallConfidenceScore,
		AgentRuns:              allRuns,
		TokensUsed:             totalUsage,
		ToolCalls:              toolCalls,
		DuplicateToolCalls:     duplicateToolCalls,
		ReasoningEffort:        effectiveReasoningEffort,
	}, enriched, nil
}

func (e *Engine) runVectorAgents(ctx context.Context, baseSystem, baseNoToolsSystem, userPrompt string, schema []byte, req model.ReviewRequest) ([]reviewAgentResult, error) {
	results := make([]reviewAgentResult, len(reviewVectors))
	errs := make([]error, len(reviewVectors))
	var wg sync.WaitGroup
	for i, vector := range reviewVectors {
		wg.Add(1)
		go func(idx int, vector struct {
			name        string
			description string
		}) {
			defer wg.Done()
			system := baseSystem + "\n\n" + vectorPromptSuffix(vector.name, vector.description)
			noToolsSystem := baseNoToolsSystem + "\n\n" + vectorPromptSuffix(vector.name, vector.description)
			result, err := e.runReviewAgent(ctx, reviewAgent{
				name:          vector.name,
				role:          "reviewer",
				system:        system,
				noToolsSystem: noToolsSystem,
				user:          userPrompt,
				schema:        schema,
				schemaKind:    llm.SchemaKindReview,
				hasTools:      true,
			}, req)
			results[idx] = result
			errs[idx] = err
		}(i, vector)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("%s reviewer failed: %w", reviewVectors[i].name, err)
		}
	}
	return results, nil
}

func (e *Engine) runMergeAgent(ctx context.Context, userPrompt string, collector reviewAgentResult, vectorResults []reviewAgentResult, schema []byte, req model.ReviewRequest) (reviewAgentResult, error) {
	systemTemplate, err := e.loadPrompt("merge_system.tmpl")
	if err != nil {
		return reviewAgentResult{}, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet string
	}{
		OutputSchemaSnippet: reviewOutputSchemaSnippetFor(req.UseJSONSchema),
	})
	if err != nil {
		return reviewAgentResult{}, fmt.Errorf("review: rendering merge system prompt: %w", err)
	}
	mergeUser, err := llm.RenderJSON(map[string]any{
		"review_context":     json.RawMessage(userPrompt),
		"collector_response": collector.resp.RawResponse,
		"vector_reviews":     vectorReviewPayloads(vectorResults),
	})
	if err != nil {
		return reviewAgentResult{}, fmt.Errorf("review: rendering merge prompt json: %w", err)
	}
	return e.runReviewAgent(ctx, reviewAgent{
		name:          "merge",
		role:          "merge",
		system:        system,
		noToolsSystem: system,
		user:          mergeUser,
		schema:        schema,
		schemaKind:    llm.SchemaKindReview,
		hasTools:      false,
	}, req)
}

func (e *Engine) runReviewAgent(ctx context.Context, agent reviewAgent, req model.ReviewRequest) (reviewAgentResult, error) {
	noToolsSystem := agent.noToolsSystem
	if noToolsSystem == "" {
		noToolsSystem = agent.system
	}
	messages := []llm.Message{
		{Role: "system", Content: agent.system},
		{Role: "user", Content: agent.user},
	}
	label := fmt.Sprintf("%s: %s", agent.role, agent.name)
	if agent.role == "reviewer" && strings.HasPrefix(agent.name, "#") {
		label = "reviewer " + agent.name
	}
	sec := e.logger.NewReasoningTracker(label)
	defer sec.End()
	llmReq := &llm.ReviewRequest{
		Messages:          messages,
		Schema:            agent.schema,
		SchemaKind:        agent.schemaKind,
		Model:             e.config.Model,
		MaxTokens:         e.config.MaxTokens,
		Temperature:       e.config.Temperature,
		TopP:              e.config.TopP,
		ExtraBody:         e.config.ExtraBody,
		ParallelToolCalls: !req.DisableParallelToolCalls,
		ReasoningEffort:   e.config.ReasoningEffort,
	}
	if agent.hasTools {
		llmReq.Tools = reviewerToolDefinitions()
	}

	totalUsage := model.TokenUsage{}
	toolCallsUsed := 0
	duplicateToolCallsUsed := 0
	toolState := &toolRoundState{
		seenFiles:      make(map[string]retrieval.FileContent),
		seenFileRanges: make(map[string][]model.LineRange),
		seenToolCalls:  make(map[string]struct{}),
	}
	var resp *llm.ReviewResponse
	var syntheticFollowup *llm.Message
	var toolCallHistory []toolCallHistoryEntry
	var toolMessages []llm.Message
	jsonRetries := 0
	effectiveReasoningEffort := e.config.ReasoningEffort
	jsonRepairWithoutTools := false
	reviewSnippet := reviewOutputSchemaSnippetFor(req.UseJSONSchema)
	if agent.schemaKind == llm.SchemaKindText {
		reviewSnippet = ""
	}

	for {
		noToolsHistory := append([]llm.Message(nil), messages...)
		var err error
		if agent.hasTools {
			noToolsHistory, err = noToolsMessagesFromRendered(noToolsSystem, messages)
			if err != nil {
				return reviewAgentResult{}, err
			}
		}
		llmReq.NoToolsMessages = noToolsHistory
		llmReq.Messages = messages
		if syntheticFollowup != nil {
			llmReq.Messages = append(append([]llm.Message(nil), messages...), *syntheticFollowup)
		}
		resp, err = e.loggedReview(ctx, llmReq, sec)
		if err != nil {
			var invalidResp *llm.InvalidResponseError
			if errors.As(err, &invalidResp) && jsonRetries < defaultMaxJSONRetries {
				if invalidResp.ReasoningEffort != "" {
					effectiveReasoningEffort = invalidResp.ReasoningEffort
					llmReq.ReasoningEffort = invalidResp.ReasoningEffort
				}
				if invalidResp.ToolsOmitted || jsonRepairWithoutTools {
					jsonRepairWithoutTools = true
					messages = noToolsHistory
					llmReq.Tools = nil
					llmReq.ParallelToolCalls = false
				}
				jsonRetries++
				e.logf("Invalid JSON response, retrying with feedback: agent=%s attempt=%d reason=%q missing=%v", agent.name, jsonRetries, invalidResp.Reason, invalidResp.MissingFields)
				e.logProgress("Model", fmt.Sprintf("status=InvalidJsonRetry, agent=%s, attempt=%d", agent.name, jsonRetries))
				if strings.TrimSpace(invalidResp.RawContent) != "" {
					messages = append(messages, llm.Message{Role: "assistant", Content: invalidResp.RawContent})
				}
				messages = append(messages, llm.Message{Role: "user", Content: buildJSONRetryFeedback(invalidResp, llm.FindingsExamplePromptSnippet())})
				syntheticFollowup = nil
				continue
			}
			return reviewAgentResult{}, err
		}
		if resp.ReasoningEffort != "" {
			effectiveReasoningEffort = resp.ReasoningEffort
			llmReq.ReasoningEffort = resp.ReasoningEffort
		}
		totalUsage = addTokenUsage(totalUsage, resp.TokensUsed)

		if len(resp.ToolCalls) == 0 {
			break
		}
		pendingToolCalls := len(resp.ToolCalls)
		if req.MaxToolCalls > 0 && toolCallsUsed+pendingToolCalls > req.MaxToolCalls {
			e.logf("Tool call limit reached, making final call without tools: agent=%s limit=%d used=%d requested=%d", agent.name, req.MaxToolCalls, toolCallsUsed, pendingToolCalls)
			finalMessages := append([]llm.Message(nil), messages...)
			if strings.TrimSpace(resp.RawResponse) != "" {
				finalMessages = append(finalMessages, llm.Message{Role: "assistant", Content: resp.RawResponse})
			}
			noToolsReq := *llmReq
			noToolsReq.Tools = nil
			noToolsReq.ParallelToolCalls = false
			noToolsReq.Messages, err = noToolsMessagesFromRendered(noToolsSystem, finalMessages)
			if err != nil {
				return reviewAgentResult{}, err
			}
			resp, err = e.reviewWithoutTools(ctx, &noToolsReq, noToolsSystem, finalMessages, reviewSnippet, sec)
			if err != nil {
				return reviewAgentResult{}, err
			}
			totalUsage = addTokenUsage(totalUsage, resp.TokensUsed)
			break
		}
		e.logf("Executing tool batch: agent=%s used=%d requested=%d", agent.name, toolCallsUsed, pendingToolCalls)
		messages = append(messages, llm.Message{Role: "assistant", Content: resp.RawResponse, ToolCalls: resp.ToolCalls})
		batch := e.executeToolCalls(ctx, req.RepoRoot, resp.ToolCalls, toolState)
		messages = append(messages, batch...)
		toolMessages = append(toolMessages, batch...)
		toolCallHistory = append(toolCallHistory, collectToolCallHistory(resp.ToolCalls, batch)...)
		duplicateToolCallsUsed += countDuplicateToolCalls(batch)
		toolCallsUsed += pendingToolCalls
		if req.MaxDuplicateToolCalls > 0 && duplicateToolCallsUsed >= req.MaxDuplicateToolCalls {
			e.logf("Duplicate tool call limit reached, making final call without tools: agent=%s limit=%d duplicates=%d", agent.name, req.MaxDuplicateToolCalls, duplicateToolCallsUsed)
			noToolsReq := *llmReq
			noToolsReq.Tools = nil
			noToolsReq.ParallelToolCalls = false
			resp, err = e.reviewWithoutTools(ctx, &noToolsReq, noToolsSystem, messages, reviewSnippet, sec)
			if err != nil {
				return reviewAgentResult{}, err
			}
			totalUsage = addTokenUsage(totalUsage, resp.TokensUsed)
			break
		}
		syntheticFollowup = &llm.Message{Role: "user", Content: syntheticToolFollowup(toolCallHistory)}
	}

	if resp == nil {
		return reviewAgentResult{}, fmt.Errorf("agent %s returned no response", agent.name)
	}
	return reviewAgentResult{
		resp:               resp,
		reasoningEffort:    effectiveReasoningEffort,
		toolMessages:       toolMessages,
		toolCallHistory:    toolCallHistory,
		duplicateToolCalls: duplicateToolCallsUsed,
		run: model.AgentRun{
			Name:               agent.name,
			Role:               agent.role,
			Findings:           len(resp.Findings),
			ToolCalls:          toolCallsUsed,
			DuplicateToolCalls: duplicateToolCallsUsed,
			TokensUsed:         totalUsage,
		},
	}, nil
}

func (e *Engine) renderReviewSystem(template string, req model.ReviewRequest, hasTools bool) (string, error) {
	systemPrompt, err := llm.RenderPrompt(template, struct {
		OutputSchemaSnippet      string
		ParallelToolCallGuidance bool
		HasTools                 bool
	}{
		OutputSchemaSnippet:      reviewOutputSchemaSnippetFor(req.UseJSONSchema),
		ParallelToolCallGuidance: !req.DisableParallelToolCalls,
		HasTools:                 hasTools,
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering review system prompt: %w", err)
	}
	return systemPrompt, nil
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

func collectorPromptSuffix() string {
	var b strings.Builder
	b.WriteString("## COLLECTOR MODE\n")
	b.WriteString("Do not produce review findings. Gather context for later specialist reviewers.\n")
	b.WriteString("The following vectors will be reviewed: ")
	for i, vector := range reviewVectors {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(vector.name)
	}
	b.WriteString(".\nUse tools to collect all relevant files, snippets, listings, callers, callees, and searches needed for comprehensive review across all vectors. Return a concise inventory of what you inspected and why it matters.")
	return b.String()
}

func vectorPromptSuffix(name, description string) string {
	return fmt.Sprintf("## SPECIALIST REVIEW VECTOR: %s\n%s\nReturn only findings for this vector. Do not include findings whose main issue belongs to another vector.", name, description)
}

func vectorReviewPayloads(results []reviewAgentResult) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, result := range results {
		out = append(out, map[string]any{
			"name":                     result.run.Name,
			"role":                     result.run.Role,
			"findings":                 result.resp.Findings,
			"overall_correctness":      result.resp.OverallCorrectness,
			"overall_explanation":      result.resp.OverallExplanation,
			"overall_confidence_score": result.resp.OverallConfidenceScore,
		})
	}
	return out
}

func supplementalFromCollector(raw string, messages []llm.Message) []model.SupplementalFile {
	out := make([]model.SupplementalFile, 0, len(messages)+1)
	if strings.TrimSpace(raw) != "" {
		out = append(out, model.SupplementalFile{
			Path:    "collector/notes",
			Content: raw,
			Kind:    "collector_notes",
			Reason:  "context gathered by collector agent",
		})
	}
	for i, msg := range messages {
		path := collectorToolPath(msg.Content)
		if path == "" {
			path = fmt.Sprintf("collector/tool-%d", i+1)
		}
		out = append(out, model.SupplementalFile{
			Path:    path,
			Content: msg.Content,
			Kind:    "collector_tool_result",
			Reason:  "tool result gathered by collector agent",
		})
	}
	return out
}

func collectorToolPath(content string) string {
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
	return llm.FindingsExamplePromptSnippet()
}

func noToolsMessages(systemTemplate string, messages []llm.Message, snippet string) ([]llm.Message, error) {
	noToolsPrompt, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet      string
		ParallelToolCallGuidance bool
		HasTools                 bool
	}{
		OutputSchemaSnippet: snippet,
		HasTools:            false,
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

func buildJSONRetryFeedback(err *llm.InvalidResponseError, exampleSnippet string) string {
	var b strings.Builder
	b.WriteString("Your previous response could not be parsed as the expected JSON output: ")
	b.WriteString(err.Reason)
	b.WriteString(".")
	if len(err.MissingFields) > 0 {
		b.WriteString(" Missing or invalid fields: ")
		b.WriteString(strings.Join(err.MissingFields, ", "))
		b.WriteString(".")
	}
	b.WriteString("\n\nRespond again with ONLY a JSON object (no prose, no markdown fences) matching this shape:\n\n")
	if exampleSnippet == "" {
		exampleSnippet = llm.FindingsExamplePromptSnippet()
	}
	b.WriteString(exampleSnippet)
	return b.String()
}

func (e *Engine) loadPrompt(name string) (string, error) {
	e.logf("Loading prompt: source=embedded name=%s", name)
	return prompts.Load(name)
}

func (e *Engine) styleGuidesFor(ctx *model.ReviewContext) ([]model.StyleGuide, error) {
	languages := changedLanguages(ctx)
	guides := make([]model.StyleGuide, 0, len(languages))
	seenFiles := make(map[string]struct{})
	for _, language := range languages {
		name, ok := builtInStyleGuideFiles[language]
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
	return guides, nil
}

var builtInStyleGuideFiles = map[string]string{
	"go":         "styleguides/go.md",
	"python":     "styleguides/python.md",
	"javascript": "styleguides/javascript.md",
	"typescript": "styleguides/typescript.md",
	"html":       "styleguides/html-css.md",
	"css":        "styleguides/html-css.md",
	"scss":       "styleguides/html-css.md",
	"csharp":     "styleguides/csharp.md",
	"sql":        "styleguides/sql.md",
	"shell":      "styleguides/bash.md",
	"helm":       "styleguides/helm.md",
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
	for _, language := range []string{"go", "python", "javascript", "typescript", "html", "css", "scss", "csharp", "sql", "shell"} {
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
	if _, ok := builtInStyleGuideFiles[language]; !ok {
		return
	}
	seen[language] = struct{}{}
}

func styleGuideLanguageForPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".mjs", ".cjs", ".jsx":
		return "javascript"
	case ".ts", ".mts", ".cts", ".tsx":
		return "typescript"
	default:
		return filetype.DetectLanguage(path)
	}
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

func reviewOutputSchemaSnippetFor(useJSONSchema bool) string {
	if useJSONSchema {
		return ""
	}
	return llm.FindingsExamplePromptSnippet()
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
		return toolError("", "retrieval_unavailable", "retrieval is unavailable for this review")
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
		return toolError("", "unsupported_tool", fmt.Sprintf("unsupported tool %q", toolCall.Name))
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
		return toolError("", "missing_argument", fmt.Sprintf("missing required argument: path; expected %s", toolArgumentSchemas["inspect_file"]))
	}
	normalizedPath := normalizeToolPath(args.Path)
	unlock := state.fileLocks.lock(normalizedPath)
	defer unlock()
	state.mu.Lock()
	seenContent, ok := state.seenFiles[normalizedPath]
	state.mu.Unlock()
	if ok {
		e.logf("Skipping duplicate tool call: name=%s path=%s", toolCall.Name, normalizedPath)
		return toolError(seenContent.Path, "already_requested", "file contents were already provided for this review")
	}

	if args.LineStart > 0 || args.LineEnd > 0 {
		e.logf("Executing tool call: name=%s path=%s line_start=%d line_end=%d", toolCall.Name, normalizedPath, args.LineStart, args.LineEnd)
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
			e.logf("Skipping duplicate tool call: name=%s path=%s line_start=%d line_end=%d", toolCall.Name, normalizedPath, requested.Start, requested.End)
			return toolError(content.Path, "already_requested", "file contents were already provided for this review")
		}
		return mustToolResultJSON(map[string]any{
			"path":       content.Path,
			"start_line": content.StartLine,
			"end_line":   content.EndLine,
			"language":   content.Language,
			"content":    content.Content,
		})
	}

	e.logf("Executing tool call: name=%s path=%s", toolCall.Name, normalizedPath)
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
		e.logf("Skipping duplicate tool call: name=%s path=%s depth=%d", toolCall.Name, normalizedPath, args.Depth)
		return toolError(normalizedPath, "already_requested", "tool result was already provided for this review")
	}
	e.logf("Executing tool call: name=%s path=%s depth=%d", toolCall.Name, normalizedPath, args.Depth)
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
		return toolError(normalizeToolPath(args.Path), "missing_argument", fmt.Sprintf("missing required argument: query; expected %s", toolArgumentSchemas["search"]))
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
				e.logf("Skipping duplicate optimized tool call: name=%s path=%s query=%q rewritten=find_callers symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Query, symbol, 10)
				return toolError(normalizedPath, "already_requested", "tool result was already provided for this review")
			}
			e.logf("Rewriting tool call: name=%s path=%s query=%q rewritten=find_callers symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Query, symbol, 10)
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
	e.logf("Executing tool call: name=%s path=%s query=%q context_lines=%d max_results=%d case_sensitive=%t", toolCall.Name, normalizedPath, args.Query, args.ContextLines, args.MaxResults, args.CaseSensitive)
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
			e.logf("Executing regex search: name=%s path=%s pattern=%q context_lines=%d max_results=%d", toolCall.Name, normalizedPath, compiled.String(), args.ContextLines, args.MaxResults)
			regexResults, err := e.retrieval.SearchRegex(ctx, repoRoot, normalizedPath, compiled, args.ContextLines, args.MaxResults)
			if err != nil {
				return toolError(normalizedPath, "retrieval_failed", err.Error())
			}
			merged := mergeSearchResults(results.Results, regexResults.Results, args.MaxResults)
			results.Results = merged
			results.ResultCount = len(merged)
		} else {
			e.logf("Skipping regex search: name=%s path=%s pattern=%q error=%v", toolCall.Name, normalizedPath, regexPattern, compileErr)
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
		return toolError(normalizeToolPath(args.Path), "missing_argument", fmt.Sprintf("missing required argument: symbol; expected %s", toolArgumentSchemas[toolCall.Name]))
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
		e.logf("Skipping duplicate tool call: name=%s path=%s symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Symbol, args.Depth)
		return toolError(normalizedPath, "already_requested", "tool result was already provided for this review")
	}
	e.logf("Executing tool call: name=%s path=%s symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Symbol, args.Depth)

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
		return `{"status":"error","error":{"code":"encoding_failed","message":"failed to encode tool result"}}`
	}
	return string(data)
}

var toolArgumentSchemas = map[string]string{
	"inspect_file": `{"path": "<repo-relative path>", "line_start"?: int, "line_end"?: int}`,
	"list_files":   `{"path"?: "<repo-relative folder>", "depth"?: int}`,
	"search":       `{"path"?: "<repo-relative path>", "query": "<text>", "context_lines"?: int, "max_results"?: int, "case_sensitive"?: bool}`,
	"find_callers": `{"symbol": "<function name>", "path"?: "<repo-relative path>", "depth"?: int}`,
	"find_callees": `{"symbol": "<function name>", "path"?: "<repo-relative path>", "depth"?: int}`,
}

func parseToolArguments(toolName string, raw string, dst any) error {
	if err := llm.LenientUnmarshal(raw, dst); err != nil {
		schema := toolArgumentSchemas[toolName]
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

func syntheticToolFollowup(history []toolCallHistoryEntry) string {
	lines := make([]string, 0, len(history)+5)
	lines = append(lines, "You called the following tools already:")
	for i, entry := range history {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, syntheticToolCallSummary(entry)))
	}
	lines = append(lines, "")
	lastResult := toolResultSummary{}
	if len(history) > 0 {
		lastResult = history[len(history)-1].Result
	}
	if lastResult.IsError && lastResult.Code != "already_requested" {
		lines = append(lines, "Please retry the last tool call.")
	} else {
		lines = append(lines, "If you need more context, continue calling tools.")
		lines = append(lines, "Otherwise, if you have enough context to judge the patch, stop calling tools and return the final review as JSON.")
	}
	return strings.Join(lines, "\n")
}

func syntheticToolCallSummary(entry toolCallHistoryEntry) string {
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
	name := toolCall.Name
	if entry.OptimizedTo != "" {
		name = fmt.Sprintf("%s (replaced by %s)", toolCall.Name, entry.OptimizedTo)
	}
	return fmt.Sprintf("%s: tool_call_id=%q, arguments=[%s]; %s", name, toolCall.ID, syntheticToolArguments(toolCall.Name, args), syntheticToolOutcome(toolCall.Name, result))
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
	req.ReasoningSink = callSec
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

func (e *Engine) logBlock(label, content string) {
	if e.logger != nil {
		e.logger.PrintBlock(label, content)
	}
}

func (e *Engine) logJSON(label string, value any) {
	if e.logger != nil {
		e.logger.PrintJSON(label, value)
	}
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
