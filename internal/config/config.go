package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/mappings"
	"gopkg.in/yaml.v3"
)

const (
	DefaultProfileName         = "default"
	DefaultFallbackProfileName = "openrouter"
	DefaultMaxContextToken     = 120000
	// DefaultMaxToolCalls is 0, meaning unlimited tool calls per agent.
	DefaultMaxToolCalls             = 0
	DefaultMaxDuplicateToolCalls    = 5
	DefaultMaxOutputRetries         = 5
	DefaultMaxReasoningSeconds      = 300
	DefaultMaxRateLimitDelaySeconds = 300
	DefaultNudgeCount               = 3
	DefaultConfigPath               = ".nickpit.yaml"
	DefaultReasoningEffort          = "high"
	DefaultGitHubTokenRef           = "${NICKPIT_GITHUB_TOKEN}"
	DefaultGitLabTokenRef           = "${NICKPIT_GITLAB_TOKEN}"
	DefaultGitLabBaseURLRef         = "${NICKPIT_GITLAB_BASE_URL}"
	// DefaultAssetBaseURL is where the published-review badge SVGs are served.
	// The Pages workflow deploys the repo's assets/ directory here.
	DefaultAssetBaseURL = "https://dgrieser.github.io/nickpit/"
)

var envReferencePattern = regexp.MustCompile(`^\$(?:([A-Za-z_][A-Za-z0-9_]*)|\{([A-Za-z_][A-Za-z0-9_]*)\})$`)

type Config struct {
	ActiveProfile string             `yaml:"active_profile"`
	Profiles      map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	Model                              string              `yaml:"model"`
	Small                              SmallModelConfig    `yaml:"small"`
	BaseURL                            string              `yaml:"base_url"`
	APIKey                             string              `yaml:"api_key"`
	SupportedModels                    []ModelCapabilities `yaml:"supported_models"`
	MaxTokens                          *int                `yaml:"max_tokens"`
	Temperature                        *float64            `yaml:"temperature"`
	TopP                               *float64            `yaml:"top_p"`
	TopK                               *int                `yaml:"top_k"`
	PresencePenalty                    *float64            `yaml:"presence_penalty"`
	ExtraBody                          map[string]any      `yaml:"extra_body"`
	DisableJSONResponseFormat          bool                `yaml:"disable_json_response_format"`
	IncludePaths                       []string            `yaml:"include_paths"`
	ExcludePaths                       []string            `yaml:"exclude_paths"`
	IncludeContent                     []string            `yaml:"include_content"`
	ExcludeContent                     []string            `yaml:"exclude_content"`
	StyleGuides                        []string            `yaml:"styleguides"`
	DisableStyleGuides                 []string            `yaml:"disable_styleguides"`
	DiffFormat                         model.DiffFormat    `yaml:"diff_format"`
	MaxContextTokens                   int                 `yaml:"max_context_tokens"`
	MaxToolCalls                       int                 `yaml:"max_tool_calls"`
	MaxDuplicateToolCalls              int                 `yaml:"max_duplicate_tool_calls"`
	MaxOutputRetries                   int                 `yaml:"max_output_retries"`
	MaxReasoningSeconds                int                 `yaml:"max_reasoning_seconds"`
	MaxRateLimitDelaySeconds           int                 `yaml:"max_rate_limit_delay_seconds"`
	NudgeCount                         int                 `yaml:"nudge_count"`
	MaxFindings                        int                 `yaml:"max_findings"`
	DisablePatchSummary                bool                `yaml:"disable_patch_summary"`
	DisableSuggestions                 bool                `yaml:"disable_suggestions"`
	DisableWorkflowTimeBudget          bool                `yaml:"disable_workflow_time_budget"`
	ReasoningEffort                    string              `yaml:"reasoning_effort"`
	Workdir                            string              `yaml:"workdir"`
	GitHubToken                        string              `yaml:"github_token"`
	GitLabToken                        string              `yaml:"gitlab_token"`
	GitLabBaseURL                      string              `yaml:"gitlab_base_url"`
	AssetBaseURL                       string              `yaml:"asset_base_url"`
	MaxContextTokensConfigured         bool                `yaml:"-"`
	APIKeyConfigured                   bool                `yaml:"-"`
	MaxToolCallsConfigured             bool                `yaml:"-"`
	MaxDuplicateToolCallsConfigured    bool                `yaml:"-"`
	MaxOutputRetriesConfigured         bool                `yaml:"-"`
	MaxReasoningSecondsConfigured      bool                `yaml:"-"`
	MaxRateLimitDelaySecondsConfigured bool                `yaml:"-"`
	NudgeCountConfigured               bool                `yaml:"-"`
	MaxFindingsConfigured              bool                `yaml:"-"`
}

