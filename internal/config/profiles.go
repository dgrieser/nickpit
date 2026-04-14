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
	if override.APIKey != "" {
		base.APIKey = override.APIKey
	}
	if override.MaxContextTokens != 0 {
		base.MaxContextTokens = override.MaxContextTokens
	}
	if override.DefaultFollowUps != 0 {
		base.DefaultFollowUps = override.DefaultFollowUps
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
	if override.PromptFile != "" {
		base.PromptFile = override.PromptFile
	}
	return normalizeProfile(base)
}
