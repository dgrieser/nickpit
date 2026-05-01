package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/output"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/internal/review"
	ghscm "github.com/dgrieser/nickpit/internal/scm/github"
	glscm "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/spf13/cobra"
)

type app struct {
	model                         string
	baseURL                       string
	apiKey                        string
	workDir                       string
	profile                       string
	temperature                   float64
	temperatureSet                bool
	topP                          float64
	topPSet                       bool
	extraBody                     string
	maxContextTokens              int
	maxContextTokensSet           bool
	includeFullFiles              bool
	includeComments               bool
	includeCommits                bool
	jsonOutput                    bool
	useJSONSchema                 bool
	maxToolCalls                  int
	maxToolCallsSet               bool
	maxDuplicateToolCalls         int
	maxDuplicateToolCallsSet      bool
	offline                       bool
	priorityThreshold             string
	configPath                    string
	githubToken                   string
	gitlabToken                   string
	gitlabBaseURL                 string
	verbose                       bool
	reasoningEffort               string
	showReasoning                 bool
	showProgress                  bool
	disableSearchToolOptimization bool
	disableParallelToolCalls      bool
	logger                        *logging.Logger
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		logging.New(os.Stderr, false, true).PrintError(err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cli := &app{
		maxContextTokens:      config.DefaultMaxContextToken,
		maxDuplicateToolCalls: config.DefaultMaxDuplicateToolCalls,
	}
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
	root.PersistentFlags().StringVar(&cli.workDir, "workdir", "", "Working directory")
	root.PersistentFlags().StringVar(&cli.profile, "profile", "default", "Config profile name")
	root.PersistentFlags().Var(newTrackedFloatValue(&cli.temperature, &cli.temperatureSet), "temperature", "Sampling temperature")
	root.PersistentFlags().Var(newTrackedFloatValue(&cli.topP, &cli.topPSet), "top-p", "Nucleus sampling probability")
	root.PersistentFlags().StringVar(&cli.extraBody, "extra-body", "", "Additional JSON object fields to merge into the LLM request body")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxContextTokens, &cli.maxContextTokensSet), "max-context-tokens", "Context token budget")
	root.PersistentFlags().BoolVar(&cli.includeFullFiles, "include-full-files", false, "Include full changed files")
	root.PersistentFlags().BoolVar(&cli.includeComments, "include-comments", true, "Include existing comments")
	root.PersistentFlags().BoolVar(&cli.includeCommits, "include-commits", true, "Include commit summaries")
	root.PersistentFlags().BoolVar(&cli.jsonOutput, "json", false, "Emit JSON output")
	root.PersistentFlags().BoolVar(&cli.useJSONSchema, "use-json-schema", false, "Use API-enforced JSON schema output")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxToolCalls, &cli.maxToolCallsSet), "max-tool-calls", "Maximum tool-call rounds (0 means unlimited by default)")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxDuplicateToolCalls, &cli.maxDuplicateToolCallsSet), "max-duplicate-tool-calls", "Maximum duplicate tool calls before tools are disabled")
	root.PersistentFlags().BoolVar(&cli.offline, "offline", false, "Skip remote review comments")
	root.PersistentFlags().StringVar(&cli.priorityThreshold, "priority-threshold", "p3", "Minimum priority to display (p0, p1, p2, p3)")
	root.PersistentFlags().StringVar(&cli.configPath, "config", ".nickpit.yaml", "Config file path")
	root.PersistentFlags().StringVar(&cli.githubToken, "github-token", "", "GitHub token override")
	root.PersistentFlags().StringVar(&cli.gitlabToken, "gitlab-token", "", "GitLab token override")
	root.PersistentFlags().StringVar(&cli.gitlabBaseURL, "gitlab-base-url", "", "GitLab API base URL")
	root.PersistentFlags().BoolVar(&cli.verbose, "verbose", false, "Print debug execution details")
	root.PersistentFlags().BoolVar(&cli.verbose, "debug", false, "Print debug execution details")
	root.PersistentFlags().StringVar(&cli.reasoningEffort, "reasoning-effort", "", "Reasoning effort level; known fallback ladder: max, xhigh, high, medium, low, minimal, none, off")
	root.PersistentFlags().BoolVar(&cli.showReasoning, "show-reasoning", false, "Print streamed model reasoning to stderr")
	root.PersistentFlags().BoolVar(&cli.showProgress, "show-progress", false, "Print review progress to stderr")
	root.PersistentFlags().BoolVar(&cli.disableSearchToolOptimization, "disable-search-tool-optimization", false, "Disable rewriting search tool calls like FunctionName( into find_callers")
	root.PersistentFlags().BoolVar(&cli.disableParallelToolCalls, "disable-parallel-tool-calls", false, "Disable parallel tool calls and the prompt guidance that encourages batching")

	root.AddCommand(cli.newLocalCmd())
	root.AddCommand(cli.newGitHubCmd())
	root.AddCommand(cli.newGitLabCmd())
	root.AddCommand(cli.newInspectCmd())
	return root
}