type SmallModelConfig struct {
	Model           string         `yaml:"model"`
	MaxTokens       *int           `yaml:"max_tokens"`
	Temperature     *float64       `yaml:"temperature"`
	TopP            *float64       `yaml:"top_p"`
	TopK            *int           `yaml:"top_k"`
	PresencePenalty *float64       `yaml:"presence_penalty"`
	ExtraBody       map[string]any `yaml:"extra_body"`
	ReasoningEffort string         `yaml:"reasoning_effort"`
}

type ModelCapabilities struct {
	Model        string                `json:"model" yaml:"model"`
	Compatible   bool                  `json:"compatible" yaml:"compatible"`
	Response     bool                  `json:"response" yaml:"response"`
	Reasoning    ReasoningCapabilities `json:"reasoning" yaml:"reasoning"`
	Tools        bool                  `json:"tools" yaml:"tools"`
	JSONSchema   *bool                 `json:"json_schema,omitempty" yaml:"json_schema,omitempty"`
	JSONResponse *bool                 `json:"json_response,omitempty" yaml:"json_response,omitempty"`
}

type ReasoningCapabilities struct {
	Traces  bool     `json:"traces" yaml:"traces"`
	Efforts []string `json:"efforts" yaml:"efforts"`
}

type Overrides struct {
	Profile                   string
	Model                     string
	Small                     SmallModelConfig
	BaseURL                   string
	APIKey                    string
	MaxTokens                 *int
	Temperature               *float64
	TopP                      *float64
	TopK                      *int
	PresencePenalty           *float64
	ExtraBody                 map[string]any
	DisableJSONResponseFormat bool
	IncludePaths              *[]string
	ExcludePaths              *[]string
	IncludeContent            *[]string
	ExcludeContent            *[]string
	// StyleGuides and DisableStyleGuides are plain slices, not *[]string like
	// the filter lists: CLI values append to the profile's list, so nil and
	// empty behave identically.
	StyleGuides               []string
	DisableStyleGuides        []string
	DiffFormat                model.DiffFormat
	MaxContextTokens          *int
	ToolCalls                 *int
	DuplicateToolCalls        *int
	OutputRetries             *int
	ReasoningSeconds          *int
	RateLimitDelaySeconds     *int
	NudgeCount                *int
	MaxFindings               *int
	DisablePatchSummary       bool
	DisableSuggestions        bool
	DisableWorkflowTimeBudget bool
	ReasoningEffort           string
	Workdir                   string
	GitHubToken               string
	GitLabToken               string
	GitLabBaseURL             string
}

type defaultProfile struct {
	name    string
	profile Profile
}

func ptrTo[T any](v T) *T { return &v }

var defaultProfiles = []defaultProfile{
	{
		name: DefaultProfileName,
		profile: Profile{
			BaseURL: "https://openrouter.ai/api/v1",
			APIKey:  "$OPENROUTER_API_KEY",
		},
	},
	{
		name: "mittwald",
		profile: Profile{
			BaseURL:         "https://llm.aihosting.mittwald.de/v1",
			Model:           "Qwen3.5-122B-A10B-FP8",
			ReasoningEffort: "high",
			Temperature:     ptrTo(0.6),
			TopP:            ptrTo(0.95),
			TopK:            ptrTo(20),
			PresencePenalty: ptrTo(1.0),
			Small: SmallModelConfig{
				Model:           "Qwen3.6-35B-A3B-FP8",
				ReasoningEffort: "none",
				Temperature:     ptrTo(0.7),
				TopP:            ptrTo(1.0),
				TopK:            ptrTo(40),
				PresencePenalty: ptrTo(2.0),
				ExtraBody: map[string]any{
					"chat_template_kwargs": map[string]any{
						"enable_thinking": false,
					},
				},
			},
			APIKey: "$MITTWALD_LLM_API_KEY",
		},
	},
	{
		name: "mistral",
		profile: Profile{
			BaseURL: "https://api.mistral.ai/v1",
			Model:   "mistral-small-latest",
			APIKey:  "$MISTRAL_API_KEY",
		},
	},
	{
		name: "deepseek",
		profile: Profile{
			BaseURL: "https://api.deepseek.com",
			Model:   "deepseek-v4-flash",
			APIKey:  "$DEEPSEEK_API_KEY",
		},
	},
	{
		name: "alibaba-cloud",
		profile: Profile{
			BaseURL: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
			Model:   "qwen-plus",
			APIKey:  "$DASHSCOPE_API_KEY",
		},
	},
	{
		name: "nvidia",
		profile: Profile{
			BaseURL:   "https://integrate.api.nvidia.com/v1",
			Model:     "",
			APIKey:    "$NVIDIA_API_KEY",
			MaxTokens: ptrTo(16384),
		},
	},
}

