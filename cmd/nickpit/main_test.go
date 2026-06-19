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
	"sync/atomic"
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

func TestLoadProfileAppliesSkipSuggestions(t *testing.T) {
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
		profile:         "default",
		configPath:      path,
		skipSuggestions: true,
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if !profile.SkipSuggestions {
		t.Fatal("expected skip suggestions CLI override")
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

func TestPRAndMRCommandsHaveURLFlag(t *testing.T) {
	cmd := newRootCmd()
	for _, args := range [][]string{{"github", "pr"}, {"gitlab", "mr"}} {
		found, _, err := cmd.Find(args)
		if err != nil {
			t.Fatal(err)
		}
		if found.Flags().Lookup("url") == nil {
			t.Fatalf("%s command missing --url flag", strings.Join(args, " "))
		}
	}
}

func TestParseGitHubPRURL(t *testing.T) {
	tests := []struct {
		raw  string
		repo string
		id   int
	}{
		{
			raw:  "https://github.com/dgrieser/nickpit/pull/60",
			repo: "dgrieser/nickpit",
			id:   60,
		},
		{
			raw:  "https://github.com/dgrieser/nickpit/pull/60/changes#diff-97bc82c1e601dde3195cc516a4ce8e58bb37ce0904b143123cc284fa568debe7L11",
			repo: "dgrieser/nickpit",
			id:   60,
		},
		{
			raw:  "https://github.com/dgrieser/nickpit/pull/60/files?plain=1",
			repo: "dgrieser/nickpit",
			id:   60,
		},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			repo, id, err := parseGitHubPRURL(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if repo != tt.repo || id != tt.id {
				t.Fatalf("parseGitHubPRURL() = %q, %d; want %q, %d", repo, id, tt.repo, tt.id)
			}
		})
	}
}

func TestParseGitHubPRURLRejectsInvalid(t *testing.T) {
	tests := []string{
		"",
		"http://github.com/dgrieser/nickpit/pull/60",
		"https://git.example.com/dgrieser/nickpit/pull/60",
		"https://github.com/dgrieser/nickpit/issues/60",
		"https://github.com/dgrieser/pull/60",
		"https://github.com/dgrieser/nickpit/pull/not-a-number",
		"https://github.com/dgrieser/nickpit/pull/0",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, _, err := parseGitHubPRURL(raw); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseGitLabMRURL(t *testing.T) {
	tests := []struct {
		raw     string
		repo    string
		id      int
		baseURL string
	}{
		{
			raw:     "https://gitlab.mittwald.it/asylum/services/kopieerapparaat/-/merge_requests/366",
			repo:    "asylum/services/kopieerapparaat",
			id:      366,
			baseURL: "https://gitlab.mittwald.it",
		},
		{
			raw:     "https://gitlab.mittwald.it/asylum/services/kopieerapparaat/-/merge_requests/366/diffs?file_path=pkg%2Frestic%2Frestic-cmd-directory-target-dir-overwrite.tpl#line_5578594d5_3",
			repo:    "asylum/services/kopieerapparaat",
			id:      366,
			baseURL: "https://gitlab.mittwald.it",
		},
		{
			raw:     "http://localhost:8080/group/project/-/merge_requests/7/commits",
			repo:    "group/project",
			id:      7,
			baseURL: "http://localhost:8080",
		},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			repo, id, baseURL, err := parseGitLabMRURL(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if repo != tt.repo || id != tt.id || baseURL != tt.baseURL {
				t.Fatalf("parseGitLabMRURL() = %q, %d, %q; want %q, %d, %q", repo, id, baseURL, tt.repo, tt.id, tt.baseURL)
			}
		})
	}
}

func TestParseGitLabMRURLRejectsInvalid(t *testing.T) {
	tests := []string{
		"",
		"ssh://gitlab.mittwald.it/group/project/-/merge_requests/366",
		"https:///group/project/-/merge_requests/366",
		"https://gitlab.mittwald.it/group/project/merge_requests/366",
		"https://gitlab.mittwald.it/-/merge_requests/366",
		"https://gitlab.mittwald.it/group/project/-/merge_requests/not-a-number",
		"https://gitlab.mittwald.it/group/project/-/merge_requests/0",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, _, _, err := parseGitLabMRURL(raw); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRemoteURLFlagValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "github url with id",
			args: []string{"github", "pr", "--url", "https://github.com/dgrieser/nickpit/pull/60", "--id", "60"},
			want: "--url can not be combined with --id",
		},
		{
			name: "github url with repo",
			args: []string{"github", "pr", "--url", "https://github.com/dgrieser/nickpit/pull/60", "--repo", "dgrieser/nickpit"},
			want: "--url can not be combined with --repo",
		},
		{
			name: "github missing id",
			args: []string{"github", "pr", "--repo", "dgrieser/nickpit"},
			want: "--id must be a positive integer",
		},
		{
			name: "gitlab url with id",
			args: []string{"gitlab", "mr", "--url", "https://gitlab.mittwald.it/asylum/services/kopieerapparaat/-/merge_requests/366", "--id", "366"},
			want: "--url can not be combined with --id",
		},
		{
			name: "gitlab url with repo",
			args: []string{"gitlab", "mr", "--url", "https://gitlab.mittwald.it/asylum/services/kopieerapparaat/-/merge_requests/366", "--repo", "asylum/services/kopieerapparaat"},
			want: "--url can not be combined with --repo",
		},
		{
			name: "gitlab missing id",
			args: []string{"gitlab", "mr", "--repo", "asylum/services/kopieerapparaat"},
			want: "--id must be a positive integer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
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
	wantAgent := "] Structured no nudges, ≤2 retries, ∞ reasoning, ∞ loop repeats, no rate-limit-delay, ∞ concurrency, ∞ tool calls, parallel, ≤5 duplicates"
	if !strings.Contains(stderr, wantAgent) {
		t.Fatalf("stderr missing agent progress line\nwant: %s\nstderr:\n%s", wantAgent, stderr)
	}
	// The Agent bracket describes the workflow (embedded default), not the model.
	wantWorkflow := "Agent      [default · embedded · "
	if !strings.Contains(stderr, wantWorkflow) || !strings.Contains(stderr, " steps] Structured no nudges") {
		t.Fatalf("stderr missing agent workflow bracket\nwant prefix: %s\nstderr:\n%s", wantWorkflow, stderr)
	}
}

func TestRunReviewShowProgressPrintsSmallModelWhenDifferent(t *testing.T) {
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
		_ = (&app{showProgress: true}).runReview(context.Background(), &recordingSource{}, nil, "mittwald", config.Profile{
			Model:            "Primary-Big",
			BaseURL:          server.URL,
			APIKey:           "token",
			ReasoningEffort:  "high",
			MaxContextTokens: 120000,
			UseJSONSchema:    true,
			Small:            config.SmallModelConfig{Model: "Small-Fast", ReasoningEffort: "low"},
		}, model.ReviewRequest{
			Mode:              model.ModeLocal,
			RepoRoot:          t.TempDir(),
			UseJSONSchema:     true,
			PriorityThreshold: "p3",
		})
	})

	wantPrimary := "Model      [Primary-Big:high @ " + server.URL + "] ready 120k context"
	if !strings.Contains(stderr, wantPrimary) {
		t.Fatalf("stderr missing primary model line\nwant: %s\nstderr:\n%s", wantPrimary, stderr)
	}
	wantSmall := "Model      [@small Small-Fast:low @ " + server.URL + "] ready 120k context"
	if !strings.Contains(stderr, wantSmall) {
		t.Fatalf("stderr missing small model line\nwant: %s\nstderr:\n%s", wantSmall, stderr)
	}
}

func TestRunReviewShowProgressOmitsSmallModelWhenSame(t *testing.T) {
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
		_ = (&app{showProgress: true}).runReview(context.Background(), &recordingSource{}, nil, "mittwald", config.Profile{
			Model:            "Primary-Big",
			BaseURL:          server.URL,
			APIKey:           "token",
			ReasoningEffort:  "high",
			MaxContextTokens: 120000,
			UseJSONSchema:    true,
		}, model.ReviewRequest{
			Mode:              model.ModeLocal,
			RepoRoot:          t.TempDir(),
			UseJSONSchema:     true,
			PriorityThreshold: "p3",
		})
	})

	// No small config → small inherits the primary model → only one Model line.
	if got := strings.Count(stderr, "] ready 120k context"); got != 1 {
		t.Fatalf("model ready lines = %d, want 1 (no separate small model)\nstderr:\n%s", got, stderr)
	}
}

