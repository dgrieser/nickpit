package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
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
			{Path: "pkg/a.go", StartLine: 10, EndLine: 20, Language: "go", Content: "before\nmatch\nafter"},
		},
	}, nil
}

type countingRetrieval struct {
	mu    sync.Mutex
	paths []string
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
	r.mu.Unlock()
	results := []retrieval.SearchResult{
		{Path: path + "/a.go", StartLine: 10, EndLine: 20, Language: "go", Content: "before\n" + query + "\nafter"},
	}
	resultCount := 1
	if query == "missing" {
		results = nil
		resultCount = 0
	}
	return &retrieval.SearchResults{
		Path:          path,
		Query:         query,
		ContextLines:  contextLines,
		MaxResults:    maxResults,
		CaseSensitive: caseSensitive,
		ResultCount:   resultCount,
		Results:       results,
	}, nil
}

func (countingRetrieval) GetFileSlice(context.Context, string, string, int, int) (*retrieval.FileSlice, error) {
	return &retrieval.FileSlice{
		Path:      "extra.go",
		StartLine: 1,
		EndLine:   2,
		Content:   "package extra",
		Language:  "go",
	}, nil
}

func (countingRetrieval) GetSymbol(context.Context, string, retrieval.SymbolRef) (*retrieval.SymbolInfo, error) {
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
			Name:      symbol.Name,
			Path:      pathOrDefault(symbol.Path, "pkg/root.go"),
			StartLine: 10,
			EndLine:   12,
			Source:    "func Run() {}",
			Children: []retrieval.CallNode{
				{
					Name:      "Start",
					Path:      "pkg/caller.go",
					StartLine: 20,
					EndLine:   24,
					Source:    "func Start() {}",
				},
			},
		},
	}, nil
}

func (r *countingRetrieval) FindCallees(_ context.Context, _ string, symbol retrieval.SymbolRef, depth int) (*retrieval.CallHierarchy, error) {
	r.mu.Lock()
	r.paths = append(r.paths, fmt.Sprintf("callees:%s:%s:%d", symbol.Path, symbol.Name, depth))
	r.mu.Unlock()
	return &retrieval.CallHierarchy{
		Mode:  "callees",
		Depth: depth,
		Root: retrieval.CallNode{
			Name:      symbol.Name,
			Path:      pathOrDefault(symbol.Path, "pkg/root.go"),
			StartLine: 10,
			EndLine:   12,
			Source:    "func Run() {}",
			Children: []retrieval.CallNode{
				{
					Name:      "Helper",
					Path:      "pkg/callee.go",
					StartLine: 30,
					EndLine:   34,
					Source:    "func Helper() {}",
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
	if want := "`inspect_file` tool"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing tool instructions: %q", req.Messages[0].Content)
	}
	if want := "`list_files` tool"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing list-files instructions: %q", req.Messages[0].Content)
	}
	if want := "`find_callers` tool"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing callers instructions: %q", req.Messages[0].Content)
	}
	if want := "`find_callees` tool"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing callees instructions: %q", req.Messages[0].Content)
	}
	if want := "call multiple tools in parallel"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing parallel guidance: %q", req.Messages[0].Content)
	}
	if want := "generate a `suggestion` block, including `body`, `line_range.start` and `line_range.end`"; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing suggestion instructions: %q", req.Messages[0].Content)
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
	if want := "\"suggestion\""; !strings.Contains(req.Messages[0].Content, want) {
		t.Fatalf("system prompt missing example suggestion JSON: %q", req.Messages[0].Content)
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
	if len(req.Tools) != 5 {
		t.Fatalf("tools = %d", len(req.Tools))
	}
	if req.Tools[0].Name != "inspect_file" {
		t.Fatalf("tool name = %q", req.Tools[0].Name)
	}
	if req.Tools[1].Name != "list_files" {
		t.Fatalf("tool name = %q", req.Tools[1].Name)
	}
	if req.Tools[2].Name != "search" {
		t.Fatalf("tool name = %q", req.Tools[2].Name)
	}
	if req.Tools[3].Name != "find_callers" {
		t.Fatalf("tool name = %q", req.Tools[3].Name)
	}
	if req.Tools[4].Name != "find_callees" {
		t.Fatalf("tool name = %q", req.Tools[4].Name)
	}
	if !req.ParallelToolCalls {
		t.Fatal("parallel tool calls should be enabled by default")
	}
}

