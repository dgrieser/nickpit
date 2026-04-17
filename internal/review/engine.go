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
    }
  },
  "required": ["path"],
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
		},
		Schema:          schema,
		Model:           e.config.Model,
		MaxTokens:       e.config.MaxTokens,
		Temperature:     e.config.Temperature,
		ReasoningEffort: e.config.ReasoningEffort,
	}

	totalUsage := model.TokenUsage{}
	toolRoundsUsed := 0
	var resp *llm.ReviewResponse

	for {
		llmReq.Messages = messages
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
		if toolRoundsUsed >= req.ToolRounds {
			resp = toolRoundLimitResponse(req.ToolRounds, resp.ToolCalls)
			break
		}
		e.logf("Executing tool round: round=%d tool_calls=%d", toolRoundsUsed+1, len(resp.ToolCalls))
		assistantMessage := llm.Message{Role: "assistant", ToolCalls: resp.ToolCalls}
		messages = append(messages, assistantMessage)
		toolMessages := e.executeToolCalls(ctx, req.RepoRoot, resp.ToolCalls)
		messages = append(messages, toolMessages...)
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

func (e *Engine) executeToolCalls(ctx context.Context, repoRoot string, toolCalls []llm.ToolCall) []llm.Message {
	results := make([]llm.Message, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		results = append(results, llm.Message{
			Role:       "tool",
			ToolCallID: toolCall.ID,
			Name:       toolCall.Name,
			Content:    e.executeToolCall(ctx, repoRoot, toolCall),
		})
	}
	return results
}

func (e *Engine) executeToolCall(ctx context.Context, repoRoot string, toolCall llm.ToolCall) string {
	if e.retrieval == nil {
		return mustToolResultJSON(map[string]any{
			"error": "retrieval is unavailable for this review",
		})
	}
	if toolCall.Name != "inspect_file" {
		return mustToolResultJSON(map[string]any{
			"error": fmt.Sprintf("unsupported tool %q", toolCall.Name),
		})
	}

	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err != nil {
		return mustToolResultJSON(map[string]any{
			"error": fmt.Sprintf("invalid tool arguments: %v", err),
		})
	}
	args.Path = strings.TrimSpace(args.Path)
	if args.Path == "" {
		return mustToolResultJSON(map[string]any{
			"error": "missing required argument: path",
		})
	}

	e.logf("Executing tool call: name=%s path=%s", toolCall.Name, args.Path)
	content, err := e.retrieval.GetFile(ctx, repoRoot, args.Path)
	if err != nil {
		return mustToolResultJSON(map[string]any{
			"path":  args.Path,
			"error": err.Error(),
		})
	}
	return mustToolResultJSON(map[string]any{
		"path":     content.Path,
		"language": content.Language,
		"content":  content.Content,
	})
}

func mustToolResultJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `{"error":"failed to encode tool result"}`
	}
	return string(data)
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
