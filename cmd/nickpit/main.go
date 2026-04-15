package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/debuglog"
	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/output"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/review"
	ghscm "github.com/dgrieser/nickpit/internal/scm/github"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/spf13/cobra"
)

type app struct {
	model                    string
	baseURL                  string
	apiKey                   string
	profile                  string
	maxContextTokens         int
	includeFullFiles         bool
	includeComments          bool
	includeCommits           bool
	jsonOutput               bool
	followUps                int
	offline                  bool
	severityThreshold        string
	reviewSystemPromptFile   string
	reviewUserPromptFile     string
	followUpSystemPromptFile string
	followUpUserPromptFile   string
	configPath               string
	localRepo                string
	githubToken              string
	gitlabToken              string
	gitlabBaseURL            string
	verbose                  bool
	logger                   *debuglog.Logger
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cli := &app{}
	root := &cobra.Command{
		Use:           "nickpit",
		Short:         "AI-powered code review for local git, GitHub PRs, and GitLab MRs",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&cli.model, "model", "", "Model identifier")
	root.PersistentFlags().StringVar(&cli.baseURL, "base-url", "", "LLM API base URL")
	root.PersistentFlags().StringVar(&cli.apiKey, "api-key", "", "LLM API key")
	root.PersistentFlags().StringVar(&cli.profile, "profile", "default", "Config profile name")
	root.PersistentFlags().IntVar(&cli.maxContextTokens, "max-context-tokens", 120000, "Context token budget")
	root.PersistentFlags().BoolVar(&cli.includeFullFiles, "include-full-files", false, "Include full changed files")
	root.PersistentFlags().BoolVar(&cli.includeComments, "include-comments", true, "Include existing comments")
	root.PersistentFlags().BoolVar(&cli.includeCommits, "include-commits", true, "Include commit summaries")
	root.PersistentFlags().BoolVar(&cli.jsonOutput, "json", false, "Emit JSON output")
	root.PersistentFlags().IntVar(&cli.followUps, "followups", 1, "Maximum follow-up rounds")
	root.PersistentFlags().BoolVar(&cli.offline, "offline", false, "Skip remote review comments")
	root.PersistentFlags().StringVar(&cli.severityThreshold, "severity-threshold", "info", "Minimum severity to display")
	root.PersistentFlags().StringVar(&cli.reviewSystemPromptFile, "review-system-prompt-file", "", "Custom review system prompt file")
	root.PersistentFlags().StringVar(&cli.reviewUserPromptFile, "review-user-prompt-file", "", "Custom review user prompt file")
	root.PersistentFlags().StringVar(&cli.followUpSystemPromptFile, "followup-system-prompt-file", "", "Custom follow-up system prompt file")
	root.PersistentFlags().StringVar(&cli.followUpUserPromptFile, "followup-user-prompt-file", "", "Custom follow-up user prompt file")
	root.PersistentFlags().StringVar(&cli.configPath, "config", ".nickpit.yaml", "Config file path")
	root.PersistentFlags().StringVar(&cli.localRepo, "local-repo", "", "Use an existing local clone for remote retrieval instead of cloning")
	root.PersistentFlags().StringVar(&cli.githubToken, "github-token", "", "GitHub token override")
	root.PersistentFlags().StringVar(&cli.gitlabToken, "gitlab-token", "", "GitLab token override")
	root.PersistentFlags().StringVar(&cli.gitlabBaseURL, "gitlab-base-url", "", "GitLab API base URL")
	root.PersistentFlags().BoolVar(&cli.verbose, "verbose", false, "Print debug execution details")
	root.PersistentFlags().BoolVar(&cli.verbose, "debug", false, "Print debug execution details")

	root.AddCommand(cli.newLocalCmd())
	root.AddCommand(cli.newGitHubCmd())
	root.AddCommand(cli.newGitLabCmd())
	root.AddCommand(cli.newRetrieveCmd())
	return root
}

func (a *app) loadProfile() (string, config.Profile, error) {
	cfg, profile, err := config.Load(a.configPath, config.Overrides{
		Profile:                  a.profile,
		Model:                    a.model,
		BaseURL:                  a.baseURL,
		APIKey:                   a.apiKey,
		MaxContextTokens:         a.maxContextTokens,
		FollowUps:                a.followUps,
		LocalRepo:                a.localRepo,
		GitHubToken:              a.githubToken,
		GitLabToken:              a.gitlabToken,
		GitLabBaseURL:            a.gitlabBaseURL,
		ReviewSystemPromptFile:   a.reviewSystemPromptFile,
		ReviewUserPromptFile:     a.reviewUserPromptFile,
		FollowUpSystemPromptFile: a.followUpSystemPromptFile,
		FollowUpUserPromptFile:   a.followUpUserPromptFile,
	})
	if err != nil {
		return "", config.Profile{}, err
	}
	return cfg.ActiveProfile, profile, nil
}

