package config

import (
	"fmt"

	"github.com/dgrieser/nickpit/internal/model"
)

// canonicalAuth folds the spellings of key authentication into one value:
// both an empty auth and AuthAPIKey mean the api_key.
func canonicalAuth(auth string) string {
	if auth == AuthAPIKey {
		return ""
	}
	return auth
}

// smallDeclaresEndpoint reports whether small names any part of an endpoint.
func smallDeclaresEndpoint(small SmallModelConfig) bool {
	return small.BaseURL != "" || small.APIKey != "" || small.API != "" || small.Auth != ""
}

// smallOwnsEndpoint reports whether profile's small model talks to an
// endpoint of its own: another base URL, or another kind of authentication.
// Another api alone on the same host and credential is not one — the key
// stays with the host it belongs to.
func smallOwnsEndpoint(profile Profile) bool {
	small := profile.Small
	if small.BaseURL != "" && !model.SameEndpoint(small.BaseURL, profile.BaseURL) {
		return true
	}
	return small.Auth != "" && canonicalAuth(small.Auth) != canonicalAuth(profile.Auth)
}

// resolveLLMEndpoint applies the defaults an authentication implies and
// rejects combinations that cannot work. It serves the primary profile and
// the effective small profile alike; field prefixes the setting names in
// errors ("" or "small.").
func resolveLLMEndpoint(profile *Profile, field string) error {
	profile.Auth = canonicalAuth(profile.Auth)
	switch profile.Auth {
	case "":
	case AuthChatGPT:
		if profile.BaseURL == "" {
			profile.BaseURL = ChatGPTBaseURL
		}
		// The plan tokens must never reach another host, whatever base_url
		// or --base-url says.
		if !model.SameEndpoint(profile.BaseURL, ChatGPTBaseURL) {
			return fmt.Errorf("config: %sauth %q only works with base_url %q, not %q", field, AuthChatGPT, ChatGPTBaseURL, profile.BaseURL)
		}
		if profile.APIKey == "" {
			profile.APIKey = ChatGPTAPIKeyPlaceholder
		}
		// ChatGPT plan tokens are admitted on the Responses API only.
		switch profile.API {
		case "":
			profile.API = APIResponses
		case APIResponses:
		default:
			return fmt.Errorf("config: %sauth %q requires api %q, not %q", field, AuthChatGPT, APIResponses, profile.API)
		}
	default:
		return fmt.Errorf("config: %sauth must be %q or %q, got %q", field, AuthAPIKey, AuthChatGPT, profile.Auth)
	}
	switch profile.API {
	case "", APIChatCompletions, APIResponses:
	default:
		return fmt.Errorf("config: %sapi must be %q or %q, got %q", field, APIChatCompletions, APIResponses, profile.API)
	}
	return nil
}
