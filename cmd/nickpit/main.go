package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/dgrieser/nickpit/internal/config"
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
	model             string
	baseURL           string
	apiKey            string
	profile           string
	maxContextTokens  int
	includeFullFiles  bool
	includeComments   bool
	includeCommits    bool
	jsonOutput        bool
	followUps         int
	offline           bool
	severityThreshold string
	promptFile        string
	configPath        string
	githubToken       string
	gitlabToken       string
	gitlabBaseURL     string
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
		Use:   "nickpit",
		Short: "AI-powered code review for local git, GitHub PRs, and GitLab MRs",
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
	root.PersistentFlags().StringVar(&cli.promptFile, "prompt-file", "", "Custom prompt file")
	root.PersistentFlags().StringVar(&cli.configPath, "config", ".nickpit.yaml", "Config file path")
	root.PersistentFlags().StringVar(&cli.githubToken, "github-token", "", "GitHub token override")
	root.PersistentFlags().StringVar(&cli.gitlabToken, "gitlab-token", "", "GitLab token override")
	root.PersistentFlags().StringVar(&cli.gitlabBaseURL, "gitlab-base-url", "", "GitLab API base URL")

	root.AddCommand(cli.newLocalCmd())
	root.AddCommand(cli.newGitHubCmd())
	root.AddCommand(cli.newGitLabCmd())
	root.AddCommand(cli.newRetrieveCmd())
	return root
}

func (a *app) loadProfile() (config.Profile, error) {
	_, profile, err := config.Load(a.configPath, config.Overrides{
		Profile:          a.profile,
		Model:            a.model,
		BaseURL:          a.baseURL,
		APIKey:           a.apiKey,
		MaxContextTokens: a.maxContextTokens,
		FollowUps:        a.followUps,
		GitHubToken:      a.githubToken,
		GitLabToken:      a.gitlabToken,
		GitLabBaseURL:    a.gitlabBaseURL,
		PromptFile:       a.promptFile,
	})
	return profile, err
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
			profile, err := a.loadProfile()
			if err != nil {
				return err
			}
			req := model.ReviewRequest{
				Mode:              model.ModeLocal,
				RepoRoot:          repoRoot,
				BaseRef:           firstNonEmpty(base, from),
				HeadRef:           firstNonEmpty(head, to),
				IncludeComments:   a.includeComments,
				IncludeCommits:    a.includeCommits,
				IncludeFullFiles:  a.includeFullFiles,
				MaxContextTokens:  profile.MaxContextTokens,
				FollowUpRounds:    profile.DefaultFollowUps,
				SeverityThreshold: a.severityThreshold,
				PromptOverride:    a.promptFile,
				Submode:           submode,
			}
			return a.runReview(cmd.Context(), git.NewLocalSource(repoRoot), retrieval.NewLocalEngine(), profile, req)
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
			profile, err := a.loadProfile()
			if err != nil {
				return err
			}
			source := ghscm.NewAdapter(ghscm.NewClient("", profile.GitHubToken))
			req := model.ReviewRequest{
				Mode:              model.ModeGitHub,
				Repo:              repo,
				Identifier:        pr,
				IncludeComments:   a.includeComments,
				IncludeCommits:    a.includeCommits,
				MaxContextTokens:  profile.MaxContextTokens,
				FollowUpRounds:    profile.DefaultFollowUps,
				SeverityThreshold: a.severityThreshold,
				PromptOverride:    a.promptFile,
				Offline:           a.offline,
			}
			return a.runReview(cmd.Context(), source, retrieval.NewLocalEngine(), profile, req)
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
			profile, err := a.loadProfile()
			if err != nil {
				return err
			}
			source := glscm.NewAdapter(glscm.NewClient(profile.GitLabBaseURL, profile.GitLabToken))
			req := model.ReviewRequest{
				Mode:              model.ModeGitLab,
				Repo:              project,
				Identifier:        mr,
				IncludeComments:   a.includeComments,
				IncludeCommits:    a.includeCommits,
				MaxContextTokens:  profile.MaxContextTokens,
				FollowUpRounds:    profile.DefaultFollowUps,
				SeverityThreshold: a.severityThreshold,
				PromptOverride:    a.promptFile,
				Offline:           a.offline,
			}
			return a.runReview(cmd.Context(), source, retrieval.NewLocalEngine(), profile, req)
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

func (a *app) runReview(ctx context.Context, source model.ReviewSource, retrievalEngine retrieval.Engine, profile config.Profile, req model.ReviewRequest) error {
	repoRoot := req.RepoRoot
	if repoRoot == "" {
		if wd, err := os.Getwd(); err == nil {
			repoRoot = wd
		}
	}
	req.RepoRoot = repoRoot
	if req.MaxContextTokens == 0 {
		req.MaxContextTokens = profile.MaxContextTokens
	}
	if req.FollowUpRounds == 0 {
		req.FollowUpRounds = profile.DefaultFollowUps
	}

	engine := review.NewEngine(source, llm.NewOpenAIClient(profile.BaseURL, profile.APIKey, profile.Model), retrievalEngine, profile)
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