func DefaultConfig() *Config {
	profiles := make(map[string]Profile, len(defaultProfiles))
	for _, entry := range defaultProfiles {
		profiles[entry.name] = cloneProfile(entry.profile)
	}
	return &Config{
		ActiveProfile: DefaultProfileName,
		Profiles:      profiles,
	}
}

func cloneProfile(profile Profile) Profile {
	if profile.MaxTokens != nil {
		value := *profile.MaxTokens
		profile.MaxTokens = &value
	}
	if profile.Temperature != nil {
		value := *profile.Temperature
		profile.Temperature = &value
	}
	if profile.TopP != nil {
		value := *profile.TopP
		profile.TopP = &value
	}
	if profile.TopK != nil {
		value := *profile.TopK
		profile.TopK = &value
	}
	if profile.PresencePenalty != nil {
		value := *profile.PresencePenalty
		profile.PresencePenalty = &value
	}
	profile.ExtraBody = cloneMap(profile.ExtraBody)
	profile.Small = cloneSmallModelConfig(profile.Small)
	profile.SupportedModels = cloneSupportedModels(profile.SupportedModels)
	profile.IncludePaths = slices.Clone(profile.IncludePaths)
	profile.ExcludePaths = slices.Clone(profile.ExcludePaths)
	profile.IncludeContent = slices.Clone(profile.IncludeContent)
	profile.ExcludeContent = slices.Clone(profile.ExcludeContent)
	profile.StyleGuides = slices.Clone(profile.StyleGuides)
	profile.DisableStyleGuides = slices.Clone(profile.DisableStyleGuides)
	return profile
}

func cloneSmallModelConfig(small SmallModelConfig) SmallModelConfig {
	if small.MaxTokens != nil {
		value := *small.MaxTokens
		small.MaxTokens = &value
	}
	if small.Temperature != nil {
		value := *small.Temperature
		small.Temperature = &value
	}
	if small.TopP != nil {
		value := *small.TopP
		small.TopP = &value
	}
	if small.TopK != nil {
		value := *small.TopK
		small.TopK = &value
	}
	if small.PresencePenalty != nil {
		value := *small.PresencePenalty
		small.PresencePenalty = &value
	}
	small.ExtraBody = cloneMap(small.ExtraBody)
	return small
}

func mergeSmallModelConfig(base, override SmallModelConfig) SmallModelConfig {
	if override.Model != "" {
		base.Model = override.Model
	}
	if override.MaxTokens != nil {
		base.MaxTokens = override.MaxTokens
	}
	if override.Temperature != nil {
		base.Temperature = override.Temperature
	}
	if override.TopP != nil {
		base.TopP = override.TopP
	}
	if override.TopK != nil {
		base.TopK = override.TopK
	}
	if override.PresencePenalty != nil {
		base.PresencePenalty = override.PresencePenalty
	}
	if override.ExtraBody != nil {
		base.ExtraBody = override.ExtraBody
	}
	if override.ReasoningEffort != "" {
		base.ReasoningEffort = override.ReasoningEffort
	}
	return base
}

func EffectiveSmallProfile(profile Profile) Profile {
	profile = cloneProfile(profile)
	small := profile.Small
	if small.Model != "" {
		profile.Model = small.Model
	}
	if small.MaxTokens != nil {
		profile.MaxTokens = small.MaxTokens
	}
	if small.Temperature != nil {
		profile.Temperature = small.Temperature
	}
	if small.TopP != nil {
		profile.TopP = small.TopP
	}
	if small.TopK != nil {
		profile.TopK = small.TopK
	}
	if small.PresencePenalty != nil {
		profile.PresencePenalty = small.PresencePenalty
	}
	if small.ExtraBody != nil {
		profile.ExtraBody = small.ExtraBody
	}
	if small.ReasoningEffort != "" {
		profile.ReasoningEffort = small.ReasoningEffort
	}
	return profile
}

