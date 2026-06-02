package review

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

type recordingMultiAgentLLM struct {
	mu             sync.Mutex
	context        int
	vectorCalls    map[string]int
	contextPayload map[string]any
}

func (s *recordingMultiAgentLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vectorCalls == nil {
		s.vectorCalls = make(map[string]int)
	}
	system := ""
	if len(req.Messages) > 0 {
		system = req.Messages[0].Content
	}
	if strings.Contains(system, "DO NOT produce review findings yourself") {
		s.context++
		if s.context == 1 && s.contextPayload == nil && len(req.Messages) > 1 {
			_ = json.Unmarshal([]byte(req.Messages[1].Content), &s.contextPayload)
		}
		return &llm.ReviewResponse{
			RawResponse: "context done\n\n## Assumed Patch Purpose\nAssumption.",
		}, nil
	}
	if strings.Contains(system, "## FOCUS ON ") {
		name := vectorNameFromSystem(system)
		s.vectorCalls[name]++
		return &llm.ReviewResponse{
			Findings:               []model.Finding{},
			OverallCorrectness:     "patch is correct",
			OverallExplanation:     name,
			OverallConfidenceScore: 0.5,
		}, nil
	}
	return &llm.ReviewResponse{
		Findings:               []model.Finding{},
		OverallCorrectness:     "patch is correct",
		OverallExplanation:     "merged",
		OverallConfidenceScore: 0.5,
	}, nil
}

func TestEngineIncludesToolchainVersionsInContextPayload(t *testing.T) {
	llmClient := &recordingMultiAgentLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetToolchainCapture(func(_ context.Context, repoRoot string, _ *model.ReviewContext) []model.ToolchainVersion {
		if repoRoot != "/some/repo" {
			t.Errorf("repoRoot passed to capture = %q", repoRoot)
		}
		return []model.ToolchainVersion{
			{Language: "go", Source: "go.mod", Field: "go", Version: "1.22.0"},
			{Language: "python", Unavailable: true},
		}
	})

	_, enriched, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         "/some/repo",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if llmClient.contextPayload == nil {
		t.Fatal("context payload was not captured")
	}
	raw, ok := llmClient.contextPayload["toolchain_versions"]
	if !ok {
		t.Fatalf("context payload missing toolchain_versions: %#v", llmClient.contextPayload)
	}
	entries, ok := raw.([]any)
	if !ok || len(entries) != 2 {
		t.Fatalf("toolchain_versions = %#v", raw)
	}
	first := entries[0].(map[string]any)
	if first["language"] != "go" || first["source"] != "go.mod" || first["field"] != "go" {
		t.Fatalf("first entry = %#v", first)
	}
	if first["version"] != "1.22.0" {
		t.Fatalf("first version = %#v", first["version"])
	}
	second := entries[1].(map[string]any)
	if second["language"] != "python" || second["unavailable"] != true {
		t.Fatalf("second entry = %#v", second)
	}
	if enriched == nil || len(enriched.ToolchainVersions) != 2 {
		t.Fatalf("enriched context toolchain_versions = %#v", enriched)
	}
}

func TestEngineSkipsToolchainCaptureWhenNil(t *testing.T) {
	llmClient := &capturingLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetToolchainCapture(nil)

	_, _, err := runReviewPipeline(engine, context.Background(), model.ReviewRequest{
		Mode:             model.ModeLocal,
		MaxContextTokens: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(llmClient.reqs) == 0 {
		t.Fatal("no requests captured")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.reqs[0].Messages[1].Content), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["toolchain_versions"]; ok {
		t.Fatalf("toolchain_versions should be omitted when capture disabled: %#v", payload)
	}
}
