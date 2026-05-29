package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/modelcheck"
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
	maxOutputRetries              int
	maxOutputRetriesSet           bool
	maxReasoningSeconds           int
	maxReasoningSecondsSet        bool
	maxReasoningLoopRepeats       int
	maxReasoningLoopRepeatsSet    bool
	maxRateLimitDelaySeconds      int
	maxRateLimitDelaySecondsSet   bool
	nudgeCount                    int
	nudgeCountSet                 bool
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
	disableReasoningExtract       bool
	verifyConcurrency             int
	verifyDropPolicy              string
	verifyDropConfidence          float64
	skipModelCheck                bool
	refreshModelCheck             bool
	logger                        *logging.Logger
}

func main() {
	// Cancel the command context on Ctrl-C / SIGTERM so in-flight,
	// context-aware work (git via exec.CommandContext, the LLM stream) is
	// interrupted and deferred cleanups — temp clones and `git worktree add`
	// entries in the user's repo — actually run instead of being orphaned.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		logging.New(os.Stderr, false, isTerminal(os.Stderr)).PrintError(err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cli := &app{
		maxContextTokens:         config.DefaultMaxContextToken,
		maxDuplicateToolCalls:    config.DefaultMaxDuplicateToolCalls,
		maxOutputRetries:         config.DefaultMaxOutputRetries,
		maxReasoningSeconds:      config.DefaultMaxReasoningSeconds,
		maxReasoningLoopRepeats:  config.DefaultMaxReasoningLoopRepeats,
		maxRateLimitDelaySeconds: config.DefaultMaxRateLimitDelaySeconds,
		nudgeCount:               config.DefaultNudgeCount,
	}
	root := &cobra.Command{
		Use:           "nickpit",
		Short:         "AI-powered code review for local git, GitHub PRs, and GitLab MRs",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if err := review.ValidateDropPolicy(cli.verifyDropPolicy); err != nil {
				return err
			}
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
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxOutputRetries, &cli.maxOutputRetriesSet), "max-output-retries", "Maximum invalid output retries")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxReasoningSeconds, &cli.maxReasoningSecondsSet), "max-reasoning-seconds", "Maximum seconds to allow reasoning before falling back to lower effort")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxReasoningLoopRepeats, &cli.maxReasoningLoopRepeatsSet), "max-reasoning-loop-repeats", "Allowed repeated reasoning loops before falling back (0 disables loop detection)")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxRateLimitDelaySeconds, &cli.maxRateLimitDelaySecondsSet), "max-rate-limit-delay-seconds", "Maximum seconds to wait for rate-limit reset times parsed from 429 responses (0 disables)")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.nudgeCount, &cli.nudgeCountSet), "nudge-count", "Number of nudge rounds asking each reviewer to look again (0 disables)")
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
	root.PersistentFlags().BoolVar(&cli.disableReasoningExtract, "disable-reasoning-extract", false, "Disable the reasoning-extractor agent that augments nudge prompts with issues the reviewer only reasoned about")
	root.PersistentFlags().IntVar(&cli.verifyConcurrency, "verify-concurrency", 4, "Maximum parallel verifier calls")
	root.PersistentFlags().StringVar(&cli.verifyDropPolicy, "verify-drop-policy", "refuted-only", "Which verifier verdicts cause a finding to be dropped before merge: none, refuted-only, refuted-and-unverified")
	root.PersistentFlags().Float64Var(&cli.verifyDropConfidence, "verify-drop-confidence", 0.8, "Minimum verifier confidence_score required to drop a finding; verdicts below this floor are kept")
	root.PersistentFlags().BoolVar(&cli.skipModelCheck, "skip-model-check", false, "Skip pre-review model capability checks")

	root.AddCommand(cli.newCheckCmd())
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
	var outputRetries *int
	if a.maxOutputRetriesSet {
		outputRetries = &a.maxOutputRetries
	}
	var reasoningSeconds *int
	if a.maxReasoningSecondsSet {
		reasoningSeconds = &a.maxReasoningSeconds
	}
	var reasoningLoopRepeats *int
	if a.maxReasoningLoopRepeatsSet {
		reasoningLoopRepeats = &a.maxReasoningLoopRepeats
	}
	var rateLimitDelaySeconds *int
	if a.maxRateLimitDelaySecondsSet {
		rateLimitDelaySeconds = &a.maxRateLimitDelaySeconds
	}
	var nudgeCount *int
	if a.nudgeCountSet {
		nudgeCount = &a.nudgeCount
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
		Profile:               a.profile,
		Model:                 a.model,
		BaseURL:               a.baseURL,
		APIKey:                a.apiKey,
		ReasoningEffort:       a.reasoningEffort,
		Temperature:           temperature,
		TopP:                  topP,
		ExtraBody:             extraBody,
		UseJSONSchema:         a.useJSONSchema,
		MaxContextTokens:      maxContextTokens,
		ToolCalls:             toolCalls,
		DuplicateToolCalls:    duplicateToolCalls,
		OutputRetries:         outputRetries,
		ReasoningSeconds:      reasoningSeconds,
		ReasoningLoopRepeats:  reasoningLoopRepeats,
		RateLimitDelaySeconds: rateLimitDelaySeconds,
		NudgeCount:            nudgeCount,
		Workdir:               a.workDir,
		GitHubToken:           a.githubToken,
		GitLabToken:           a.gitlabToken,
		GitLabBaseURL:         a.gitlabBaseURL,
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
				Mode:                    model.ModeLocal,
				RepoRoot:                repoRoot,
				Workdir:                 profile.Workdir,
				BaseRef:                 firstNonEmpty(base, from),
				HeadRef:                 firstNonEmpty(head, to),
				IncludeComments:         a.includeComments,
				IncludeCommits:          a.includeCommits,
				IncludeFullFiles:        a.includeFullFiles,
				MaxContextTokens:        profile.MaxContextTokens,
				MaxToolCalls:            profile.MaxToolCalls,
				MaxDuplicateToolCalls:   profile.MaxDuplicateToolCalls,
				MaxOutputRetries:        profile.MaxOutputRetries,
				MaxReasoningSeconds:     profile.MaxReasoningSeconds,
				MaxReasoningLoopRepeats: profile.MaxReasoningLoopRepeats,
				NudgeCount:              profile.NudgeCount,
				UseJSONSchema:           profile.UseJSONSchema,
				PriorityThreshold:       a.priorityThreshold,
				Submode:                 submode,
			}
			return a.runReview(cmd.Context(), git.NewLocalSource(repoRoot), retrieval.NewLocalEngine(), profileName, profile, req)
		},
	}
	switch submode {
	case "commits":
		cmd.Flags().StringVar(&from, "from", "", "Base commit")
		cmd.Flags().StringVar(&to, "to", "HEAD", "Head commit")
	case "branch":
		cmd.Flags().StringVar(&base, "base", "", "Base branch, e.g. the target branch; usually main or master")
		cmd.Flags().StringVar(&head, "head", "HEAD", "Head branch, e.g. the source branch; usually the branch to review")
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
				Mode:                    model.ModeGitHub,
				Workdir:                 profile.Workdir,
				Repo:                    repo,
				Identifier:              pr,
				IncludeComments:         a.includeComments,
				IncludeCommits:          a.includeCommits,
				MaxContextTokens:        profile.MaxContextTokens,
				MaxToolCalls:            profile.MaxToolCalls,
				MaxDuplicateToolCalls:   profile.MaxDuplicateToolCalls,
				MaxOutputRetries:        profile.MaxOutputRetries,
				MaxReasoningSeconds:     profile.MaxReasoningSeconds,
				MaxReasoningLoopRepeats: profile.MaxReasoningLoopRepeats,
				NudgeCount:              profile.NudgeCount,
				UseJSONSchema:           profile.UseJSONSchema,
				PriorityThreshold:       a.priorityThreshold,
				Offline:                 a.offline,
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
	var publish bool
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
				Mode:                    model.ModeGitLab,
				Workdir:                 profile.Workdir,
				Repo:                    project,
				Identifier:              mr,
				IncludeComments:         a.includeComments,
				IncludeCommits:          a.includeCommits,
				MaxContextTokens:        profile.MaxContextTokens,
				MaxToolCalls:            profile.MaxToolCalls,
				MaxDuplicateToolCalls:   profile.MaxDuplicateToolCalls,
				MaxOutputRetries:        profile.MaxOutputRetries,
				MaxReasoningSeconds:     profile.MaxReasoningSeconds,
				MaxReasoningLoopRepeats: profile.MaxReasoningLoopRepeats,
				NudgeCount:              profile.NudgeCount,
				UseJSONSchema:           profile.UseJSONSchema,
				PriorityThreshold:       a.priorityThreshold,
				Offline:                 a.offline,
				PostReview:              publish,
			}
			return a.runReview(cmd.Context(), source, retrieval.NewLocalEngine(), profileName, profile, req)
		},
	}
	mrCmd.Flags().StringVar(&project, "repo", "", "GitLab project group/name (inferred from git remote if omitted)")
	mrCmd.Flags().IntVar(&mr, "id", 0, "Merge request IID")
	mrCmd.Flags().BoolVar(&publish, "publish", false, "Post the review back to the GitLab MR as comments (summary + one per finding)")
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			if lineStart > 0 || lineEnd > 0 {
				content, err := engine.GetFileSlice(cmd.Context(), repoRoot, path, lineStart, lineEnd)
				if err != nil {
					return err
				}
				return a.writeInspectOutput(content)
			}
			content, err := engine.GetFile(cmd.Context(), repoRoot, path)
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			listing, err := engine.ListFiles(cmd.Context(), repoRoot, path, depth)
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			results, err := engine.Search(cmd.Context(), repoRoot, path, query, contextLines, maxResults, caseSensitive)
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			hierarchy, err := engine.FindCallers(cmd.Context(), repoRoot, retrieval.SymbolRef{Name: callersSymbol, Path: callersPath}, callersDepth)
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			hierarchy, err := engine.FindCallees(cmd.Context(), repoRoot, retrieval.SymbolRef{Name: calleesSymbol, Path: calleesPath}, calleesDepth)
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