func (a *app) newLocalCmd() *cobra.Command {
	local := &cobra.Command{Use: "local", Short: "Review local git changes"}
	local.AddCommand(a.newLocalReviewCmd("uncommitted"))
	local.AddCommand(a.newLocalReviewCmd("commits"))
	local.AddCommand(a.newLocalReviewCmd("branch"))
	return local
}

func (a *app) newLocalReviewCmd(submode string) *cobra.Command {
	var from, to, base, head string
	cmd := &cobra.Command{
		Use:   submode,
		Short: "Run a local review",
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			profileName, profile, err := a.loadProfile()
			if err != nil {
				return err
			}
			req := model.ReviewRequest{
				Mode:                     model.ModeLocal,
				RepoRoot:                 repoRoot,
				LocalRepo:                profile.LocalRepo,
				BaseRef:                  firstNonEmpty(base, from),
				HeadRef:                  firstNonEmpty(head, to),
				IncludeComments:          a.includeComments,
				IncludeCommits:           a.includeCommits,
				IncludeFullFiles:         a.includeFullFiles,
				MaxContextTokens:         profile.MaxContextTokens,
				FollowUpRounds:           profile.DefaultFollowUps,
				SeverityThreshold:        a.severityThreshold,
				ReviewSystemPromptFile:   profile.ReviewSystemPromptFile,
				ReviewUserPromptFile:     profile.ReviewUserPromptFile,
				FollowUpSystemPromptFile: profile.FollowUpSystemPromptFile,
				FollowUpUserPromptFile:   profile.FollowUpUserPromptFile,
				Submode:                  submode,
			}
			return a.runReview(cmd.Context(), git.NewLocalSource(repoRoot), retrieval.NewLocalEngine(), profileName, profile, req)
		},
	}
	switch submode {
	case "commits":
		cmd.Flags().StringVar(&from, "from", "", "Base commit")
		cmd.Flags().StringVar(&to, "to", "HEAD", "Head commit")
	case "branch":
		cmd.Flags().StringVar(&base, "base", "", "Base branch")
		cmd.Flags().StringVar(&head, "head", "HEAD", "Head branch")
	}
	return cmd
}

func (a *app) newGitHubCmd() *cobra.Command {
	var repo string
	var pr int
	cmd := &cobra.Command{
		Use:   "github",
		Short: "Review GitHub pull requests",
	}
	prCmd := &cobra.Command{
		Use:   "pr",
		Short: "Review a GitHub PR",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profileName, profile, err := a.loadProfile()
			if err != nil {
				return err
			}
			source := ghscm.NewAdapter(ghscm.NewClient("", profile.GitHubToken))
			req := model.ReviewRequest{
				Mode:                     model.ModeGitHub,
				LocalRepo:                profile.LocalRepo,
				Repo:                     repo,
				Identifier:               pr,
				IncludeComments:          a.includeComments,
				IncludeCommits:           a.includeCommits,
				MaxContextTokens:         profile.MaxContextTokens,
				FollowUpRounds:           profile.DefaultFollowUps,
				SeverityThreshold:        a.severityThreshold,
				ReviewSystemPromptFile:   profile.ReviewSystemPromptFile,
				ReviewUserPromptFile:     profile.ReviewUserPromptFile,
				FollowUpSystemPromptFile: profile.FollowUpSystemPromptFile,
				FollowUpUserPromptFile:   profile.FollowUpUserPromptFile,
				Offline:                  a.offline,
			}
			return a.runReview(cmd.Context(), source, retrieval.NewLocalEngine(), profileName, profile, req)
		},
	}
	prCmd.Flags().StringVar(&repo, "repo", "", "GitHub repo owner/name")
	prCmd.Flags().IntVar(&pr, "pr", 0, "Pull request number")
	_ = prCmd.MarkFlagRequired("repo")
	_ = prCmd.MarkFlagRequired("pr")
	cmd.AddCommand(prCmd)
	return cmd
}

