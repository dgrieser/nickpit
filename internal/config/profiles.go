package config

import (
	"fmt"
	"slices"

	"github.com/dgrieser/nickpit/internal/model"
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
		overrideProfileBaseURL(&base, override.BaseURL)
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
	if override.MinP != nil {
		base.MinP = override.MinP
	}
	if override.PresencePenalty != nil {
		base.PresencePenalty = override.PresencePenalty
	}
	if override.RepetitionPenalty != nil {
		base.RepetitionPenalty = override.RepetitionPenalty
	}
	if override.ExtraBody != nil {
		base.ExtraBody = override.ExtraBody
	}
	if override.DisableJSONResponseFormat {
		base.DisableJSONResponseFormat = true
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
	if override.StyleGuides != nil {
		base.StyleGuides = slices.Clone(override.StyleGuides)
	}
	if override.DisableStyleGuides != nil {
		base.DisableStyleGuides = slices.Clone(override.DisableStyleGuides)
	}
	if override.ProjectContext != nil {
		base.ProjectContext = slices.Clone(override.ProjectContext)
	}
	if override.DisableProjectContext {
		base.DisableProjectContext = true
	}
	if override.DiffFormat != "" {
		base.DiffFormat = override.DiffFormat
	}
	if override.MaxContextTokensConfigured {
		base.MaxContextTokensConfigured = true
		base.MaxContextTokens = override.MaxContextTokens
	} else if override.MaxContextTokens != 0 {
		base.MaxContextTokens = override.MaxContextTokens
	}
	if override.MaxRequestBytesConfigured {
		base.MaxRequestBytesConfigured = true
		base.MaxRequestBytes = override.MaxRequestBytes
	} else if override.MaxRequestBytes != 0 {
		base.MaxRequestBytes = override.MaxRequestBytes
	}
	if override.MaxToolResultPercentConfigured {
		base.MaxToolResultPercentConfigured = true
		base.MaxToolResultPercent = override.MaxToolResultPercent
	} else if override.MaxToolResultPercent != 0 {
		base.MaxToolResultPercent = override.MaxToolResultPercent
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
	if override.ForceAllNudges {
		base.ForceAllNudges = true
	}
	if override.MaxFindingsConfigured {
		base.MaxFindingsConfigured = true
		base.MaxFindings = override.MaxFindings
	} else if override.MaxFindings != 0 {
		base.MaxFindings = override.MaxFindings
	}
	if override.MaxSessionsConfigured {
		base.MaxSessionsConfigured = true
		base.MaxSessions = override.MaxSessions
	} else if override.MaxSessions != 0 {
		base.MaxSessions = override.MaxSessions
	}
	if override.DisablePatchSummary {
		base.DisablePatchSummary = true
	}
	if override.DisableSuggestions {
		base.DisableSuggestions = true
	}
	if override.DisableWorkflowTimeBudget {
		base.DisableWorkflowTimeBudget = true
	}
	if override.TimeBudgetScaleConfigured {
		base.TimeBudgetScaleConfigured = true
		base.TimeBudgetScale = override.TimeBudgetScale
	} else if override.TimeBudgetScale != 0 {
		base.TimeBudgetScale = override.TimeBudgetScale
	}
	if override.ReasoningEffort != "" {
		base.ReasoningEffort = override.ReasoningEffort
	}
	if override.Workdir != "" {
		base.Workdir = override.Workdir
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

// overrideProfileBaseURL keeps pre-declared model capabilities tied to the
// endpoint they describe. A different endpoint must be probed or provide its
// own declarations instead of inheriting capabilities by model name alone.
func overrideProfileBaseURL(profile *Profile, baseURL string) {
	if !model.SameEndpoint(profile.BaseURL, baseURL) {
		profile.SupportedModels = nil
	}
	profile.BaseURL = baseURL
}
