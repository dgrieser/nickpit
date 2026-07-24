package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
	"github.com/dgrieser/nickpit/prompts"
	"github.com/google/uuid"
)

type stubSource struct{}

func (stubSource) ResolveContext(context.Context, model.ReviewRequest) (*model.ReviewContext, error) {
	return &model.ReviewContext{
		Mode: model.ModeLocal,
		Repository: model.RepositoryInfo{
			FullName: "repo",
		},
		Title:       "title",
		Description: "description",
		ChangedFiles: []model.ChangedFile{
			{Path: "main.go", Status: model.FileModified, Additions: 1},
		},
		Diff: "diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n",
		DiffFiles: []model.DiffFile{
			{FilePath: "main.go", Language: "go", Content: "diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n"},
		},
		DiffHunks: []model.DiffHunk{
			{FilePath: "main.go", Language: "go", OldStart: 1, OldLines: 1000, NewStart: 1, NewLines: 1000, Content: "-old\n+new\n"},
		},
	}, nil
}

type scopeTrimSource struct{}

func (scopeTrimSource) ResolveContext(context.Context, model.ReviewRequest) (*model.ReviewContext, error) {
	return &model.ReviewContext{
		Mode: model.ModeLocal,
		ChangedFiles: []model.ChangedFile{
			{Path: "a.go", Status: model.FileModified},
			{Path: "b.go", Status: model.FileModified},
		},
		Diff: "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-old\n+new\n" +
			"diff --git a/b.go b/b.go\n@@ -10 +10 @@\n-old\n+new\n",
		DiffFiles: []model.DiffFile{
			{FilePath: "a.go", Content: strings.Repeat("a", 200)},
			{FilePath: "b.go", Content: strings.Repeat("b", 200)},
		},
		DiffHunks: []model.DiffHunk{
			{FilePath: "a.go", OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1, Content: "-old\n+new\n"},
			{FilePath: "b.go", OldStart: 10, OldLines: 1, NewStart: 10, NewLines: 1, Content: "-old\n+new\n"},
		},
	}, nil
}

func TestResolveAndTrimContextPreservesPreTrimDiffScopeHunks(t *testing.T) {
	engine := NewEngine(scopeTrimSource{}, stubLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetToolchainCapture(nil)
	engine.trimmer = NewTrimmer(1, exactEstimator{})
	ctx, err := engine.resolveAndTrimContext(context.Background(), model.ReviewRequest{Mode: model.ModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.DiffScopeHunks) != 2 {
		t.Fatalf("diff scope hunks = %#v, want both pre-trim hunks", ctx.DiffScopeHunks)
	}
	if len(ctx.DiffHunks) >= len(ctx.DiffScopeHunks) {
		t.Fatalf("prompt hunks = %d, scope hunks = %d; test did not trim prompt diff", len(ctx.DiffHunks), len(ctx.DiffScopeHunks))
	}
}

type filteredOutSource struct{}

func (filteredOutSource) ResolveContext(context.Context, model.ReviewRequest) (*model.ReviewContext, error) {
	return &model.ReviewContext{
		Mode:         model.ModeLocal,
		ChangedFiles: []model.ChangedFile{{Path: "README.md", Status: model.FileModified}},
		Diff:         "diff --git a/README.md b/README.md\n@@ -1 +1 @@\n-old\n+new\n",
		DiffFiles:    []model.DiffFile{{FilePath: "README.md", Language: "markdown", Content: "diff --git a/README.md b/README.md\n@@ -1 +1 @@\n-old\n+new\n"}},
		DiffHunks:    []model.DiffHunk{{FilePath: "README.md", Content: "+new\n"}},
	}, nil
}

type stubLLM struct{}

func (stubLLM) Review(context.Context, *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	return &llm.ReviewResponse{
		Findings: []model.Finding{
			{Title: "info", Body: "low", ConfidenceScore: 0.5, Priority: intPtr(3), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}},
			{Title: "error", Body: "high", ConfidenceScore: 0.9, Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 2, End: 2}}},
		},
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "summary",
		OverallConfidenceScore: 0.9,
	}, nil
}

func TestReviewContextFilterPathAndContent(t *testing.T) {
	repoRoot := t.TempDir()
	for path, content := range map[string]string{
		"app/main.go":      "package main\nfunc main() {}\n",
		"app/generated.go": "package main\n// DO NOT EDIT\n",
		"web/app.ts":       "export const app = true\n",
	} {
		fullPath := filepath.Join(repoRoot, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ctx := &model.ReviewContext{
		ChangedFiles: []model.ChangedFile{
			{Path: "app/main.go", Status: model.FileModified},
			{Path: "app/generated.go", Status: model.FileModified},
			{Path: "web/app.ts", Status: model.FileModified},
			{Path: "old.go", Status: model.FileDeleted},
		},
		Diff: strings.Join([]string{
			"diff --git a/app/main.go b/app/main.go\n@@ -1 +1 @@\n-old\n+new\n",
			"diff --git a/app/generated.go b/app/generated.go\n@@ -1 +1 @@\n-old\n+new\n",
			"diff --git a/web/app.ts b/web/app.ts\n@@ -1 +1 @@\n-old\n+new\n",
			"diff --git a/old.go b/old.go\n@@ -1 +0,0 @@\n-old\n",
		}, ""),
		DiffHunks: []model.DiffHunk{
			{FilePath: "app/main.go", Content: "+new\n"},
			{FilePath: "app/generated.go", Content: "+new\n"},
			{FilePath: "web/app.ts", Content: "+new\n"},
			{FilePath: "old.go", Content: "-old\n"},
		},
		DiffFiles: []model.DiffFile{
			{FilePath: "app/main.go", Content: "diff --git a/app/main.go b/app/main.go\n@@ -1 +1 @@\n-old\n+new\n"},
			{FilePath: "app/generated.go", Content: "diff --git a/app/generated.go b/app/generated.go\n@@ -1 +1 @@\n-old\n+new\n"},
			{FilePath: "web/app.ts", Content: "diff --git a/web/app.ts b/web/app.ts\n@@ -1 +1 @@\n-old\n+new\n"},
			{FilePath: "old.go", Content: "diff --git a/old.go b/old.go\n@@ -1 +0,0 @@\n-old\n"},
		},
		Comments: []model.Comment{
			{Path: "app/main.go", Body: "keep inline"},
			{Path: "app/generated.go", Body: "drop inline"},
			{Body: "keep general"},
		},
	}
	filter, err := newReviewContextFilter(model.ReviewRequest{
		RepoRoot:       repoRoot,
		IncludePaths:   []string{`\.go$`},
		ExcludePaths:   []string{`generated`},
		IncludeContent: []string{`(?m)^package main`},
		ExcludeContent: []string{`DO NOT EDIT`},
	})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})

	allFiltered, err := engine.applyReviewContextFilter(context.Background(), ctx, model.ReviewRequest{RepoRoot: repoRoot}, filter)
	if err != nil {
		t.Fatal(err)
	}
	if allFiltered {
		t.Fatal("expected one file to remain")
	}
	if len(ctx.ChangedFiles) != 1 || ctx.ChangedFiles[0].Path != "app/main.go" {
		t.Fatalf("changed files = %#v", ctx.ChangedFiles)
	}
	if len(ctx.DiffHunks) != 1 || ctx.DiffHunks[0].FilePath != "app/main.go" {
		t.Fatalf("diff hunks = %#v", ctx.DiffHunks)
	}
	if len(ctx.DiffFiles) != 1 || ctx.DiffFiles[0].FilePath != "app/main.go" {
		t.Fatalf("diff files = %#v", ctx.DiffFiles)
	}
	if !strings.Contains(ctx.Diff, "app/main.go") || strings.Contains(ctx.Diff, "generated.go") || strings.Contains(ctx.Diff, "web/app.ts") || strings.Contains(ctx.Diff, "old.go") {
		t.Fatalf("diff = %q", ctx.Diff)
	}
	if got := commentBodies(ctx.Comments); !reflect.DeepEqual(got, []string{"keep inline", "keep general"}) {
		t.Fatalf("comments = %#v", got)
	}
	if len(ctx.OmittedSections) == 0 || !strings.Contains(ctx.OmittedSections[0], "files omitted by filters") {
		t.Fatalf("omitted sections = %#v", ctx.OmittedSections)
	}
}

func TestReviewContextFilterAllOmitted(t *testing.T) {
	ctx := &model.ReviewContext{
		ChangedFiles: []model.ChangedFile{{Path: "README.md", Status: model.FileModified}},
		Diff:         "diff --git a/README.md b/README.md\n@@ -1 +1 @@\n-old\n+new\n",
		DiffFiles:    []model.DiffFile{{FilePath: "README.md", Content: "diff --git a/README.md b/README.md\n@@ -1 +1 @@\n-old\n+new\n"}},
		DiffHunks:    []model.DiffHunk{{FilePath: "README.md", Content: "+new\n"}},
	}
	filter, err := newReviewContextFilter(model.ReviewRequest{IncludePaths: []string{`\.go$`}})
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(nil, nil, retrieval.NewLocalEngine(), config.Profile{})

	allFiltered, err := engine.applyReviewContextFilter(context.Background(), ctx, model.ReviewRequest{}, filter)
	if err != nil {
		t.Fatal(err)
	}
	if !allFiltered {
		t.Fatal("expected all files filtered")
	}
	if len(ctx.ChangedFiles) != 0 || strings.TrimSpace(ctx.Diff) != "" || len(ctx.DiffFiles) != 0 || len(ctx.DiffHunks) != 0 {
		t.Fatalf("context not empty: files=%#v diff=%q diffFiles=%#v hunks=%#v", ctx.ChangedFiles, ctx.Diff, ctx.DiffFiles, ctx.DiffHunks)
	}
	if !reviewContextAllFiltered(ctx) {
		t.Fatalf("omitted sections = %#v", ctx.OmittedSections)
	}
}

