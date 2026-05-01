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

func (e *Engine) Run(ctx context.Context, req model.ReviewRequest) (*model.ReviewResult, error) {
	e.logf("Starting review: mode=%s repo=%s id=%d submode=%s repo_root=%s", req.Mode, req.Repo, req.Identifier, req.Submode, req.RepoRoot)
	reviewCtx, err := e.source.ResolveContext(ctx, req)
	if err != nil {
		return nil, err
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
	systemTemplate, err := e.loadPrompt("review_system.tmpl")
	if err != nil {
		return nil, err
	}
	systemPrompt, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet      string
		ParallelToolCallGuidance bool
		HasTools                 bool
	}{
		OutputSchemaSnippet:      reviewOutputSchemaSnippetFor(req.UseJSONSchema),
		ParallelToolCallGuidance: !req.DisableParallelToolCalls,
		HasTools:                 true,
	})
	if err != nil {
		return nil, fmt.Errorf("review: rendering review system prompt: %w", err)
	}
	payload := model.PromptPayloadFromContext(trimmed)
	payload.StyleGuides, err = e.styleGuidesFor(trimmed)
	if err != nil {
		return nil, err
	}
	userPrompt, err := llm.RenderJSON(payload)
	if err != nil {
		return nil, fmt.Errorf("review: rendering review prompt json: %w", err)
	}
	e.logf("Rendered review context JSON: lines=%d chars=%d", lineCount(userPrompt), len(userPrompt))

	var schema []byte
	if req.UseJSONSchema {
		schema = llm.FindingsSchema
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	llmReq := &llm.ReviewRequest{
		Messages: messages,
		Tools: []llm.ToolDefinition{
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
		},
		Schema:            schema,
		Model:             e.config.Model,
		MaxTokens:         e.config.MaxTokens,
		Temperature:       e.config.Temperature,
		TopP:              e.config.TopP,
		ExtraBody:         e.config.ExtraBody,
		ParallelToolCalls: !req.DisableParallelToolCalls,
		ReasoningEffort:   e.config.ReasoningEffort,
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
	jsonRetries := 0
	effectiveReasoningEffort := e.config.ReasoningEffort

	for {
		llmReq.Messages = messages
		if syntheticFollowup != nil {
			llmReq.Messages = append(append([]llm.Message(nil), messages...), *syntheticFollowup)
		}
		resp, err = e.llm.Review(ctx, llmReq)
		if err != nil {
			var invalidResp *llm.InvalidResponseError
			if errors.As(err, &invalidResp) && jsonRetries < defaultMaxJSONRetries {
				if invalidResp.ReasoningEffort != "" {
					effectiveReasoningEffort = invalidResp.ReasoningEffort
					llmReq.ReasoningEffort = invalidResp.ReasoningEffort
				}
				jsonRetries++
				e.logf("Invalid JSON response, retrying with feedback: attempt=%d reason=%q missing=%v", jsonRetries, invalidResp.Reason, invalidResp.MissingFields)
				e.logProgress("Model", fmt.Sprintf("status=InvalidJsonRetry, attempt=%d", jsonRetries))
				if strings.TrimSpace(invalidResp.RawContent) != "" {
					messages = append(messages, llm.Message{Role: "assistant", Content: invalidResp.RawContent})
				}
				messages = append(messages, llm.Message{Role: "user", Content: buildJSONRetryFeedback(invalidResp)})
				syntheticFollowup = nil
				continue
			}
			return nil, err
		}
		if resp.ReasoningEffort != "" {
			effectiveReasoningEffort = resp.ReasoningEffort
			llmReq.ReasoningEffort = resp.ReasoningEffort
		}
		totalUsage.PromptTokens += resp.TokensUsed.PromptTokens
		totalUsage.CompletionTokens += resp.TokensUsed.CompletionTokens
		totalUsage.TotalTokens += resp.TokensUsed.TotalTokens

		if len(resp.ToolCalls) == 0 {
			break
		}
		pendingToolCalls := len(resp.ToolCalls)
		if req.MaxToolCalls > 0 && toolCallsUsed+pendingToolCalls > req.MaxToolCalls {
			e.logf("Tool call limit reached, making final call without tools: limit=%d used=%d requested=%d", req.MaxToolCalls, toolCallsUsed, pendingToolCalls)
			e.logProgress("Tool", fmt.Sprintf("status=LimitReached, limit=%d, finalizing review", req.MaxToolCalls))
			finalMessages := append([]llm.Message(nil), messages...)
			if strings.TrimSpace(resp.RawResponse) != "" {
				finalMessages = append(finalMessages, llm.Message{Role: "assistant", Content: resp.RawResponse})
			}
			resp, err = e.reviewWithoutTools(ctx, llmReq, systemTemplate, finalMessages, req.UseJSONSchema)
			if err != nil {
				return nil, err
			}
			if resp.ReasoningEffort != "" {
				effectiveReasoningEffort = resp.ReasoningEffort
				llmReq.ReasoningEffort = resp.ReasoningEffort
			}
			totalUsage.PromptTokens += resp.TokensUsed.PromptTokens
			totalUsage.CompletionTokens += resp.TokensUsed.CompletionTokens
			totalUsage.TotalTokens += resp.TokensUsed.TotalTokens
			break
		}
		e.logf("Executing tool batch: used=%d requested=%d", toolCallsUsed, pendingToolCalls)
		assistantMessage := llm.Message{Role: "assistant", Content: resp.RawResponse, ToolCalls: resp.ToolCalls}
		messages = append(messages, assistantMessage)
		toolMessages := e.executeToolCalls(ctx, req.RepoRoot, resp.ToolCalls, toolState)
		messages = append(messages, toolMessages...)
		toolCallHistory = append(toolCallHistory, collectToolCallHistory(resp.ToolCalls, toolMessages)...)
		duplicateToolCallsUsed += countDuplicateToolCalls(toolMessages)
		if req.MaxDuplicateToolCalls > 0 && duplicateToolCallsUsed >= req.MaxDuplicateToolCalls {
			e.logf("Duplicate tool call limit reached, making final call without tools: limit=%d duplicates=%d", req.MaxDuplicateToolCalls, duplicateToolCallsUsed)
			e.logProgress("Tool", fmt.Sprintf("status=DuplicateLimitReached, limit=%d, duplicates=%d, finalizing review", req.MaxDuplicateToolCalls, duplicateToolCallsUsed))
			toolCallsUsed += pendingToolCalls
			resp, err = e.reviewWithoutTools(ctx, llmReq, systemTemplate, messages, req.UseJSONSchema)
			if err != nil {
				return nil, err
			}
			if resp.ReasoningEffort != "" {
				effectiveReasoningEffort = resp.ReasoningEffort
				llmReq.ReasoningEffort = resp.ReasoningEffort
			}
			totalUsage.PromptTokens += resp.TokensUsed.PromptTokens
			totalUsage.CompletionTokens += resp.TokensUsed.CompletionTokens
			totalUsage.TotalTokens += resp.TokensUsed.TotalTokens
			break
		}
		syntheticFollowup = &llm.Message{
			Role:    "user",
			Content: syntheticToolFollowup(toolCallHistory),
		}
		toolCallsUsed += pendingToolCalls
	}

	filtered := filterByPriority(resp.Findings, req.PriorityThreshold)
	e.logf(
		"Review complete: findings=%d filtered=%d threshold=%s tool_calls=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d",
		len(resp.Findings),
		len(filtered),
		req.PriorityThreshold,
		len(toolCallHistory),
		totalUsage.PromptTokens,
		totalUsage.CompletionTokens,
		totalUsage.TotalTokens,
	)
	mode := string(req.Mode)
	if req.Submode != "" {
		mode = mode + ":" + req.Submode
	}
	return &model.ReviewResult{
		Findings:               filtered,
		OverallCorrectness:     resp.OverallCorrectness,
		OverallExplanation:     resp.OverallExplanation,
		OverallConfidenceScore: resp.OverallConfidenceScore,
		TokensUsed:             totalUsage,
		Model:                  e.config.Model,
		Mode:                   mode,
		Repo:                   req.Repo,
		Identifier:             req.Identifier,
		ToolCalls:              toolCallsUsed,
		MaxToolCalls:           req.MaxToolCalls,
		ReasoningEffort:        effectiveReasoningEffort,
		BaseURL:                e.config.BaseURL,
		BaseRef:                reviewCtx.Repository.BaseRef,
		HeadRef:                reviewCtx.Repository.HeadRef,
		MaxDuplicateToolCalls:  req.MaxDuplicateToolCalls,
		DuplicateToolCalls:     duplicateToolCallsUsed,
	}, nil
}

func (e *Engine) reviewWithoutTools(ctx context.Context, llmReq *llm.ReviewRequest, systemTemplate string, messages []llm.Message, useJSONSchema bool) (*llm.ReviewResponse, error) {
	noToolsPrompt, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet      string
		ParallelToolCallGuidance bool
		HasTools                 bool
	}{
		OutputSchemaSnippet: reviewOutputSchemaSnippetFor(useJSONSchema),
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
	finalMessages[0] = llm.Message{Role: "system", Content: noToolsPrompt}
	llmReq.Messages = finalMessages
	llmReq.Tools = nil
	for attempt := 0; ; attempt++ {
		resp, err := e.llm.Review(ctx, llmReq)
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
		llmReq.Messages = append(llmReq.Messages, llm.Message{Role: "user", Content: buildJSONRetryFeedback(invalidResp)})
	}
}

func buildJSONRetryFeedback(err *llm.InvalidResponseError) string {
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
	b.WriteString(llm.FindingsExamplePromptSnippet())
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
	for _, language := range []string{"go", "python", "javascript", "typescript", "html", "css", "scss", "csharp", "sql"} {
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
