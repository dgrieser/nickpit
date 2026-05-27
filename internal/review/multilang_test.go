package review

import (
	"context"
	"encoding/json"
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

	_, err := engine.Run(context.Background(), model.ReviewRequest{
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

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[0].Messages[1].Content), &payload); err != nil {
		t.Fatal(err)
	}
	styleGuides := payload["style_guides"].([]any)
	if len(styleGuides) != 1 {
		t.Fatalf("style guides = %#v", payload["style_guides"])
	}
	pythonStyleGuide := styleGuides[0].(map[string]any)
	if pythonStyleGuide["language"] != "python" {
		t.Fatalf("style guide language = %#v", pythonStyleGuide["language"])
	}
	if content, _ := pythonStyleGuide["content"].(string); !strings.Contains(content, "# Python Style Guide") {
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

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[0].Messages[1].Content), &payload); err != nil {
		t.Fatal(err)
	}
	styleGuides := payload["style_guides"].([]any)
	if len(styleGuides) != 5 {
		t.Fatalf("style guides = %#v", payload["style_guides"])
	}
	contentsByLanguage := make(map[string]string)
	for _, item := range styleGuides {
		styleGuide := item.(map[string]any)
		contentsByLanguage[styleGuide["language"].(string)] = styleGuide["content"].(string)
	}
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

func styleGuideContentsForContext(t *testing.T, reviewCtx *model.ReviewContext) map[string]string {
	t.Helper()
	llmClient := &capturingLLM{}
	engine := NewEngine(kubernetesStyleGuideDiffSource{ctx: reviewCtx}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[0].Messages[1].Content), &payload); err != nil {
		t.Fatal(err)
	}
	contentsByLanguage := make(map[string]string)
	styleGuides, _ := payload["style_guides"].([]any)
	for _, item := range styleGuides {
		styleGuide := item.(map[string]any)
		contentsByLanguage[styleGuide["language"].(string)] = styleGuide["content"].(string)
	}
	return contentsByLanguage
}
