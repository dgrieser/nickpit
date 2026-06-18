package modelcheck

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestCapabilityCacheRoundTripAndNormalizeBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-capabilities.json")
	capability := config.ModelCapabilities{
		Model:        " test-model ",
		Compatible:   true,
		Response:     true,
		Tools:        true,
		JSONResponse: boolPtr(true),
		Reasoning:    config.ReasoningCapabilities{Traces: true, Efforts: []string{"high"}},
	}

	if err := WriteCachedCapability(path, "https://example.invalid/v1/", "fp1", capability, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ReadCachedCapability(path, "https://example.invalid/v1", "test-model", "fp1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected cached capability")
	}
	if got.Model != "test-model" || !got.Compatible || !got.Response || !got.Tools {
		t.Fatalf("capability = %#v", got)
	}
	if got.JSONResponse == nil || !*got.JSONResponse {
		t.Fatalf("json response = %v, want true", got.JSONResponse)
	}
	if got.Reasoning.Efforts[0] != "high" {
		t.Fatalf("efforts = %#v", got.Reasoning.Efforts)
	}
}

func TestCapabilityCacheReplacesMatchingModelAndBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-capabilities.json")
	first := config.ModelCapabilities{Model: "model", Response: true, Reasoning: config.ReasoningCapabilities{Efforts: []string{"high"}}}
	second := config.ModelCapabilities{Model: "model", Response: true, Reasoning: config.ReasoningCapabilities{Efforts: []string{"low"}}}

	if err := WriteCachedCapability(path, "https://example.invalid/v1", "fp1", first, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if err := WriteCachedCapability(path, "https://example.invalid/v1/", "fp1", second, time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ReadCachedCapability(path, "https://example.invalid/v1", "model", "fp1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected cached capability")
	}
	if len(got.Reasoning.Efforts) != 1 || got.Reasoning.Efforts[0] != "low" {
		t.Fatalf("efforts = %#v, want replacement", got.Reasoning.Efforts)
	}
}

func TestCapabilityCacheMissingFile(t *testing.T) {
	_, ok, err := ReadCachedCapability(filepath.Join(t.TempDir(), "missing.json"), "https://example.invalid/v1", "model", "fp1")
	if ok {
		t.Fatal("missing cache should not return capability")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want os.ErrNotExist", err)
	}
}

func TestCapabilityCacheKeysOnSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-capabilities.json")
	url := "https://example.invalid/v1"
	high := config.ModelCapabilities{Model: "model", Response: true, Reasoning: config.ReasoningCapabilities{Efforts: []string{"high"}}}
	low := config.ModelCapabilities{Model: "model", Response: true, Reasoning: config.ReasoningCapabilities{Efforts: []string{"low"}}}

	// Same (base_url, model), different settings fingerprints → two coexisting entries.
	if err := WriteCachedCapability(path, url, "fpA", high, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if err := WriteCachedCapability(path, url, "fpB", low, time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}

	gotA, ok, err := ReadCachedCapability(path, url, "model", "fpA")
	if err != nil || !ok {
		t.Fatalf("fpA read: ok=%v err=%v", ok, err)
	}
	if len(gotA.Reasoning.Efforts) != 1 || gotA.Reasoning.Efforts[0] != "high" {
		t.Fatalf("fpA efforts = %#v, want [high]", gotA.Reasoning.Efforts)
	}
	gotB, ok, err := ReadCachedCapability(path, url, "model", "fpB")
	if err != nil || !ok {
		t.Fatalf("fpB read: ok=%v err=%v", ok, err)
	}
	if len(gotB.Reasoning.Efforts) != 1 || gotB.Reasoning.Efforts[0] != "low" {
		t.Fatalf("fpB efforts = %#v, want [low]", gotB.Reasoning.Efforts)
	}
	// An unseen fingerprint misses even though (base_url, model) match.
	if _, ok, err := ReadCachedCapability(path, url, "model", "fpC"); err != nil || ok {
		t.Fatalf("fpC read: ok=%v err=%v, want miss", ok, err)
	}
}

func TestCapabilityCacheLegacyEntryMissesFingerprint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-capabilities.json")
	url := "https://example.invalid/v1"
	cap := config.ModelCapabilities{Model: "model", Response: true, Reasoning: config.ReasoningCapabilities{Efforts: []string{"high"}}}

	// A legacy entry written before settings awareness has an empty fingerprint.
	if err := WriteCachedCapability(path, url, "", cap, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	// A real (non-empty) fingerprint never matches it → re-probe after upgrade.
	if _, ok, err := ReadCachedCapability(path, url, "model", "fpReal"); err != nil || ok {
		t.Fatalf("legacy read with fingerprint: ok=%v err=%v, want miss", ok, err)
	}
	// The legacy entry is still addressable with the empty fingerprint.
	if _, ok, err := ReadCachedCapability(path, url, "model", ""); err != nil || !ok {
		t.Fatalf("legacy read with empty fingerprint: ok=%v err=%v, want hit", ok, err)
	}
}

func TestFindProfileCapabilityUsesModelOnly(t *testing.T) {
	profile := config.Profile{
		Model:   "model",
		BaseURL: "https://example.invalid/v1",
		SupportedModels: []config.ModelCapabilities{{
			Model:      "model",
			Compatible: true,
			Reasoning:  config.ReasoningCapabilities{Efforts: []string{"high"}},
		}},
	}
	got, ok := FindProfileCapability(profile)
	if !ok {
		t.Fatal("expected profile capability")
	}
	if got.Model != "model" {
		t.Fatalf("model = %q", got.Model)
	}
}
