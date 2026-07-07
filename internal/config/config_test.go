package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

func intPtr(v int) *int {
	return &v
}

// sgSources joins the Source of each styleguide spec for concise assertions.
func sgSources(specs []model.StyleGuideSpec) string {
	sources := make([]string, len(specs))
	for i, spec := range specs {
		sources[i] = spec.Source
	}
	return strings.Join(sources, ",")
}

func TestDefaultConfigUsesProviderDefaults(t *testing.T) {
	cfg := DefaultConfig()
	profile := cfg.Profiles[DefaultProfileName]

	if profile.Model != "" {
		t.Fatalf("default profile model should be empty, got %q", profile.Model)
	}
	if profile.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("base url = %q", profile.BaseURL)
	}
	if profile.MaxToolCalls != 0 {
		t.Fatalf("default max tool calls = %d", profile.MaxToolCalls)
	}
	if profile.MaxDuplicateToolCalls != 0 {
		t.Fatalf("default max duplicate tool calls = %d", profile.MaxDuplicateToolCalls)
	}
	if profile.MaxOutputRetries != 0 {
		t.Fatalf("default max output retries = %d", profile.MaxOutputRetries)
	}
	if profile.MaxReasoningSeconds != 0 {
		t.Fatalf("default max reasoning seconds = %d", profile.MaxReasoningSeconds)
	}
	if profile.MaxRateLimitDelaySeconds != 0 {
		t.Fatalf("default max rate limit delay seconds = %d", profile.MaxRateLimitDelaySeconds)
	}
	if profile.NudgeCount != 0 {
		t.Fatalf("default nudge count = %d", profile.NudgeCount)
	}
	if profile.APIKey != "$OPENROUTER_API_KEY" {
		t.Fatalf("default api key ref = %q", profile.APIKey)
	}
	if profile.GitHubToken != "" {
		t.Fatalf("default github token ref = %q", profile.GitHubToken)
	}
	if profile.GitLabToken != "" {
		t.Fatalf("default gitlab token ref = %q", profile.GitLabToken)
	}
	if profile.GitLabBaseURL != "" {
		t.Fatalf("default gitlab base url ref = %q", profile.GitLabBaseURL)
	}

	mittwald := cfg.Profiles["mittwald"]
	if mittwald.BaseURL != "https://llm.aihosting.mittwald.de/v1" {
		t.Fatalf("mittwald base url = %q", mittwald.BaseURL)
	}
	if mittwald.APIKey != "$MITTWALD_LLM_API_KEY" {
		t.Fatalf("mittwald api key ref = %q", mittwald.APIKey)
	}
	mistral := cfg.Profiles["mistral"]
	if mistral.APIKey != "$MISTRAL_API_KEY" {
		t.Fatalf("mistral api key ref = %q", mistral.APIKey)
	}
}

func TestLoadConfigUsesOpenRouterAPIKeyEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "from-openrouter-env")
	t.Setenv("NICKPIT_API_KEY", "from-generic-env")
	t.Setenv("NICKPIT_MODEL", "test-model")

	_, profile, err := Load("", Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.APIKey != "from-openrouter-env" {
		t.Fatalf("api key = %q", profile.APIKey)
	}
	if profile.ReasoningEffort != DefaultReasoningEffort {
		t.Fatalf("reasoning effort = %q", profile.ReasoningEffort)
	}
	if profile.MaxRateLimitDelaySeconds != DefaultMaxRateLimitDelaySeconds {
		t.Fatalf("max rate limit delay seconds = %d", profile.MaxRateLimitDelaySeconds)
	}
	if profile.NudgeCount != DefaultNudgeCount {
		t.Fatalf("nudge count = %d", profile.NudgeCount)
	}
}

func TestLoadConfigUsesSmallModelEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "from-openrouter-env")
	t.Setenv("NICKPIT_MODEL", "primary-model")
	t.Setenv("NICKPIT_SMALL_MODEL", "small-model")
	t.Setenv("NICKPIT_SMALL_REASONING_EFFORT", "low")
	t.Setenv("NICKPIT_SMALL_MAX_TOKENS", "2048")
	t.Setenv("NICKPIT_SMALL_TEMPERATURE", "0.25")
	t.Setenv("NICKPIT_SMALL_TOP_P", "0.85")
	t.Setenv("NICKPIT_SMALL_TOP_K", "40")
	t.Setenv("NICKPIT_SMALL_PRESENCE_PENALTY", "0.1")
	t.Setenv("NICKPIT_SMALL_EXTRA_BODY", `{"chat_template_kwargs":{"enable_thinking":false}}`)

	_, profile, err := Load("", Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Small.Model != "small-model" {
		t.Fatalf("small model = %q", profile.Small.Model)
	}
	if profile.Small.ReasoningEffort != "low" {
		t.Fatalf("small reasoning effort = %q", profile.Small.ReasoningEffort)
	}
	if profile.Small.MaxTokens == nil || *profile.Small.MaxTokens != 2048 {
		t.Fatalf("small max tokens = %v", profile.Small.MaxTokens)
	}
	if profile.Small.Temperature == nil || *profile.Small.Temperature != 0.25 {
		t.Fatalf("small temperature = %v", profile.Small.Temperature)
	}
	if profile.Small.TopP == nil || *profile.Small.TopP != 0.85 {
		t.Fatalf("small top_p = %v", profile.Small.TopP)
	}
	if profile.Small.TopK == nil || *profile.Small.TopK != 40 {
		t.Fatalf("small top_k = %v", profile.Small.TopK)
	}
	if profile.Small.PresencePenalty == nil || *profile.Small.PresencePenalty != 0.1 {
		t.Fatalf("small presence penalty = %v", profile.Small.PresencePenalty)
	}
	chatTemplateKwargs, ok := profile.Small.ExtraBody["chat_template_kwargs"].(map[string]any)
	if !ok || chatTemplateKwargs["enable_thinking"] != false {
		t.Fatalf("small extra body = %#v", profile.Small.ExtraBody)
	}
}

func TestLoadConfigUsesConfiguredRateLimitDelay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_rate_limit_delay_seconds: 12
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxRateLimitDelaySeconds != 12 {
		t.Fatalf("max rate limit delay seconds = %d", profile.MaxRateLimitDelaySeconds)
	}
	if !profile.MaxRateLimitDelaySecondsConfigured {
		t.Fatal("expected max_rate_limit_delay_seconds to be marked as configured")
	}
}

func TestLoadConfigUsesConfiguredNudgeCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    nudge_count: 0
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.NudgeCount != 0 {
		t.Fatalf("nudge count = %d", profile.NudgeCount)
	}
	if !profile.NudgeCountConfigured {
		t.Fatal("expected nudge_count to be marked as configured")
	}
}

func TestLoadConfigUsesConfiguredSmallModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: primary-model
    reasoning_effort: high
    small:
      model: small-model
      reasoning_effort: low
      max_tokens: 2048
      temperature: 0.25
      top_p: 0.85
      top_k: 40
      presence_penalty: 0.1
      extra_body:
        chat_template_kwargs:
          enable_thinking: false
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Small.Model != "small-model" {
		t.Fatalf("small model = %q", profile.Small.Model)
	}
	if profile.Small.ReasoningEffort != "low" {
		t.Fatalf("small reasoning effort = %q", profile.Small.ReasoningEffort)
	}
	if profile.Small.MaxTokens == nil || *profile.Small.MaxTokens != 2048 {
		t.Fatalf("small max tokens = %v", profile.Small.MaxTokens)
	}
	if profile.Small.Temperature == nil || *profile.Small.Temperature != 0.25 {
		t.Fatalf("small temperature = %v", profile.Small.Temperature)
	}
	if profile.Small.TopP == nil || *profile.Small.TopP != 0.85 {
		t.Fatalf("small top_p = %v", profile.Small.TopP)
	}
	if profile.Small.TopK == nil || *profile.Small.TopK != 40 {
		t.Fatalf("small top_k = %v", profile.Small.TopK)
	}
	if profile.Small.PresencePenalty == nil || *profile.Small.PresencePenalty != 0.1 {
		t.Fatalf("small presence penalty = %v", profile.Small.PresencePenalty)
	}
	chatTemplateKwargs, ok := profile.Small.ExtraBody["chat_template_kwargs"].(map[string]any)
	if !ok || chatTemplateKwargs["enable_thinking"] != false {
		t.Fatalf("small extra body = %#v", profile.Small.ExtraBody)
	}
}

func TestLoadConfigDisablePatchSummary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    disable_patch_summary: true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.DisablePatchSummary {
		t.Fatal("expected disable_patch_summary to be enabled")
	}
}

func TestLoadConfigDisableSuggestions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    disable_suggestions: true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.DisableSuggestions {
		t.Fatal("expected disable_suggestions to be enabled")
	}
}

func TestLoadConfigDisableWorkflowTimeBudget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    disable_workflow_time_budget: true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.DisableWorkflowTimeBudget {
		t.Fatal("expected disable_workflow_time_budget to be enabled")
	}
}

func TestLoadConfigTracksEmptyConfiguredAPIKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("NICKPIT_API_KEY", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    api_key:
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.APIKeyConfigured {
		t.Fatal("expected api_key to be marked as configured")
	}
	if profile.APIKey != "" {
		t.Fatalf("api key = %q", profile.APIKey)
	}
}