func TestEngineCanDisableParallelToolCallsAndGuidance(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, retrieval.NewLocalEngine(), config.Profile{Model: "test"})

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:                     model.ModeLocal,
		MaxContextTokens:         1000,
		DisableParallelToolCalls: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := llmClient.reqs[0]
	if req.ParallelToolCalls {
		t.Fatal("parallel tool calls should be disabled")
	}
	if strings.Contains(req.Messages[0].Content, "call all required tools in the same turn rather than serializing them") {
		t.Fatalf("system prompt should omit parallel guidance: %q", req.Messages[0].Content)
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}
	if result.ToolCalls != 1 {
		t.Fatalf("tool calls = %d", result.ToolCalls)
	}
	if got := result.TokensUsed.TotalTokens; got != 10 {
		t.Fatalf("total tokens = %d", got)
	}

	req := llmClient.reqs[1]
	if len(req.Messages) != 5 {
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
	if req.Messages[4].Role != "user" {
		t.Fatalf("follow-up role = %q", req.Messages[4].Role)
	}
	if want := "You called the following tools already:"; !strings.Contains(req.Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", req.Messages[4].Content)
	}
	if want := "1. inspect_file: tool_call_id=\"call_1\", arguments=[path=\"extra.go\"]; result=[lines=1]"; !strings.Contains(req.Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", req.Messages[4].Content)
	}
	if want := "If you need more context, continue calling tools."; !strings.Contains(req.Messages[4].Content, want) {
		t.Fatalf("follow-up missing continue instruction: %q", req.Messages[4].Content)
	}
	if want := "Otherwise, if you have enough context to judge the patch, stop calling tools and return the final review as JSON."; !strings.Contains(req.Messages[4].Content, want) {
		t.Fatalf("follow-up missing stop instruction: %q", req.Messages[4].Content)
	}
}

func TestEngineExecutesInspectFileToolCallsWithLineRange(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go","line_start":4,"line_end":8}`},
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := llmClient.reqs[1]
	var payload map[string]any
	if err := json.Unmarshal([]byte(req.Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v\n%s", err, req.Messages[3].Content)
	}
	if payload["path"] != "extra.go" || payload["content"] != "package extra" || payload["language"] != "go" {
		t.Fatalf("tool payload = %#v", payload)
	}
	if payload["start_line"] != float64(1) || payload["end_line"] != float64(2) {
		t.Fatalf("tool payload line range = %#v", payload)
	}
	if want := "1. inspect_file: tool_call_id=\"call_1\", arguments=[path=\"extra.go\", line_start=4, line_end=8]; result=[lines=1]"; !strings.Contains(req.Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", req.Messages[4].Content)
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("tool error status = %#v", payload["status"])
	}
	errorPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("tool error payload = %#v", payload)
	}
	if errorPayload["code"] != "missing_argument" {
		t.Fatalf("tool error code = %#v", errorPayload["code"])
	}
	if errorPayload["message"] != "missing required argument: path" {
		t.Fatalf("tool error payload = %#v", payload)
	}
	if llmClient.reqs[1].Messages[4].Role != "user" {
		t.Fatalf("follow-up role = %q", llmClient.reqs[1].Messages[4].Role)
	}
	if want := "1. inspect_file: tool_call_id=\"call_1\", arguments=[path=\"<path>\"]; error=\"missing required argument: path\""; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
	if want := "Please retry the last tool call."; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up missing retry instruction: %q", llmClient.reqs[1].Messages[4].Content)
	}
	if strings.Contains(llmClient.reqs[1].Messages[4].Content, "If you need more context, continue calling tools.") {
		t.Fatalf("follow-up should not include regular continuation instructions after retryable error: %q", llmClient.reqs[1].Messages[4].Content)
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", len(llmClient.reqs))
	}
	if len(llmClient.reqs[2].Tools) != 0 {
		t.Fatalf("final call should have no tools, got %d", len(llmClient.reqs[2].Tools))
	}
	if result.OverallCorrectness != "patch is correct" {
		t.Fatalf("overall_correctness = %q", result.OverallCorrectness)
	}
	if result.ToolCalls != 1 {
		t.Fatalf("tool calls = %d", result.ToolCalls)
	}
}

