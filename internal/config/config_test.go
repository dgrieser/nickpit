package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigUsesProviderDefaults(t *testing.T) {
	cfg := DefaultConfig()
	profile := cfg.Profiles[DefaultProfileName]

	if profile.Model != "gpt-oss-120b" {
		t.Fatalf("model = %q", profile.Model)
	}
	if profile.BaseURL != "https://llm.aihosting.mittwald.de/v1" {
		t.Fatalf("base url = %q", profile.BaseURL)
	}
	if profile.DefaultToolRounds != 0 {
		t.Fatalf("default tool rounds = %d", profile.DefaultToolRounds)
	}
}

func TestLoadConfigUsesOpenRouterAPIKeyEnv(t *testing.T) {
	t.Setenv("MITTWALD_LLM_API_KEY", "from-mittwald-env")

	_, profile, err := Load("", Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.APIKey != "from-mittwald-env" {
		t.Fatalf("api key = %q", profile.APIKey)
	}
}

func TestLoadConfigTracksEmptyConfiguredAPIKey(t *testing.T) {
	t.Setenv("MITTWALD_LLM_API_KEY", "")
	t.Setenv("NICKPIT_API_KEY", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
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
    local_repo: ~/repo
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("NICKPIT_MODEL", "from-env")
	t.Setenv("NICKPIT_LOCAL_REPO", "/env/repo")
	cfg, profile, err := Load(path, Overrides{
		Profile:          "work",
		BaseURL:          "https://override.invalid/v1",
		MaxContextTokens: 777,
		LocalRepo:        "/override/repo",
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
	if profile.LocalRepo != "/override/repo" {
		t.Fatalf("local repo = %q", profile.LocalRepo)
	}
}

func TestLoadConfigUseJSONSchemaOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
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
	_, profile, err := Load("", Overrides{UseJSONSchema: true})
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

func TestLoadConfigMaxTokensFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
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

func TestLoadConfigToolRoundsFromFileAndOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    default_tool_rounds: 2
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.DefaultToolRounds != 2 {
		t.Fatalf("default tool rounds = %d", profile.DefaultToolRounds)
	}

	_, profile, err = Load(path, Overrides{ToolRounds: 4})
	if err != nil {
		t.Fatal(err)
	}
	if profile.DefaultToolRounds != 4 {
		t.Fatalf("override default tool rounds = %d", profile.DefaultToolRounds)
	}
}