func (a *app) loadProfile() (string, config.Profile, error) {
	var maxContextTokens *int
	if a.maxContextTokensSet {
		maxContextTokens = &a.maxContextTokens
	}
	var toolCalls *int
	if a.maxToolCallsSet {
		toolCalls = &a.maxToolCalls
	}
	var duplicateToolCalls *int
	if a.maxDuplicateToolCallsSet {
		duplicateToolCalls = &a.maxDuplicateToolCalls
	}
	var temperature *float64
	if a.temperatureSet {
		temperature = &a.temperature
	}
	var topP *float64
	if a.topPSet {
		topP = &a.topP
	}
	var extraBody map[string]any
	if strings.TrimSpace(a.extraBody) != "" {
		if err := json.Unmarshal([]byte(a.extraBody), &extraBody); err != nil {
			return "", config.Profile{}, fmt.Errorf("parsing --extra-body JSON object: %w", err)
		}
		if extraBody == nil {
			return "", config.Profile{}, fmt.Errorf("--extra-body must be a JSON object")
		}
	}
	cfg, profile, err := config.Load(a.configPath, config.Overrides{
		Profile:            a.profile,
		Model:              a.model,
		BaseURL:            a.baseURL,
		APIKey:             a.apiKey,
		ReasoningEffort:    a.reasoningEffort,
		Temperature:        temperature,
		TopP:               topP,
		ExtraBody:          extraBody,
		UseJSONSchema:      a.useJSONSchema,
		MaxContextTokens:   maxContextTokens,
		ToolCalls:          toolCalls,
		DuplicateToolCalls: duplicateToolCalls,
		Workdir:            a.workDir,
		GitHubToken:        a.githubToken,
		GitLabToken:        a.gitlabToken,
		GitLabBaseURL:      a.gitlabBaseURL,
	})
	if err != nil {
		return "", config.Profile{}, err
	}
	return cfg.ActiveProfile, profile, nil
}

type trackedIntValue struct {
	target *int
	set    *bool
}

func newTrackedIntValue(target *int, set *bool) *trackedIntValue {
	return &trackedIntValue{target: target, set: set}
}

func (v *trackedIntValue) String() string {
	if v == nil || v.target == nil {
		return "0"
	}
	return fmt.Sprintf("%d", *v.target)
}

func (v *trackedIntValue) Set(value string) error {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	*v.target = parsed
	if v.set != nil {
		*v.set = true
	}
	return nil
}

func (v *trackedIntValue) Type() string {
	return "int"
}

type trackedFloatValue struct {
	target *float64
	set    *bool
}

func newTrackedFloatValue(target *float64, set *bool) *trackedFloatValue {
	return &trackedFloatValue{target: target, set: set}
}

func (v *trackedFloatValue) String() string {
	if v == nil || v.target == nil {
		return "0"
	}
	return fmt.Sprintf("%g", *v.target)
}

