package review

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
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
	}, nil
}

type stubLLM struct{}

func (stubLLM) Review(context.Context, *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	return &llm.ReviewResponse{
		Findings: []model.Finding{
			{Title: "[P3] info", Body: "low", ConfidenceScore: 0.5, Priority: intPtr(3), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}},
			{Title: "[P1] error", Body: "high", ConfidenceScore: 0.9, Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 2, End: 2}}},
		},
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "summary",
		OverallConfidenceScore: 0.9,
	}, nil
}

type capturingLLM struct {
	reqs  []*llm.ReviewRequest
	resps []*llm.ReviewResponse
}

func (s *capturingLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	cloned := *req
	if len(req.Messages) > 0 {
		cloned.Messages = append([]llm.Message(nil), req.Messages...)
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

type stubRetrieval struct{}

func (stubRetrieval) GetFile(context.Context, string, string) (*retrieval.FileContent, error) {
	return &retrieval.FileContent{
		Path:     "extra.go",
		Content:  "package extra",
		Language: "go",
	}, nil
}

func (stubRetrieval) ListFiles(context.Context, string, string) (*retrieval.DirectoryListing, error) {
	return &retrieval.DirectoryListing{
		Path:  "pkg",
		Files: []string{"pkg/a.go", "pkg/b.go"},
	}, nil
}

type countingRetrieval struct {
	paths []string
}

func (r *countingRetrieval) GetFile(_ context.Context, _ string, path string) (*retrieval.FileContent, error) {
	r.paths = append(r.paths, path)
	return &retrieval.FileContent{
		Path:     path,
		Content:  "package extra",
		Language: "go",
	}, nil
}

func (r *countingRetrieval) ListFiles(_ context.Context, _ string, path string) (*retrieval.DirectoryListing, error) {
	r.paths = append(r.paths, "list:"+path)
	return &retrieval.DirectoryListing{
		Path:  path,
		Files: []string{path + "/a.go", path + "/b.go"},
	}, nil
}

func (countingRetrieval) GetFileSlice(context.Context, string, string, int, int) (*retrieval.FileSlice, error) {
	return nil, errors.New("unexpected GetFileSlice call")
}

func (countingRetrieval) GetSymbol(context.Context, string, retrieval.SymbolRef) (*retrieval.SymbolInfo, error) {
	return nil, errors.New("unexpected GetSymbol call")
}

func (countingRetrieval) FindCallers(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, errors.New("unexpected FindCallers call")
}

func (countingRetrieval) FindCallees(context.Context, string, retrieval.SymbolRef, int) (*retrieval.CallHierarchy, error) {
	return nil, errors.New("unexpected FindCallees call")
}

func (stubRetrieval) GetFileSlice(context.Context, string, string, int, int) (*retrieval.FileSlice, error) {
	return nil, errors.New("unexpected GetFileSlice call")
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

func TestEnginePriorityFilter(t *testing.T) {
	engine := NewEngine(stubSource{}, stubLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:              model.ModeLocal,
		MaxContextTokens:  1000,
		PriorityThreshold: "p1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %d", len(result.Findings))
	}
}

func TestEngineSplitsSystemAndUserPrompts(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}

	req := llmClient.reqs[0]
	if len(req.Messages) != 2 {
		t.Fatalf("messages = %d", len(req.Messages))
	}
	if req.Messages[0].Content == req.Messages[1].Content {
		t.Fatal("system and user prompts should differ")
	}
	if req.Messages[0].Content == "" || req.Messages[1].Content == "" {
		t.Fatal("system and user prompts should both be populated")
	}
	if want := "You are acting as a senior engineer performing a thorough code review for a proposed code change made by another engineer."; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt = %q", req.Messages[0].Content)
	}
	if want := "If you need more code context, call the `inspect_file` tool"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing tool instructions: %q", req.Messages[0].Content)
	}
	if want := "Do not request the same file more than once."; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing duplicate-request guard: %q", req.Messages[0].Content)
	}
	if want := "Make sure to output the findings in the following JSON format:"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing example JSON instructions: %q", req.Messages[0].Content)
	}
	if want := "\"overall_correctness\": \"patch is correct\""; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing rendered example JSON: %q", req.Messages[0].Content)
	}
	if want := "\"title\": \"[P1] Example title\""; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing example finding JSON: %q", req.Messages[0].Content)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(req.Messages[1].Content), &payload); err != nil {
		t.Fatalf("user prompt should be valid json: %v\n%s", err, req.Messages[1].Content)
	}
	repository, ok := payload["repository"].(map[string]any)
	if !ok {
		t.Fatalf("user prompt missing repository object: %#v", payload)
	}
	if repository["full_name"] != "repo" {
		t.Fatalf("repository.full_name = %#v", repository["full_name"])
	}
	if payload["title"] != "title" {
		t.Fatalf("title = %#v", payload["title"])
	}
	if _, ok := payload["changed_files"]; !ok {
		t.Fatalf("user prompt missing changed_files: %#v", payload)
	}
	for _, unwanted := range []string{"mode", "checkout_root", "diff"} {
		if _, exists := payload[unwanted]; exists {
			t.Fatalf("user prompt unexpectedly contains %q: %#v", unwanted, payload[unwanted])
		}
	}
	if len(req.Tools) != 2 {
		t.Fatalf("tools = %d", len(req.Tools))
	}
	if req.Tools[0].Name != "inspect_file" {
		t.Fatalf("tool name = %q", req.Tools[0].Name)
	}
	if req.Tools[1].Name != "list_files" {
		t.Fatalf("tool name = %q", req.Tools[1].Name)
	}
}