func TestEngineCountsParallelToolCallsIndividuallyForLimit(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"main.go"}`},
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", len(llmClient.reqs))
	}
	if len(llmClient.reqs[1].Tools) != 0 {
		t.Fatalf("final call should have no tools, got %d", len(llmClient.reqs[1].Tools))
	}
	if result.ToolCalls != 0 {
		t.Fatalf("tool calls = %d", result.ToolCalls)
	}
	if len(retrievalEngine.paths) != 0 {
		t.Fatalf("retrieval should not run when batch exceeds limit: %#v", retrievalEngine.paths)
	}
}

func TestEngineStopsAtDuplicateToolCallLimit(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go"}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"./extra.go"}`},
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
		Mode:                  model.ModeLocal,
		MaxContextTokens:      1000,
		MaxToolCalls:          3,
		MaxDuplicateToolCalls: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", len(llmClient.reqs))
	}
	if len(llmClient.reqs[2].Tools) != 0 {
		t.Fatalf("final call should have no tools, got %d", len(llmClient.reqs[2].Tools))
	}
	if result.OverallCorrectness != "patch is correct" {
		t.Fatalf("overall_correctness = %q", result.OverallCorrectness)
	}
	if result.ToolCalls != 2 {
		t.Fatalf("tool calls = %d", result.ToolCalls)
	}
	if result.DuplicateToolCalls != 1 {
		t.Fatalf("duplicate tool calls = %d", result.DuplicateToolCalls)
	}
	if len(retrievalEngine.paths) != 1 {
		t.Fatalf("retrieval calls = %d", len(retrievalEngine.paths))
	}
}

func TestEngineReturnsErrorForDuplicateFileRequests(t *testing.T) {
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
		MaxToolCalls:     2,
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
	if payload["status"] != "error" {
		t.Fatalf("duplicate tool payload = %#v", payload)
	}
	if payload["path"] != "extra.go" {
		t.Fatalf("duplicate tool path = %#v", payload["path"])
	}
	errorPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("duplicate tool error payload = %#v", payload)
	}
	if errorPayload["code"] != "already_requested" {
		t.Fatalf("duplicate tool error code = %#v", errorPayload["code"])
	}
	if errorPayload["message"] != "file contents were already provided for this review" {
		t.Fatalf("duplicate tool error message = %#v", errorPayload["message"])
	}
	if _, ok := payload["content"]; ok {
		t.Fatalf("duplicate tool payload should omit content: %#v", payload)
	}
	if _, ok := payload["language"]; ok {
		t.Fatalf("duplicate tool payload should omit language: %#v", payload)
	}
	if llmClient.reqs[2].Messages[6].Role != "user" {
		t.Fatalf("duplicate follow-up role = %q", llmClient.reqs[2].Messages[6].Role)
	}
	if want := "1. inspect_file: tool_call_id=\"call_1\", arguments=[path=\"./extra.go\"]; result=[lines=1]"; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("duplicate follow-up missing first tool call = %q", llmClient.reqs[2].Messages[6].Content)
	}
	if want := "2. inspect_file: tool_call_id=\"call_2\", arguments=[path=\"extra.go\"]; error=\"file contents were already provided for this review\""; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("duplicate follow-up content = %q", llmClient.reqs[2].Messages[6].Content)
	}
	if want := "If you need more context, continue calling tools."; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("duplicate follow-up missing regular continuation instructions: %q", llmClient.reqs[2].Messages[6].Content)
	}
	if strings.Contains(llmClient.reqs[2].Messages[6].Content, "Please retry the last tool call.") {
		t.Fatalf("duplicate follow-up should not request retry: %q", llmClient.reqs[2].Messages[6].Content)
	}
}

func TestEngineReturnsErrorForAlreadyCoveredFileRangeRequests(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"extra.go","line_start":1,"line_end":10}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "inspect_file", Arguments: `{"path":"extra.go","line_start":2,"line_end":3}`},
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
		MaxToolCalls:     2,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[2].Messages[5].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("duplicate range tool payload = %#v", payload)
	}
	errorPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("duplicate range error payload = %#v", payload)
	}
	if errorPayload["code"] != "already_requested" {
		t.Fatalf("duplicate range error code = %#v", errorPayload["code"])
	}
	if errorPayload["message"] != "file contents were already provided for this review" {
		t.Fatalf("duplicate range error message = %#v", errorPayload["message"])
	}
	if want := "2. inspect_file: tool_call_id=\"call_2\", arguments=[path=\"extra.go\", line_start=2, line_end=3]; error=\"file contents were already provided for this review\""; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("duplicate range follow-up = %q", llmClient.reqs[2].Messages[6].Content)
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "list:pkg:1" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["path"] != "pkg" {
		t.Fatalf("list payload path = %#v", payload["path"])
	}
	if payload["depth"] != float64(1) {
		t.Fatalf("list payload depth = %#v", payload["depth"])
	}
	files, ok := payload["files"].([]any)
	if !ok || len(files) != 2 {
		t.Fatalf("list payload files = %#v", payload["files"])
	}
	if llmClient.reqs[1].Messages[4].Role != "user" {
		t.Fatalf("follow-up role = %q", llmClient.reqs[1].Messages[4].Role)
	}
	if want := "1. list_files: tool_call_id=\"call_1\", arguments=[path=\"pkg\", depth=1]; result=[files=2]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
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