func TestLoadConfigDefaultProfileFallsBackToGenericAPIKeyEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("NICKPIT_API_KEY", "generic-key")
	t.Setenv("NICKPIT_MODEL", "test-model")

	_, profile, err := Load("", Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	// The default profile's $OPENROUTER_API_KEY reference resolves to empty,
	// so the generic NICKPIT_API_KEY is used as the last resort.
	if profile.APIKey != "generic-key" {
		t.Fatalf("api key = %q, want generic-key", profile.APIKey)
	}
}

func TestLoadConfigGenericAPIKeyEnvDoesNotOverrideConfiguredKey(t *testing.T) {
	t.Setenv("NICKPIT_API_KEY", "generic-key")
	t.Setenv("NICKPIT_MODEL", "test-model")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    api_key: configured-key
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.APIKey != "configured-key" {
		t.Fatalf("api key = %q, want configured-key", profile.APIKey)
	}
}

func TestLoadConfigUsesPrimaryModelEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "from-openrouter-env")
	t.Setenv("NICKPIT_MODEL", "primary-model")
	t.Setenv("NICKPIT_REASONING_EFFORT", "low")
	t.Setenv("NICKPIT_MAX_TOKENS", "4096")
	t.Setenv("NICKPIT_TEMPERATURE", "0.25")
	t.Setenv("NICKPIT_TOP_P", "0.85")
	t.Setenv("NICKPIT_EXTRA_BODY", `{"chat_template_kwargs":{"enable_thinking":false}}`)

	_, profile, err := Load("", Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Model != "primary-model" {
		t.Fatalf("model = %q", profile.Model)
	}
	if profile.ReasoningEffort != "low" {
		t.Fatalf("reasoning effort = %q", profile.ReasoningEffort)
	}
	if profile.MaxTokens == nil || *profile.MaxTokens != 4096 {
		t.Fatalf("max tokens = %v", profile.MaxTokens)
	}
	if profile.Temperature == nil || *profile.Temperature != 0.25 {
		t.Fatalf("temperature = %v", profile.Temperature)
	}
	if profile.TopP == nil || *profile.TopP != 0.85 {
		t.Fatalf("top_p = %v", profile.TopP)
	}
	kwargs, ok := profile.ExtraBody["chat_template_kwargs"].(map[string]any)
	if !ok || kwargs["enable_thinking"] != false {
		t.Fatalf("extra body = %#v", profile.ExtraBody)
	}
}

func TestLoadConfigRejectsInvalidPrimaryModelEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "from-openrouter-env")
	t.Setenv("NICKPIT_MODEL", "primary-model")
	t.Setenv("NICKPIT_TEMPERATURE", "not-a-number")

	if _, _, err := Load("", Overrides{}); err == nil {
		t.Fatal("expected error for non-numeric NICKPIT_TEMPERATURE")
	}
}

