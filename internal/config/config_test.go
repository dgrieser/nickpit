package config

import (
	"os"
	"path/filepath"
	"testing"
)

func intPtr(v int) *int {
	return &v
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
	if mittwald.Model != "gpt-oss-120b" {
		t.Fatalf("mittwald model = %q", mittwald.Model)
	}
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
	if profile.APIKey != "" {
		t.Fatalf("api key = %q", profile.APIKey)
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

func TestLoadConfigUseJSONSchemaOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    use_json_schema: true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.UseJSONSchema {
		t.Fatal("expected use_json_schema from config to be enabled")
	}
}

func TestLoadConfigUseJSONSchemaCLIOverride(t *testing.T) {
	_, profile, err := Load("", Overrides{UseJSONSchema: true, Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.UseJSONSchema {
		t.Fatal("expected use_json_schema override to be enabled")
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

func TestLoadConfigExplicitZeroToolCallOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_tool_calls: 2
    max_duplicate_tool_calls: 3
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{
		ToolCalls:          intPtr(0),
		DuplicateToolCalls: intPtr(0),
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