func TestRunSpecPipelineReturnsCleanResultWhenFiltersOmitAll(t *testing.T) {
	engine := NewEngine(filteredOutSource{}, stubLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test-model"})
	result, _, err := engine.RunSpecPipeline(context.Background(), &Pipeline{needsSource: true}, model.ReviewRequest{
		Mode:         model.ModeLocal,
		IncludePaths: []string{`\.go$`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected result")
		return
	}
	if len(result.Findings) != 0 {
		t.Fatalf("findings = %#v", result.Findings)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != allChangedFilesFilteredWarning {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
	if result.OverallExplanation != "All changed files were omitted by filters." {
		t.Fatalf("overall explanation = %q", result.OverallExplanation)
	}
}

func TestPromptPayloadDefaultsToDiffFiles(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload := reviewPromptPayload(t, llmClient.reqs[0])
	if _, ok := payload["diff_files"]; !ok {
		t.Fatalf("payload missing diff_files: %#v", payload)
	}
	if _, ok := payload["diff_hunks"]; ok {
		t.Fatalf("payload should not include diff_hunks in default mode: %#v", payload["diff_hunks"])
	}
	diffFiles := payload["diff_files"].([]any)
	first := diffFiles[0].(map[string]any)
	if first["file_path"] != "main.go" || first["language"] != "go" {
		t.Fatalf("diff file = %#v", first)
	}
	if content, _ := first["content"].(string); !strings.HasPrefix(content, "diff --git a/main.go b/main.go\n") {
		t.Fatalf("diff file content = %.80q", content)
	}
}

func TestPromptPayloadCanUseLegacyDiffHunks(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		DiffFormat:       model.DiffFormatGitJson,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload := reviewPromptPayload(t, llmClient.reqs[0])
	if _, ok := payload["diff_hunks"]; !ok {
		t.Fatalf("payload missing diff_hunks: %#v", payload)
	}
	if _, ok := payload["diff_files"]; ok {
		t.Fatalf("payload should not include diff_files in hunk mode: %#v", payload["diff_files"])
	}
}

func reviewPromptPayload(t *testing.T, req *llm.ReviewRequest) map[string]any {
	t.Helper()
	content := taskMessageContent(req)
	if content == "" {
		t.Fatalf("review request messages = %#v", req)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("unmarshal review payload: %v\n%s", err, content)
	}
	return payload
}

func TestOutputFormatSnippetIncludesExampleWhenProvided(t *testing.T) {
	snippets, err := agentCommonSystemPromptSnippets("merge", exampleSnippetFor(llm.SchemaKindMerge, false), false)
	if err != nil {
		t.Fatalf("agentCommonSystemPromptSnippets returned err: %v", err)
	}
	if !strings.Contains(snippets.outputFormat, strings.TrimSpace(llm.MergeExamplePromptSnippet())) {
		t.Fatalf("output format missing merge example:\n%s", snippets.outputFormat)
	}
	if strings.Contains(snippets.outputFormat, "response_format.json_schema") {
		t.Fatalf("output format should not branch on response_format mode:\n%s", snippets.outputFormat)
	}
}

func TestOutputFormatSnippetSkippedWithoutExample(t *testing.T) {
	snippets, err := agentCommonSystemPromptSnippets("context", "", false)
	if err != nil {
		t.Fatalf("agentCommonSystemPromptSnippets returned err: %v", err)
	}
	if snippets.outputFormat != "" {
		t.Fatalf("output format = %q, want empty", snippets.outputFormat)
	}
}

func TestReviewSystemPromptGatesSearchLocationGuidanceOnTools(t *testing.T) {
	engine := NewEngine(stubSource{}, &capturingLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	template := "{{.FindingInstructionsSnippet}}"

	withTools, err := engine.renderReviewSystemWithFocus(template, "", model.ReviewRequest{}, true, "review", nil, false)
	if err != nil {
		t.Fatalf("renderReviewSystemWithFocus with tools returned err: %v", err)
	}
	if !strings.Contains(withTools, "ALWAYS use `search` to fill every `code_location`") {
		t.Fatalf("with-tools prompt missing search code_location guidance:\n%s", withTools)
	}
	for _, want := range []string{
		"DO NOT include the following in `body`",
		"full replacement code",
		"full patches",
		"or before/after patches",
		"do not output duplicate suggestions for the same fix",
		"put the following in `suggestions[].body`",
	} {
		if !strings.Contains(withTools, want) {
			t.Fatalf("review prompt missing body/suggestion split instruction %q:\n%s", want, withTools)
		}
	}

	withoutTools, err := engine.renderReviewSystemWithFocus(template, "", model.ReviewRequest{}, false, "review", nil, false)
	if err != nil {
		t.Fatalf("renderReviewSystemWithFocus without tools returned err: %v", err)
	}
	if strings.Contains(withoutTools, "ALWAYS use `search`") {
		t.Fatalf("no-tools prompt mentions the search tool guidance:\n%s", withoutTools)
	}
}

func TestMergeAndDedupePromptsAvoidDuplicateSuggestions(t *testing.T) {
	commonSnippets, err := agentCommonSystemPromptSnippets("merge", exampleSnippetFor(llm.SchemaKindMerge, false), false)
	if err != nil {
		t.Fatalf("agentCommonSystemPromptSnippets returned err: %v", err)
	}
	for _, file := range []string{"agent_dedupe_system_prompt.tmpl", "agent_cluster_merge_system_prompt.tmpl"} {
		template, err := prompts.Load(file)
		if err != nil {
			t.Fatalf("load %s: %v", file, err)
		}
		system, err := llm.RenderPrompt(template, struct {
			FindingInstructionsSnippet string
			PrioritySnippet            string
			OutputFormatSnippet        string
			DisableSuggestions         bool
			DisableDiffScope           bool
			StyleGuideToolchainSnippet string
		}{
			FindingInstructionsSnippet: commonSnippets.findingInstructions,
			PrioritySnippet:            commonSnippets.priority,
			OutputFormatSnippet:        commonSnippets.outputFormat,
		})
		if err != nil {
			t.Fatalf("render %s: %v", file, err)
		}
		for _, want := range []string{
			"output at most one suggestion for each surviving finding",
			"select the clearest, most actionable fix",
			"output at most one `suggestions` entry per finding",
			"triggering condition",
			"root cause",
			"same code condition and same edit",
			"example or subset",
			"regression test",
			"bundled finding",
		} {
			if !strings.Contains(system, want) {
				t.Fatalf("%s missing duplicate suggestion guidance %q:\n%s", file, want, system)
			}
		}
	}
}

func TestRunAgentDoesNotInsertSeparateExampleMessage(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	_, err := engine.runAgent(context.Background(), agentSpec{
		name:       "Merge Findings",
		role:       "merge",
		system:     "system prompt",
		user:       `{"task":true}`,
		schemaKind: llm.SchemaKindMerge,
		hasTools:   false,
	}, model.ReviewRequest{})
	if err != nil {
		t.Fatalf("runAgent returned err: %v", err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(llmClient.reqs))
	}
	messages := llmClient.reqs[0].Messages
	if len(messages) != 2 {
		t.Fatalf("messages = %#v, want only system/user", messages)
	}
	if messages[0].Role != "system" || messages[0].Content != "system prompt" {
		t.Fatalf("system message = %#v", messages[0])
	}
	if messages[1].Role != "user" || messages[1].Content != `{"task":true}` {
		t.Fatalf("task message = %#v", messages[1])
	}
}

func TestRunAgentSkipsExampleMessageForTextAgents(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	_, err := engine.runAgent(context.Background(), agentSpec{
		name:       "Collect Context",
		role:       "context",
		system:     "system prompt",
		user:       "plain text task",
		schemaKind: llm.SchemaKindText,
		hasTools:   false,
	}, model.ReviewRequest{})
	if err != nil {
		t.Fatalf("runAgent returned err: %v", err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(llmClient.reqs))
	}
	messages := llmClient.reqs[0].Messages
	if len(messages) != 2 {
		t.Fatalf("messages = %#v, want only system/user", messages)
	}
	if messages[1].Role != "user" || messages[1].Content != "plain text task" {
		t.Fatalf("task message = %#v", messages[1])
	}
}

func taskMessageContent(req *llm.ReviewRequest) string {
	if req == nil || len(req.Messages) < 2 {
		return ""
	}
	return req.Messages[1].Content
}

func commentBodies(comments []model.Comment) []string {
	out := make([]string, 0, len(comments))
	for _, comment := range comments {
		out = append(out, comment.Body)
	}
	return out
}

func TestParseDiffGitPath(t *testing.T) {
	cases := map[string]string{
		"diff --git a/app/main.go b/app/main.go":         "app/main.go",
		"diff --git a/my file.go b/my file.go":           "my file.go",
		"diff --git a/dir/old name.go b/dir/new name.go": "dir/new name.go",
		"diff --git a/old.go b/old.go":                   "old.go",
		"not a diff header":                              "",
	}
	for line, want := range cases {
		if got := parseDiffGitPath(line); got != want {
			t.Errorf("parseDiffGitPath(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestFilterUnifiedDiffSpacePath(t *testing.T) {
	diff := "diff --git a/my file.go b/my file.go\n@@ -1 +1 @@\n-old\n+new\n" +
		"diff --git a/drop.go b/drop.go\n@@ -1 +1 @@\n-old\n+new\n"
	out := filterUnifiedDiff(diff, map[string]bool{"my file.go": true})
	if !strings.Contains(out, "my file.go") || strings.Contains(out, "drop.go") {
		t.Fatalf("filtered diff = %q", out)
	}
}

type capturingLLM struct {
	mu    sync.Mutex
	reqs  []*llm.ReviewRequest
	resps []*llm.ReviewResponse
}

var uuidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

type multiAgentLLM struct {
	mu                sync.Mutex
	context           int
	vectorCalls       map[string]int
	verifyCalls       int
	mergeTools        int
	mergePayload      map[string]any
	mergeSchema       []byte
	mergeRequests     []*llm.ReviewRequest
	contextSystem     string
	vectorContext     map[string]string
	vectorSystem      map[string]string
	vectorNudge       map[string]string
	events            []string
	contextFailErr    error
	vectorFailErr     map[string]error
	verifyInvalid     map[string]bool
	vectorFindings    map[string]int
	dedupeResponses   []*llm.ReviewResponse
	dedupeFailErr     error
	mergeResponses    []*llm.ReviewResponse
	mergeFailErr      error
	finalizeFailErr   error
	finalizeRequests  []*llm.ReviewRequest
	verdictRequests   []*llm.ReviewRequest
	summarizeRequests []*llm.ReviewRequest
	verdictFailErr    error
}

func (s *multiAgentLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vectorCalls == nil {
		s.vectorCalls = make(map[string]int)
	}
	if s.vectorSystem == nil {
		s.vectorSystem = make(map[string]string)
	}
	if s.vectorContext == nil {
		s.vectorContext = make(map[string]string)
	}
	if s.vectorNudge == nil {
		s.vectorNudge = make(map[string]string)
	}
	if req.SchemaKind == llm.SchemaKindVerify {
		s.events = append(s.events, "verify")
		s.verifyCalls++
		valid := true
		for title := range s.verifyInvalid {
			for _, msg := range req.Messages {
				if strings.Contains(msg.Content, title) {
					valid = false
					break
				}
			}
			if !valid {
				break
			}
		}
		verdict := model.VerdictConfirmed
		if !valid {
			verdict = model.VerdictRefuted
		}
		return &llm.ReviewResponse{
			Verification: &model.FindingVerification{Verdict: verdict, Priority: 2, ConfidenceScore: 0.9, Remarks: "verified"},
			TokensUsed:   model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}, nil
	}
	system := ""
	if len(req.Messages) > 0 {
		system = req.Messages[0].Content
	}
	if strings.Contains(system, "DO NOT produce review findings yourself") {
		s.contextSystem = system
		s.context++
		if s.contextFailErr != nil {
			return nil, s.contextFailErr
		}
		if s.context == 1 {
			return &llm.ReviewResponse{
				ToolCalls: []llm.ToolCall{{ID: "context_call", Name: "list_files", Arguments: `{"path":"internal","depth":1}`}},
			}, nil
		}
		return &llm.ReviewResponse{
			RawResponse: "context inspected internal listing\n\n## Assumed Patch Purpose\nThis is an assumption: the patch appears intended to update review context collection.",
			TokensUsed:  model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}, nil
	}
	if strings.Contains(system, "## FOCUS ON ") {
		name := vectorNameFromSystem(system)
		s.vectorCalls[name]++
		s.vectorSystem[name] = system
		for _, msg := range req.Messages {
			if msg.Role == "user" && strings.Contains(msg.Content, "## Notes") {
				s.vectorContext[name] = msg.Content
			}
			if msg.Role == "user" && strings.Contains(msg.Content, "You may have missed issues") {
				s.vectorNudge[name] = msg.Content
			}
		}
		if err := s.vectorFailErr[name]; err != nil {
			return nil, err
		}
		if s.vectorCalls[name] == 1 {
			return &llm.ReviewResponse{
				ToolCalls: []llm.ToolCall{{ID: "tool_" + name, Name: "inspect_file", Arguments: `{"path":"main.go"}`}},
			}, nil
		}
		count := 1
		if s.vectorFindings != nil && s.vectorFindings[name] > 0 {
			count = s.vectorFindings[name]
		}
		findings := make([]model.Finding, 0, count)
		for i := 0; i < count; i++ {
			title := "Fix " + name
			if i > 0 {
				title = fmt.Sprintf("Fix %s %s", name, testFindingIndexWords[i%len(testFindingIndexWords)])
			}
			// Line spacing keeps the synthetic findings in dedupe.Possible
			// territory across vectors (gap 12–24 lines, shared body) so the
			// cluster merge agent is exercised, while same-vector extras land
			// far apart (gap ≥50) so they survive the mechanical dedupe
			// pre-pass and still reach the LLM dedupe agent.
			line := 1 + 12*testVectorIndex(name) + 50*i
			findings = append(findings, model.Finding{
				Title:           title,
				Body:            "body",
				ConfidenceScore: 0.9,
				Priority:        intPtr(2),
				CodeLocation:    testLineCodeLocation("main.go", line),
			})
		}
		return &llm.ReviewResponse{
			Findings:               findings,
			OverallCorrectness:     "patch is incorrect",
			OverallExplanation:     name,
			OverallConfidenceScore: 0.9,
			TokensUsed:             model.TokenUsage{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3},
		}, nil
	}
	if req.SchemaKind == llm.SchemaKindFinalize {
		cloned := *req
		cloned.Messages = cloneTestMessages(req.Messages)
		s.finalizeRequests = append(s.finalizeRequests, &cloned)
		if s.finalizeFailErr != nil {
			return nil, s.finalizeFailErr
		}
		findings := testPayloadFindingsFromJSON(taskMessageContent(req))
		for i := range findings {
			if findings[i].Verification == nil {
				findings[i].Verification = &model.FindingVerification{ID: findings[i].ID, Verdict: model.VerdictConfirmed, Priority: model.PriorityRank(findings[i].Priority), ConfidenceScore: 0.9, Remarks: "verified"}
			}
			findings[i].Finalization = &model.FindingFinalization{
				Title:           "Final " + findings[i].Title,
				Body:            "FINALIZED_MARKER " + findings[i].Body,
				Priority:        model.PriorityRank(findings[i].Priority),
				ConfidenceScore: 0.8,
				Remarks:         "finalized",
			}
		}
		return &llm.ReviewResponse{
			Findings:   findings,
			TokensUsed: model.TokenUsage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
		}, nil
	}
	if req.SchemaKind == llm.SchemaKindVerdict {
		cloned := *req
		cloned.Messages = cloneTestMessages(req.Messages)
		s.verdictRequests = append(s.verdictRequests, &cloned)
		if s.verdictFailErr != nil {
			return nil, s.verdictFailErr
		}
		findings := testPayloadFindingsFromJSON(taskMessageContent(req))
		return &llm.ReviewResponse{
			OverallCorrectness:     "patch is incorrect",
			OverallExplanation:     fmt.Sprintf("VERDICT_MARKER findings=%d", len(findings)),
			OverallConfidenceScore: 0.88,
			TokensUsed:             model.TokenUsage{PromptTokens: 6, CompletionTokens: 2, TotalTokens: 8},
		}, nil
	}
	if req.SchemaKind == llm.SchemaKindSummarize {
		cloned := *req
		cloned.Messages = cloneTestMessages(req.Messages)
		s.summarizeRequests = append(s.summarizeRequests, &cloned)
		items := testPayloadFindingsFromJSON(taskMessageContent(req))
		out := make([]model.Finding, 0, len(items))
		for _, item := range items {
			body := item.Body
			if body == "" {
				body = item.Title
			}
			out = append(out, model.Finding{ID: item.ID, Summarization: &model.FindingSummarization{Body: "SUMMARY_MARKER " + body}})
		}
		return &llm.ReviewResponse{
			Findings:   out,
			TokensUsed: model.TokenUsage{PromptTokens: 4, CompletionTokens: 1, TotalTokens: 5},
		}, nil
	}
	s.mergeTools = len(req.Tools)
	s.events = append(s.events, "merge")
	s.mergeSchema = append([]byte(nil), req.Schema...)
	cloned := *req
	cloned.Messages = cloneTestMessages(req.Messages)
	s.mergeRequests = append(s.mergeRequests, &cloned)
	taskContent := taskMessageContent(req)
	if taskContent != "" {
		if strings.Contains(taskContent, `"review_findings"`) {
			if s.dedupeFailErr != nil {
				return nil, s.dedupeFailErr
			}
			if len(s.dedupeResponses) > 0 {
				resp := s.dedupeResponses[0]
				s.dedupeResponses = s.dedupeResponses[1:]
				return resp, nil
			}
			findings := testPayloadFindingIDsFromJSON(taskContent)
			return &llm.ReviewResponse{
				Findings:               findings,
				OverallCorrectness:     "patch is incorrect",
				OverallExplanation:     "deduped",
				OverallConfidenceScore: 0.95,
				TokensUsed:             model.TokenUsage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4},
			}, nil
		}
		_ = json.Unmarshal([]byte(taskContent), &s.mergePayload)
	}
	if s.mergeFailErr != nil {
		return nil, s.mergeFailErr
	}
	if len(s.mergeResponses) > 0 {
		resp := s.mergeResponses[0]
		s.mergeResponses = s.mergeResponses[1:]
		return resp, nil
	}
	// Default cluster-merge behavior: echo the cluster findings unchanged,
	// which is a valid response (count within bounds, every finding matched).
	if findings := testClusterFindings(s.mergePayload); len(findings) > 0 {
		return &llm.ReviewResponse{
			Findings:               findings,
			OverallCorrectness:     "patch is incorrect",
			OverallExplanation:     "merged",
			OverallConfidenceScore: 0.95,
			TokensUsed:             model.TokenUsage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4},
		}, nil
	}
	return &llm.ReviewResponse{
		Findings: []model.Finding{{
			Title:           "Fix merged issue",
			Body:            "body",
			ConfidenceScore: 0.95,
			Priority:        intPtr(1),
			CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
			Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.9, Remarks: "confirmed"},
		}},
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "merged",
		OverallConfidenceScore: 0.95,
		TokensUsed:             model.TokenUsage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4},
	}, nil
}

// testFindingIndexWords keeps same-vector extra findings titled with distinct
// word tokens (digits would be dropped by the dedupe tokenizer, making the
// titles identical and the findings mechanical duplicates).
var testFindingIndexWords = []string{"alpha", "beta", "gamma", "delta", "epsilon"}

func testVectorIndex(name string) int {
	for i, vector := range reviewVectors {
		if vector.name == name {
			return i
		}
	}
	return 0
}

func testClusterFindings(payload map[string]any) []model.Finding {
	raw, ok := payload["cluster_findings"].([]any)
	if !ok {
		return nil
	}
	findings := make([]model.Finding, 0, len(raw))
	for _, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		data, err := json.Marshal(m["finding"])
		if err != nil {
			continue
		}
		var f model.Finding
		if json.Unmarshal(data, &f) == nil {
			findings = append(findings, f)
		}
	}
	return findings
}

// testPayloadFindingIDsFromJSON echoes the dedupe payload's findings back.
// Full findings (not just IDs) are returned so the cluster merge downstream
// still sees titles and code locations; stripped findings would all compare
// as dedupe.Distinct and the merge agent would never run.
func testPayloadFindingIDsFromJSON(data string) []model.Finding {
	var payload struct {
		ReviewFindings struct {
			Findings []model.Finding `json:"findings"`
		} `json:"review_findings"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err == nil && len(payload.ReviewFindings.Findings) > 0 {
		return payload.ReviewFindings.Findings
	}
	matches := uuidRe.FindAllString(data, -1)
	seen := make(map[string]struct{}, len(matches))
	findings := make([]model.Finding, 0, len(matches))
	for _, id := range matches {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		findings = append(findings, model.Finding{
			ID:           id,
			Verification: &model.FindingVerification{ID: id, Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.9, Remarks: "verified"},
		})
	}
	return findings
}

func testPayloadFindingsFromJSON(data string) []model.Finding {
	var payload struct {
		Findings []model.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil
	}
	return payload.Findings
}

func vectorNameFromSystem(system string) string {
	marker := "## FOCUS ON "
	_, rest, found := strings.Cut(system, marker)
	if !found {
		return ""
	}
	rest = strings.TrimSpace(rest)
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[:nl]
	}
	switch strings.TrimSpace(rest) {
	case "CODE QUALITY AND CORRECTNESS":
		return "Code Quality"
	case "BEST PRACTICES":
		return "Best Practices"
	default:
		return titleCaseASCII(strings.ToLower(strings.TrimSpace(rest)))
	}
}

// titleCaseASCII upper-cases the first letter of each whitespace-separated word.
// It replaces the deprecated strings.Title for the ASCII focus names this test
// helper handles, avoiding a golang.org/x/text dependency.
func titleCaseASCII(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func (s *capturingLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := *req
	if len(req.Messages) > 0 {
		cloned.Messages = cloneTestMessages(req.Messages)
	}
	if len(req.NoToolsMessages) > 0 {
		cloned.NoToolsMessages = cloneTestMessages(req.NoToolsMessages)
	}
	if len(req.Tools) > 0 {
		cloned.Tools = append([]llm.ToolDefinition(nil), req.Tools...)
	}
	s.reqs = append(s.reqs, &cloned)
	if len(s.resps) > 0 {
		resp := s.resps[0]
		s.resps = s.resps[1:]
		return resp, nil
	}
	return &llm.ReviewResponse{
		OverallCorrectness:     "patch is correct",
		OverallExplanation:     "summary",
		OverallConfidenceScore: 0.5,
	}, nil
}

func cloneTestMessages(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return nil
	}
	cloned := append([]llm.Message(nil), messages...)
	for i := range cloned {
		if len(messages[i].ToolCalls) > 0 {
			cloned[i].ToolCalls = append([]llm.ToolCall(nil), messages[i].ToolCalls...)
		}
	}
	return cloned
}

func TestRunAgent_NudgeDuplicate(t *testing.T) {
	first := nudgeFinding("A", 1)
	second := nudgeFinding("B", 2)
	duplicate := nudgeFinding("A", 1)
	third := nudgeFinding("C", 3)
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, first)},
			{resp: func() *llm.ReviewResponse {
				resp := nudgeReviewResponse("second", 2, second)
				resp.ReasoningEffort = "low"
				return resp
			}()},
			{resp: nudgeReviewResponse("duplicate", 3, duplicate)},
			{resp: nudgeReviewResponse("third", 4, third)},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{NudgeCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "B", "C"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if result.run.Findings != 3 {
		t.Fatalf("agent run findings = %d", result.run.Findings)
	}
	if result.run.TokensUsed.TotalTokens != 10 {
		t.Fatalf("tokens = %d", result.run.TokensUsed.TotalTokens)
	}
	if len(llmClient.reqs) != 4 {
		t.Fatalf("llm calls = %d, want 4", len(llmClient.reqs))
	}
	wantEfforts := []string{"high", "high", "low", "low"}
	for i, req := range llmClient.reqs {
		if req.ReasoningEffort != wantEfforts[i] {
			t.Fatalf("call %d reasoning effort = %q, want %q", i+1, req.ReasoningEffort, wantEfforts[i])
		}
	}
	for i, req := range llmClient.reqs[1:] {
		got := req.Messages
		if len(got) != 3 || !strings.Contains(got[len(got)-1].Content, "missed issues") {
			t.Fatalf("nudge %d messages = %#v", i+1, got)
		}
		for _, msg := range got {
			if msg.Role == "assistant" {
				t.Fatalf("nudge %d retained assistant answer: %#v", i+1, got)
			}
			if strings.Contains(msg.Content, "second") || strings.Contains(msg.Content, "duplicate") {
				t.Fatalf("nudge %d retained previous nudge content: %#v", i+1, got)
			}
		}
	}
}

func TestReviewerNudgeBaseMessagesKeepsToolPairsAndDropsAssistantAnswers(t *testing.T) {
	messages := []llm.Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "review"},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "read_file", Arguments: `{}`}}},
		{Role: "tool", ToolCallID: "call-1", Content: "file contents"},
		{Role: "assistant", Content: "final answer"},
	}

	got := reviewerNudgeBaseMessages(messages)
	if len(got) != 4 {
		t.Fatalf("messages = %#v", got)
	}
	if got[2].Role != "assistant" || len(got[2].ToolCalls) != 1 || got[3].Role != "tool" {
		t.Fatalf("tool pair not preserved: %#v", got)
	}
	for _, msg := range got {
		if msg.Content == "final answer" {
			t.Fatalf("assistant answer retained: %#v", got)
		}
	}
}

func TestRunAgent_NudgeKeepDuplicate(t *testing.T) {
	first := nudgeFinding("A", 1)
	changedLocation := nudgeFinding("A", 2)
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, first)},
			{resp: nudgeReviewResponse("changed", 1, changedLocation)},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{NudgeCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
}

func TestTestingDuplicateFileValidatorRejectsSameFile(t *testing.T) {
	first := nudgeFindingInFile("A", "main.go", 1)
	second := nudgeFindingInFile("B", "./main.go", 2)
	resp := nudgeReviewResponse("duplicate", 1, first, second)

	invalid := validateTestingDuplicateFileResponse(nil, resp)
	if invalid == nil {
		t.Fatal("validator accepted duplicate Testing findings for same file")
	}
	if got := invalid.Error(); strings.Contains(got, "invalid JSON") || strings.Contains(got, "missing or invalid fields") {
		t.Fatalf("validator error = %q, want semantic validation wording", got)
	}
	if !strings.Contains(invalid.Reason, "main.go") {
		t.Fatalf("reason = %q, want file path", invalid.Reason)
	}
	rendered, err := renderPromptFile(invalid.RetryGuidanceTemplate, invalid.RetryGuidanceData)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"at most one finding per file", "`A`", "`B`"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("retry guidance missing %q:\n%s", want, rendered)
		}
	}

	otherFile := nudgeFindingInFile("C", "other.go", 3)
	if invalid := validateTestingDuplicateFileResponse(nil, nudgeReviewResponse("ok", 1, first, otherFile)); invalid != nil {
		t.Fatalf("validator rejected different files: %v", invalid)
	}

	exactDuplicate := first
	exactResp := nudgeReviewResponse("exact duplicate", 1, first, exactDuplicate)
	if msg := enforceTestingDuplicateFileResponse("Testing", nil, exactResp); msg != "" {
		t.Fatalf("exact duplicate enforcement message = %q, want none", msg)
	}
	if got := len(exactResp.Findings); got != 1 {
		t.Fatalf("exact duplicate findings = %d, want collapsed to 1", got)
	}
}

func TestTestingDuplicateFileValidatorRejectsExistingSessionFile(t *testing.T) {
	existing := []model.Finding{nudgeFindingInFile("Existing", "main.go", 1)}
	resp := nudgeReviewResponse("duplicate", 1, nudgeFindingInFile("Nudge", "main.go", 2))

	invalid := validateTestingDuplicateFileResponse(existing, resp)
	if invalid == nil {
		t.Fatal("validator accepted nudge finding for existing file")
	}
	rendered, err := renderPromptFile(invalid.RetryGuidanceTemplate, invalid.RetryGuidanceData)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"already reported", "earlier in this session", "`Existing`", "`Nudge`"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("retry guidance missing %q:\n%s", want, rendered)
		}
	}

	exactReplay := nudgeReviewResponse("exact replay", 1, existing[0])
	if invalid := validateTestingDuplicateFileResponse(existing, exactReplay); invalid != nil {
		t.Fatalf("validator rejected exact replay that appendNewFindings would ignore: %v", invalid)
	}
}

func TestRunAgent_TestingDuplicateFileInitialRetry(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("duplicate", 1,
				nudgeFindingInFile("A", "main.go", 1),
				nudgeFindingInFile("B", "main.go", 2),
			)},
			{resp: nudgeReviewResponse("retry", 1, nudgeFindingInFile("Combined", "main.go", 1))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), testingNudgeTestAgent(), model.ReviewRequest{MaxOutputRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"Combined"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("llm calls = %d, want initial plus retry", len(llmClient.reqs))
	}
	retryMessage := llmClient.reqs[1].Messages[len(llmClient.reqs[1].Messages)-1].Content
	if !strings.Contains(retryMessage, "valid JSON but failed response validation") {
		t.Fatalf("retry message missing semantic validation framing:\n%s", retryMessage)
	}
	for _, notWant := range []string{"could not be parsed", "Missing or invalid fields: findings"} {
		if strings.Contains(retryMessage, notWant) {
			t.Fatalf("retry message contains misleading %q:\n%s", notWant, retryMessage)
		}
	}
	if !strings.Contains(retryMessage, "maximum of one finding per file") {
		t.Fatalf("retry message missing Testing duplicate-file guidance:\n%s", retryMessage)
	}
}

func TestRunAgent_TestingDuplicateFileNudgeRetryAgainstExisting(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("initial", 1, nudgeFindingInFile("A", "main.go", 1))},
			{resp: nudgeReviewResponse("nudge duplicate", 1,
				nudgeFindingInFile("B", "main.go", 2),
				nudgeFindingInFile("C", "other.go", 3),
			)},
			{resp: nudgeReviewResponse("nudge retry", 1, nudgeFindingInFile("C", "other.go", 3))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), testingNudgeTestAgent(), model.ReviewRequest{NudgeCount: 1, MaxOutputRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "C"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 3 {
		t.Fatalf("llm calls = %d, want initial, nudge, retry", len(llmClient.reqs))
	}
	retryMessage := llmClient.reqs[2].Messages[len(llmClient.reqs[2].Messages)-1].Content
	if !strings.Contains(retryMessage, "omit same-file findings") {
		t.Fatalf("retry message missing nudge guidance:\n%s", retryMessage)
	}
}

func TestRunAgent_TestingDuplicateFileRetryExhaustionDropsExtras(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("duplicate", 1,
				nudgeFindingInFile("A", "main.go", 1),
				nudgeFindingInFile("B", "main.go", 2),
			)},
			{resp: nudgeReviewResponse("still duplicate", 1,
				nudgeFindingInFile("Retry A", "main.go", 1),
				nudgeFindingInFile("Retry B", "main.go", 2),
			)},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), testingNudgeTestAgent(), model.ReviewRequest{MaxOutputRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"Retry A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if got, want := result.run.Status, model.AgentRunStatusPartial; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if !strings.Contains(result.run.Error, "duplicate-file Testing findings dropped") || !strings.Contains(result.run.Error, "Retry B") {
		t.Fatalf("run error = %q, want dropped duplicate details", result.run.Error)
	}
}

func TestRunAgent_InvalidJSONRetryExhaustionUsesPartialResponse(t *testing.T) {
	first := nudgeReviewResponse("first invalid", 1, nudgeFinding("First", 1))
	second := nudgeReviewResponse("second invalid", 2, nudgeFinding("Second", 2))
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{err: &llm.InvalidResponseError{
				RawContent:      first.RawResponse,
				Reason:          "response is missing required fields",
				MissingFields:   []string{"findings[0].suggestions[0].code_location"},
				PartialResponse: first,
			}},
			{err: &llm.InvalidResponseError{
				RawContent:      second.RawResponse,
				Reason:          "response is missing required fields",
				MissingFields:   []string{"findings[0].suggestions[0].code_location"},
				PartialResponse: second,
			}},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{MaxOutputRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"Second"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("llm calls = %d, want initial plus retry", len(llmClient.reqs))
	}
	if result.run.TokensUsed.TotalTokens != 3 {
		t.Fatalf("tokens = %d, want initial plus partial retry tokens", result.run.TokensUsed.TotalTokens)
	}
	if result.run.InvalidResponse != nil {
		t.Fatalf("invalid response diagnostic = %#v, want nil when partial response is accepted", result.run.InvalidResponse)
	}
}

func TestRunAgent_InvalidJSONFailurePreservesFinalResponse(t *testing.T) {
	first := &llm.InvalidResponseError{
		RawContent: "å first malformed response",
		Reason:     "first recovery failure",
	}
	final := &llm.InvalidResponseError{
		RawContent: "\x1bå final malformed response api_key=\"sk-supersecretvalue\" SK-ABCDEFGHIJK",
		Reason:     "final candidate repair failure password=hunter123\x1b",
	}
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{{err: first}, {err: final}},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("verdict"), model.ReviewRequest{MaxOutputRetries: 1})
	if err == nil {
		t.Fatal("expected invalid JSON failure")
	}
	if result.run.InvalidResponse == nil {
		t.Fatal("invalid response diagnostic is nil")
	}
	if got, want := result.run.InvalidResponse.Reason, `final candidate repair failure password="[redacted]"`; got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
	if got, want := result.run.InvalidResponse.RawContent, `å final malformed response api_key="[redacted]" [redacted]`; got != want {
		t.Fatalf("raw content = %q, want %q", got, want)
	}
}

func TestRunAgent_InvalidJSONFailureOmitsEmptyDiagnostic(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{err: &llm.InvalidResponseError{}},
			{err: &llm.InvalidResponseError{}},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("verdict"), model.ReviewRequest{MaxOutputRetries: 1})
	if err == nil {
		t.Fatal("expected invalid JSON failure")
	}
	if result.run.InvalidResponse != nil {
		t.Fatalf("invalid response diagnostic = %#v, want nil", result.run.InvalidResponse)
	}
}

func TestRunAgent_SuccessOmitsInvalidResponseDiagnostic(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("valid response", 1, nudgeFinding("Finding", 1))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("verdict"), model.ReviewRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if result.run.InvalidResponse != nil {
		t.Fatalf("invalid response diagnostic = %#v, want nil", result.run.InvalidResponse)
	}
}

func TestTestingDuplicateFilePruneRecordsDroppedFindingKeys(t *testing.T) {
	existing := []model.Finding{nudgeFindingInFile("Existing", "main.go", 1)}
	dropped := nudgeFindingInFile("Dropped", "main.go", 2)

	kept, drops := pruneTestingDuplicateFileFindings(existing, []model.Finding{dropped, dropped})
	if len(kept) != 0 {
		t.Fatalf("kept findings = %#v, want none", kept)
	}
	if len(drops) != 1 {
		t.Fatalf("drops = %#v, want one entry for exact duplicate dropped finding", drops)
	}
	if drops[0].Title != "Dropped" || drops[0].File != "main.go" {
		t.Fatalf("drop = %#v, want Dropped/main.go", drops[0])
	}
}

func TestRunAgent_NudgeZeroDisables(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, nudgeFinding("A", 1))},
			{resp: nudgeReviewResponse("second", 1, nudgeFinding("B", 2))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{NudgeCount: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("llm calls = %d, want 1", len(llmClient.reqs))
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
}

func TestRunAgent_NudgeBudgetsResetOnceBeforeNudges(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, nudgeFinding("A", 1))},
			{resp: &llm.ReviewResponse{
				ToolCalls:   []llm.ToolCall{{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`}},
				RawResponse: "inspect first nudge",
			}},
			{resp: nudgeReviewResponse("second", 1, nudgeFinding("B", 2))},
			{resp: &llm.ReviewResponse{
				ToolCalls:   []llm.ToolCall{{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"main.go"}`}},
				RawResponse: "inspect second nudge",
			}},
			{resp: nudgeReviewResponse("third", 1, nudgeFinding("C", 3))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestToolAgent("review"), model.ReviewRequest{NudgeCount: 2, MaxToolCalls: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.run.ToolCalls != 1 {
		t.Fatalf("tool calls = %d, want one shared nudge-phase budget", result.run.ToolCalls)
	}
	if len(result.toolMessages) != 1 {
		t.Fatalf("tool messages = %d, want only first nudge tool execution", len(result.toolMessages))
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "B", "C"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 5 {
		t.Fatalf("llm calls = %d, want 5", len(llmClient.reqs))
	}
	if len(llmClient.reqs[4].Tools) != 0 {
		t.Fatalf("second nudge final call should run without tools, got %d tools", len(llmClient.reqs[4].Tools))
	}
}

func TestRunAgent_NudgeReviewerOnly(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("context", 1)},
			{resp: nudgeReviewResponse("unused", 1, nudgeFinding("B", 2))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	_, err := engine.runAgent(context.Background(), nudgeTestAgent("context"), model.ReviewRequest{NudgeCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("llm calls = %d, want 1", len(llmClient.reqs))
	}
}

func TestRunAgent_ReasoningExtractorAugmentsNudges(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	llmClient := &reasoningExtractLLM{
		reviewerReasoning: []string{
			"initial reasoning only",
			"nudge one reasoning only",
			"nudge two reasoning only",
		},
		collectOutputs: []string{
			"collect-initial",
		},
		updateOutputs: []string{
			"delta from initial",
			"delta from nudge one",
		},
		firstCollectStarted: started,
		releaseFirstCollect: release,
	}
	engine := nudgeTestEngine(llmClient)

	type runResult struct {
		result agentResult
		err    error
	}
	done := make(chan runResult, 1)
	go func() {
		result, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{
			NudgeCount:          2,
			ModelEmitsReasoning: true,
		})
		done <- runResult{result: result, err: err}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("collect did not start")
	}
	time.Sleep(20 * time.Millisecond)
	llmClient.mu.Lock()
	reviewerCallsBeforeRelease := len(llmClient.reviewerMessages)
	updateCallsBeforeRelease := len(llmClient.updateFullLists)
	llmClient.mu.Unlock()
	if reviewerCallsBeforeRelease != 1 {
		t.Fatalf("reviewer calls before collect release = %d, want 1", reviewerCallsBeforeRelease)
	}
	if updateCallsBeforeRelease != 0 {
		t.Fatalf("update findings calls before collect release = %d, want 0", updateCallsBeforeRelease)
	}
	close(release)

	var got runResult
	select {
	case got = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runAgent did not finish")
	}
	if got.err != nil {
		t.Fatal(got.err)
	}

	llmClient.mu.Lock()
	collectInputs := append([]string(nil), llmClient.collectInputs...)
	updateFullLists := append([]string(nil), llmClient.updateFullLists...)
	updateFindingsJSON := append([]string(nil), llmClient.updateFindingsJSON...)
	reviewerMessages := append([]string(nil), llmClient.reviewerMessages...)
	llmClient.mu.Unlock()

	if want := []string{"initial reasoning only"}; !reflect.DeepEqual(collectInputs, want) {
		t.Fatalf("collect inputs = %#v, want %#v", collectInputs, want)
	}
	if want := []string{"collect-initial", "collect-initial"}; !reflect.DeepEqual(updateFullLists, want) {
		t.Fatalf("update findings lists = %#v, want %#v", updateFullLists, want)
	}
	if len(updateFindingsJSON) != 2 || !strings.Contains(updateFindingsJSON[0], "Initial finding") || !strings.Contains(updateFindingsJSON[1], "Nudge 1 finding") {
		t.Fatalf("update findings JSON = %#v", updateFindingsJSON)
	}
	if len(reviewerMessages) != 3 {
		t.Fatalf("reviewer messages = %d, want 3", len(reviewerMessages))
	}
	if !strings.Contains(reviewerMessages[1], "delta from initial") {
		t.Fatalf("first nudge missing reasoning delta: %q", reviewerMessages[1])
	}
	if !strings.Contains(reviewerMessages[2], "delta from nudge one") {
		t.Fatalf("second nudge missing reasoning delta: %q", reviewerMessages[2])
	}
	if got.result.run.TokensUsed.TotalTokens == 0 {
		t.Fatal("extractor token usage should be folded into reviewer run")
	}
}

func TestRunAgent_ReasoningExtractorVerboseLogsFindingsWithContext(t *testing.T) {
	llmClient := &reasoningExtractLLM{
		reviewerReasoning: []string{"initial reasoning only"},
		collectOutputs:    []string{"collected issue\nsecond collected issue"},
		updateOutputs:     []string{"reduced issue"},
	}
	engine := nudgeTestEngine(llmClient)
	var buf lockedTestBuffer
	engine.SetLogger(logging.New(&buf, true, false))

	_, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{
		NudgeCount:          1,
		ModelEmitsReasoning: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	for _, want := range []string{
		"[extract: Mine Reasoning of review · test-model:high] Extracted reasoning findings:",
		"collected issue",
		"second collected issue",
		"[review: review · Nudge 1/1 · test-model:high] Extracted reasoning findings sent to nudge:",
		"- reduced issue",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("verbose log missing %q:\n%s", want, got)
		}
	}
}

type lockedTestBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedTestBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedTestBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestRunAgent_PerIterCollectInsideInitialReviewer(t *testing.T) {
	llmClient := &multiIterReasoningExtractLLM{
		reviewerReasoning: []string{
			"iter 1 reasoning",
			"iter 2 reasoning",
			"nudge reasoning must not extract",
		},
		collectOutputs: []string{"iter-1-list", "iter-2-list"},
		updateOutputs:  []string{"delta after nudge"},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestToolAgent("review"), model.ReviewRequest{
		NudgeCount:          1,
		ModelEmitsReasoning: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	llmClient.mu.Lock()
	collectInputs := append([]string(nil), llmClient.collectInputs...)
	updateFullLists := append([]string(nil), llmClient.updateFullLists...)
	reviewerCallNum := llmClient.reviewerCallNum
	reviewerMessages := append([]string(nil), llmClient.reviewerMessages...)
	llmClient.mu.Unlock()

	sort.Strings(collectInputs)
	if want := []string{"iter 1 reasoning", "iter 2 reasoning"}; !reflect.DeepEqual(collectInputs, want) {
		t.Fatalf("collect inputs = %#v, want %#v (nudge iter must not fire collect)", collectInputs, want)
	}
	if len(updateFullLists) != 1 {
		t.Fatalf("update findings calls = %d, want 1", len(updateFullLists))
	}
	gotLines := strings.Split(updateFullLists[0], "\n")
	sort.Strings(gotLines)
	if want := []string{"iter-1-list", "iter-2-list"}; !reflect.DeepEqual(gotLines, want) {
		t.Fatalf("update findings combined list lines = %#v, want %#v", gotLines, want)
	}
	if reviewerCallNum != 3 {
		t.Fatalf("reviewer LLM calls = %d, want 3 (iter1+iter2+nudge1)", reviewerCallNum)
	}
	if len(reviewerMessages) < 3 || !strings.Contains(reviewerMessages[2], "delta after nudge") {
		t.Fatalf("nudge message missing update findings delta: %#v", reviewerMessages)
	}
	if result.run.TokensUsed.TotalTokens == 0 {
		t.Fatal("extractor token usage should be folded into reviewer run")
	}
}

func TestRunAgent_NudgeErrorKeepsPriorFindingsAsPartial(t *testing.T) {
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, nudgeFinding("A", 1))},
			{resp: nudgeReviewResponse("second", 1, nudgeFinding("B", 2))},
			{err: errors.New("boom")},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{NudgeCount: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := result.run.Status, model.AgentRunStatusPartial; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if !strings.Contains(result.run.Error, "nudge 2") {
		t.Fatalf("run.Error = %q, want containing %q", result.run.Error, "nudge 2")
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A", "B"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 3 {
		t.Fatalf("llm calls = %d, want 3", len(llmClient.reqs))
	}
}

func TestRunAgent_CodeLocationRepairKeepsResponseWithoutRetry(t *testing.T) {
	valid := nudgeFinding("Valid", 1)
	invalid := nudgeFinding("Invalid", 2)
	invalid.CodeLocation.Content = "line 3"
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, valid, invalid)},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestToolAgent("review"), model.ReviewRequest{MaxOutputRetries: 1, RepoRoot: "."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"Valid", "Invalid"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if got := result.resp.Findings[1].CodeLocation.LineRange; got != (model.LineRange{Start: 3, End: 3, Count: 1}) {
		t.Fatalf("repaired range = %+v, want line 3", got)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("llm calls = %d, want no retry", len(llmClient.reqs))
	}
}

func TestRunAgent_CodeLocationRepairRunsWithoutModelTools(t *testing.T) {
	invalid := nudgeFinding("Invalid", 2)
	invalid.CodeLocation.Content = "line 3"
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, invalid)},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{MaxOutputRetries: 1, RepoRoot: "."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := result.resp.Findings[0].CodeLocation.LineRange; got != (model.LineRange{Start: 3, End: 3, Count: 1}) {
		t.Fatalf("repaired range = %+v, want line 3", got)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("llm calls = %d, want no retry", len(llmClient.reqs))
	}
}

func TestRunAgent_CodeLocationMissingAnchorRetriesWithoutFindLinesGuidance(t *testing.T) {
	missing := nudgeFinding("Missing", 1)
	missing.CodeLocation.FilePath = ""
	fixed := nudgeFinding("Fixed", 1)
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, missing)},
			{resp: nudgeReviewResponse("second", 1, fixed)},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{MaxOutputRetries: 1, RepoRoot: "."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"Fixed"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("llm calls = %d, want one retry", len(llmClient.reqs))
	}
	retryPrompt := joinedRequestContent(llmClient.reqs[1].Messages)
	if strings.Contains(retryPrompt, "find_lines") {
		t.Fatalf("no-tools code-location retry mentioned find_lines:\n%s", retryPrompt)
	}
	if !strings.Contains(retryPrompt, "file_path") || !strings.Contains(retryPrompt, "line_range") {
		t.Fatalf("retry prompt = %q, want file_path and line_range guidance", retryPrompt)
	}
}

func TestRunAgent_CodeLocationRetryConsumesOutputRetryBudget(t *testing.T) {
	missing := nudgeFinding("Missing", 1)
	missing.CodeLocation.FilePath = ""
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, missing)},
			{resp: nudgeReviewResponse("duplicate", 1,
				nudgeFindingInFile("A", "main.go", 1),
				nudgeFindingInFile("B", "main.go", 2),
			)},
			{resp: nudgeReviewResponse("should not be requested", 1, nudgeFindingInFile("C", "main.go", 1))},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), testingNudgeTestAgent(), model.ReviewRequest{MaxOutputRetries: 1, RepoRoot: "."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("llm calls = %d, want code-location retry to consume the only output retry", len(llmClient.reqs))
	}
	if result.run.Status != model.AgentRunStatusPartial {
		t.Fatalf("run status = %q, want partial after duplicate validator cannot retry", result.run.Status)
	}
}

func TestRunAgent_CodeLocationMissingAnchorRetryOnlyOnce(t *testing.T) {
	first := nudgeFinding("First missing", 1)
	first.CodeLocation.FilePath = ""
	second := nudgeFinding("Second missing", 1)
	second.CodeLocation.FilePath = ""
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: nudgeReviewResponse("first", 1, first)},
			{resp: nudgeReviewResponse("second", 1, second)},
		},
	}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{MaxOutputRetries: 2, RepoRoot: "."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"Second missing"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if got := result.resp.Findings[0].CodeLocation.FilePath; got != "" {
		t.Fatalf("file_path = %q, want unrepaired empty path after one retry", got)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("llm calls = %d, want exactly one retry", len(llmClient.reqs))
	}
}

// TestRunAgent_NoToolsFallbackKeepsRenderedSystemPrompt is the regression test
// for the no-tools fallback re-parsing already-rendered system prompts as Go
// templates: a reviewer prompt embedding a helm style guide contains
// `{{ default ... }}`, which is not a valid Go template function, so the
// re-render failed the whole agent. The fallback must reuse the prepared
// no-tools messages instead.
func TestRunAgent_NoToolsFallbackKeepsRenderedSystemPrompt(t *testing.T) {
	helmSystem := "reviewer system prompt with helm style guide content: {{ default \"standalone\" .Values.mode }} and {{ include \"chart.name\" . }}"
	llmClient := &scriptedLLM{
		results: []scriptedLLMResult{
			{resp: &llm.ReviewResponse{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"a.go"}`},
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"b.go"}`},
				},
				RawResponse: "inspecting",
			}},
			{resp: nudgeReviewResponse("final", 1, nudgeFinding("A", 1))},
		},
	}
	engine := nudgeTestEngine(llmClient)
	agent := nudgeTestToolAgent("review")
	agent.system = helmSystem

	// MaxToolCalls=1 with a two-call batch forces the final no-tools call.
	result, err := engine.runAgent(context.Background(), agent, model.ReviewRequest{MaxToolCalls: 1, MaxOutputRetries: 1})
	if err != nil {
		t.Fatalf("runAgent returned err: %v", err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("llm calls = %d, want initial plus no-tools final", len(llmClient.reqs))
	}
	noToolsReq := llmClient.reqs[1]
	if len(noToolsReq.Tools) != 0 {
		t.Fatalf("final call tools = %d, want none", len(noToolsReq.Tools))
	}
	if len(noToolsReq.Messages) == 0 || !strings.Contains(noToolsReq.Messages[0].Content, `{{ default "standalone" .Values.mode }}`) {
		t.Fatalf("no-tools system prompt lost the rendered helm content: %q", noToolsReq.Messages[0].Content)
	}
}

// TestBuildAgentLoopRequestZeroOutputRetriesIsUnlimited pins the user decision
// that an explicitly configured 0 means unlimited output retries everywhere:
// buildAgentLoopRequest must not clobber 0 back to the default (the default
// for UNSET config is injected by the config layer, not here).
func TestBuildAgentLoopRequestZeroOutputRetriesIsUnlimited(t *testing.T) {
	engine := nudgeTestEngine(&scriptedLLM{})
	loopReq, sec := engine.buildAgentLoopRequest(nudgeTestAgent("review"), model.ReviewRequest{MaxOutputRetries: 0})
	defer sec.End()
	if loopReq.MaxOutputRetries != 0 {
		t.Fatalf("MaxOutputRetries = %d, want 0 (unlimited) passed through", loopReq.MaxOutputRetries)
	}

	loopReq, sec = engine.buildAgentLoopRequest(nudgeTestAgent("review"), model.ReviewRequest{MaxOutputRetries: 3})
	defer sec.End()
	if loopReq.MaxOutputRetries != 3 {
		t.Fatalf("MaxOutputRetries = %d, want 3", loopReq.MaxOutputRetries)
	}
}

// TestRunAgent_ZeroOutputRetriesRetriesBeyondDefault proves 0 behaves as
// unlimited for a reviewer agent: more invalid responses than the old default
// (5) still end in success instead of exhausting a silently-injected budget.
func TestRunAgent_ZeroOutputRetriesRetriesBeyondDefault(t *testing.T) {
	invalidCalls := defaultMaxOutputRetries + 2
	results := make([]scriptedLLMResult, 0, invalidCalls+1)
	for range invalidCalls {
		results = append(results, scriptedLLMResult{err: &llm.InvalidResponseError{
			Reason:        "bad json",
			MissingFields: []string{"findings"},
		}})
	}
	results = append(results, scriptedLLMResult{resp: nudgeReviewResponse("final", 1, nudgeFinding("A", 1))})
	llmClient := &scriptedLLM{results: results}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{MaxOutputRetries: 0})
	if err != nil {
		t.Fatalf("runAgent returned err: %v", err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != invalidCalls+1 {
		t.Fatalf("llm calls = %d, want %d retries beyond the old default", len(llmClient.reqs), invalidCalls+1)
	}
}

// A response consisting only of invalid tool calls with no raw content must
// retry with a corrective note while keeping the history alternating: a
// placeholder assistant turn precedes the user feedback, so strict-role
// providers do not see two consecutive user messages.
func TestRunAgentInvalidToolCallsEmptyRawRetriesWithAlternatingRoles(t *testing.T) {
	invalid := &llm.ReviewResponse{ToolCalls: []llm.ToolCall{{ID: "1", Name: "bogus_tool", Arguments: "{}"}}}
	results := []scriptedLLMResult{
		{resp: invalid},
		{resp: nudgeReviewResponse("final", 1, nudgeFinding("A", 1))},
	}
	llmClient := &scriptedLLM{results: results}
	engine := nudgeTestEngine(llmClient)

	result, err := engine.runAgent(context.Background(), nudgeTestAgent("review"), model.ReviewRequest{})
	if err != nil {
		t.Fatalf("runAgent returned err: %v", err)
	}
	if got, want := findingTitles(result.resp.Findings), []string{"A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("llm calls = %d, want 2", len(llmClient.reqs))
	}
	retry := llmClient.reqs[1].Messages
	if len(retry) < 2 {
		t.Fatalf("retry messages = %#v, want at least placeholder + feedback", retry)
	}
	placeholder, feedback := retry[len(retry)-2], retry[len(retry)-1]
	if placeholder.Role != "assistant" || placeholder.Content != "[invalid tool calls]" {
		t.Fatalf("placeholder = %#v, want assistant [invalid tool calls]", placeholder)
	}
	if feedback.Role != "user" || !strings.Contains(feedback.Content, "bogus_tool") {
		t.Fatalf("feedback = %#v, want user message naming the invalid call", feedback)
	}
	for i := 1; i < len(retry); i++ {
		if retry[i].Role == retry[i-1].Role {
			t.Fatalf("retry history has consecutive %q roles at %d: %#v", retry[i].Role, i, retry)
		}
	}
}

func TestAppendNewFindingsDuplicateKeys(t *testing.T) {
	base := nudgeFinding("Same", 1)
	sameIDSameTitle := nudgeFinding(" same ", 2)
	sameIDSameTitle.ID = base.ID
	sameIDDifferentTitle := nudgeFinding("Different", 1)
	sameIDDifferentTitle.ID = base.ID
	sameTitleSameLocation := nudgeFinding("SAME", 1)
	sameTitleDifferentLocation := nudgeFinding("Same", 3)

	got := appendNewFindings([]model.Finding{base}, []model.Finding{
		sameIDSameTitle,
		sameIDDifferentTitle,
		sameTitleSameLocation,
		sameTitleDifferentLocation,
	})
	if gotTitles, want := findingTitles(got), []string{"Same", "Different", "Same"}; !reflect.DeepEqual(gotTitles, want) {
		t.Fatalf("findings = %#v, want %#v", gotTitles, want)
	}
}

func TestFormatReasoningFindingsList(t *testing.T) {
	got := formatReasoningFindingsList("first possible issue\n\n- already listed\n  second possible issue  ")
	want := "- first possible issue\n- already listed\n- second possible issue"
	if got != want {
		t.Fatalf("formatted findings = %q, want %q", got, want)
	}
}

func TestFormatReasoningFindingsListNormalizesBullets(t *testing.T) {
	input := strings.Join([]string{
		"* asterisk bullet",
		"+ plus bullet",
		"-no space after dash",
		"1. numbered dot",
		"12) numbered paren",
		"  * indented asterisk",
		"-",
		"plain line",
	}, "\n")
	want := strings.Join([]string{
		"- asterisk bullet",
		"- plus bullet",
		"- no space after dash",
		"- numbered dot",
		"- numbered paren",
		"- indented asterisk",
		"- plain line",
	}, "\n")
	if got := formatReasoningFindingsList(input); got != want {
		t.Fatalf("formatted findings = %q, want %q", got, want)
	}
}

func nudgeTestEngine(llmClient llm.Client) *Engine {
	return &Engine{
		llm:       llmClient,
		retrieval: stubRetrieval{},
		config: config.Profile{
			Model:           "test-model",
			ReasoningEffort: "high",
		},
	}
}

func nudgeTestAgent(role string) agentSpec {
	return agentSpec{
		name:       role,
		role:       role,
		system:     "system",
		user:       "user",
		schemaKind: llm.SchemaKindReview,
		hasTools:   false,
	}
}

func testingNudgeTestAgent() agentSpec {
	agent := nudgeTestAgent("review")
	agent.name = "Testing"
	agent.reviewSessionValidateResponse = validateTestingDuplicateFileResponse
	agent.reviewSessionEnforceResponse = enforceTestingDuplicateFileResponse
	return agent
}

func nudgeTestToolAgent(role string) agentSpec {
	agent := nudgeTestAgent(role)
	agent.hasTools = true
	return agent
}

func nudgeReviewResponse(label string, tokens int, findings ...model.Finding) *llm.ReviewResponse {
	return &llm.ReviewResponse{
		Findings:               findings,
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     label,
		OverallConfidenceScore: 0.9,
		RawResponse:            `{"label":"` + label + `"}`,
		TokensUsed:             model.TokenUsage{PromptTokens: tokens, CompletionTokens: tokens, TotalTokens: tokens},
	}
}

func nudgeFinding(title string, line int) model.Finding {
	return model.Finding{
		ID:              uuid.NewString(),
		Title:           title,
		Body:            "body",
		ConfidenceScore: 0.9,
		Priority:        intPtr(2),
		CodeLocation:    testLineCodeLocation("main.go", line),
	}
}

func nudgeFindingInFile(title, file string, line int) model.Finding {
	finding := nudgeFinding(title, line)
	finding.CodeLocation = testLineCodeLocation(file, line)
	return finding
}

func testLineCodeLocation(file string, line int) model.CodeLocation {
	return model.CodeLocation{
		FilePath:  file,
		LineRange: model.LineRange{Start: line, End: line, Count: 1},
		Language:  "go",
		Content:   fmt.Sprintf("line %d", line),
	}
}

func findingTitles(findings []model.Finding) []string {
	titles := make([]string, 0, len(findings))
	for _, finding := range findings {
		titles = append(titles, finding.Title)
	}
	return titles
}

func joinedRequestContent(messages []llm.Message) string {
	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		parts = append(parts, msg.Content)
	}
	return strings.Join(parts, "\n")
}

type stubRetrieval struct{}

func (stubRetrieval) GetFile(context.Context, string, string) (*retrieval.FileContent, error) {
	return &retrieval.FileContent{
		Path:     "extra.go",
		Content:  "package extra",
		Language: "go",
	}, nil
}

func (s stubRetrieval) FindLines(ctx context.Context, repoRoot, path, code string) (*retrieval.FindLinesResult, error) {
	line := 1
	if _, err := fmt.Sscanf(strings.TrimSpace(code), "line %d", &line); err == nil && line > 0 {
		if path == "" {
			path = "main.go"
		}
		return &retrieval.FindLinesResult{
			Path:       path,
			Code:       code,
			MatchCount: 1,
			Matches: []retrieval.FindLinesMatch{{
				CodeLocation: retrieval.CodeLocation{
					FilePath:  path,
					LineRange: retrieval.LineRange{Start: line, End: line, Count: 1},
					Language:  "go",
					Content:   code,
				},
			}},
		}, nil
	}
	content, err := s.GetFile(ctx, repoRoot, path)
	if err != nil {
		return nil, err
	}
	content.Path = path
	if content.Path == "" {
		content.Path = "extra.go"
	}
	return retrieval.FindLinesIn(content, code), nil
}

func (stubRetrieval) ListFiles(context.Context, string, string, int) (*retrieval.DirectoryListing, error) {
	return &retrieval.DirectoryListing{
		Path:  "pkg",
		Files: []string{"pkg/a.go", "pkg/b.go"},
	}, nil
}

func (stubRetrieval) Search(context.Context, string, string, string, int, int, bool) (*retrieval.SearchResults, error) {
	return &retrieval.SearchResults{
		Path:          "",
		Query:         "match",
		ContextLines:  5,
		CaseSensitive: false,
		ResultCount:   1,
		Results: []retrieval.SearchResult{
			{CodeLocation: retrieval.CodeLocation{
				FilePath:  "pkg/a.go",
				LineRange: retrieval.LineRange{Start: 15, End: 15, Count: 1},
				Language:  "go",
				Content:   "match",
			}},
		},
	}, nil
}

func (stubRetrieval) SearchRegex(context.Context, string, string, *regexp.Regexp, int, int) (*retrieval.SearchResults, error) {
	return &retrieval.SearchResults{
		Path:         "",
		ContextLines: 5,
		ResultCount:  0,
		Results:      []retrieval.SearchResult{},
	}, nil
}

type countingRetrieval struct {
	mu               sync.Mutex
	paths            []string
	literalResults   []retrieval.SearchResult
	regexResults     []retrieval.SearchResult
	hasCustomResults bool
}

func (r *countingRetrieval) GetFile(_ context.Context, _ string, path string) (*retrieval.FileContent, error) {
	r.mu.Lock()
	r.paths = append(r.paths, path)
	r.mu.Unlock()
	return &retrieval.FileContent{
		Path:     path,
		Content:  "package extra",
		Language: "go",
	}, nil
}

func (r *countingRetrieval) FindLines(ctx context.Context, repoRoot, path, code string) (*retrieval.FindLinesResult, error) {
	content, err := r.GetFile(ctx, repoRoot, path)
	if err != nil {
		return nil, err
	}
	return retrieval.FindLinesIn(content, code), nil
}

func (r *countingRetrieval) ListFiles(_ context.Context, _ string, path string, depth int) (*retrieval.DirectoryListing, error) {
	r.mu.Lock()
	r.paths = append(r.paths, fmt.Sprintf("list:%s:%d", path, depth))
	r.mu.Unlock()
	return &retrieval.DirectoryListing{
		Path:  path,
		Files: []string{path + "/a.go", path + "/b.go"},
	}, nil
}

func (r *countingRetrieval) Search(_ context.Context, _ string, path, query string, contextLines, maxResults int, caseSensitive bool) (*retrieval.SearchResults, error) {
	r.mu.Lock()
	r.paths = append(r.paths, fmt.Sprintf("search:%s:%s:%d:%d:%t", path, query, contextLines, maxResults, caseSensitive))
	custom := r.hasCustomResults
	customResults := r.literalResults
	r.mu.Unlock()
	var results []retrieval.SearchResult
	if custom {
		results = customResults
	} else {
		results = []retrieval.SearchResult{
			{CodeLocation: retrieval.CodeLocation{
				FilePath:  path + "/a.go",
				LineRange: retrieval.LineRange{Start: 15, End: 15, Count: 1},
				Language:  "go",
				Content:   query,
			}},
		}
		if query == "missing" {
			results = nil
		}
	}
	return &retrieval.SearchResults{
		Path:          path,
		Query:         query,
		ContextLines:  contextLines,
		MaxResults:    maxResults,
		CaseSensitive: caseSensitive,
		ResultCount:   len(results),
		Results:       results,
	}, nil
}

func (r *countingRetrieval) SearchRegex(_ context.Context, _ string, path string, pattern *regexp.Regexp, contextLines, maxResults int) (*retrieval.SearchResults, error) {
	r.mu.Lock()
	r.paths = append(r.paths, fmt.Sprintf("search_regex:%s:%s:%d:%d", path, pattern.String(), contextLines, maxResults))
	results := r.regexResults
	r.mu.Unlock()
	if results == nil {
		results = []retrieval.SearchResult{}
	}
	return &retrieval.SearchResults{
		Path:         path,
		Query:        pattern.String(),
		ContextLines: contextLines,
		MaxResults:   maxResults,
		ResultCount:  len(results),
		Results:      results,
	}, nil
}

func (*countingRetrieval) GetFileSlice(context.Context, string, string, int, int) (*retrieval.FileSlice, error) {
	return &retrieval.FileSlice{
		Path:      "extra.go",
		StartLine: 1,
		EndLine:   2,
		Content:   "package extra",
		Language:  "go",
	}, nil
}

func (*countingRetrieval) GetSymbol(context.Context, string, retrieval.SymbolRef) (*retrieval.SymbolInfo, error) {
	return nil, errors.New("unexpected GetSymbol call")
}

func (r *countingRetrieval) FindCallers(_ context.Context, _ string, symbol retrieval.SymbolRef, depth int) (*retrieval.CallHierarchy, error) {
	r.mu.Lock()
	r.paths = append(r.paths, fmt.Sprintf("callers:%s:%s:%d", symbol.Path, symbol.Name, depth))
	r.mu.Unlock()
	return &retrieval.CallHierarchy{
		Mode:  "callers",
		Depth: depth,
		Root: retrieval.CallNode{
			Name:         symbol.Name,
			CodeLocation: testCallNodeLocation(pathOrDefault(symbol.Path, "pkg/root.go"), 10, 12, "func Run() {}"),
			Children: []retrieval.CallNode{
				{
					Name:         "Start",
					CodeLocation: testCallNodeLocation("pkg/caller.go", 20, 24, "func Start() {}"),
				},
			},
		},
	}, nil
}

// testCallNodeLocation mirrors how the retrieval engine embeds a node's
// location so review-level assertions exercise the same shape.
func testCallNodeLocation(path string, startLine, endLine int, source string) retrieval.CodeLocation {
	return retrieval.CodeLocation{
		FilePath:  path,
		LineRange: retrieval.LineRange{Start: startLine, End: endLine, Count: endLine - startLine + 1},
		Language:  "go",
		Content:   source,
	}
}

func (r *countingRetrieval) FindCallees(_ context.Context, _ string, symbol retrieval.SymbolRef, depth int) (*retrieval.CallHierarchy, error) {
	r.mu.Lock()
	r.paths = append(r.paths, fmt.Sprintf("callees:%s:%s:%d", symbol.Path, symbol.Name, depth))
	r.mu.Unlock()
	return &retrieval.CallHierarchy{
		Mode:  "callees",
		Depth: depth,
		Root: retrieval.CallNode{
			Name:         symbol.Name,
			CodeLocation: testCallNodeLocation(pathOrDefault(symbol.Path, "pkg/root.go"), 10, 12, "func Run() {}"),
			Children: []retrieval.CallNode{
				{
					Name:         "Helper",
					CodeLocation: testCallNodeLocation("pkg/callee.go", 30, 34, "func Helper() {}"),
				},
			},
		},
	}, nil
}

func (stubRetrieval) GetFileSlice(context.Context, string, string, int, int) (*retrieval.FileSlice, error) {
	return &retrieval.FileSlice{
		Path:      "extra.go",
		StartLine: 1,
		EndLine:   2,
		Content:   "package extra",
		Language:  "go",
	}, nil
}

func (stubRetrieval) GetSymbol(context.Context, string, retrieval.SymbolRef) (*retrieval.SymbolInfo, error) {
	return nil, errors.New("unexpected GetSymbol call")
}

func (stubRetrieval) FindCallers(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, errors.New("unexpected FindCallers call")
}

func (stubRetrieval) FindCallees(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, errors.New("unexpected FindCallees call")
}

func TestReviewerToolDefinitionsComeFromCatalogInStableOrder(t *testing.T) {
	definitions := reviewerToolDefinitions()
	wantNames := []string{"inspect_file", "list_files", "search", "find_callers", "find_callees"}
	if len(definitions) != len(wantNames) {
		t.Fatalf("tool definitions = %d", len(definitions))
	}
	for i, wantName := range wantNames {
		if definitions[i].Name != wantName {
			t.Fatalf("tool[%d].Name = %q, want %q", i, definitions[i].Name, wantName)
		}
		if definitions[i].Description == "" {
			t.Fatalf("tool[%d] missing description", i)
		}
	}
}

func TestReviewerToolDefinitionsContainValidCatalogSchemas(t *testing.T) {
	definitions := reviewerToolDefinitions()
	requiredByTool := map[string][]string{
		"inspect_file": {"path"},
		"search":       {"query"},
		"find_callers": {"symbol"},
		"find_callees": {"symbol"},
	}
	for _, definition := range definitions {
		var schema map[string]any
		if err := json.Unmarshal(definition.Parameters, &schema); err != nil {
			t.Fatalf("%s schema should be valid JSON: %v", definition.Name, err)
		}
		if schema["type"] != "object" {
			t.Fatalf("%s schema type = %#v", definition.Name, schema["type"])
		}
		if schema["additionalProperties"] != false {
			t.Fatalf("%s schema additionalProperties = %#v", definition.Name, schema["additionalProperties"])
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok || len(properties) == 0 {
			t.Fatalf("%s schema missing properties: %#v", definition.Name, schema["properties"])
		}
		if definition.Name == "search" {
			maxResults, ok := properties["max_results"].(map[string]any)
			if !ok {
				t.Fatalf("search schema missing max_results property: %#v", properties)
			}
			if maxResults["minimum"] != float64(0) {
				t.Fatalf("search max_results minimum = %#v, want 0", maxResults["minimum"])
			}
		}
		required := map[string]bool{}
		for _, value := range schemaStringSlice(schema["required"]) {
			required[value] = true
		}
		for _, name := range requiredByTool[definition.Name] {
			if !required[name] {
				t.Fatalf("%s schema missing required field %q: %#v", definition.Name, name, schema["required"])
			}
		}
	}
}

func TestToolInstructionsTemplateUsesGeneratedListing(t *testing.T) {
	engine := NewEngine(stubSource{}, &capturingLLM{}, nil, config.Profile{})
	rendered, err := engine.renderToolInstructions(toolInstructionsConfig{agentRole: "review", parallelToolCallGuidance: true})
	if err != nil {
		t.Fatal(err)
	}
	listing := toolInstructionsListing()
	if !strings.Contains(rendered, listing) {
		t.Fatalf("tool instructions missing generated listing:\n%s", rendered)
	}
	if !strings.Contains(listing, "- `search` tool with a `query` (search text, or an exact line or block of code)") {
		t.Fatalf("generated listing missing search tool: %q", listing)
	}
	if strings.Contains(listing, "`find_lines`") {
		t.Fatalf("generated listing still mentions the removed find_lines tool: %q", listing)
	}
}

func TestToolErrorMessagesComeFromCatalog(t *testing.T) {
	tests := []struct {
		name string
		data toolErrorData
		want string
	}{
		{
			name: "retrieval unavailable",
			data: toolErrorData{Code: "retrieval_unavailable"},
			want: "retrieval is unavailable for this review",
		},
		{
			name: "unsupported tool",
			data: toolErrorData{Code: "unsupported_tool", ToolName: "bad_tool"},
			want: `unsupported tool "bad_tool"`,
		},
		{
			name: "missing argument",
			data: toolErrorData{Code: "missing_argument", Argument: "query", Schema: toolArgumentSchema("search")},
			want: `missing required argument: query; expected {"path"?: "<repo-relative path>", "query": "<text, or line(s) of code>", "context_lines"?: int, "max_results"?: int, "case_sensitive"?: bool}`,
		},
		{
			name: "already requested file",
			data: toolErrorData{Code: "already_requested_file"},
			want: "file contents were already provided for this review",
		},
		{
			name: "already requested tool",
			data: toolErrorData{Code: "already_requested_tool"},
			want: "tool result was already provided for this review",
		},
		{
			name: "encoding failed",
			data: toolErrorData{Code: "encoding_failed"},
			want: "failed to encode tool result",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toolErrorMessage(tt.data); got != tt.want {
				t.Fatalf("toolErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSyntheticToolFollowupRendersBranches(t *testing.T) {
	engine := NewEngine(stubSource{}, &capturingLLM{}, nil, config.Profile{})
	baseHistory := []toolCallHistoryEntry{
		{
			ToolCall: llm.ToolCall{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run(","context_lines":0}`},
			Result:   toolResultSummary{Lines: 2, Files: 1},
		},
	}
	optimizedHistory := []toolCallHistoryEntry{
		{
			ToolCall:    llm.ToolCall{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run(","context_lines":0}`},
			Result:      toolResultSummary{Lines: 2, Files: 1},
			OptimizedTo: "find_callers",
		},
	}
	errorHistory := []toolCallHistoryEntry{
		{
			ToolCall: llm.ToolCall{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"missing.go"}`},
			Result:   toolResultSummary{IsError: true, Code: "retrieval_failed", Message: "not found"},
		},
	}

	retry, err := engine.renderSyntheticToolFollowup(errorHistory, "review")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(retry, "Please retry the last tool call.") {
		t.Fatalf("retry follow-up = %q", retry)
	}

	contextRendered, err := engine.renderSyntheticToolFollowup(baseHistory, "context")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(contextRendered, "return the summary of what you believe the patch is intended to do") {
		t.Fatalf("context follow-up = %q", contextRendered)
	}

	reviewRendered, err := engine.renderSyntheticToolFollowup(optimizedHistory, "review")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reviewRendered, "1. search (replaced by find_callers): tool_call_id=\"call_1\"") {
		t.Fatalf("review follow-up missing structured history: %q", reviewRendered)
	}
	if !strings.Contains(reviewRendered, "return the requested JSON format") {
		t.Fatalf("review follow-up = %q", reviewRendered)
	}

	verifyRendered, err := engine.renderSyntheticToolFollowup(baseHistory, "verify")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(verifyRendered, "return the requested JSON format") || !strings.Contains(verifyRendered, "verification") {
		t.Fatalf("verify follow-up = %q", verifyRendered)
	}

	// The discussion agent needs its own continuation guidance: without it the
	// synthetic turn only lists prior tool calls and a tool-assisted chat can
	// answer with a tool recap instead of an answer.
	discussRendered, err := engine.renderSyntheticToolFollowup(baseHistory, "discuss")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(discussRendered, "answer the author's question directly in plain markdown") {
		t.Fatalf("discuss follow-up = %q", discussRendered)
	}
}

func TestEngineRunsContextVectorsMergeWithIndependentToolBudgets(t *testing.T) {
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, trimmed, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != len(reviewVectors) {
		t.Fatalf("merged findings = %d, want %d", len(result.Findings), len(reviewVectors))
	}
	// The synthetic vector findings form one Possible cluster, so the cluster
	// merge runs exactly one micro-merge agent.
	expectedAgentRuns := 1 + len(reviewVectors) + 1
	if len(result.AgentRuns) != expectedAgentRuns {
		t.Fatalf("agent runs = %d, want context + %d reviewers + 1 merge", len(result.AgentRuns), len(reviewVectors))
	}
	expectedToolCalls := 1 + len(reviewVectors)
	if result.TotalToolCalls != expectedToolCalls {
		t.Fatalf("tool calls = %d, want context + one per vector", result.TotalToolCalls)
	}
	if llmClient.verifyCalls != len(reviewVectors) {
		t.Fatalf("verify calls = %d, want one per vector finding", llmClient.verifyCalls)
	}
	if result.VerifyTokensUsed.TotalTokens != len(reviewVectors)*2 {
		t.Fatalf("verify tokens = %d, want %d", result.VerifyTokensUsed.TotalTokens, len(reviewVectors)*2)
	}
	wantUsage := model.TokenUsage{
		PromptTokens:     1 + len(reviewVectors)*2 + 3 + len(reviewVectors),
		CompletionTokens: 1 + len(reviewVectors)*1 + 1 + len(reviewVectors),
		TotalTokens:      2 + len(reviewVectors)*3 + 4 + len(reviewVectors)*2,
	}
	if result.TokensUsed != wantUsage {
		t.Fatalf("total tokens = %+v, want %+v including verifier spend", result.TokensUsed, wantUsage)
	}
	if got := llmClient.events[len(llmClient.events)-1]; got != "merge" {
		t.Fatalf("last event = %q, want merge after verification", got)
	}
	if len(trimmed.SupplementalContext) == 0 {
		t.Fatal("context was not attached")
	}
	if trimmed.SupplementalContext[0].Kind != "context_tool_result" {
		t.Fatalf("first supplemental context = %#v, want context tool result", trimmed.SupplementalContext[0])
	}
	if !strings.Contains(llmClient.contextSystem, "DO NOT produce review findings yourself") {
		t.Fatalf("context prompt missing standalone instructions: %q", llmClient.contextSystem)
	}
	if !strings.Contains(llmClient.contextSystem, "# Go — Common Developer Guideline") || strings.Contains(llmClient.contextSystem, "### Go — Common Developer Guideline (go)") {
		t.Fatalf("context prompt missing styleguide content: %q", llmClient.contextSystem)
	}
	if strings.Contains(llmClient.contextSystem, "Make sure to output the findings") {
		t.Fatalf("context prompt should not include review output instructions: %q", llmClient.contextSystem)
	}
	if llmClient.mergeTools != 0 {
		t.Fatalf("merge tools = %d, want 0", llmClient.mergeTools)
	}
	if _, ok := llmClient.mergePayload["context_response"]; ok {
		t.Fatalf("merge payload should not include context_response: %#v", llmClient.mergePayload)
	}
	if notes, _ := llmClient.mergePayload["context_agent_notes"].(string); !strings.Contains(notes, "## Notes") {
		t.Fatalf("merge payload context_agent_notes = %#v", llmClient.mergePayload["context_agent_notes"])
	}
	if reviewContext, ok := llmClient.mergePayload["review_context"].(map[string]any); ok {
		if _, ok := reviewContext["style_guides"]; ok {
			t.Fatalf("merge review_context should not include style_guides: %#v", reviewContext["style_guides"])
		}
	}
	if len(llmClient.mergeRequests) == 0 {
		t.Fatal("missing merge request")
	}
	if system := llmClient.mergeRequests[0].Messages[0].Content; !strings.Contains(system, "# Go — Common Developer Guideline") || strings.Contains(system, "### Go — Common Developer Guideline (go)") {
		t.Fatalf("merge prompt missing styleguide content: %q", system)
	}
	clusterEntries, ok := llmClient.mergePayload["cluster_findings"].([]any)
	if !ok || len(clusterEntries) != len(reviewVectors) {
		t.Fatalf("merge payload cluster_findings = %#v, want %d entries", llmClient.mergePayload["cluster_findings"], len(reviewVectors))
	}
	for _, raw := range clusterEntries {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("cluster entry = %#v", raw)
		}
		if reviewer, _ := entry["reviewer"].(string); reviewer == "" {
			t.Fatalf("cluster entry missing reviewer provenance: %#v", entry)
		}
		if _, ok := entry["finding"].(map[string]any); !ok {
			t.Fatalf("cluster entry missing finding: %#v", entry)
		}
	}
	for _, vector := range reviewVectors {
		if llmClient.vectorCalls[vector.name] != 2 {
			t.Fatalf("%s calls = %d, want tool + final", vector.name, llmClient.vectorCalls[vector.name])
		}
		system := llmClient.vectorSystem[vector.name]
		if !strings.Contains(system, "You are acting as a senior engineer performing a thorough code review") {
			t.Fatalf("%s prompt lost base review prompt", vector.name)
		}
		if !strings.Contains(system, "## FOCUS ON ") {
			t.Fatalf("%s prompt missing focus snippet", vector.name)
		}
		if !strings.Contains(system, "# Go — Common Developer Guideline") || strings.Contains(system, "### Go — Common Developer Guideline (go)") {
			t.Fatalf("%s prompt missing styleguide content: %q", vector.name, system)
		}
		if !strings.Contains(system, "provided `toolchain_versions`") {
			t.Fatalf("%s prompt missing toolchain reminder: %q", vector.name, system)
		}
		contextNote := llmClient.vectorContext[vector.name]
		if !strings.Contains(contextNote, "## Notes") || !strings.Contains(contextNote, "## Assumed Patch Purpose") {
			t.Fatalf("%s prompt missing context agent markdown note: %q", vector.name, contextNote)
		}
	}
}

func TestMultiAgentVerifiesBeforeMergeAndDropsInvalidFindings(t *testing.T) {
	llmClient := &multiAgentLLM{
		verifyInvalid: map[string]bool{"Fix Security": true},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
		Concurrency:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if llmClient.verifyCalls != len(reviewVectors) {
		t.Fatalf("verify calls = %d, want %d", llmClient.verifyCalls, len(reviewVectors))
	}
	firstMerge := slices.Index(llmClient.events, "merge")
	if firstMerge < len(reviewVectors) {
		t.Fatalf("first merge event at %d before verification complete: %#v", firstMerge, llmClient.events)
	}
	clusterEntries, ok := llmClient.mergePayload["cluster_findings"].([]any)
	if !ok {
		t.Fatalf("merge payload cluster_findings = %#v", llmClient.mergePayload["cluster_findings"])
	}
	var mergedFindingCount int
	var sawSecurity bool
	for _, raw := range clusterEntries {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("cluster entry = %#v", raw)
		}
		finding, ok := entry["finding"].(map[string]any)
		if !ok {
			t.Fatalf("cluster entry missing finding: %#v", entry)
		}
		mergedFindingCount++
		if finding["title"] == "Fix Security" {
			sawSecurity = true
		}
		if _, ok := finding["verification"].(map[string]any); !ok {
			t.Fatalf("finding missing verification in merge input: %#v", finding)
		}
	}
	if mergedFindingCount != len(reviewVectors)-1 {
		t.Fatalf("final merge input findings = %d, want %d", mergedFindingCount, len(reviewVectors)-1)
	}
	if sawSecurity {
		t.Fatal("invalid Security finding reached merge input")
	}
}

// The cluster merge agent only sees ambiguous (Possible) clusters and may
// merge them down to one finding without rewording anything; the validator
// must accept that — the old pairwise validator deadlocked on it.
func TestMultiAgentClusterMergeAcceptsMergedClusterWithoutTextChanges(t *testing.T) {
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
		MaxOutputRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	clusterRequests := clusterMergeRequests(t, llmClient.mergeRequests)
	if len(clusterRequests) != 1 {
		t.Fatalf("merge requests = %d, want one micro-merge for the single cluster", len(clusterRequests))
	}
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "merge step") {
			t.Fatalf("unexpected merge warning: %#v", result.Warnings)
		}
	}
}

func mergePayloadFromRequest(t *testing.T, req *llm.ReviewRequest) map[string]any {
	t.Helper()
	var payload map[string]any
	content := taskMessageContent(req)
	if content == "" {
		t.Fatalf("merge request messages = %d", len(req.Messages))
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("merge payload unmarshal: %v", err)
	}
	return payload
}

func clusterMergeRequests(t *testing.T, requests []*llm.ReviewRequest) []*llm.ReviewRequest {
	t.Helper()
	var out []*llm.ReviewRequest
	for _, req := range requests {
		payload := mergePayloadFromRequest(t, req)
		if _, ok := payload["cluster_findings"]; ok {
			out = append(out, req)
		}
	}
	return out
}

func TestFindingPromptPayloadDisableSuggestionsStripsNestedSuggestions(t *testing.T) {
	original := model.Finding{
		ID:          "11111111-1111-4111-8111-111111111111",
		Title:       "Fix issue",
		Suggestions: []model.Suggestion{{Body: "top-level suggestion", LineRange: model.LineRange{Start: 1, End: 1}}},
		Finalization: &model.FindingFinalization{
			Title:       "Final issue",
			Suggestions: []model.Suggestion{{Body: "final suggestion", LineRange: model.LineRange{Start: 2, End: 2}}},
		},
		Summarization: &model.FindingSummarization{
			Body:        "Short issue",
			Suggestions: []model.Suggestion{{Body: "summary suggestion", LineRange: model.LineRange{Start: 3, End: 3}}},
		},
	}

	kept := findingPromptPayload(original, false)
	if !reflect.DeepEqual(kept, original) {
		t.Fatalf("non-skipped payload mutated finding: %+v", kept)
	}
	stripped := findingPromptPayload(original, true)
	if len(stripped.Suggestions) != 0 {
		t.Fatalf("top-level suggestions = %+v, want stripped", stripped.Suggestions)
	}
	if stripped.Finalization == nil || len(stripped.Finalization.Suggestions) != 0 {
		t.Fatalf("finalization suggestions = %+v, want stripped", stripped.Finalization)
	}
	if stripped.Summarization == nil || len(stripped.Summarization.Suggestions) != 0 {
		t.Fatalf("summarization suggestions = %+v, want stripped", stripped.Summarization)
	}
	if len(original.Suggestions) != 1 || len(original.Finalization.Suggestions) != 1 || len(original.Summarization.Suggestions) != 1 {
		t.Fatalf("original finding was mutated: %+v", original)
	}
}

func TestDedupeAgentDisableSuggestionsOmitsSuggestions(t *testing.T) {
	a := clusterTestFinding("Fix alpha issue", 1)
	a.Suggestions = []model.Suggestion{{Body: "alpha suggestion should be omitted", LineRange: model.LineRange{Start: 1, End: 1}}}
	b := clusterTestFinding("Fix beta issue", 13)
	b.Suggestions = []model.Suggestion{{Body: "beta suggestion should be omitted", LineRange: model.LineRange{Start: 13, End: 13}}}
	llmClient := &multiAgentLLM{
		dedupeResponses: []*llm.ReviewResponse{{
			Findings:               []model.Finding{a, b},
			OverallCorrectness:     "patch is incorrect",
			OverallExplanation:     "deduped",
			OverallConfidenceScore: 0.9,
		}},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	input := agentResult{
		run:  model.AgentRun{Name: "Reviewer A", Role: "review"},
		resp: &llm.ReviewResponse{Findings: []model.Finding{a, b}, OverallConfidenceScore: 0.9},
	}

	if _, err := engine.callDedupeAgent(context.Background(), "", input, nil, llm.ResponseConstraints{}, model.ReviewRequest{DisableSuggestions: true}); err != nil {
		t.Fatalf("callDedupeAgent returned err: %v", err)
	}
	if len(llmClient.mergeRequests) != 1 {
		t.Fatalf("dedupe requests = %d, want 1", len(llmClient.mergeRequests))
	}
	req := llmClient.mergeRequests[0]
	if strings.Contains(req.Messages[0].Content, "include suggestions") {
		t.Fatalf("dedupe system prompt should not ask for suggestions when skipped:\n%s", req.Messages[0].Content)
	}
	userPrompt := taskMessageContent(req)
	if strings.Contains(userPrompt, "suggestion should be omitted") || strings.Contains(userPrompt, `"suggestions"`) {
		t.Fatalf("dedupe user prompt should not include suggestions:\n%s", userPrompt)
	}
}

func TestClusterMergeDisableSuggestionsOmitsSuggestions(t *testing.T) {
	a := clusterTestFinding("Fix alpha issue", 1)
	a.Suggestions = []model.Suggestion{{Body: "alpha suggestion should be omitted", LineRange: model.LineRange{Start: 1, End: 1}}}
	b := clusterTestFinding("Fix beta issue", 13)
	b.Suggestions = []model.Suggestion{{Body: "beta suggestion should be omitted", LineRange: model.LineRange{Start: 13, End: 13}}}
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "Reviewer A", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{a}, OverallConfidenceScore: 0.9}},
		{name: "Reviewer B", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{b}, OverallConfidenceScore: 0.9}},
	}

	result, _ := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{DisableSuggestions: true})

	if len(llmClient.mergeRequests) != 1 {
		t.Fatalf("merge requests = %d, want 1", len(llmClient.mergeRequests))
	}
	req := llmClient.mergeRequests[0]
	if strings.Contains(req.Messages[0].Content, "include suggestions") {
		t.Fatalf("merge system prompt should not ask for suggestions when skipped:\n%s", req.Messages[0].Content)
	}
	userPrompt := taskMessageContent(req)
	if strings.Contains(userPrompt, "suggestion should be omitted") || strings.Contains(userPrompt, `"suggestions"`) {
		t.Fatalf("merge user prompt should not include suggestions:\n%s", userPrompt)
	}
	for _, finding := range result.resp.Findings {
		if len(finding.Suggestions) != 0 {
			t.Fatalf("merge output suggestions = %+v, want stripped", finding.Suggestions)
		}
	}
}

func TestMultiAgentMergeValidationWarnsAfterRetryExhausted(t *testing.T) {
	ghost := model.Finding{
		Title:           "Ghost",
		Body:            "new",
		ConfidenceScore: 0.8,
		Priority:        intPtr(2),
		CodeLocation:    model.CodeLocation{FilePath: "ghost.go", LineRange: model.LineRange{Start: 99, End: 99}},
	}
	llmClient := &multiAgentLLM{
		mergeResponses: []*llm.ReviewResponse{
			{Findings: []model.Finding{ghost}, OverallCorrectness: "patch is incorrect"},
			{Findings: []model.Finding{ghost}, OverallCorrectness: "patch is incorrect"},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
		MaxOutputRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	clusterRequests := clusterMergeRequests(t, llmClient.mergeRequests)
	if len(clusterRequests) != 2 {
		t.Fatalf("merge requests = %d, want initial attempt plus one retry", len(clusterRequests))
	}
	foundWarning := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "Merge Findings merge step partial result") && strings.Contains(warning, "unmatched_finding") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("warnings = %#v, want nonfatal merge validation warning", result.Warnings)
	}
	for _, finding := range result.Findings {
		if finding.Title == "Ghost" {
			t.Fatalf("invalid micro-merge must not keep unmatched ghost finding: %#v", result.Findings)
		}
	}
	// The failed micro-merge keeps its cluster unmerged instead of dropping it.
	if len(result.Findings) != len(reviewVectors) {
		t.Fatalf("findings = %d, want all %d cluster findings preserved", len(result.Findings), len(reviewVectors))
	}
}

// clusterTestFinding builds findings that pair up as dedupe.Possible (same
// file, 12-line gap via the line argument, shared body, moderately related
// titles) so runClusterMergeAgents sends them to a micro-merge agent.
func clusterTestFinding(title string, line int) model.Finding {
	f := mergeTestFinding(title, line)
	f.ID = uuid.NewString()
	f.Verification.ID = f.ID
	return f
}

func TestClusterMergeReturnedRunMatchesPartialStepStatus(t *testing.T) {
	a := clusterTestFinding("Fix alpha issue", 1)
	b := clusterTestFinding("Fix beta issue", 13)
	ghost := clusterTestFinding("Ghost", 99)
	ghost.CodeLocation.FilePath = "ghost.go"
	engine := NewEngine(stubSource{}, &multiAgentLLM{
		mergeResponses: []*llm.ReviewResponse{
			{Findings: []model.Finding{ghost}, OverallCorrectness: "patch is incorrect"},
			{Findings: []model.Finding{ghost}, OverallCorrectness: "patch is incorrect"},
		},
	}, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "Reviewer A", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{a}, OverallConfidenceScore: 0.9}},
		{name: "Reviewer B", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{b}, OverallConfidenceScore: 0.9}},
	}

	result, runs := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{MaxOutputRetries: 1})

	if len(runs) != 1 {
		t.Fatalf("merge runs = %d, want 1", len(runs))
	}
	if runs[0].Status != model.AgentRunStatusPartial {
		t.Fatalf("stored run status = %q, want partial", runs[0].Status)
	}
	if result.run.Status != model.AgentRunStatusPartial {
		t.Fatalf("returned run status = %q, want partial", result.run.Status)
	}
	if !strings.Contains(result.run.Error, "unmatched_finding") {
		t.Fatalf("returned run error = %q, want validation reason", result.run.Error)
	}
	if len(result.resp.Findings) != 2 {
		t.Fatalf("findings = %d, want cluster preserved unmerged", len(result.resp.Findings))
	}
	for _, finding := range result.resp.Findings {
		if finding.Title == "Ghost" {
			t.Fatalf("invalid micro-merge response must be discarded: %#v", result.resp.Findings)
		}
	}
}

func TestClusterMergeErrorMarksRunFailedAndKeepsFindings(t *testing.T) {
	engine := NewEngine(stubSource{}, &multiAgentLLM{
		mergeFailErr: errors.New("merge upstream fail"),
	}, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "Reviewer A", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{clusterTestFinding("Fix alpha issue", 1)}, OverallConfidenceScore: 0.9}},
		{name: "Reviewer B", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{clusterTestFinding("Fix beta issue", 13)}, OverallConfidenceScore: 0.9}},
	}

	result, runs := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(runs) != 1 {
		t.Fatalf("merge runs = %d, want 1", len(runs))
	}
	if runs[0].Status != model.AgentRunStatusFailed {
		t.Fatalf("stored run status = %q, want failed", runs[0].Status)
	}
	if !strings.Contains(result.run.Error, "merge upstream fail") {
		t.Fatalf("returned run error = %q, want merge error", result.run.Error)
	}
	if len(result.resp.Findings) != 2 {
		t.Fatalf("findings = %d, want cluster preserved unmerged on failure", len(result.resp.Findings))
	}
}

func TestClusterMergeFoldsMechanicalDuplicatesWithoutLLM(t *testing.T) {
	a := clusterTestFinding("Fix duplicated cleanup issue", 5)
	b := clusterTestFinding("Fix duplicated cleanup issue", 5)
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "Reviewer A", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{a}, OverallConfidenceScore: 0.9}},
		{name: "Reviewer B", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{b}, OverallConfidenceScore: 0.9}},
	}

	result, runs := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(llmClient.mergeRequests) != 0 {
		t.Fatalf("merge requests = %d, want pure mechanical fold", len(llmClient.mergeRequests))
	}
	if len(result.resp.Findings) != 1 {
		t.Fatalf("findings = %d, want duplicates folded to one", len(result.resp.Findings))
	}
	if len(runs) != 1 || runs[0].Status != model.AgentRunStatusSkipped {
		t.Fatalf("runs = %#v, want one synthetic skipped run", runs)
	}
}

func TestClusterMergeDistinctFindingsPassThroughWithoutLLM(t *testing.T) {
	a := clusterTestFinding("Fix alpha issue", 1)
	b := clusterTestFinding("Improve unrelated subsystem", 1)
	b.CodeLocation.FilePath = "other.go"
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "Reviewer A", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{a}, OverallConfidenceScore: 0.9}},
		{name: "Reviewer B", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{b}, OverallConfidenceScore: 0.9}},
	}

	result, runs := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(llmClient.mergeRequests) != 0 {
		t.Fatalf("merge requests = %d, want none for distinct findings", len(llmClient.mergeRequests))
	}
	if len(result.resp.Findings) != 2 {
		t.Fatalf("findings = %d, want both distinct findings kept", len(result.resp.Findings))
	}
	if len(runs) != 1 || runs[0].Status != model.AgentRunStatusSkipped {
		t.Fatalf("runs = %#v, want one synthetic skipped run", runs)
	}
}

// Imported bare findings files carry no overall_correctness; when the merge
// resolves purely mechanically (no LLM verdict either), preserved findings
// must not be reported under a "patch is correct" default.
func TestClusterMergeVerdictlessInputsWithFindingsReportIncorrect(t *testing.T) {
	a := clusterTestFinding("Fix alpha issue", 1)
	b := clusterTestFinding("Improve unrelated subsystem", 1)
	b.CodeLocation.FilePath = "other.go"
	engine := NewEngine(stubSource{}, &multiAgentLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "a.json", role: "merge_input", response: &llm.ReviewResponse{Findings: []model.Finding{a}}},
		{name: "b.json", role: "merge_input", response: &llm.ReviewResponse{Findings: []model.Finding{b}}},
	}

	result, _ := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(result.resp.Findings) != 2 {
		t.Fatalf("findings = %d, want both preserved", len(result.resp.Findings))
	}
	if result.resp.OverallCorrectness != "patch is incorrect" {
		t.Fatalf("overall correctness = %q with %d findings, want patch is incorrect", result.resp.OverallCorrectness, len(result.resp.Findings))
	}

	// Explicit "patch is correct" alongside findings stays untouched.
	inputs[0].response.OverallCorrectness = "patch is correct"
	result, _ = engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{})
	if result.resp.OverallCorrectness != "patch is correct" {
		t.Fatalf("overall correctness = %q, want explicit input verdict preserved", result.resp.OverallCorrectness)
	}
}

// Cross-file findings with near-identical titles cluster as Possible and are
// judged by a micro-merge agent instead of passing through as duplicates.
func TestClusterMergeCrossFileTitleTwinsRouteToLLM(t *testing.T) {
	a := clusterTestFinding("Test coverage missing for nested subdirectories", 176)
	a.CodeLocation.FilePath = "controllers/logrotate_assets_test.go"
	b := clusterTestFinding("Test coverage missing for nested subdirectories under cronjobs and container", 38)
	b.CodeLocation.FilePath = "controllers/logrotate_assets_integration_test.go"
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "Reviewer A", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{a}, OverallConfidenceScore: 0.9}},
		{name: "Reviewer B", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{b}, OverallConfidenceScore: 0.9}},
	}

	result, runs := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(llmClient.mergeRequests) != 1 {
		t.Fatalf("merge requests = %d, want one micro-merge for the cross-file cluster", len(llmClient.mergeRequests))
	}
	payload := mergePayloadFromRequest(t, llmClient.mergeRequests[0])
	cluster, _ := payload["cluster_findings"].([]any)
	if len(cluster) != 2 {
		t.Fatalf("cluster payload = %d findings, want both cross-file twins", len(cluster))
	}
	if len(runs) != 1 || runs[0].Status != model.AgentRunStatusOK {
		t.Fatalf("runs = %#v, want one ok micro-merge", runs)
	}
	// Default fixture echoes the cluster unchanged; both findings survive.
	if len(result.resp.Findings) != 2 {
		t.Fatalf("findings = %d, want echo of both", len(result.resp.Findings))
	}
}

func TestClusterMergeCrossFileRelatedTitleAndBodyRouteToLLM(t *testing.T) {
	a := clusterTestFinding("Bash script missing strict mode flags", 3)
	a.Body = "Script uses set -e only; unset variables and pipeline failures pass silently without strict mode flags."
	a.CodeLocation.FilePath = "controllers/logrotate/logrotate.sh"
	b := clusterTestFinding("Bash strict mode flags not enabled per style guide", 100)
	b.Body = "Tests cover SIGTERM but not unset variables or pipeline failures caused by missing strict mode flags."
	b.CodeLocation.FilePath = "controllers/logrotate_assets_test.go"
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "Reviewer A", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{a}, OverallConfidenceScore: 0.9}},
		{name: "Reviewer B", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{b}, OverallConfidenceScore: 0.9}},
	}

	result, runs := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(llmClient.mergeRequests) != 1 {
		t.Fatalf("merge requests = %d, want one micro-merge for the related cross-file cluster", len(llmClient.mergeRequests))
	}
	payload := mergePayloadFromRequest(t, llmClient.mergeRequests[0])
	cluster, _ := payload["cluster_findings"].([]any)
	if len(cluster) != 2 {
		t.Fatalf("cluster payload = %d findings, want both related cross-file findings", len(cluster))
	}
	if len(runs) != 1 || runs[0].Status != model.AgentRunStatusOK {
		t.Fatalf("runs = %#v, want one ok micro-merge", runs)
	}
	// Default fixture echoes the cluster unchanged; both findings survive.
	if len(result.resp.Findings) != 2 {
		t.Fatalf("findings = %d, want echo of both", len(result.resp.Findings))
	}
}

func TestClusterMergeCrossFileRootCauseRouteToLLM(t *testing.T) {
	a := clusterTestFinding("Unquoted path expansion breaks target-dir restore", 10)
	a.Body = "The target-dir template leaves `$TMP` unquoted in `find`; `mkdir` and `chown` also receive unquoted paths. Paths containing spaces split into separate arguments and the restore fails."
	a.CodeLocation.FilePath = "pkg/restore/restore-directory-target-dir.tpl"
	b := clusterTestFinding("Directory template mishandles paths containing spaces", 12)
	b.Body = "The directory template passes `$TMP` to `find` without quotes, then calls `mkdir` and `chown` with unquoted destination paths. Whitespace in a path causes word splitting and command failure."
	b.CodeLocation.FilePath = "pkg/restore/restore-directory.tpl"
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "Reviewer A", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{a}, OverallConfidenceScore: 0.9}},
		{name: "Reviewer B", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{b}, OverallConfidenceScore: 0.9}},
	}

	result, runs := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(llmClient.mergeRequests) != 1 {
		t.Fatalf("merge requests = %d, want one micro-merge for the root-cause cross-file cluster", len(llmClient.mergeRequests))
	}
	payload := mergePayloadFromRequest(t, llmClient.mergeRequests[0])
	cluster, _ := payload["cluster_findings"].([]any)
	if len(cluster) != 2 {
		t.Fatalf("cluster payload = %d findings, want both root-cause findings", len(cluster))
	}
	signals, ok := payload["cluster_signals"].([]any)
	if !ok {
		t.Fatalf("cluster_signals = %#v", payload["cluster_signals"])
	}
	if len(signals) != 1 || signals[0] != "same root-cause signals across related files" {
		t.Fatalf("cluster_signals = %#v, want root-cause routing reason", signals)
	}
	for _, raw := range cluster {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("cluster entry = %#v", raw)
		}
		if _, ok := entry["root_cause_key"]; ok {
			t.Fatalf("cluster entry should not include content-specific root_cause_key: %#v", entry)
		}
		if _, ok := entry["fix_pattern"]; ok {
			t.Fatalf("cluster entry should not include content-specific fix_pattern: %#v", entry)
		}
		if _, ok := entry["finding"].(map[string]any); !ok {
			t.Fatalf("cluster entry missing finding payload: %#v", entry)
		}
	}
	system := llmClient.mergeRequests[0].Messages[0].Content
	for _, want := range []string{
		"root-cause signals",
		"same root cause and fix pattern",
		"Do not split findings only because the affected variable",
		"Same vulnerability/failure class + same mitigation/fix",
		"`cluster_signals`",
	} {
		if !strings.Contains(system, want) {
			t.Fatalf("merge system prompt missing %q:\n%s", want, system)
		}
	}
	if len(runs) != 1 || runs[0].Status != model.AgentRunStatusOK {
		t.Fatalf("runs = %#v, want one ok micro-merge", runs)
	}
	if len(result.resp.Findings) != 2 {
		t.Fatalf("findings = %d, want echo of both", len(result.resp.Findings))
	}
}

func TestClusterMergeSingleInputSkipsMergeAndReturnsReviewerFindings(t *testing.T) {
	finding := mergeTestFinding("Fix A", 1)
	engine := NewEngine(stubSource{}, &multiAgentLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{{
		name:     "Reviewer A",
		role:     "review",
		response: &llm.ReviewResponse{Findings: []model.Finding{finding}, OverallConfidenceScore: 0.9},
	}}

	result, runs := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(runs) != 1 {
		t.Fatalf("merge runs = %d, want 1 skipped run", len(runs))
	}
	if runs[0].Status != model.AgentRunStatusSkipped {
		t.Fatalf("merge status = %q, want skipped", runs[0].Status)
	}
	if len(result.resp.Findings) != 1 || result.resp.Findings[0].Title != finding.Title {
		t.Fatalf("result findings = %#v, want single reviewer finding", result.resp.Findings)
	}
	if len(engine.llm.(*multiAgentLLM).mergeRequests) != 0 {
		t.Fatalf("merge requests = %d, want none for single input", len(engine.llm.(*multiAgentLLM).mergeRequests))
	}
}

func TestCloneReviewResponseDeepCopiesMutableFindingFields(t *testing.T) {
	priority := 2
	original := &llm.ReviewResponse{
		Findings: []model.Finding{{
			Title:           "Fix A",
			Body:            "body",
			ConfidenceScore: 0.9,
			Priority:        &priority,
			CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
			Suggestions:     []model.Suggestion{{Body: "suggestion", LineRange: model.LineRange{Start: 1, End: 1}}},
			Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.9, Remarks: "verified"},
			Finalization:    &model.FindingFinalization{Title: "final", Body: "final body", Priority: 2, ConfidenceScore: 0.8, Remarks: "finalized"},
		}},
	}

	clone := cloneReviewResponse(original)
	*clone.Findings[0].Priority = 1
	clone.Findings[0].Suggestions[0].Body = "changed"
	clone.Findings[0].Verification.Verdict = model.VerdictRefuted
	clone.Findings[0].Finalization.Title = "changed"

	if *original.Findings[0].Priority != 2 {
		t.Fatalf("original priority mutated = %d", *original.Findings[0].Priority)
	}
	if original.Findings[0].Suggestions[0].Body != "suggestion" {
		t.Fatalf("original suggestion mutated = %q", original.Findings[0].Suggestions[0].Body)
	}
	if original.Findings[0].Verification.Verdict != model.VerdictConfirmed {
		t.Fatalf("original verification mutated = %q", original.Findings[0].Verification.Verdict)
	}
	if original.Findings[0].Finalization.Title != "final" {
		t.Fatalf("original finalization mutated = %q", original.Findings[0].Finalization.Title)
	}
}

// Absorbing an exact duplicate without touching the surviving finding's text
// is a valid merge. The old pairwise validator rejected this shape and
// deadlocked into retry exhaustion (the v3 duplicate-output bug).
func TestClusterMergeValidationAcceptsAbsorbedDuplicateWithoutTextChange(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	b := mergeTestFindingWithID("Fix A", 1)
	cluster := []model.Finding{a, b}
	resp := &llm.ReviewResponse{Findings: []model.Finding{a}}

	if invalid := validateClusterMergeResponse(resp, cluster); invalid != nil {
		t.Fatalf("validateClusterMergeResponse = %v, want nil for absorbed duplicate", invalid)
	}
}

func TestClusterMergeValidationRejectsEmptyAndOversizedOutput(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	b := mergeTestFindingWithID("Fix B", 13)
	cluster := []model.Finding{a, b}

	invalid := validateClusterMergeResponse(&llm.ReviewResponse{}, cluster)
	if invalid == nil || !strings.Contains(invalid.Reason, "count_mismatch") {
		t.Fatalf("empty output invalid = %v, want count_mismatch", invalid)
	}

	invalid = validateClusterMergeResponse(&llm.ReviewResponse{Findings: []model.Finding{a, b, a}}, cluster)
	if invalid == nil || !strings.Contains(invalid.Reason, "count_mismatch") {
		t.Fatalf("oversized output invalid = %v, want count_mismatch", invalid)
	}
}

func TestClusterMergeValidationRejectsUnmatchedFinding(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	b := mergeTestFindingWithID("Fix B", 13)
	ghost := mergeTestFindingWithID("Ghost", 99)
	ghost.CodeLocation.FilePath = "ghost.go"

	invalid := validateClusterMergeResponse(&llm.ReviewResponse{Findings: []model.Finding{a, ghost}}, []model.Finding{a, b})
	if invalid == nil || !strings.Contains(invalid.Reason, "unmatched_finding") {
		t.Fatalf("ghost output invalid = %v, want unmatched_finding", invalid)
	}
}

func TestMergeRetryGuidanceListsAllowedUnknownAndDroppedIDs(t *testing.T) {
	const (
		id1       = "11111111-1111-4111-8111-111111111111"
		id2       = "22222222-2222-4222-8222-222222222222"
		unknownID = "af5fc1a4-fd98-40a3-95ad-ba44f9852efd"
	)
	a := mergeTestFinding("Fix A", 1)
	a.ID = id1
	a.Verification.ID = id1
	b := mergeTestFinding("Fix B", 13)
	b.ID = id2
	b.Verification.ID = id2
	ghost := mergeTestFinding("Ghost", 99)
	ghost.ID = unknownID
	ghost.Verification.ID = unknownID
	ghost.CodeLocation.FilePath = "ghost.go"
	blank := mergeTestFinding("Blank ghost", 100)
	blank.ID = ""
	blank.Verification.ID = ""
	blank.CodeLocation.FilePath = "blank.go"

	invalid := validateClusterMergeResponse(
		&llm.ReviewResponse{Findings: []model.Finding{ghost, blank}},
		[]model.Finding{a, b},
	)
	if invalid == nil {
		t.Fatal("want invalid response")
	}
	rendered, err := renderPromptFile(invalid.RetryGuidanceTemplate, invalid.RetryGuidanceData)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Allowed cluster IDs, in order:",
		"`" + id1 + "`",
		"`" + id2 + "`",
		"Output IDs not in the cluster:",
		"`" + unknownID + "`",
		"<empty id>",
		"Dropped input IDs:",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("retry guidance missing %q:\n%s", want, rendered)
		}
	}
}

func TestClusterMergeValidationAllowsIDMatchWithRefinedLocation(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	b := mergeTestFindingWithID("Fix B", 13)
	merged := a
	merged.CodeLocation.LineRange = model.LineRange{Start: 1, End: 13}
	merged.MergedFrom = []string{b.ID}

	if invalid := validateClusterMergeResponse(&llm.ReviewResponse{Findings: []model.Finding{merged}}, []model.Finding{a, b}); invalid != nil {
		t.Fatalf("validateClusterMergeResponse = %v, want nil for ID match with extended location", invalid)
	}
}

// The P1 regression scenario: a response returning one unchanged input while
// silently losing the other distinct cluster members must be rejected.
func TestClusterMergeValidationRejectsSilentlyDroppedFindings(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	b := mergeTestFindingWithID("Fix B", 13)
	c := mergeTestFindingWithID("Fix C", 25)

	invalid := validateClusterMergeResponse(&llm.ReviewResponse{Findings: []model.Finding{a}}, []model.Finding{a, b, c})
	if invalid == nil || !strings.Contains(invalid.Reason, "dropped_findings count=2") {
		t.Fatalf("invalid = %v, want dropped_findings count=2", invalid)
	}

	// Declaring the absorbed findings in merged_from makes the same shape valid.
	absorbing := a
	absorbing.MergedFrom = []string{b.ID, c.ID}
	if invalid := validateClusterMergeResponse(&llm.ReviewResponse{Findings: []model.Finding{absorbing}}, []model.Finding{a, b, c}); invalid != nil {
		t.Fatalf("validateClusterMergeResponse = %v, want nil with full merged_from coverage", invalid)
	}
}

func TestClusterMergeRepairMissingMergedFrom(t *testing.T) {
	a := mergeTestFindingWithID("Cleanup leaves nested rotated assets", 1)
	a.Body = "Cleanup leaves nested rotated assets behind after log rotation."
	b := mergeTestFindingWithID("Nested rotated assets remain after cleanup", 22)
	b.Body = "Nested rotated assets remain behind because cleanup misses rotated logs."
	merged := a
	merged.Title = "Nested rotated assets remain after cleanup"
	merged.Body = "Cleanup leaves nested rotated assets behind after log rotation and misses rotated logs."
	merged.CodeLocation.LineRange = model.LineRange{Start: 1, End: 22}
	resp := &llm.ReviewResponse{Findings: []model.Finding{merged}}
	cluster := []model.Finding{a, b}

	if invalid := validateClusterMergeResponse(resp, cluster); invalid == nil {
		t.Fatal("validateClusterMergeResponse accepted response before provenance repair")
	}
	if repaired := repairClusterMergeProvenance(resp, cluster); repaired != 1 {
		t.Fatalf("repairClusterMergeProvenance = %d, want 1", repaired)
	}
	if !slices.Contains(resp.Findings[0].MergedFrom, b.ID) {
		t.Fatalf("merged_from = %#v, want %q", resp.Findings[0].MergedFrom, b.ID)
	}
	if invalid := validateClusterMergeResponse(resp, cluster); invalid != nil {
		t.Fatalf("validateClusterMergeResponse = %v, want nil after provenance repair", invalid)
	}
}

func TestClusterMergeRepairMissingMergedFromForRootCauseMatch(t *testing.T) {
	a := mergeTestFindingWithID("Unquoted path expansion breaks target-dir restore", 10)
	a.Body = "The target-dir template leaves `$TMP` unquoted in `find`; `mkdir` and `chown` also receive unquoted paths. Paths containing spaces split into separate arguments and the restore fails."
	a.CodeLocation = model.CodeLocation{FilePath: "pkg/restore/restore-directory-target-dir.tpl", LineRange: model.LineRange{Start: 10, End: 18}}
	b := mergeTestFindingWithID("Directory template mishandles paths containing spaces", 12)
	b.Body = "The directory template passes `$TMP` to `find` without quotes, then calls `mkdir` and `chown` with unquoted destination paths. Whitespace in a path causes word splitting and command failure."
	b.CodeLocation = model.CodeLocation{FilePath: "pkg/restore/restore-directory.tpl", LineRange: model.LineRange{Start: 12, End: 20}}
	merged := a
	merged.Title = "Unquoted restore paths break destinations containing spaces"
	merged.Body = "Related restore templates pass `$TMP` to `find` and call `mkdir` and `chown` with unquoted paths, so destinations containing spaces are split into separate arguments."
	resp := &llm.ReviewResponse{Findings: []model.Finding{merged}}
	cluster := []model.Finding{a, b}

	if invalid := validateClusterMergeResponse(resp, cluster); invalid == nil {
		t.Fatal("validateClusterMergeResponse accepted response before provenance repair")
	}
	if repaired := repairClusterMergeProvenance(resp, cluster); repaired != 1 {
		t.Fatalf("repairClusterMergeProvenance = %d, want 1", repaired)
	}
	if !slices.Contains(resp.Findings[0].MergedFrom, b.ID) {
		t.Fatalf("merged_from = %#v, want %q", resp.Findings[0].MergedFrom, b.ID)
	}
	if invalid := validateClusterMergeResponse(resp, cluster); invalid != nil {
		t.Fatalf("validateClusterMergeResponse = %v, want nil after provenance repair", invalid)
	}
}

func TestClusterMergeRepairMissingMergedFromSkipsAmbiguousAbsorber(t *testing.T) {
	a := mergeTestFindingWithID("Shared cleanup leak in temp files", 1)
	a.Body = "Shared cleanup leak leaves rotated files behind."
	b := mergeTestFindingWithID("Shared cleanup leak in asset files", 13)
	b.Body = "Shared cleanup leak leaves rotated files behind."
	c := mergeTestFindingWithID("Shared cleanup leak", 25)
	c.Body = "Shared cleanup leak leaves rotated files behind."
	resp := &llm.ReviewResponse{Findings: []model.Finding{a, b}}
	cluster := []model.Finding{a, b, c}

	if repaired := repairClusterMergeProvenance(resp, cluster); repaired != 0 {
		t.Fatalf("repairClusterMergeProvenance = %d, want 0 for ambiguous absorber", repaired)
	}
	invalid := validateClusterMergeResponse(resp, cluster)
	if invalid == nil || !strings.Contains(invalid.Reason, "dropped_findings count=1") {
		t.Fatalf("invalid = %v, want dropped_findings count=1", invalid)
	}
}

func TestClusterMergeRepairMissingMergedFromSkipsUnrelatedDrop(t *testing.T) {
	a := mergeTestFindingWithID("Cleanup leaves nested rotated assets", 1)
	a.Body = "Cleanup leaves nested rotated assets behind after log rotation."
	b := mergeTestFindingWithID("Secret token is written to logs", 80)
	b.Body = "Debug logging writes a secret token to application logs."
	resp := &llm.ReviewResponse{Findings: []model.Finding{a}}
	cluster := []model.Finding{a, b}

	if repaired := repairClusterMergeProvenance(resp, cluster); repaired != 0 {
		t.Fatalf("repairClusterMergeProvenance = %d, want 0 for unrelated drop", repaired)
	}
	invalid := validateClusterMergeResponse(resp, cluster)
	if invalid == nil || !strings.Contains(invalid.Reason, "dropped_findings count=1") {
		t.Fatalf("invalid = %v, want dropped_findings count=1", invalid)
	}
}

// merged_from accounting is lenient: trimmed entries, duplicates, the
// finding's own id, and unknown ids inside merged_from are ignored — they
// cannot fake coverage, so only genuinely missing inputs fail. Output finding
// ids themselves are strict: a reminted id fails as unknown even when the
// content still matches an input.
func TestClusterMergeValidationMergedFromLeniency(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	b := mergeTestFindingWithID("Fix B", 13)

	absorbing := a
	absorbing.MergedFrom = []string{"  " + b.ID + "  ", b.ID, a.ID, "not-a-real-id", ""}
	if invalid := validateClusterMergeResponse(&llm.ReviewResponse{Findings: []model.Finding{absorbing}}, []model.Finding{a, b}); invalid != nil {
		t.Fatalf("validateClusterMergeResponse = %v, want nil despite messy merged_from", invalid)
	}

	// Garbage merged_from must not fake coverage of a real, missing input.
	absorbing.MergedFrom = []string{"not-a-real-id"}
	invalid := validateClusterMergeResponse(&llm.ReviewResponse{Findings: []model.Finding{absorbing}}, []model.Finding{a, b})
	if invalid == nil || !strings.Contains(invalid.Reason, "dropped_findings count=1") {
		t.Fatalf("invalid = %v, want dropped_findings count=1", invalid)
	}

	// Reminted output id, same location and title: the input is still
	// accounted via attribution (no dropped_findings), but the unknown output
	// id itself now fails validation.
	reminted := mergeTestFindingWithID("Fix A", 1)
	invalid = validateClusterMergeResponse(&llm.ReviewResponse{Findings: []model.Finding{reminted}}, []model.Finding{a})
	if invalid == nil || !strings.Contains(invalid.Reason, "unknown_ids count=1") {
		t.Fatalf("invalid = %v, want unknown_ids count=1 for reminted output id", invalid)
	}
	if strings.Contains(invalid.Reason, "dropped_findings") {
		t.Fatalf("invalid = %v, attribution should still cover the input (no dropped_findings)", invalid)
	}
}

// Provenance is internal to the merge step: accepted responses leave it
// stripped so it never reaches results or posted reviews.
func TestClusterMergeStripsMergedFromOnAccept(t *testing.T) {
	a := clusterTestFinding("Fix alpha issue", 1)
	b := clusterTestFinding("Fix beta issue", 13)
	absorbing := a
	absorbing.MergedFrom = []string{b.ID}
	engine := NewEngine(stubSource{}, &multiAgentLLM{
		mergeResponses: []*llm.ReviewResponse{{
			Findings:               []model.Finding{absorbing},
			OverallCorrectness:     "patch is incorrect",
			OverallExplanation:     "merged",
			OverallConfidenceScore: 0.9,
		}},
	}, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "Reviewer A", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{a}, OverallConfidenceScore: 0.9}},
		{name: "Reviewer B", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{b}, OverallConfidenceScore: 0.9}},
	}

	result, runs := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(runs) != 1 || runs[0].Status != model.AgentRunStatusOK {
		t.Fatalf("runs = %#v, want one ok micro-merge", runs)
	}
	if len(result.resp.Findings) != 1 {
		t.Fatalf("findings = %d, want merged single finding", len(result.resp.Findings))
	}
	if result.resp.Findings[0].MergedFrom != nil {
		t.Fatalf("merged_from = %#v, want stripped from accepted output", result.resp.Findings[0].MergedFrom)
	}
}

func TestClusterMergeRepairsMissingMergedFromWithoutRetry(t *testing.T) {
	a := clusterTestFinding("Log cleanup skips rotated tmp files", 1)
	a.Body = "Cleanup leaves rotated tmp files behind after log rotation."
	b := clusterTestFinding("Log cleanup skips rotated tmp files in nested dirs", 30)
	b.Body = "Cleanup leaves rotated tmp files in nested directories after log rotation."
	merged := a
	merged.Title = b.Title
	merged.Body = "Cleanup leaves rotated tmp files in nested directories behind after log rotation."
	merged.CodeLocation.LineRange = model.LineRange{Start: 1, End: 30}
	llmClient := &multiAgentLLM{
		mergeResponses: []*llm.ReviewResponse{{
			Findings:               []model.Finding{merged},
			OverallCorrectness:     "patch is incorrect",
			OverallExplanation:     "merged",
			OverallConfidenceScore: 0.9,
		}},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "Reviewer A", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{a}, OverallConfidenceScore: 0.9}},
		{name: "Reviewer B", role: "review", response: &llm.ReviewResponse{Findings: []model.Finding{b}, OverallConfidenceScore: 0.9}},
	}

	result, runs := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{MaxOutputRetries: 1})

	if len(llmClient.mergeRequests) != 1 {
		t.Fatalf("merge requests = %d, want one request with provenance repair and no retry", len(llmClient.mergeRequests))
	}
	if len(runs) != 1 || runs[0].Status != model.AgentRunStatusOK {
		t.Fatalf("runs = %#v, want one ok micro-merge", runs)
	}
	if len(result.resp.Findings) != 1 {
		t.Fatalf("findings = %d, want repaired merged single finding", len(result.resp.Findings))
	}
	if result.resp.Findings[0].MergedFrom != nil {
		t.Fatalf("merged_from = %#v, want stripped from accepted output", result.resp.Findings[0].MergedFrom)
	}
}

func TestMechanicallyDedupeFindingsFoldsDuplicateClusters(t *testing.T) {
	a := mergeTestFindingWithID("Fix duplicated cleanup issue", 5)
	b := mergeTestFindingWithID("Fix duplicated cleanup issue", 5)
	distinct := mergeTestFindingWithID("Improve unrelated subsystem", 5)
	distinct.CodeLocation.FilePath = "other.go"

	reduced, absorbed := mechanicallyDedupeFindings([]model.Finding{a, b, distinct})

	if absorbed != 1 || len(reduced) != 2 {
		t.Fatalf("reduced = %d absorbed = %d, want 2/1", len(reduced), absorbed)
	}
	if reduced[1].Title != distinct.Title {
		t.Fatalf("distinct finding lost: %#v", reduced)
	}

	untouched, absorbed := mechanicallyDedupeFindings([]model.Finding{a, distinct})
	if absorbed != 0 || len(untouched) != 2 {
		t.Fatalf("no-duplicate input changed: %d/%d", len(untouched), absorbed)
	}
}

func TestMechanicallyDedupeFindingsRoutesSuggestionChoiceToLLM(t *testing.T) {
	a := mergeTestFindingWithID("Fix duplicated cleanup issue", 5)
	a.Suggestions = []model.Suggestion{{Body: "first candidate"}}
	b := mergeTestFindingWithID("Fix duplicated cleanup issue", 5)
	b.Suggestions = []model.Suggestion{{Body: "better candidate"}}

	reduced, absorbed := mechanicallyDedupeFindings([]model.Finding{a, b})

	if absorbed != 0 || len(reduced) != 2 {
		t.Fatalf("reduced = %d absorbed = %d, want unchanged 2/0 for agent selection", len(reduced), absorbed)
	}
}

func TestRunDedupeAgentsSelectsOneSuggestionWithLLM(t *testing.T) {
	a := mergeTestFindingWithID("Fix duplicated cleanup issue", 5)
	a.Suggestions = []model.Suggestion{{Body: "first candidate"}}
	b := mergeTestFindingWithID("Fix duplicated cleanup issue", 5)
	b.Suggestions = []model.Suggestion{{Body: "better candidate"}}
	selected := a
	selected.Suggestions = []model.Suggestion{b.Suggestions[0]}
	llmClient := &multiAgentLLM{dedupeResponses: []*llm.ReviewResponse{{Findings: []model.Finding{selected}}}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: []model.Finding{a, b}},
		run:  model.AgentRun{Name: "Testing", Role: "review"},
	}}

	runs := engine.runDedupeAgents(context.Background(), "", vectorResults, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(llmClient.mergeRequests) != 1 || len(runs) != 1 {
		t.Fatalf("requests/runs = %d/%d, want one dedupe agent", len(llmClient.mergeRequests), len(runs))
	}
	got := vectorResults[0].resp.Findings
	if len(got) != 1 || len(got[0].Suggestions) != 1 || got[0].Suggestions[0].Body != "better candidate" {
		t.Fatalf("dedupe findings = %+v, want LLM-selected suggestion", got)
	}
}

func TestClusterMergeSelectsOneSuggestionWithLLM(t *testing.T) {
	a := mergeTestFindingWithID("Fix duplicated cleanup issue", 5)
	a.Suggestions = []model.Suggestion{{Body: "first candidate"}}
	b := mergeTestFindingWithID("Fix duplicated cleanup issue", 5)
	b.Suggestions = []model.Suggestion{{Body: "better candidate"}}
	selected := a
	selected.Suggestions = []model.Suggestion{b.Suggestions[0]}
	selected.MergedFrom = []string{b.ID}
	llmClient := &multiAgentLLM{mergeResponses: []*llm.ReviewResponse{{Findings: []model.Finding{selected}}}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	inputs := []pairwiseMergeInput{
		{name: "A", response: &llm.ReviewResponse{Findings: []model.Finding{a}}},
		{name: "B", response: &llm.ReviewResponse{Findings: []model.Finding{b}}},
	}

	result, runs := engine.runClusterMergeAgents(context.Background(), "{}", "", inputs, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(llmClient.mergeRequests) != 1 || len(runs) != 1 || runs[0].Status != model.AgentRunStatusOK {
		t.Fatalf("requests/runs = %d/%+v, want one successful merge agent", len(llmClient.mergeRequests), runs)
	}
	got := result.resp.Findings
	if len(got) != 1 || len(got[0].Suggestions) != 1 || got[0].Suggestions[0].Body != "better candidate" {
		t.Fatalf("merge findings = %+v, want LLM-selected suggestion", got)
	}
}

// The mechanical pre-pass folds clear duplicates before the LLM dedupe agent
// runs; a lane reduced to one finding skips the LLM dedupe entirely.
func TestRunDedupeAgentsMechanicalPrePassSkipsLLMWhenReduced(t *testing.T) {
	a := mergeTestFindingWithID("Fix duplicated cleanup issue", 5)
	b := mergeTestFindingWithID("Fix duplicated cleanup issue", 5)
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: []model.Finding{a, b}},
		run:  model.AgentRun{Name: "Testing", Role: "review"},
	}}

	runs := engine.runDedupeAgents(context.Background(), "", vectorResults, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if len(llmClient.mergeRequests) != 0 {
		t.Fatalf("LLM dedupe requests = %d, want 0 after mechanical fold to one", len(llmClient.mergeRequests))
	}
	if len(runs) != 0 {
		t.Fatalf("dedupe runs = %#v, want none", runs)
	}
	if got := vectorResults[0].resp.Findings; len(got) != 1 {
		t.Fatalf("lane findings = %d, want 1 after mechanical fold", len(got))
	}
}

func TestDedupeAgentAcceptsDedupedReviewerFindings(t *testing.T) {
	a := mergeTestFindingWithID("Fix duplicated issue", 1)
	b := mergeTestFindingWithID("Fix duplicated issue", 1)
	llmClient := &multiAgentLLM{
		dedupeResponses: []*llm.ReviewResponse{{
			Findings:               []model.Finding{a},
			OverallCorrectness:     "patch is incorrect",
			OverallExplanation:     "deduped",
			OverallConfidenceScore: 0.9,
		}},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	input := agentResult{
		resp: &llm.ReviewResponse{Findings: []model.Finding{a, b}},
		run:  model.AgentRun{Name: "Testing", Role: "review"},
	}

	resp, run := engine.runDedupeAgent(context.Background(), "", input, nil, llm.ResponseConstraints{}, model.ReviewRequest{})

	if run.Status != model.AgentRunStatusOK {
		t.Fatalf("dedupe status = %q, want ok: %s", run.Status, run.Error)
	}
	if resp == nil || len(resp.Findings) != 1 || resp.Findings[0].ID != a.ID {
		t.Fatalf("dedupe findings = %#v, want one preserved input finding", resp)
	}
}

func TestDedupeAgentRejectsUnknownIDsAndFallsBack(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	b := mergeTestFindingWithID("Fix B", 2)
	unknown := mergeTestFindingWithID("Fix A", 1)
	llmClient := &multiAgentLLM{
		dedupeResponses: []*llm.ReviewResponse{
			{Findings: []model.Finding{unknown}},
			{Findings: []model.Finding{unknown}},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	input := agentResult{
		resp: &llm.ReviewResponse{Findings: []model.Finding{a, b}},
		run:  model.AgentRun{Name: "Testing", Role: "review"},
	}

	resp, run := engine.runDedupeAgent(context.Background(), "", input, nil, llm.ResponseConstraints{}, model.ReviewRequest{MaxOutputRetries: 1})

	if resp != nil {
		t.Fatalf("dedupe resp = %#v, want fallback/no replacement", resp)
	}
	if run.Status != model.AgentRunStatusPartial {
		t.Fatalf("dedupe status = %q, want partial", run.Status)
	}
	if !strings.Contains(run.Error, "unknown_ids") {
		t.Fatalf("dedupe error = %q, want unknown_ids", run.Error)
	}
}

func TestDedupeRetryGuidanceListsAllowedAndUnknownIDs(t *testing.T) {
	const (
		id1       = "11111111-1111-4111-8111-111111111111"
		id2       = "22222222-2222-4222-8222-222222222222"
		unknownID = "af5fc1a4-fd98-40a3-95ad-ba44f9852efd"
	)
	a := mergeTestFinding("Fix A", 1)
	a.ID = id1
	a.Verification.ID = id1
	b := mergeTestFinding("Fix B", 2)
	b.ID = id2
	b.Verification.ID = id2
	unknown := mergeTestFinding("Fix A", 1)
	unknown.ID = unknownID
	unknown.Verification.ID = unknownID
	blank := mergeTestFinding("Fix B", 2)
	blank.ID = ""
	blank.Verification.ID = ""

	invalid := validateDedupeResponse(
		&llm.ReviewResponse{Findings: []model.Finding{unknown, blank}},
		&llm.ReviewResponse{Findings: []model.Finding{a, b}},
	)
	if invalid == nil {
		t.Fatal("want invalid response")
	}
	rendered, err := renderPromptFile(invalid.RetryGuidanceTemplate, invalid.RetryGuidanceData)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Allowed input IDs, in order:",
		"`" + id1 + "`",
		"`" + id2 + "`",
		"Unknown output IDs:",
		"`" + unknownID + "`",
		"<empty id>",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("retry guidance missing %q:\n%s", want, rendered)
		}
	}
}

func TestDedupeValidationRejectsTooMuchDrop(t *testing.T) {
	var input []model.Finding
	for i := range 10 {
		input = append(input, mergeTestFindingWithID(fmt.Sprintf("Fix %d", i), i+1))
	}
	resp := &llm.ReviewResponse{Findings: append([]model.Finding(nil), input[:4]...)}

	invalid := validateDedupeResponse(resp, &llm.ReviewResponse{Findings: input})

	if invalid == nil {
		t.Fatal("expected count_too_low, got nil")
	}
	if !strings.Contains(invalid.Reason, "count_too_low") {
		t.Fatalf("dedupe reason = %q, want count_too_low", invalid.Reason)
	}
}

func TestDedupeValidationAllowsSmallListDropToOne(t *testing.T) {
	input := []model.Finding{
		mergeTestFindingWithID("Fix A", 1),
		mergeTestFindingWithID("Fix A duplicate", 1),
		mergeTestFindingWithID("Fix A duplicate again", 1),
	}
	resp := &llm.ReviewResponse{Findings: []model.Finding{input[0]}}

	if invalid := validateDedupeResponse(resp, &llm.ReviewResponse{Findings: input}); invalid != nil {
		t.Fatalf("validateDedupeResponse returned %v, want nil", invalid)
	}
}

func TestDedupeValidationRejectsDuplicateOutputIDs(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	resp := &llm.ReviewResponse{Findings: []model.Finding{a, a}}

	invalid := validateDedupeResponse(resp, &llm.ReviewResponse{Findings: []model.Finding{a}})

	if invalid == nil {
		t.Fatal("expected duplicate_ids, got nil")
	}
	if !strings.Contains(invalid.Reason, "duplicate_ids") {
		t.Fatalf("dedupe reason = %q, want duplicate_ids", invalid.Reason)
	}
}

func TestDedupeValidationRejectsMultipleSuggestions(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	out := a
	out.Suggestions = []model.Suggestion{{Body: "first"}, {Body: "second"}}

	invalid := validateDedupeResponse(&llm.ReviewResponse{Findings: []model.Finding{out}}, &llm.ReviewResponse{Findings: []model.Finding{a}})

	if invalid == nil || !strings.Contains(invalid.Reason, "too_many_suggestions") {
		t.Fatalf("invalid = %v, want too_many_suggestions", invalid)
	}
	rendered, err := renderPromptFile(invalid.RetryGuidanceTemplate, invalid.RetryGuidanceData)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "at most one suggestion") {
		t.Fatalf("retry guidance missing suggestion limit:\n%s", rendered)
	}
}

func TestClusterMergeValidationRejectsMultipleSuggestions(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	out := a
	out.Suggestions = []model.Suggestion{{Body: "first"}, {Body: "second"}}

	invalid := validateClusterMergeResponse(&llm.ReviewResponse{Findings: []model.Finding{out}}, []model.Finding{a})

	if invalid == nil || !strings.Contains(invalid.Reason, "too_many_suggestions") {
		t.Fatalf("invalid = %v, want too_many_suggestions", invalid)
	}
	rendered, err := renderPromptFile(invalid.RetryGuidanceTemplate, invalid.RetryGuidanceData)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "at most one suggestion") {
		t.Fatalf("retry guidance missing suggestion limit:\n%s", rendered)
	}
}

// TestDedupeValidationSkipsVerificationEchoForUnverifiedInputs covers custom
// specs that dedupe BEFORE verification (or over raw findings_from JSON): the
// inputs carry no verification, so the response cannot echo one, and demanding
// it made validation impossible and burned every retry.
func TestDedupeValidationSkipsVerificationEchoForUnverifiedInputs(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	a.Verification = nil
	b := mergeTestFindingWithID("Fix B", 2)
	b.Verification = nil
	input := &llm.ReviewResponse{Findings: []model.Finding{a, b}}

	if invalid := validateDedupeResponse(&llm.ReviewResponse{Findings: []model.Finding{a, b}}, input); invalid != nil {
		t.Fatalf("validateDedupeResponse = %v, want nil for unverified inputs echoed without verification", invalid)
	}
}

func TestDedupeValidationStillRequiresVerificationEchoForVerifiedInputs(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	input := &llm.ReviewResponse{Findings: []model.Finding{a}}
	out := a
	out.Verification = nil

	invalid := validateDedupeResponse(&llm.ReviewResponse{Findings: []model.Finding{out}}, input)
	if invalid == nil || !strings.Contains(invalid.Reason, "verification_mismatch") {
		t.Fatalf("invalid = %v, want verification_mismatch for verified input echoed without verification", invalid)
	}
}

// TestFlattenMergeMembersNormalizesCrossReviewerDuplicateIDs pins the fix for
// cross-reviewer ID collisions: EnsureFindingIDs only dedupes within a single
// response, so two reviewers can emit the same ID. The flattened merge input
// must remint collisions (keeping Verification.ID in sync) without mutating
// the reviewers' original responses.
func TestFlattenMergeMembersNormalizesCrossReviewerDuplicateIDs(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	b := mergeTestFinding("Fix B", 13)
	b.ID = a.ID
	b.Verification.ID = a.ID
	inputs := []pairwiseMergeInput{
		{name: "Code Quality", response: &llm.ReviewResponse{Findings: []model.Finding{a}}},
		{name: "Security", response: &llm.ReviewResponse{Findings: []model.Finding{b}}},
	}

	findings, reviewerByID := flattenMergeMembers(inputs)

	if len(findings) != 2 {
		t.Fatalf("flattened findings = %d, want 2", len(findings))
	}
	if findings[0].ID == findings[1].ID {
		t.Fatalf("duplicate IDs survived flattening: %q", findings[0].ID)
	}
	for i := range findings {
		if findings[i].Verification == nil || findings[i].Verification.ID != findings[i].ID {
			t.Fatalf("finding %d verification ID = %#v, want synced with %q", i, findings[i].Verification, findings[i].ID)
		}
	}
	if len(reviewerByID) != 2 {
		t.Fatalf("reviewerByID = %#v, want 2 collision-free entries", reviewerByID)
	}
	if reviewerByID[findings[0].ID] != "Code Quality" || reviewerByID[findings[1].ID] != "Security" {
		t.Fatalf("reviewerByID = %#v, want per-reviewer attribution preserved", reviewerByID)
	}
	// The reviewers' original responses must not be mutated through shared
	// verification pointers when a collision is reminted.
	original := inputs[1].response.Findings[0]
	if original.ID != a.ID || original.Verification.ID != a.ID {
		t.Fatalf("original response mutated: id=%q verification_id=%q, want %q", original.ID, original.Verification.ID, a.ID)
	}
}

// TestClusterMergeValidationRejectsRemintedOutputIDs pins that unknown output
// IDs now fail validation (they were previously computed for retry guidance
// but never failed the response).
func TestClusterMergeValidationRejectsRemintedOutputIDs(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	b := mergeTestFindingWithID("Fix B", 13)
	reminted := a
	reminted.ID = uuid.NewString()
	// Cover both inputs via merged_from so unknown_ids is the only failure.
	reminted.MergedFrom = []string{a.ID, b.ID}

	invalid := validateClusterMergeResponse(&llm.ReviewResponse{Findings: []model.Finding{reminted}}, []model.Finding{a, b})
	if invalid == nil || !strings.Contains(invalid.Reason, "unknown_ids count=1") {
		t.Fatalf("invalid = %v, want unknown_ids failure for reminted output ID", invalid)
	}
}

func TestClusterMergeValidationRejectsDuplicateOutputIDs(t *testing.T) {
	a := mergeTestFindingWithID("Fix A", 1)
	b := mergeTestFindingWithID("Fix B", 13)
	dupe := a
	dupe.Body = "same finding repeated under the same id"

	invalid := validateClusterMergeResponse(&llm.ReviewResponse{Findings: []model.Finding{a, dupe}}, []model.Finding{a, b})
	if invalid == nil || !strings.Contains(invalid.Reason, "duplicate_ids count=1") {
		t.Fatalf("invalid = %v, want duplicate_ids failure for repeated output ID", invalid)
	}
}

func TestNoToolsMessagesUsesAgentRoleForCommonSnippets(t *testing.T) {
	messages := []llm.Message{{Role: "system", Content: "old"}}
	reviewerMessages, err := noToolsMessages("review", "{{.FindingInstructionsSnippet}}", messages, "", "", false)
	if err != nil {
		t.Fatalf("reviewer noToolsMessages returned err: %v", err)
	}
	verifyMessages, err := noToolsMessages("verify", "{{.FindingInstructionsSnippet}}", messages, "", "", false)
	if err != nil {
		t.Fatalf("verify noToolsMessages returned err: %v", err)
	}
	if !strings.Contains(reviewerMessages[0].Content, "suggestions") {
		t.Fatalf("reviewer no-tools prompt missing reviewer snippet: %q", reviewerMessages[0].Content)
	}
	if strings.Contains(verifyMessages[0].Content, "suggestions") {
		t.Fatalf("verify no-tools prompt used reviewer snippet: %q", verifyMessages[0].Content)
	}
}

func TestNoToolsMessagesCanDisableSuggestions(t *testing.T) {
	messages := []llm.Message{{Role: "system", Content: "old"}}
	reviewerMessages, err := noToolsMessages("review", "{{.FindingInstructionsSnippet}}", messages, "", "", true)
	if err != nil {
		t.Fatalf("reviewer noToolsMessages returned err: %v", err)
	}
	system := reviewerMessages[0].Content
	if !strings.Contains(system, "do not output `suggestions`") {
		t.Fatalf("reviewer no-tools prompt missing skip instruction: %q", system)
	}
	if strings.Contains(system, "generate one or more `suggestions`") {
		t.Fatalf("reviewer no-tools prompt still asks for suggestions: %q", system)
	}
	if strings.Contains(system, "only in `suggestions[].body`") {
		t.Fatalf("reviewer no-tools prompt still routes patch code into suggestions: %q", system)
	}
	if strings.Contains(system, "DO NOT include the following in `body`") {
		t.Fatalf("reviewer no-tools prompt still includes suggestions-only body guard: %q", system)
	}
}

func TestStripResultSuggestions(t *testing.T) {
	result := &model.ReviewResult{Findings: []model.Finding{{
		Title:       "t",
		Suggestions: []model.Suggestion{{Body: "fix"}},
	}}}

	result.StripSuggestions()

	if len(result.Findings[0].Suggestions) != 0 {
		t.Fatalf("suggestions = %+v, want stripped", result.Findings[0].Suggestions)
	}
}

func mergeTestFinding(title string, line int) model.Finding {
	return model.Finding{
		Title:           title,
		Body:            "body",
		ConfidenceScore: 0.9,
		Priority:        intPtr(2),
		CodeLocation:    model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: line, End: line}},
		Verification:    &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.9, Remarks: "confirmed"},
	}
}

func mergeTestFindingWithID(title string, line int) model.Finding {
	f := mergeTestFinding(title, line)
	f.ID = uuid.NewString()
	f.Verification.ID = f.ID
	return f
}

func TestEngineVectorNudgeRepeatsReviewerQuestions(t *testing.T) {
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
		NudgeCount:       1,
	})
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"Code Quality": "- Does it work correctly?",
		"Security":     "- Are there security concerns?",
		"Testing":      "- Are changed behaviors covered by focused tests?",
	}
	for name, want := range checks {
		nudge := llmClient.vectorNudge[name]
		if !strings.Contains(nudge, "Ask yourself these original questions again:") {
			t.Fatalf("%s nudge missing question header: %q", name, nudge)
		}
		if !strings.Contains(nudge, want) {
			t.Fatalf("%s nudge missing question %q: %q", name, want, nudge)
		}
	}
}

func TestMultiAgentToleratesVectorFailure(t *testing.T) {
	llmClient := &multiAgentLLM{
		vectorFailErr: map[string]error{"Security": errors.New("security upstream fail")},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatalf("RunWithContext returned err: %v", err)
	}
	var securityRun *model.AgentRun
	successfulReviewers := 0
	for i := range result.AgentRuns {
		run := &result.AgentRuns[i]
		if run.Name == "Security" {
			securityRun = run
			continue
		}
		if run.Role == "review" && run.Status == model.AgentRunStatusOK {
			successfulReviewers++
		}
	}
	if securityRun == nil {
		t.Fatal("missing Security AgentRun")
	}
	if securityRun.Status != model.AgentRunStatusFailed {
		t.Fatalf("Security status = %q, want failed", securityRun.Status)
	}
	if !strings.Contains(securityRun.Error, "security upstream fail") {
		t.Fatalf("Security run error = %q", securityRun.Error)
	}
	expectedSuccessfulReviewers := len(reviewVectors) - 1
	if successfulReviewers != expectedSuccessfulReviewers {
		t.Fatalf("successful reviewer runs = %d, want %d", successfulReviewers, expectedSuccessfulReviewers)
	}
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "Security reviewer failed") && strings.Contains(w, "security upstream fail") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("expected Security warning in %#v", result.Warnings)
	}
}

func TestMultiAgentToleratesContextFailure(t *testing.T) {
	llmClient := &multiAgentLLM{contextFailErr: errors.New("context upstream fail")}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatalf("RunWithContext returned err: %v", err)
	}
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "Context agent failed") && strings.Contains(w, "context upstream fail") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("expected context warning in %#v", result.Warnings)
	}
	for _, name := range []string{"Code Quality", "Security", "Architecture", "Performance", "Testing", "Best Practices"} {
		if llmClient.vectorContext[name] != "" {
			t.Fatalf("%s reviewer received non-empty context notes: %q", name, llmClient.vectorContext[name])
		}
	}
}

func TestEnsurePromptsRejectsNilEnrichedContext(t *testing.T) {
	engine := NewEngine(stubSource{}, stubLLM{}, stubRetrieval{}, config.Profile{Model: "test"})

	err := engine.ensurePrompts(&PipelineState{})
	if err == nil || !strings.Contains(err.Error(), "nil enriched context") {
		t.Fatalf("ensurePrompts error = %v, want nil enriched context", err)
	}
}

func TestCollectStepRejectsNilBaseContext(t *testing.T) {
	engine := NewEngine(stubSource{}, stubLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	step := engine.collectStepFunc()

	err := step(context.Background(), &stepContext{Engine: engine, Req: model.ReviewRequest{}}, &PipelineState{})
	if err == nil || !strings.Contains(err.Error(), "nil base context") {
		t.Fatalf("collect step error = %v, want nil base context", err)
	}
}

func TestMultiAgentToleratesMergeFailure(t *testing.T) {
	llmClient := &multiAgentLLM{mergeFailErr: errors.New("merge upstream fail")}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatalf("RunWithContext returned err: %v", err)
	}
	if len(result.Findings) != len(reviewVectors) {
		t.Fatalf("findings = %d, want all cluster findings preserved on merge failure", len(result.Findings))
	}
	if !strings.Contains(result.OverallExplanation, "Merged") {
		t.Fatalf("OverallExplanation = %q, want mechanical merge summary", result.OverallExplanation)
	}
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "Merge Findings merge step failed") && strings.Contains(w, "merge upstream fail") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("expected merge warning in %#v", result.Warnings)
	}
	var mergeRun *model.AgentRun
	for i := range result.AgentRuns {
		if result.AgentRuns[i].Role == "merge" {
			mergeRun = &result.AgentRuns[i]
		}
	}
	if mergeRun == nil || mergeRun.Status != model.AgentRunStatusFailed {
		t.Fatalf("merge AgentRun = %#v, want Status=failed", mergeRun)
	}
}

func TestMultiAgentToleratesAllVectorsFailing(t *testing.T) {
	llmClient := &multiAgentLLM{
		vectorFailErr: map[string]error{
			"Code Quality":   errors.New("cq fail"),
			"Security":       errors.New("sec fail"),
			"Architecture":   errors.New("arch fail"),
			"Performance":    errors.New("perf fail"),
			"Testing":        errors.New("test fail"),
			"Best Practices": errors.New("bp fail"),
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatalf("RunWithContext returned err: %v", err)
	}
	failedReviewers := 0
	for _, run := range result.AgentRuns {
		if run.Role == "review" && run.Status == model.AgentRunStatusFailed {
			failedReviewers++
		}
	}
	if failedReviewers != len(reviewVectors) {
		t.Fatalf("failed reviewer runs = %d, want %d", failedReviewers, len(reviewVectors))
	}
	if len(result.Warnings) < len(reviewVectors) {
		t.Fatalf("warnings = %d, want at least %d", len(result.Warnings), len(reviewVectors))
	}
	// All-vectors-failed must short-circuit the merge LLM call to avoid
	// hallucinated findings from an empty payload.
	foundShortCircuit := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "All vector reviewers failed") && strings.Contains(w, "skipped merge agent") {
			foundShortCircuit = true
		}
	}
	if !foundShortCircuit {
		t.Fatalf("expected short-circuit warning in %#v", result.Warnings)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("expected empty findings on short-circuit, got %d", len(result.Findings))
	}
	var mergeRun *model.AgentRun
	for i := range result.AgentRuns {
		if result.AgentRuns[i].Role == "merge" {
			mergeRun = &result.AgentRuns[i]
		}
	}
	if mergeRun == nil || mergeRun.Status != model.AgentRunStatusSkipped {
		t.Fatalf("merge AgentRun = %#v, want Status=skipped", mergeRun)
	}
}

func TestReviewerQuestionsRenderFromSeparateTemplates(t *testing.T) {
	engine := NewEngine(stubSource{}, stubLLM{}, stubRetrieval{}, config.Profile{Model: "test"})
	baseTemplate, err := prompts.Load("agent_review_general_system_prompt.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	for _, vector := range reviewVectors {
		focusTemplate, err := prompts.Load(vector.focusFile)
		if err != nil {
			t.Fatal(err)
		}
		questionsTemplate, err := prompts.Load(vector.questionsFile)
		if err != nil {
			t.Fatal(err)
		}
		for question := range strings.SplitSeq(questionsTemplate, "\n") {
			question = strings.TrimSpace(question)
			if question == "" {
				continue
			}
			if strings.Contains(focusTemplate, question) {
				t.Fatalf("%s focus template still contains question %q", vector.name, question)
			}
		}
		questionsSnippet, err := engine.renderReviewerQuestionsSnippet(vector.questionsFile)
		if err != nil {
			t.Fatal(err)
		}
		system, err := engine.renderReviewSystemWithQuestions(baseTemplate, vector.focusFile, questionsSnippet, model.ReviewRequest{}, false, "review", nil, false)
		if err != nil {
			t.Fatal(err)
		}
		firstQuestion := strings.TrimSpace(strings.Split(strings.TrimSpace(questionsTemplate), "\n")[0])
		if !strings.Contains(system, firstQuestion) {
			t.Fatalf("%s rendered system missing first question %q", vector.name, firstQuestion)
		}
		if strings.Contains(system, "{{.QuestionsSnippet}}") {
			t.Fatalf("%s rendered system contains unresolved questions placeholder", vector.name)
		}
	}
}

func TestEngineMergeSchemaHonorsPriorityThreshold(t *testing.T) {
	llmClient := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:              model.ModeLocal,
		RepoRoot:          ".",
		MaxContextTokens:  1000,
		PriorityThreshold: "p1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.mergeSchema) == 0 {
		t.Fatal("merge schema empty")
	}
	priority := schemaFindingProperty(t, llmClient.mergeSchema, "priority")
	if got := priority["minimum"]; got != float64(0) {
		t.Fatalf("priority minimum = %#v, want 0", got)
	}
	if got := priority["maximum"]; got != float64(1) {
		t.Fatalf("priority maximum = %#v, want 1", got)
	}
}

func schemaFindingProperty(t *testing.T, schema []byte, property string) map[string]any {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(schema, &root); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	properties := root["properties"].(map[string]any)
	findings := properties["findings"].(map[string]any)
	items := findings["items"].(map[string]any)
	findingProps := items["properties"].(map[string]any)
	out, ok := findingProps[property].(map[string]any)
	if !ok {
		t.Fatalf("finding property %q missing: %#v", property, findingProps[property])
	}
	return out
}

func TestParseToolResultSummaryCountsSearchResultFiles(t *testing.T) {
	result := `{"result_count":3,"results":[` +
		`{"code_location":{"file_path":"a.go","line_range":{"start":1,"end":1,"count":1},"content":"x"}},` +
		`{"code_location":{"file_path":"a.go","line_range":{"start":9,"end":9,"count":1},"content":"x"}},` +
		`{"code_location":{"file_path":"b.go","line_range":{"start":2,"end":2,"count":1},"content":"x"}}]}`

	summary := parseToolResultSummary(result)

	if !summary.HasResultCount || summary.ResultCount != 3 {
		t.Fatalf("result count = %d (has=%t), want 3", summary.ResultCount, summary.HasResultCount)
	}
	if summary.Files != 2 {
		t.Fatalf("files = %d, want 2 distinct code_location file paths", summary.Files)
	}
}

type blockingRetrieval struct {
	started chan string
	release chan struct{}
}

func (r *blockingRetrieval) GetFile(_ context.Context, _ string, path string) (*retrieval.FileContent, error) {
	r.started <- path
	<-r.release
	return &retrieval.FileContent{
		Path:     path,
		Content:  "package extra",
		Language: "go",
	}, nil
}

func (blockingRetrieval) ListFiles(context.Context, string, string, int) (*retrieval.DirectoryListing, error) {
	return nil, errors.New("unexpected ListFiles call")
}

func (blockingRetrieval) SearchRegex(context.Context, string, string, *regexp.Regexp, int, int) (*retrieval.SearchResults, error) {
	return &retrieval.SearchResults{Results: []retrieval.SearchResult{}}, nil
}

func (blockingRetrieval) Search(context.Context, string, string, string, int, int, bool) (*retrieval.SearchResults, error) {
	return nil, errors.New("unexpected Search call")
}

func (blockingRetrieval) GetFileSlice(context.Context, string, string, int, int) (*retrieval.FileSlice, error) {
	return nil, errors.New("unexpected GetFileSlice call")
}

func (blockingRetrieval) GetSymbol(context.Context, string, retrieval.SymbolRef) (*retrieval.SymbolInfo, error) {
	return nil, errors.New("unexpected GetSymbol call")
}

func (blockingRetrieval) FindCallers(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, errors.New("unexpected FindCallers call")
}

func (blockingRetrieval) FindCallees(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, errors.New("unexpected FindCallees call")
}

func (blockingRetrieval) FindLines(context.Context, string, string, string) (*retrieval.FindLinesResult, error) {
	return nil, errors.New("unexpected FindLines call")
}

func TestEngineExecutesIndependentToolCallsConcurrently(t *testing.T) {
	retrievalEngine := &blockingRetrieval{
		started: make(chan string, 2),
		release: make(chan struct{}),
	}
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
	state := &toolRoundState{
		seenFiles:      make(map[string]retrieval.FileContent),
		seenFileRanges: make(map[string][]model.LineRange),
		seenToolCalls:  make(map[string]struct{}),
	}

	done := make(chan []llm.Message, 1)
	go func() {
		done <- engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
			{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"a.go"}`},
			{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"b.go"}`},
		}, state)
	}()

	started := map[string]struct{}{}
	for len(started) < 2 {
		path := <-retrievalEngine.started
		started[path] = struct{}{}
	}
	close(retrievalEngine.release)

	results := <-done
	if len(results) != 2 {
		t.Fatalf("tool messages = %d", len(results))
	}
	if results[0].ToolCallID != "call_1" || results[1].ToolCallID != "call_2" {
		t.Fatalf("tool message order = %#v", results)
	}
}

func TestEngineDedupesDuplicateToolCallsWithinParallelRound(t *testing.T) {
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, &capturingLLM{}, retrievalEngine, config.Profile{Model: "test"})
	state := &toolRoundState{
		seenFiles:      make(map[string]retrieval.FileContent),
		seenFileRanges: make(map[string][]model.LineRange),
		seenToolCalls:  make(map[string]struct{}),
	}

	results := engine.executeToolCalls(context.Background(), "", []llm.ToolCall{
		{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
		{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"./extra.go"}`},
	}, state)

	if len(results) != 2 {
		t.Fatalf("tool messages = %d", len(results))
	}
	if len(retrievalEngine.paths) != 1 {
		t.Fatalf("retrieval calls = %d", len(retrievalEngine.paths))
	}
	var firstPayload map[string]any
	if err := json.Unmarshal([]byte(results[0].Content), &firstPayload); err != nil {
		t.Fatalf("first tool payload should be valid json: %v", err)
	}
	if firstPayload["path"] != "extra.go" {
		t.Fatalf("first tool payload = %#v", firstPayload)
	}
	var secondPayload map[string]any
	if err := json.Unmarshal([]byte(results[1].Content), &secondPayload); err != nil {
		t.Fatalf("second tool payload should be valid json: %v", err)
	}
	if secondPayload["status"] != "error" {
		t.Fatalf("duplicate tool payload = %#v", secondPayload)
	}
	errorPayload, ok := secondPayload["error"].(map[string]any)
	if !ok {
		t.Fatalf("duplicate error payload = %#v", secondPayload)
	}
	if errorPayload["code"] != "already_requested" {
		t.Fatalf("duplicate error code = %#v", errorPayload["code"])
	}
}