func TestAgentSummaryFlagsAndOrder(t *testing.T) {
	profile := config.Profile{MaxRateLimitDelaySeconds: 300}
	req := model.ReviewRequest{
		UseJSONSchema:           true,
		NudgeCount:              3,
		MaxOutputRetries:        5,
		MaxReasoningSeconds:     300,
		MaxReasoningLoopRepeats: 5,
		MaxDuplicateToolCalls:   5,
		Concurrency:             15,
		SkipSuggestions:         true,
		DisablePatchSummary:     true,
		DisableReasoningExtract: true,
		VerifyDropPolicy:        "refuted-only",
		VerifyDropConfidence:    0.8,
		PriorityThreshold:       "p1",
	}
	got := agentSummary(profile, req)
	want := "Structured ≤3 nudges, ≤5 retries, ≤300s reasoning, ≤5 loop repeats, ≤300s rate-limit-delay, ≤15 concurrency, ∞ tool calls, parallel, ≤5 duplicates, skip suggestions, no patch summary, no reasoning extract, drop refuted-only ≥0.8, ≥p1"
	if got != want {
		t.Fatalf("agentSummary()\n got: %s\nwant: %s", got, want)
	}
}

func TestAgentSummaryOmitsDefaultsAndSerial(t *testing.T) {
	req := model.ReviewRequest{
		DisableParallelToolCalls: true,
		Concurrency:              10,
		VerifyDropPolicy:         "none",
		PriorityThreshold:        "p3",
	}
	got := agentSummary(config.Profile{}, req)
	want := "Unstructured no nudges, no retries, ∞ reasoning, ∞ loop repeats, no rate-limit-delay, ≤10 concurrency, ∞ tool calls, ∞ duplicates"
	if got != want {
		t.Fatalf("agentSummary()\n got: %s\nwant: %s", got, want)
	}
}