func (v *trackedFloatValue) Set(value string) error {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}
	*v.target = parsed
	if v.set != nil {
		*v.set = true
	}
	return nil
}

func (v *trackedFloatValue) Type() string {
	return "float"
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
				Mode:                  model.ModeLocal,
				RepoRoot:              repoRoot,
				Workdir:               profile.Workdir,
				BaseRef:               firstNonEmpty(base, from),
				HeadRef:               firstNonEmpty(head, to),
				IncludeComments:       a.includeComments,
				IncludeCommits:        a.includeCommits,
				IncludeFullFiles:      a.includeFullFiles,
				MaxContextTokens:      profile.MaxContextTokens,
				MaxToolCalls:          profile.MaxToolCalls,
				MaxDuplicateToolCalls: profile.MaxDuplicateToolCalls,
				UseJSONSchema:         profile.UseJSONSchema,
				PriorityThreshold:     a.priorityThreshold,
				Submode:               submode,
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
				Mode:                  model.ModeGitHub,
				Workdir:               profile.Workdir,
				Repo:                  repo,
				Identifier:            pr,
				IncludeComments:       a.includeComments,
				IncludeCommits:        a.includeCommits,
				MaxContextTokens:      profile.MaxContextTokens,
				MaxToolCalls:          profile.MaxToolCalls,
				MaxDuplicateToolCalls: profile.MaxDuplicateToolCalls,
				UseJSONSchema:         profile.UseJSONSchema,
				PriorityThreshold:     a.priorityThreshold,
				Offline:               a.offline,
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
				Mode:                  model.ModeGitLab,
				Workdir:               profile.Workdir,
				Repo:                  project,
				Identifier:            mr,
				IncludeComments:       a.includeComments,
				IncludeCommits:        a.includeCommits,
				MaxContextTokens:      profile.MaxContextTokens,
				MaxToolCalls:          profile.MaxToolCalls,
				MaxDuplicateToolCalls: profile.MaxDuplicateToolCalls,
				UseJSONSchema:         profile.UseJSONSchema,
				PriorityThreshold:     a.priorityThreshold,
				Offline:               a.offline,
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
	var depth int
	var lineStart, lineEnd int
	var query string
	var contextLines, maxResults int
	var caseSensitive bool
	fileCmd := &cobra.Command{
		Use:   "file",
		Short: "Retrieve a file",
		RunE: func(_ *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			if lineStart > 0 || lineEnd > 0 {
				content, err := engine.GetFileSlice(context.Background(), repoRoot, path, lineStart, lineEnd)
				if err != nil {
					return err
				}
				return a.writeInspectOutput(content)
			}
			content, err := engine.GetFile(context.Background(), repoRoot, path)
			if err != nil {
				return err
			}
			return a.writeInspectOutput(content)
		},
	}
	fileCmd.Flags().StringVar(&path, "path", "", "Relative file path")
	fileCmd.Flags().IntVar(&lineStart, "line-start", 0, "Optional starting line number for partial file retrieval")
	fileCmd.Flags().IntVar(&lineEnd, "line-end", 0, "Optional ending line number for partial file retrieval")
	_ = fileCmd.MarkFlagRequired("path")

	listFilesCmd := &cobra.Command{
		Use:   "list",
		Short: "List files in a folder",
		RunE: func(_ *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			listing, err := engine.ListFiles(context.Background(), repoRoot, path, depth)
			if err != nil {
				return err
			}
			return a.writeInspectOutput(listing)
		},
	}
	listFilesCmd.Flags().StringVar(&path, "path", "", "Relative folder path; leave empty to list the repo root")
	listFilesCmd.Flags().IntVar(&depth, "depth", 1, "Directory depth to traverse when listing files")

	searchCmd := &cobra.Command{
		Use:   "search",
		Short: "Search recursively in a file or folder",
		RunE: func(_ *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			results, err := engine.Search(context.Background(), repoRoot, path, query, contextLines, maxResults, caseSensitive)
			if err != nil {
				return err
			}
			return a.writeInspectOutput(results)
		},
	}
	searchCmd.Flags().StringVar(&path, "path", "", "Relative file or folder path; leave empty to search from the repo root")
	searchCmd.Flags().StringVar(&query, "query", "", "Search string")
	searchCmd.Flags().IntVar(&contextLines, "context-lines", 5, "Number of surrounding lines to include before and after each match")
	searchCmd.Flags().IntVar(&maxResults, "max-results", 0, "Maximum number of matches to return; 0 means unlimited")
	searchCmd.Flags().BoolVar(&caseSensitive, "case-sensitive", false, "Use case-sensitive matching")
	_ = searchCmd.MarkFlagRequired("query")

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
	callersCmd.Flags().StringVar(&callersPath, "path", "", "Relative file or folder path containing the function; leave empty to search from the repo root")
	callersCmd.Flags().IntVar(&callersDepth, "depth", 10, "Traversal depth")
	_ = callersCmd.MarkFlagRequired("symbol")

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
	calleesCmd.Flags().StringVar(&calleesPath, "path", "", "Relative file or folder path containing the function; leave empty to search from the repo root")
	calleesCmd.Flags().IntVar(&calleesDepth, "depth", 10, "Traversal depth")
	_ = calleesCmd.MarkFlagRequired("symbol")

	cmd.AddCommand(fileCmd, listFilesCmd, searchCmd, callersCmd, calleesCmd)
	return cmd
}

