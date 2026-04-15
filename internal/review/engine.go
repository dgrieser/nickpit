package review

import (
	"context"
	"fmt"
	"os"

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
	systemPrompt, err := e.loadPrompt(req.ReviewSystemPromptFile, "review_system.tmpl")
	if err != nil {
		return nil, err
	}
	userTemplate, err := e.loadPrompt(req.ReviewUserPromptFile, "review_user.tmpl")
	if err != nil {
		return nil, err
	}
	userPrompt, err := llm.RenderPrompt(userTemplate, trimmed)
	if err != nil {
		return nil, fmt.Errorf("review: rendering review prompt: %w", err)
	}

	var schema []byte
	if req.UseJSONSchema {
		schema = llm.FindingsSchema
	}

	llmReq := &llm.ReviewRequest{
		SystemPrompt:    systemPrompt,
		UserContent:     userPrompt,
		Schema:          schema,
		Model:           e.config.Model,
		MaxTokens:       4096,
		Temperature:     0.2,
		ReasoningEffort: e.config.ReasoningEffort,
	}

	resp, err := e.llm.Review(ctx, llmReq)
	if err != nil {
		return nil, err
	}

	for round := 0; round < req.FollowUpRounds; round++ {
		if len(resp.FollowUpRequests) == 0 || e.retrieval == nil {
			e.logf("Follow-up loop stopped: round=%d requests=%d retrieval=%t", round+1, len(resp.FollowUpRequests), e.retrieval != nil)
			break
		}
		e.logf("Running follow-up round: round=%d requests=%d", round+1, len(resp.FollowUpRequests))
		reviewCtx.SupplementalContext = append(reviewCtx.SupplementalContext, ExecuteRetrievals(ctx, e.retrieval, req.RepoRoot, resp.FollowUpRequests, e.logf)...)
		trimmed = trimmer.Trim(reviewCtx)
		e.logf("Trimmed context: round=%d supplemental=%d omitted=%d", round+1, len(trimmed.SupplementalContext), len(trimmed.OmittedSections))
		e.logJSON("Rendered follow-up context JSON:", trimmed)

		systemPrompt, err = e.loadPrompt(req.FollowUpSystemPromptFile, "followup_system.tmpl")
		if err != nil {
			return nil, err
		}
		followupTemplate, err := e.loadPrompt(req.FollowUpUserPromptFile, "followup_user.tmpl")
		if err != nil {
			return nil, err
		}
		userPrompt, err = llm.RenderPrompt(followupTemplate, trimmed)
		if err != nil {
			return nil, fmt.Errorf("review: rendering follow-up prompt: %w", err)
		}
		llmReq.SystemPrompt = systemPrompt
		llmReq.UserContent = userPrompt
		resp, err = e.llm.Review(ctx, llmReq)
		if err != nil {
			return nil, err
		}
	}

	filtered := filterByPriority(resp.Findings, req.PriorityThreshold)
	e.logf("Review complete: findings=%d filtered=%d threshold=%s", len(resp.Findings), len(filtered), req.PriorityThreshold)
	return &model.ReviewResult{
		Findings:               filtered,
		OverallCorrectness:     resp.OverallCorrectness,
		OverallExplanation:     resp.OverallExplanation,
		OverallConfidenceScore: resp.OverallConfidenceScore,
		TokensUsed:             resp.TokensUsed,
		Model:                  e.config.Model,
		Mode:                   string(req.Mode),
		Repo:                   req.Repo,
		Identifier:             req.Identifier,
		FollowUpRound:          req.FollowUpRounds,
	}, nil
}

func (e *Engine) loadPrompt(path, fallback string) (string, error) {
	if path != "" {
		e.logf("Loading prompt: source=override path=%s", path)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("review: reading prompt %s: %w", path, err)
		}
		return string(data), nil
	}
	e.logf("Loading prompt: source=embedded name=%s", fallback)
	return prompts.Load(fallback)
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
