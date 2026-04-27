package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultProfileName           = "default"
	DefaultFallbackProfileName   = "openrouter"
	DefaultMaxContextToken       = 120000
	MaxToolCalls                 = 0
	DefaultMaxDuplicateToolCalls = 5
	DefaultConfigPath            = ".nickpit.yaml"
	DefaultReasoningEffort       = "high"
	DefaultGitHubTokenRef        = "${GITHUB_TOKEN}"
	DefaultGitLabTokenRef        = "${GITLAB_TOKEN}"
	DefaultGitLabBaseURLRef      = "${GITLAB_BASE_URL}"
)

var envReferencePattern = regexp.MustCompile(`^\$(?:([A-Za-z_][A-Za-z0-9_]*)|\{([A-Za-z_][A-Za-z0-9_]*)\})$`)

type Config struct {
	ActiveProfile string             `yaml:"active_profile"`
	Profiles      map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	Model                           string   `yaml:"model"`
	BaseURL                         string   `yaml:"base_url"`
	APIKey                          string   `yaml:"api_key"`
	MaxTokens                       *int     `yaml:"max_tokens"`
	Temperature                     *float64 `yaml:"temperature"`
	UseJSONSchema                   bool     `yaml:"use_json_schema"`
	MaxContextTokens                int      `yaml:"max_context_tokens"`
	MaxToolCalls                    int      `yaml:"max_tool_calls"`
	MaxDuplicateToolCalls           int      `yaml:"max_duplicate_tool_calls"`
	ReasoningEffort                 string   `yaml:"reasoning_effort"`
	Workdir                         string   `yaml:"workdir"`
	GitHubToken                     string   `yaml:"github_token"`
	GitLabToken                     string   `yaml:"gitlab_token"`
	GitLabBaseURL                   string   `yaml:"gitlab_base_url"`
	MaxContextTokensConfigured      bool     `yaml:"-"`
	APIKeyConfigured                bool     `yaml:"-"`
	MaxToolCallsConfigured          bool     `yaml:"-"`
	MaxDuplicateToolCallsConfigured bool     `yaml:"-"`
}

type Overrides struct {
	Profile            string
	Model              string
	BaseURL            string
	APIKey             string
	MaxTokens          *int
	Temperature        *float64
	UseJSONSchema      bool
	MaxContextTokens   *int
	ToolCalls          *int
	DuplicateToolCalls *int
	ReasoningEffort    string
	Workdir            string
	GitHubToken        string
	GitLabToken        string
	GitLabBaseURL      string
}

type defaultProfile struct {
	name    string
	profile Profile
}

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
			BaseURL: "https://llm.aihosting.mittwald.de/v1",
			Model:   "gpt-oss-120b",
			APIKey:  "$MITTWALD_LLM_API_KEY",
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
			BaseURL: "https://integrate.api.nvidia.com/v1",
			Model:   "",
			APIKey:  "$NVIDIA_API_KEY",
            MaxTokens: 16384,
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
	return profile
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
	applyEnv(cfg, resolvedProfile)

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

func applyEnv(cfg *Config, profileName string) {
	profile := cfg.Profiles[profileName]
	if value := os.Getenv("NICKPIT_MODEL"); value != "" {
		profile.Model = value
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
	if value := os.Getenv("GITLAB_TOKEN"); value != "" {
		profile.GitLabToken = value
	}
	if value := os.Getenv("GITLAB_BASE_URL"); value != "" {
		profile.GitLabBaseURL = value
	}
	cfg.Profiles[profileName] = profile
}

func applyOverrides(profile Profile, overrides Overrides) (Profile, error) {
	if overrides.Model != "" {
		profile.Model = overrides.Model
	}
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
	if overrides.UseJSONSchema {
		profile.UseJSONSchema = true
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
	if profile.MaxContextTokens == 0 && !profile.MaxContextTokensConfigured {
		profile.MaxContextTokens = DefaultMaxContextToken
	}
	if profile.MaxToolCalls == 0 && !profile.MaxToolCallsConfigured {
		profile.MaxToolCalls = MaxToolCalls
	}
	if profile.MaxDuplicateToolCalls == 0 && !profile.MaxDuplicateToolCallsConfigured {
		profile.MaxDuplicateToolCalls = DefaultMaxDuplicateToolCalls
	}
	if profile.Workdir != "" {
		profile.Workdir = expandPath(profile.Workdir)
	}
	if profile.GitLabBaseURL == "" {
		profile.GitLabBaseURL = "https://gitlab.com/api/v4"
	}
	return profile, nil
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
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
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
