package review

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

type scriptedVerifyLLM struct {
	mu        sync.Mutex
	calls     int
	requests  []*llm.ReviewRequest
	responses []*llm.ReviewResponse
	err       error
}

func (s *scriptedVerifyLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.requests = append(s.requests, req)
	if s.err != nil {
		return nil, s.err
	}
	if req.SchemaKind != llm.SchemaKindVerify {
		return nil, errors.New("expected verify schema kind")
	}
	if len(s.responses) == 0 {
		return &llm.ReviewResponse{
			Verification: &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.5, Remarks: "ok"},
			TokensUsed:   model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}, nil
	}
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
}

func sampleReviewCtx() *model.ReviewContext {
	return &model.ReviewContext{
		Mode:       model.ModeLocal,
		Repository: model.RepositoryInfo{FullName: "repo"},
		Title:      "title",
		ChangedFiles: []model.ChangedFile{
			{Path: "main.go", Status: model.FileModified, Additions: 1},
		},
		Diff: "diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n",
	}
}

func TestVerifyAllAttachesByIndex(t *testing.T) {
	llmClient := &scriptedVerifyLLM{
		responses: []*llm.ReviewResponse{
			{
				Verification: &model.FindingVerification{Valid: true, Priority: 1, ConfidenceScore: 0.9, Remarks: "real bug"},
				TokensUsed:   model.TokenUsage{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3},
			},
			{
				Verification: &model.FindingVerification{Valid: false, Priority: 3, ConfidenceScore: 0.85, Remarks: "not reachable"},
				TokensUsed:   model.TokenUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	findings := []model.Finding{
		{Title: "first", Body: "b1", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}},
		{Title: "second", Body: "b2", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}},
	}
	verifications, usage, err := engine.VerifyAll(context.Background(), sampleReviewCtx(), findings, VerifyOptions{Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(verifications) != 2 {
		t.Fatalf("verifications = %d, want 2", len(verifications))
	}
	if !verifications[0].Valid || verifications[0].Remarks != "real bug" {
		t.Fatalf("verifications[0] = %#v", verifications[0])
	}
	if verifications[1].Valid || verifications[1].Remarks != "not reachable" {
		t.Fatalf("verifications[1] = %#v", verifications[1])
	}
	if usage.TotalTokens != 6 {
		t.Fatalf("usage total = %d, want 6", usage.TotalTokens)
	}
}

func TestVerifyAllErrorsBecomeFallbackVerifications(t *testing.T) {
	llmClient := &scriptedVerifyLLM{err: errors.New("upstream fail")}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	findings := []model.Finding{
		{Title: "x", Body: "x", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}},
	}
	verifications, _, err := engine.VerifyAll(context.Background(), sampleReviewCtx(), findings, VerifyOptions{Concurrency: 1})
	if err != nil {
		t.Fatalf("VerifyAll returned err: %v", err)
	}
	if len(verifications) != 1 {
		t.Fatalf("verifications len = %d", len(verifications))
	}
	if verifications[0] != nil {
		t.Fatalf("verification = %#v, want nil", verifications[0])
	}
}

func TestVerifyIncludesStyleGuides(t *testing.T) {
	llmClient := &scriptedVerifyLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	finding := model.Finding{
		Title:        "x",
		Body:         "x",
		Priority:     intPtr(1),
		CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
	}
	_, _, err := engine.Verify(context.Background(), VerifyRequest{ReviewCtx: sampleReviewCtx(), Finding: finding})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}
	if len(llmClient.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(llmClient.requests))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.requests[0].Messages[1].Content), &payload); err != nil {
		t.Fatalf("unmarshal user prompt: %v", err)
	}
	styleGuides, ok := payload["style_guides"].([]any)
	if !ok || len(styleGuides) != 1 {
		t.Fatalf("style guides = %#v", payload["style_guides"])
	}
	goStyleGuide := styleGuides[0].(map[string]any)
	if goStyleGuide["language"] != "go" {
		t.Fatalf("style guide language = %#v", goStyleGuide["language"])
	}
	if content, _ := goStyleGuide["content"].(string); !strings.Contains(content, "# Go Style Guide") {
		t.Fatalf("style guide content = %.80q", content)
	}
}

func TestVerifyIncludesSuggestions(t *testing.T) {
	llmClient := &scriptedVerifyLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	finding := model.Finding{
		Title:        "x",
		Body:         "x",
		Priority:     intPtr(1),
		CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
		Suggestions: []model.Suggestion{
			{Body: "replacement one", LineRange: model.LineRange{Start: 1, End: 1}},
			{Body: "replacement two", LineRange: model.LineRange{Start: 2, End: 3}},
		},
	}
	_, _, err := engine.Verify(context.Background(), VerifyRequest{ReviewCtx: sampleReviewCtx(), Finding: finding})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.requests[0].Messages[1].Content), &payload); err != nil {
		t.Fatalf("unmarshal user prompt: %v", err)
	}
	verifyFinding := payload["finding"].(map[string]any)
	suggestions, ok := verifyFinding["suggestions"].([]any)
	if !ok || len(suggestions) != 2 {
		t.Fatalf("suggestions = %#v", verifyFinding["suggestions"])
	}
	if first := suggestions[0].(map[string]any); first["body"] != "replacement one" {
		t.Fatalf("first suggestion = %#v", first)
	}
}