func TestModelSummaryRendersExtraBody(t *testing.T) {
	temp := 0.6
	maxTokens := 16384
	profile := config.Profile{
		MaxTokens:   &maxTokens,
		Temperature: &temp,
		ExtraBody: map[string]any{
			"chat_template_kwargs": map[string]any{"enable_thinking": true},
			"min_p":                0.05,
		},
	}
	got := modelSummary(profile, model.ReviewRequest{MaxContextTokens: 120000})
	want := "120k context, 16.4k output, temp=0.6, enable_thinking=true, min_p=0.05"
	if got != want {
		t.Fatalf("modelSummary() = %q, want %q", got, want)
	}
}

func TestModelSummaryOmitsOutputWhenUnset(t *testing.T) {
	got := modelSummary(config.Profile{}, model.ReviewRequest{MaxContextTokens: 120000})
	if want := "120k context"; got != want {
		t.Fatalf("modelSummary() = %q, want %q", got, want)
	}
}

func TestModelSummarySmallOutputBudgetNotTruncated(t *testing.T) {
	mt := 512
	got := modelSummary(config.Profile{MaxTokens: &mt}, model.ReviewRequest{MaxContextTokens: 120000})
	if want := "120k context, 512 output"; got != want {
		t.Fatalf("modelSummary() = %q, want %q", got, want)
	}
}

func TestFormatExtraBody(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]any
		want string // entries joined with "|"
	}{
		{name: "nil", in: nil, want: ""},
		{name: "scalars sorted", in: map[string]any{"repetition_penalty": 1.05, "min_p": 0.05}, want: "min_p=0.05|repetition_penalty=1.05"},
		{name: "nested unique leaf", in: map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": true}, "min_p": 0.05}, want: "enable_thinking=true|min_p=0.05"},
		{name: "colliding leaf uses full path", in: map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": true}, "chat_template_kwargs2": map[string]any{"enable_thinking": true}}, want: "chat_template_kwargs.enable_thinking=true|chat_template_kwargs2.enable_thinking=true"},
		{name: "array and bool", in: map[string]any{"stop": []any{"a", "b"}, "flag": false}, want: "flag=false|stop=[a, b]"},
		{name: "empty nested map", in: map[string]any{"opts": map[string]any{}}, want: "opts={}"},
		{name: "non-empty map in slice", in: map[string]any{"items": []any{map[string]any{"k": "v", "n": 1.0}}}, want: "items=[{k=v, n=1}]"},
		{name: "empty map in slice", in: map[string]any{"items": []any{map[string]any{}}}, want: "items=[{}]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(formatExtraBody(tt.in), "|"); got != tt.want {
				t.Fatalf("formatExtraBody() = %q, want %q", got, tt.want)
			}
		})
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
	profile := config.Profile{
		Model:           "model",
		BaseURL:         "http://127.0.0.1:1/",
		APIKey:          "token",
		ReasoningEffort: "high",
	}
	// Cache the capability under the same settings fingerprint the resolver
	// computes, so the hit avoids probing the dead endpoint.
	if err := modelcheck.WriteCachedCapability(filepath.Join(cacheDir, "model-capabilities.json"), profile.BaseURL, requestSettingsFingerprint(profile), compatibleCapability("model"), time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("source called")
	source := &recordingSource{err: wantErr}
	err := (&app{}).runReview(context.Background(), source, nil, "default", profile, model.ReviewRequest{Mode: model.ModeLocal, RepoRoot: t.TempDir()})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want source error", err)
	}
	if !source.called {
		t.Fatal("source should run when cached capabilities satisfy model check")
	}
}