func TestLoadConfigPrefersNickpitSCMEnv(t *testing.T) {
	t.Setenv("NICKPIT_MODEL", "test-model")
	t.Setenv("GITHUB_TOKEN", "bare-github")
	t.Setenv("NICKPIT_GITHUB_TOKEN", "prefixed-github")
	t.Setenv("GITLAB_TOKEN", "bare-gitlab")
	t.Setenv("NICKPIT_GITLAB_TOKEN", "prefixed-gitlab")
	t.Setenv("GITLAB_BASE_URL", "https://bare.gitlab.invalid/api/v4")
	t.Setenv("NICKPIT_GITLAB_BASE_URL", "https://prefixed.gitlab.invalid/api/v4")

	_, profile, err := Load("", Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.GitHubToken != "prefixed-github" {
		t.Fatalf("github token = %q", profile.GitHubToken)
	}
	if profile.GitLabToken != "prefixed-gitlab" {
		t.Fatalf("gitlab token = %q", profile.GitLabToken)
	}
	if profile.GitLabBaseURL != "https://prefixed.gitlab.invalid/api/v4" {
		t.Fatalf("gitlab base url = %q", profile.GitLabBaseURL)
	}
}

func TestLoadConfigUsesBareSCMEnvFallback(t *testing.T) {
	t.Setenv("NICKPIT_MODEL", "test-model")
	t.Setenv("NICKPIT_GITHUB_TOKEN", "")
	t.Setenv("NICKPIT_GITLAB_TOKEN", "")
	t.Setenv("NICKPIT_GITLAB_BASE_URL", "")
	t.Setenv("GITHUB_TOKEN", "bare-github")
	t.Setenv("GITLAB_TOKEN", "bare-gitlab")
	t.Setenv("GITLAB_BASE_URL", "https://bare.gitlab.invalid/api/v4")

	_, profile, err := Load("", Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.GitHubToken != "bare-github" {
		t.Fatalf("github token = %q", profile.GitHubToken)
	}
	if profile.GitLabToken != "bare-gitlab" {
		t.Fatalf("gitlab token = %q", profile.GitLabToken)
	}
	if profile.GitLabBaseURL != "https://bare.gitlab.invalid/api/v4" {
		t.Fatalf("gitlab base url = %q", profile.GitLabBaseURL)
	}
}

func TestLoadConfigExpandsAPIKeyEnvReferenceFromYAML(t *testing.T) {
	t.Setenv("TEST_LLM_API_KEY", "yaml-key")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    api_key: $TEST_LLM_API_KEY
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.APIKey != "yaml-key" {
		t.Fatalf("api key = %q", profile.APIKey)
	}
}

func TestLoadConfigExpandsBracedAPIKeyEnvReferenceFromCLI(t *testing.T) {
	t.Setenv("TEST_LLM_API_KEY", "cli-key")

	_, profile, err := Load("", Overrides{
		Model:  "test-model",
		APIKey: "${TEST_LLM_API_KEY}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.APIKey != "cli-key" {
		t.Fatalf("api key = %q", profile.APIKey)
	}
}

func TestLoadConfigOpenRouterProfileFallsBackToDefault(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "openrouter-key")

	cfg, profile, err := Load("", Overrides{
		Profile: "openrouter",
		Model:   "test-model",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveProfile != "openrouter" {
		t.Fatalf("active profile = %q", cfg.ActiveProfile)
	}
	if profile.Model != "test-model" {
		t.Fatalf("model = %q", profile.Model)
	}
	if profile.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("base url = %q", profile.BaseURL)
	}
	if profile.APIKey != "openrouter-key" {
		t.Fatalf("api key = %q", profile.APIKey)
	}
}

func TestLoadConfigExplicitOpenRouterProfileWins(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "default-key")
	t.Setenv("CUSTOM_OPENROUTER_API_KEY", "custom-key")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: default-model
    base_url: https://default.invalid/v1
    api_key: $OPENROUTER_API_KEY
  openrouter:
    model: custom-model
    base_url: https://custom.invalid/v1
    api_key: $CUSTOM_OPENROUTER_API_KEY
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, profile, err := Load(path, Overrides{Profile: "openrouter"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveProfile != "openrouter" {
		t.Fatalf("active profile = %q", cfg.ActiveProfile)
	}
	if profile.Model != "custom-model" {
		t.Fatalf("model = %q", profile.Model)
	}
	if profile.BaseURL != "https://custom.invalid/v1" {
		t.Fatalf("base url = %q", profile.BaseURL)
	}
	if profile.APIKey != "custom-key" {
		t.Fatalf("api key = %q", profile.APIKey)
	}
}

func TestLoadConfigWithOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
active_profile: work
profiles:
  work:
    model: from-file
    base_url: https://example.invalid/v1
    max_context_tokens: 999
    workdir: ~/repo
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("NICKPIT_MODEL", "from-env")
	t.Setenv("NICKPIT_WORKDIR", "/env/repo")
	cfg, profile, err := Load(path, Overrides{
		Profile:          "work",
		BaseURL:          "https://override.invalid/v1",
		MaxContextTokens: intPtr(777),
		Workdir:          "/override/repo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveProfile != "work" {
		t.Fatalf("active profile = %q", cfg.ActiveProfile)
	}
	if profile.Model != "from-env" {
		t.Fatalf("model = %q", profile.Model)
	}
	if profile.BaseURL != "https://override.invalid/v1" {
		t.Fatalf("base url = %q", profile.BaseURL)
	}
	if profile.MaxContextTokens != 777 {
		t.Fatalf("max context tokens = %d", profile.MaxContextTokens)
	}
	if profile.Workdir != "/override/repo" {
		t.Fatalf("local repo = %q", profile.Workdir)
	}
}

func TestLoadConfigDisableJSONResponseFormatOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    disable_json_response_format: true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.DisableJSONResponseFormat {
		t.Fatal("expected disable_json_response_format from config to be enabled")
	}
}

func TestLoadConfigFiltersFromFileAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    include_paths: ["\\.go$"]
    exclude_paths: ["\\.pb\\.go$"]
    include_content: ["(?m)^package "]
    exclude_content: ["DO NOT EDIT"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(profile.IncludePaths, ",") != "\\.go$" {
		t.Fatalf("include paths = %#v", profile.IncludePaths)
	}
	if strings.Join(profile.ExcludePaths, ",") != "\\.pb\\.go$" {
		t.Fatalf("exclude paths = %#v", profile.ExcludePaths)
	}
	if strings.Join(profile.IncludeContent, ",") != "(?m)^package " {
		t.Fatalf("include content = %#v", profile.IncludeContent)
	}
	if strings.Join(profile.ExcludeContent, ",") != "DO NOT EDIT" {
		t.Fatalf("exclude content = %#v", profile.ExcludeContent)
	}

	includePaths := []string{"\\.ts$"}
	excludeContent := []string{"generated"}
	_, profile, err = Load(path, Overrides{IncludePaths: &includePaths, ExcludeContent: &excludeContent})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(profile.IncludePaths, ",") != "\\.ts$" {
		t.Fatalf("override include paths = %#v", profile.IncludePaths)
	}
	if strings.Join(profile.ExcludeContent, ",") != "generated" {
		t.Fatalf("override exclude content = %#v", profile.ExcludeContent)
	}
}

func TestLoadConfigStyleGuidesAppendOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    styleguides: ["a.md", "https://example.com/rules.md"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if sgSources(profile.StyleGuides) != "a.md,https://example.com/rules.md" {
		t.Fatalf("styleguides = %#v", profile.StyleGuides)
	}

	// CLI values append to the file's list (unlike the filter lists, which
	// replace); exact duplicates and empties are dropped, specs are trimmed.
	_, profile, err = Load(path, Overrides{StyleGuides: []string{" b.md ", "a.md", ""}})
	if err != nil {
		t.Fatal(err)
	}
	if sgSources(profile.StyleGuides) != "a.md,https://example.com/rules.md,b.md" {
		t.Fatalf("appended styleguides = %#v", profile.StyleGuides)
	}
}

func TestLoadConfigRejectsInvalidStyleGuideURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    styleguides: ["https:///no-host.md"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = Load(path, Overrides{})
	if err == nil || !strings.Contains(err.Error(), "styleguides[0] invalid URL") {
		t.Fatalf("error = %v, want invalid URL", err)
	}
}

func TestLoadConfigParsesStructuredStyleGuides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    styleguides:
      - team.md
      - source: go-1.19.md
        language: go
        version: "1.19"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	want := []model.StyleGuideSpec{
		{Source: "team.md"},
		{Source: "go-1.19.md", Language: "go", Version: "1.19"},
	}
	if !slices.Equal(profile.StyleGuides, want) {
		t.Fatalf("styleguides = %#v, want %#v", profile.StyleGuides, want)
	}
}

func TestLoadConfigRejectsStyleGuideVersionWithoutLanguage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    styleguides:
      - source: x.md
        version: "1.19"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path, Overrides{}); err == nil || !strings.Contains(err.Error(), "no language") {
		t.Fatalf("error = %v, want version-without-language error", err)
	}
}

