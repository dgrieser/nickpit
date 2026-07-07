package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"sort"
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
	"github.com/dgrieser/nickpit/internal/serve"
	"github.com/dgrieser/nickpit/internal/styleguide"
	"github.com/dgrieser/nickpit/internal/workflow"
	"github.com/dgrieser/nickpit/mappings"
	"github.com/spf13/cobra"
)

// version is overridden at release build time via -ldflags "-X main.version=...".
var version = "dev"

type app struct {
	model                   string
	smallModel              string
	baseURL                 string
	apiKey                  string
	workDir                 string
	profile                 string
	profileSet              bool
	temperature             float64
	temperatureSet          bool
	topP                    float64
	topPSet                 bool
	topK                    int
	topKSet                 bool
	presencePenalty         float64
	presencePenaltySet      bool
	extraBody               string
	maxOutputTokens         int
	maxOutputTokensSet      bool
	smallMaxTokens          int
	smallMaxTokensSet       bool
	smallTemperature        float64
	smallTemperatureSet     bool
	smallTopP               float64
	smallTopPSet            bool
	smallTopK               int
	smallTopKSet            bool
	smallPresencePenalty    float64
	smallPresencePenaltySet bool
	smallExtraBody          string
	maxContextTokens        int
	maxContextTokensSet     bool
	includeFullFiles        bool
	includeComments         bool
	includeCommits          bool
	includePaths            []string
	includePathsSet         bool
	excludePaths            []string
	excludePathsSet         bool
	includeContent          []string
	includeContentSet       bool
	excludeContent          []string
	excludeContentSet       bool
	// styleGuides needs no companion Set bool: CLI values append to the
	// profile's list, so an unset (nil) flag is indistinguishable from empty.
	styleGuides                   []string
	disableStyleGuides            []string
	diffFormat                    string
	jsonOutput                    bool
	disableJSONResponseFormat     bool
	maxToolCalls                  int
	maxToolCallsSet               bool
	maxDuplicateToolCalls         int
	maxDuplicateToolCallsSet      bool
	maxOutputRetries              int
	maxOutputRetriesSet           bool
	maxReasoningSeconds           int
	maxReasoningSecondsSet        bool
	maxRateLimitDelaySeconds      int
	maxRateLimitDelaySecondsSet   bool
	nudgeCount                    int
	nudgeCountSet                 bool
	maxFindings                   int
	maxFindingsSet                bool
	priorityThreshold             string
	configPath                    string
	githubToken                   string
	gitlabToken                   string
	gitlabBaseURL                 string
	verbose                       bool
	reasoningEffort               string
	smallReasoningEffort          string
	showReasoning                 bool
	showProgress                  bool
	disableSearchToolOptimization bool
	disableParallelToolCalls      bool
	disableReasoningExtract       bool
	disablePatchSummary           bool
	disableSuggestions            bool
	disableWorkflowTimeBudget     bool
	concurrency                   int
	verifyDropPolicy              string
	confidenceThreshold           float64
	disableModelCheck             bool
	refreshModelCheck             bool
	specPath                      string
	stepName                      string
	findingsFiles                 []string
	logger                        *logging.Logger
	// reviewStart anchors the whole-review runtime (model check, checkout,
	// pipeline through summarize), stamped at runReview entry.
	reviewStart time.Time
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
		maxRateLimitDelaySeconds: config.DefaultMaxRateLimitDelaySeconds,
		nudgeCount:               config.DefaultNudgeCount,
	}
	root := &cobra.Command{
		Use:           "nickpit",
		Short:         "AI-powered code review for local git, GitHub PRs, and GitLab MRs",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			rootFlags := cmd.Root().PersistentFlags()
			cli.profileSet = rootFlags.Changed("profile")
			cli.includePathsSet = rootFlags.Changed("include-path")
			cli.excludePathsSet = rootFlags.Changed("exclude-path")
			cli.includeContentSet = rootFlags.Changed("include-content")
			cli.excludeContentSet = rootFlags.Changed("exclude-content")
			if err := review.ValidateDropPolicy(cli.verifyDropPolicy); err != nil {
				return err
			}
			normalizedPriority, err := model.NormalizePriorityThreshold(cli.priorityThreshold)
			if err != nil {
				return fmt.Errorf("invalid --priority-threshold: %w", err)
			}
			cli.priorityThreshold = normalizedPriority
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
	root.PersistentFlags().StringVar(&cli.smallModel, "small-model", "", "Small model identifier for workflow steps using model: \"@small\"")
	root.PersistentFlags().StringVar(&cli.baseURL, "base-url", "", "LLM API base URL")
	root.PersistentFlags().StringVar(&cli.apiKey, "api-key", "", "LLM API key")
	root.PersistentFlags().StringVar(&cli.workDir, "workdir", "", "Working directory")
	root.PersistentFlags().StringVar(&cli.profile, "profile", "default", "Config profile name")
	root.PersistentFlags().Var(newTrackedFloatValue(&cli.temperature, &cli.temperatureSet), "temperature", "Sampling temperature")
	root.PersistentFlags().Var(newTrackedFloatValue(&cli.topP, &cli.topPSet), "top-p", "Nucleus sampling probability")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.topK, &cli.topKSet), "top-k", "Top-k sampling cutoff")
	root.PersistentFlags().Var(newTrackedFloatValue(&cli.presencePenalty, &cli.presencePenaltySet), "presence-penalty", "Presence penalty")
	root.PersistentFlags().StringVar(&cli.extraBody, "extra-body", "", "Additional JSON object fields to merge into the LLM request body")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxOutputTokens, &cli.maxOutputTokensSet), "max-output-tokens", "Maximum output (completion) tokens the model may generate; distinct from --max-context-tokens (input budget)")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.smallMaxTokens, &cli.smallMaxTokensSet), "small-max-output-tokens", "Maximum output (completion) tokens for workflow steps using model: \"@small\"")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.smallMaxTokens, &cli.smallMaxTokensSet), "small-max-tokens", "Deprecated alias for --small-max-output-tokens")
	_ = root.PersistentFlags().MarkDeprecated("small-max-tokens", "use --small-max-output-tokens")
	root.PersistentFlags().Var(newTrackedFloatValue(&cli.smallTemperature, &cli.smallTemperatureSet), "small-temperature", "Sampling temperature for workflow steps using model: \"@small\"")
	root.PersistentFlags().Var(newTrackedFloatValue(&cli.smallTopP, &cli.smallTopPSet), "small-top-p", "Nucleus sampling probability for workflow steps using model: \"@small\"")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.smallTopK, &cli.smallTopKSet), "small-top-k", "Top-k sampling cutoff for workflow steps using model: \"@small\"")
	root.PersistentFlags().Var(newTrackedFloatValue(&cli.smallPresencePenalty, &cli.smallPresencePenaltySet), "small-presence-penalty", "Presence penalty for workflow steps using model: \"@small\"")
	root.PersistentFlags().StringVar(&cli.smallExtraBody, "small-extra-body", "", "Additional JSON object fields for workflow steps using model: \"@small\"")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxContextTokens, &cli.maxContextTokensSet), "max-context-tokens", "Context token budget")
	root.PersistentFlags().BoolVar(&cli.includeFullFiles, "include-full-files", false, "Include full changed files")
	root.PersistentFlags().BoolVar(&cli.includeComments, "include-comments", true, "Include existing comments")
	root.PersistentFlags().BoolVar(&cli.includeCommits, "include-commits", true, "Include commit summaries")
	root.PersistentFlags().StringArrayVar(&cli.includePaths, "include-path", nil, "Include changed files whose repo-relative path matches this regex; repeatable")
	root.PersistentFlags().StringArrayVar(&cli.excludePaths, "exclude-path", nil, "Exclude changed files whose repo-relative path matches this regex; repeatable")
	root.PersistentFlags().StringArrayVar(&cli.includeContent, "include-content", nil, "Include changed files whose full post-change content matches this regex; repeatable")
	root.PersistentFlags().StringArrayVar(&cli.excludeContent, "exclude-content", nil, "Exclude changed files whose full post-change content matches this regex; repeatable")
	root.PersistentFlags().StringArrayVar(&cli.styleGuides, "styleguide", nil, "Additional styleguide for all agents: file path or HTTP(S) URL, appended to the profile's styleguides; repeatable")
	root.PersistentFlags().StringArrayVar(&cli.disableStyleGuides, "disable-styleguide", nil, "Disable the built-in styleguide for a language; available: all, "+strings.Join(mappings.StyleGuideOrder(), ", ")+"; repeatable")
	root.PersistentFlags().StringVar(&cli.diffFormat, "diff-format", "", "Diff format for agent prompts: git or git-json")
	root.PersistentFlags().BoolVar(&cli.jsonOutput, "json", false, "Emit JSON output")
	root.PersistentFlags().BoolVar(&cli.disableJSONResponseFormat, "disable-json-response-format", false, "Disable the API-enforced JSON response format (response_format json_schema); on by default, falls back to prompt-embedded schema")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxToolCalls, &cli.maxToolCallsSet), "max-tool-calls", "Maximum tool-call rounds (0 means unlimited by default)")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxDuplicateToolCalls, &cli.maxDuplicateToolCallsSet), "max-duplicate-tool-calls", "Maximum duplicate tool calls before tools are disabled")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxOutputRetries, &cli.maxOutputRetriesSet), "max-output-retries", "Maximum invalid output retries")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxReasoningSeconds, &cli.maxReasoningSecondsSet), "max-reasoning-seconds", "Maximum seconds to allow reasoning before falling back to lower effort")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxRateLimitDelaySeconds, &cli.maxRateLimitDelaySecondsSet), "max-rate-limit-delay-seconds", "Maximum seconds to wait for rate-limit reset times parsed from 429 responses (0 disables)")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.nudgeCount, &cli.nudgeCountSet), "nudge-count", "Number of nudge rounds asking each reviewer to look again (0 disables)")
	root.PersistentFlags().Var(newTrackedIntValue(&cli.maxFindings, &cli.maxFindingsSet), "max-findings", "Maximum findings each review agent may report; weakest findings are cut when exceeded (0 = unlimited)")
	root.PersistentFlags().StringVar(&cli.priorityThreshold, "priority-threshold", "3", "Minimum priority to display: 0 (highest) to 3 (lowest)")
	root.PersistentFlags().StringVar(&cli.configPath, "config", ".nickpit.yaml", "Config file path")
	root.PersistentFlags().StringVar(&cli.githubToken, "github-token", "", "GitHub token override")
	root.PersistentFlags().StringVar(&cli.gitlabToken, "gitlab-token", "", "GitLab token override")
	root.PersistentFlags().StringVar(&cli.gitlabBaseURL, "gitlab-base-url", "", "GitLab API base URL")
	root.PersistentFlags().BoolVar(&cli.verbose, "verbose", false, "Print debug execution details")
	root.PersistentFlags().BoolVar(&cli.verbose, "debug", false, "Print debug execution details")
	root.PersistentFlags().StringVar(&cli.reasoningEffort, "reasoning-effort", "", "Reasoning effort level; known fallback ladder: max, xhigh, high, medium, low, minimal, none, off")
	root.PersistentFlags().StringVar(&cli.smallReasoningEffort, "small-reasoning-effort", "", "Reasoning effort for --small-model / workflow model: \"@small\"; defaults to --reasoning-effort")
	root.PersistentFlags().BoolVar(&cli.showReasoning, "show-reasoning", false, "Print streamed model reasoning to stderr")
	root.PersistentFlags().BoolVar(&cli.showProgress, "show-progress", false, "Print review progress to stderr")
	root.PersistentFlags().BoolVar(&cli.disableSearchToolOptimization, "disable-search-tool-optimization", false, "Disable rewriting search tool calls like FunctionName( into find_callers")
	root.PersistentFlags().BoolVar(&cli.disableParallelToolCalls, "disable-parallel-tool-calls", false, "Disable parallel tool calls and the prompt guidance that encourages batching")
	root.PersistentFlags().BoolVar(&cli.disableReasoningExtract, "disable-reasoning-extract", false, "Disable the reasoning-extractor agent that augments nudge prompts with issues the reviewer only reasoned about")
	root.PersistentFlags().BoolVar(&cli.disablePatchSummary, "disable-patch-summary", false, "Omit the assumed patch-purpose summary from the final review output")
	root.PersistentFlags().BoolVar(&cli.disableSuggestions, "disable-suggestions", false, "Omit code suggestions from prompts and review output")
	root.PersistentFlags().BoolVar(&cli.disableWorkflowTimeBudget, "disable-workflow-time-budget", false, "Ignore time_budget entries in workflow specs")
	root.PersistentFlags().IntVar(&cli.concurrency, "concurrency", 10, "Maximum parallel LLM agent loops across the whole run (0 = unlimited)")
	root.PersistentFlags().StringVar(&cli.verifyDropPolicy, "verify-drop-policy", "refuted-only", "Which verifier verdicts cause a finding to be dropped before merge: none, refuted-only, refuted-and-unverified")
	root.PersistentFlags().Float64Var(&cli.confidenceThreshold, "confidence-threshold", 0.7, "Minimum finalized confidence_score required for the verdict step to keep a finding (0 = keep all)")
	root.PersistentFlags().BoolVar(&cli.disableModelCheck, "disable-model-check", false, "Disable pre-review model capability checks")
	root.PersistentFlags().StringVar(&cli.specPath, "spec", "", "Run a workflow spec file (YAML) instead of the embedded default workflow")
	root.PersistentFlags().StringVar(&cli.stepName, "step", "", "Run a single pipeline step (e.g. merge, finalize, verdict, summarize, review:security); mutually exclusive with --spec")
	root.PersistentFlags().StringArrayVar(&cli.findingsFiles, "findings", nil, "Findings JSON file(s) to inject; repeatable. For --step merge each file is one group")

	root.AddCommand(cli.newCheckCmd())
	root.AddCommand(cli.newGitCmd())
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
	var rateLimitDelaySeconds *int
	if a.maxRateLimitDelaySecondsSet {
		rateLimitDelaySeconds = &a.maxRateLimitDelaySeconds
	}
	var nudgeCount *int
	if a.nudgeCountSet {
		nudgeCount = &a.nudgeCount
	}
	var maxFindings *int
	if a.maxFindingsSet {
		maxFindings = &a.maxFindings
	}
	var temperature *float64
	if a.temperatureSet {
		temperature = &a.temperature
	}
	var topP *float64
	if a.topPSet {
		topP = &a.topP
	}
	var topK *int
	if a.topKSet {
		topK = &a.topK
	}
	var presencePenalty *float64
	if a.presencePenaltySet {
		presencePenalty = &a.presencePenalty
	}
	var maxTokens *int
	if a.maxOutputTokensSet {
		maxTokens = &a.maxOutputTokens
	}
	var extraBody map[string]any
	if strings.TrimSpace(a.extraBody) != "" {
		parsed, err := parseExtraBodyFlag("--extra-body", a.extraBody)
		if err != nil {
			return "", config.Profile{}, err
		}
		extraBody = parsed
	}
	var smallMaxTokens *int
	if a.smallMaxTokensSet {
		smallMaxTokens = &a.smallMaxTokens
	}
	var smallTemperature *float64
	if a.smallTemperatureSet {
		smallTemperature = &a.smallTemperature
	}
	var smallTopP *float64
	if a.smallTopPSet {
		smallTopP = &a.smallTopP
	}
	var smallTopK *int
	if a.smallTopKSet {
		smallTopK = &a.smallTopK
	}
	var smallPresencePenalty *float64
	if a.smallPresencePenaltySet {
		smallPresencePenalty = &a.smallPresencePenalty
	}
	var smallExtraBody map[string]any
	if strings.TrimSpace(a.smallExtraBody) != "" {
		parsed, err := parseExtraBodyFlag("--small-extra-body", a.smallExtraBody)
		if err != nil {
			return "", config.Profile{}, err
		}
		smallExtraBody = parsed
	}
	var includePaths *[]string
	if a.includePathsSet {
		includePaths = &a.includePaths
	}
	var excludePaths *[]string
	if a.excludePathsSet {
		excludePaths = &a.excludePaths
	}
	var includeContent *[]string
	if a.includeContentSet {
		includeContent = &a.includeContent
	}
	var excludeContent *[]string
	if a.excludeContentSet {
		excludeContent = &a.excludeContent
	}
	// Only override the profile when --profile was given explicitly (or set
	// programmatically via loadProfileNamed); the flag default "default" must
	// not shadow an active_profile from the config file.
	profileOverride := ""
	if a.profileSet {
		profileOverride = a.profile
	}
	cfg, profile, err := config.Load(a.configPath, config.Overrides{
		Profile: profileOverride,
		Model:   a.model,
		Small: config.SmallModelConfig{
			Model:           a.smallModel,
			MaxTokens:       smallMaxTokens,
			Temperature:     smallTemperature,
			TopP:            smallTopP,
			TopK:            smallTopK,
			PresencePenalty: smallPresencePenalty,
			ExtraBody:       smallExtraBody,
			ReasoningEffort: a.smallReasoningEffort,
		},
		BaseURL:                   a.baseURL,
		APIKey:                    a.apiKey,
		ReasoningEffort:           a.reasoningEffort,
		Temperature:               temperature,
		TopP:                      topP,
		TopK:                      topK,
		PresencePenalty:           presencePenalty,
		ExtraBody:                 extraBody,
		MaxTokens:                 maxTokens,
		DisableJSONResponseFormat: a.disableJSONResponseFormat,
		IncludePaths:              includePaths,
		ExcludePaths:              excludePaths,
		IncludeContent:            includeContent,
		ExcludeContent:            excludeContent,
		StyleGuides:               a.styleGuides,
		DisableStyleGuides:        a.disableStyleGuides,
		DiffFormat:                model.DiffFormat(a.diffFormat),
		MaxContextTokens:          maxContextTokens,
		ToolCalls:                 toolCalls,
		DuplicateToolCalls:        duplicateToolCalls,
		OutputRetries:             outputRetries,
		ReasoningSeconds:          reasoningSeconds,
		RateLimitDelaySeconds:     rateLimitDelaySeconds,
		NudgeCount:                nudgeCount,
		MaxFindings:               maxFindings,
		DisablePatchSummary:       a.disablePatchSummary,
		DisableSuggestions:        a.disableSuggestions,
		DisableWorkflowTimeBudget: a.disableWorkflowTimeBudget,
		Workdir:                   a.workDir,
		GitHubToken:               a.githubToken,
		GitLabToken:               a.gitlabToken,
		GitLabBaseURL:             a.gitlabBaseURL,
	})
	if err != nil {
		return "", config.Profile{}, err
	}
	return cfg.ActiveProfile, profile, nil
}

