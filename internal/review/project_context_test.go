package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/tokenestimate"
	"github.com/dgrieser/nickpit/internal/workflow"
	"github.com/dgrieser/nickpit/prompts"
)

func fullProjectContext() *model.ProjectContext {
	return &model.ProjectContext{
		Version:         1,
		Summary:         "Multi-tenant billing API",
		Deployment:      "internet-facing",
		Users:           "authenticated customers",
		Criticality:     "high",
		Data:            []string{"payment card data"},
		TrustBoundaries: []string{"internal/api accepts untrusted bodies"},
		Assumptions:     []string{"the proxy terminates TLS"},
		NonGoals:        []string{"single region only"},
		Notes:           "Anything the fields miss.",
		Sources:         []string{".nickpit/context.yaml"},
	}
}

func newProjectContextEngine(t *testing.T) *Engine {
	t.Helper()
	return NewEngine(stubSource{}, &capturingLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
}

func TestProjectContextSnippetRendersEveryField(t *testing.T) {
	engine := newProjectContextEngine(t)
	got, err := engine.renderStyleGuideToolchainSnippet("review", nil, false, fullProjectContext())
	if err != nil {
		t.Fatalf("renderStyleGuideToolchainSnippet returned err: %v", err)
	}
	for _, want := range []string{
		"## PROJECT CONTEXT",
		"- Summary: Multi-tenant billing API",
		"- Deployment: internet-facing",
		"- Users: authenticated customers",
		"- Criticality: high",
		"- Sensitive data handled:",
		"  - payment card data",
		"- Trust boundaries:",
		"  - internal/api accepts untrusted bodies",
		"- Environment assumptions:",
		"  - the proxy terminates TLS",
		"- Declared non-goals:",
		"  - single region only",
		"Anything the fields miss.",
		"Declared in: .nickpit/context.yaml",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("snippet missing %q:\n%s", want, got)
		}
	}
}

// The context is repo-authored, so the prompt has to say that it calibrates a
// finding rather than removing one. Without these lines a project could talk
// the reviewers out of an entire class of issue.
func TestProjectContextSnippetCarriesAntiSuppressionGuidance(t *testing.T) {
	engine := newProjectContextEngine(t)
	got, err := engine.renderStyleGuideToolchainSnippet("security", nil, false, fullProjectContext())
	if err != nil {
		t.Fatalf("renderStyleGuideToolchainSnippet returned err: %v", err)
	}
	for _, want := range []string{
		"NOT rules",
		"NEVER let it remove a finding",
		"report the issue anyway",
		"not as verified fact",
		"prefer what the code shows",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("snippet missing anti-suppression guidance %q:\n%s", want, got)
		}
	}
}

func TestProjectContextSnippetOmittedWhenAbsent(t *testing.T) {
	engine := newProjectContextEngine(t)
	tests := map[string]*model.ProjectContext{
		"nil":                nil,
		"zero value":         {},
		"version only":       {Version: 1},
		"provenance only":    {Sources: []string{"ops.yaml"}},
		"whitespace scalars": {Summary: ""},
	}
	for name, pc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := engine.renderStyleGuideToolchainSnippet("review", nil, false, pc)
			if err != nil {
				t.Fatalf("renderStyleGuideToolchainSnippet returned err: %v", err)
			}
			if got != "" {
				t.Fatalf("snippet = %q, want empty so no bare heading is emitted", got)
			}
		})
	}
}

// The snippet also carries styleguides, and the two must stay distinguishable:
// styleguides are rules, project context is not.
func TestProjectContextAndStyleGuidesCoexist(t *testing.T) {
	engine := newProjectContextEngine(t)
	guides := []model.StyleGuide{{Language: "go", Content: "### Guide\nrule text"}}
	got, err := engine.renderStyleGuideToolchainSnippet("review", guides, true, fullProjectContext())
	if err != nil {
		t.Fatalf("renderStyleGuideToolchainSnippet returned err: %v", err)
	}
	project := strings.Index(got, "## PROJECT CONTEXT")
	style := strings.Index(got, "## STYLEGUIDES")
	if project < 0 || style < 0 {
		t.Fatalf("snippet missing a heading:\n%s", got)
	}
	if project > style {
		t.Fatalf("project context must precede the styleguides:\n%s", got)
	}
	if !strings.Contains(got, "They are rules to follow, not background context.") {
		t.Fatalf("styleguide framing lost:\n%s", got)
	}
	if !strings.Contains(got, "rule text") {
		t.Fatalf("styleguide content lost:\n%s", got)
	}
	if !strings.Contains(got, "toolchain_versions") {
		t.Fatalf("toolchain instruction lost:\n%s", got)
	}
}

