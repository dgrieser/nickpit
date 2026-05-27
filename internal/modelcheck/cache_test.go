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

	if err := WriteCachedCapability(path, "https://example.invalid/v1/", capability, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ReadCachedCapability(path, "https://example.invalid/v1", "test-model")
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

	if err := WriteCachedCapability(path, "https://example.invalid/v1", first, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if err := WriteCachedCapability(path, "https://example.invalid/v1/", second, time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ReadCachedCapability(path, "https://example.invalid/v1", "model")
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
	_, ok, err := ReadCachedCapability(filepath.Join(t.TempDir(), "missing.json"), "https://example.invalid/v1", "model")
	if ok {
		t.Fatal("missing cache should not return capability")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want os.ErrNotExist", err)
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