func (a *app) runReview(ctx context.Context, source model.ReviewSource, retrievalEngine retrieval.Engine, profileName string, profile config.Profile, req model.ReviewRequest) error {
	logger := logging.New(os.Stderr, a.verbose, true)
	logger.SetShowReasoning(a.showReasoning)
	logger.SetShowProgress(a.showProgress)
	a.logger = logger

	if profile.APIKey == "" {
		if profile.APIKeyConfigured {
			return fmt.Errorf("profile %q has an empty api_key value; %s", profileName, missingAPIKeyHint(profileName, true))
		}
		return fmt.Errorf("missing LLM API key for profile %q; %s", profileName, missingAPIKeyHint(profileName, false))
	}

	repoRoot, cleanup, err := a.resolveRepoRoot(ctx, source, profile, req)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	req.RepoRoot = repoRoot
	req.DisableParallelToolCalls = a.disableParallelToolCalls
	req.ProfileName = profileName
	if req.MaxContextTokens == 0 {
		req.MaxContextTokens = profile.MaxContextTokens
	}
	if req.MaxToolCalls == 0 {
		req.MaxToolCalls = profile.MaxToolCalls
	}
	if req.MaxDuplicateToolCalls == 0 {
		req.MaxDuplicateToolCalls = profile.MaxDuplicateToolCalls
	}
	a.logProgress("Model", modelSummary(profile, req))

	client := llm.NewOpenAIClient(profile.BaseURL, profile.APIKey, profile.Model)
	client.SetLogger(logger)
	engine := review.NewEngine(source, client, retrievalEngine, profile)
	engine.SetLogger(logger)
	engine.SetSearchToolOptimization(!a.disableSearchToolOptimization)
	result, err := engine.Run(ctx, req)
	if errors.Is(err, llm.ErrInvalidJSON) {
		a.logProgress("Result", fmt.Sprintf("status=InvalidJson, error=%v", err))
		return err
	}
	if err != nil {
		a.logProgress("Result", fmt.Sprintf("status=ERROR, error=%v", err))
		return err
	}
	a.logProgress("Result", reviewResultSummary(result))
	var formatter output.Formatter
	if a.jsonOutput {
		formatter = output.NewJSONFormatter(os.Stdout)
	} else {
		formatter = output.NewTerminalFormatter(os.Stdout, true)
	}
	return formatter.FormatFindings(result)
}

