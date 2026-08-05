package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/modelcheck"
	"github.com/dgrieser/nickpit/internal/session"
	"github.com/dgrieser/nickpit/internal/workflow"
	"github.com/spf13/cobra"
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
    max_findings: 8
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
		maxFindings:                 0,
		maxFindingsSet:              true,
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
	if profile.MaxFindings != 0 {
		t.Fatalf("max findings = %d", profile.MaxFindings)
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

func TestLoadProfileAppliesMaxRequestBytesOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_request_bytes: 1000
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	app := &app{
		profile:            "default",
		configPath:         path,
		maxRequestBytes:    2048,
		maxRequestBytesSet: true,
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxRequestBytes != 2048 {
		t.Fatalf("max request bytes = %d, want 2048", profile.MaxRequestBytes)
	}
}

func TestLoadProfileAppliesMaxToolResultPercentOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    max_tool_result_percent: 10
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	app := &app{
		profile:                 "default",
		configPath:              path,
		maxToolResultPercent:    0,
		maxToolResultPercentSet: true,
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.MaxToolResultPercent != 0 {
		t.Fatalf("max tool result percent = %d, want explicit zero", profile.MaxToolResultPercent)
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

func TestLoadProfileAppendsStyleGuideCLIValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    styleguides: ["team.md"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	app := &app{
		profile:     "default",
		configPath:  path,
		styleGuides: []string{"https://example.com/rules.md"},
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	var sources []string
	for _, spec := range profile.StyleGuides {
		sources = append(sources, spec.Source)
	}
	if strings.Join(sources, ",") != "team.md,https://example.com/rules.md" {
		t.Fatalf("styleguides = %#v", profile.StyleGuides)
	}
}

func TestLoadProfileAppendsDisableStyleGuideCLIValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
profiles:
  default:
    model: test-model
    disable_styleguides: ["python"]
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	app := &app{
		profile:            "default",
		configPath:         path,
		disableStyleGuides: []string{" Go ", "python"},
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(profile.DisableStyleGuides, ",") != "python,go" {
		t.Fatalf("disable styleguides = %#v", profile.DisableStyleGuides)
	}

	app.disableStyleGuides = []string{"cobol"}
	_, _, err = app.loadProfile()
	if err == nil || !strings.Contains(err.Error(), `disable_styleguides[1] unknown language "cobol"`) || !strings.Contains(err.Error(), "go, python") {
		t.Fatalf("error = %v, want unknown language listing available ones", err)
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

func TestNormalizePriorityThreshold(t *testing.T) {
	for in, want := range map[string]string{"0": "p0", "1": "p1", "2": "p2", "3": "p3"} {
		got, err := model.NormalizePriorityThreshold(in)
		if err != nil {
			t.Fatalf("NormalizePriorityThreshold(%q) error: %v", in, err)
		}
		if got != want {
			t.Fatalf("NormalizePriorityThreshold(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{"p3", "4", "-1", "", "high"} {
		if _, err := model.NormalizePriorityThreshold(in); err == nil {
			t.Fatalf("NormalizePriorityThreshold(%q) expected error, got nil", in)
		}
	}
}

func TestLoadProfileAppliesDisableSuggestions(t *testing.T) {
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
		profile:            "default",
		configPath:         path,
		disableSuggestions: true,
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if !profile.DisableSuggestions {
		t.Fatal("expected skip suggestions CLI override")
	}
}

func TestLoadProfileAppliesDisableWorkflowTimeBudget(t *testing.T) {
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
		profile:                   "default",
		configPath:                path,
		disableWorkflowTimeBudget: true,
	}
	_, profile, err := app.loadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if !profile.DisableWorkflowTimeBudget {
		t.Fatal("expected skip workflow time budget CLI override")
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
	if cmd.PersistentFlags().Lookup("disable-model-check") == nil {
		t.Fatal("disable-model-check flag missing")
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
		"max-output-tokens",
		"max-request-bytes",
		"max-tool-result-percent",
		"small-max-output-tokens",
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
	diffScope := cmd.PersistentFlags().Lookup("disable-diff-scope")
	if diffScope == nil || diffScope.DefValue != "false" {
		t.Fatalf("disable-diff-scope flag = %#v, want default false", diffScope)
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

func TestGitLocalChangeCommandsPresent(t *testing.T) {
	cmd := newRootCmd()
	tests := []struct {
		args      []string
		wantShort string
	}{
		{
			args:      []string{"git", "uncommitted"},
			wantShort: "Review staged and unstaged tracked changes against HEAD; untracked files excluded",
		},
		{
			args:      []string{"git", "staged"},
			wantShort: "Review staged changes",
		},
		{
			args:      []string{"git", "unstaged"},
			wantShort: "Review unstaged tracked changes",
		},
		{
			args:      []string{"git", "commits"},
			wantShort: "Review a specific commit range",
		},
		{
			args:      []string{"git", "branch"},
			wantShort: "Review a branch against a base branch",
		},
	}

	for _, tt := range tests {
		found, _, err := cmd.Find(tt.args)
		if err != nil {
			t.Fatal(err)
		}
		if found == nil || found.Use != tt.args[len(tt.args)-1] {
			t.Fatalf("%s command missing: %#v", strings.Join(tt.args, " "), found)
		}
		if found.Short != tt.wantShort {
			t.Fatalf("%s short = %q, want %q", strings.Join(tt.args, " "), found.Short, tt.wantShort)
		}
	}
}

func TestInspectHistoryCommandsPresent(t *testing.T) {
	cmd := newRootCmd()
	tests := []struct {
		args      []string
		alias     string
		wantFlags []string
	}{
		{
			args:      []string{"inspect", "log"},
			alias:     "commits",
			wantFlags: []string{"commit", "since", "until", "author", "paths", "message", "message-regex", "case-sensitive", "limit"},
		},
		{
			args:      []string{"inspect", "show"},
			alias:     "commit",
			wantFlags: []string{"commit", "to", "paths", "max-commits"},
		},
	}

	for _, tt := range tests {
		found, _, err := cmd.Find(tt.args)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.Join(tt.args, " ")
		if found == nil || found.Use != tt.args[len(tt.args)-1] {
			t.Fatalf("%s command missing: %#v", name, found)
		}
		if !slices.Contains(found.Aliases, tt.alias) {
			t.Fatalf("%s aliases = %#v, want %q", name, found.Aliases, tt.alias)
		}
		for _, flag := range tt.wantFlags {
			if found.Flags().Lookup(flag) == nil {
				t.Fatalf("%s flag %q missing", name, flag)
			}
		}
	}

	// git_show cannot run without a revision, so the mirror command requires it.
	show, _, err := cmd.Find([]string{"inspect", "show"})
	if err != nil {
		t.Fatal(err)
	}
	annotations := show.Flags().Lookup("commit").Annotations[cobra.BashCompOneRequiredFlag]
	if !slices.Contains(annotations, "true") {
		t.Fatalf("inspect show commit flag should be required: %#v", annotations)
	}
}

func TestInspectReferencesCommandPresent(t *testing.T) {
	cmd := newRootCmd()
	references, _, err := cmd.Find([]string{"inspect", "references"})
	if err != nil {
		t.Fatal(err)
	}
	if references == nil || references.Use != "references" {
		t.Fatalf("inspect references command missing: %#v", references)
	}
	if references.Flags().Lookup("path") == nil || references.Flags().Lookup("symbol") == nil {
		t.Fatalf("inspect references flags missing")
	}
	annotations := references.Flags().Lookup("symbol").Annotations[cobra.BashCompOneRequiredFlag]
	if !slices.Contains(annotations, "true") {
		t.Fatalf("inspect references symbol flag should be required: %#v", annotations)
	}
	if pathAnnotations := references.Flags().Lookup("path").Annotations[cobra.BashCompOneRequiredFlag]; slices.Contains(pathAnnotations, "true") {
		t.Fatalf("inspect references path flag should be optional: %#v", pathAnnotations)
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
		{
			raw:  "https://www.github.com/dgrieser/nickpit/pull/60",
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
		"✓ Schema Enforcement",
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
	if strings.Contains(out, "Tool Use With Schema Enforcement") || strings.Contains(out, "Fallback to prompt-embedded schema") {
		t.Fatalf("output must omit combined-probe row and fallback line when neither applies\n%s", out)
	}
}

func TestWriteModelCheckOutputShowsCombinedProbeAndFallback(t *testing.T) {
	out := captureStdout(t, func() {
		err := (&app{}).writeModelCheckOutput("test-model", modelcheck.Result{
			DisableJSONResponseFormat: true,
			Probes: []modelcheck.ProbeResult{
				{Name: "configured_no_tools", ReasoningEffort: "high", Reasoned: true, Status: modelcheck.StatusOK},
				{Name: "configured_tools", ReasoningEffort: "high", Tools: true, Status: modelcheck.StatusOK},
				{Name: "configured_json_output", ReasoningEffort: "high", Status: modelcheck.StatusOK},
				{Name: "configured_json_schema", ReasoningEffort: "high", Status: modelcheck.StatusOK},
				{Name: "configured_tools_json_schema", ReasoningEffort: "high", Tools: true, Status: modelcheck.StatusFailed, Error: "model made no tool calls while the json_schema response format was active"},
			},
			PassedEfforts: []string{"high"},
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "! Tool Use With Schema Enforcement\n✓ Fallback to prompt-embedded schema\n") {
		t.Fatalf("fallback line must follow the combined-probe row\n%s", out)
	}
	if strings.Contains(out, "✗ Tool Use With Schema Enforcement") {
		t.Fatalf("combined-probe degrade must use the soft mark, not the failure cross\n%s", out)
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
	if !requirements.JSONSchema || requirements.JSONOutput {
		t.Fatalf("default small-model requirements = %+v, want JSON schema only", requirements)
	}

	result := modelcheck.Result{
		Probes: []modelcheck.ProbeResult{
			{Name: "configured_no_tools", ReasoningEffort: "low", Status: modelcheck.StatusOK},
			{Name: "configured_tools", ReasoningEffort: "low", Status: modelcheck.StatusUnsupported, Error: "tools unsupported"},
			{Name: "configured_json_output", ReasoningEffort: "low", Status: modelcheck.StatusOK},
			{Name: "configured_json_schema", ReasoningEffort: "low", Status: modelcheck.StatusOK},
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
	if !requirements.Tools || !requirements.JSONSchema || requirements.JSONOutput {
		t.Fatalf("review small-model requirements = %+v, want tools and JSON schema", requirements)
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

func TestSmallModelRequirementsForCategorizeAliasSkipTools(t *testing.T) {
	alias := workflow.SmallModelAlias
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{{
		Type:   workflow.StepVerifyPrefix + "security",
		Config: &workflow.StepOverride{Categorize: &workflow.AgentOverride{Model: &alias}},
	}}}
	small := smallModelRequirementsForSpec(spec, model.ReviewRequest{})
	if small.Tools || !small.JSONSchema || small.JSONOutput {
		t.Fatalf("categorize small-model requirements = %+v, want JSON schema without tools", small)
	}
	// The verifier itself stays on the primary model, which still needs tools.
	if primary := primaryModelRequirements(spec, model.ReviewRequest{}); !primary.Tools {
		t.Fatalf("primary requirements = %+v, want tools for the verify step", primary)
	}
}

func TestSmallModelRequirementsHonorSchemaOverride(t *testing.T) {
	alias := workflow.SmallModelAlias
	disable := true
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{{
		Type: workflow.StepFinalize,
		Config: &workflow.StepOverride{
			Model:                     &alias,
			DisableJSONResponseFormat: &disable,
		},
	}}}
	requirements := smallModelRequirementsForSpec(spec, model.ReviewRequest{})
	if !requirements.JSONOutput || requirements.JSONSchema || requirements.Tools {
		t.Fatalf("disable override requirements = %+v, want JSON output only", requirements)
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
			DisableJSONResponseFormat:  false,
		}, model.ReviewRequest{
			Mode:                      model.ModeLocal,
			RepoRoot:                  t.TempDir(),
			MaxOutputRetries:          2,
			MaxDuplicateToolCalls:     5,
			DisableJSONResponseFormat: false,
			PriorityThreshold:         "p3",
		})
		if err == nil || !strings.Contains(err.Error(), "model check failed") {
			t.Fatalf("error = %v, want model check failure", err)
		}
	})

	wantModel := "Model      [Qwen3.5-122B-A10B-FP8:high @ " + server.URL + "] ready 120k context"
	if !strings.Contains(stderr, wantModel) {
		t.Fatalf("stderr missing model progress line\nwant: %s\nstderr:\n%s", wantModel, stderr)
	}
	wantAgent := "] Structured no nudges, ≤2 retries, ∞ reasoning, no rate-limit-delay, ∞ concurrency, ∞ tool calls, parallel, ≤5 duplicates"
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
			Model:                     "Primary-Big",
			BaseURL:                   server.URL,
			APIKey:                    "token",
			ReasoningEffort:           "high",
			MaxContextTokens:          120000,
			DisableJSONResponseFormat: true,
			Small:                     config.SmallModelConfig{Model: "Small-Fast", ReasoningEffort: "low"},
		}, model.ReviewRequest{
			Mode:                      model.ModeLocal,
			RepoRoot:                  t.TempDir(),
			DisableJSONResponseFormat: true,
			PriorityThreshold:         "p3",
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
			Model:                     "Primary-Big",
			BaseURL:                   server.URL,
			APIKey:                    "token",
			ReasoningEffort:           "high",
			MaxContextTokens:          120000,
			DisableJSONResponseFormat: true,
		}, model.ReviewRequest{
			Mode:                      model.ModeLocal,
			RepoRoot:                  t.TempDir(),
			DisableJSONResponseFormat: true,
			PriorityThreshold:         "p3",
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
		DisableJSONResponseFormat: false,
		NudgeCount:                3,
		MaxFindings:               10,
		MaxOutputRetries:          5,
		MaxReasoningSeconds:       300,
		MaxDuplicateToolCalls:     5,
		Concurrency:               15,
		DisableSuggestions:        true,
		DisablePatchSummary:       true,
		DisableReasoningExtract:   true,
		VerifyDropPolicy:          model.DropPolicyRefutedOnly,
		ConfidenceThreshold:       0.7,
		PriorityThreshold:         "p1",
	}
	got := agentSummary(profile, req)
	want := "Structured ≤3 nudges, ≤5 retries, ≤300s reasoning, ≤300s rate-limit-delay, ≤15 concurrency, ∞ tool calls, ≤10 findings, parallel, ≤5 duplicates, no suggestions, no patch summary, no reasoning extract, drop refuted-only, confidence ≥0.7, ≥p1"
	if got != want {
		t.Fatalf("agentSummary()\n got: %s\nwant: %s", got, want)
	}
}

func TestAgentSummaryOmitsDefaultsAndSerial(t *testing.T) {
	req := model.ReviewRequest{
		DisableJSONResponseFormat: true,
		DisableParallelToolCalls:  true,
		Concurrency:               10,
		VerifyDropPolicy:          model.DropPolicyNone,
		PriorityThreshold:         "p3",
	}
	got := agentSummary(config.Profile{}, req)
	want := "Unstructured no nudges, no retries, ∞ reasoning, no rate-limit-delay, ≤10 concurrency, ∞ tool calls, ∞ duplicates"
	if got != want {
		t.Fatalf("agentSummary()\n got: %s\nwant: %s", got, want)
	}
}

func TestSpecHasStepFindsVerdictInPipeline(t *testing.T) {
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{
		{Pipeline: []workflow.StepEntry{
			{Type: workflow.StepMerge},
			{Type: workflow.StepFinalize},
			{Type: workflow.StepVerdict},
		}},
	}}
	if !specHasStep(spec, workflow.StepVerdict) {
		t.Fatal("specHasStep did not find verdict inside pipeline")
	}
	without := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{{Type: workflow.StepMerge}}}
	if specHasStep(without, workflow.StepVerdict) {
		t.Fatal("specHasStep found verdict in merge-only spec")
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
	err := (&app{disableModelCheck: true}).runReview(context.Background(), source, nil, "default", config.Profile{
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

func TestCacheableModelResult(t *testing.T) {
	capWith := func(mutate func(*config.ModelCapabilities)) modelcheck.Result {
		c := compatibleCapability("model")
		mutate(&c)
		return modelcheck.ResultFromCapability(c, false)
	}
	no := false
	tests := []struct {
		name   string
		result modelcheck.Result
		want   bool
	}{
		{"fully compatible", capWith(func(*config.ModelCapabilities) {}), true},
		// The P3 case: a model that cannot do API-enforced json_schema but can emit
		// plain JSON is still usable via the prompt-embedded fallback, so it must be
		// cached rather than re-probed every run.
		{"json_schema unsupported, json output ok", capWith(func(c *config.ModelCapabilities) { c.JSONSchema = &no }), true},
		{"no structured output at all", capWith(func(c *config.ModelCapabilities) { c.JSONSchema = &no; c.JSONResponse = &no }), false},
		{"tools unsupported", capWith(func(c *config.ModelCapabilities) { c.Tools = false }), false},
		{"no reasoning efforts", capWith(func(c *config.ModelCapabilities) { c.Reasoning.Efforts = nil }), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cacheableModelResult(tt.result); got != tt.want {
				t.Fatalf("cacheableModelResult = %v, want %v", got, tt.want)
			}
		})
	}
}

// A source-less spec (e.g. --step merge on imported findings) still calls the
// LLM, so the model probe must run when credentials are present — that is what
// lets the json_schema fallback apply. It must not be hard-failed by the model
// check, matching the deferred-credential design.
func TestRunReviewProbesModelForSourcelessSpec(t *testing.T) {
	var probes atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probes.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "reasoning_effort unsupported"}})
	}))
	defer server.Close()

	source := &recordingSource{}
	err := (&app{stepName: "merge"}).runReview(context.Background(), source, nil, "default", config.Profile{
		Model:           "model",
		BaseURL:         server.URL,
		APIKey:          "token",
		ReasoningEffort: "high",
	}, model.ReviewRequest{Mode: model.ModeLocal, RepoRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("source-less merge run returned err: %v", err)
	}
	if probes.Load() == 0 {
		t.Fatal("model probe did not run for source-less spec; json_schema fallback would be skipped")
	}
	if source.called {
		t.Fatal("source-less spec must not resolve a review source")
	}
}

// A model that lacks API-enforced json_schema but can emit plain JSON is usable
// via the prompt-embedded fallback. `check model` must report it healthy (exit
// zero), matching what a real review of the same model does.
func TestCheckModelDegradesWhenJSONResponseFormatUnsupported(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.yaml")
	cfg := `profiles:
  default:
    base_url: http://127.0.0.1:1
    api_key: token
    model: model
    reasoning_effort: high
    supported_models:
      - model: model
        compatible: true
        response: true
        tools: true
        json_response: true
        json_schema: false
        reasoning:
          traces: true
          efforts: [high]
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	checkCmd := (&app{configPath: cfgPath, profile: "default"}).newCheckCmd()
	checkCmd.SetArgs([]string{"model"})
	var err error
	_ = captureOutput(t, &os.Stdout, func() { err = checkCmd.ExecuteContext(context.Background()) })
	if err != nil {
		t.Fatalf("check model exited non-zero for a json_schema-incapable but JSON-capable model: %v", err)
	}
}

// When @small forces the whole run onto the prompt-embedded schema, the primary
// model — accepted on its json_schema probe alone — must be re-validated for
// plain JSON output, since its reviewers now run prompt-only. A primary that
// cannot emit plain JSON must fail the model check up front, not mid-review.
func TestRunReviewRevalidatesPrimaryWhenSmallForcesPromptSchema(t *testing.T) {
	no, yes := false, true
	primaryCap := config.ModelCapabilities{
		Model: "primary", Compatible: true, Response: true, Tools: true,
		JSONResponse: &no, // primary cannot do plain JSON output
		JSONSchema:   &yes,
		Reasoning:    config.ReasoningCapabilities{Traces: true, Efforts: []string{"high"}},
	}
	smallCap := config.ModelCapabilities{
		Model: "small", Compatible: true, Response: true, Tools: true,
		JSONResponse: &yes,
		JSONSchema:   &no, // small lacks json_schema → forces the global prompt fallback
		Reasoning:    config.ReasoningCapabilities{Traces: true, Efforts: []string{"high"}},
	}
	source := &recordingSource{}
	err := (&app{}).runReview(context.Background(), source, nil, "default", config.Profile{
		Model:           "primary",
		BaseURL:         "http://127.0.0.1:1",
		APIKey:          "token",
		ReasoningEffort: "high",
		Small:           config.SmallModelConfig{Model: "small"},
		SupportedModels: []config.ModelCapabilities{primaryCap, smallCap},
	}, model.ReviewRequest{Mode: model.ModeLocal, RepoRoot: t.TempDir()})
	if err == nil {
		t.Fatal("expected model check failure: primary cannot emit plain JSON after @small forced the prompt-embedded schema")
	}
	if !strings.Contains(err.Error(), "JSON text output") {
		t.Fatalf("error = %v, want primary JSON-output validation failure", err)
	}
	if source.called {
		t.Fatal("source must not run when the model check fails")
	}
}

// A primary-model step that turns response_format off (disable_json_response_format:
// true) runs prompt-only and needs plain JSON output. A primary that passes the
// json_schema probe but fails the plain-JSON probe must be rejected by the
// pre-review check, not pass it and fail inside that step.
func TestRunReviewValidatesPrimaryPromptOnlyStepRequirements(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "wf.yaml")
	spec := "version: 1\nsteps:\n  - type: collect-context\n  - type: review:security\n    config:\n      disable_json_response_format: true\n"
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	no, yes := false, true
	primaryCap := config.ModelCapabilities{
		Model: "primary", Compatible: true, Response: true, Tools: true,
		JSONResponse: &no, // cannot do plain JSON output
		JSONSchema:   &yes,
		Reasoning:    config.ReasoningCapabilities{Traces: true, Efforts: []string{"high"}},
	}
	source := &recordingSource{}
	err := (&app{specPath: specPath}).runReview(context.Background(), source, nil, "default", config.Profile{
		Model:           "primary",
		BaseURL:         "http://127.0.0.1:1",
		APIKey:          "token",
		ReasoningEffort: "high",
		SupportedModels: []config.ModelCapabilities{primaryCap},
	}, model.ReviewRequest{Mode: model.ModeLocal, RepoRoot: t.TempDir()})
	if err == nil {
		t.Fatal("expected model check failure: prompt-only primary step needs plain JSON output the model lacks")
	}
	if !strings.Contains(err.Error(), "JSON text output") {
		t.Fatalf("error = %v, want primary JSON-output validation failure", err)
	}
	if source.called {
		t.Fatal("source must not run when the model check fails")
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
	toolsJSONSchema := true
	return config.ModelCapabilities{
		Model:           modelName,
		Compatible:      true,
		Response:        true,
		Tools:           true,
		JSONResponse:    &jsonResponse,
		JSONSchema:      &jsonSchema,
		ToolsJSONSchema: &toolsJSONSchema,
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

func TestLiveProgressEnabledOnlyForPlainTTY(t *testing.T) {
	tests := []struct {
		name                                                string
		tty                                                 bool
		term                                                string
		verbose, progress, reasoning, disableLive, expected bool
	}{
		{name: "plain tty", tty: true, term: "xterm-256color", expected: true},
		{name: "pipe", tty: false, term: "xterm-256color"},
		{name: "dumb terminal", tty: true, term: "dumb"},
		{name: "verbose", tty: true, term: "xterm", verbose: true},
		{name: "explicit progress", tty: true, term: "xterm", progress: true},
		{name: "reasoning", tty: true, term: "xterm", reasoning: true},
		{name: "disable-live-progress flag", tty: true, term: "xterm-256color", disableLive: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := liveProgressEnabled(tt.tty, tt.term, tt.verbose, tt.progress, tt.reasoning, tt.disableLive); got != tt.expected {
				t.Fatalf("liveProgressEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// goreleaser injects the tag without its leading "v", so a release renders clean
// (its tag already pins the commit). An untagged build carries the short commit,
// because "dev" alone identifies nothing in a bug report.
func TestDisplayVersion(t *testing.T) {
	originalVersion, originalCommit := version, commit
	t.Cleanup(func() { version, commit = originalVersion, originalCommit })
	commit = "995c9107a8bd4cf0"
	buildRevision = func() string { return shortCommit(commit) }
	t.Cleanup(func() { buildRevision = sync.OnceValue(resolveBuildRevision) })

	for _, tt := range []struct{ build, want string }{
		{"0.0.14", "v0.0.14"},
		{"1.2.3-rc1", "v1.2.3-rc1"},
		{"v0.0.14", "v0.0.14"},
		{" dev ", "dev+995c910"},
		{"", "dev+995c910"},
		{"feature-branch", "feature-branch+995c910"},
	} {
		version = tt.build
		if got := displayVersion(); got != tt.want {
			t.Fatalf("displayVersion(%q) = %q, want %q", tt.build, got, tt.want)
		}
	}

	// No revision anywhere (no ldflags, no embedded build info): the bare name.
	buildRevision = func() string { return "" }
	version = "dev"
	if got := displayVersion(); got != "dev" {
		t.Fatalf("revisionless dev build = %q", got)
	}
}

// The revision falls back from -X main.commit to Go's embedded VCS stamp, keeps
// seven characters, and flags a modified tree.
func TestResolveBuildRevision(t *testing.T) {
	originalCommit := commit
	t.Cleanup(func() { commit = originalCommit })

	commit = "995c9107a8bd4cf0deadbeef"
	if got := resolveBuildRevision(); got != "995c910" {
		t.Fatalf("ldflags commit = %q, want the short form", got)
	}
	commit = "995c910"
	if got := resolveBuildRevision(); got != "995c910" {
		t.Fatalf("an already-short ldflags commit must pass through: %q", got)
	}

	// The Makefile appends "-dirty" to the stamped commit, so truncation must
	// step around the suffix instead of eating it — reporting a modified tree as
	// clean is worse than not reporting it at all.
	commit = "995c9107a8bd4cf0deadbeef-dirty"
	if got := resolveBuildRevision(); got != "995c910-dirty" {
		t.Fatalf("dirty ldflags commit = %q, want the short form with the suffix", got)
	}
	commit = "995c910-dirty"
	if got := resolveBuildRevision(); got != "995c910-dirty" {
		t.Fatalf("an already-short dirty commit must pass through: %q", got)
	}
	// A bare suffix names no revision at all.
	commit = "-dirty"
	if got := resolveBuildRevision(); got == "-dirty" {
		t.Fatalf("a commitless -dirty must not render as a revision: %q", got)
	}

	// Unstamped: whatever the test binary's own build info says. `go test` embeds
	// VCS info in a git checkout, so assert the shape rather than a fixed hash.
	commit = ""
	rev := resolveBuildRevision()
	if rev != "" {
		bare := strings.TrimSuffix(rev, "-dirty")
		if len(bare) != 7 {
			t.Fatalf("embedded revision %q should be a 7-char hash (plus optional -dirty)", rev)
		}
	}
}

// emitResult is the one funnel every consumer sits behind, so stamping the build
// there puts it in the JSON on stdout and in the saved session at once (and, for
// a published review, in the hidden SCM envelope rendered from the same result).
func TestEmitResultStampsVersionForOutputAndSession(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })
	version = "3.2.1"

	dir := t.TempDir()
	a := &app{sessionDir: dir, jsonOutput: true, logger: logging.New(io.Discard, false, false)}
	req := model.ReviewRequest{Mode: model.ModeLocal, RepoRoot: dir}
	result := &model.ReviewResult{ReviewID: "rev-emit", OverallCorrectness: "patch is correct"}

	var err error
	out := captureStdout(t, func() {
		err = a.emitResult(context.Background(), nil, config.Profile{}, req, result, nil, "")
	})
	if err != nil {
		t.Fatalf("emitResult: %v", err)
	}
	var payload map[string]any
	if jsonErr := json.Unmarshal([]byte(out), &payload); jsonErr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", jsonErr, out)
	}
	if got := payload["nickpit_version"]; got != "v3.2.1" {
		t.Fatalf("stdout nickpit_version = %#v", got)
	}

	store, storeErr := session.NewStore(dir)
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	sess, latestErr := store.Latest()
	if latestErr != nil || sess == nil {
		t.Fatalf("no session saved: %v", latestErr)
	}
	if sess.Result == nil || sess.Result.NickpitVersion != "v3.2.1" {
		t.Fatalf("session result version = %+v", sess.Result)
	}
}

// The logger every command logs through carries the flags AND the version, so a
// new command cannot forget to name its build.
func TestNewLoggerCarriesFlagsAndVersion(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })
	version = "9.9.9"
	a := &app{verbose: true, showProgress: true}
	logger := a.newLogger()
	if got := logger.Version(); got != "v9.9.9" {
		t.Fatalf("logger version = %q", got)
	}
	if !logger.Enabled() {
		t.Fatal("newLogger dropped --verbose")
	}
}

func TestChatSessionHintOnlyForSavedTerminalSession(t *testing.T) {
	want := "\n---\n\nTo chat about this review, or copy it to the clipboard, run:\n" +
		"nickpit chat --session abc-123\nnickpit session abc-123 --clipboard"
	if got := chatSessionHint("abc-123", true, false, 80); got != want {
		t.Fatalf("chatSessionHint() = %q", got)
	}
	colored := chatSessionHint("abc-123", true, true, 12)
	// Both lines share the periwinkle command color; only the subcommand that
	// tells them apart is lifted to the lighter shade.
	if !strings.Contains(colored, "\x1b[2m────────────\x1b[0m") ||
		!strings.Contains(colored, "\x1b[2mTo chat about this review, or copy it to the clipboard, run:\x1b[0m") ||
		!strings.Contains(colored, "\x1b[38;2;179;189;255mnickpit \x1b[38;2;179;209;250mchat\x1b[38;2;179;189;255m --session abc-123\x1b[0m") ||
		!strings.Contains(colored, "\x1b[38;2;179;189;255mnickpit \x1b[38;2;179;209;250msession\x1b[38;2;179;189;255m abc-123 --clipboard\x1b[0m") {
		t.Fatalf("colored chat hint = %q", colored)
	}
	if strings.Contains(colored, "48;2;40;42;64") {
		t.Fatalf("chat command should not carry a block background: %q", colored)
	}
	if got := chatSessionHint("", true, false, 80); got != "" {
		t.Fatalf("empty session hint = %q", got)
	}
	if got := chatSessionHint("abc-123", false, false, 80); got != "" {
		t.Fatalf("non-terminal hint = %q", got)
	}
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

func TestGitLabServeCommandFlags(t *testing.T) {
	cmd := newRootCmd()
	found, _, err := cmd.Find([]string{"gitlab", "serve"})
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"serve-config", "listen", "review-concurrency", "serve-log-dir", "shutdown-grace"} {
		if found.Flags().Lookup(flag) == nil {
			t.Fatalf("serve command missing --%s flag", flag)
		}
	}
}

func TestGitLabServeFailsWithoutConfig(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gitlab", "serve", "--serve-config", filepath.Join(t.TempDir(), "absent.yaml")})
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "serve config") {
		t.Fatalf("err = %v, want serve config error", err)
	}
}

func TestIsUserAbort(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	if !isUserAbort(canceled, errors.New("llm: request failed")) {
		t.Fatal("a canceled context should be treated as a user abort regardless of the surfaced error")
	}
	if !isUserAbort(context.Background(), fmt.Errorf("review run aborted: %w", context.Canceled)) {
		t.Fatal("an error wrapping context.Canceled should be a user abort")
	}
	if isUserAbort(context.Background(), errors.New("review failed: all reviewer agents errored")) {
		t.Fatal("a genuine failure with a live context should not be a user abort")
	}
	if isUserAbort(context.Background(), context.DeadlineExceeded) {
		t.Fatal("a deadline/timeout is not a user abort")
	}
}

func TestApplyEnvDefaults(t *testing.T) {
	t.Setenv("NICKPIT_CONFIG", "env.nickpit.yaml")
	t.Setenv("NICKPIT_SESSION_DIR", "/tmp/env-sessions")
	t.Setenv("NICKPIT_OUTPUT", "json")
	t.Setenv("NICKPIT_PRIORITY_THRESHOLD", "1")
	t.Setenv("NICKPIT_VERIFY_DROP_POLICY", "refuted-only")
	t.Setenv("NICKPIT_CONFIDENCE_THRESHOLD", "0.42")
	t.Setenv("NICKPIT_SPEC", "env-workflow.yaml")

	cli := &app{configPath: config.DefaultConfigPath, outputFormat: "markdown", priorityThreshold: "3", confidenceThreshold: 0.7}
	if err := cli.applyEnvDefaults(func(string) bool { return false }); err != nil {
		t.Fatal(err)
	}
	if cli.configPath != "env.nickpit.yaml" {
		t.Fatalf("config path = %q", cli.configPath)
	}
	if cli.sessionDir != "/tmp/env-sessions" {
		t.Fatalf("session dir = %q", cli.sessionDir)
	}
	if cli.outputFormat != "json" {
		t.Fatalf("output format = %q", cli.outputFormat)
	}
	if cli.priorityThreshold != "1" {
		t.Fatalf("priority threshold = %q", cli.priorityThreshold)
	}
	if cli.verifyDropPolicy != "refuted-only" {
		t.Fatalf("verify drop policy = %q", cli.verifyDropPolicy)
	}
	if cli.confidenceThreshold != 0.42 {
		t.Fatalf("confidence threshold = %v", cli.confidenceThreshold)
	}
	if cli.specPath != "env-workflow.yaml" {
		t.Fatalf("spec path = %q", cli.specPath)
	}
}

func TestApplyEnvDefaultsKeepsExplicitFlags(t *testing.T) {
	t.Setenv("NICKPIT_CONFIG", "env.nickpit.yaml")
	t.Setenv("NICKPIT_OUTPUT", "json")
	t.Setenv("NICKPIT_CONFIDENCE_THRESHOLD", "0.42")

	cli := &app{configPath: "flag.nickpit.yaml", outputFormat: "raw", confidenceThreshold: 0.9}
	if err := cli.applyEnvDefaults(func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if cli.configPath != "flag.nickpit.yaml" || cli.outputFormat != "raw" || cli.confidenceThreshold != 0.9 {
		t.Fatalf("flags overwritten: config=%q output=%q confidence=%v", cli.configPath, cli.outputFormat, cli.confidenceThreshold)
	}
}

// An explicit --step must suppress an ambient NICKPIT_SPEC: the two are mutually
// exclusive, so injecting the spec would turn a valid single-step run into an error.
func TestApplyEnvDefaultsSkipsSpecEnvWhenStepGiven(t *testing.T) {
	t.Setenv("NICKPIT_SPEC", "env-workflow.yaml")

	cli := &app{stepName: "merge"}
	if err := cli.applyEnvDefaults(func(name string) bool { return name == "step" }); err != nil {
		t.Fatal(err)
	}
	if cli.specPath != "" {
		t.Fatalf("spec path = %q, want empty", cli.specPath)
	}
}

func TestApplyEnvDefaultsRejectsInvalidConfidenceThreshold(t *testing.T) {
	t.Setenv("NICKPIT_CONFIDENCE_THRESHOLD", "high")

	err := (&app{}).applyEnvDefaults(func(string) bool { return false })
	if err == nil || !strings.Contains(err.Error(), "NICKPIT_CONFIDENCE_THRESHOLD") {
		t.Fatalf("err = %v, want NICKPIT_CONFIDENCE_THRESHOLD parse error", err)
	}
}

func TestHunkLineRangeIsInclusive(t *testing.T) {
	tests := []struct {
		name string
		hunk model.DiffHunk
		want string
	}{
		{name: "multi line", hunk: model.DiffHunk{NewStart: 10, NewLines: 3}, want: "10-12"},
		{name: "single line", hunk: model.DiffHunk{NewStart: 10, NewLines: 1}, want: "10-10"},
		{name: "pure deletion", hunk: model.DiffHunk{NewStart: 10, NewLines: 0}, want: "10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hunkLineRange(tt.hunk); got != tt.want {
				t.Fatalf("hunkLineRange(%d,%d) = %q, want %q", tt.hunk.NewStart, tt.hunk.NewLines, got, tt.want)
			}
		})
	}
}

// inspect log/show read git, not an LLM, so they must work on an installation
// that has no model configured — like the other inspect subcommands, which load
// no profile at all.
func TestInspectHistoryWorksWithoutAnLLMProfile(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, key := range []string{"NICKPIT_MODEL", "NICKPIT_BASE_URL", "NICKPIT_PROFILE", "NICKPIT_CONFIG"} {
		t.Setenv(key, "")
	}

	a := &app{}
	repoRoot, profile, history, err := a.inspectHistory(context.Background())
	if err != nil {
		t.Fatalf("inspectHistory without a model: %v", err)
	}
	if repoRoot == "" || history == nil {
		t.Fatalf("repoRoot = %q, history = %#v", repoRoot, history)
	}
	if profile.Model != "" {
		t.Fatalf("profile unexpectedly carries a model: %q", profile.Model)
	}
	// The diff format still comes from the normalized profile, so `inspect show`
	// keeps returning the configured shape.
	if profile.DiffFormat != model.DiffFormatGit {
		t.Fatalf("diff format = %q, want the git default", profile.DiffFormat)
	}
}

// inspect log/show document --paths as repository-relative, so running them from
// a subdirectory must still anchor git at the repository root.
func TestInspectHistoryAnchorsAtTheRepositoryRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "--quiet", "."},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	sub := filepath.Join(root, "internal", "config")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	for _, key := range []string{"NICKPIT_MODEL", "NICKPIT_BASE_URL", "NICKPIT_PROFILE", "NICKPIT_CONFIG"} {
		t.Setenv(key, "")
	}

	a := &app{}
	repoRoot, _, _, err := a.inspectHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	gotRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != wantRoot {
		t.Fatalf("repo root = %q, want the repository root %q", gotRoot, wantRoot)
	}

	// Outside any repository the current directory is kept, so the provider
	// reports the missing repository itself.
	outside := t.TempDir()
	t.Chdir(outside)
	repoRoot, _, _, err = a.inspectHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantOutside, err := filepath.EvalSymlinks(outside)
	if err != nil {
		t.Fatal(err)
	}
	gotOutside, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if gotOutside != wantOutside {
		t.Fatalf("repo root = %q, want the current directory %q", gotOutside, wantOutside)
	}
}
