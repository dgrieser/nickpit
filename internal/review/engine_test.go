package review

import (
	"context"
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
			{Title: "[P3] info", Body: "low", ConfidenceScore: 0.5, Priority: intPtr(3), CodeLocation: model.CodeLocation{AbsoluteFilePath: "/tmp/main.go", LineRange: model.LineRange{Start: 1, End: 1}}},
			{Title: "[P1] error", Body: "high", ConfidenceScore: 0.9, Priority: intPtr(1), CodeLocation: model.CodeLocation{AbsoluteFilePath: "/tmp/main.go", LineRange: model.LineRange{Start: 2, End: 2}}},
		},
		OverallCorrectness:     "patch is incorrect",
		OverallExplanation:     "summary",
		OverallConfidenceScore: 0.9,
	}, nil
}

type capturingLLM struct {
	reqs []*llm.ReviewRequest
}

func (s *capturingLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.reqs = append(s.reqs, req)
	return &llm.ReviewResponse{
		OverallCorrectness:     "patch is correct",
		OverallExplanation:     "summary",
		OverallConfidenceScore: 0.5,
	}, nil
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
	if want := "You are acting as a reviewer for a proposed code change made by another engineer."; !strings.Contains(req.SystemPrompt, want) {
		t.Fatalf("system prompt = %q", req.SystemPrompt)
	}
	if contains := "Repository: repo"; !strings.Contains(req.UserContent, contains) {
		t.Fatalf("user prompt missing %q: %q", contains, req.UserContent)
	}
}

func intPtr(v int) *int {
	return &v
}
