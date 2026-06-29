package review

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
)

type multilangRetrieval struct {
	stubRetrieval
}

func (multilangRetrieval) GetFile(context.Context, string, string) (*retrieval.FileContent, error) {
	return &retrieval.FileContent{
		Path:     "module.py",
		Content:  "def run():\n    return 1",
		Language: "python",
	}, nil
}

func (multilangRetrieval) Search(context.Context, string, string, string, int, int, bool) (*retrieval.SearchResults, error) {
	return &retrieval.SearchResults{
		Path:         "web",
		Query:        "run",
		ContextLines: 2,
		ResultCount:  1,
		Results: []retrieval.SearchResult{
			{Path: "web/app.ts", StartLine: 1, EndLine: 3, Language: "nodejs", Content: "export function run() {\n  return 1\n}"},
		},
	}, nil
}

func TestEngineSerializesPythonAndNodeToolPayloadLanguages(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"module.py"}`},
					{ID: "call_2", Name: "search", Arguments: `{"path":"web","query":"run","context_lines":2}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, multilangRetrieval{}, config.Profile{Model: "test"})

	_, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     2,
	})
	if err != nil {
		t.Fatal(err)
	}

	var inspectPayload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &inspectPayload); err != nil {
		t.Fatal(err)
	}
	if inspectPayload["language"] != "python" {
		t.Fatalf("inspect payload = %#v", inspectPayload)
	}

	var searchPayload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[4].Content), &searchPayload); err != nil {
		t.Fatal(err)
	}
	results := searchPayload["results"].([]any)
	first := results[0].(map[string]any)
	if first["language"] != "nodejs" {
		t.Fatalf("search payload = %#v", searchPayload)
	}
}

type pythonDiffSource struct{}

func (pythonDiffSource) ResolveContext(context.Context, model.ReviewRequest) (*model.ReviewContext, error) {
	return &model.ReviewContext{
		Mode: model.ModeLocal,
		Repository: model.RepositoryInfo{
			FullName: "repo",
		},
		Title:       "title",
		Description: "description",
		ChangedFiles: []model.ChangedFile{
			{Path: "module.py", Status: model.FileModified, Additions: 1},
			{Path: "README.md", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: "module.py",
				Language: "python",
				OldStart: 1,
				OldLines: 1,
				NewStart: 1,
				NewLines: 1,
				Content:  "-old\n+new\n",
			},
		},
	}, nil
}

func TestEngineAddsPythonStyleGuideForPythonDiffs(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(pythonDiffSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(taskMessageContent(llmClient.reqs[0])), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["style_guides"]; ok {
		t.Fatalf("user prompt should not include style_guides: %#v", payload["style_guides"])
	}
	contentsByLanguage := styleGuideContentsFromSystem(llmClient.reqs[0].Messages[0].Content)
	if len(contentsByLanguage) != 1 {
		t.Fatalf("style guides = %#v", contentsByLanguage)
	}
	if content := contentsByLanguage["python"]; !strings.Contains(content, "# Python Style Guide") {
		t.Fatalf("style guide content = %.80q", content)
	}
}

type styleGuideDiffSource struct{}

func (styleGuideDiffSource) ResolveContext(context.Context, model.ReviewRequest) (*model.ReviewContext, error) {
	return &model.ReviewContext{
		Mode: model.ModeLocal,
		Repository: model.RepositoryInfo{
			FullName: "repo",
		},
		Title:       "title",
		Description: "description",
		ChangedFiles: []model.ChangedFile{
			{Path: "web/app.js", Status: model.FileModified, Additions: 1},
			{Path: "web/app.ts", Status: model.FileModified, Additions: 1},
			{Path: "web/index.html", Status: model.FileModified, Additions: 1},
			{Path: "web/styles.css", Status: model.FileModified, Additions: 1},
			{Path: "src/Program.cs", Status: model.FileModified, Additions: 1},
			{Path: "charts/app/Chart.yaml", Status: model.FileModified, Additions: 1},
		},
	}, nil
}

func TestEngineAddsStyleGuidesForUntrackedMarkdownGuides(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(styleGuideDiffSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(taskMessageContent(llmClient.reqs[0])), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["style_guides"]; ok {
		t.Fatalf("user prompt should not include style_guides: %#v", payload["style_guides"])
	}
	contentsByLanguage := styleGuideContentsFromSystem(llmClient.reqs[0].Messages[0].Content)
	for language, want := range map[string]string{
		"javascript": "# JavaScript Style Guide",
		"typescript": "# TypeScript Style Guide",
		"html":       "# HTML & CSS Style Guide",
		"csharp":     "# C# Style Guide",
		"helm":       "# Helm Style Guide",
	} {
		if !strings.Contains(contentsByLanguage[language], want) {
			t.Fatalf("%s style guide content = %.80q", language, contentsByLanguage[language])
		}
	}
	if _, ok := contentsByLanguage["css"]; ok {
		t.Fatalf("HTML/CSS guide should be included once: %#v", contentsByLanguage)
	}
}

type kubernetesStyleGuideDiffSource struct {
	ctx *model.ReviewContext
}

func (s kubernetesStyleGuideDiffSource) ResolveContext(context.Context, model.ReviewRequest) (*model.ReviewContext, error) {
	return s.ctx, nil
}

func TestEngineAddsKubernetesStyleGuideForPathSignals(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "k8s/deployment.yaml", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{FilePath: "k8s/deployment.yaml", Language: "yaml", Content: "-old\n+new\n"},
		},
	})
	if content := contentsByLanguage["kubernetes"]; !strings.Contains(content, "# Kubernetes Style Guide") {
		t.Fatalf("kubernetes style guide content = %.80q", content)
	}
}