func TestLoadConfigRejectsUnknownStyleGuideLanguage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    styleguides:
      - source: x.md
        language: klingon
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path, Overrides{}); err == nil || !strings.Contains(err.Error(), "unknown language") {
		t.Fatalf("error = %v, want unknown-language error", err)
	}
}

func TestMergeProfilesReplacesStyleGuides(t *testing.T) {
	base := Profile{StyleGuides: []model.StyleGuideSpec{{Source: "base.md"}}, DisableStyleGuides: []string{"go"}}
	merged := mergeProfiles(base, Profile{StyleGuides: []model.StyleGuideSpec{{Source: "override.md"}}, DisableStyleGuides: []string{"python"}})
	if sgSources(merged.StyleGuides) != "override.md" {
		t.Fatalf("merged styleguides = %#v", merged.StyleGuides)
	}
	if strings.Join(merged.DisableStyleGuides, ",") != "python" {
		t.Fatalf("merged disable styleguides = %#v", merged.DisableStyleGuides)
	}
	kept := mergeProfiles(base, Profile{})
	if sgSources(kept.StyleGuides) != "base.md" {
		t.Fatalf("kept styleguides = %#v", kept.StyleGuides)
	}
	if strings.Join(kept.DisableStyleGuides, ",") != "go" {
		t.Fatalf("kept disable styleguides = %#v", kept.DisableStyleGuides)
	}
}

func TestLoadConfigWorkdirFromFileSurvivesBuiltinMerge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// "default" exists as a built-in profile, so the file profile goes through
	// mergeProfiles; workdir must survive that merge.
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    workdir: /some/repo
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Workdir != "/some/repo" {
		t.Fatalf("workdir = %q, want /some/repo", profile.Workdir)
	}
}

