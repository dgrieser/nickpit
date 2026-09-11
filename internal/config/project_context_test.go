package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProjectContextConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigProjectContextAppendOverride(t *testing.T) {
	path := writeProjectContextConfig(t, `
profiles:
  default:
    model: test-model
    project_context: ["ops.yaml", "https://example.com/context.yaml"]
`)

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(profile.ProjectContext, ",") != "ops.yaml,https://example.com/context.yaml" {
		t.Fatalf("project_context = %#v", profile.ProjectContext)
	}

	// CLI values append to the file's list, matching styleguides; exact
	// duplicates and empties are dropped and specs are trimmed.
	_, profile, err = Load(path, Overrides{ProjectContext: []string{" extra.yaml ", "ops.yaml", ""}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(profile.ProjectContext, ",") != "ops.yaml,https://example.com/context.yaml,extra.yaml" {
		t.Fatalf("appended project_context = %#v", profile.ProjectContext)
	}
}

func TestLoadConfigProjectContextEnv(t *testing.T) {
	path := writeProjectContextConfig(t, `
profiles:
  default:
    model: test-model
    project_context: ["ops.yaml"]
`)
	t.Setenv("NICKPIT_PROJECT_CONTEXT", " env.yaml ")

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(profile.ProjectContext, ",") != "ops.yaml,env.yaml" {
		t.Fatalf("project_context = %#v, want the env value appended", profile.ProjectContext)
	}
}

func TestLoadConfigRejectsInvalidProjectContextURL(t *testing.T) {
	path := writeProjectContextConfig(t, `
profiles:
  default:
    model: test-model
    project_context: ["https:///no-host.yaml"]
`)

	if _, _, err := Load(path, Overrides{}); err == nil || !strings.Contains(err.Error(), "project_context[0]") {
		t.Fatalf("error = %v, want a project_context URL error", err)
	}
}

func TestLoadConfigDisableProjectContext(t *testing.T) {
	path := writeProjectContextConfig(t, `
profiles:
  default:
    model: test-model
    disable_project_context: true
`)

	_, profile, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.DisableProjectContext {
		t.Fatal("DisableProjectContext = false, want the config value honored")
	}

	// The flag can only turn the switch on: there is no --enable counterpart, so
	// an unset flag must not undo the config.
	_, profile, err = Load(path, Overrides{DisableProjectContext: false})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.DisableProjectContext {
		t.Fatal("an unset --disable-project-context flag cleared the config value")
	}

	offPath := writeProjectContextConfig(t, `
profiles:
  default:
    model: test-model
`)
	_, profile, err = Load(offPath, Overrides{DisableProjectContext: true})
	if err != nil {
		t.Fatal(err)
	}
	if !profile.DisableProjectContext {
		t.Fatal("DisableProjectContext = false, want the flag to turn it on")
	}
}