func (a *app) newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run environment and model checks",
	}
	modelCmd := &cobra.Command{
		Use:   "model",
		Short: "Check the configured model",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profileName, profile, err := a.loadProfile()
			if err != nil {
				return err
			}
			if profile.APIKey == "" {
				if profile.APIKeyConfigured {
					return fmt.Errorf("profile %q has an empty api_key value; %s", profileName, missingAPIKeyHint(profileName, true))
				}
				return fmt.Errorf("missing LLM API key for profile %q; %s", profileName, missingAPIKeyHint(profileName, false))
			}
			logger := logging.New(os.Stderr, a.verbose, isTerminal(os.Stderr))
			logger.SetShowReasoning(a.showReasoning)
			logger.SetShowProgress(a.showProgress)
			a.logger = logger
			checkReq := model.ReviewRequest{
				MaxContextTokens:        profile.MaxContextTokens,
				MaxToolCalls:            profile.MaxToolCalls,
				MaxDuplicateToolCalls:   profile.MaxDuplicateToolCalls,
				MaxOutputRetries:        profile.MaxOutputRetries,
				MaxReasoningSeconds:     profile.MaxReasoningSeconds,
				MaxReasoningLoopRepeats: profile.MaxReasoningLoopRepeats,
				UseJSONSchema:           profile.UseJSONSchema,
			}
			a.logProgress("Model", modelSummary(profile, checkReq))
			a.logProgress("Agent", agentSummary(profile, checkReq))
			client := llm.NewOpenAIClient(profile.BaseURL, profile.APIKey, profile.Model)
			client.SetLogger(logger)
			client.SetMaxRateLimitDelay(time.Duration(profile.MaxRateLimitDelaySeconds) * time.Second)
			result, err := a.resolveModelCapabilities(cmd.Context(), client, profile, a.refreshModelCheck)
			if err != nil {
				return err
			}
			a.logProgress("ModelCheck", modelCheckSummary(result))
			if a.jsonOutput {
				if err := writeJSON(struct {
					Check modelcheck.CheckSummary `json:"check"`
				}{Check: result.Summary()}); err != nil {
					return err
				}
				return validatePreReviewModelCheck(result)
			}
			if err := a.writeModelCheckOutput(result); err != nil {
				return err
			}
			return validatePreReviewModelCheck(result)
		},
	}
	modelCmd.Flags().BoolVar(&a.refreshModelCheck, "refresh", false, "Refresh stored model capabilities by running live probes")
	cmd.AddCommand(modelCmd)
	return cmd
}