func cloneSupportedModels(models []ModelCapabilities) []ModelCapabilities {
	if models == nil {
		return nil
	}
	cloned := make([]ModelCapabilities, len(models))
	for i, model := range models {
		cloned[i] = model
		cloned[i].Reasoning.Efforts = append([]string(nil), model.Reasoning.Efforts...)
		if model.JSONSchema != nil {
			value := *model.JSONSchema
			cloned[i].JSONSchema = &value
		}
		if model.JSONResponse != nil {
			value := *model.JSONResponse
			cloned[i].JSONResponse = &value
		}
	}
	return cloned
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = cloneValue(item)
	}
	return cloned
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i, item := range typed {
			cloned[i] = cloneValue(item)
		}
		return cloned
	default:
		return typed
	}
}

func Load(path string, overrides Overrides) (*Config, Profile, error) {
	cfg := DefaultConfig()
	if path == "" {
		path = DefaultConfigPath
	}

	if _, err := loadFile(cfg, path); err != nil {
		return nil, Profile{}, err
	}

	activeProfile := cfg.ActiveProfile
	if overrides.Profile != "" {
		activeProfile = overrides.Profile
	}
	if activeProfile == "" {
		activeProfile = DefaultProfileName
	}
	resolvedProfile := resolveProfileName(cfg, activeProfile)
	if err := applyEnv(cfg, resolvedProfile); err != nil {
		return nil, Profile{}, err
	}

	profile, err := ResolveProfile(cfg, resolvedProfile)
	if err != nil {
		return nil, Profile{}, err
	}
	profile, err = applyOverrides(profile, overrides)
	if err != nil {
		return nil, Profile{}, err
	}
	cfg.ActiveProfile = activeProfile
	cfg.Profiles[activeProfile] = profile
	return cfg, profile, nil
}

func resolveProfileName(cfg *Config, name string) string {
	if name == DefaultFallbackProfileName {
		if _, ok := cfg.Profiles[name]; !ok {
			return DefaultProfileName
		}
	}
	return name
}

func loadFile(cfg *Config, path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("config: reading %s: %w", path, err)
	}

	expanded := os.ExpandEnv(string(data))
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(expanded), &root); err != nil {
		return false, fmt.Errorf("config: parsing %s: %w", path, err)
	}
	if len(root.Content) == 0 {
		return true, nil
	}

	var fileCfg Config
	if err := root.Content[0].Decode(&fileCfg); err != nil {
		return false, fmt.Errorf("config: parsing %s: %w", path, err)
	}
	if err := markConfiguredFields(root.Content[0], &fileCfg); err != nil {
		return false, fmt.Errorf("config: parsing %s: %w", path, err)
	}

	if fileCfg.ActiveProfile != "" {
		cfg.ActiveProfile = fileCfg.ActiveProfile
	}
	for name, profile := range fileCfg.Profiles {
		base := cfg.Profiles[name]
		cfg.Profiles[name] = mergeProfiles(base, profile)
	}
	return true, nil
}

