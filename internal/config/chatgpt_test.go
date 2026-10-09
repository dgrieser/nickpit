package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
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

func loadSmallTestProfile(t *testing.T, body string) (Profile, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, profile, err := Load(path, Overrides{})
	return profile, err
}

// The small model may use any protocol and any authentication, independent
// of the primary model.
func TestSmallModelOnAnyAPIAndAuth(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		want    Profile
		wantErr string
	}{
		{
			name: "api key primary, ChatGPT plan small",
			body: "active_profile: p\nprofiles:\n  p:\n    base_url: https://llm.example.com/v1\n    api_key: primary-key\n    model: big\n    supported_models: [{model: big, compatible: true}]\n    small:\n      model: gpt-small\n      auth: chatgpt\n",
			want: Profile{Model: "gpt-small", BaseURL: ChatGPTBaseURL, APIKey: ChatGPTAPIKeyPlaceholder, API: APIResponses, Auth: AuthChatGPT},
		},
		{
			name: "ChatGPT primary, api key small elsewhere over chat completions",
			body: "active_profile: p\nprofiles:\n  p:\n    auth: chatgpt\n    model: big\n    small:\n      model: qwen\n      base_url: https://llm.example.com/v1\n      api_key: small-key\n",
			want: Profile{Model: "qwen", BaseURL: "https://llm.example.com/v1", APIKey: "small-key"},
		},
		{
			name: "ChatGPT primary, platform key small on the same host over responses",
			body: "active_profile: p\nprofiles:\n  p:\n    auth: chatgpt\n    model: big\n    small:\n      model: gpt-mini\n      auth: api_key\n      api_key: platform-key\n      api: responses\n",
			want: Profile{Model: "gpt-mini", BaseURL: ChatGPTBaseURL, APIKey: "platform-key", API: APIResponses},
		},
		{
			name: "same endpoint, small switches protocol only",
			body: "active_profile: p\nprofiles:\n  p:\n    base_url: https://api.openai.com/v1\n    api_key: k\n    model: big\n    small:\n      model: mini\n      api: responses\n",
			want: Profile{Model: "mini", BaseURL: "https://api.openai.com/v1", APIKey: "k", API: APIResponses},
		},
		{
			name:    "own endpoint without its own key",
			body:    "active_profile: p\nprofiles:\n  p:\n    auth: chatgpt\n    model: big\n    small:\n      model: mini\n      auth: api_key\n",
			wantErr: "small.api_key is empty",
		},
		{
			name:    "ChatGPT small pinned to the ChatGPT host",
			body:    "active_profile: p\nprofiles:\n  p:\n    base_url: https://llm.example.com/v1\n    api_key: k\n    model: big\n    small:\n      auth: chatgpt\n      base_url: https://llm.example.com/v1\n",
			wantErr: "small.auth \"chatgpt\" only works with base_url",
		},
		{
			name:    "ChatGPT small over chat completions",
			body:    "active_profile: p\nprofiles:\n  p:\n    base_url: https://llm.example.com/v1\n    api_key: k\n    model: big\n    small:\n      auth: chatgpt\n      api: chat_completions\n",
			wantErr: "small.auth \"chatgpt\" requires api",
		},
		{
			name:    "unknown small api",
			body:    "active_profile: p\nprofiles:\n  p:\n    base_url: https://llm.example.com/v1\n    api_key: k\n    model: big\n    small:\n      api: grpc\n",
			wantErr: "small.api must be",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile, err := loadSmallTestProfile(t, tc.body)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			small := EffectiveSmallProfile(profile)
			got := Profile{Model: small.Model, BaseURL: small.BaseURL, APIKey: small.APIKey, API: small.API, Auth: small.Auth}
			if got.BaseURL != tc.want.BaseURL || got.Model != tc.want.Model || got.APIKey != tc.want.APIKey || got.API != tc.want.API || got.Auth != tc.want.Auth {
				t.Fatalf("small = %+v, want %+v", got, tc.want)
			}
			if small.APIKey == profile.APIKey && small.BaseURL != profile.BaseURL {
				t.Fatalf("primary key %q leaked to small endpoint %q", profile.APIKey, small.BaseURL)
			}
			if !model.SameEndpoint(small.BaseURL, profile.BaseURL) && small.SupportedModels != nil {
				t.Fatalf("primary supported_models carried to another endpoint: %v", small.SupportedModels)
			}
		})
	}
}

func TestSmallAPIAndAuthFromEnvironment(t *testing.T) {
	t.Setenv("NICKPIT_SMALL_AUTH", "chatgpt")
	t.Setenv("NICKPIT_SMALL_MODEL", "gpt-small")
	profile, err := loadSmallTestProfile(t, "active_profile: p\nprofiles:\n  p:\n    base_url: https://llm.example.com/v1\n    api_key: k\n    model: big\n")
	if err != nil {
		t.Fatal(err)
	}
	if small := EffectiveSmallProfile(profile); small.Auth != AuthChatGPT || small.API != APIResponses || small.BaseURL != ChatGPTBaseURL {
		t.Fatalf("small = auth %q api %q base %q", small.Auth, small.API, small.BaseURL)
	}
}

func TestAuthAPIKeyIsTheDefault(t *testing.T) {
	profile, err := loadSmallTestProfile(t, "active_profile: p\nprofiles:\n  p:\n    base_url: https://llm.example.com/v1\n    api_key: k\n    auth: api_key\n    model: big\n")
	if err != nil || profile.Auth != "" {
		t.Fatalf("auth = %q, %v", profile.Auth, err)
	}
}
