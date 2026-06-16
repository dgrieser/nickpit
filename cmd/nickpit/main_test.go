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
	"github.com/dgrieser/nickpit/internal/workflow"
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

func TestLoadProfileAppliesSmallModelCLIOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: primary-model
    reasoning_effort: high
    small:
      model: file-small-model
      reasoning_effort: medium
      max_tokens: 1024
      temperature: 0.25
      top_p: 0.75
      top_k: 20
      presence_penalty: 0.05
      extra_body:
        chat_template_kwargs:
          enable_thinking: false
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	app := &app{
		profile:                 "default",
		configPath:              path,
		smallModel:              "cli-small-model",
		smallReasoningEffort:    "low",
		smallMaxTokens:          2048,
		smallMaxTokensSet:       true,
		smallTemperature:        0.5,
		smallTemperatureSet:     true,
		smallTopP:               0.9,
		smallTopPSet:            true,
		smallTopK:               40,
		smallTopKSet:            true,
		smallPresencePenalty:    0.1,
		smallPresencePenaltySet: true,
		smallExtraBody:          `{"chat_template_kwargs":{"enable_thinking":true}}`,
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.Small.Model != "cli-small-model" {
		t.Fatalf("small model = %q", profile.Small.Model)
	}
	if profile.Small.ReasoningEffort != "low" {
		t.Fatalf("small reasoning effort = %q", profile.Small.ReasoningEffort)
	}
	if profile.Small.MaxTokens == nil || *profile.Small.MaxTokens != 2048 {
		t.Fatalf("small max tokens = %v", profile.Small.MaxTokens)
	}
	if profile.Small.Temperature == nil || *profile.Small.Temperature != 0.5 {
		t.Fatalf("small temperature = %v", profile.Small.Temperature)
	}
	if profile.Small.TopP == nil || *profile.Small.TopP != 0.9 {
		t.Fatalf("small top_p = %v", profile.Small.TopP)
	}
	if profile.Small.TopK == nil || *profile.Small.TopK != 40 {
		t.Fatalf("small top_k = %v", profile.Small.TopK)
	}
	if profile.Small.PresencePenalty == nil || *profile.Small.PresencePenalty != 0.1 {
		t.Fatalf("small presence penalty = %v", profile.Small.PresencePenalty)
	}
	chatTemplateKwargs, ok := profile.Small.ExtraBody["chat_template_kwargs"].(map[string]any)
	if !ok || chatTemplateKwargs["enable_thinking"] != true {
		t.Fatalf("small extra body = %#v", profile.Small.ExtraBody)
	}
}

func TestLoadProfileAppliesFilterCLIOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    include_paths: ["\\.go$"]
    exclude_paths: ["\\.pb\\.go$"]
    include_content: ["package main"]
    exclude_content: ["DO NOT EDIT"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	app := &app{
		profile:           "default",
		configPath:        path,
		includePaths:      []string{"\\.ts$"},
		includePathsSet:   true,
		excludePaths:      []string{"\\.gen\\.ts$"},
		excludePathsSet:   true,
		includeContent:    []string{"export "},
		includeContentSet: true,
		excludeContent:    []string{"generated"},
		excludeContentSet: true,
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(profile.IncludePaths, ",") != "\\.ts$" {
		t.Fatalf("include paths = %#v", profile.IncludePaths)
	}
	if strings.Join(profile.ExcludePaths, ",") != "\\.gen\\.ts$" {
		t.Fatalf("exclude paths = %#v", profile.ExcludePaths)
	}
	if strings.Join(profile.IncludeContent, ",") != "export " {
		t.Fatalf("include content = %#v", profile.IncludeContent)
	}
	if strings.Join(profile.ExcludeContent, ",") != "generated" {
		t.Fatalf("exclude content = %#v", profile.ExcludeContent)
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
    top_k: 20
    presence_penalty: 0.05
    extra_body:
      chat_template_kwargs:
        enable_thinking: false
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	app := &app{
		profile:            "default",
		configPath:         path,
		temperature:        1,
		temperatureSet:     true,
		topP:               1,
		topPSet:            true,
		topK:               40,
		topKSet:            true,
		presencePenalty:    0.1,
		presencePenaltySet: true,
		extraBody:          `{"chat_template_kwargs":{"enable_thinking":true,"clear_thinking":false}}`,
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
	if profile.TopK == nil || *profile.TopK != 40 {
		t.Fatalf("top_k = %v", profile.TopK)
	}
	if profile.PresencePenalty == nil || *profile.PresencePenalty != 0.1 {
		t.Fatalf("presence penalty = %v", profile.PresencePenalty)
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
	if cmd.PersistentFlags().Lookup("verify-concurrency") != nil {
		t.Fatal("verify-concurrency flag should be replaced by --concurrency")
	}
	vc := cmd.PersistentFlags().Lookup("concurrency")
	if vc == nil {
		t.Fatal("concurrency flag missing")
		return
	}
	if vc.DefValue != "10" {
		t.Fatalf("concurrency default = %q, want 10", vc.DefValue)
	}
	if cmd.PersistentFlags().Lookup("skip-model-check") == nil {
		t.Fatal("skip-model-check flag missing")
	}
	if cmd.PersistentFlags().Lookup("small-model") == nil {
		t.Fatal("small-model flag missing")
	}
	if cmd.PersistentFlags().Lookup("small-reasoning-effort") == nil {
		t.Fatal("small-reasoning-effort flag missing")
	}
	for _, name := range []string{
		"top-k",
		"presence-penalty",
		"small-max-tokens",
		"small-temperature",
		"small-top-p",
		"small-top-k",
		"small-presence-penalty",
		"small-extra-body",
	} {
		if cmd.PersistentFlags().Lookup(name) == nil {
			t.Fatalf("%s flag missing", name)
		}
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
		err := (&app{}).writeModelCheckOutput("test-model", modelcheck.Result{
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
		"test-model",
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

func TestSmallModelRequirementsForDefaultSpecDoNotRequireTools(t *testing.T) {
	requirements := smallModelRequirementsForSpec(workflow.DefaultSpec(), model.ReviewRequest{})
	if !requirements.Uses() {
		t.Fatal("default spec should use the small model")
	}
	if requirements.Tools {
		t.Fatalf("default small-model requirements should not require tools: %+v", requirements)
	}
	if !requirements.JSONOutput || requirements.JSONSchema {
		t.Fatalf("default small-model requirements = %+v, want JSON output only", requirements)
	}

	result := modelcheck.Result{
		Probes: []modelcheck.ProbeResult{
			{Name: "configured_no_tools", ReasoningEffort: "low", Status: modelcheck.StatusOK},
			{Name: "configured_tools", ReasoningEffort: "low", Status: modelcheck.StatusUnsupported, Error: "tools unsupported"},
			{Name: "configured_json_output", ReasoningEffort: "low", Status: modelcheck.StatusOK},
		},
		PassedEfforts: []string{"low"},
	}
	if err := validateSmallModelCheck(result, requirements); err != nil {
		t.Fatalf("validateSmallModelCheck returned %v, want nil", err)
	}
}

func TestSmallModelRequirementsSkipSpecWithoutAlias(t *testing.T) {
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{
		{Type: workflow.StepReviewPrefix + "security"},
		{Type: workflow.StepFinalize},
	}}
	requirements := smallModelRequirementsForSpec(spec, model.ReviewRequest{})
	if requirements.Uses() {
		t.Fatalf("small-model requirements = %+v, want unused", requirements)
	}
}

func TestResolveActiveSpecUsesCustomSpecForSmallRequirements(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(path, []byte(`
version: 1
steps:
  - type: review:security
`), 0o644); err != nil {
		t.Fatal(err)
	}

	spec, err := (&app{specPath: path}).resolveActiveSpec()
	if err != nil {
		t.Fatal(err)
	}
	requirements := smallModelRequirementsForSpec(spec, model.ReviewRequest{})
	if requirements.Uses() {
		t.Fatalf("small-model requirements = %+v, want custom spec without @small to be unused", requirements)
	}
}

func TestSmallModelRequirementsRequireToolsForReviewAlias(t *testing.T) {
	alias := workflow.SmallModelAlias
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{{
		Type:   workflow.StepReviewPrefix + "security",
		Config: &workflow.StepOverride{Model: &alias},
	}}}
	requirements := smallModelRequirementsForSpec(spec, model.ReviewRequest{})
	if !requirements.Tools || !requirements.JSONOutput || requirements.JSONSchema {
		t.Fatalf("review small-model requirements = %+v, want tools and JSON output", requirements)
	}

	result := modelcheck.Result{
		Probes: []modelcheck.ProbeResult{
			{Name: "configured_no_tools", ReasoningEffort: "low", Status: modelcheck.StatusOK},
			{Name: "configured_tools", ReasoningEffort: "low", Status: modelcheck.StatusUnsupported, Error: "tools unsupported"},
			{Name: "configured_json_output", ReasoningEffort: "low", Status: modelcheck.StatusOK},
		},
		PassedEfforts: []string{"low"},
	}
	err := validateSmallModelCheck(result, requirements)
	if err == nil || !strings.Contains(err.Error(), "tool use") {
		t.Fatalf("validateSmallModelCheck error = %v, want tool-use failure", err)
	}
}

func TestSmallModelRequirementsHonorSchemaOverride(t *testing.T) {
	alias := workflow.SmallModelAlias
	useSchema := true
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{{
		Type: workflow.StepFinalize,
		Config: &workflow.StepOverride{
			Model:         &alias,
			UseJSONSchema: &useSchema,
		},
	}}}
	requirements := smallModelRequirementsForSpec(spec, model.ReviewRequest{})
	if !requirements.JSONSchema || requirements.JSONOutput || requirements.Tools {
		t.Fatalf("schema override requirements = %+v, want schema only", requirements)
	}
}

func TestSmallModelConfiguredForSamplingOverride(t *testing.T) {
	topK := 40
	profile := config.Profile{
		Model:           "same-model",
		ReasoningEffort: "low",
		Small:           config.SmallModelConfig{TopK: &topK},
	}
	if !smallModelConfigured(profile) {
		t.Fatal("expected small sampling override to require a small-model check")
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

	wantModel := "Model      [Qwen3.5-122B-A10B-FP8:high @ " + server.URL + "] ready 120k context"
	if !strings.Contains(stderr, wantModel) {
		t.Fatalf("stderr missing model progress line\nwant: %s\nstderr:\n%s", wantModel, stderr)
	}
	wantAgent := "] Structured no nudges, ≤2 retries, ∞ reasoning, ∞ loop repeats, no rate-limit-delay, ∞ tool calls, ≤5 duplicates, ∞ concurrency, parallel"
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
