package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatGPTProfileUsesPlaceholderKeyAndOpenAIEndpoint(t *testing.T) {
	_, profile, err := Load("", Overrides{Profile: "chatgpt"})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Auth != AuthChatGPT || profile.BaseURL != ChatGPTBaseURL || profile.APIKey != ChatGPTAPIKeyPlaceholder {
		t.Fatalf("profile auth=%q base_url=%q api_key=%q", profile.Auth, profile.BaseURL, profile.APIKey)
	}
	if profile.Model == "" {
		t.Fatal("chatgpt profile has no default model")
	}
}

func TestChatGPTAuthRefusesForeignBaseURL(t *testing.T) {
	_, _, err := Load("", Overrides{Profile: "chatgpt", BaseURL: "https://llm.example.com/v1"})
	if err == nil || !strings.Contains(err.Error(), "only works with base_url") {
		t.Fatalf("error = %v", err)
	}
}

func TestChatGPTAuthOnCustomProfileDefaultsBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
active_profile: plan
profiles:
  plan:
    auth: chatgpt
    model: some-model
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.BaseURL != ChatGPTBaseURL || profile.APIKey != ChatGPTAPIKeyPlaceholder {
		t.Fatalf("base_url=%q api_key=%q", profile.BaseURL, profile.APIKey)
	}
}

func TestUnknownAuthIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
profiles:
  default:
    auth: magic
    model: m
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path, Overrides{}); err == nil || !strings.Contains(err.Error(), "auth must be") {
		t.Fatalf("error = %v", err)
	}
}

func TestEffectiveSmallProfileDropsChatGPTAuthOnForeignEndpoint(t *testing.T) {
	profile := Profile{
		Auth:    AuthChatGPT,
		BaseURL: ChatGPTBaseURL,
		APIKey:  ChatGPTAPIKeyPlaceholder,
		Model:   "big",
		Small:   SmallModelConfig{Model: "small", BaseURL: "https://other.example.com/v1", APIKey: "k"},
	}
	small := EffectiveSmallProfile(profile)
	if small.Auth != "" || small.APIKey != "k" {
		t.Fatalf("small auth=%q api_key=%q", small.Auth, small.APIKey)
	}
	profile.Small = SmallModelConfig{Model: "small"}
	if same := EffectiveSmallProfile(profile); same.Auth != AuthChatGPT {
		t.Fatalf("small on the same endpoint lost auth: %q", same.Auth)
	}
}

func TestAPISelection(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	_, profile, err := Load("", Overrides{Profile: "chatgpt"})
	if err != nil || profile.API != APIResponses {
		t.Fatalf("chatgpt profile api = %q, %v", profile.API, err)
	}

	_, profile, err = Load(write(t, "profiles:\n  default:\n    model: m\n    api: responses\n    base_url: https://api.openai.com/v1\n    api_key: k\n"), Overrides{})
	if err != nil || profile.API != APIResponses || profile.Auth != "" {
		t.Fatalf("api-key responses profile api=%q auth=%q err=%v", profile.API, profile.Auth, err)
	}

	_, profile, err = Load(write(t, "active_profile: plan\nprofiles:\n  plan:\n    auth: chatgpt\n    model: m\n"), Overrides{})
	if err != nil || profile.API != APIResponses {
		t.Fatalf("auth: chatgpt must imply api responses: api=%q err=%v", profile.API, err)
	}

	_, _, err = Load(write(t, "active_profile: plan\nprofiles:\n  plan:\n    auth: chatgpt\n    api: chat_completions\n    model: m\n"), Overrides{})
	if err == nil || !strings.Contains(err.Error(), "requires api") {
		t.Fatalf("chatgpt over chat completions error = %v", err)
	}

	_, _, err = Load(write(t, "profiles:\n  default:\n    model: m\n    api: grpc\n"), Overrides{})
	if err == nil || !strings.Contains(err.Error(), "api must be") {
		t.Fatalf("unknown api error = %v", err)
	}
}

func TestEffectiveSmallProfileResetsAPIOnForeignEndpoint(t *testing.T) {
	profile := Profile{
		API:     APIResponses,
		BaseURL: ChatGPTBaseURL,
		Model:   "big",
		Small:   SmallModelConfig{Model: "small", BaseURL: "https://other.example.com/v1", APIKey: "k"},
	}
	if small := EffectiveSmallProfile(profile); small.API != "" {
		t.Fatalf("small api = %q", small.API)
	}
	profile.Small = SmallModelConfig{Model: "small"}
	if small := EffectiveSmallProfile(profile); small.API != APIResponses {
		t.Fatalf("small on the same endpoint lost api: %q", small.API)
	}
}