func TestRunReviewReprobesWhenSettingsChange(t *testing.T) {
	var probes atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probes.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "reasoning_effort unsupported"}})
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	t.Setenv("NICKPIT_CACHE_DIR", cacheDir)
	// Capability cached under a DIFFERENT settings combination (no extra_body).
	cachedProfile := config.Profile{Model: "model", BaseURL: server.URL, ReasoningEffort: "high"}
	if err := modelcheck.WriteCachedCapability(filepath.Join(cacheDir, "model-capabilities.json"), cachedProfile.BaseURL, requestSettingsFingerprint(cachedProfile), compatibleCapability("model"), time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}

	// Same model/base_url but extra_body added → different fingerprint → the
	// cache must miss and the live probe runs (and fails against the test server).
	source := &recordingSource{}
	err := (&app{}).runReview(context.Background(), source, nil, "default", config.Profile{
		Model:            "model",
		BaseURL:          server.URL,
		APIKey:           "token",
		ReasoningEffort:  "high",
		MaxContextTokens: 120000,
		ExtraBody:        map[string]any{"enable_thinking": true},
	}, model.ReviewRequest{Mode: model.ModeLocal, RepoRoot: t.TempDir()})
	if err == nil {
		t.Fatal("expected model-check failure from live probe, got nil")
	}
	if probes.Load() == 0 {
		t.Fatal("changed settings must trigger a live probe; cache should have missed")
	}
	if source.called {
		t.Fatal("source should not run when the model check fails")
	}
}

func TestRequestSettingsFingerprint(t *testing.T) {
	withProfile := func(p config.Profile, fn func(*config.Profile)) config.Profile {
		fn(&p)
		return p
	}
	base := config.Profile{
		Model:           "model",
		ReasoningEffort: "high",
		ExtraBody:       map[string]any{"min_p": 0.05},
	}
	baseFP := requestSettingsFingerprint(base)
	if requestSettingsFingerprint(base) != baseFP {
		t.Fatal("fingerprint not stable for identical settings")
	}

	temp, topP, pp := 0.6, 0.9, 1.0
	topK, maxTok := 20, 16384
	variants := map[string]config.Profile{
		"model":            withProfile(base, func(p *config.Profile) { p.Model = "other" }),
		"reasoning_effort": withProfile(base, func(p *config.Profile) { p.ReasoningEffort = "low" }),
		"temperature":      withProfile(base, func(p *config.Profile) { p.Temperature = &temp }),
		"top_p":            withProfile(base, func(p *config.Profile) { p.TopP = &topP }),
		"top_k":            withProfile(base, func(p *config.Profile) { p.TopK = &topK }),
		"presence_penalty": withProfile(base, func(p *config.Profile) { p.PresencePenalty = &pp }),
		"max_tokens":       withProfile(base, func(p *config.Profile) { p.MaxTokens = &maxTok }),
		"extra_body":       withProfile(base, func(p *config.Profile) { p.ExtraBody = map[string]any{"min_p": 0.1} }),
	}
	for name, p := range variants {
		if requestSettingsFingerprint(p) == baseFP {
			t.Fatalf("changing %s did not change the fingerprint", name)
		}
	}

	// Equal extra_body content (independently constructed map) → same fingerprint.
	if eq := withProfile(base, func(p *config.Profile) { p.ExtraBody = map[string]any{"min_p": 0.05} }); requestSettingsFingerprint(eq) != baseFP {
		t.Fatal("equal extra_body content produced a different fingerprint")
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
