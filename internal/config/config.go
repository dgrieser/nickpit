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
	DefaultModel           = "gpt-4o"
	DefaultBaseURL         = "https://api.openai.com/v1"
	DefaultMaxContextToken = 120000
	DefaultFollowUps       = 1
	DefaultConfigPath      = ".llm-review.yaml"
)

type Config struct {
	ActiveProfile string             `yaml:"active_profile"`
	Profiles      map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	Model            string `yaml:"model"`
	BaseURL          string `yaml:"base_url"`
	APIKey           string `yaml:"api_key"`
	MaxContextTokens int    `yaml:"max_context_tokens"`
	DefaultFollowUps int    `yaml:"default_followups"`
	GitHubToken      string `yaml:"github_token"`
	GitLabToken      string `yaml:"gitlab_token"`
	GitLabBaseURL    string `yaml:"gitlab_base_url"`
	PromptFile       string `yaml:"prompt_file"`
}

type Overrides struct {
	Profile          string
	Model            string
	BaseURL          string
	APIKey           string
	MaxContextTokens int
	FollowUps        int
	GitHubToken      string
	GitLabToken      string
	GitLabBaseURL    string
	PromptFile       string
}

func DefaultConfig() *Config {
	return &Config{
		ActiveProfile: DefaultProfileName,
		Profiles: map[string]Profile{
			DefaultProfileName: {
				Model:            DefaultModel,
				BaseURL:          DefaultBaseURL,
				MaxContextTokens: DefaultMaxContextToken,
				DefaultFollowUps: DefaultFollowUps,
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

	if err := loadFile(cfg, path); err != nil {
		return nil, Profile{}, err
	}
	applyEnv(cfg)

	activeProfile := cfg.ActiveProfile
	if overrides.Profile != "" {
		activeProfile = overrides.Profile
	}
	if activeProfile == "" {
		activeProfile = DefaultProfileName
	}

	profile, err := ResolveProfile(cfg, activeProfile)
	if err != nil {
		return nil, Profile{}, err
	}
	profile = applyOverrides(profile, overrides)
	cfg.ActiveProfile = activeProfile
	cfg.Profiles[activeProfile] = profile
	return cfg, profile, nil
}

func loadFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("config: reading %s: %w", path, err)
	}

	expanded := os.ExpandEnv(string(data))
	var fileCfg Config
	if err := yaml.Unmarshal([]byte(expanded), &fileCfg); err != nil {
		return fmt.Errorf("config: parsing %s: %w", path, err)
	}

	if fileCfg.ActiveProfile != "" {
		cfg.ActiveProfile = fileCfg.ActiveProfile
	}
	for name, profile := range fileCfg.Profiles {
		base := cfg.Profiles[name]
		cfg.Profiles[name] = mergeProfiles(base, profile)
	}
	return nil
}

func applyEnv(cfg *Config) {
	profile := cfg.Profiles[cfg.ActiveProfile]
	if value := os.Getenv("LLM_REVIEW_MODEL"); value != "" {
		profile.Model = value
	}
	if value := os.Getenv("LLM_REVIEW_BASE_URL"); value != "" {
		profile.BaseURL = value
	}
	if value := os.Getenv("LLM_REVIEW_API_KEY"); value != "" {
		profile.APIKey = value
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
	cfg.Profiles[cfg.ActiveProfile] = profile
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
	if overrides.MaxContextTokens > 0 {
		profile.MaxContextTokens = overrides.MaxContextTokens
	}
	if overrides.FollowUps >= 0 {
		profile.DefaultFollowUps = overrides.FollowUps
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
	if overrides.PromptFile != "" {
		profile.PromptFile = overrides.PromptFile
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
	if profile.DefaultFollowUps == 0 {
		profile.DefaultFollowUps = DefaultFollowUps
	}
	if profile.GitLabBaseURL == "" {
		profile.GitLabBaseURL = "https://gitlab.com/api/v4"
	}
	if profile.PromptFile != "" {
		profile.PromptFile = expandPath(profile.PromptFile)
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
