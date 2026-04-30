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
	if len(styleGuides) != 4 {
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
	} {
		if !strings.Contains(contentsByLanguage[language], want) {
			t.Fatalf("%s style guide content = %.80q", language, contentsByLanguage[language])
		}
	}
	if _, ok := contentsByLanguage["css"]; ok {
		t.Fatalf("HTML/CSS guide should be included once: %#v", contentsByLanguage)
	}
}