func (a *app) runReview(ctx context.Context, source model.ReviewSource, retrievalEngine retrieval.Engine, profileName string, profile config.Profile, req model.ReviewRequest) error {
	logger := logging.New(os.Stderr, a.verbose, isTerminal(os.Stderr))
	logger.SetShowReasoning(a.showReasoning)
	logger.SetShowProgress(a.showProgress)
	a.logger = logger

	if profile.APIKey == "" {
		if profile.APIKeyConfigured {
			return fmt.Errorf("profile %q has an empty api_key value; %s", profileName, missingAPIKeyHint(profileName, true))
		}
		return fmt.Errorf("missing LLM API key for profile %q; %s", profileName, missingAPIKeyHint(profileName, false))
	}

	req.DisableParallelToolCalls = a.disableParallelToolCalls
	req.DisableReasoningExtract = a.disableReasoningExtract
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
	if req.MaxOutputRetries == 0 && !profile.MaxOutputRetriesConfigured {
		req.MaxOutputRetries = profile.MaxOutputRetries
	}
	if req.MaxReasoningSeconds == 0 && !profile.MaxReasoningSecondsConfigured {
		req.MaxReasoningSeconds = profile.MaxReasoningSeconds
	}
	if req.MaxReasoningLoopRepeats == 0 && !profile.MaxReasoningLoopRepeatsConfigured {
		req.MaxReasoningLoopRepeats = profile.MaxReasoningLoopRepeats
	}
	a.logProgress("Model", modelSummary(profile, req))
	a.logProgress("Agent", agentSummary(profile, req))

	client := llm.NewOpenAIClient(profile.BaseURL, profile.APIKey, profile.Model)
	client.SetLogger(logger)
	client.SetMaxRateLimitDelay(time.Duration(profile.MaxRateLimitDelaySeconds) * time.Second)
	if !a.skipModelCheck {
		checkResult, err := a.resolveModelCapabilities(ctx, client, profile, false)
		if err != nil {
			return err
		}
		if err := validatePreReviewModelCheck(checkResult); err != nil {
			return err
		}
		req.ModelEmitsReasoning = checkResult.Summary().Reasoning.Traces
		client.SetAllowedReasoningEfforts(checkResult.PassedEfforts)
		a.logProgress("ModelCheck", modelCheckSummary(checkResult))
	} else {
		req.ModelEmitsReasoning = true
		a.logProgress("ModelCheck", "skipped")
	}

	repoRoot, cleanup, err := a.resolveRepoRoot(ctx, source, profile, req)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	req.RepoRoot = repoRoot
	req.VerifyConcurrency = a.verifyConcurrency
	req.VerifyDropPolicy = a.verifyDropPolicy
	req.VerifyDropConfidence = a.verifyDropConfidence

	engine := review.NewEngine(source, client, retrievalEngine, profile)
	engine.SetLogger(logger)
	engine.SetSearchToolOptimization(!a.disableSearchToolOptimization)
	result, trimmedCtx, err := engine.RunWithContext(ctx, req)
	if errors.Is(err, llm.ErrInvalidJSON) {
		a.logProgress("Result", fmt.Sprintf("status=InvalidJson, error=%v", err))
		return err
	}
	if err != nil {
		a.logProgress("Result", fmt.Sprintf("status=ERROR, error=%v", err))
		return err
	}
	a.logProgress("Result", reviewResultSummary(result))

	if len(result.Findings) > 0 && trimmedCtx != nil {
		finalizeOpts := review.FinalizeOptions{
			UseJSONSchema:            req.UseJSONSchema,
			MaxOutputRetries:         req.MaxOutputRetries,
			MaxReasoningSeconds:      req.MaxReasoningSeconds,
			MaxReasoningLoopRepeats:  req.MaxReasoningLoopRepeats,
			DisableParallelToolCalls: req.DisableParallelToolCalls,
			RepoRoot:                 req.RepoRoot,
		}
		finalized, finalizeRun, finalizeErr := engine.Finalize(ctx, trimmedCtx, result, finalizeOpts)
		if finalizeErr != nil {
			a.logProgress("Finalize", fmt.Sprintf("status=ERROR, error=%v; falling back to verified result", finalizeErr))
			result.Warnings = append(result.Warnings, fmt.Sprintf("Finalize failed: %v; using verified result", finalizeErr))
			finalizeRun.Name = "finalize"
			finalizeRun.Role = "finalize"
			finalizeRun.Status = model.AgentRunStatusFailed
			finalizeRun.Error = finalizeErr.Error()
			result.FinalizeTokensUsed = finalizeRun.TokensUsed
			result.AgentRuns = append(result.AgentRuns, finalizeRun)
		} else {
			finalized.FinalizeTokensUsed = finalizeRun.TokensUsed
			finalized.AgentRuns = append(finalized.AgentRuns, finalizeRun)
			result = finalized
		}
	}

	var formatter output.Formatter
	if a.jsonOutput {
		formatter = output.NewJSONFormatter(os.Stdout)
	} else {
		formatter = output.NewTerminalFormatter(os.Stdout, true)
	}
	if err := formatter.FormatFindings(result); err != nil {
		return err
	}
	// Publish the review back to the origin (GitLab MR) when requested. The
	// review already succeeded and printed to stdout, so a publish failure is a
	// warning, never a hard error. Only sources implementing ReviewPublisher
	// (GitLab) act here; github/local are unaffected.
	if req.PostReview && (len(result.Findings) > 0 || strings.TrimSpace(result.OverallExplanation) != "") {
		if publisher, ok := source.(model.ReviewPublisher); ok {
			a.logProgress("Publish", fmt.Sprintf("posting review to %s !%d", req.Repo, req.Identifier))
			if err := publisher.PublishReview(ctx, req, result); err != nil {
				a.logProgress("Publish", fmt.Sprintf("status=ERROR, error=%v", err))
				result.Warnings = append(result.Warnings, fmt.Sprintf("Publish failed: %v", err))
			} else {
				a.logProgress("Publish", "done")
			}
		} else {
			a.logProgress("Publish", "skipped (source does not support publishing)")
		}
	}
	// Distinguish "review produced nothing because every reviewer crashed"
	// from "review succeeded with some soft warnings" — only the former is a
	// CI-level failure. Empty findings alone are not a failure (clean diff).
	if reviewProducedNothing(result) {
		return fmt.Errorf("review failed: all reviewer agents errored (%d warning(s))", len(result.Warnings))
	}
	return nil
}

