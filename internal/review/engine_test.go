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
	reqs []*llm.ReviewRequest
	resps []*llm.ReviewResponse
}

func (s *capturingLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.reqs = append(s.reqs, req)
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
	if req.SystemPrompt == req.UserContent {
		t.Fatal("system and user prompts should differ")
	}
	if req.SystemPrompt == "" || req.UserContent == "" {
		t.Fatal("system and user prompts should both be populated")
	}
	if want := "You are acting as a senior engineer performing a thorough code review for a proposed code change made by another engineer."; !strings.Contains(req.SystemPrompt, want) {
		t.Fatalf("system prompt = %q", req.SystemPrompt)
	}
	if want := "Make sure to output the findings in the following JSON format:"; !strings.Contains(req.SystemPrompt, want) {
		t.Fatalf("system prompt missing example JSON instructions: %q", req.SystemPrompt)
	}
	if want := "\"overall_correctness\": \"patch is correct\""; !strings.Contains(req.SystemPrompt, want) {
		t.Fatalf("system prompt missing rendered example JSON: %q", req.SystemPrompt)
	}
	if want := "\"title\": \"[P1] Example title\""; !strings.Contains(req.SystemPrompt, want) {
		t.Fatalf("system prompt missing example finding JSON: %q", req.SystemPrompt)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(req.UserContent), &payload); err != nil {
		t.Fatalf("user prompt should be valid json: %v\n%s", err, req.UserContent)
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

func TestEngineFollowUpPromptUsesJSONInspectPayload(t *testing.T) {
	llmClient := &capturingLLM{
		resps: []*llm.ReviewResponse{
			{
				FollowUpRequests: []model.FollowUpRequest{
					{Type: "file", Path: "extra.go", Reason: "need more context"},
				},
				OverallCorrectness:     "patch is correct",
				OverallExplanation:     "summary",
				OverallConfidenceScore: 0.5,
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
		FollowUpRounds:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) != 2 {
		t.Fatalf("requests = %d", len(llmClient.reqs))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[1].UserContent), &payload); err != nil {
		t.Fatalf("follow-up prompt should be valid json: %v\n%s", err, llmClient.reqs[1].UserContent)
	}
	if _, ok := payload["follow_up_requests"]; !ok {
		t.Fatalf("follow-up prompt missing follow_up_requests: %#v", payload)
	}
	items, ok := payload["supplemental_context"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("supplemental_context = %#v", payload["supplemental_context"])
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("supplemental item = %#v", items[0])
	}
	if item["path"] != "extra.go" || item["content"] != "package extra" || item["language"] != "go" {
		t.Fatalf("supplemental item = %#v", item)
	}
	for _, unwanted := range []string{"reason", "kind"} {
		if _, exists := item[unwanted]; exists {
			t.Fatalf("supplemental item unexpectedly contains %q: %#v", unwanted, item[unwanted])
		}
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
	if strings.Contains(llmClient.reqs[0].SystemPrompt, "Example JSON output:") {
		t.Fatalf("system prompt should omit example snippet when API schema is enabled: %q", llmClient.reqs[0].SystemPrompt)
	}
}

func intPtr(v int) *int {
	return &v
}