func pathOrDefault(path, fallback string) string {
	if path == "" {
		return fallback
	}
	return path + "/root.go"
}

type scriptedLLM struct {
	reqs    []*llm.ReviewRequest
	results []scriptedLLMResult
}

type scriptedLLMResult struct {
	resp *llm.ReviewResponse
	err  error
}

func (s *scriptedLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	cloned := *req
	if len(req.Messages) > 0 {
		cloned.Messages = cloneTestMessages(req.Messages)
	}
	if len(req.NoToolsMessages) > 0 {
		cloned.NoToolsMessages = cloneTestMessages(req.NoToolsMessages)
	}
	if len(req.Tools) > 0 {
		cloned.Tools = append([]llm.ToolDefinition(nil), req.Tools...)
	}
	s.reqs = append(s.reqs, &cloned)
	if len(s.results) == 0 {
		return &llm.ReviewResponse{
			OverallCorrectness:     "patch is correct",
			OverallExplanation:     "ok",
			OverallConfidenceScore: 0.5,
		}, nil
	}
	next := s.results[0]
	s.results = s.results[1:]
	return next.resp, next.err
}

type reasoningExtractLLM struct {
	mu                      sync.Mutex
	reviewerReasoning       []string
	reviewerMessages        []string
	reviewerModels          []string
	collectInputs           []string
	collectModels           []string
	collectOutputs          []string
	updateFullLists         []string
	updateFindingsJSON      []string
	updateModels            []string
	updateOutputs           []string
	firstCollectStarted     chan struct{}
	releaseFirstCollect     chan struct{}
	firstCollectStartedOnce sync.Once
}