func (a *app) resolveModelCapabilities(ctx context.Context, client llm.Client, profile config.Profile, refresh bool) (modelcheck.Result, error) {
	if !refresh {
		if capability, ok := modelcheck.FindProfileCapability(profile); ok {
			result := modelcheck.ResultFromCapability(capability, profile.UseJSONSchema)
			a.logProgress("ModelCheck", "source=profile")
			return result, nil
		}
		cachePath, err := modelcheck.DefaultCachePath()
		if err != nil {
			a.logf("Model capability cache unavailable: %v", err)
		} else {
			capability, ok, err := modelcheck.ReadCachedCapability(cachePath, profile.BaseURL, profile.Model)
			if err == nil && ok {
				result := modelcheck.ResultFromCapability(capability, profile.UseJSONSchema)
				a.logProgress("ModelCheck", "source=cache")
				return result, nil
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				a.logf("Model capability cache ignored: %v", err)
			}
		}
	}

	checker := modelcheck.New(client, profile)
	checker.SetLogger(a.logger)
	checker.SetParallel(!a.disableParallelToolCalls)
	result := checker.Run(ctx)
	if err := validatePreReviewModelCheck(result); err != nil {
		return result, nil
	}
	capability := modelcheck.CapabilityFromResult(result)
	cachePath, err := modelcheck.DefaultCachePath()
	if err != nil {
		a.logf("Model capability cache unavailable: %v", err)
		return result, nil
	}
	if err := modelcheck.WriteCachedCapability(cachePath, profile.BaseURL, capability, time.Now()); err != nil {
		a.logf("Model capability cache write failed: %v", err)
	}
	return result, nil
}