func parseExtraBodyFlag(name, raw string) (map[string]any, error) {
	var extraBody map[string]any
	if err := json.Unmarshal([]byte(raw), &extraBody); err != nil {
		return nil, fmt.Errorf("parsing %s JSON object: %w", name, err)
	}
	if extraBody == nil {
		return nil, fmt.Errorf("%s must be a JSON object", name)
	}
	return extraBody, nil
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

func (a *app) newGitCmd() *cobra.Command {
	local := &cobra.Command{Use: "git", Short: "Review local git changes"}
	local.AddCommand(a.newLocalReviewCmd("uncommitted"))
	local.AddCommand(a.newLocalReviewCmd("staged"))
	local.AddCommand(a.newLocalReviewCmd("unstaged"))
	local.AddCommand(a.newLocalReviewCmd("commits"))
	local.AddCommand(a.newLocalReviewCmd("branch"))
	return local
}

func (a *app) newLocalReviewCmd(submode string) *cobra.Command {
	var from, to, base, head string
	cmd := &cobra.Command{
		Use:   submode,
		Short: localReviewShort(submode),
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			profileName, profile, err := a.loadProfileForSpec()
			if err != nil {
				return err
			}
			req := model.ReviewRequest{
				Mode:                      model.ModeLocal,
				RepoRoot:                  repoRoot,
				Workdir:                   profile.Workdir,
				BaseRef:                   firstNonEmpty(base, from),
				HeadRef:                   firstNonEmpty(head, to),
				IncludeComments:           a.includeComments,
				IncludeCommits:            a.includeCommits,
				IncludeFullFiles:          a.includeFullFiles,
				IncludePaths:              profile.IncludePaths,
				ExcludePaths:              profile.ExcludePaths,
				IncludeContent:            profile.IncludeContent,
				ExcludeContent:            profile.ExcludeContent,
				DiffFormat:                profile.DiffFormat,
				MaxContextTokens:          profile.MaxContextTokens,
				MaxToolCalls:              profile.MaxToolCalls,
				MaxDuplicateToolCalls:     profile.MaxDuplicateToolCalls,
				MaxOutputRetries:          profile.MaxOutputRetries,
				MaxReasoningSeconds:       profile.MaxReasoningSeconds,
				NudgeCount:                profile.NudgeCount,
				MaxFindings:               profile.MaxFindings,
				DisablePatchSummary:       profile.DisablePatchSummary,
				DisableSuggestions:        profile.DisableSuggestions,
				DisableJSONResponseFormat: profile.DisableJSONResponseFormat,
				PriorityThreshold:         a.priorityThreshold,
				ConfidenceThreshold:       a.confidenceThreshold,
				Submode:                   submode,
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

func localReviewShort(submode string) string {
	switch submode {
	case "uncommitted":
		return "Review staged and unstaged tracked changes against HEAD; untracked files excluded"
	case "staged":
		return "Review staged changes"
	case "unstaged":
		return "Review unstaged tracked changes"
	default:
		return "Run a local review"
	}
}

func (a *app) newGitHubCmd() *cobra.Command {
	var repo string
	var pr int
	var rawURL string
	var publish bool
	cmd := &cobra.Command{
		Use:   "github",
		Short: "Review GitHub pull requests",
	}
	prCmd := &cobra.Command{
		Use:   "pr",
		Short: "Review a GitHub PR",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("url") {
				if cmd.Flags().Changed("id") {
					return fmt.Errorf("--url can not be combined with --id")
				}
				if cmd.Flags().Changed("repo") {
					return fmt.Errorf("--url can not be combined with --repo")
				}
				var err error
				repo, pr, err = parseGitHubPRURL(rawURL)
				if err != nil {
					return err
				}
			} else {
				if pr <= 0 {
					return fmt.Errorf("--id must be a positive integer")
				}
			}
			if repo == "" {
				repo = inferRepo()
				if repo == "" {
					return fmt.Errorf("--repo is required (could not infer from git remote)")
				}
			}
			profileName, profile, err := a.loadProfileForSpec()
			if err != nil {
				return err
			}
			source := ghscm.NewAdapter(ghscm.NewClient("", profile.GitHubToken), profile.AssetBaseURL)
			req := model.ReviewRequest{
				Mode:                      model.ModeGitHub,
				Workdir:                   profile.Workdir,
				Repo:                      repo,
				Identifier:                pr,
				IncludeComments:           a.includeComments,
				IncludeCommits:            a.includeCommits,
				IncludePaths:              profile.IncludePaths,
				ExcludePaths:              profile.ExcludePaths,
				IncludeContent:            profile.IncludeContent,
				ExcludeContent:            profile.ExcludeContent,
				DiffFormat:                profile.DiffFormat,
				MaxContextTokens:          profile.MaxContextTokens,
				MaxToolCalls:              profile.MaxToolCalls,
				MaxDuplicateToolCalls:     profile.MaxDuplicateToolCalls,
				MaxOutputRetries:          profile.MaxOutputRetries,
				MaxReasoningSeconds:       profile.MaxReasoningSeconds,
				NudgeCount:                profile.NudgeCount,
				MaxFindings:               profile.MaxFindings,
				DisablePatchSummary:       profile.DisablePatchSummary,
				DisableSuggestions:        profile.DisableSuggestions,
				DisableJSONResponseFormat: profile.DisableJSONResponseFormat,
				PriorityThreshold:         a.priorityThreshold,
				ConfidenceThreshold:       a.confidenceThreshold,
				PostReview:                publish,
			}
			return a.runReview(cmd.Context(), source, retrieval.NewLocalEngine(), profileName, profile, req)
		},
	}
	prCmd.Flags().StringVar(&repo, "repo", "", "GitHub repo owner/name (inferred from git remote if omitted)")
	prCmd.Flags().IntVar(&pr, "id", 0, "Pull request number")
	prCmd.Flags().StringVar(&rawURL, "url", "", "GitHub pull request URL")
	prCmd.Flags().BoolVar(&publish, "publish", false, "Post the review back to the GitHub PR as a review (summary + one comment per finding)")
	cmd.AddCommand(prCmd)
	return cmd
}

func (a *app) newGitLabCmd() *cobra.Command {
	var project string
	var mr int
	var rawURL string
	var publish bool
	cmd := &cobra.Command{
		Use:   "gitlab",
		Short: "Review GitLab merge requests",
	}
	mrCmd := &cobra.Command{
		Use:   "mr",
		Short: "Review a GitLab merge request",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("url") {
				if cmd.Flags().Changed("id") {
					return fmt.Errorf("--url can not be combined with --id")
				}
				if cmd.Flags().Changed("repo") {
					return fmt.Errorf("--url can not be combined with --repo")
				}
				var gitlabBaseURL string
				var err error
				project, mr, gitlabBaseURL, err = parseGitLabMRURL(rawURL)
				if err != nil {
					return err
				}
				a.gitlabBaseURL = gitlabBaseURL
			} else {
				if mr <= 0 {
					return fmt.Errorf("--id must be a positive integer")
				}
			}
			if project == "" {
				project = inferRepo()
				if project == "" {
					return fmt.Errorf("--repo is required (could not infer from git remote)")
				}
			}
			profileName, profile, err := a.loadProfileForSpec()
			if err != nil {
				return err
			}
			source := glscm.NewAdapter(glscm.NewClient(profile.GitLabBaseURL, profile.GitLabToken), profile.AssetBaseURL)
			req := model.ReviewRequest{
				Mode:                      model.ModeGitLab,
				Workdir:                   profile.Workdir,
				Repo:                      project,
				Identifier:                mr,
				IncludeComments:           a.includeComments,
				IncludeCommits:            a.includeCommits,
				IncludePaths:              profile.IncludePaths,
				ExcludePaths:              profile.ExcludePaths,
				IncludeContent:            profile.IncludeContent,
				ExcludeContent:            profile.ExcludeContent,
				DiffFormat:                profile.DiffFormat,
				MaxContextTokens:          profile.MaxContextTokens,
				MaxToolCalls:              profile.MaxToolCalls,
				MaxDuplicateToolCalls:     profile.MaxDuplicateToolCalls,
				MaxOutputRetries:          profile.MaxOutputRetries,
				MaxReasoningSeconds:       profile.MaxReasoningSeconds,
				NudgeCount:                profile.NudgeCount,
				MaxFindings:               profile.MaxFindings,
				DisablePatchSummary:       profile.DisablePatchSummary,
				DisableSuggestions:        profile.DisableSuggestions,
				DisableJSONResponseFormat: profile.DisableJSONResponseFormat,
				PriorityThreshold:         a.priorityThreshold,
				ConfidenceThreshold:       a.confidenceThreshold,
				PostReview:                publish,
			}
			return a.runReview(cmd.Context(), source, retrieval.NewLocalEngine(), profileName, profile, req)
		},
	}
	mrCmd.Flags().StringVar(&project, "repo", "", "GitLab project group/name (inferred from git remote if omitted)")
	mrCmd.Flags().IntVar(&mr, "id", 0, "Merge request IID")
	mrCmd.Flags().StringVar(&rawURL, "url", "", "GitLab merge request URL")
	mrCmd.Flags().BoolVar(&publish, "publish", false, "Post the review back to the GitLab MR as comments (summary + one per finding)")
	cmd.AddCommand(mrCmd)
	cmd.AddCommand(a.newGitLabServeCmd())
	return cmd
}

func (a *app) newGitLabServeCmd() *cobra.Command {
	var serveConfigPath string
	var listen string
	var reviewConcurrency int
	var logDir string
	var shutdownGrace time.Duration
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Run a webhook daemon reviewing GitLab MRs automatically",
		Long: "Run an HTTP daemon receiving GitLab group webhooks (merge request + emoji + comment events). " +
			"MR activity triggers reviews on projects carrying the opt-in topic; awarding the trigger " +
			"emoji on an MR requests a review explicitly, revoking it aborts. MR comments starting with " +
			"the command keyword control the daemon: /nickpit review|abort|status|help. Each review runs " +
			"as a separate nickpit child process.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadServe(serveConfigPath)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("listen") {
				cfg.Listen = listen
			}
			if cmd.Flags().Changed("review-concurrency") {
				cfg.ReviewConcurrency = reviewConcurrency
			}
			if cmd.Flags().Changed("serve-log-dir") {
				cfg.LogDir = logDir
			}
			if cmd.Flags().Changed("shutdown-grace") {
				cfg.ShutdownGrace = shutdownGrace.String()
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("serve config: %w", err)
			}
			baseURL := cfg.GitLabBaseURL
			if a.gitlabBaseURL != "" {
				baseURL = a.gitlabBaseURL
			}
			baseURL = glscm.NormalizeBaseURL(baseURL)

			logLevel := slog.LevelInfo
			if a.verbose {
				logLevel = slog.LevelDebug
			}
			var logHandler slog.Handler
			if a.jsonOutput {
				logHandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
			} else {
				logHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
			}
			log := slog.New(logHandler)

			groups, warnings := serve.NewGroupSet(cmd.Context(), cfg.Groups, baseURL, func(ctx context.Context, client *glscm.Client) (int, error) {
				user, err := client.CurrentUser(ctx)
				if err != nil {
					return 0, err
				}
				return user.ID, nil
			})
			for _, warning := range warnings {
				log.Warn("bot user lookup failed; own emoji awards are filtered by name only (start_emoji never equals trigger_emoji, enforced at config validation) and own note replies are only kept out of command parsing by never starting with the command keyword", "error", warning)
			}

			// Group tokens and webhook secrets typically sit in the daemon's
			// environment (server.yaml ${VAR} references); review children
			// must only receive their own group's injected token.
			scrub := make([]string, 0, len(cfg.Groups)*2)
			for _, group := range cfg.Groups {
				scrub = append(scrub, group.Token, group.WebhookSecret)
			}
			runner, err := serve.NewExecRunner(scrub)
			if err != nil {
				return err
			}
			dispatcher := serve.NewDispatcher(runner, serve.GitLabTopicLookup, serve.WorkerConfig{
				Topic:      cfg.Topic,
				StartEmoji: cfg.StartEmojiName(),
				BaseURL:    baseURL,
				ConfigPath: a.configPath,
				ExtraArgs:  cfg.Review.ExtraArgs,
				LogDir:     cfg.LogDir,
			}, log)
			handler := serve.NewHandler(groups, dispatcher, serve.HandlerConfig{
				TriggerEmoji:   cfg.TriggerEmoji,
				CommandKeyword: cfg.CommandKeyword,
				AckEmoji:       cfg.AckEmojiName(),
			}, log)
			server := serve.NewServer(cfg.Listen, handler, dispatcher, cfg.ShutdownGraceDuration(), log)
			return server.Run(cmd.Context(), cfg.ReviewConcurrency)
		},
	}
	serveCmd.Flags().StringVar(&serveConfigPath, "serve-config", config.DefaultServeConfigPath, "Serve daemon config file (groups, tokens, webhook secrets)")
	serveCmd.Flags().StringVar(&listen, "listen", config.DefaultServeListen, "HTTP listen address")
	serveCmd.Flags().IntVar(&reviewConcurrency, "review-concurrency", config.DefaultServeReviewConcurrency, "Maximum parallel review child processes")
	serveCmd.Flags().StringVar(&logDir, "serve-log-dir", config.DefaultServeLogDir, "Directory for per-review child process logs")
	serveCmd.Flags().DurationVar(&shutdownGrace, "shutdown-grace", 10*time.Minute, "How long running reviews may finish after SIGTERM before being terminated (an interrupted publish heals on the next run via comment fingerprints)")
	return serveCmd
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

	var findLinesPath, findLinesCode string
	findLinesCmd := &cobra.Command{
		Use:   "find-lines",
		Short: "Find line numbers for exact code text",
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			result, err := engine.FindLines(cmd.Context(), repoRoot, findLinesPath, findLinesCode)
			if err != nil {
				return err
			}
			return a.writeInspectOutput(result)
		},
	}
	findLinesCmd.Flags().StringVar(&findLinesPath, "path", "", "Relative file or folder path; leave empty to search the whole repo")
	findLinesCmd.Flags().StringVar(&findLinesCode, "code", "", "Exact line or contiguous block of code to locate")
	_ = findLinesCmd.MarkFlagRequired("code")

	cmd.AddCommand(fileCmd, listFilesCmd, searchCmd, findLinesCmd, callersCmd, calleesCmd)
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
			profileName, profile, err := a.loadProfileForSpec()
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
				MaxContextTokens:          profile.MaxContextTokens,
				MaxToolCalls:              profile.MaxToolCalls,
				MaxDuplicateToolCalls:     profile.MaxDuplicateToolCalls,
				MaxOutputRetries:          profile.MaxOutputRetries,
				MaxReasoningSeconds:       profile.MaxReasoningSeconds,
				DiffFormat:                profile.DiffFormat,
				DisableJSONResponseFormat: profile.DisableJSONResponseFormat,
			}
			ctx := logging.WithProgressInfo(cmd.Context(), profileProgressInfo(profile))
			a.logProgress(ctx, logging.StageModel, logging.StateReady, modelSummary(profile, checkReq))
			a.logSmallModelReady(ctx, profile, checkReq)
			a.logProgress(ctx, logging.StageAgent, logging.StateNone, agentSummary(profile, checkReq))
			client := llm.NewOpenAIClient(profile.BaseURL, profile.APIKey, profile.Model)
			client.SetLogger(logger)
			client.SetMaxRateLimitDelay(time.Duration(profile.MaxRateLimitDelaySeconds) * time.Second)
			result, err := a.resolveModelCapabilities(ctx, client, profile, profile.Model, profile.ReasoningEffort, "", a.refreshModelCheck)
			if err != nil {
				return err
			}
			a.logProgress(ctx, logging.StageModelCheck, logging.StateDone, modelCheckSummary(result))
			spec, err := a.resolveActiveSpec()
			if err != nil {
				return err
			}
			smallRequirements := smallModelRequirementsForSpec(spec, checkReq)
			smallResult, smallChecked, err := a.checkSmallModel(ctx, client, profile, a.refreshModelCheck)
			if err != nil {
				return err
			}
			// Mirror runReview's run-wide json_schema→prompt fallback so a model
			// usable only via the prompt-embedded schema reports healthy here too,
			// instead of exiting non-zero while real reviews of the same model
			// succeed. The API response format is global: either model lacking
			// json_schema degrades both, so validation then requires only plain JSON
			// output.
			if !result.DisableJSONResponseFormat && a.jsonSchemaFallbackRequired(ctx, result, "model") {
				result.DisableJSONResponseFormat = true
			}
			if smallChecked && smallRequirements.JSONSchema && !result.DisableJSONResponseFormat &&
				a.jsonSchemaFallbackRequired(ctx, smallResult, "small model") {
				result.DisableJSONResponseFormat = true
			}
			if result.DisableJSONResponseFormat {
				checkReq.DisableJSONResponseFormat = true
				smallResult.DisableJSONResponseFormat = true
				smallRequirements = smallModelRequirementsForSpec(spec, checkReq)
			}
			if a.jsonOutput {
				out := struct {
					Check modelcheck.CheckSummary  `json:"check"`
					Small *modelcheck.CheckSummary `json:"small,omitempty"`
				}{Check: result.Summary()}
				if smallChecked {
					summary := smallResult.Summary()
					out.Small = &summary
				}
				if err := writeJSON(out); err != nil {
					return err
				}
				if err := validateModelCheckRequirements(result, primaryModelRequirements(spec, checkReq)); err != nil {
					return err
				}
				if smallChecked {
					return validateSmallModelCheck(smallResult, smallRequirements)
				}
				return nil
			}
			if err := a.writeModelCheckOutput(profile.Model, result); err != nil {
				return err
			}
			if smallChecked {
				if err := a.writeModelCheckOutput(config.EffectiveSmallProfile(profile).Model, smallResult); err != nil {
					return err
				}
			}
			if err := validateModelCheckRequirements(result, primaryModelRequirements(spec, checkReq)); err != nil {
				return err
			}
			if smallChecked {
				return validateSmallModelCheck(smallResult, smallRequirements)
			}
			return nil
		},
	}
	modelCmd.Flags().BoolVar(&a.refreshModelCheck, "refresh", false, "Refresh stored model capabilities by running live probes")
	cmd.AddCommand(modelCmd)
	return cmd
}