func TestEngineAddsKubernetesStyleGuideForYAMLHunkContent(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "config/app.yaml", Status: model.FileModified, Additions: 2},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: "config/app.yaml",
				Language: "yaml",
				Content:  "+apiVersion: apps/v1\n+kind: Deployment\n",
			},
		},
	})
	if content := contentsByLanguage["kubernetes"]; !strings.Contains(content, "# Kubernetes Style Guide") {
		t.Fatalf("kubernetes style guide content = %.80q", content)
	}
}

func TestEngineAddsKubernetesStyleGuideForGoOperatorSignals(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "internal/controller/redis_controller.go", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: "internal/controller/redis_controller.go",
				Language: "go",
				Content:  "+import \"sigs.k8s.io/controller-runtime/pkg/client\"\n",
			},
		},
	})
	if content := contentsByLanguage["go"]; !strings.Contains(content, "# Go Style Guide") {
		t.Fatalf("go style guide content = %.80q", content)
	}
	if content := contentsByLanguage["kubernetes"]; !strings.Contains(content, "# Kubernetes Style Guide") {
		t.Fatalf("kubernetes style guide content = %.80q", content)
	}
}

func TestEngineDoesNotAddKubernetesStyleGuideForGenericVersionedAPIPath(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "api/v1/users.go", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: "api/v1/users.go",
				Language: "go",
				Content:  "+type User struct { ID string }\n",
			},
		},
	})
	if _, ok := contentsByLanguage["kubernetes"]; ok {
		t.Fatalf("kubernetes style guide should not be included: %#v", contentsByLanguage)
	}
}

func TestEngineAddsHelmAndKubernetesStyleGuidesForManifestTemplates(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "charts/app/templates/deployment.yaml", Status: model.FileModified, Additions: 2},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: "charts/app/templates/deployment.yaml",
				Language: "helm",
				Content:  "+apiVersion: apps/v1\n+kind: Deployment\n",
			},
		},
	})
	if content := contentsByLanguage["helm"]; !strings.Contains(content, "# Helm Style Guide") {
		t.Fatalf("helm style guide content = %.80q", content)
	}
	if content := contentsByLanguage["kubernetes"]; !strings.Contains(content, "# Kubernetes Style Guide") {
		t.Fatalf("kubernetes style guide content = %.80q", content)
	}
}

func TestEngineDoesNotAddKubernetesStyleGuideForGenericYAML(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "config/settings.yaml", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{FilePath: "config/settings.yaml", Language: "yaml", Content: "+debug: true\n"},
		},
	})
	if _, ok := contentsByLanguage["kubernetes"]; ok {
		t.Fatalf("kubernetes style guide should not be included: %#v", contentsByLanguage)
	}
}

