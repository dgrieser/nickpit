package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultProfileName     = "default"
	DefaultModel           = "gpt-oss-120b"
	DefaultBaseURL         = "https://llm.aihosting.mittwald.de/v1"
	DefaultMaxContextToken = 120000
	MaxToolCalls           = 0
	DefaultConfigPath      = ".nickpit.yaml"
	DefaultReasoningEffort = "high"
)

type Config struct {
	ActiveProfile string             `yaml:"active_profile"`
	Profiles      map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	Model            string   `yaml:"model"`
	BaseURL          string   `yaml:"base_url"`
	APIKey           string   `yaml:"api_key"`
	MaxTokens        *int     `yaml:"max_tokens"`
	Temperature      *float64 `yaml:"temperature"`
	UseJSONSchema    bool     `yaml:"use_json_schema"`
	MaxContextTokens int      `yaml:"max_context_tokens"`
	MaxToolCalls     int      `yaml:"max_tool_calls"`
	ReasoningEffort  string   `yaml:"reasoning_effort"`
	Workdir          string   `yaml:"workdir"`
	GitHubToken      string   `yaml:"github_token"`
	GitLabToken      string   `yaml:"gitlab_token"`
	GitLabBaseURL    string   `yaml:"gitlab_base_url"`
	APIKeyConfigured bool     `yaml:"-"`
}

type Overrides struct {
	Profile          string
	Model            string
	BaseURL          string
	APIKey           string
	MaxTokens        *int
	Temperature      *float64
	UseJSONSchema    bool
	MaxContextTokens int
	ToolCalls        int
	ReasoningEffort  string
	Workdir          string
	GitHubToken      string
	GitLabToken      string
	GitLabBaseURL    string
}

func DefaultConfig() *Config {
	return &Config{
		ActiveProfile: DefaultProfileName,
		Profiles: map[string]Profile{
			DefaultProfileName: {
				Model:            DefaultModel,
				BaseURL:          DefaultBaseURL,
				MaxContextTokens: DefaultMaxContextToken,
				MaxToolCalls:     MaxToolCalls,
				ReasoningEffort:  DefaultReasoningEffort,
				GitHubToken:      os.Getenv("GITHUB_TOKEN"),
				GitLabToken:      os.Getenv("GITLAB_TOKEN"),
				GitLabBaseURL:    getEnvOrDefault("GITLAB_BASE_URL", "https://gitlab.com/api/v4"),
			},
		},
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
	applyEnv(cfg, activeProfile)

	profile, err := ResolveProfile(cfg, activeProfile)
	if err != nil {
		return nil, Profile{}, err
	}
	profile = applyOverrides(profile, overrides)
	cfg.ActiveProfile = activeProfile
	cfg.Profiles[activeProfile] = profile
	return cfg, profile, nil
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
	var fileCfg Config
	if err := yaml.Unmarshal([]byte(expanded), &fileCfg); err != nil {
		return false, fmt.Errorf("config: parsing %s: %w", path, err)
	}
	if err := markConfiguredFields(data, &fileCfg); err != nil {
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
	if value := os.Getenv("MITTWALD_LLM_API_KEY"); value != "" {
		profile.APIKey = value
	} else if value := os.Getenv("NICKPIT_API_KEY"); value != "" {
		profile.APIKey = value
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

func applyOverrides(profile Profile, overrides Overrides) Profile {
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
	if overrides.MaxContextTokens > 0 {
		profile.MaxContextTokens = overrides.MaxContextTokens
	}
	if overrides.ToolCalls > 0 {
		profile.MaxToolCalls = overrides.ToolCalls
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

func normalizeProfile(profile Profile) Profile {
	if profile.Model == "" {
		profile.Model = DefaultModel
	}
	if profile.BaseURL == "" {
		profile.BaseURL = DefaultBaseURL
	}
	if profile.MaxContextTokens == 0 {
		profile.MaxContextTokens = DefaultMaxContextToken
	}
	if profile.MaxToolCalls == 0 {
		profile.MaxToolCalls = MaxToolCalls
	}
	if profile.Workdir != "" {
		profile.Workdir = expandPath(profile.Workdir)
	}
	if profile.GitLabBaseURL == "" {
		profile.GitLabBaseURL = "https://gitlab.com/api/v4"
	}
	return profile
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

func getEnvOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func markConfiguredFields(data []byte, cfg *Config) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}
	if len(root.Content) == 0 {
		return nil
	}

	profiles := mappingValue(root.Content[0], "profiles")
	if profiles == nil || profiles.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(profiles.Content); i += 2 {
		name := profiles.Content[i].Value
		profileNode := profiles.Content[i+1]
		profile := cfg.Profiles[name]
		profile.APIKeyConfigured = mappingValue(profileNode, "api_key") != nil
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
