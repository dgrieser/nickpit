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
}

var searchFunctionQueryPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\((?:\))?$`)

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
	result.BaseURL = e.config.BaseURL
	result.BaseRef = reviewCtx.Repository.BaseRef
	result.HeadRef = reviewCtx.Repository.HeadRef
	return result, enrichedCtx, nil
}

func (e *Engine) reviewWithoutTools(ctx context.Context, llmReq *llm.ReviewRequest, systemTemplate string, messages []llm.Message, systemSnippet string, maxOutputRetries int, sec *logging.ReasoningSection) (*llm.ReviewResponse, error) {
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
		if !errors.As(err, &invalidResp) || !outputRetriesRemaining(attempt, maxOutputRetries) {
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
		feedback, err := e.renderJSONRetryFeedback(invalidResp, exampleSnippet)
		if err != nil {
			return nil, err
		}
		llmReq.Messages = append(llmReq.Messages, llm.Message{Role: "user", Content: feedback})
	}
}

type reviewAgent struct {
	name          string
	role          string
	system        string
	noToolsSystem string
	user          string
	extraMessages []llm.Message
	schema        []byte
	schemaKind    llm.SchemaKind
	hasTools      bool
}

type reviewAgentResult struct {
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
	systemPrompt, err := e.renderReviewSystemWithFocus(systemTemplate, "", req, true)
	if err != nil {
		return nil, nil, err
	}
	noToolsSystem, err := e.renderReviewSystemWithFocus(systemTemplate, "", req, false)
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
		AgentRuns:              []model.AgentRun{run.run},
		TokensUsed:             run.run.TokensUsed,
		TotalToolCalls:         run.run.ToolCalls,
		ReasoningEffort:        run.reasoningEffort,
	}, reviewCtx, nil
}

var reviewVectors = []struct {
	name      string
	focusFile string
}{
	{"Code Quality", "agent_review_codequality_system_prompt.tmpl"},
	{"Security", "agent_review_security_system_prompt.tmpl"},
	{"Architecture", "agent_review_architecture_system_prompt.tmpl"},
	{"Performance", "agent_review_performance_system_prompt.tmpl"},
	{"Testing", "agent_review_testing_system_prompt.tmpl"},
	{"Best Practices", "agent_review_bestpractices_system_prompt.tmpl"},
}

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
	userPrompt, err := llm.RenderJSON(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("review: rendering review prompt json: %w", err)
	}
	e.logf("Rendered review context JSON: lines=%d chars=%d", lineCount(userPrompt), len(userPrompt))

	contextResult, err := e.runContextAgent(ctx, reviewAgent{
		name:          "context",
		role:          "context",
		system:        contextSystem,
		noToolsSystem: contextSystem,
		user:          userPrompt,
		schemaKind:    llm.SchemaKindText,
		hasTools:      true,
	}, req)
	if err != nil {
		return nil, nil, err
	}

	enriched := model.CloneContext(reviewCtx)
	enriched.SupplementalContext = append(enriched.SupplementalContext, supplementalFromContextAgent(contextResult.toolMessages)...)
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
	contextMessages := contextAgentMarkdownMessages(contextResult.contentMessages)
	vectorResults, err := e.runVectorAgents(ctx, baseTemplate, enrichedPrompt, contextMessages, schema, req)
	if err != nil {
		return nil, nil, err
	}

	mergeResult, err := e.runMergeAgent(ctx, enrichedPrompt, contextAgentMarkdownContent(contextResult.contentMessages), vectorResults, schema, req)
	if err != nil {
		return nil, nil, err
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
		TotalToolCalls:         toolCalls,
		ReasoningEffort:        effectiveReasoningEffort,
	}, enriched, nil
}

func (e *Engine) runVectorAgents(ctx context.Context, baseTemplate, userPrompt string, contextMessages []llm.Message, schema []byte, req model.ReviewRequest) ([]reviewAgentResult, error) {
	results := make([]reviewAgentResult, len(reviewVectors))
	errs := make([]error, len(reviewVectors))
	var wg sync.WaitGroup
	for i, vector := range reviewVectors {
		wg.Add(1)
		go func(idx int, vector struct {
			name      string
			focusFile string
		}) {
			defer wg.Done()
			system, err := e.renderReviewSystem(baseTemplate, vector.focusFile, req, true)
			if err != nil {
				errs[idx] = err
				return
			}
			noToolsSystem, err := e.renderReviewSystem(baseTemplate, vector.focusFile, req, false)
			if err != nil {
				errs[idx] = err
				return
			}
			result, err := e.runReviewAgent(ctx, reviewAgent{
				name:          vector.name,
				role:          "reviewer",
				system:        system,
				noToolsSystem: noToolsSystem,
				user:          userPrompt,
				extraMessages: contextMessages,
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

func (e *Engine) runMergeAgent(ctx context.Context, userPrompt string, contextNotes string, vectorResults []reviewAgentResult, schema []byte, req model.ReviewRequest) (reviewAgentResult, error) {
	systemTemplate, err := e.loadPrompt("agent_merge_system_prompt.tmpl")
	if err != nil {
		return reviewAgentResult{}, err
	}
	commonSnippets, err := agentCommonSystemPromptSnippets("merge", reviewOutputSchemaSnippetFor(req.UseJSONSchema))
	if err != nil {
		return reviewAgentResult{}, err
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
		return reviewAgentResult{}, fmt.Errorf("review: rendering merge system prompt: %w", err)
	}
	mergeUser, err := llm.RenderJSON(map[string]any{
		"review_context":      json.RawMessage(userPrompt),
		"context_agent_notes": contextNotes,
		"vector_reviews":      vectorReviewPayloads(vectorResults),
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

func (e *Engine) runContextAgent(ctx context.Context, agent reviewAgent, req model.ReviewRequest) (contextAgentResult, error) {
	result, err := e.runReviewAgent(ctx, agent, req)
	if err != nil {
		return contextAgentResult{}, err
	}
	return contextAgentResult{
		run:                result.run,
		reasoningEffort:    result.reasoningEffort,
		contentMessages:    result.contentMessages,
		toolMessages:       result.toolMessages,
		toolCallHistory:    result.toolCallHistory,
		duplicateToolCalls: result.duplicateToolCalls,
	}, nil
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

func (e *Engine) runReviewAgent(ctx context.Context, agent reviewAgent, req model.ReviewRequest) (reviewAgentResult, error) {
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
	reviewSnippet := reviewOutputSchemaSnippetFor(req.UseJSONSchema)
	if agent.schemaKind == llm.SchemaKindText {
		reviewSnippet = ""
	}
	loopResult, err := e.runAgentLoop(ctx, agentLoopRequest{
		AgentName:                  agent.name,
		AgentKind:                  agentLoopKind(agent.role),
		Messages:                   messages,
		Tools:                      tools,
		Schema:                     agent.schema,
		SchemaKind:                 agent.schemaKind,
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
		Section:                    sec,
		NoToolsSystem:              noToolsSystem,
		NoToolsSchemaSnippet:       reviewSnippet,
		JSONRetryExampleSnippet:    llm.FindingsExamplePromptSnippet(),
		JSONRetryProgressAgentName: agent.name,
		NoToolsMessages: func(messages []llm.Message) ([]llm.Message, error) {
			if !agent.hasTools {
				return append([]llm.Message(nil), messages...), nil
			}
			return noToolsMessagesFromRendered(noToolsSystem, messages)
		},
	})
	if err != nil {
		return reviewAgentResult{}, err
	}
	return reviewAgentResult{
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

func (e *Engine) renderReviewSystem(template, focusName string, req model.ReviewRequest, hasTools bool) (string, error) {
	focusSnippet, err := e.loadPrompt(focusName)
	if err != nil {
		return "", err
	}
	return e.renderReviewSystemWithFocus(template, focusSnippet, req, hasTools)
}

func (e *Engine) renderReviewSystemWithFocus(template, focusSnippet string, req model.ReviewRequest, hasTools bool) (string, error) {
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
	}{
		OutputSchemaSnippet:        outputSchemaSnippet,
		FindingInstructionsSnippet: commonSnippets.findingInstructions,
		PrioritySnippet:            commonSnippets.priority,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		ParallelToolCallGuidance:   !req.DisableParallelToolCalls,
		HasTools:                   hasTools,
		FocusSnippet:               strings.TrimSpace(focusSnippet),
		ToolInstructions:           toolInstructions,
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering review system prompt: %w", err)
	}
	return systemPrompt, nil
}

type toolInstructionsConfig struct {
	kind                     string
	parallelToolCallGuidance bool
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
		ToolListing:              toolInstructionsListing(),
	})
	if err != nil {
		return "", fmt.Errorf("review: rendering tool instructions prompt: %w", err)
	}
	return rendered, nil
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
	return llm.FindingsExamplePromptSnippet()
}

func noToolsMessages(systemTemplate string, messages []llm.Message, snippet string) ([]llm.Message, error) {
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
	}{
		OutputSchemaSnippet:        snippet,
		FindingInstructionsSnippet: commonSnippets.findingInstructions,
		PrioritySnippet:            commonSnippets.priority,
		OutputFormatSnippet:        commonSnippets.outputFormat,
		HasTools:                   false,
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
	outputFormat, err := agentCommonSystemPromptSnippet(kind, "output_format", outputSchemaSnippet)
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
		})
	}
	return guides, nil
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
		e.logf("Skipping duplicate tool call: name=%s path=%s", toolCall.Name, normalizedPath)
		return toolError(seenContent.Path, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_file"}))
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
		return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
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
				e.logf("Skipping duplicate optimized tool call: name=%s path=%s query=%q rewritten=find_callers symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Query, symbol, 10)
				return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
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
		e.logf("Skipping duplicate tool call: name=%s path=%s symbol=%q depth=%d", toolCall.Name, normalizedPath, args.Symbol, args.Depth)
		return toolError(normalizedPath, "already_requested", toolErrorMessage(toolErrorData{Code: "already_requested_tool"}))
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

type toolErrorData struct {
	Code     string
	ToolName string
	Argument string
	Schema   string
	Message  string
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