func TestStyleGuideDetectorAddsKubernetesFromSupplementalContext(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "README.md", Status: model.FileModified, Additions: 1},
		},
		Diff: "diff --git a/README.md b/README.md\n@@ -1 +1 @@\n-old\n+new\n",
		SupplementalContext: []model.SupplementalFile{
			{
				Path:     "notes/rendered.yaml",
				Language: "yaml",
				Content:  "apiVersion: batch/v1\nkind: CronJob\n",
				Kind:     "context_tool_result",
			},
		},
	})
	if content := contentsByLanguage["kubernetes"]; !strings.Contains(content, "# Kubernetes Style Guide") {
		t.Fatalf("kubernetes style guide content = %.80q", content)
	}
}

func TestStyleGuideDetectorAddsKubernetesFromChangedFileContent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config", "app.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("apiVersion: v1\nkind: Service\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:         model.ModeLocal,
		CheckoutRoot: root,
		Repository:   model.RepositoryInfo{FullName: "repo"},
		Title:        "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "config/app.yaml", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{FilePath: "config/app.yaml", Language: "yaml", Content: "+metadata:\n+  name: app\n"},
		},
	})
	if content := contentsByLanguage["kubernetes"]; !strings.Contains(content, "# Kubernetes Style Guide") {
		t.Fatalf("kubernetes style guide content = %.80q", content)
	}
}

func TestStyleGuideDetectorDedupeKeepsOneKubernetesGuide(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "k8s/deployment.yaml", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{FilePath: "k8s/deployment.yaml", Language: "yaml", Content: "+apiVersion: apps/v1\n+kind: Deployment\n"},
		},
		SupplementalContext: []model.SupplementalFile{
			{Path: "rendered/service.yaml", Language: "yaml", Content: "apiVersion: v1\nkind: Service\n"},
		},
	})
	if len(contentsByLanguage) != 1 {
		t.Fatalf("style guides = %#v", contentsByLanguage)
	}
	if content := contentsByLanguage["kubernetes"]; !strings.Contains(content, "# Kubernetes Style Guide") {
		t.Fatalf("kubernetes style guide content = %.80q", content)
	}
}

func TestStyleGuideDetectorAddsSQLFromEmbeddedQuery(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "internal/store/users.go", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: "internal/store/users.go",
				Language: "go",
				Content:  "+const userQuery = `SELECT id, email FROM users WHERE active = true`\n",
			},
		},
	})
	if content := contentsByLanguage["go"]; !strings.Contains(content, "# Go Style Guide") {
		t.Fatalf("go style guide content = %.80q", content)
	}
	if content := contentsByLanguage["sql"]; !strings.Contains(content, "# SQL Optimization Patterns") {
		t.Fatalf("sql style guide content = %.80q", content)
	}
}

func TestStyleGuideDetectorDoesNotAddSQLForGenericSelectText(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "internal/ui/copy.go", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: "internal/ui/copy.go",
				Language: "go",
				Content:  "+const label = \"Select a color before continuing\"\n",
			},
		},
	})
	if _, ok := contentsByLanguage["sql"]; ok {
		t.Fatalf("sql style guide should not be included: %#v", contentsByLanguage)
	}
}

func TestStyleGuideDetectorAddsBashFromExtensionlessScript(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "scripts/deploy", Status: model.FileModified, Additions: 2},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: "scripts/deploy",
				Language: "text",
				Content:  "+#!/usr/bin/env bash\n+set -Eeuo pipefail\n",
			},
		},
	})
	if content := contentsByLanguage["shell"]; !strings.Contains(content, "# Bash Style Guide") {
		t.Fatalf("bash style guide content = %.80q", content)
	}
}

func TestStyleGuideDetectorAddsBashFromWorkflowRunStep(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: ".github/workflows/deploy.yml", Status: model.FileModified, Additions: 2},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: ".github/workflows/deploy.yml",
				Language: "yaml",
				Content:  "+      run: |\n+        set -Eeuo pipefail\n",
			},
		},
	})
	if content := contentsByLanguage["shell"]; !strings.Contains(content, "# Bash Style Guide") {
		t.Fatalf("bash style guide content = %.80q", content)
	}
}

