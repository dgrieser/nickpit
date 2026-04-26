package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExampleYAMLContainsDefaultProfiles(t *testing.T) {
	data, err := ExampleYAML()
	if err != nil {
		t.Fatal(err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveProfile != DefaultProfileName {
		t.Fatalf("active profile = %q", cfg.ActiveProfile)
	}
	if len(cfg.Profiles) != len(defaultProfiles) {
		t.Fatalf("profiles = %d, want %d", len(cfg.Profiles), len(defaultProfiles))
	}

	for _, entry := range defaultProfiles {
		profile, ok := cfg.Profiles[entry.name]
		if !ok {
			t.Fatalf("missing profile %q", entry.name)
		}
		if profile.Model != entry.profile.Model {
			t.Fatalf("%s model = %q", entry.name, profile.Model)
		}
		if profile.BaseURL != entry.profile.BaseURL {
			t.Fatalf("%s base url = %q", entry.name, profile.BaseURL)
		}
		if profile.APIKey != canonicalEnvRef(entry.profile.APIKey) {
			t.Fatalf("%s api key = %q", entry.name, profile.APIKey)
		}
		if profile.MaxContextTokens != DefaultMaxContextToken {
			t.Fatalf("%s max context tokens = %d", entry.name, profile.MaxContextTokens)
		}
		if profile.MaxToolCalls != MaxToolCalls {
			t.Fatalf("%s max tool calls = %d", entry.name, profile.MaxToolCalls)
		}
		if profile.MaxDuplicateToolCalls != DefaultMaxDuplicateToolCalls {
			t.Fatalf("%s max duplicate tool calls = %d", entry.name, profile.MaxDuplicateToolCalls)
		}
	}
}