func (s *reasoningExtractLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	system := ""
	if len(req.Messages) > 0 {
		system = req.Messages[0].Content
	}
	user := lastUserContent(req.Messages)
	switch {
	case isReasoningCollectPrompt(system):
		input := collectPayloadReasoning(user)
		s.mu.Lock()
		idx := len(s.collectInputs)
		s.collectInputs = append(s.collectInputs, input)
		s.collectModels = append(s.collectModels, req.Model)
		output := outputAt(s.collectOutputs, idx)
		started := s.firstCollectStarted
		release := s.releaseFirstCollect
		s.mu.Unlock()
		if idx == 0 && started != nil {
			s.firstCollectStartedOnce.Do(func() { close(started) })
		}
		if idx == 0 && release != nil {
			<-release
		}
		return textResponse(output, 1), nil
	case isReasoningUpdatePrompt(system):
		fullList, findingsJSON := updatePayloadParts(user)
		s.mu.Lock()
		idx := len(s.updateFullLists)
		s.updateFullLists = append(s.updateFullLists, fullList)
		s.updateFindingsJSON = append(s.updateFindingsJSON, findingsJSON)
		s.updateModels = append(s.updateModels, req.Model)
		output := outputAt(s.updateOutputs, idx)
		s.mu.Unlock()
		return textResponse(output, 1), nil
	default:
		s.mu.Lock()
		idx := len(s.reviewerMessages)
		s.reviewerMessages = append(s.reviewerMessages, user)
		s.reviewerModels = append(s.reviewerModels, req.Model)
		reasoning := outputAt(s.reviewerReasoning, idx)
		s.mu.Unlock()
		if req.ReasoningSink != nil {
			req.ReasoningSink.Append(reasoning)
		}
		title := "Initial finding"
		if idx > 0 {
			title = fmt.Sprintf("Nudge %d finding", idx)
		}
		resp := nudgeReviewResponse(title, 10, nudgeFinding(title, idx+1))
		resp.Reasoned = true
		return resp, nil
	}
}

