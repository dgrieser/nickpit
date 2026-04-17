package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

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
	workDir                  string
	profile                  string
	maxContextTokens         int
	includeFullFiles         bool
	includeComments          bool
	includeCommits           bool
	jsonOutput               bool
	useJSONSchema            bool
	followUps                int
	offline                  bool
	priorityThreshold        string
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
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if cli.workDir == "" {
				return nil
			}
			if err := os.Chdir(cli.workDir); err != nil {
				return fmt.Errorf("changing working directory to %q: %w", cli.workDir, err)
			}
			return nil
		},
	}

	root.PersistentFlags().StringVar(&cli.model, "model", "", "Model identifier")
	root.PersistentFlags().StringVar(&cli.baseURL, "base-url", "", "LLM API base URL")
	root.PersistentFlags().StringVar(&cli.apiKey, "api-key", "", "LLM API key")
	root.PersistentFlags().StringVar(&cli.workDir, "workdir", "", "Set the process working directory before running the command")
	root.PersistentFlags().StringVar(&cli.profile, "profile", "default", "Config profile name")
	root.PersistentFlags().IntVar(&cli.maxContextTokens, "max-context-tokens", 120000, "Context token budget")
	root.PersistentFlags().BoolVar(&cli.includeFullFiles, "include-full-files", false, "Include full changed files")
	root.PersistentFlags().BoolVar(&cli.includeComments, "include-comments", true, "Include existing comments")
	root.PersistentFlags().BoolVar(&cli.includeCommits, "include-commits", true, "Include commit summaries")
	root.PersistentFlags().BoolVar(&cli.jsonOutput, "json", false, "Emit JSON output")
	root.PersistentFlags().BoolVar(&cli.useJSONSchema, "use-json-schema", false, "Use API-enforced JSON schema output")
	root.PersistentFlags().IntVar(&cli.followUps, "followups", 5, "Maximum follow-up rounds")
	root.PersistentFlags().BoolVar(&cli.offline, "offline", false, "Skip remote review comments")
	root.PersistentFlags().StringVar(&cli.priorityThreshold, "priority-threshold", "p3", "Minimum priority to display (p0, p1, p2, p3)")
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
	root.AddCommand(cli.newInspectCmd())
	return root
}

func (a *app) loadProfile() (string, config.Profile, error) {
	cfg, profile, err := config.Load(a.configPath, config.Overrides{
		Profile:                  a.profile,
		Model:                    a.model,
		BaseURL:                  a.baseURL,
		APIKey:                   a.apiKey,
		UseJSONSchema:            a.useJSONSchema,
		MaxContextTokens:         a.maxContextTokens,
		FollowUps:                a.followUps,
		LocalRepo:                a.localRepo,
		GitHubToken:              a.githubToken,
		GitLabToken:              a.gitlabToken,
		GitLabBaseURL:            a.gitlabBaseURL,
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
				UseJSONSchema:            profile.UseJSONSchema,
				PriorityThreshold:        a.priorityThreshold,
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
			if repo == "" {
				repo = inferRepo()
				if repo == "" {
					return fmt.Errorf("--repo is required (could not infer from git remote)")
				}
			}
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
				UseJSONSchema:            profile.UseJSONSchema,
				PriorityThreshold:        a.priorityThreshold,
				Offline:                  a.offline,
			}
			return a.runReview(cmd.Context(), source, retrieval.NewLocalEngine(), profileName, profile, req)
		},
	}
	prCmd.Flags().StringVar(&repo, "repo", "", "GitHub repo owner/name (inferred from git remote if omitted)")
	prCmd.Flags().IntVar(&pr, "id", 0, "Pull request number")
	_ = prCmd.MarkFlagRequired("id")
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
			if project == "" {
				project = inferRepo()
				if project == "" {
					return fmt.Errorf("--repo is required (could not infer from git remote)")
				}
			}
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
				UseJSONSchema:            profile.UseJSONSchema,
				PriorityThreshold:        a.priorityThreshold,
				Offline:                  a.offline,
			}
			return a.runReview(cmd.Context(), source, retrieval.NewLocalEngine(), profileName, profile, req)
		},
	}
	mrCmd.Flags().StringVar(&project, "repo", "", "GitLab project group/name (inferred from git remote if omitted)")
	mrCmd.Flags().IntVar(&mr, "id", 0, "Merge request IID")
	_ = mrCmd.MarkFlagRequired("id")
	cmd.AddCommand(mrCmd)
	return cmd
}

