package review

import (
	"context"
	"encoding/json"
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
		MaxToolCalls:     1,
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