func TestEngineReturnsErrorForDuplicateListFilesRequests(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{"path":"pkg","depth":1}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "list_files", Arguments: `{"path":"./pkg","depth":1}`},
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
		MaxToolCalls:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "list:pkg:1" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}
	if want := "2. list_files: tool_call_id=\"call_2\", arguments=[path=\"./pkg\", depth=1]; error=\"tool result was already provided for this review\""; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[2].Messages[6].Content)
	}
}

func TestEngineAllowsEmptyPathForListFilesToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{}`},
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "list::1" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["path"] != "" {
		t.Fatalf("list payload path = %#v", payload["path"])
	}
	if payload["depth"] != float64(1) {
		t.Fatalf("list payload depth = %#v", payload["depth"])
	}
	if llmClient.reqs[1].Messages[4].Role != "user" {
		t.Fatalf("follow-up role = %q", llmClient.reqs[1].Messages[4].Role)
	}
	if want := "1. list_files: tool_call_id=\"call_1\", arguments=[path=\".\", depth=1]; result=[files=2]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
}

func TestEngineExecutesCallersToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "find_callers", Arguments: `{"symbol":"Run","path":"pkg","depth":2}`},
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callers:pkg:Run:2" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["symbol"] != "Run" || payload["path"] != "pkg" || payload["mode"] != "callers" {
		t.Fatalf("callers payload = %#v", payload)
	}
	if want := "1. find_callers: tool_call_id=\"call_1\", arguments=[path=\"pkg\", symbol=\"Run\", depth=2]; result=[lines=2, files=2]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
}

func TestEngineReturnsErrorForDuplicateFindCallersRequests(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "find_callers", Arguments: `{"symbol":"Run","path":"pkg","depth":2}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "find_callers", Arguments: `{"symbol":"Run","path":"./pkg","depth":2}`},
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
		MaxToolCalls:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callers:pkg:Run:2" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}
	if want := "2. find_callers: tool_call_id=\"call_2\", arguments=[path=\"./pkg\", symbol=\"Run\", depth=2]; error=\"tool result was already provided for this review\""; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[2].Messages[6].Content)
	}
}

func TestEngineAllowsEmptyPathForCalleesToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "find_callees", Arguments: `{"symbol":"Run"}`},
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callees::Run:10" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["symbol"] != "Run" || payload["path"] != "" || payload["mode"] != "callees" {
		t.Fatalf("callees payload = %#v", payload)
	}
	if want := "1. find_callees: tool_call_id=\"call_1\", arguments=[path=\".\", symbol=\"Run\", depth=10]; result=[lines=2, files=2]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
}

func TestEnginePrintsToolCallsWhenEnabled(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{"path":"pkg","depth":1}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	var buf bytes.Buffer
	logger := logging.New(&buf, false, false)
	logger.SetShowProgress(true)
	engine := NewEngine(stubSource{}, llmClient, &countingRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logger)

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "Tool: list_files(path=\"pkg\", depth=1) → result=[files=2]") {
		t.Fatalf("tool call banner missing: %q", got)
	}
	if strings.Contains(got, `"files": [`) || strings.Contains(got, "pkg/a.go") {
		t.Fatalf("tool call output should omit content payloads: %q", got)
	}
}

func TestEnginePrintsOptimizedSearchReplacementWhenEnabled(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run("}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	var buf bytes.Buffer
	logger := logging.New(&buf, false, false)
	logger.SetShowProgress(true)
	engine := NewEngine(stubSource{}, llmClient, &countingRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logger)

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "Tool: find_callers(instead_of=\"search\", path=\"pkg\", symbol=\"Run\", depth=10) → result=[lines=2, files=2]") {
		t.Fatalf("optimized tool call banner missing: %q", got)
	}
}

func TestEngineDoesNotPrintToolCallsByDefault(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "list_files", Arguments: `{"path":"pkg","depth":1}`},
				},
			},
			{
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
			},
		},
	}
	var buf bytes.Buffer
	logger := logging.New(&buf, false, false)
	engine := NewEngine(stubSource{}, llmClient, &countingRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logger)

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); strings.Contains(got, "Tool call:") {
		t.Fatalf("tool calls should be hidden by default: %q", got)
	}
}