func (a *app) runReview(ctx context.Context, source model.ReviewSource, retrievalEngine retrieval.Engine, profileName string, profile config.Profile, req model.ReviewRequest) error {
	a.reviewStart = time.Now()
	logger := logging.New(os.Stderr, a.verbose, isTerminal(os.Stderr))
	logger.SetShowReasoning(a.showReasoning)
	logger.SetShowProgress(a.showProgress)
	a.logger = logger
	ctx = logging.WithProgressInfo(ctx, profileProgressInfo(profile))

	// Resolve the workflow spec. Every review runs through a workflow: an
	// explicit --spec/--step, or the embedded DefaultSpec otherwise. There is no
	// separate "default" code path. The profile was already applied by the caller
	// via loadProfileForSpec, before the source adapter and request were built.
	spec, err := a.resolveActiveSpec()
	if err != nil {
		return err
	}

	// Resolve additional styleguides (files/URLs) before the credential gate,
	// model check, and checkout: a broken spec should fail immediately, not
	// after expensive setup. profile.Workdir anchors relative paths because
	// only the --workdir flag chdirs; a workdir from the profile or
	// NICKPIT_WORKDIR must still resolve them correctly.
	additionalGuides, err := styleguide.Resolve(ctx, profile.StyleGuides, profile.Workdir)
	if err != nil {
		return err
	}

	// Source-less workflows (e.g. --step merge / --step finalize on imported
	// findings) operate on injected findings and may make no LLM call at all — a
	// single-input merge is a passthrough, finalize on empty findings is a no-op.
	// Defer the hard credential requirement for them: the client is still built,
	// so any LLM call that does happen fails at call time if credentials are
	// missing, exactly as --disable-model-check behaves. The model probe itself
	// still runs whenever credentials are present (see below), so the json_schema
	// fallback applies to the LLM calls these specs do make.
	needsSource := spec.NeedsSource()

	if needsSource && profile.APIKey == "" {
		if profile.APIKeyConfigured {
			return fmt.Errorf("profile %q has an empty api_key value; %s", profileName, missingAPIKeyHint(profileName, true))
		}
		return fmt.Errorf("missing LLM API key for profile %q; %s", profileName, missingAPIKeyHint(profileName, false))
	}

	req.DisableParallelToolCalls = a.disableParallelToolCalls
	req.DisableReasoningExtract = a.disableReasoningExtract
	req.Concurrency = a.concurrency
	req.VerifyDropPolicy = a.verifyDropPolicy
	req.ConfidenceThreshold = a.confidenceThreshold
	if req.DiffFormat == "" {
		req.DiffFormat = profile.DiffFormat
	}
	if profile.DisablePatchSummary {
		req.DisablePatchSummary = true
	}
	if profile.DisableSuggestions {
		req.DisableSuggestions = true
	}
	if profile.DisableWorkflowTimeBudget {
		req.DisableWorkflowTimeBudget = true
	}
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
	a.logProgress(ctx, logging.StageModel, logging.StateReady, modelSummary(profile, req))
	a.logSmallModelReady(ctx, profile, req)
	agentCtx := logging.WithProgressInfo(ctx, workflowProgressInfo(spec, a.specPath, a.stepName))
	a.logProgress(agentCtx, logging.StageAgent, logging.StateNone, agentSummary(profile, req))

	client := llm.NewOpenAIClient(profile.BaseURL, profile.APIKey, profile.Model)
	client.SetLogger(logger)
	client.SetMaxRateLimitDelay(time.Duration(profile.MaxRateLimitDelaySeconds) * time.Second)
	if !a.disableModelCheck && profile.APIKey != "" {
		checkResult, err := a.resolveModelCapabilities(ctx, client, profile, profile.Model, profile.ReasoningEffort, "", false)
		if err != nil {
			return err
		}
		// JSON response format is on by default. If a model cannot do API-enforced
		// json_schema, degrade the whole run to the prompt-embedded schema rather
		// than failing: the API response format is a single global request setting,
		// so a primary OR small model lacking json_schema disables it for both.
		// Resolve the fallback fully BEFORE validating, so each model is checked
		// against exactly the JSON mode its steps will use. This runs for
		// source-less specs too, so imported-findings merge/finalize/verify/summarize
		// degrade the same way a normal review would.
		if !req.DisableJSONResponseFormat && a.jsonSchemaFallbackRequired(ctx, checkResult, "model") {
			req.DisableJSONResponseFormat = true
		}
		req.ModelEmitsReasoning = checkResult.Summary().Reasoning.Traces
		a.logProgress(ctx, logging.StageModelCheck, logging.StateDone, modelCheckSummary(checkResult))

		smallRequirements := smallModelRequirementsForSpec(spec, req)
		smallResult, smallChecked := modelcheck.Result{}, false
		if smallRequirements.Uses() {
			smallResult, smallChecked, err = a.checkSmallModel(ctx, client, profile, false)
			if err != nil {
				return err
			}
			if smallChecked && smallRequirements.JSONSchema && !req.DisableJSONResponseFormat &&
				a.jsonSchemaFallbackRequired(ctx, smallResult, "small model") {
				req.DisableJSONResponseFormat = true
			}
		}

		// req.DisableJSONResponseFormat is now final for the run. Validate each
		// model against exactly what its steps will ask for — including any
		// primary-model step that turned response_format off (it needs plain JSON
		// output, not json_schema). The hard gate only applies when a source
		// guarantees LLM calls; a source-less spec may be a passthrough, so
		// (matching the deferred-credential design) it is not blocked up front and
		// any LLM call it makes fails at call time if the model is unusable.
		checkResult.DisableJSONResponseFormat = req.DisableJSONResponseFormat
		if needsSource {
			if err := validateModelCheckRequirements(checkResult, primaryModelRequirements(spec, req)); err != nil {
				return err
			}
			if smallChecked {
				if err := validateSmallModelCheck(smallResult, smallModelRequirementsForSpec(spec, req)); err != nil {
					return err
				}
			}
			// Restrict reasoning efforts to the probed set only for source-backed
			// runs; source-less runs keep the unrestricted default so a probe that
			// could not run (e.g. offline) never narrows them to an empty set.
			client.SetAllowedReasoningEfforts(checkResult.PassedEfforts)
		}
	} else {
		req.ModelEmitsReasoning = true
		a.logProgress(ctx, logging.StageModelCheck, logging.StateSkip, "")
	}

	engine := review.NewEngine(source, client, retrievalEngine, profile)
	engine.SetLogger(logger)
	engine.SetSearchToolOptimization(!a.disableSearchToolOptimization)
	engine.SetAdditionalStyleGuides(additionalGuides)
	engine.SetDisabledStyleGuides(profile.DisableStyleGuides)

	// Single path: the embedded DefaultSpec and any user-supplied spec run
	// identically. The pipeline may skip source/repo resolution (e.g.
	// merge/finalize from a findings file) and runs finalize/summarize as
	// in-pipeline steps rather than as a separate post-processing pass.
	return a.runWorkflow(ctx, engine, source, spec, profile, req)
}

