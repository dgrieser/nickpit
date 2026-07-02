package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProfileFixtures(t *testing.T) (cfgPath, specPath string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, "cfg.yaml")
	cfg := `profiles:
  default:
    base_url: http://default
    api_key: kd
    model: m-default
  alt:
    base_url: http://alt
    api_key: ka
    model: m-alt
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	specPath = filepath.Join(dir, "wf.yaml")
	spec := "version: 1\nprofile: alt\nsteps:\n  - type: merge\n"
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath, specPath
}

// A spec's profile must be resolved by the command before the source adapter and
// request are built, so loadProfileForSpec returns the spec-selected profile.
func TestLoadProfileForSpecHonorsSpecProfile(t *testing.T) {
	cfg, spec := writeProfileFixtures(t)
	a := &app{configPath: cfg, profile: "default", specPath: spec}
	name, profile, err := a.loadProfileForSpec()
	if err != nil {
		t.Fatal(err)
	}
	if name != "alt" {
		t.Fatalf("profile name = %q, want alt", name)
	}
	if profile.Model != "m-alt" || profile.BaseURL != "http://alt" {
		t.Fatalf("profile = %q/%q, want m-alt/http://alt", profile.Model, profile.BaseURL)
	}
}

func TestLoadProfileForSpecKeepsDefaultWithoutSpecProfile(t *testing.T) {
	cfg, _ := writeProfileFixtures(t)
	// No spec at all.
	a := &app{configPath: cfg, profile: "default"}
	name, profile, err := a.loadProfileForSpec()
	if err != nil {
		t.Fatal(err)
	}
	if name != "default" || profile.Model != "m-default" {
		t.Fatalf("profile = %q/%q, want default/m-default", name, profile.Model)
	}
}

func TestLoadProfileForSpecIgnoresProfileForSingleStep(t *testing.T) {
	cfg, _ := writeProfileFixtures(t)
	// --step builds a one-step spec that carries no profile, so the default
	// profile is kept even though a spec file path is also present.
	a := &app{configPath: cfg, profile: "default", stepName: "merge"}
	name, _, err := a.loadProfileForSpec()
	if err != nil {
		t.Fatal(err)
	}
	if name != "default" {
		t.Fatalf("profile name = %q, want default for --step", name)
	}
}

// active_profile from the config file must select the profile when --profile
// is left at its flag default; an explicit --profile (even "default") wins.
func TestLoadProfileHonorsConfigActiveProfile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.yaml")
	cfg := `active_profile: alt
profiles:
  default:
    base_url: http://default
    api_key: kd
    model: m-default
  alt:
    base_url: http://alt
    api_key: ka
    model: m-alt
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	// Flag default "default", not explicitly set: active_profile wins.
	a := &app{configPath: cfgPath, profile: "default"}
	name, profile, err := a.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if name != "alt" || profile.Model != "m-alt" {
		t.Fatalf("profile = %q/%q, want alt/m-alt", name, profile.Model)
	}

	// --profile=default passed explicitly: overrides active_profile.
	a = &app{configPath: cfgPath, profile: "default", profileSet: true}
	name, profile, err = a.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if name != "default" || profile.Model != "m-default" {
		t.Fatalf("profile = %q/%q, want default/m-default", name, profile.Model)
	}
}