type multiIterReasoningExtractLLM struct {
	mu                sync.Mutex
	reviewerCallNum   int
	reviewerMessages  []string
	reviewerReasoning []string
	collectInputs     []string
	collectOutputs    []string
	updateFullLists   []string
	updateOutputs     []string
}

func (s *multiIterReasoningExtractLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	system := ""
	if len(req.Messages) > 0 {
		system = req.Messages[0].Content
	}
	user := lastUserContent(req.Messages)
	switch {
	case isReasoningCollectPrompt(system):
		input := collectPayloadReasoning(user)
		s.mu.Lock()
		idx := len(s.collectInputs)
		s.collectInputs = append(s.collectInputs, input)
		output := outputAt(s.collectOutputs, idx)
		s.mu.Unlock()
		return textResponse(output, 1), nil
	case isReasoningUpdatePrompt(system):
		fullList, _ := updatePayloadParts(user)
		s.mu.Lock()
		idx := len(s.updateFullLists)
		s.updateFullLists = append(s.updateFullLists, fullList)
		output := outputAt(s.updateOutputs, idx)
		s.mu.Unlock()
		return textResponse(output, 1), nil
	default:
		s.mu.Lock()
		idx := s.reviewerCallNum
		s.reviewerCallNum++
		s.reviewerMessages = append(s.reviewerMessages, user)
		reasoning := outputAt(s.reviewerReasoning, idx)
		s.mu.Unlock()
		if req.ReasoningSink != nil {
			req.ReasoningSink.Append(reasoning)
		}
		if idx == 0 {
			return &llm.ReviewResponse{
				RawResponse: "tool-call-iter-1",
				ToolCalls: []llm.ToolCall{{
					ID:        "iter1",
					Name:      "inspect_file",
					Arguments: `{"path":"main.go"}`,
				}},
				Reasoned:   true,
				TokensUsed: model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 1},
			}, nil
		}
		title := fmt.Sprintf("Finding %d", idx)
		resp := nudgeReviewResponse(title, 10, nudgeFinding(title, idx+1))
		resp.Reasoned = true
		return resp, nil
	}
}