// emitResult formats the result to stdout, optionally publishes it back to the
// origin, and reports the "all reviewers errored" CI failure.
func (a *app) emitResult(ctx context.Context, source model.ReviewSource, req model.ReviewRequest, result *model.ReviewResult) error {
	if req.DisableSuggestions {
		result.StripSuggestions()
	}
	var formatter output.Formatter
	if a.jsonOutput {
		formatter = output.NewJSONFormatter(os.Stdout)
	} else {
		formatter = output.NewTerminalFormatter(os.Stdout, isTerminal(os.Stdout))
	}
	if err := formatter.FormatFindings(result); err != nil {
		return err
	}
	// Publish the review back to the origin (GitHub PR or GitLab MR) when
	// requested. The review already succeeded and printed to stdout, so a
	// publish failure is a warning, never a hard error. Only sources
	// implementing ReviewPublisher (GitHub, GitLab) act here; local reviews
	// are unaffected.
	if req.PostReview && (len(result.Findings) > 0 || strings.TrimSpace(result.OverallExplanation) != "") {
		if publisher, ok := source.(model.ReviewPublisher); ok {
			// GitLab numbers MRs with "!", GitHub PRs with "#".
			sigil := "!"
			if req.Mode == model.ModeGitHub {
				sigil = "#"
			}
			a.logProgress(ctx, logging.StagePublish, logging.StateStart, fmt.Sprintf("posting to %s %s%d", req.Repo, sigil, req.Identifier))
			if err := publisher.PublishReview(ctx, req, result); err != nil {
				a.logProgress(ctx, logging.StagePublish, logging.StateError, fmt.Sprintf("error=%v", err))
				result.Warnings = append(result.Warnings, fmt.Sprintf("Publish failed: %v", err))
			} else {
				a.logProgress(ctx, logging.StagePublish, logging.StateDone, "")
			}
		} else {
			a.logProgress(ctx, logging.StagePublish, logging.StateSkip, "source does not support publishing")
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

// runWorkflow executes a spec through the pipeline: the embedded DefaultSpec for
// an ordinary review, or a user-supplied --spec/--step. Source/repo resolution
// is skipped when no step needs a source (e.g. a merge/finalize-from-file run).
// Finalize/summarize, when present in the spec, run inside the pipeline.
func (a *app) runWorkflow(ctx context.Context, engine *review.Engine, source model.ReviewSource, spec workflow.Spec, profile config.Profile, req model.ReviewRequest) error {
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		return err
	}
	confidenceWarning := ""
	if req.ConfidenceThreshold > 0 && !specHasStep(spec, workflow.StepVerdict) {
		confidenceWarning = fmt.Sprintf("confidence threshold %.2f is configured but workflow has no verdict step; threshold will not be applied", req.ConfidenceThreshold)
		a.logProgress(ctx, logging.StageVerdict, logging.StateWarn, confidenceWarning)
	}
	if pipeline.NeedsSource() {
		repoRoot, cleanup, err := a.resolveRepoRoot(ctx, source, profile, req)
		if err != nil {
			return err
		}
		if cleanup != nil {
			defer cleanup()
		}
		req.RepoRoot = repoRoot
	} else if cwd, err := os.Getwd(); err == nil {
		req.RepoRoot = cwd
	}

	result, _, err := engine.RunSpecPipeline(ctx, pipeline, req)
	if errors.Is(err, llm.ErrInvalidJSON) {
		a.logProgress(ctx, logging.StageResult, logging.StateError, fmt.Sprintf("invalid_json error=%v", err))
		return err
	}
	if err != nil {
		a.logProgress(ctx, logging.StageResult, logging.StateError, fmt.Sprintf("error=%v", err))
		return err
	}
	if result == nil {
		return fmt.Errorf("review: engine returned no result")
	}
	if confidenceWarning != "" {
		result.Warnings = append(result.Warnings, confidenceWarning)
	}
	if !a.reviewStart.IsZero() {
		result.RuntimeSeconds = model.RuntimeSeconds(time.Since(a.reviewStart))
	}
	a.logProgress(ctx, logging.StageResult, logging.StateOK, reviewResultSummary(result))
	return a.emitResult(ctx, source, req, result)
}

func specHasStep(spec workflow.Spec, stepType string) bool {
	for _, entry := range spec.FlatSteps() {
		if entry.Type == stepType {
			return true
		}
	}
	return false
}

// resolveSpec builds the workflow spec from --spec or --step, applying any
// top-level --findings injection.
func (a *app) resolveSpec() (workflow.Spec, error) {
	if a.specPath != "" && a.stepName != "" {
		return workflow.Spec{}, fmt.Errorf("--spec and --step are mutually exclusive")
	}
	if a.stepName != "" {
		return workflow.SingleStepSpec(a.stepName, a.findingsFiles), nil
	}
	spec, err := workflow.Load(a.specPath)
	if err != nil {
		return workflow.Spec{}, err
	}
	if err := seedFindings(&spec, a.findingsFiles); err != nil {
		return workflow.Spec{}, err
	}
	return spec, nil
}

func (a *app) resolveActiveSpec() (workflow.Spec, error) {
	if a.specPath != "" || a.stepName != "" {
		return a.resolveSpec()
	}
	return workflow.DefaultSpec(), nil
}

// seedFindings attaches top-level --findings to the first step that actually
// consumes findings (verify/dedupe/merge/finalize/verdict/summarize) and does
// not already declare findings_from, so `--spec w.yaml --findings f.json` lands
// where it is read instead of being silently dropped on a collect-context/review
// step. Returns an error when findings are supplied but no step would consume them.
func seedFindings(spec *workflow.Spec, findings []string) error {
	if len(findings) == 0 {
		return nil
	}
	for _, entry := range spec.FlatSteps() {
		if len(entry.FindingsFrom) == 0 && workflow.StepConsumesFindings(entry.Type) {
			entry.FindingsFrom = findings
			return nil
		}
	}
	return fmt.Errorf("--findings given but the spec has no verify/dedupe/merge/finalize/verdict/summarize step to consume them; add findings_from to a specific step")
}

// loadProfileNamed loads a profile by name (e.g. a spec's `profile:` field),
// applying the same CLI overrides as loadProfile.
func (a *app) loadProfileNamed(name string) (string, config.Profile, error) {
	savedProfile, savedSet := a.profile, a.profileSet
	a.profile, a.profileSet = name, true
	defer func() { a.profile, a.profileSet = savedProfile, savedSet }()
	return a.loadProfile()
}

// loadProfileForSpec loads the effective profile for a review command, honoring
// a workflow spec's `profile:` field. It must be used instead of loadProfile by
// commands that build a review source and request, so a spec-selected profile
// drives the SCM adapter (token/base URL), the request budgets, and the workdir
// — not just the LLM client. CLI flags still override the spec profile.
func (a *app) loadProfileForSpec() (string, config.Profile, error) {
	name, profile, err := a.loadProfile()
	if err != nil {
		return "", config.Profile{}, err
	}
	specProfile, err := a.specProfile()
	if err != nil {
		return "", config.Profile{}, err
	}
	if specProfile != "" && specProfile != name {
		return a.loadProfileNamed(specProfile)
	}
	return name, profile, nil
}

// specProfile returns the `profile:` declared by a --spec file, or "" when no
// spec is given, the spec declares none, or a single --step is used.
func (a *app) specProfile() (string, error) {
	if a.specPath == "" || a.stepName != "" {
		return "", nil
	}
	spec, err := workflow.Load(a.specPath)
	if err != nil {
		return "", err
	}
	return spec.Profile, nil
}

func (a *app) resolveModelCapabilities(ctx context.Context, client llm.Client, profile config.Profile, model, effort, alias string, refresh bool) (modelcheck.Result, error) {
	settings := requestSettingsFingerprint(profile)
	if !refresh {
		if capability, ok := modelcheck.FindProfileCapabilityFor(profile, model); ok {
			result := modelcheck.ResultFromCapability(capability, profile.DisableJSONResponseFormat)
			a.logProgress(ctx, logging.StageModelCheck, logging.StateOK, "source=profile")
			return result, nil
		}
		cachePath, err := modelcheck.DefaultCachePath()
		if err != nil {
			a.logf(ctx, "Model capability cache unavailable: %v", err)
		} else {
			capability, ok, err := modelcheck.ReadCachedCapability(cachePath, profile.BaseURL, model, settings)
			if err == nil && ok {
				result := modelcheck.ResultFromCapability(capability, profile.DisableJSONResponseFormat)
				a.logProgress(ctx, logging.StageModelCheck, logging.StateOK, "source=cache")
				return result, nil
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				a.logf(ctx, "Model capability cache ignored: %v", err)
			}
		}
	}

	checker := modelcheck.NewForModel(client, profile, model, effort)
	checker.SetLogger(a.logger)
	checker.SetParallel(!a.disableParallelToolCalls)
	// alias ("@small" for the small model, empty for the primary) is shown before
	// the model in probe progress lines so they name which configured model is
	// being checked.
	checker.SetModelAlias(alias)
	result := checker.Run(ctx)
	if !cacheableModelResult(result) {
		return result, nil
	}
	capability := modelcheck.CapabilityFromResult(result)
	cachePath, err := modelcheck.DefaultCachePath()
	if err != nil {
		a.logf(ctx, "Model capability cache unavailable: %v", err)
		return result, nil
	}
	if err := modelcheck.WriteCachedCapability(cachePath, profile.BaseURL, settings, capability, time.Now()); err != nil {
		a.logf(ctx, "Model capability cache write failed: %v", err)
	}
	return result, nil
}

// smallModelConfigured reports whether @small has distinct request settings and
// therefore warrants its own capability check.
func smallModelConfigured(profile config.Profile) bool {
	return !reflect.DeepEqual(modelCheckProfileSignature(profile), modelCheckProfileSignature(config.EffectiveSmallProfile(profile)))
}

type modelCheckProfile struct {
	Model           string
	MaxTokens       *int
	Temperature     *float64
	TopP            *float64
	TopK            *int
	PresencePenalty *float64
	ExtraBody       map[string]any
	ReasoningEffort string
}

func modelCheckProfileSignature(profile config.Profile) modelCheckProfile {
	return modelCheckProfile{
		Model:           strings.TrimSpace(profile.Model),
		MaxTokens:       profile.MaxTokens,
		Temperature:     profile.Temperature,
		TopP:            profile.TopP,
		TopK:            profile.TopK,
		PresencePenalty: profile.PresencePenalty,
		ExtraBody:       profile.ExtraBody,
		ReasoningEffort: profile.ReasoningEffort,
	}
}

// requestSettingsFingerprint hashes the request settings the model probe runs
// with, so the capability cache keys on the settings combination (a changed
// temperature/top_*/presence_penalty/extra_body/max_tokens/reasoning_effort
// re-probes instead of returning a stale hit). It reuses modelCheckProfile so
// the cache identity and the small-model-distinct check share one field set.
// encoding/json sorts map keys, so extra_body ordering does not affect the hash.
func requestSettingsFingerprint(profile config.Profile) string {
	data, err := json.Marshal(modelCheckProfileSignature(profile))
	if err != nil {
		// modelCheckProfile holds only JSON-encodable fields, so this is
		// unreachable; fall back to an empty fingerprint rather than panicking.
		return ""
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

type modelCapabilityRequirements struct {
	Response   bool
	Tools      bool
	JSONOutput bool
	JSONSchema bool
}

func (r modelCapabilityRequirements) Uses() bool {
	return r.Response || r.Tools || r.JSONOutput || r.JSONSchema
}

func (r *modelCapabilityRequirements) merge(other modelCapabilityRequirements) {
	r.Response = r.Response || other.Response
	r.Tools = r.Tools || other.Tools
	r.JSONOutput = r.JSONOutput || other.JSONOutput
	r.JSONSchema = r.JSONSchema || other.JSONSchema
}

func (r *modelCapabilityRequirements) requireJSON(useSchema bool) {
	r.Response = true
	if useSchema {
		r.JSONSchema = true
	} else {
		r.JSONOutput = true
	}
}

func smallModelRequirementsForSpec(spec workflow.Spec, req model.ReviewRequest) modelCapabilityRequirements {
	return modelRequirementsForSpec(spec, req, true)
}

// primaryModelRequirements is the capability set the spec demands of the primary
// model: the baseline tool use, reasoning, and run-level JSON mode a review
// needs, plus every step (and review-internal agent) left on the primary model.
// A step that turns response_format off via disable_json_response_format requires
// plain JSON output instead of json_schema, so a primary that passes the schema
// probe but fails the plain-JSON probe is rejected up front rather than failing
// inside that prompt-only step — mirroring how the small model is validated.
func primaryModelRequirements(spec workflow.Spec, req model.ReviewRequest) modelCapabilityRequirements {
	requirements := modelCapabilityRequirements{Response: true, Tools: true}
	requirements.requireJSON(!req.DisableJSONResponseFormat)
	requirements.merge(modelRequirementsForSpec(spec, req, false))
	return requirements
}

// modelRequirementsForSpec accumulates the capability requirements the spec
// places on one model. useSmallModel selects which: true gathers the steps (and
// review-internal agents) routed to @small, false gathers those left on the
// primary model. Per-step disable_json_response_format is honored — and, like
// the runtime override resolution, treated monotonically (it may disable but not
// re-enable a run that already disabled response_format) — so a prompt-only step
// requires plain JSON output while a default step requires json_schema.
func modelRequirementsForSpec(spec workflow.Spec, req model.ReviewRequest, useSmallModel bool) modelCapabilityRequirements {
	var requirements modelCapabilityRequirements
	for _, entry := range spec.FlatSteps() {
		stepReq := req
		stepUsesSmall := entry.Config != nil && usesSmallAlias(entry.Config.Model)
		if entry.Config != nil && entry.Config.DisableJSONResponseFormat != nil {
			stepReq.DisableJSONResponseFormat = stepReq.DisableJSONResponseFormat || *entry.Config.DisableJSONResponseFormat
		}
		if stepUsesSmall == useSmallModel {
			requirements.merge(stepModelRequirements(entry.Type, stepReq.DisableJSONResponseFormat))
		}
		if !strings.HasPrefix(entry.Type, workflow.StepReviewPrefix) || entry.Config == nil {
			continue
		}
		if agentUsesSmall(stepUsesSmall, entry.Config.MineReasoning) == useSmallModel {
			requirements.merge(textModelRequirements())
		}
		if agentUsesSmall(stepUsesSmall, entry.Config.CompileFindings) == useSmallModel {
			requirements.merge(textModelRequirements())
		}
		if agentUsesSmall(stepUsesSmall, entry.Config.Nudge) == useSmallModel {
			requirements.merge(reviewerModelRequirements(agentDisableJSONResponseFormat(stepReq, entry.Config.Nudge)))
		}
	}
	return requirements
}

func usesSmallAlias(model *string) bool {
	return model != nil && strings.TrimSpace(*model) == workflow.SmallModelAlias
}

func agentModel(override *workflow.AgentOverride) *string {
	if override == nil {
		return nil
	}
	return override.Model
}

func agentUsesSmall(stepUsesSmall bool, override *workflow.AgentOverride) bool {
	if override == nil || override.Model == nil {
		return stepUsesSmall
	}
	return usesSmallAlias(agentModel(override))
}

func textModelRequirements() modelCapabilityRequirements {
	return modelCapabilityRequirements{Response: true}
}

func reviewerModelRequirements(disableJSONResponseFormat bool) modelCapabilityRequirements {
	requirements := modelCapabilityRequirements{Response: true, Tools: true}
	requirements.requireJSON(!disableJSONResponseFormat)
	return requirements
}

func stepModelRequirements(stepType string, disableJSONResponseFormat bool) modelCapabilityRequirements {
	switch {
	case stepType == workflow.StepCollectContext:
		return modelCapabilityRequirements{Response: true, Tools: true}
	case strings.HasPrefix(stepType, workflow.StepReviewPrefix),
		stepType == workflow.StepVerify,
		strings.HasPrefix(stepType, workflow.StepVerifyPrefix),
		strings.HasPrefix(stepType, workflow.StepNudgePrefix):
		return reviewerModelRequirements(disableJSONResponseFormat)
	case stepType == workflow.StepDedupe,
		strings.HasPrefix(stepType, workflow.StepDedupePrefix),
		stepType == workflow.StepMerge,
		stepType == workflow.StepFinalize,
		stepType == workflow.StepVerdict,
		stepType == workflow.StepSummarize:
		requirements := modelCapabilityRequirements{}
		requirements.requireJSON(!disableJSONResponseFormat)
		return requirements
	case strings.HasPrefix(stepType, workflow.StepExtractPrefix):
		return textModelRequirements()
	default:
		return textModelRequirements()
	}
}

func agentDisableJSONResponseFormat(stepReq model.ReviewRequest, override *workflow.AgentOverride) bool {
	// Monotonic, matching workflow.AgentOverride.Resolve: an override may disable
	// response_format but never re-enable it once the run disabled it, so the
	// requirement computation predicts the JSON mode the agent will actually use.
	if override != nil && override.DisableJSONResponseFormat != nil {
		return stepReq.DisableJSONResponseFormat || *override.DisableJSONResponseFormat
	}
	return stepReq.DisableJSONResponseFormat
}

// checkSmallModel runs (or loads cached) capabilities for the effective small
// profile when it differs from the primary model request config, mirroring the
// primary check. The bool reports whether a check was actually performed.
func (a *app) checkSmallModel(ctx context.Context, client llm.Client, profile config.Profile, refresh bool) (modelcheck.Result, bool, error) {
	if !smallModelConfigured(profile) {
		return modelcheck.Result{}, false, nil
	}
	smallProfile := config.EffectiveSmallProfile(profile)
	smallCtx := logging.WithProgressInfo(ctx, smallModelProgressInfo(smallProfile))
	result, err := a.resolveModelCapabilities(smallCtx, client, smallProfile, smallProfile.Model, smallProfile.ReasoningEffort, workflow.SmallModelAlias, refresh)
	if err != nil {
		return result, true, err
	}
	a.logProgress(smallCtx, logging.StageModelCheck, logging.StateDone, modelCheckSummary(result))
	return result, true, nil
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
		if run.Role != "review" {
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
		a.logf(ctx, "Resolved repo root: source=provided path=%s", req.RepoRoot)
		return req.RepoRoot, nil, nil
	}
	if req.Mode == model.ModeLocal {
		wd, err := os.Getwd()
		if err == nil {
			a.logf(ctx, "Resolved repo root: source=working_dir path=%s", wd)
		}
		return wd, nil, err
	}
	maxToolCalls := req.MaxToolCalls
	if maxToolCalls == 0 {
		maxToolCalls = profile.MaxToolCalls
	}
	if !req.IncludeFullFiles && !hasContentFilters(req) && maxToolCalls < 0 {
		a.logf(ctx, "Skipping remote checkout: include_full_files=%t content_filters=%t max_tool_calls=%d", req.IncludeFullFiles, hasContentFilters(req), maxToolCalls)
		return "", nil, nil
	}
	remote, ok := source.(model.RemoteCheckoutSource)
	if !ok {
		wd, err := os.Getwd()
		if err == nil {
			a.logf(ctx, "Skipping remote checkout: reason=no_support fallback=%s", wd)
		}
		return wd, nil, err
	}
	spec, err := remote.ResolveCheckout(ctx, req)
	if err != nil {
		return "", nil, err
	}
	a.logf(ctx, "Preparing remote checkout: provider=%s repo=%s head_ref=%s head_sha=%s", spec.Provider, spec.Repo, spec.HeadRef, spec.HeadSHA)
	manager := git.NewCheckoutManager()
	repoRoot, cleanup, err := manager.Prepare(ctx, *spec, git.CheckoutOptions{
		Workdir: req.Workdir,
		Token:   checkoutToken(req.Mode, profile),
	})
	if err != nil {
		return "", nil, err
	}
	a.logf(ctx, "Prepared repo root: path=%s", repoRoot)
	return repoRoot, cleanup, nil
}

func hasContentFilters(req model.ReviewRequest) bool {
	return len(req.IncludeContent) > 0 || len(req.ExcludeContent) > 0
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

func (a *app) writeModelCheckOutput(modelName string, result modelcheck.Result) error {
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
	fmt.Fprintf(&sb, "%s\n", label(modelName))
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

// validateSmallModelCheck applies only the gates required by the workflow uses
// of @small, then labels failures so the user can tell which model is
// incompatible.
func validateSmallModelCheck(result modelcheck.Result, requirements modelCapabilityRequirements) error {
	if err := validateModelCheckRequirements(result, requirements); err != nil {
		return fmt.Errorf("small model: %w", err)
	}
	return nil
}

func validatePreReviewModelCheck(result modelcheck.Result) error {
	requirements := modelCapabilityRequirements{Response: true, Tools: true}
	requirements.requireJSON(!result.DisableJSONResponseFormat)
	return validateModelCheckRequirements(result, requirements)
}

// jsonSchemaFallbackRequired reports whether a probed model cannot do
// API-enforced json_schema, logging the prompt-embedded fallback notice when it
// cannot. The API response format is a single global request setting, so the
// caller disables it for the whole run (both primary and small models). label
// names the model in the warning ("model" or "small model"). Shared by runReview
// and `check model` so the two paths cannot drift apart on the degrade rule.
func (a *app) jsonSchemaFallbackRequired(ctx context.Context, result modelcheck.Result, label string) bool {
	probe := result.ConfiguredJSONSchema()
	if probe.Status == modelcheck.StatusOK {
		return false
	}
	a.logProgress(ctx, logging.StageModelCheck, logging.StateWarn, fmt.Sprintf("%s lacks json_schema response format (%s); falling back to prompt-embedded schema", label, probe.Status))
	return true
}

// cacheableModelResult reports whether a freshly probed result is worth
// persisting to the capability cache. A model is cacheable when it is usable by
// either structured-output route: API-enforced json_schema, or — since runReview
// degrades a model that lacks json_schema to the prompt-embedded schema — plain
// JSON output. Without the degraded check, a json_schema-incapable but otherwise
// fine model failed the strict gate, was never cached, and re-probed (and
// re-warned) on every run. A model that also fails tools/reasoning or emits no
// JSON at all stays uncached so a later run can re-probe a fixed endpoint.
func cacheableModelResult(result modelcheck.Result) bool {
	if validatePreReviewModelCheck(result) == nil {
		return true
	}
	degraded := result
	degraded.DisableJSONResponseFormat = true
	return validatePreReviewModelCheck(degraded) == nil
}

func validateModelCheckRequirements(result modelcheck.Result, requirements modelCapabilityRequirements) error {
	if !requirements.Uses() {
		return nil
	}
	if len(result.PassedEfforts) == 0 {
		return fmt.Errorf("model check failed: no reasoning efforts passed")
	}
	if requirements.Tools {
		if probe := result.ConfiguredTools(); probe.Status != modelcheck.StatusOK {
			return fmt.Errorf("model check failed for tool use at reasoning effort %q: status=%s error=%s", probe.ReasoningEffort, probe.Status, probe.Error)
		}
	}
	if requirements.JSONSchema {
		if probe := result.ConfiguredJSONSchema(); probe.Status != modelcheck.StatusOK {
			return fmt.Errorf("model check failed for JSON schema output at reasoning effort %q: status=%s error=%s", probe.ReasoningEffort, probe.Status, probe.Error)
		}
	}
	if requirements.JSONOutput {
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
	case *retrieval.FindLinesResult:
		if _, err := fmt.Fprintf(os.Stdout, "%d match(es)\n", typed.MatchCount); err != nil {
			return err
		}
		for i, match := range typed.Matches {
			if i > 0 {
				if _, err := fmt.Fprintln(os.Stdout); err != nil {
					return err
				}
			}
			loc := match.CodeLocation
			if _, err := fmt.Fprintf(os.Stdout, "%s:%d-%d (%s)\n", loc.FilePath, loc.LineRange.Start, loc.LineRange.End, loc.Language); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(os.Stdout, loc.Content); err != nil {
				return err
			}
		}
		return nil
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

func parseGitHubPRURL(raw string) (string, int, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", 0, fmt.Errorf("parsing --url: %w", err)
	}
	if u.Scheme != "https" || (!strings.EqualFold(u.Host, "github.com") && !strings.EqualFold(u.Host, "www.github.com")) {
		return "", 0, fmt.Errorf("--url must use https://github.com or https://www.github.com")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[0] == "" || parts[1] == "" || parts[2] != "pull" {
		return "", 0, fmt.Errorf("--url must be a GitHub PR URL like https://github.com/owner/repo/pull/123")
	}
	pr, err := parsePositiveURLID(parts[3], "pull request number")
	if err != nil {
		return "", 0, err
	}
	return parts[0] + "/" + parts[1], pr, nil
}

func parseGitLabMRURL(raw string) (string, int, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", 0, "", fmt.Errorf("parsing --url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", 0, "", fmt.Errorf("--url must use http or https")
	}
	if u.Host == "" {
		return "", 0, "", fmt.Errorf("--url must include a GitLab host")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "-" || parts[i+1] != "merge_requests" {
			continue
		}
		if i == 0 {
			return "", 0, "", fmt.Errorf("--url must include a GitLab project path before /-/merge_requests/")
		}
		mr, err := parsePositiveURLID(parts[i+2], "merge request IID")
		if err != nil {
			return "", 0, "", err
		}
		return strings.Join(parts[:i], "/"), mr, u.Scheme + "://" + u.Host, nil
	}
	return "", 0, "", fmt.Errorf("--url must be a GitLab MR URL like https://gitlab.example.com/group/project/-/merge_requests/123")
}

func parsePositiveURLID(raw, label string) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("--url has invalid %s %q", label, raw)
	}
	return id, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (a *app) logf(ctx context.Context, format string, args ...any) {
	if a.logger == nil {
		return
	}
	a.logger.Verbosef(ctx, format, args...)
}

func (a *app) logProgress(ctx context.Context, stage logging.Stage, state logging.State, msg string) {
	if a.logger == nil {
		return
	}
	a.logger.Progress(ctx, stage, state, msg)
}

// profileProgressInfo builds the command-level logging identity: model, effort
// and endpoint, rendered in the bracket of every top-level progress line.
func profileProgressInfo(profile config.Profile) logging.ProgressInfo {
	return logging.ProgressInfo{
		Model:   profile.Model,
		Effort:  profile.ReasoningEffort,
		BaseURL: profile.BaseURL,
	}
}

// smallModelProgressInfo mirrors profileProgressInfo for the profile's small
// model, so the small-model line renders its own model and effort and prefixes
// the model with "@small" (the alias steps reference it by) to set it apart from
// the primary.
func smallModelProgressInfo(profile config.Profile) logging.ProgressInfo {
	return logging.ProgressInfo{
		Model:   workflow.SmallModelAlias + " " + profile.Model,
		Effort:  profile.ReasoningEffort,
		BaseURL: profile.BaseURL,
	}
}

// workflowProgressInfo builds the Agent-line bracket identity: the workflow
// name, where it came from (the embedded default, a --spec file, or a single
// --step), and its flattened step count. It deliberately omits the model — the
// Model line above already shows that — so the Agent line answers "which
// workflow, from where" instead of repeating model:effort @ url.
func workflowProgressInfo(spec workflow.Spec, specPath, stepName string) logging.ProgressInfo {
	info := logging.ProgressInfo{WorkflowSteps: len(spec.FlatSteps())}
	switch {
	case specPath != "":
		base := filepath.Base(specPath)
		info.Workflow = strings.TrimSuffix(base, filepath.Ext(base))
		info.WorkflowSource = specPath
	case stepName != "":
		info.Workflow = stepName
		info.WorkflowSource = "step"
	default:
		info.Workflow = "default"
		info.WorkflowSource = "embedded"
	}
	return info
}

// logSmallModelReady prints a Model ready line for the profile's small model
// when it resolves to a model different from the primary, so --show-progress
// surfaces the second model in play (the one @small steps run on). When the
// small config inherits the primary model there is nothing extra to show.
func (a *app) logSmallModelReady(ctx context.Context, profile config.Profile, req model.ReviewRequest) {
	small := config.EffectiveSmallProfile(profile)
	if small.Model == profile.Model {
		return
	}
	smallCtx := logging.WithProgressInfo(ctx, smallModelProgressInfo(small))
	a.logProgress(smallCtx, logging.StageModel, logging.StateReady, modelSummary(small, req))
}

func modelSummary(profile config.Profile, req model.ReviewRequest) string {
	flags := []string{model.HumanTokens(req.MaxContextTokens) + " context"}
	if profile.MaxTokens != nil {
		flags = append(flags, model.HumanTokens(*profile.MaxTokens)+" output")
	}
	if profile.Temperature != nil {
		flags = append(flags, fmt.Sprintf("temp=%g", *profile.Temperature))
	}
	if profile.TopP != nil {
		flags = append(flags, fmt.Sprintf("top_p=%g", *profile.TopP))
	}
	if profile.TopK != nil {
		flags = append(flags, fmt.Sprintf("top_k=%d", *profile.TopK))
	}
	if profile.PresencePenalty != nil {
		flags = append(flags, fmt.Sprintf("presence_penalty=%g", *profile.PresencePenalty))
	}
	flags = append(flags, formatExtraBody(profile.ExtraBody)...)
	return strings.Join(flags, ", ")
}

// formatExtraBody renders a profile's extra_body entries for the Model progress
// line. Nested maps are flattened to dotted leaf paths; each leaf shows by its
// bare final segment when that segment is unique across all entries, or by its
// full dotted path when the segment collides. Output is sorted so the line is
// stable across runs.
func formatExtraBody(m map[string]any) []string {
	if len(m) == 0 {
		return nil
	}
	type leaf struct{ path, value string }
	var leaves []leaf
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		if sub, ok := v.(map[string]any); ok && len(sub) > 0 {
			keys := make([]string, 0, len(sub))
			for k := range sub {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				child := k
				if prefix != "" {
					child = prefix + "." + k
				}
				walk(child, sub[k])
			}
			return
		}
		leaves = append(leaves, leaf{path: prefix, value: extraBodyValue(v)})
	}
	walk("", m)

	nameCount := map[string]int{}
	for _, lf := range leaves {
		nameCount[extraBodyLeafName(lf.path)]++
	}
	out := make([]string, 0, len(leaves))
	for _, lf := range leaves {
		key := lf.path
		if name := extraBodyLeafName(lf.path); nameCount[name] == 1 {
			key = name
		}
		out = append(out, key+"="+lf.value)
	}
	sort.Strings(out)
	return out
}

// extraBodyLeafName returns the final dotted segment of a flattened key.
func extraBodyLeafName(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return path[i+1:]
	}
	return path
}

// extraBodyValue renders a leaf value compactly: bools as true/false, slices as
// [a, b], everything else via %v (so float64 uses %g formatting, e.g. 0.05).
// Top-level maps are flattened by formatExtraBody before reaching here, but a
// map nested inside a slice is rendered in place as {k=v, ...}.
func extraBodyValue(v any) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(val)
	case string:
		return val
	case []any:
		parts := make([]string, len(val))
		for i, e := range val {
			parts[i] = extraBodyValue(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		return "{" + strings.Join(formatExtraBody(val), ", ") + "}"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func agentSummary(profile config.Profile, req model.ReviewRequest) string {
	kind := "Unstructured"
	if !req.DisableJSONResponseFormat {
		kind = "Structured"
	}
	flags := []string{
		disablable(req.NudgeCount, "", "nudges"),
		disablable(req.MaxOutputRetries, "", "retries"),
		unlimited(req.MaxReasoningSeconds, "s", "reasoning"),
		disablable(profile.MaxRateLimitDelaySeconds, "s", "rate-limit-delay"),
		unlimited(req.Concurrency, "", "concurrency"),
		unlimited(req.MaxToolCalls, "", "tool calls"),
	}
	// Unset (0) means unlimited and is the default; only surface a real cap.
	if req.MaxFindings > 0 {
		flags = append(flags, fmt.Sprintf("≤%d findings", req.MaxFindings))
	}
	if !req.DisableParallelToolCalls {
		flags = append(flags, "parallel")
	}
	flags = append(flags, unlimited(req.MaxDuplicateToolCalls, "", "duplicates"))
	if req.DisableSuggestions {
		flags = append(flags, "no suggestions")
	}
	if req.DisablePatchSummary {
		flags = append(flags, "no patch summary")
	}
	if req.DisableReasoningExtract {
		flags = append(flags, "no reasoning extract")
	}
	if req.VerifyDropPolicy != "" && req.VerifyDropPolicy != review.DropPolicyNone {
		flags = append(flags, fmt.Sprintf("drop %s", req.VerifyDropPolicy))
	}
	if req.ConfidenceThreshold > 0 {
		flags = append(flags, fmt.Sprintf("confidence ≥%g", req.ConfidenceThreshold))
	}
	// p3 is the lowest rank (--priority-threshold default): everything shown, so
	// the filter is only worth surfacing when set above it.
	if req.PriorityThreshold != "" && req.PriorityThreshold != "p3" {
		flags = append(flags, "≥"+req.PriorityThreshold)
	}
	return fmt.Sprintf("%s %s", kind, strings.Join(flags, ", "))
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
	parts := []string{
		fmt.Sprintf("findings=%d", len(result.Findings)),
		fmt.Sprintf("total_tool_calls=%d", result.TotalToolCalls),
		fmt.Sprintf("duplicate_tool_calls=%d", totalDuplicateToolCalls(result.AgentRuns)),
		fmt.Sprintf("prompt_tokens=%s", model.HumanTokens(result.TokensUsed.PromptTokens)),
		fmt.Sprintf("completion_tokens=%s", model.HumanTokens(result.TokensUsed.CompletionTokens)),
		fmt.Sprintf("total_tokens=%s", model.HumanTokens(result.TokensUsed.TotalTokens)),
	}
	if result.RuntimeSeconds > 0 {
		parts = append(parts, fmt.Sprintf("runtime=%s", model.HumanDuration(time.Duration(result.RuntimeSeconds*float64(time.Second)))))
	}
	return strings.Join(parts, ", ")
}

func totalDuplicateToolCalls(runs []model.AgentRun) int {
	total := 0
	for _, run := range runs {
		total += run.DuplicateToolCalls
	}
	return total
}