// Every judging role renders this one snippet, so a role whose system template
// lost the placeholder would silently stop receiving the project context.
func TestEveryJudgingRoleTemplateCarriesTheSnippet(t *testing.T) {
	roles := map[string]string{
		"review":   "agent_review_general_system_prompt.tmpl",
		"verify":   "agent_verify_system_prompt.tmpl",
		"dedupe":   "agent_dedupe_system_prompt.tmpl",
		"merge":    "agent_cluster_merge_system_prompt.tmpl",
		"context":  "agent_context_system_prompt.tmpl",
		"finalize": "agent_finalize_system_prompt.tmpl",
		"verdict":  "agent_verdict_system_prompt.tmpl",
		"discuss":  "agent_discuss_system_prompt.tmpl",
	}
	engine := newProjectContextEngine(t)
	for role, file := range roles {
		t.Run(role, func(t *testing.T) {
			body, err := prompts.Load(file)
			if err != nil {
				t.Fatalf("loading %s: %v", file, err)
			}
			if !strings.Contains(body, "{{.StyleGuideToolchainSnippet}}") {
				t.Fatalf("%s no longer renders StyleGuideToolchainSnippet, so it cannot receive the project context", file)
			}
			got, err := engine.renderStyleGuideToolchainSnippet(role, nil, false, fullProjectContext())
			if err != nil {
				t.Fatalf("renderStyleGuideToolchainSnippet(%s) returned err: %v", role, err)
			}
			if !strings.Contains(got, "## PROJECT CONTEXT") {
				t.Fatalf("role %s got no project context:\n%s", role, got)
			}
		})
	}
}