func TestEngineDoesNotUseAPISchemaByDefault(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	if len(llmClient.reqs[0].Schema) != 0 {
		t.Fatalf("schema should be empty by default, got %s", string(llmClient.reqs[0].Schema))
	}
}

func TestEngineExecutesInspectFileToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				Findings: []model.Finding{
					{Title: "[P1] error", Body: "high", ConfidenceScore: 0.9, Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 2, End: 2}}},
				},
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
				TokensUsed:             model.TokenUsage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		ToolRounds:       1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	if result.ToolRounds != 1 {
		t.Fatalf("tool rounds = %d", result.ToolRounds)
	}
	if got := result.TokensUsed.TotalTokens; got != 10 {
		t.Fatalf("total tokens = %d", got)
	}

	req := llmClient.reqs[1]
	if len(req.Messages) != 4 {
		t.Fatalf("messages = %d", len(req.Messages))
	}
	if req.Messages[2].Role != "assistant" {
		t.Fatalf("assistant role = %q", req.Messages[2].Role)
	}
	if len(req.Messages[2].ToolCalls) != 1 || req.Messages[2].ToolCalls[0].Name != "inspect_file" {
		t.Fatalf("assistant tool calls = %#v", req.Messages[2].ToolCalls)
	}
	if req.Messages[3].Role != "tool" {
		t.Fatalf("tool role = %q", req.Messages[3].Role)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(req.Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v\n%s", err, req.Messages[3].Content)
	}
	if payload["path"] != "extra.go" || payload["content"] != "package extra" || payload["language"] != "go" {
		t.Fatalf("tool payload = %#v", payload)
	}
}

func TestEngineUsesAPISchemaWhenEnabled(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		UseJSONSchema:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 1 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	if string(llmClient.reqs[0].Schema) != string(llm.FindingsSchema) {
		t.Fatalf("schema = %s", string(llmClient.reqs[0].Schema))
	}
	if strings.Contains(llmClient.reqs[0].Messages[0].Content, "Example JSON output:") {
		t.Fatalf("system prompt should omit example snippet when API schema is enabled: %q", llmClient.reqs[0].Messages[0].Content)
	}
}

func TestEngineReturnsToolErrorsToModel(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":""}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		ToolRounds:       1,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["error"] != "missing required argument: path" {
		t.Fatalf("tool error payload = %#v", payload)
	}
}

func TestEngineStopsAtToolRoundLimit(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"main.go"}`},
				},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		ToolRounds:       1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OverallCorrectness != "patch is incorrect" {
		t.Fatalf("overall_correctness = %q", result.OverallCorrectness)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %d", len(result.Findings))
	}
}

func TestEngineReturnsAlreadyProvidedForDuplicateFileRequests(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"./extra.go"}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		ToolRounds:       2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 {
		t.Fatalf("retrieval calls = %d", len(retrievalEngine.paths))
	}
	if retrievalEngine.paths[0] != "extra.go" {
		t.Fatalf("retrieval path = %q", retrievalEngine.paths[0])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[2].Messages[5].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["status"] != "already_provided" {
		t.Fatalf("duplicate tool payload = %#v", payload)
	}
	if payload["path"] != "extra.go" {
		t.Fatalf("duplicate tool path = %#v", payload["path"])
	}
	if _, ok := payload["content"]; ok {
		t.Fatalf("duplicate tool payload should omit content: %#v", payload)
	}
	if _, ok := payload["language"]; ok {
		t.Fatalf("duplicate tool payload should omit language: %#v", payload)
	}
}

func TestEngineExecutesInspectListFilesToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{"path":"pkg"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		ToolRounds:       1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "list:pkg" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["path"] != "pkg" {
		t.Fatalf("list payload path = %#v", payload["path"])
	}
	files, ok := payload["files"].([]any)
	if !ok || len(files) != 2 {
		t.Fatalf("list payload files = %#v", payload["files"])
	}
}

func TestEngineTreatsZeroToolRoundsAsUnlimited(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "list_files", Arguments: `{"path":"pkg"}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	retrievalEngine := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, retrievalEngine, config.Profile{Model: "test"})

	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		ToolRounds:       0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OverallCorrectness != "patch is correct" {
		t.Fatalf("overall_correctness = %q", result.OverallCorrectness)
	}
	if result.ToolRounds != 2 {
		t.Fatalf("tool rounds = %d", result.ToolRounds)
	}
	if len(retrievalEngine.paths) != 2 {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}
}

func intPtr(v int) *int {
	return &v
}