func textResponse(content string, tokens int) *llm.ReviewResponse {
	return &llm.ReviewResponse{
		RawResponse: content,
		TokensUsed:  model.TokenUsage{PromptTokens: tokens, CompletionTokens: tokens, TotalTokens: tokens},
	}
}

func outputAt(values []string, idx int) string {
	if idx < len(values) {
		return values[idx]
	}
	return "NONE"
}

func lastUserContent(messages []llm.Message) string {
	for _, message := range slices.Backward(messages) {
		if message.Role == "user" {
			return message.Content
		}
	}
	return ""
}

func isReasoningCollectPrompt(system string) bool {
	return strings.Contains(system, "extracting possible findings in the notes")
}

func isReasoningUpdatePrompt(system string) bool {
	return strings.Contains(system, "making sure that all findings noted by one engineer")
}

func collectPayloadReasoning(content string) string {
	const notesStart = "Notes made by an engineer while doing a thorough code review:"
	if _, rest, ok := strings.Cut(content, notesStart); ok {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(strings.TrimPrefix(content, "Reviewer reasoning trace:"))
}

func updatePayloadParts(content string) (string, string) {
	listStart := "List of findings noted down by one engineer:"
	findingsStart := "JSON with the findings of a code review done by another engineer:"
	if !strings.Contains(content, listStart) || !strings.Contains(content, findingsStart) {
		listStart = "Extracted issue list:"
		findingsStart = "Current findings JSON:"
	}
	_, rest, _ := strings.Cut(content, listStart)
	list, findings, _ := strings.Cut(rest, findingsStart)
	return strings.TrimSpace(list), strings.TrimSpace(findings)
}

func schemaStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if ok {
			result = append(result, text)
		}
	}
	return result
}