func TestReviewSystemPromptCarriesProjectContext(t *testing.T) {
	engine := newProjectContextEngine(t)
	template, err := prompts.Load("agent_review_general_system_prompt.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	system, err := engine.renderReviewSystemWithFocus(template, "", model.ReviewRequest{}, true, "review", nil, false, fullProjectContext())
	if err != nil {
		t.Fatalf("renderReviewSystemWithFocus returned err: %v", err)
	}
	if !strings.Contains(system, "## PROJECT CONTEXT") || !strings.Contains(system, "internet-facing") {
		t.Fatalf("review system prompt missing project context:\n%s", system)
	}

	without, err := engine.renderReviewSystemWithFocus(template, "", model.ReviewRequest{}, true, "review", nil, false, nil)
	if err != nil {
		t.Fatalf("renderReviewSystemWithFocus returned err: %v", err)
	}
	if strings.Contains(without, "PROJECT CONTEXT") {
		t.Fatalf("review system prompt mentions project context with none set:\n%s", without)
	}
}

func TestCaptureProjectContextMergesRepoAndOverlay(t *testing.T) {
	engine := newProjectContextEngine(t)
	engine.SetProjectContextLoader(func(context.Context, model.ReviewSource, model.ReviewRequest) (*model.ProjectContext, []string) {
		return &model.ProjectContext{Summary: "repo", Deployment: "internal", Sources: []string{".nickpit/context.yaml"}}, nil
	})
	engine.SetProjectContextOverlay([]*model.ProjectContext{
		{Deployment: "internet-facing", Sources: []string{"ops.yaml"}},
	})

	reviewCtx := sampleReviewCtx()
	engine.captureProjectContext(context.Background(), reviewCtx, model.ReviewRequest{})

	if reviewCtx.ProjectContext == nil {
		t.Fatal("ProjectContext = nil, want the merged context")
	}
	if reviewCtx.ProjectContext.Summary != "repo" {
		t.Fatalf("summary = %q, want the repo value", reviewCtx.ProjectContext.Summary)
	}
	if reviewCtx.ProjectContext.Deployment != "internet-facing" {
		t.Fatalf("deployment = %q, want the operator overlay to win", reviewCtx.ProjectContext.Deployment)
	}
}

func TestCaptureProjectContextRecordsWarnings(t *testing.T) {
	engine := newProjectContextEngine(t)
	engine.SetProjectContextLoader(func(context.Context, model.ReviewSource, model.ReviewRequest) (*model.ProjectContext, []string) {
		return nil, []string{"project context ignored: bad yaml"}
	})

	reviewCtx := sampleReviewCtx()
	engine.captureProjectContext(context.Background(), reviewCtx, model.ReviewRequest{})

	if reviewCtx.ProjectContext != nil {
		t.Fatalf("ProjectContext = %+v, want none", reviewCtx.ProjectContext)
	}
	if len(reviewCtx.OmittedSections) != 1 || !strings.Contains(reviewCtx.OmittedSections[0], "bad yaml") {
		t.Fatalf("OmittedSections = %v, want the warning recorded", reviewCtx.OmittedSections)
	}
}

// Disabling distrusts the repository; it does not turn the feature off, so an
// operator entry must still land.
func TestCaptureProjectContextDisabledKeepsOverlay(t *testing.T) {
	engine := newProjectContextEngine(t)
	called := false
	engine.SetProjectContextLoader(func(context.Context, model.ReviewSource, model.ReviewRequest) (*model.ProjectContext, []string) {
		called = true
		return &model.ProjectContext{Summary: "repo claims"}, nil
	})
	engine.SetProjectContextOverlay([]*model.ProjectContext{{Summary: "operator says"}})
	engine.SetDisableRepoProjectContext(true)

	reviewCtx := sampleReviewCtx()
	engine.captureProjectContext(context.Background(), reviewCtx, model.ReviewRequest{})

	if called {
		t.Fatal("repository loader ran with the repo context disabled")
	}
	if reviewCtx.ProjectContext == nil || reviewCtx.ProjectContext.Summary != "operator says" {
		t.Fatalf("ProjectContext = %+v, want the operator entry", reviewCtx.ProjectContext)
	}
}

func TestCaptureProjectContextNoSources(t *testing.T) {
	engine := newProjectContextEngine(t)
	engine.SetProjectContextLoader(func(context.Context, model.ReviewSource, model.ReviewRequest) (*model.ProjectContext, []string) {
		return nil, nil
	})
	reviewCtx := sampleReviewCtx()
	engine.captureProjectContext(context.Background(), reviewCtx, model.ReviewRequest{})
	if reviewCtx.ProjectContext != nil {
		t.Fatalf("ProjectContext = %+v, want nil", reviewCtx.ProjectContext)
	}
	if len(reviewCtx.OmittedSections) != 0 {
		t.Fatalf("OmittedSections = %v, want none", reviewCtx.OmittedSections)
	}
}

// The trimmer buys back tokens by dropping context. Project context must not be
// among what it drops: losing it changes how findings are judged rather than
// merely shortening the prompt.
func TestTrimmerKeepsProjectContext(t *testing.T) {
	reviewCtx := sampleReviewCtx()
	reviewCtx.ProjectContext = fullProjectContext()
	reviewCtx.Comments = []model.Comment{{Author: "a", Body: strings.Repeat("x", 4000)}}
	reviewCtx.SupplementalContext = []model.SupplementalFile{{Path: "big.go", Content: strings.Repeat("y", 4000)}}

	trimmed, err := NewTrimmer(64, tokenestimate.SimpleEstimator{}).Trim(reviewCtx)
	if err != nil {
		t.Fatalf("Trim returned err: %v", err)
	}
	if trimmed.ProjectContext == nil || trimmed.ProjectContext.Deployment != "internet-facing" {
		t.Fatalf("ProjectContext = %+v, want it preserved through trimming", trimmed.ProjectContext)
	}
}

// A dedupe or merge step can drop the block to buy back tokens.
func TestStepContextIncludeControlsProjectContext(t *testing.T) {
	engine := newProjectContextEngine(t)
	pc := fullProjectContext()
	st := &PipelineState{projectContext: pc, enrichedPrompt: "{}"}

	on, err := engine.resolveStepPromptContext(st, nil)
	if err != nil {
		t.Fatalf("resolveStepPromptContext returned err: %v", err)
	}
	if on.projectContext != pc {
		t.Fatalf("projectContext = %+v, want it included by default", on.projectContext)
	}

	off := false
	override := &workflow.StepOverride{Context: &workflow.ContextInclude{ProjectContext: &off}}
	got, err := engine.resolveStepPromptContext(st, override)
	if err != nil {
		t.Fatalf("resolveStepPromptContext returned err: %v", err)
	}
	if got.projectContext != nil {
		t.Fatalf("projectContext = %+v, want it dropped by project_context: false", got.projectContext)
	}
}

// End-to-end over the real seam: a file on disk, read through the production
// LocalSource and loader, ending up in a reviewer's system prompt.
func TestProjectContextFromLocalRepoReachesReviewPrompt(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".nickpit"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "deployment: internet-facing\ncriticality: high\ntrust_boundaries:\n  - handlers accept untrusted bodies\n"
	if err := os.WriteFile(filepath.Join(dir, ".nickpit", "context.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(git.NewLocalSource(dir), &capturingLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	reviewCtx := sampleReviewCtx()
	engine.captureProjectContext(context.Background(), reviewCtx, model.ReviewRequest{RepoRoot: dir})

	if reviewCtx.ProjectContext == nil {
		t.Fatal("ProjectContext = nil, want the repository's file loaded")
	}
	if reviewCtx.ProjectContext.Sources[0] != ".nickpit/context.yaml" {
		t.Fatalf("sources = %v", reviewCtx.ProjectContext.Sources)
	}

	template, err := prompts.Load("agent_review_general_system_prompt.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	system, err := engine.renderReviewSystemWithFocus(template, "", model.ReviewRequest{}, true, "review", nil, false, reviewCtx.ProjectContext)
	if err != nil {
		t.Fatalf("renderReviewSystemWithFocus returned err: %v", err)
	}
	for _, want := range []string{"## PROJECT CONTEXT", "internet-facing", "handlers accept untrusted bodies", "Declared in: .nickpit/context.yaml"} {
		if !strings.Contains(system, want) {
			t.Fatalf("review system prompt missing %q:\n%s", want, system)
		}
	}
}
