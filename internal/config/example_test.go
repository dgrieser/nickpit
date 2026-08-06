package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/toollimits"
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
		if profile.MaxToolResultPercent != DefaultMaxToolResultPercent {
			t.Fatalf("%s max tool result percent = %d", entry.name, profile.MaxToolResultPercent)
		}
		if profile.MaxToolCalls != toollimits.DefaultMaxToolCalls {
			t.Fatalf("%s max tool calls = %d", entry.name, profile.MaxToolCalls)
		}
		if profile.MaxDuplicateToolCalls != toollimits.DefaultMaxDuplicateToolCalls {
			t.Fatalf("%s max duplicate tool calls = %d", entry.name, profile.MaxDuplicateToolCalls)
		}
		if profile.MaxOutputRetries != DefaultMaxOutputRetries {
			t.Fatalf("%s max output retries = %d", entry.name, profile.MaxOutputRetries)
		}
		if profile.MaxReasoningSeconds != DefaultMaxReasoningSeconds {
			t.Fatalf("%s max reasoning seconds = %d", entry.name, profile.MaxReasoningSeconds)
		}
		if profile.NudgeCount != DefaultNudgeCount {
			t.Fatalf("%s nudge count = %d", entry.name, profile.NudgeCount)
		}
		if profile.DisablePatchSummary {
			t.Fatalf("%s disable patch summary = true, want false default", entry.name)
		}
		if profile.DiffFormat != model.DiffFormatGit {
			t.Fatalf("%s diff format = %q", entry.name, profile.DiffFormat)
		}
		if profile.ReasoningEffort != DefaultReasoningEffort {
			t.Fatalf("%s reasoning effort = %q", entry.name, profile.ReasoningEffort)
		}
	}
}

// mappingKeys collects the keys of a YAML mapping node.
func mappingKeys(t *testing.T, node *yaml.Node) map[string]bool {
	t.Helper()
	if node.Kind != yaml.MappingNode {
		t.Fatalf("node kind = %v, want mapping", node.Kind)
	}
	keys := make(map[string]bool, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		keys[node.Content[i].Value] = true
	}
	return keys
}

// yamlTagNames returns the serialized yaml key of every exported field of typ,
// skipping fields marked with `yaml:"-"`.
func yamlTagNames(t *testing.T, typ reflect.Type) []string {
	t.Helper()
	names := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if name == "" {
			t.Fatalf("%s.%s has no yaml tag", typ.Name(), field.Name)
		}
		if name == "-" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// TestExampleProfileNodeCoversAllProfileKeys guards the generated example
// against drifting from the Profile struct: every serialized yaml key must
// appear in the example profile mapping.
func TestExampleProfileNodeCoversAllProfileKeys(t *testing.T) {
	keys := mappingKeys(t, exampleProfileNode(Profile{}))
	for _, name := range yamlTagNames(t, reflect.TypeFor[Profile]()) {
		if !keys[name] {
			t.Errorf("example profile omits yaml key %q", name)
		}
	}
}

// TestSupportedModelsNodeCoversAllCapabilityKeys does the same for the
// supported_models entries; optional pointer capabilities are set so their
// conditional keys are rendered.
func TestSupportedModelsNodeCoversAllCapabilityKeys(t *testing.T) {
	capability := ModelCapabilities{
		JSONSchema:      ptrTo(true),
		JSONResponse:    ptrTo(true),
		ToolsJSONSchema: ptrTo(true),
	}
	seq := supportedModelsNode([]ModelCapabilities{capability})
	if seq.Kind != yaml.SequenceNode || len(seq.Content) != 1 {
		t.Fatalf("supported models node = %+v, want one-entry sequence", seq)
	}
	keys := mappingKeys(t, seq.Content[0])
	for _, name := range yamlTagNames(t, reflect.TypeFor[ModelCapabilities]()) {
		if !keys[name] {
			t.Errorf("supported_models entry omits yaml key %q", name)
		}
	}
}
