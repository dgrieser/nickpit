package config

import "fmt"

func ResolveProfile(cfg *Config, name string) (Profile, error) {
	if cfg == nil {
		return Profile{}, fmt.Errorf("config: nil config")
	}
	profile, ok := cfg.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("config: profile %q not found", name)
	}
	return normalizeProfile(profile), nil
}

func mergeProfiles(base, override Profile) Profile {
	if override.Model != "" {
		base.Model = override.Model
	}
	if override.BaseURL != "" {
		base.BaseURL = override.BaseURL
	}
	if override.APIKeyConfigured {
		base.APIKeyConfigured = true
		base.APIKey = override.APIKey
	} else if override.APIKey != "" {
		base.APIKey = override.APIKey
	}
	if override.MaxTokens != nil {
		base.MaxTokens = override.MaxTokens
	}
	if override.Temperature != nil {
		base.Temperature = override.Temperature
	}
	if override.UseJSONSchema {
		base.UseJSONSchema = true
	}
	if override.MaxContextTokens != 0 {
		base.MaxContextTokens = override.MaxContextTokens
	}
	if override.DefaultFollowUps != 0 {
		base.DefaultFollowUps = override.DefaultFollowUps
	}
	if override.ReasoningEffort != "" {
		base.ReasoningEffort = override.ReasoningEffort
	}
	if override.GitHubToken != "" {
		base.GitHubToken = override.GitHubToken
	}
	if override.GitLabToken != "" {
		base.GitLabToken = override.GitLabToken
	}
	if override.GitLabBaseURL != "" {
		base.GitLabBaseURL = override.GitLabBaseURL
	}
	return normalizeProfile(base)
}