func TestEngineExecutesSearchToolCalls(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"","query":"ttlExtenders","context_lines":5,"max_results":20,"case_sensitive":false}`},
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "search::ttlExtenders:5:20:false" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["query"] != "ttlExtenders" {
		t.Fatalf("search payload query = %#v", payload["query"])
	}
	if payload["context_lines"] != float64(5) {
		t.Fatalf("search payload context_lines = %#v", payload["context_lines"])
	}
	if payload["max_results"] != float64(20) {
		t.Fatalf("search payload max_results = %#v", payload["max_results"])
	}
	if payload["case_sensitive"] != false {
		t.Fatalf("search payload case_sensitive = %#v", payload["case_sensitive"])
	}
	if payload["result_count"] != float64(1) {
		t.Fatalf("search payload result_count = %#v", payload["result_count"])
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("search payload results = %#v", payload["results"])
	}
	firstResult, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("search payload first result = %#v", results[0])
	}
	if firstResult["language"] != "go" {
		t.Fatalf("search payload result language = %#v", firstResult["language"])
	}
	if firstResult["content"] != "before\nttlExtenders\nafter" {
		t.Fatalf("search payload result content = %#v", firstResult["content"])
	}
	if want := "1. search: tool_call_id=\"call_1\", arguments=[path=\".\", query=\"ttlExtenders\", context_lines=5, max_results=20, case_sensitive=false]; result=[files=1, result_count=1]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
}

func TestEngineRewritesSearchFunctionQueryToFindCallers(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run("}`},
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callers:pkg:Run:10" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["mode"] != "callers" || payload["symbol"] != "Run" || payload["path"] != "pkg" {
		t.Fatalf("tool payload = %#v", payload)
	}
}

func TestEngineRewritesSearchMethodQueryToFindCallers(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Close()"}`},
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callers:pkg:Close:10" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].Messages[3].Content), &payload); err != nil {
		t.Fatalf("tool payload should be valid json: %v", err)
	}
	if payload["mode"] != "callers" || payload["symbol"] != "Close" || payload["path"] != "pkg" {
		t.Fatalf("tool payload = %#v", payload)
	}
}

func TestEngineDedupesOptimizedSearchAgainstFindCallers(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run("}`},
				},
			},
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_2", Name: "find_callers", Arguments: `{"path":"./pkg","symbol":"Run","depth":10}`},
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
		MaxToolCalls:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "callers:pkg:Run:10" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}
	if want := "1. search (replaced by find_callers): tool_call_id=\"call_1\", arguments=[path=\"pkg\", query=\"Run(\", context_lines=0, max_results=0, case_sensitive=false]"; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[2].Messages[6].Content)
	}
	if want := "2. find_callers: tool_call_id=\"call_2\", arguments=[path=\"./pkg\", symbol=\"Run\", depth=10]; error=\"tool result was already provided for this review\""; !strings.Contains(llmClient.reqs[2].Messages[6].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[2].Messages[6].Content)
	}
}

func TestEngineReportsZeroSearchResultsExplicitly(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"missing"}`},
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
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if want := "1. search: tool_call_id=\"call_1\", arguments=[path=\"pkg\", query=\"missing\", context_lines=0, max_results=0, case_sensitive=false]; result=[result_count=0]"; !strings.Contains(llmClient.reqs[1].Messages[4].Content, want) {
		t.Fatalf("follow-up content = %q", llmClient.reqs[1].Messages[4].Content)
	}
}

func TestEngineCanDisableSearchToolOptimization(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "search", Arguments: `{"path":"pkg","query":"Run("}`},
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
	engine.SetSearchToolOptimization(false)

	_, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrievalEngine.paths) != 1 || retrievalEngine.paths[0] != "search:pkg:Run(:0:0:false" {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
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
		MaxToolCalls:     0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OverallCorrectness != "patch is correct" {
		t.Fatalf("overall_correctness = %q", result.OverallCorrectness)
	}
	if result.ToolCalls != 2 {
		t.Fatalf("tool calls = %d", result.ToolCalls)
	}
	if len(retrievalEngine.paths) != 2 {
		t.Fatalf("retrieval paths = %#v", retrievalEngine.paths)
	}
}

func intPtr(v int) *int {
	return &v
}

func pathOrDefault(path, fallback string) string {
	if path == "" {
		return fallback
	}
	return path + "/root.go"
}
