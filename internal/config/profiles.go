package config

import (
	"fmt"
	"slices"
)

func ResolveProfile(cfg *Config, name string) (Profile, error) {
	if cfg == nil {
		return Profile{}, fmt.Errorf("config: nil config")
	}
	profile, ok := cfg.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("config: profile %q not found", name)
	}
	return profile, nil
}

func mergeProfiles(base, override Profile) Profile {
	if override.Model != "" {
		base.Model = override.Model
	}
	base.Small = mergeSmallModelConfig(base.Small, override.Small)
	if override.BaseURL != "" {
		base.BaseURL = override.BaseURL
	}
	if override.APIKeyConfigured {
		base.APIKeyConfigured = true
		base.APIKey = override.APIKey
	} else if override.APIKey != "" {
		base.APIKey = override.APIKey
	}
	if override.SupportedModels != nil {
		base.SupportedModels = cloneSupportedModels(override.SupportedModels)
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
	if override.UseJSONSchema {
		base.UseJSONSchema = true
	}
	if override.IncludePaths != nil {
		base.IncludePaths = slices.Clone(override.IncludePaths)
	}
	if override.ExcludePaths != nil {
		base.ExcludePaths = slices.Clone(override.ExcludePaths)
	}
	if override.IncludeContent != nil {
		base.IncludeContent = slices.Clone(override.IncludeContent)
	}
	if override.ExcludeContent != nil {
		base.ExcludeContent = slices.Clone(override.ExcludeContent)
	}
	if override.MaxContextTokensConfigured {
		base.MaxContextTokensConfigured = true
		base.MaxContextTokens = override.MaxContextTokens
	} else if override.MaxContextTokens != 0 {
		base.MaxContextTokens = override.MaxContextTokens
	}
	if override.MaxToolCallsConfigured {
		base.MaxToolCallsConfigured = true
		base.MaxToolCalls = override.MaxToolCalls
	} else if override.MaxToolCalls != 0 {
		base.MaxToolCalls = override.MaxToolCalls
	}
	if override.MaxDuplicateToolCallsConfigured {
		base.MaxDuplicateToolCallsConfigured = true
		base.MaxDuplicateToolCalls = override.MaxDuplicateToolCalls
	} else if override.MaxDuplicateToolCalls != 0 {
		base.MaxDuplicateToolCalls = override.MaxDuplicateToolCalls
	}
	if override.MaxOutputRetriesConfigured {
		base.MaxOutputRetriesConfigured = true
		base.MaxOutputRetries = override.MaxOutputRetries
	} else if override.MaxOutputRetries != 0 {
		base.MaxOutputRetries = override.MaxOutputRetries
	}
	if override.MaxReasoningSecondsConfigured {
		base.MaxReasoningSecondsConfigured = true
		base.MaxReasoningSeconds = override.MaxReasoningSeconds
	} else if override.MaxReasoningSeconds != 0 {
		base.MaxReasoningSeconds = override.MaxReasoningSeconds
	}
	if override.MaxReasoningLoopRepeatsConfigured {
		base.MaxReasoningLoopRepeatsConfigured = true
		base.MaxReasoningLoopRepeats = override.MaxReasoningLoopRepeats
	} else if override.MaxReasoningLoopRepeats != 0 {
		base.MaxReasoningLoopRepeats = override.MaxReasoningLoopRepeats
	}
	if override.MaxRateLimitDelaySecondsConfigured {
		base.MaxRateLimitDelaySecondsConfigured = true
		base.MaxRateLimitDelaySeconds = override.MaxRateLimitDelaySeconds
	} else if override.MaxRateLimitDelaySeconds != 0 {
		base.MaxRateLimitDelaySeconds = override.MaxRateLimitDelaySeconds
	}
	if override.NudgeCountConfigured {
		base.NudgeCountConfigured = true
		base.NudgeCount = override.NudgeCount
	} else if override.NudgeCount != 0 {
		base.NudgeCount = override.NudgeCount
	}
	if override.DisablePatchSummary {
		base.DisablePatchSummary = true
	}
	if override.SkipSuggestions {
		base.SkipSuggestions = true
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
	if override.AssetBaseURL != "" {
		base.AssetBaseURL = override.AssetBaseURL
	}
	return base
}