func (a *app) newInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Use retrieval features without running review",
		Long:  "Use retrieval features directly against the current repository without running an LLM review.",
	}
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
			return a.writeInspectOutput(content)
		},
	}
	fileCmd.Flags().StringVar(&path, "path", "", "Relative file path")
	_ = fileCmd.MarkFlagRequired("path")

	var start, end int
	linesCmd := &cobra.Command{
		Use:   "lines",
		Short: "Retrieve file lines",
		RunE: func(_ *cobra.Command, _ []string) error {
			if start <= 0 && end <= 0 {
				return fmt.Errorf("inspect lines requires --start or --end")
			}
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			content, err := engine.GetFileSlice(context.Background(), repoRoot, path, start, end)
			if err != nil {
				return err
			}
			return a.writeInspectOutput(content)
		},
	}
	linesCmd.Flags().StringVar(&path, "path", "", "Relative file path")
	linesCmd.Flags().IntVar(&start, "start", 0, "Start line (optional if --end is set)")
	linesCmd.Flags().IntVar(&end, "end", 0, "End line (optional if --start is set)")
	_ = linesCmd.MarkFlagRequired("path")

	var callersSymbol, callersPath string
	var callersDepth int
	callersCmd := &cobra.Command{
		Use:   "callers",
		Short: "Retrieve callers",
		RunE: func(_ *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			hierarchy, err := engine.FindCallers(context.Background(), repoRoot, retrieval.SymbolRef{Name: callersSymbol, Path: callersPath}, callersDepth)
			if err != nil {
				return err
			}
			return a.writeInspectOutput(hierarchy)
		},
	}
	callersCmd.Flags().StringVar(&callersSymbol, "symbol", "", "Function name")
	callersCmd.Flags().StringVar(&callersPath, "path", "", "Relative file path containing the function")
	callersCmd.Flags().IntVar(&callersDepth, "depth", 10, "Traversal depth")
	_ = callersCmd.MarkFlagRequired("symbol")
	_ = callersCmd.MarkFlagRequired("path")

	var calleesSymbol, calleesPath string
	var calleesDepth int
	calleesCmd := &cobra.Command{
		Use:   "callees",
		Short: "Retrieve callees",
		RunE: func(_ *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			hierarchy, err := engine.FindCallees(context.Background(), repoRoot, retrieval.SymbolRef{Name: calleesSymbol, Path: calleesPath}, calleesDepth)
			if err != nil {
				return err
			}
			return a.writeInspectOutput(hierarchy)
		},
	}
	calleesCmd.Flags().StringVar(&calleesSymbol, "symbol", "", "Function name")
	calleesCmd.Flags().StringVar(&calleesPath, "path", "", "Relative file path containing the function")
	calleesCmd.Flags().IntVar(&calleesDepth, "depth", 10, "Traversal depth")
	_ = calleesCmd.MarkFlagRequired("symbol")
	_ = calleesCmd.MarkFlagRequired("path")

	cmd.AddCommand(fileCmd, linesCmd, callersCmd, calleesCmd)
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
		a.logf("Resolved repo root: source=provided path=%s", req.RepoRoot)
		return req.RepoRoot, nil, nil
	}
	if req.Mode == model.ModeLocal {
		wd, err := os.Getwd()
		if err == nil {
			a.logf("Resolved repo root: source=working_dir path=%s", wd)
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
			a.logf("Skipping remote checkout: reason=no_support fallback=%s", wd)
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
	a.logf("Prepared repo root: path=%s", repoRoot)
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

func (a *app) writeInspectOutput(value any) error {
	if a.jsonOutput {
		return writeJSON(value)
	}
	switch typed := value.(type) {
	case *retrieval.FileContent:
		if _, err := fmt.Fprintf(os.Stdout, "%s (%s)\n", typed.Path, typed.Language); err != nil {
			return err
		}
		if typed.Content == "" {
			_, err := fmt.Fprintln(os.Stdout)
			return err
		}
		_, err := fmt.Fprintln(os.Stdout, typed.Content)
		return err
	case *retrieval.FileSlice:
		if _, err := fmt.Fprintf(os.Stdout, "%s:%d-%d (%s)\n", typed.Path, typed.StartLine, typed.EndLine, typed.Language); err != nil {
			return err
		}
		if typed.Content == "" {
			_, err := fmt.Fprintln(os.Stdout)
			return err
		}
		_, err := fmt.Fprintln(os.Stdout, typed.Content)
		return err
	case *retrieval.CallHierarchy:
		_, err := fmt.Fprintln(os.Stdout, typed.Render())
		return err
	default:
		return writeJSON(value)
	}
}

func inferRepo() string {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return parseRepoFromRemoteURL(strings.TrimSpace(string(out)))
}

func parseRepoFromRemoteURL(raw string) string {
	// URL schemes: https://, ssh://, git://
	// e.g. https://github.com/owner/repo.git
	//      ssh://git@gitlab.example.com:29418/group/project.git
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		path := strings.TrimPrefix(u.Path, "/")
		return strings.TrimSuffix(path, ".git")
	}
	// SCP-style: git@github.com:owner/repo.git
	//            git@gitlab.com:group/project.git
	if i := strings.Index(raw, ":"); i != -1 {
		return strings.TrimSuffix(raw[i+1:], ".git")
	}
	return ""
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