// reviewProducedNothing reports whether the review pipeline collapsed: every
// reviewer-role AgentRun failed and no findings emerged. Returns false if any
// reviewer succeeded (even partially) — those runs may legitimately produce no
// findings on a clean diff.
func reviewProducedNothing(result *model.ReviewResult) bool {
	if result == nil {
		return false
	}
	if len(result.Findings) > 0 {
		return false
	}
	reviewerSeen := false
	for _, run := range result.AgentRuns {
		if run.Role != "reviewer" {
			continue
		}
		reviewerSeen = true
		if run.Status != model.AgentRunStatusFailed {
			return false
		}
	}
	return reviewerSeen
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

func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	return err == nil && (stat.Mode()&os.ModeCharDevice) != 0
}

func (a *app) writeModelCheckOutput(result modelcheck.Result) error {
	s := result.Summary()
	useANSI := isTerminal(os.Stdout)

	mark := func(v bool) string {
		if useANSI {
			if v {
				return "\x1b[32m✓\x1b[0m"
			}
			return "\x1b[31m✗\x1b[0m"
		}
		if v {
			return "✓"
		}
		return "✗"
	}
	label := func(text string) string {
		if useANSI {
			return "\x1b[1m" + text + "\x1b[0m"
		}
		return text
	}
	effort := func(e string) string {
		if useANSI {
			return "\x1b[34m" + e + "\x1b[0m"
		}
		return e
	}
	optionalMark := func(v *bool) string {
		if v == nil {
			return "?"
		}
		return mark(*v)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %s\n", mark(s.Compatible), label("Model is compatible"))
	fmt.Fprintf(&sb, "\n")
	fmt.Fprintf(&sb, "%s Response\n", mark(s.Response))
	fmt.Fprintf(&sb, "%s Tool Use\n", mark(s.Tools))
	fmt.Fprintf(&sb, "%s Structured Output\n", optionalMark(s.JSONResponse))
	if s.JSONSchema != nil {
		fmt.Fprintf(&sb, "%s JSON Schema\n", optionalMark(s.JSONSchema))
	}
	fmt.Fprintf(&sb, "%s Reasoning Traces\n", mark(s.Reasoning.Traces))
	fmt.Fprintf(&sb, "\n")
	fmt.Fprintf(&sb, "%s\n", label("Supported Efforts"))
	if len(s.Reasoning.Efforts) == 0 {
		fmt.Fprintf(&sb, "  none\n")
	}
	for _, e := range s.Reasoning.Efforts {
		fmt.Fprintf(&sb, "  %s %s\n", mark(true), effort(e))
	}
	_, err := fmt.Fprint(os.Stdout, sb.String())
	return err
}

func validatePreReviewModelCheck(result modelcheck.Result) error {
	if len(result.PassedEfforts) == 0 {
		return fmt.Errorf("model check failed: no reasoning efforts passed")
	}
	if probe := result.ConfiguredTools(); probe.Status != modelcheck.StatusOK {
		return fmt.Errorf("model check failed for tool use at reasoning effort %q: status=%s error=%s", probe.ReasoningEffort, probe.Status, probe.Error)
	}
	if result.UseJSONSchema {
		if probe := result.ConfiguredJSONSchema(); probe.Status != modelcheck.StatusOK {
			return fmt.Errorf("model check failed for JSON schema output at reasoning effort %q: status=%s error=%s", probe.ReasoningEffort, probe.Status, probe.Error)
		}
	} else {
		if probe := result.ConfiguredJSONOutput(); probe.Status != modelcheck.StatusOK {
			return fmt.Errorf("model check failed for JSON text output at reasoning effort %q: status=%s error=%s", probe.ReasoningEffort, probe.Status, probe.Error)
		}
	}
	return nil
}

func modelCheckSummary(result modelcheck.Result) string {
	counts := map[modelcheck.Status]int{}
	for _, probe := range result.Probes {
		counts[probe.Status]++
	}
	return fmt.Sprintf(
		"ok=%d unsupported=%d failed=%d passed_efforts=%s",
		counts[modelcheck.StatusOK],
		counts[modelcheck.StatusUnsupported],
		counts[modelcheck.StatusFailed],
		strings.Join(result.PassedEfforts, ","),
	)
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
	if _, after, ok := strings.Cut(raw, ":"); ok {
		return strings.TrimSuffix(after, ".git")
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
	if profile.Temperature != nil {
		flags = append(flags, fmt.Sprintf("temp=%g", *profile.Temperature))
	}
	if profile.TopP != nil {
		flags = append(flags, fmt.Sprintf("top_p=%g", *profile.TopP))
	}
	return fmt.Sprintf("%s:%s [%s] @ %s",
		profile.Model, profile.ReasoningEffort,
		strings.Join(flags, ", "),
		profile.BaseURL,
	)
}

func agentSummary(profile config.Profile, req model.ReviewRequest) string {
	kind := "Unstructured"
	if req.UseJSONSchema {
		kind = "Structured"
	}
	flags := []string{
		disablable(req.NudgeCount, "", "nudges"),
		disablable(req.MaxOutputRetries, "", "retries"),
		unlimited(req.MaxReasoningSeconds, "s", "reasoning"),
		unlimited(req.MaxReasoningLoopRepeats, "", "loop repeats"),
		disablable(profile.MaxRateLimitDelaySeconds, "s", "rate-limit-delay"),
		unlimited(req.MaxToolCalls, "", "tool calls"),
		unlimited(req.MaxDuplicateToolCalls, "", "duplicates"),
	}
	if !req.DisableParallelToolCalls {
		flags = append(flags, "parallel")
	}
	return fmt.Sprintf("%s [%s]", kind, strings.Join(flags, ", "))
}

func unlimited(n int, unit, label string) string {
	if n <= 0 {
		return "∞ " + label
	}
	return fmt.Sprintf("≤%d%s %s", n, unit, label)
}

func disablable(n int, unit, label string) string {
	if n <= 0 {
		return "no " + label
	}
	return fmt.Sprintf("≤%d%s %s", n, unit, label)
}

func reviewResultSummary(result *model.ReviewResult) string {
	return strings.Join([]string{
		"status=OK",
		fmt.Sprintf("findings=%d", len(result.Findings)),
		fmt.Sprintf("total_tool_calls=%d", result.TotalToolCalls),
		fmt.Sprintf("duplicate_tool_calls=%d", totalDuplicateToolCalls(result.AgentRuns)),
		fmt.Sprintf("prompt_tokens=%d", result.TokensUsed.PromptTokens),
		fmt.Sprintf("completion_tokens=%d", result.TokensUsed.CompletionTokens),
		fmt.Sprintf("total_tokens=%d", result.TokensUsed.TotalTokens),
	}, ", ")
}

func totalDuplicateToolCalls(runs []model.AgentRun) int {
	total := 0
	for _, run := range runs {
		total += run.DuplicateToolCalls
	}
	return total
}