func missingAPIKeyHint(profileName string, configured bool) string {
	envVar := defaultAPIKeyEnvVar(profileName)
	if configured {
		return fmt.Sprintf("set %s or provide a non-empty api_key in config", envVar)
	}
	return fmt.Sprintf("set %s or provide api_key in config", envVar)
}

func defaultAPIKeyEnvVar(profileName string) string {
	defaults := config.DefaultConfig()
	profile, ok := defaults.Profiles[profileName]
	if !ok && profileName == config.DefaultFallbackProfileName {
		profile = defaults.Profiles[config.DefaultProfileName]
		ok = true
	}
	if ok {
		if envVar := envVarName(profile.APIKey); envVar != "" {
			return envVar
		}
	}
	return "NICKPIT_API_KEY"
}

func envVarName(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
	}
	if !strings.HasPrefix(value, "$") {
		return ""
	}
	return strings.TrimPrefix(value, "$")
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
	maxToolCalls := req.MaxToolCalls
	if maxToolCalls == 0 {
		maxToolCalls = profile.MaxToolCalls
	}
	if !req.IncludeFullFiles && maxToolCalls < 0 {
		a.logf("Skipping remote checkout: include_full_files=%t max_tool_calls=%d", req.IncludeFullFiles, maxToolCalls)
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
		Workdir: req.Workdir,
		Token:   checkoutToken(req.Mode, profile),
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
	case *retrieval.DirectoryListing:
		if _, err := fmt.Fprintf(os.Stdout, "%s\n", typed.Path); err != nil {
			return err
		}
		for _, file := range typed.Files {
			if _, err := fmt.Fprintln(os.Stdout, file); err != nil {
				return err
			}
		}
		return nil
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
	case *retrieval.SearchResults:
		for i, result := range typed.Results {
			if i > 0 {
				if _, err := fmt.Fprintln(os.Stdout); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(os.Stdout, "%s:%d-%d (%s)\n", result.Path, result.StartLine, result.EndLine, result.Language); err != nil {
				return err
			}
			if result.Content == "" {
				if _, err := fmt.Fprintln(os.Stdout); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintln(os.Stdout, result.Content); err != nil {
				return err
			}
		}
		return nil
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

func (a *app) logProgress(label, summary string) {
	if a.logger == nil {
		return
	}
	a.logger.PrintProgress(label, summary)
}

func modelSummary(profile config.Profile, req model.ReviewRequest) string {
	flags := []string{fmt.Sprintf("%dk context", req.MaxContextTokens/1000)}
	if req.MaxToolCalls > 0 {
		flags = append(flags, fmt.Sprintf("≤%d tool calls", req.MaxToolCalls))
	}
	if req.MaxDuplicateToolCalls > 0 {
		flags = append(flags, fmt.Sprintf("≤%d duplicate tool calls", req.MaxDuplicateToolCalls))
	}
	if !req.DisableParallelToolCalls {
		flags = append(flags, "parallel")
	}
	if req.UseJSONSchema {
		flags = append(flags, "structured")
	}
	return fmt.Sprintf("%s:%s [%s] @ %s",
		profile.Model, profile.ReasoningEffort,
		strings.Join(flags, ", "),
		profile.BaseURL,
	)
}

func reviewResultSummary(result *model.ReviewResult) string {
	return strings.Join([]string{
		"status=OK",
		fmt.Sprintf("findings=%d", len(result.Findings)),
		fmt.Sprintf("tool_calls=%d", result.ToolCalls),
		fmt.Sprintf("duplicate_tool_calls=%d", result.DuplicateToolCalls),
		fmt.Sprintf("prompt_tokens=%d", result.TokensUsed.PromptTokens),
		fmt.Sprintf("completion_tokens=%d", result.TokensUsed.CompletionTokens),
		fmt.Sprintf("total_tokens=%d", result.TokensUsed.TotalTokens),
	}, ", ")
}