func applyEnv(cfg *Config, profileName string) error {
	profile := cfg.Profiles[profileName]
	if value := os.Getenv("NICKPIT_MODEL"); value != "" {
		profile.Model = value
	}
	if value := os.Getenv("NICKPIT_SMALL_MODEL"); value != "" {
		profile.Small.Model = value
	}
	if value := os.Getenv("NICKPIT_SMALL_REASONING_EFFORT"); value != "" {
		profile.Small.ReasoningEffort = value
	}
	if value := os.Getenv("NICKPIT_REASONING_EFFORT"); value != "" {
		profile.ReasoningEffort = value
	}
	if value := os.Getenv("NICKPIT_MAX_TOKENS"); value != "" {
		parsed, err := parseEnvInt("NICKPIT_MAX_TOKENS", value)
		if err != nil {
			return err
		}
		profile.MaxTokens = &parsed
	}
	if value := os.Getenv("NICKPIT_TEMPERATURE"); value != "" {
		parsed, err := parseEnvFloat("NICKPIT_TEMPERATURE", value)
		if err != nil {
			return err
		}
		profile.Temperature = &parsed
	}
	if value := os.Getenv("NICKPIT_TOP_P"); value != "" {
		parsed, err := parseEnvFloat("NICKPIT_TOP_P", value)
		if err != nil {
			return err
		}
		profile.TopP = &parsed
	}
	if value := os.Getenv("NICKPIT_TOP_K"); value != "" {
		parsed, err := parseEnvInt("NICKPIT_TOP_K", value)
		if err != nil {
			return err
		}
		profile.TopK = &parsed
	}
	if value := os.Getenv("NICKPIT_PRESENCE_PENALTY"); value != "" {
		parsed, err := parseEnvFloat("NICKPIT_PRESENCE_PENALTY", value)
		if err != nil {
			return err
		}
		profile.PresencePenalty = &parsed
	}
	if value := os.Getenv("NICKPIT_EXTRA_BODY"); strings.TrimSpace(value) != "" {
		extraBody, err := parseEnvExtraBody("NICKPIT_EXTRA_BODY", value)
		if err != nil {
			return err
		}
		profile.ExtraBody = extraBody
	}
	if value := os.Getenv("NICKPIT_SMALL_MAX_TOKENS"); value != "" {
		parsed, err := parseEnvInt("NICKPIT_SMALL_MAX_TOKENS", value)
		if err != nil {
			return err
		}
		profile.Small.MaxTokens = &parsed
	}
	if value := os.Getenv("NICKPIT_SMALL_TEMPERATURE"); value != "" {
		parsed, err := parseEnvFloat("NICKPIT_SMALL_TEMPERATURE", value)
		if err != nil {
			return err
		}
		profile.Small.Temperature = &parsed
	}
	if value := os.Getenv("NICKPIT_SMALL_TOP_P"); value != "" {
		parsed, err := parseEnvFloat("NICKPIT_SMALL_TOP_P", value)
		if err != nil {
			return err
		}
		profile.Small.TopP = &parsed
	}
	if value := os.Getenv("NICKPIT_SMALL_TOP_K"); value != "" {
		parsed, err := parseEnvInt("NICKPIT_SMALL_TOP_K", value)
		if err != nil {
			return err
		}
		profile.Small.TopK = &parsed
	}
	if value := os.Getenv("NICKPIT_SMALL_PRESENCE_PENALTY"); value != "" {
		parsed, err := parseEnvFloat("NICKPIT_SMALL_PRESENCE_PENALTY", value)
		if err != nil {
			return err
		}
		profile.Small.PresencePenalty = &parsed
	}
	if value := os.Getenv("NICKPIT_SMALL_EXTRA_BODY"); strings.TrimSpace(value) != "" {
		extraBody, err := parseEnvExtraBody("NICKPIT_SMALL_EXTRA_BODY", value)
		if err != nil {
			return err
		}
		profile.Small.ExtraBody = extraBody
	}
	if value := os.Getenv("NICKPIT_BASE_URL"); value != "" {
		profile.BaseURL = value
	}
	if value := os.Getenv("NICKPIT_WORKDIR"); value != "" {
		profile.Workdir = value
	}
	if value := os.Getenv("GITHUB_TOKEN"); value != "" {
		profile.GitHubToken = value
	}
	if value := os.Getenv("NICKPIT_GITHUB_TOKEN"); value != "" {
		profile.GitHubToken = value
	}
	if value := os.Getenv("GITLAB_TOKEN"); value != "" {
		profile.GitLabToken = value
	}
	if value := os.Getenv("NICKPIT_GITLAB_TOKEN"); value != "" {
		profile.GitLabToken = value
	}
	if value := os.Getenv("GITLAB_BASE_URL"); value != "" {
		profile.GitLabBaseURL = value
	}
	if value := os.Getenv("NICKPIT_GITLAB_BASE_URL"); value != "" {
		profile.GitLabBaseURL = value
	}
	// NICKPIT_API_KEY is the last-resort API key: it applies only when the
	// active profile's api_key (after resolving an $ENV reference such as
	// $OPENROUTER_API_KEY) would be empty. Configured keys, profile-specific
	// env vars, and --api-key all take precedence.
	if value := os.Getenv("NICKPIT_API_KEY"); value != "" && expandEnvReference(profile.APIKey) == "" {
		profile.APIKey = value
	}
	cfg.Profiles[profileName] = profile
	return nil
}

