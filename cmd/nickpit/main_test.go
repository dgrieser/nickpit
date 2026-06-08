package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/modelcheck"
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
    max_output_retries: 4
    max_reasoning_seconds: 5
    max_rate_limit_delay_seconds: 6
    nudge_count: 7
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	app := &app{
		profile:                     "default",
		configPath:                  path,
		maxToolCalls:                0,
		maxToolCallsSet:             true,
		maxDuplicateToolCalls:       0,
		maxDuplicateToolCallsSet:    true,
		maxOutputRetries:            0,
		maxOutputRetriesSet:         true,
		maxReasoningSeconds:         0,
		maxReasoningSecondsSet:      true,
		maxRateLimitDelaySeconds:    0,
		maxRateLimitDelaySecondsSet: true,
		nudgeCount:                  0,
		nudgeCountSet:               true,
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
	if profile.MaxOutputRetries != 0 {
		t.Fatalf("max output retries = %d", profile.MaxOutputRetries)
	}
	if profile.MaxReasoningSeconds != 0 {
		t.Fatalf("max reasoning seconds = %d", profile.MaxReasoningSeconds)
	}
	if profile.MaxRateLimitDelaySeconds != 0 {
		t.Fatalf("max rate limit delay seconds = %d", profile.MaxRateLimitDelaySeconds)
	}
	if profile.NudgeCount != 0 {
		t.Fatalf("nudge count = %d", profile.NudgeCount)
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

func TestLoadProfileAppliesRateLimitDelayCLIOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_rate_limit_delay_seconds: 12
    nudge_count: 2
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	app := &app{
		profile:                     "default",
		configPath:                  path,
		maxRateLimitDelaySeconds:    20,
		maxRateLimitDelaySecondsSet: true,
		nudgeCount:                  4,
		nudgeCountSet:               true,
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxRateLimitDelaySeconds != 20 {
		t.Fatalf("max rate limit delay seconds = %d", profile.MaxRateLimitDelaySeconds)
	}
	if profile.NudgeCount != 4 {
		t.Fatalf("nudge count = %d", profile.NudgeCount)
	}
}

func TestLoadProfileAppliesDisablePatchSummary(t *testing.T) {
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
		profile:             "default",
		configPath:          path,
		disablePatchSummary: true,
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if !profile.DisablePatchSummary {
		t.Fatal("expected disable patch summary CLI override")
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

func TestRootCmdDropsVerifySkipFlags(t *testing.T) {
	cmd := newRootCmd()
	for _, name := range []string{"no-verify", "no-finalize"} {
		if cmd.PersistentFlags().Lookup(name) != nil {
			t.Fatalf("unexpected persistent flag %q", name)
		}
	}
	if cmd.PersistentFlags().Lookup("verify-concurrency") == nil {
		t.Fatal("verify-concurrency flag missing")
	}
	if cmd.PersistentFlags().Lookup("skip-model-check") == nil {
		t.Fatal("skip-model-check flag missing")
	}
	if cmd.PersistentFlags().Lookup("disable-reasoning-extract") == nil {
		t.Fatal("disable-reasoning-extract flag missing")
	}
}

func TestRootCmdHasCheckModel(t *testing.T) {
	cmd := newRootCmd()
	check, _, err := cmd.Find([]string{"check", "model"})
	if err != nil {
		t.Fatal(err)
	}
	if check == nil || check.Use != "model" {
		t.Fatalf("check model command missing: %#v", check)
	}
	if check.Flags().Lookup("refresh") == nil {
		t.Fatal("check model refresh flag missing")
	}
}

func TestWriteModelCheckOutputUsesTerminalSummary(t *testing.T) {
	out := captureStdout(t, func() {
		err := (&app{}).writeModelCheckOutput(modelcheck.Result{
			Probes: []modelcheck.ProbeResult{
				{Name: "configured_no_tools", ReasoningEffort: "high", Reasoned: true, Status: modelcheck.StatusOK},
				{Name: "configured_tools", ReasoningEffort: "high", Tools: true, Status: modelcheck.StatusOK},
				{Name: "configured_json_output", ReasoningEffort: "high", Status: modelcheck.StatusOK},
				{Name: "configured_json_schema", ReasoningEffort: "high", Status: modelcheck.StatusOK},
			},
			PassedEfforts: []string{"high", "medium"},
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{
		"✓ Model is compatible",
		"✓ Tool Use",
		"✓ Structured Output",
		"✓ JSON Schema",
		"✓ Reasoning Traces",
		"Supported Efforts",
		"  ✓ high",
		"  ✓ medium",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "check:") || strings.Contains(out, "json_response:") {
		t.Fatalf("output should not use YAML style\n%s", out)
	}
}

type recordingSource struct {
	called bool
	err    error
}

func (s *recordingSource) ResolveContext(context.Context, model.ReviewRequest) (*model.ReviewContext, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	return &model.ReviewContext{Repository: model.RepositoryInfo{}, ChangedFiles: []model.ChangedFile{}}, nil
}

func TestRunReviewRunsModelCheckBeforeSourceWork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "reasoning_effort unsupported"},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	source := &recordingSource{}
	err := (&app{}).runReview(context.Background(), source, nil, "default", config.Profile{
		Model:           "model",
		BaseURL:         server.URL,
		APIKey:          "token",
		ReasoningEffort: "high",
	}, model.ReviewRequest{Mode: model.ModeLocal, RepoRoot: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "model check failed") {
		t.Fatalf("error = %v, want model check failure", err)
	}
	if source.called {
		t.Fatal("source should not run before model check passes")
	}
}

func TestRunReviewShowProgressPrintsModelBeforeModelCheckFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "reasoning_effort unsupported"},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	stderr := captureStderr(t, func() {
		err := (&app{showProgress: true}).runReview(context.Background(), &recordingSource{}, nil, "mittwald", config.Profile{
			Model:                      "Qwen3.5-122B-A10B-FP8",
			BaseURL:                    server.URL,
			APIKey:                     "token",
			ReasoningEffort:            "high",
			MaxContextTokens:           120000,
			MaxDuplicateToolCalls:      5,
			MaxOutputRetries:           2,
			MaxOutputRetriesConfigured: true,
			UseJSONSchema:              true,
		}, model.ReviewRequest{
			Mode:                  model.ModeLocal,
			RepoRoot:              t.TempDir(),
			MaxOutputRetries:      2,
			MaxDuplicateToolCalls: 5,
			UseJSONSchema:         true,
			PriorityThreshold:     "p3",
		})
		if err == nil || !strings.Contains(err.Error(), "model check failed") {
			t.Fatalf("error = %v, want model check failure", err)
		}
	})

	wantModel := "Model: Qwen3.5-122B-A10B-FP8:high [120k context] @ " + server.URL
	if !strings.Contains(stderr, wantModel) {
		t.Fatalf("stderr missing model progress line\nwant: %s\nstderr:\n%s", wantModel, stderr)
	}
	wantAgent := "Agent: Structured [no nudges, ≤2 retries, ∞ reasoning, ∞ loop repeats, no rate-limit-delay, ∞ tool calls, ≤5 duplicates, parallel]"
	if !strings.Contains(stderr, wantAgent) {
		t.Fatalf("stderr missing agent progress line\nwant: %s\nstderr:\n%s", wantAgent, stderr)
	}
}

func TestRunReviewSkipModelCheckBypassesChecker(t *testing.T) {
	wantErr := errors.New("source called")
	source := &recordingSource{err: wantErr}
	err := (&app{skipModelCheck: true}).runReview(context.Background(), source, nil, "default", config.Profile{
		Model:           "model",
		BaseURL:         "http://127.0.0.1:1",
		APIKey:          "token",
		ReasoningEffort: "high",
	}, model.ReviewRequest{Mode: model.ModeLocal, RepoRoot: t.TempDir()})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want source error", err)
	}
	if !source.called {
		t.Fatal("source should run when model check is skipped")
	}
}

func TestRunReviewUsesProfileSupportedModelCapabilities(t *testing.T) {
	wantErr := errors.New("source called")
	source := &recordingSource{err: wantErr}
	err := (&app{}).runReview(context.Background(), source, nil, "default", config.Profile{
		Model:           "model",
		BaseURL:         "http://127.0.0.1:1",
		APIKey:          "token",
		ReasoningEffort: "high",
		SupportedModels: []config.ModelCapabilities{compatibleCapability("model")},
	}, model.ReviewRequest{Mode: model.ModeLocal, RepoRoot: t.TempDir()})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want source error", err)
	}
	if !source.called {
		t.Fatal("source should run when profile capabilities satisfy model check")
	}
}

func TestRunReviewUsesCachedModelCapabilities(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("NICKPIT_CACHE_DIR", cacheDir)
	if err := modelcheck.WriteCachedCapability(filepath.Join(cacheDir, "model-capabilities.json"), "http://127.0.0.1:1", compatibleCapability("model"), time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("source called")
	source := &recordingSource{err: wantErr}
	err := (&app{}).runReview(context.Background(), source, nil, "default", config.Profile{
		Model:           "model",
		BaseURL:         "http://127.0.0.1:1/",
		APIKey:          "token",
		ReasoningEffort: "high",
	}, model.ReviewRequest{Mode: model.ModeLocal, RepoRoot: t.TempDir()})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want source error", err)
	}
	if !source.called {
		t.Fatal("source should run when cached capabilities satisfy model check")
	}
}

func compatibleCapability(modelName string) config.ModelCapabilities {
	jsonResponse := true
	jsonSchema := true
	return config.ModelCapabilities{
		Model:        modelName,
		Compatible:   true,
		Response:     true,
		Tools:        true,
		JSONResponse: &jsonResponse,
		JSONSchema:   &jsonSchema,
		Reasoning: config.ReasoningCapabilities{
			Traces:  true,
			Efforts: []string{"high"},
		},
	}
}

func captureOutput(t *testing.T, stream **os.File, fn func()) string {
	t.Helper()
	original := *stream
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	*stream = w
	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	defer func() {
		*stream = original
		_ = r.Close()
	}()

	fn()

	_ = w.Close()
	return <-done
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return captureOutput(t, &os.Stderr, fn)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return captureOutput(t, &os.Stdout, fn)
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