func TestStyleGuideDetectorAddsHTMLCSSForTSX(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "web/Button.tsx", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: "web/Button.tsx",
				Language: "nodejs",
				Content:  "+export function Button() { return <button aria-label=\"Save\">Save</button> }\n",
			},
		},
	})
	if content := contentsByLanguage["typescript"]; !strings.Contains(content, "# TypeScript Style Guide") {
		t.Fatalf("typescript style guide content = %.80q", content)
	}
	if content := contentsByLanguage["html"]; !strings.Contains(content, "# HTML & CSS Style Guide") {
		t.Fatalf("html/css style guide content = %.80q", content)
	}
}

func TestStyleGuideDetectorDoesNotAddHTMLCSSForGoTypeParameter(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "internal/set/set.go", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: "internal/set/set.go",
				Language: "go",
				Content:  "+func NewSet[T comparable](values ...T) map[T]struct{} { return nil }\n",
			},
		},
	})
	if _, ok := contentsByLanguage["html"]; ok {
		t.Fatalf("html/css style guide should not be included: %#v", contentsByLanguage)
	}
}

func TestStyleGuideDetectorAddsHTMLCSSForCSSInTS(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "web/theme.ts", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{
				FilePath: "web/theme.ts",
				Language: "nodejs",
				Content:  "+export const button = css`@media (min-width: 40rem) { color: var(--accent); }`\n",
			},
		},
	})
	if content := contentsByLanguage["typescript"]; !strings.Contains(content, "# TypeScript Style Guide") {
		t.Fatalf("typescript style guide content = %.80q", content)
	}
	if content := contentsByLanguage["html"]; !strings.Contains(content, "# HTML & CSS Style Guide") {
		t.Fatalf("html/css style guide content = %.80q", content)
	}
}

func TestStyleGuideDetectorDedupesHTMLCSSGuide(t *testing.T) {
	contentsByLanguage := styleGuideContentsForContext(t, &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "web/index.html", Status: model.FileModified, Additions: 1},
			{Path: "web/styles.css", Status: model.FileModified, Additions: 1},
		},
		DiffHunks: []model.DiffHunk{
			{FilePath: "web/index.html", Language: "html", Content: "+<main aria-label=\"Dashboard\"></main>\n"},
			{FilePath: "web/styles.css", Language: "css", Content: "+.button { color: red; }\n"},
		},
	})
	if len(contentsByLanguage) != 1 {
		t.Fatalf("style guides = %#v", contentsByLanguage)
	}
	if content := contentsByLanguage["html"]; !strings.Contains(content, "# HTML & CSS Style Guide") {
		t.Fatalf("html/css style guide content = %.80q", content)
	}
}

func styleGuideContentsForContext(t *testing.T, reviewCtx *model.ReviewContext) map[string]string {
	t.Helper()
	llmClient := &capturingLLM{}
	engine := NewEngine(kubernetesStyleGuideDiffSource{ctx: reviewCtx}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         reviewCtx.CheckoutRoot,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(taskMessageContent(llmClient.reqs[0])), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["style_guides"]; ok {
		t.Fatalf("user prompt should not include style_guides: %#v", payload["style_guides"])
	}
	return styleGuideContentsFromSystem(llmClient.reqs[0].Messages[0].Content)
}

func styleGuideContentsFromSystem(system string) map[string]string {
	titleLanguages := map[string]string{
		"# Bash Style Guide":          "shell",
		"# C# Style Guide":            "csharp",
		"# Go Style Guide":            "go",
		"# Helm Style Guide":          "helm",
		"# HTML & CSS Style Guide":    "html",
		"# JavaScript Style Guide":    "javascript",
		"# Kubernetes Style Guide":    "kubernetes",
		"# Python Style Guide":        "python",
		"# SQL Optimization Patterns": "sql",
		"# TypeScript Style Guide":    "typescript",
	}
	out := make(map[string]string)
	for _, section := range strings.Split(system, "\n# ")[1:] {
		section = "# " + section
		heading, _, ok := strings.Cut(section, "\n")
		if !ok {
			continue
		}
		language, ok := titleLanguages[strings.TrimSpace(heading)]
		if !ok {
			continue
		}
		out[language] = section
	}
	return out
}