func applyOverrides(profile Profile, overrides Overrides) (Profile, error) {
	if overrides.Model != "" {
		profile.Model = overrides.Model
	}
	profile.Small = mergeSmallModelConfig(profile.Small, overrides.Small)
	if overrides.BaseURL != "" {
		profile.BaseURL = overrides.BaseURL
	}
	if overrides.APIKey != "" {
		profile.APIKey = overrides.APIKey
	}
	if overrides.MaxTokens != nil {
		profile.MaxTokens = overrides.MaxTokens
	}
	if overrides.Temperature != nil {
		profile.Temperature = overrides.Temperature
	}
	if overrides.TopP != nil {
		profile.TopP = overrides.TopP
	}
	if overrides.TopK != nil {
		profile.TopK = overrides.TopK
	}
	if overrides.PresencePenalty != nil {
		profile.PresencePenalty = overrides.PresencePenalty
	}
	if overrides.ExtraBody != nil {
		profile.ExtraBody = overrides.ExtraBody
	}
	if overrides.DisableJSONResponseFormat {
		profile.DisableJSONResponseFormat = true
	}
	if overrides.IncludePaths != nil {
		profile.IncludePaths = slices.Clone(*overrides.IncludePaths)
	}
	if overrides.ExcludePaths != nil {
		profile.ExcludePaths = slices.Clone(*overrides.ExcludePaths)
	}
	if overrides.IncludeContent != nil {
		profile.IncludeContent = slices.Clone(*overrides.IncludeContent)
	}
	if overrides.ExcludeContent != nil {
		profile.ExcludeContent = slices.Clone(*overrides.ExcludeContent)
	}
	// CLI styleguides and disabled languages append to the profile's lists
	// instead of replacing them; duplicates are dropped during normalization.
	profile.StyleGuides = append(slices.Clone(profile.StyleGuides), overrides.StyleGuides...)
	profile.DisableStyleGuides = append(slices.Clone(profile.DisableStyleGuides), overrides.DisableStyleGuides...)
	if overrides.DiffFormat != "" {
		profile.DiffFormat = overrides.DiffFormat
	}
	if overrides.MaxContextTokens != nil {
		profile.MaxContextTokens = *overrides.MaxContextTokens
		profile.MaxContextTokensConfigured = true
	}
	if overrides.ToolCalls != nil {
		profile.MaxToolCalls = *overrides.ToolCalls
		profile.MaxToolCallsConfigured = true
	}
	if overrides.DuplicateToolCalls != nil {
		profile.MaxDuplicateToolCalls = *overrides.DuplicateToolCalls
		profile.MaxDuplicateToolCallsConfigured = true
	}
	if overrides.OutputRetries != nil {
		profile.MaxOutputRetries = *overrides.OutputRetries
		profile.MaxOutputRetriesConfigured = true
	}
	if overrides.ReasoningSeconds != nil {
		profile.MaxReasoningSeconds = *overrides.ReasoningSeconds
		profile.MaxReasoningSecondsConfigured = true
	}
	if overrides.RateLimitDelaySeconds != nil {
		profile.MaxRateLimitDelaySeconds = *overrides.RateLimitDelaySeconds
		profile.MaxRateLimitDelaySecondsConfigured = true
	}
	if overrides.NudgeCount != nil {
		profile.NudgeCount = *overrides.NudgeCount
		profile.NudgeCountConfigured = true
	}
	if overrides.MaxFindings != nil {
		profile.MaxFindings = *overrides.MaxFindings
		profile.MaxFindingsConfigured = true
	}
	if overrides.DisablePatchSummary {
		profile.DisablePatchSummary = true
	}
	if overrides.DisableSuggestions {
		profile.DisableSuggestions = true
	}
	if overrides.DisableWorkflowTimeBudget {
		profile.DisableWorkflowTimeBudget = true
	}
	if overrides.ReasoningEffort != "" {
		profile.ReasoningEffort = overrides.ReasoningEffort
	}
	if overrides.Workdir != "" {
		profile.Workdir = overrides.Workdir
	}
	if overrides.GitHubToken != "" {
		profile.GitHubToken = overrides.GitHubToken
	}
	if overrides.GitLabToken != "" {
		profile.GitLabToken = overrides.GitLabToken
	}
	if overrides.GitLabBaseURL != "" {
		profile.GitLabBaseURL = overrides.GitLabBaseURL
	}
	return normalizeProfile(profile)
}

func parseEnvInt(name, value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer: %w", name, err)
	}
	return parsed, nil
}

func parseEnvFloat(name, value string) (float64, error) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be a number: %w", name, err)
	}
	return parsed, nil
}

func parseEnvExtraBody(name, value string) (map[string]any, error) {
	var extraBody map[string]any
	if err := json.Unmarshal([]byte(value), &extraBody); err != nil {
		return nil, fmt.Errorf("config: parsing %s JSON object: %w", name, err)
	}
	if extraBody == nil {
		return nil, fmt.Errorf("config: %s must be a JSON object", name)
	}
	return extraBody, nil
}

