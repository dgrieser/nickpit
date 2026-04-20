package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/debuglog"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/prompts"
)

type Engine struct {
	source    model.ReviewSource
	llm       llm.Client
	retrieval retrieval.Engine
	config    config.Profile
	trimmer   *Trimmer
	logger    *debuglog.Logger
}

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

func NewEngine(source model.ReviewSource, llmClient llm.Client, retrievalEngine retrieval.Engine, profile config.Profile) *Engine {
	return &Engine{
		source:    source,
		llm:       llmClient,
		retrieval: retrievalEngine,
		config:    profile,
	}
}

func (e *Engine) SetLogger(logger *debuglog.Logger) {
	e.logger = logger
}

func (e *Engine) Run(ctx context.Context, req model.ReviewRequest) (*model.ReviewResult, error) {
	e.logf("Starting review: mode=%s repo=%s id=%d submode=%s repo_root=%s", req.Mode, req.Repo, req.Identifier, req.Submode, req.RepoRoot)
	reviewCtx, err := e.source.ResolveContext(ctx, req)
	if err != nil {
		return nil, err
	}
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
	e.logJSON("Rendered review context JSON:", trimmed)
	systemTemplate, err := e.loadPrompt("review_system.tmpl")
	if err != nil {
		return nil, err
	}
	systemPrompt, err := llm.RenderPrompt(systemTemplate, struct {
		OutputSchemaSnippet string
	}{
		OutputSchemaSnippet: reviewOutputSchemaSnippetFor(req.UseJSONSchema),
	})
	if err != nil {
		return nil, fmt.Errorf("review: rendering review system prompt: %w", err)
	}
	userPrompt, err := llm.RenderJSON(model.PromptPayloadFromContext(trimmed))
	if err != nil {
		return nil, fmt.Errorf("review: rendering review prompt json: %w", err)
	}

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
				Description: "Retrieve the complete contents of one repo-relative file for code review.",
				Parameters:  inspectFileToolParameters,
			},
			{
				Name:        "list_files",
				Description: "List files in one repo-relative folder to discover candidate files for review.",
				Parameters:  listFilesToolParameters,
			},
			{
				Name:        "search",
				Description: "Search recursively for a string in one repo-relative file or folder and return matching snippets.",
				Parameters:  searchToolParameters,
			},
		},
		Schema:          schema,
		Model:           e.config.Model,
		MaxTokens:       e.config.MaxTokens,
		Temperature:     e.config.Temperature,
		ReasoningEffort: e.config.ReasoningEffort,
	}

	totalUsage := model.TokenUsage{}
	toolRoundsUsed := 0
	seenFiles := make(map[string]retrieval.FileContent)
	var resp *llm.ReviewResponse
	var syntheticFollowup *llm.Message
	var toolCallHistory []toolCallHistoryEntry

	for {
		llmReq.Messages = messages
		if syntheticFollowup != nil {
			llmReq.Messages = append(append([]llm.Message(nil), messages...), *syntheticFollowup)
		}
		resp, err = e.llm.Review(ctx, llmReq)
		if err != nil {
			return nil, err
		}
		totalUsage.PromptTokens += resp.TokensUsed.PromptTokens
		totalUsage.CompletionTokens += resp.TokensUsed.CompletionTokens
		totalUsage.TotalTokens += resp.TokensUsed.TotalTokens

		if len(resp.ToolCalls) == 0 {
			break
		}
		if req.ToolRounds > 0 && toolRoundsUsed >= req.ToolRounds {
			resp = toolRoundLimitResponse(req.ToolRounds, resp.ToolCalls)
			break
		}
		e.logf("Executing tool round: round=%d tool_calls=%d", toolRoundsUsed+1, len(resp.ToolCalls))
		assistantMessage := llm.Message{Role: "assistant", ToolCalls: resp.ToolCalls}
		messages = append(messages, assistantMessage)
		toolMessages := e.executeToolCalls(ctx, req.RepoRoot, resp.ToolCalls, seenFiles)
		messages = append(messages, toolMessages...)
		toolCallHistory = append(toolCallHistory, collectToolCallHistory(resp.ToolCalls, toolMessages)...)
		syntheticFollowup = &llm.Message{
			Role:    "user",
			Content: syntheticToolFollowup(toolCallHistory),
		}
		toolRoundsUsed++
	}

	filtered := filterByPriority(resp.Findings, req.PriorityThreshold)
	e.logf("Review complete: findings=%d filtered=%d threshold=%s", len(resp.Findings), len(filtered), req.PriorityThreshold)
	return &model.ReviewResult{
		Findings:               filtered,
		OverallCorrectness:     resp.OverallCorrectness,
		OverallExplanation:     resp.OverallExplanation,
		OverallConfidenceScore: resp.OverallConfidenceScore,
		TokensUsed:             totalUsage,
		Model:                  e.config.Model,
		Mode:                   string(req.Mode),
		Repo:                   req.Repo,
		Identifier:             req.Identifier,
		ToolRounds:             toolRoundsUsed,
	}, nil
}

