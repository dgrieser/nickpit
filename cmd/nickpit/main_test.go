package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfileRespectsExplicitZeroToolCallOverrides(t *testing.T) {
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

	app := &app{
		profile:                  "default",
		configPath:               path,
		maxToolCalls:             0,
		maxToolCallsSet:          true,
		maxDuplicateToolCalls:    0,
		maxDuplicateToolCallsSet: true,
	}
	_, profile, err := app.loadProfile()
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

func TestLoadProfileRespectsExplicitZeroMaxContextTokensOverride(t *testing.T) {
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

	app := &app{
		profile:             "default",
		configPath:          path,
		maxContextTokens:    0,
		maxContextTokensSet: true,
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxContextTokens != 0 {
		t.Fatalf("max context tokens = %d", profile.MaxContextTokens)
	}
}

func TestLoadProfileAppliesSamplingCLIOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    temperature: 0.25
    top_p: 0.75
    extra_body:
      chat_template_kwargs:
        enable_thinking: false
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	app := &app{
		profile:        "default",
		configPath:     path,
		temperature:    1,
		temperatureSet: true,
		topP:           1,
		topPSet:        true,
		extraBody:      `{"chat_template_kwargs":{"enable_thinking":true,"clear_thinking":false}}`,
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.Temperature == nil {
		t.Fatal("expected temperature override")
	}
	if *profile.Temperature != 1 {
		t.Fatalf("temperature = %v", *profile.Temperature)
	}
	if profile.TopP == nil {
		t.Fatal("expected top_p override")
	}
	if *profile.TopP != 1 {
		t.Fatalf("top_p = %v", *profile.TopP)
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

func TestLoadProfileRejectsInvalidExtraBodyOverride(t *testing.T) {
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

	app := &app{
		profile:    "default",
		configPath: path,
		extraBody:  `[`,
	}
	_, _, err = app.loadProfile()
	if err == nil {
		t.Fatal("expected invalid --extra-body error")
	}
}

func TestMissingAPIKeyHintUsesDefaultProfileEnv(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		want    string
	}{
		{
			name:    "default",
			profile: "default",
			want:    "set OPENROUTER_API_KEY or provide api_key in config",
		},
		{
			name:    "openrouter alias",
			profile: "openrouter",
			want:    "set OPENROUTER_API_KEY or provide api_key in config",
		},
		{
			name:    "mittwald",
			profile: "mittwald",
			want:    "set MITTWALD_LLM_API_KEY or provide api_key in config",
		},
		{
			name:    "mistral",
			profile: "mistral",
			want:    "set MISTRAL_API_KEY or provide api_key in config",
		},
		{
			name:    "unknown",
			profile: "custom",
			want:    "set NICKPIT_API_KEY or provide api_key in config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := missingAPIKeyHint(tt.profile, false)
			if got != tt.want {
				t.Fatalf("hint = %q, want %q", got, tt.want)
			}
		})
	}
}