// applyProfileDefaults fills tunables that are unset (zero and not explicitly
// configured) with their built-in defaults. It is the single source of truth
// shared by normalizeProfile (runtime) and exampleProfile (the generated
// .nickpit.yaml.example) so the two cannot drift.
func applyProfileDefaults(profile Profile) Profile {
	if profile.MaxContextTokens == 0 && !profile.MaxContextTokensConfigured {
		profile.MaxContextTokens = DefaultMaxContextToken
	}
	if profile.MaxToolCalls == 0 && !profile.MaxToolCallsConfigured {
		profile.MaxToolCalls = DefaultMaxToolCalls
	}
	if profile.MaxDuplicateToolCalls == 0 && !profile.MaxDuplicateToolCallsConfigured {
		profile.MaxDuplicateToolCalls = DefaultMaxDuplicateToolCalls
	}
	if profile.MaxOutputRetries == 0 && !profile.MaxOutputRetriesConfigured {
		profile.MaxOutputRetries = DefaultMaxOutputRetries
	}
	if profile.MaxReasoningSeconds == 0 && !profile.MaxReasoningSecondsConfigured {
		profile.MaxReasoningSeconds = DefaultMaxReasoningSeconds
	}
	if profile.MaxRateLimitDelaySeconds == 0 && !profile.MaxRateLimitDelaySecondsConfigured {
		profile.MaxRateLimitDelaySeconds = DefaultMaxRateLimitDelaySeconds
	}
	if profile.NudgeCount == 0 && !profile.NudgeCountConfigured {
		profile.NudgeCount = DefaultNudgeCount
	}
	if profile.DiffFormat == "" {
		profile.DiffFormat = model.DiffFormatGit
	}
	if profile.ReasoningEffort == "" {
		profile.ReasoningEffort = DefaultReasoningEffort
	}
	if profile.AssetBaseURL == "" {
		profile.AssetBaseURL = DefaultAssetBaseURL
	}
	return profile
}

func normalizeProfile(profile Profile) (Profile, error) {
	profile.APIKey = expandEnvReference(profile.APIKey)
	profile.GitHubToken = expandEnvReference(profile.GitHubToken)
	profile.GitLabToken = expandEnvReference(profile.GitLabToken)
	profile.GitLabBaseURL = expandEnvReference(profile.GitLabBaseURL)
	if profile.Model == "" {
		return Profile{}, fmt.Errorf("config: no model specified; set model in profile or pass --model")
	}
	if profile.BaseURL == "" {
		return Profile{}, fmt.Errorf("config: no base URL specified; set base URL in profile or pass --base-url")
	}
	profile = applyProfileDefaults(profile)
	if profile.MaxOutputRetries < 0 {
		return Profile{}, fmt.Errorf("config: max_output_retries must be non-negative")
	}
	if profile.MaxReasoningSeconds < 0 {
		return Profile{}, fmt.Errorf("config: max_reasoning_seconds must be non-negative")
	}
	if profile.MaxRateLimitDelaySeconds < 0 {
		return Profile{}, fmt.Errorf("config: max_rate_limit_delay_seconds must be non-negative")
	}
	if profile.NudgeCount < 0 {
		return Profile{}, fmt.Errorf("config: nudge_count must be non-negative")
	}
	if profile.MaxFindings < 0 {
		return Profile{}, fmt.Errorf("config: max_findings must be non-negative")
	}
	if profile.DiffFormat != model.DiffFormatGit && profile.DiffFormat != model.DiffFormatGitJson {
		return Profile{}, fmt.Errorf("config: diff_format must be one of: git, git-json")
	}
	if profile.Workdir != "" {
		profile.Workdir = expandPath(profile.Workdir)
	}
	if profile.GitLabBaseURL == "" {
		profile.GitLabBaseURL = "https://gitlab.com/api/v4"
	}
	if err := validateRegexList("include_paths", profile.IncludePaths); err != nil {
		return Profile{}, err
	}
	if err := validateRegexList("exclude_paths", profile.ExcludePaths); err != nil {
		return Profile{}, err
	}
	if err := validateRegexList("include_content", profile.IncludeContent); err != nil {
		return Profile{}, err
	}
	if err := validateRegexList("exclude_content", profile.ExcludeContent); err != nil {
		return Profile{}, err
	}
	styleGuides, err := normalizeStyleGuideSpecs(profile.StyleGuides)
	if err != nil {
		return Profile{}, err
	}
	profile.StyleGuides = styleGuides
	disabledStyleGuides, err := normalizeDisabledStyleGuideLanguages(profile.DisableStyleGuides)
	if err != nil {
		return Profile{}, err
	}
	profile.DisableStyleGuides = disabledStyleGuides
	return profile, nil
}