func (a *app) newGitLabCmd() *cobra.Command {
	var project string
	var mr int
	cmd := &cobra.Command{
		Use:   "gitlab",
		Short: "Review GitLab merge requests",
	}
	mrCmd := &cobra.Command{
		Use:   "mr",
		Short: "Review a GitLab merge request",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profileName, profile, err := a.loadProfile()
			if err != nil {
				return err
			}
			source := glscm.NewAdapter(glscm.NewClient(profile.GitLabBaseURL, profile.GitLabToken))
			req := model.ReviewRequest{
				Mode:                     model.ModeGitLab,
				LocalRepo:                profile.LocalRepo,
				Repo:                     project,
				Identifier:               mr,
				IncludeComments:          a.includeComments,
				IncludeCommits:           a.includeCommits,
				MaxContextTokens:         profile.MaxContextTokens,
				FollowUpRounds:           profile.DefaultFollowUps,
				SeverityThreshold:        a.severityThreshold,
				ReviewSystemPromptFile:   profile.ReviewSystemPromptFile,
				ReviewUserPromptFile:     profile.ReviewUserPromptFile,
				FollowUpSystemPromptFile: profile.FollowUpSystemPromptFile,
				FollowUpUserPromptFile:   profile.FollowUpUserPromptFile,
				Offline:                  a.offline,
			}
			return a.runReview(cmd.Context(), source, retrieval.NewLocalEngine(), profileName, profile, req)
		},
	}
	mrCmd.Flags().StringVar(&project, "project", "", "GitLab project group/name")
	mrCmd.Flags().IntVar(&mr, "mr", 0, "Merge request IID")
	_ = mrCmd.MarkFlagRequired("project")
	_ = mrCmd.MarkFlagRequired("mr")
	cmd.AddCommand(mrCmd)
	return cmd
}

func (a *app) newRetrieveCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "retrieve", Short: "Inspect retrieval engine output"}
	engine := retrieval.NewLocalEngine()

	var path string
	fileCmd := &cobra.Command{
		Use:   "file",
		Short: "Retrieve a file",
		RunE: func(_ *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			content, err := engine.GetFile(context.Background(), repoRoot, path)
			if err != nil {
				return err
			}
			return writeJSON(content)
		},
	}
	fileCmd.Flags().StringVar(&path, "path", "", "Relative file path")
	_ = fileCmd.MarkFlagRequired("path")

	var start, end int
	linesCmd := &cobra.Command{
		Use:   "lines",
		Short: "Retrieve file lines",
		RunE: func(_ *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			content, err := engine.GetFileSlice(context.Background(), repoRoot, path, start, end)
			if err != nil {
				return err
			}
			return writeJSON(content)
		},
	}
	linesCmd.Flags().StringVar(&path, "path", "", "Relative file path")
	linesCmd.Flags().IntVar(&start, "start", 1, "Start line")
	linesCmd.Flags().IntVar(&end, "end", 1, "End line")
	_ = linesCmd.MarkFlagRequired("path")

	var symbol, direction string
	var depth int
	stackCmd := &cobra.Command{
		Use:   "function-stack",
		Short: "Retrieve caller or callee hierarchy",
		RunE: func(_ *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			var hierarchy *retrieval.CallHierarchy
			if direction == "callers" {
				hierarchy, err = engine.FindCallers(context.Background(), repoRoot, retrieval.SymbolRef{Name: symbol}, depth)
			} else {
				hierarchy, err = engine.FindCallees(context.Background(), repoRoot, retrieval.SymbolRef{Name: symbol}, depth)
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, hierarchy.Render())
			return nil
		},
	}
	stackCmd.Flags().StringVar(&symbol, "symbol", "", "Function name")
	stackCmd.Flags().StringVar(&direction, "direction", "callers", "callers or callees")
	stackCmd.Flags().IntVar(&depth, "depth", 2, "Traversal depth")
	_ = stackCmd.MarkFlagRequired("symbol")

	callersCmd := &cobra.Command{
		Use:   "callers",
		Short: "Retrieve callers",
		RunE: func(_ *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			hierarchy, err := engine.FindCallers(context.Background(), repoRoot, retrieval.SymbolRef{Name: symbol}, depth)
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, hierarchy.Render())
			return nil
		},
	}
	callersCmd.Flags().StringVar(&symbol, "symbol", "", "Function name")
	callersCmd.Flags().IntVar(&depth, "depth", 2, "Traversal depth")
	_ = callersCmd.MarkFlagRequired("symbol")

	calleesCmd := &cobra.Command{
		Use:   "callees",
		Short: "Retrieve callees",
		RunE: func(_ *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			hierarchy, err := engine.FindCallees(context.Background(), repoRoot, retrieval.SymbolRef{Name: symbol}, depth)
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, hierarchy.Render())
			return nil
		},
	}
	calleesCmd.Flags().StringVar(&symbol, "symbol", "", "Function name")
	calleesCmd.Flags().IntVar(&depth, "depth", 2, "Traversal depth")
	_ = calleesCmd.MarkFlagRequired("symbol")

	cmd.AddCommand(fileCmd, linesCmd, callersCmd, calleesCmd, stackCmd)
	return cmd
}

