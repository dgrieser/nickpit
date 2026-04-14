package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
)

type Engine struct {
	source    model.ReviewSource
	llm       llm.Client
	retrieval retrieval.Engine
	promptDir string
	config    config.Profile
	trimmer   *Trimmer
}

func NewEngine(source model.ReviewSource, llmClient llm.Client, retrievalEngine retrieval.Engine, profile config.Profile) *Engine {
	return &Engine{
		source:    source,
		llm:       llmClient,
		retrieval: retrievalEngine,
		promptDir: "prompts",
		config:    profile,
	}
}

func (e *Engine) Run(ctx context.Context, req model.ReviewRequest) (*model.ReviewResult, error) {
	reviewCtx, err := e.source.ResolveContext(ctx, req)
	if err != nil {
		return nil, err
	}

	if req.IncludeFullFiles && e.retrieval != nil && req.RepoRoot != "" {
		for _, file := range reviewCtx.ChangedFiles {
			content, err := e.retrieval.GetFile(ctx, req.RepoRoot, file.Path)
			if err != nil {
				continue
			}
			reviewCtx.SupplementalContext = append(reviewCtx.SupplementalContext, model.SupplementalFile{
				Path:     file.Path,
				Content:  joinLines(content.Lines),
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
	systemPrompt, err := e.loadPrompt(req.PromptOverride, "default_review.tmpl")
	if err != nil {
		return nil, err
	}
	userPrompt, err := llm.RenderPrompt(systemPrompt, trimmed)
	if err != nil {
		return nil, fmt.Errorf("review: rendering review prompt: %w", err)
	}

	llmReq := &llm.ReviewRequest{
		SystemPrompt: systemPrompt,
		UserContent:  userPrompt,
		Schema:       llm.FindingsSchema,
		Model:        e.config.Model,
		MaxTokens:    4096,
		Temperature:  0.2,
	}

	resp, err := e.llm.Review(ctx, llmReq)
	if err != nil {
		return nil, err
	}

	for round := 0; round < req.FollowUpRounds; round++ {
		if len(resp.FollowUpRequests) == 0 || e.retrieval == nil {
			break
		}
		reviewCtx.SupplementalContext = append(reviewCtx.SupplementalContext, ExecuteRetrievals(ctx, e.retrieval, req.RepoRoot, resp.FollowUpRequests)...)
		trimmed = trimmer.Trim(reviewCtx)

		followupTemplate, err := e.loadPrompt("", "followup_request.tmpl")
		if err != nil {
			return nil, err
		}
		userPrompt, err = llm.RenderPrompt(followupTemplate, trimmed)
		if err != nil {
			return nil, fmt.Errorf("review: rendering follow-up prompt: %w", err)
		}
		llmReq.UserContent = userPrompt
		resp, err = e.llm.Review(ctx, llmReq)
		if err != nil {
			return nil, err
		}
	}

	filtered := filterBySeverity(resp.Findings, req.SeverityThreshold)
	return &model.ReviewResult{
		Findings:      filtered,
		Summary:       resp.Summary,
		TokensUsed:    resp.TokensUsed,
		Model:         e.config.Model,
		Mode:          string(req.Mode),
		Repo:          req.Repo,
		Identifier:    req.Identifier,
		FollowUpRound: req.FollowUpRounds,
	}, nil
}

func (e *Engine) loadPrompt(override, fallback string) (string, error) {
	path := override
	if path == "" {
		path = e.config.PromptFile
	}
	if path == "" {
		path = filepath.Join(e.promptDir, fallback)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("review: reading prompt %s: %w", path, err)
	}
	return string(data), nil
}

func filterBySeverity(findings []model.Finding, threshold string) []model.Finding {
	minimum := model.SeverityRank(model.ParseSeverity(threshold))
	filtered := make([]model.Finding, 0, len(findings))
	for _, finding := range findings {
		if model.SeverityRank(finding.Severity) >= minimum {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for _, line := range lines[1:] {
		out += "\n" + line
	}
	return out
}