// normalizeDisabledStyleGuideLanguages trims, lowercases, and dedupes the
// disable_styleguides list (first occurrence wins) and rejects languages that
// have no built-in styleguide. The special value "all" expands to every
// built-in styleguide language; other entries are still validated first so
// typos never hide behind it.
func normalizeDisabledStyleGuideLanguages(languages []string) ([]string, error) {
	if len(languages) == 0 {
		return nil, nil
	}
	sawAll := false
	normalized := make([]string, 0, len(languages))
	seen := make(map[string]struct{}, len(languages))
	for i, value := range languages {
		language := strings.ToLower(strings.TrimSpace(value))
		if language == "" {
			continue
		}
		if language == "all" {
			sawAll = true
			continue
		}
		if _, ok := mappings.StyleGuideFile(language); !ok {
			return nil, fmt.Errorf("config: disable_styleguides[%d] unknown language %q; available: all, %s", i, value, strings.Join(mappings.StyleGuideOrder(), ", "))
		}
		if _, ok := seen[language]; ok {
			continue
		}
		seen[language] = struct{}{}
		normalized = append(normalized, language)
	}
	if sawAll {
		return mappings.StyleGuideOrder(), nil
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
}

// normalizeStyleGuideSpecs trims specs, drops empties, dedupes exact
// duplicates (first occurrence wins), and shape-validates URL specs. Whether a
// file exists or a URL is fetchable is checked at resolution time, not here:
// config load also runs for commands that never fetch styleguides.
func normalizeStyleGuideSpecs(specs []string) ([]string, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for i, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		if _, ok := seen[spec]; ok {
			continue
		}
		seen[spec] = struct{}{}
		if styleGuideSpecIsURL(spec) {
			parsed, err := url.Parse(spec)
			if err != nil {
				return nil, fmt.Errorf("config: styleguides[%d] invalid URL %q: %w", i, spec, err)
			}
			if parsed.Host == "" {
				return nil, fmt.Errorf("config: styleguides[%d] invalid URL %q: missing host", i, spec)
			}
		}
		normalized = append(normalized, spec)
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
}

// styleGuideSpecIsURL reports whether a styleguide spec addresses a remote
// guide. Only an explicit http(s):// prefix counts; every other spec is a
// file path (so Windows drive paths or odd strings never turn into fetches).
func styleGuideSpecIsURL(spec string) bool {
	lower := strings.ToLower(spec)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func validateRegexList(key string, patterns []string) error {
	for i, pattern := range patterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("config: %s[%d] invalid regex %q: %w", key, i, pattern, err)
		}
	}
	return nil
}

func expandEnvReference(value string) string {
	matches := envReferencePattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) == 0 {
		return value
	}
	name := matches[1]
	if name == "" {
		name = matches[2]
	}
	return os.Getenv(name)
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	// Only expand "~" and "~/..."; "~user/..." is left untouched (expanding it
	// against the current user's home would mangle the path).
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func markConfiguredFields(root *yaml.Node, cfg *Config) error {
	if root == nil {
		return nil
	}
	profiles := mappingValue(root, "profiles")
	if profiles == nil || profiles.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(profiles.Content); i += 2 {
		name := profiles.Content[i].Value
		profileNode := profiles.Content[i+1]
		profile := cfg.Profiles[name]
		profile.MaxContextTokensConfigured = mappingValue(profileNode, "max_context_tokens") != nil
		profile.APIKeyConfigured = mappingValue(profileNode, "api_key") != nil
		profile.MaxToolCallsConfigured = mappingValue(profileNode, "max_tool_calls") != nil
		profile.MaxDuplicateToolCallsConfigured = mappingValue(profileNode, "max_duplicate_tool_calls") != nil
		profile.MaxOutputRetriesConfigured = mappingValue(profileNode, "max_output_retries") != nil
		profile.MaxReasoningSecondsConfigured = mappingValue(profileNode, "max_reasoning_seconds") != nil
		profile.MaxRateLimitDelaySecondsConfigured = mappingValue(profileNode, "max_rate_limit_delay_seconds") != nil
		profile.NudgeCountConfigured = mappingValue(profileNode, "nudge_count") != nil
		profile.MaxFindingsConfigured = mappingValue(profileNode, "max_findings") != nil
		cfg.Profiles[name] = profile
	}
	return nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