func (a *app) runReview(ctx context.Context, source model.ReviewSource, retrievalEngine retrieval.Engine, profileName string, profile config.Profile, req model.ReviewRequest) error {
	logger := debuglog.New(os.Stderr, a.verbose, true)
	a.logger = logger

	if profile.APIKey == "" {
		if profile.APIKeyConfigured {
			return fmt.Errorf("profile %q has an empty api_key value; set OPENROUTER_API_KEY or provide a non-empty api_key in config", profileName)
		}
		return fmt.Errorf("missing LLM API key for profile %q; set OPENROUTER_API_KEY or provide api_key in config", profileName)
	}

	repoRoot, cleanup, err := a.resolveRepoRoot(ctx, source, profile, req)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	req.RepoRoot = repoRoot
	if req.MaxContextTokens == 0 {
		req.MaxContextTokens = profile.MaxContextTokens
	}
	if req.FollowUpRounds == 0 {
		req.FollowUpRounds = profile.DefaultFollowUps
	}

	client := llm.NewOpenAIClient(profile.BaseURL, profile.APIKey, profile.Model)
	client.SetLogger(logger)
	engine := review.NewEngine(source, client, retrievalEngine, profile)
	engine.SetLogger(logger)
	result, err := engine.Run(ctx, req)
	if err != nil {
		return err
	}
	var formatter output.Formatter
	if a.jsonOutput {
		formatter = output.NewJSONFormatter(os.Stdout)
	} else {
		formatter = output.NewTerminalFormatter(os.Stdout, true)
	}
	return formatter.FormatFindings(result)
}

func (a *app) resolveRepoRoot(ctx context.Context, source model.ReviewSource, profile config.Profile, req model.ReviewRequest) (string, func(), error) {
	if req.RepoRoot != "" {
		a.logf("Using provided repo root: %s", req.RepoRoot)
		return req.RepoRoot, nil, nil
	}
	if req.Mode == model.ModeLocal {
		wd, err := os.Getwd()
		if err == nil {
			a.logf("Resolved local repo root from working directory: %s", wd)
		}
		return wd, nil, err
	}
	followUpRounds := req.FollowUpRounds
	if followUpRounds == 0 {
		followUpRounds = profile.DefaultFollowUps
	}
	if !req.IncludeFullFiles && followUpRounds <= 0 {
		a.logf("Skipping remote checkout: include_full_files=%t follow_up_rounds=%d", req.IncludeFullFiles, followUpRounds)
		return "", nil, nil
	}
	remote, ok := source.(model.RemoteCheckoutSource)
	if !ok {
		wd, err := os.Getwd()
		if err == nil {
			a.logf("Source has no remote checkout support; falling back to working directory: %s", wd)
		}
		return wd, nil, err
	}
	spec, err := remote.ResolveCheckout(ctx, req)
	if err != nil {
		return "", nil, err
	}
	a.logf("Preparing remote checkout: provider=%s repo=%s head_ref=%s head_sha=%s", spec.Provider, spec.Repo, spec.HeadRef, spec.HeadSHA)
	manager := git.NewCheckoutManager()
	repoRoot, cleanup, err := manager.Prepare(ctx, *spec, git.CheckoutOptions{
		LocalRepo: req.LocalRepo,
		Token:     checkoutToken(req.Mode, profile),
	})
	if err != nil {
		return "", nil, err
	}
	a.logf("Prepared repo root: %s", repoRoot)
	return repoRoot, cleanup, nil
}

func checkoutToken(mode model.ReviewMode, profile config.Profile) string {
	switch mode {
	case model.ModeGitHub:
		return profile.GitHubToken
	case model.ModeGitLab:
		return profile.GitLabToken
	default:
		return ""
	}
}

func writeJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (a *app) logf(format string, args ...any) {
	if a.logger == nil {
		return
	}
	a.logger.Printf(format, args...)
}