func TestLoadConfigDisableStyleGuidesAppendOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    disable_styleguides: ["SQL", "python"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{DisableStyleGuides: []string{"go", "sql", ""}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(profile.DisableStyleGuides, ",") != "sql,python,go" {
		t.Fatalf("disable styleguides = %#v", profile.DisableStyleGuides)
	}
}

func TestLoadConfigDisableStyleGuidesAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    disable_styleguides: ["python"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// "all" (from either source) expands to every built-in language.
	_, profile, err := Load(path, Overrides{DisableStyleGuides: []string{"ALL"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.DisableStyleGuides) < 10 || !slices.Contains(profile.DisableStyleGuides, "go") || !slices.Contains(profile.DisableStyleGuides, "kubernetes") {
		t.Fatalf("disable styleguides = %#v, want all built-in languages", profile.DisableStyleGuides)
	}

	// Typos are still rejected even when "all" is present.
	_, _, err = Load(path, Overrides{DisableStyleGuides: []string{"all", "cobol"}})
	if err == nil || !strings.Contains(err.Error(), `unknown language "cobol"`) {
		t.Fatalf("error = %v, want unknown language despite all", err)
	}
}

func TestLoadConfigRejectsUnknownDisabledStyleGuide(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    disable_styleguides: ["cobol"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = Load(path, Overrides{})
	if err == nil || !strings.Contains(err.Error(), `disable_styleguides[0] unknown language "cobol"`) {
		t.Fatalf("error = %v, want unknown language", err)
	}
}

func TestLoadConfigRejectsInvalidFilterRegex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    include_paths: ["["]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = Load(path, Overrides{})
	if err == nil || !strings.Contains(err.Error(), "include_paths[0] invalid regex") {
		t.Fatalf("error = %v, want invalid regex", err)
	}
}

func TestLoadConfigSupportedModels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    supported_models:
      - model: test-model
        compatible: true
        response: true
        tools: true
        json_response: true
        json_schema: false
        reasoning:
          traces: true
          efforts: [high, medium]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.SupportedModels) != 1 {
		t.Fatalf("supported models = %d, want 1", len(profile.SupportedModels))
	}
	got := profile.SupportedModels[0]
	if got.Model != "test-model" || !got.Compatible || !got.Response || !got.Tools {
		t.Fatalf("supported model = %#v", got)
	}
	if got.JSONResponse == nil || !*got.JSONResponse {
		t.Fatalf("json response = %v, want true", got.JSONResponse)
	}
	if got.JSONSchema == nil || *got.JSONSchema {
		t.Fatalf("json schema = %v, want false", got.JSONSchema)
	}
	if !got.Reasoning.Traces || strings.Join(got.Reasoning.Efforts, ",") != "high,medium" {
		t.Fatalf("reasoning = %#v", got.Reasoning)
	}
}

func TestCloneProfileCopiesSupportedModels(t *testing.T) {
	jsonSchema := true
	profile := Profile{SupportedModels: []ModelCapabilities{{
		Model:      "model",
		JSONSchema: &jsonSchema,
		Reasoning:  ReasoningCapabilities{Efforts: []string{"high"}},
	}}}
	cloned := cloneProfile(profile)
	cloned.SupportedModels[0].Reasoning.Efforts[0] = "low"
	*cloned.SupportedModels[0].JSONSchema = false

	if profile.SupportedModels[0].Reasoning.Efforts[0] != "high" {
		t.Fatal("supported model efforts were not cloned")
	}
	if !*profile.SupportedModels[0].JSONSchema {
		t.Fatal("supported model json schema pointer was not cloned")
	}
}

func TestLoadConfigDisableJSONResponseFormatCLIOverride(t *testing.T) {
	_, profile, err := Load("", Overrides{DisableJSONResponseFormat: true, Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.DisableJSONResponseFormat {
		t.Fatal("expected disable_json_response_format override to be enabled")
	}
}

func TestLoadConfigDefaultsDiffFormatToGit(t *testing.T) {
	_, profile, err := Load("", Overrides{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if profile.DiffFormat != model.DiffFormatGit {
		t.Fatalf("diff format = %q", profile.DiffFormat)
	}
}

func TestLoadConfigDiffFormatFromFileAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
active_profile: custom
profiles:
  custom:
    model: test-model
    base_url: https://example.test/v1
    diff_format: git-json
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.DiffFormat != model.DiffFormatGitJson {
		t.Fatalf("diff format = %q", profile.DiffFormat)
	}

	_, profile, err = Load(path, Overrides{DiffFormat: model.DiffFormatGit})
	if err != nil {
		t.Fatal(err)
	}
	if profile.DiffFormat != model.DiffFormatGit {
		t.Fatalf("override diff format = %q", profile.DiffFormat)
	}
}

func TestLoadConfigRejectsInvalidDiffFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
active_profile: custom
profiles:
  custom:
    model: test-model
    base_url: https://example.test/v1
    diff_format: raw
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = Load(path, Overrides{})
	if err == nil || !strings.Contains(err.Error(), "diff_format") {
		t.Fatalf("err = %v, want diff_format validation error", err)
	}
}

func TestLoadConfigTemperatureFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    temperature: 0.35
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Temperature == nil {
		t.Fatal("expected temperature from config")
	}
	if *profile.Temperature != 0.35 {
		t.Fatalf("temperature = %v", *profile.Temperature)
	}
}

func TestLoadConfigAssetBaseURLFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    asset_base_url: https://badges.example.com/np/
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.AssetBaseURL != "https://badges.example.com/np/" {
		t.Fatalf("asset_base_url = %q, want configured value", profile.AssetBaseURL)
	}
}

func TestLoadConfigAssetBaseURLDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.AssetBaseURL != DefaultAssetBaseURL {
		t.Fatalf("asset_base_url = %q, want default %q", profile.AssetBaseURL, DefaultAssetBaseURL)
	}
}

func TestLoadConfigTopPFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    top_p: 0.85
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.TopP == nil {
		t.Fatal("expected top_p from config")
	}
	if *profile.TopP != 0.85 {
		t.Fatalf("top_p = %v", *profile.TopP)
	}
}

func TestLoadConfigTopKAndPresencePenaltyFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    top_k: 40
    presence_penalty: 0.1
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.TopK == nil || *profile.TopK != 40 {
		t.Fatalf("top_k = %v", profile.TopK)
	}
	if profile.PresencePenalty == nil || *profile.PresencePenalty != 0.1 {
		t.Fatalf("presence_penalty = %v", profile.PresencePenalty)
	}
}

func TestLoadConfigTopKAndPresencePenaltyFromEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "from-openrouter-env")
	t.Setenv("NICKPIT_MODEL", "test-model")
	t.Setenv("NICKPIT_TOP_K", "50")
	t.Setenv("NICKPIT_PRESENCE_PENALTY", "0.2")

	_, profile, err := Load("", Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.TopK == nil || *profile.TopK != 50 {
		t.Fatalf("top_k = %v", profile.TopK)
	}
	if profile.PresencePenalty == nil || *profile.PresencePenalty != 0.2 {
		t.Fatalf("presence_penalty = %v", profile.PresencePenalty)
	}
}

func TestLoadConfigExtraBodyFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    extra_body:
      chat_template_kwargs:
        enable_thinking: true
        clear_thinking: false
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.ExtraBody == nil {
		t.Fatal("expected extra_body from config")
	}
	chatTemplateKwargs, ok := profile.ExtraBody["chat_template_kwargs"].(map[string]any)
	if !ok {
		t.Fatalf("chat_template_kwargs = %#v", profile.ExtraBody["chat_template_kwargs"])
	}
	if chatTemplateKwargs["enable_thinking"] != true {
		t.Fatalf("enable_thinking = %v", chatTemplateKwargs["enable_thinking"])
	}
	if chatTemplateKwargs["clear_thinking"] != false {
		t.Fatalf("clear_thinking = %v", chatTemplateKwargs["clear_thinking"])
	}
}

func TestLoadConfigMaxTokensFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_tokens: 2048
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxTokens == nil {
		t.Fatal("expected max_tokens from config")
	}
	if *profile.MaxTokens != 2048 {
		t.Fatalf("max_tokens = %d", *profile.MaxTokens)
	}
}

func TestLoadConfigMaxToolCallsFromFileAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_tool_calls: 2
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxToolCalls != 2 {
		t.Fatalf("default max tool calls = %d", profile.MaxToolCalls)
	}

	_, profile, err = Load(path, Overrides{ToolCalls: intPtr(4)})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxToolCalls != 4 {
		t.Fatalf("override default max tool calls = %d", profile.MaxToolCalls)
	}
}

func TestLoadConfigMaxDuplicateToolCallsFromFileAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_duplicate_tool_calls: 2
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxDuplicateToolCalls != 2 {
		t.Fatalf("default max duplicate tool calls = %d", profile.MaxDuplicateToolCalls)
	}

	_, profile, err = Load(path, Overrides{DuplicateToolCalls: intPtr(4)})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxDuplicateToolCalls != 4 {
		t.Fatalf("override default max duplicate tool calls = %d", profile.MaxDuplicateToolCalls)
	}
}

func TestLoadConfigMaxOutputRetriesFromFileAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_output_retries: 2
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxOutputRetries != 2 {
		t.Fatalf("default max output retries = %d", profile.MaxOutputRetries)
	}

	_, profile, err = Load(path, Overrides{OutputRetries: intPtr(4)})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxOutputRetries != 4 {
		t.Fatalf("override default max output retries = %d", profile.MaxOutputRetries)
	}
}

