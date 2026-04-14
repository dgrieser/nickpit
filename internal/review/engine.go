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
	e.logf("Starting review: mode=%s repo=%s identifier=%d submode=%s repo_root=%s", req.Mode, req.Repo, req.Identifier, req.Submode, req.RepoRoot)
	reviewCtx, err := e.source.ResolveContext(ctx, req)
	if err != nil {
		return nil, err
	}
	e.logf("Resolved review context: title=%q changed_files=%d commits=%d comments=%d diff_bytes=%d", reviewCtx.Title, len(reviewCtx.ChangedFiles), len(reviewCtx.Commits), len(reviewCtx.Comments), len(reviewCtx.Diff))

	if req.IncludeFullFiles && e.retrieval != nil && req.RepoRoot != "" {
		e.logf("Including full changed files in supplemental context")
		for _, file := range reviewCtx.ChangedFiles {
			e.logf("Retrieving full file: %s", file.Path)
			content, err := e.retrieval.GetFile(ctx, req.RepoRoot, file.Path)
			if err != nil {
				e.logf("Skipping full file retrieval for %s: %v", file.Path, err)
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
	e.logf("Trimmed review context: changed_files=%d supplemental=%d omitted_sections=%d token_budget=%d", len(trimmed.ChangedFiles), len(trimmed.SupplementalContext), len(trimmed.OmittedSections), req.MaxContextTokens)
	systemPrompt, err := e.loadPrompt(req.PromptOverride, "default_review.tmpl")
	if err != nil {
		return nil, err
	}
	e.logBlock("System prompt:", systemPrompt)
	userPrompt, err := llm.RenderPrompt(systemPrompt, trimmed)
	if err != nil {
		return nil, fmt.Errorf("review: rendering review prompt: %w", err)
	}
	e.logBlock("Rendered review prompt:", userPrompt)

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
			e.logf("Stopping follow-up loop at round %d: requests=%d retrieval_available=%t", round+1, len(resp.FollowUpRequests), e.retrieval != nil)
			break
		}
		e.logf("Executing follow-up round %d with %d retrieval requests", round+1, len(resp.FollowUpRequests))
		reviewCtx.SupplementalContext = append(reviewCtx.SupplementalContext, ExecuteRetrievals(ctx, e.retrieval, req.RepoRoot, resp.FollowUpRequests, e.logf)...)
		trimmed = trimmer.Trim(reviewCtx)
		e.logf("Trimmed context after follow-up round %d: supplemental=%d omitted_sections=%d", round+1, len(trimmed.SupplementalContext), len(trimmed.OmittedSections))

		followupTemplate, err := e.loadPrompt("", "followup_request.tmpl")
		if err != nil {
			return nil, err
		}
		e.logBlock("Follow-up prompt template:", followupTemplate)
		userPrompt, err = llm.RenderPrompt(followupTemplate, trimmed)
		if err != nil {
			return nil, fmt.Errorf("review: rendering follow-up prompt: %w", err)
		}
		e.logBlock("Rendered follow-up prompt:", userPrompt)
		llmReq.UserContent = userPrompt
		resp, err = e.llm.Review(ctx, llmReq)
		if err != nil {
			return nil, err
		}
	}

	filtered := filterBySeverity(resp.Findings, req.SeverityThreshold)
	e.logf("Review complete: findings_before_filter=%d findings_after_filter=%d threshold=%s", len(resp.Findings), len(filtered), req.SeverityThreshold)
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
	if path != "" {
		e.logf("Loading prompt override from %s", path)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("review: reading prompt %s: %w", path, err)
		}
		return string(data), nil
	}
	e.logf("Loading embedded prompt %s", fallback)
	return prompts.Load(fallback)
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