func TestShouldDropFinding(t *testing.T) {
	cases := []struct {
		name       string
		verdict    string
		confidence float64
		policy     string
		wantDrop   bool
		wantReason string
	}{
		{"confirmed never drops (refuted-only)", model.VerdictConfirmed, 0.99, "refuted-only", false, "kept"},
		{"confirmed never drops (both)", model.VerdictConfirmed, 0.99, "refuted-and-unverified", false, "kept"},
		{"refuted high confidence drops", model.VerdictRefuted, 0.85, "refuted-only", true, model.VerdictRefuted},
		{"refuted low confidence drops", model.VerdictRefuted, 0.01, "refuted-only", true, model.VerdictRefuted},
		{"refuted zero confidence drops", model.VerdictRefuted, 0.0, "refuted-only", true, model.VerdictRefuted},
		{"unverified kept (refuted-only)", model.VerdictUnverified, 0.95, "refuted-only", false, "kept"},
		{"unverified drops (both)", model.VerdictUnverified, 0.0, "refuted-and-unverified", true, model.VerdictUnverified},
		{"refuted policy=none kept", model.VerdictRefuted, 0.99, "none", false, "kept"},
		{"missing verdict treated as unverified (refuted-only)", "", 0.99, "refuted-only", false, "kept"},
		{"missing verdict treated as unverified (both)", "", 0.99, "refuted-and-unverified", true, model.VerdictUnverified},
		{"bogus policy defaults to refuted-only behavior", model.VerdictRefuted, 0.9, "garbage", true, model.VerdictRefuted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := &model.FindingVerification{Verdict: tc.verdict, ConfidenceScore: tc.confidence}
			drop, reason := shouldDropFinding(v, tc.policy)
			if drop != tc.wantDrop {
				t.Fatalf("drop = %v, want %v", drop, tc.wantDrop)
			}
			if reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

func TestShouldDropFindingNilVerification(t *testing.T) {
	drop, reason := shouldDropFinding(nil, "refuted-and-unverified")
	if drop || reason != "kept" {
		t.Fatalf("nil verification: drop=%v reason=%q", drop, reason)
	}
}

func TestNormalizeDropPolicyFallback(t *testing.T) {
	for _, p := range []string{"", "garbage", "REFUTED-ONLY"} {
		if got := normalizeDropPolicy(p); got != "refuted-only" {
			t.Fatalf("normalizeDropPolicy(%q) = %q, want refuted-only", p, got)
		}
	}
	for _, p := range []string{"none", "refuted-only", "refuted-and-unverified"} {
		if got := normalizeDropPolicy(p); got != p {
			t.Fatalf("normalizeDropPolicy(%q) = %q, want passthrough", p, got)
		}
	}
}

func TestValidateDropPolicy(t *testing.T) {
	for _, p := range []string{"none", "refuted-only", "refuted-and-unverified"} {
		if err := ValidateDropPolicy(p); err != nil {
			t.Fatalf("ValidateDropPolicy(%q) = %v, want nil", p, err)
		}
	}
	for _, p := range []string{"", "garbage", "REFUTED-ONLY", "refuted_only"} {
		if err := ValidateDropPolicy(p); err == nil {
			t.Fatalf("ValidateDropPolicy(%q) = nil, want error", p)
		}
	}
}