func TestLoadConfigMaxReasoningSecondsFromFileAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_reasoning_seconds: 2
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxReasoningSeconds != 2 {
		t.Fatalf("default max reasoning seconds = %d", profile.MaxReasoningSeconds)
	}

	_, profile, err = Load(path, Overrides{ReasoningSeconds: intPtr(4)})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxReasoningSeconds != 4 {
		t.Fatalf("override default max reasoning seconds = %d", profile.MaxReasoningSeconds)
	}
}

func TestLoadConfigNudgeCountFromFileAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    nudge_count: 2
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.NudgeCount != 2 {
		t.Fatalf("nudge count = %d", profile.NudgeCount)
	}

	_, profile, err = Load(path, Overrides{NudgeCount: intPtr(4)})
	if err != nil {
		t.Fatal(err)
	}
	if profile.NudgeCount != 4 {
		t.Fatalf("override nudge count = %d", profile.NudgeCount)
	}
}

func TestLoadConfigRejectsNegativeNudgeCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    nudge_count: -1
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = Load(path, Overrides{})
	if err == nil {
		t.Fatal("expected negative nudge count error")
	}
	if got, want := err.Error(), "nudge_count must be non-negative"; !strings.Contains(got, want) {
		t.Fatalf("error = %q, want containing %q", got, want)
	}
}

func TestLoadConfigMaxFindingsFromFileAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_findings: 10
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxFindings != 10 {
		t.Fatalf("max findings = %d", profile.MaxFindings)
	}
	if !profile.MaxFindingsConfigured {
		t.Fatal("expected max_findings to be marked as configured")
	}

	_, profile, err = Load(path, Overrides{MaxFindings: intPtr(4)})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxFindings != 4 {
		t.Fatalf("override max findings = %d", profile.MaxFindings)
	}

	// Explicit zero from the CLI must win over the config file (unlimited).
	_, profile, err = Load(path, Overrides{MaxFindings: intPtr(0)})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxFindings != 0 {
		t.Fatalf("explicit zero max findings = %d", profile.MaxFindings)
	}
}

func TestLoadConfigMaxFindingsDefaultsToUnlimited(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxFindings != 0 {
		t.Fatalf("default max findings = %d, want 0 (unlimited)", profile.MaxFindings)
	}
	if profile.MaxFindingsConfigured {
		t.Fatal("unset max_findings marked as configured")
	}
}

func TestLoadConfigRejectsNegativeMaxFindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_findings: -1
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = Load(path, Overrides{})
	if err == nil {
		t.Fatal("expected negative max findings error")
	}
	if got, want := err.Error(), "max_findings must be non-negative"; !strings.Contains(got, want) {
		t.Fatalf("error = %q, want containing %q", got, want)
	}
}

func TestLoadConfigExplicitZeroToolCallOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_tool_calls: 2
    max_duplicate_tool_calls: 3
    max_output_retries: 4
    max_reasoning_seconds: 5
    nudge_count: 7
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{
		ToolCalls:          intPtr(0),
		DuplicateToolCalls: intPtr(0),
		OutputRetries:      intPtr(0),
		ReasoningSeconds:   intPtr(0),
		NudgeCount:         intPtr(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxToolCalls != 0 {
		t.Fatalf("max tool calls = %d", profile.MaxToolCalls)
	}
	if profile.MaxDuplicateToolCalls != 0 {
		t.Fatalf("max duplicate tool calls = %d", profile.MaxDuplicateToolCalls)
	}
	if profile.MaxOutputRetries != 0 {
		t.Fatalf("max output retries = %d", profile.MaxOutputRetries)
	}
	if profile.MaxReasoningSeconds != 0 {
		t.Fatalf("max reasoning seconds = %d", profile.MaxReasoningSeconds)
	}
	if profile.NudgeCount != 0 {
		t.Fatalf("nudge count = %d", profile.NudgeCount)
	}
}

func TestLoadConfigExplicitZeroMaxContextTokensOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_context_tokens: 1234
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{
		MaxContextTokens: intPtr(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxContextTokens != 0 {
		t.Fatalf("max context tokens = %d", profile.MaxContextTokens)
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	if got := expandPath("~"); got != home {
		t.Fatalf("expandPath(~) = %q, want %q", got, home)
	}
	if got, want := expandPath("~/work"), filepath.Join(home, "work"); got != want {
		t.Fatalf("expandPath(~/work) = %q, want %q", got, want)
	}
	// ~user paths cannot be expanded against the current home; leave untouched.
	if got := expandPath("~otheruser/work"); got != "~otheruser/work" {
		t.Fatalf("expandPath(~otheruser/work) = %q, want unchanged", got)
	}
	if got := expandPath("/absolute/path"); got != "/absolute/path" {
		t.Fatalf("expandPath(/absolute/path) = %q, want unchanged", got)
	}
	if got := expandPath(""); got != "" {
		t.Fatalf("expandPath(empty) = %q, want empty", got)
	}
}
