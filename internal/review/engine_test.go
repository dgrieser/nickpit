package review

import (
	"context"
	"path/filepath"
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
			{Severity: model.SeverityInfo, Category: "style", FilePath: "main.go", Title: "info", Description: "low", Confidence: 0.5},
			{Severity: model.SeverityError, Category: "bug", FilePath: "main.go", Title: "error", Description: "high", Confidence: 0.9},
		},
		Summary: "summary",
	}, nil
}

func TestEngineSeverityFilter(t *testing.T) {
	engine := NewEngine(stubSource{}, stubLLM{}, retrieval.NewLocalEngine(), config.Profile{Model: "test"})
	engine.promptDir = filepath.Join("..", "..", "prompts")
	result, err := engine.Run(context.Background(), model.ReviewRequest{
		Mode:              model.ModeLocal,
		MaxContextTokens:  1000,
		SeverityThreshold: "error",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %d", len(result.Findings))
	}
}