func (e *Engine) loadPrompt(name string) (string, error) {
	e.logf("Loading prompt: source=embedded name=%s", name)
	return prompts.Load(name)
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

func toolRoundLimitResponse(limit int, toolCalls []llm.ToolCall) *llm.ReviewResponse {
	names := make([]string, 0, len(toolCalls))
	for _, call := range toolCalls {
		names = append(names, call.Name)
	}
	return &llm.ReviewResponse{
		Findings: []model.Finding{
			{
				Title:           "[P2] Return final review JSON instead of more tool calls",
				Body:            fmt.Sprintf("The model requested additional tool calls after reaching the configured tool-call limit (%d), so the review could not be finalized as structured JSON.", limit),
				ConfidenceScore: 0.2,
				Priority:        priorityPtr(2),
				CodeLocation: model.CodeLocation{
					FilePath: "",
					LineRange: model.LineRange{
						Start: 1,
						End:   1,
					},
				},
			},
		},
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     fmt.Sprintf("tool round limit reached after tool calls: %s", strings.Join(names, ", ")),
		OverallConfidenceScore: 0.2,
		ToolCalls:              toolCalls,
	}
}

func (e *Engine) executeToolCalls(ctx context.Context, repoRoot string, toolCalls []llm.ToolCall, seenFiles map[string]retrieval.FileContent) []llm.Message {
	results := make([]llm.Message, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		results = append(results, llm.Message{
			Role:       "tool",
			ToolCallID: toolCall.ID,
			Name:       toolCall.Name,
			Content:    e.executeToolCall(ctx, repoRoot, toolCall, seenFiles),
		})
	}
	return results
}

func (e *Engine) executeToolCall(ctx context.Context, repoRoot string, toolCall llm.ToolCall, seenFiles map[string]retrieval.FileContent) string {
	if e.retrieval == nil {
		return toolError("", "retrieval_unavailable", "retrieval is unavailable for this review")
	}
	switch toolCall.Name {
	case "inspect_file":
		return e.executeInspectFile(ctx, repoRoot, toolCall, seenFiles)
	case "list_files":
		return e.executeListFiles(ctx, repoRoot, toolCall)
	case "search":
		return e.executeSearch(ctx, repoRoot, toolCall)
	default:
		return toolError("", "unsupported_tool", fmt.Sprintf("unsupported tool %q", toolCall.Name))
	}
}

func (e *Engine) executeInspectFile(ctx context.Context, repoRoot string, toolCall llm.ToolCall, seenFiles map[string]retrieval.FileContent) string {

	var args struct {
		Path      string `json:"path"`
		LineStart int    `json:"line_start"`
		LineEnd   int    `json:"line_end"`
	}
	if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err != nil {
		return toolError("", "invalid_arguments", fmt.Sprintf("invalid tool arguments: %v", err))
	}
	args.Path = strings.TrimSpace(args.Path)
	if args.Path == "" {
		return toolError("", "missing_argument", "missing required argument: path")
	}
	normalizedPath := normalizeToolPath(args.Path)
	if content, ok := seenFiles[normalizedPath]; ok {
		e.logf("Skipping duplicate tool call: name=%s path=%s", toolCall.Name, normalizedPath)
		return toolError(content.Path, "already_requested", "file contents were already provided for this review")
	}

	if args.LineStart > 0 || args.LineEnd > 0 {
		e.logf("Executing tool call: name=%s path=%s line_start=%d line_end=%d", toolCall.Name, normalizedPath, args.LineStart, args.LineEnd)
		content, err := e.retrieval.GetFileSlice(ctx, repoRoot, normalizedPath, args.LineStart, args.LineEnd)
		if err != nil {
			return toolError(normalizedPath, "retrieval_failed", err.Error())
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
	seenFiles[normalizedPath] = *content
	return payload
}

func (e *Engine) executeListFiles(ctx context.Context, repoRoot string, toolCall llm.ToolCall) string {
	var args struct {
		Path  string `json:"path"`
		Depth int    `json:"depth"`
	}
	if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err != nil {
		return toolError("", "invalid_arguments", fmt.Sprintf("invalid tool arguments: %v", err))
	}
	args.Path = strings.TrimSpace(args.Path)
	if args.Depth <= 0 {
		args.Depth = 1
	}
	normalizedPath := normalizeToolPath(args.Path)
	e.logf("Executing tool call: name=%s path=%s depth=%d", toolCall.Name, normalizedPath, args.Depth)
	listing, err := e.retrieval.ListFiles(ctx, repoRoot, normalizedPath, args.Depth)
	if err != nil {
		return toolError(normalizedPath, "retrieval_failed", err.Error())
	}
	return mustToolResultJSON(map[string]any{
		"path":  listing.Path,
		"depth": args.Depth,
		"files": listing.Files,
	})
}

func (e *Engine) executeSearch(ctx context.Context, repoRoot string, toolCall llm.ToolCall) string {
	var args struct {
		Path          string `json:"path"`
		Query         string `json:"query"`
		ContextLines  int    `json:"context_lines"`
		MaxResults    int    `json:"max_results"`
		CaseSensitive bool   `json:"case_sensitive"`
	}
	if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err != nil {
		return toolError("", "invalid_arguments", fmt.Sprintf("invalid tool arguments: %v", err))
	}
	args.Path = strings.TrimSpace(args.Path)
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return toolError(normalizeToolPath(args.Path), "missing_argument", "missing required argument: query")
	}
	if args.ContextLines < 0 {
		args.ContextLines = 5
	}
	normalizedPath := normalizeToolPath(args.Path)
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
		"results":        results.Results,
	})
}

func mustToolResultJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `{"status":"error","error":{"code":"encoding_failed","message":"failed to encode tool result"}}`
	}
	return string(data)
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
	lines := make([]string, 0, len(history)+4)
	for i, entry := range history {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, syntheticToolCallSummary(entry.ToolCall, entry.Result)))
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

func syntheticToolCallSummary(toolCall llm.ToolCall, result toolResultSummary) string {
	var args struct {
		Path          string `json:"path"`
		LineStart     int    `json:"line_start"`
		LineEnd       int    `json:"line_end"`
		Depth         int    `json:"depth"`
		Query         string `json:"query"`
		ContextLines  int    `json:"context_lines"`
		MaxResults    int    `json:"max_results"`
		CaseSensitive bool   `json:"case_sensitive"`
	}
	_ = json.Unmarshal([]byte(toolCall.Arguments), &args)
	path := strings.TrimSpace(args.Path)

	switch toolCall.Name {
	case "list_files":
		if path == "" {
			path = "<repo root>"
		}
		if args.Depth <= 0 {
			args.Depth = 1
		}
		return syntheticToolCallMessage(fmt.Sprintf("You requested to list files for %s with depth %d, see tool_call_id %s", path, args.Depth, toolCall.ID), result)
	case "inspect_file":
		if path == "" {
			path = "<path>"
		}
		if args.LineStart > 0 || args.LineEnd > 0 {
			return syntheticToolCallMessage(fmt.Sprintf("You requested to inspect file %s with line_start %d and line_end %d, see tool_call_id %s", path, args.LineStart, args.LineEnd, toolCall.ID), result)
		}
		return syntheticToolCallMessage(fmt.Sprintf("You requested to inspect file %s, see tool_call_id %s", path, toolCall.ID), result)
	case "search":
		if path == "" {
			path = "<repo root>"
		}
		if args.ContextLines < 0 {
			args.ContextLines = 5
		}
		return syntheticToolCallMessage(fmt.Sprintf("You requested to search for %q under %s with context_lines %d, max_results %d, and case_sensitive %t, see tool_call_id %s", args.Query, path, args.ContextLines, args.MaxResults, args.CaseSensitive, toolCall.ID), result)
	default:
		if path == "" {
			path = "<path>"
		}
		return syntheticToolCallMessage(fmt.Sprintf("You requested to call tool %s for %s, see tool_call_id %s", toolCall.Name, path, toolCall.ID), result)
	}
}

type toolResultSummary struct {
	IsError bool
	Code    string
	Message string
}

type toolCallHistoryEntry struct {
	ToolCall llm.ToolCall
	Result   toolResultSummary
}

func collectToolCallHistory(toolCalls []llm.ToolCall, toolMessages []llm.Message) []toolCallHistoryEntry {
	results := make(map[string]toolResultSummary, len(toolMessages))
	for _, msg := range toolMessages {
		results[msg.ToolCallID] = parseToolResultSummary(msg.Content)
	}

	history := make([]toolCallHistoryEntry, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		history = append(history, toolCallHistoryEntry{
			ToolCall: toolCall,
			Result:   results[toolCall.ID],
		})
	}
	return history
}

func parseToolResultSummary(content string) toolResultSummary {
	var payload struct {
		Status string `json:"status"`
		Error  struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return toolResultSummary{}
	}
	return toolResultSummary{
		IsError: payload.Status == "error",
		Code:    payload.Error.Code,
		Message: payload.Error.Message,
	}
}

func syntheticToolCallMessage(base string, result toolResultSummary) string {
	if !result.IsError || result.Message == "" {
		return base
	}
	return fmt.Sprintf("%s. This tool call failed: %s", base, result.Message)
}

func normalizeToolPath(path string) string {
	return strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), "./")
}

func priorityPtr(v int) *int {
	return &v
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
