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
	"github.com/google/uuid"
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
	cloned := *req
	if len(req.Messages) > 0 {
		cloned.Messages = cloneTestMessages(req.Messages)
	}
	if len(req.NoToolsMessages) > 0 {
		cloned.NoToolsMessages = cloneTestMessages(req.NoToolsMessages)
	}
	if len(req.Tools) > 0 {
		cloned.Tools = append([]llm.ToolDefinition(nil), req.Tools...)
	}
	s.requests = append(s.requests, &cloned)
	if s.err != nil {
		return nil, s.err
	}
	if req.SchemaKind != llm.SchemaKindVerify {
		return nil, errors.New("expected verify schema kind")
	}
	if len(s.responses) == 0 {
		return &llm.ReviewResponse{
			Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.5, Remarks: "ok"},
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
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.9, Remarks: "real bug"},
				TokensUsed:   model.TokenUsage{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3},
			},
			{
				Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 3, ConfidenceScore: 0.85, Remarks: "not reachable"},
				TokensUsed:   model.TokenUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	findings := []model.Finding{
		{Title: "first", Body: "b1", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}},
		{Title: "second", Body: "b2", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}},
	}
	verifications, usage, warnings, err := engine.VerifyAll(context.Background(), sampleReviewCtx(), findings, VerifyOptions{Limiter: NewLimiter(1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(verifications) != 2 {
		t.Fatalf("verifications = %d, want 2", len(verifications))
	}
	if verifications[0].Verdict != model.VerdictConfirmed || verifications[0].Remarks != "real bug" {
		t.Fatalf("verifications[0] = %#v", verifications[0])
	}
	if verifications[1].Verdict != model.VerdictRefuted || verifications[1].Remarks != "not reachable" {
		t.Fatalf("verifications[1] = %#v", verifications[1])
	}
	if usage.TotalTokens != 6 {
		t.Fatalf("usage total = %d, want 6", usage.TotalTokens)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
}

func TestVerifyAllErrorsBecomeFallbackVerifications(t *testing.T) {
	llmClient := &scriptedVerifyLLM{err: errors.New("upstream fail")}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	findings := []model.Finding{
		{Title: "x", Body: "x", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}},
	}
	verifications, _, warnings, err := engine.VerifyAll(context.Background(), sampleReviewCtx(), findings, VerifyOptions{Limiter: NewLimiter(1)})
	if err != nil {
		t.Fatalf("VerifyAll returned err: %v", err)
	}
	if len(verifications) != 1 {
		t.Fatalf("verifications len = %d", len(verifications))
	}
	if verifications[0] != nil {
		t.Fatalf("verification = %#v, want nil", verifications[0])
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want 1", warnings)
	}
	if !strings.Contains(warnings[0], "Verify failed") || !strings.Contains(warnings[0], "upstream fail") {
		t.Fatalf("warning content = %q", warnings[0])
	}
}

func TestVerifyAllCancelledContextWarnsOnceAndStops(t *testing.T) {
	llmClient := &scriptedVerifyLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	findings := []model.Finding{
		{Title: "a", Body: "b", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}},
		{Title: "b", Body: "b", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}},
		{Title: "c", Body: "b", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "c.go", LineRange: model.LineRange{Start: 3, End: 3}}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	verifications, _, warnings, err := engine.VerifyAll(ctx, sampleReviewCtx(), findings, VerifyOptions{Limiter: NewLimiter(1)})
	if err != nil {
		t.Fatalf("VerifyAll returned err: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want exactly one aggregate cancellation warning", warnings)
	}
	if !strings.Contains(warnings[0], "skipped 3 remaining") {
		t.Fatalf("warning content = %q", warnings[0])
	}
	for i, v := range verifications {
		if v != nil {
			t.Fatalf("verifications[%d] = %#v, want nil", i, v)
		}
	}
	if llmClient.calls != 0 {
		t.Fatalf("LLM calls = %d, want 0 after cancellation", llmClient.calls)
	}
}

func TestVerifyAllDoesNotMutateInputFindings(t *testing.T) {
	llmClient := &scriptedVerifyLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	findings := []model.Finding{
		{ID: "not-a-uuid", Title: "x", Body: "x", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}},
	}
	_, _, _, err := engine.VerifyAll(context.Background(), sampleReviewCtx(), findings, VerifyOptions{Limiter: NewLimiter(1)})
	if err != nil {
		t.Fatalf("VerifyAll returned err: %v", err)
	}
	if findings[0].ID != "not-a-uuid" {
		t.Fatalf("input ID mutated to %q", findings[0].ID)
	}
	if len(llmClient.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(llmClient.requests))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.requests[0].Messages[1].Content), &payload); err != nil {
		t.Fatalf("unmarshal user prompt: %v", err)
	}
	verifyFinding := payload["finding"].(map[string]any)
	if verifyFinding["id"] == "not-a-uuid" || strings.TrimSpace(verifyFinding["id"].(string)) == "" {
		t.Fatalf("verify prompt finding id = %#v, want generated copy ID", verifyFinding["id"])
	}
}

func TestVerifyAndFilterPropagatesCorrectedIDs(t *testing.T) {
	// One reviewer emits an invalid non-empty ID and a pair of duplicate valid
	// IDs. EnsureFindingIDs normalizes them before verification; the corrected
	// IDs must survive onto the kept findings and stay in sync with their
	// verifications.
	dupID := uuid.NewString()
	reviewerFindings := []model.Finding{
		{ID: "not-a-uuid", Title: "invalid id", Body: "b", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}},
		{ID: dupID, Title: "dup one", Body: "b", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}},
		{ID: dupID, Title: "dup two", Body: "b", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "c.go", LineRange: model.LineRange{Start: 3, End: 3}}},
	}

	llmClient := &scriptedVerifyLLM{} // default verdict is confirmed, so nothing is dropped.
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: reviewerFindings},
		run:  model.AgentRun{Name: "Reviewer 1", Role: "review", Status: model.AgentRunStatusOK},
	}}

	_, _, err := engine.verifyAndFilterVectorFindings(context.Background(), sampleReviewCtx(), vectorResults, model.ReviewRequest{}, NewLimiter(0), "")
	if err != nil {
		t.Fatalf("verifyAndFilterVectorFindings returned err: %v", err)
	}

	kept := vectorResults[0].resp.Findings
	if len(kept) != len(reviewerFindings) {
		t.Fatalf("kept findings = %d, want %d", len(kept), len(reviewerFindings))
	}
	seen := make(map[string]struct{}, len(kept))
	for i, f := range kept {
		if _, parseErr := uuid.Parse(f.ID); parseErr != nil {
			t.Fatalf("kept[%d].ID = %q, want valid UUID", i, f.ID)
		}
		if _, dup := seen[f.ID]; dup {
			t.Fatalf("kept[%d].ID = %q is duplicated across kept findings", i, f.ID)
		}
		seen[f.ID] = struct{}{}
		if f.Verification == nil {
			t.Fatalf("kept[%d].Verification = nil, want attached verification", i)
		}
		if f.Verification.ID != f.ID {
			t.Fatalf("kept[%d].Verification.ID = %q, want %q (finding ID)", i, f.Verification.ID, f.ID)
		}
	}
}

func TestVerifyExecutesToolCallsThroughAgentLoop(t *testing.T) {
	llmClient := &scriptedVerifyLLM{
		responses: []*llm.ReviewResponse{
			{
				ToolCalls: []llm.ToolCall{{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"main.go"}`}},
				TokensUsed: model.TokenUsage{
					PromptTokens:     1,
					CompletionTokens: 1,
					TotalTokens:      2,
				},
			},
			{
				Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.9, Remarks: "confirmed"},
				TokensUsed: model.TokenUsage{
					PromptTokens:     3,
					CompletionTokens: 2,
					TotalTokens:      5,
				},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	finding := model.Finding{
		Title:        "x",
		Body:         "x",
		Priority:     intPtr(1),
		CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
	}

	verification, usage, err := engine.Verify(context.Background(), VerifyRequest{
		ReviewCtx:     sampleReviewCtx(),
		Finding:       finding,
		MaxToolCalls:  2,
		RepoRoot:      "/repo",
		UseJSONSchema: true,
	})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}
	if verification == nil || verification.Remarks != "confirmed" {
		t.Fatalf("verification = %#v", verification)
	}
	if usage.TotalTokens != 7 {
		t.Fatalf("usage total = %d, want 7", usage.TotalTokens)
	}
	if len(llmClient.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(llmClient.requests))
	}
	systemPrompt := llmClient.requests[0].Messages[0].Content
	if want := "`inspect_file` tool"; !strings.Contains(systemPrompt, want) {
		t.Fatalf("system prompt missing tool instructions: %q", systemPrompt)
	}
	if want := "Only make additional tool calls, if the provided context is really insufficient."; !strings.Contains(systemPrompt, want) {
		t.Fatalf("system prompt missing verifier tool guidance: %q", systemPrompt)
	}
	if llmClient.requests[0].SchemaKind != llm.SchemaKindVerify {
		t.Fatalf("first schema kind = %v", llmClient.requests[0].SchemaKind)
	}
	secondMessages := llmClient.requests[1].Messages
	if len(secondMessages) < 4 {
		t.Fatalf("second request messages = %d, want tool loop history", len(secondMessages))
	}
	if secondMessages[2].Role != "assistant" || len(secondMessages[2].ToolCalls) != 1 {
		t.Fatalf("assistant tool call message = %#v", secondMessages[2])
	}
	if secondMessages[3].Role != "tool" || !strings.Contains(secondMessages[3].Content, `"content":"package extra"`) {
		t.Fatalf("tool response message = %#v", secondMessages[3])
	}
	if last := secondMessages[len(secondMessages)-1]; last.Role != "user" || !strings.Contains(last.Content, "You used the following tools up to now") {
		t.Fatalf("synthetic followup = %#v", last)
	}
}

// TestVerifySystemPromptHasNonFindingRule pins the Layer-1 detection guidance:
// the verifier is told to refute affirmation/non-finding "findings" with 0.0
// confidence so they are demoted (never blocking) downstream.
func TestVerifySystemPromptHasNonFindingRule(t *testing.T) {
	llmClient := &scriptedVerifyLLM{
		responses: []*llm.ReviewResponse{
			{Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 3, ConfidenceScore: 0.0, Remarks: "non-finding"}},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	_, _, err := engine.Verify(context.Background(), VerifyRequest{
		ReviewCtx:     sampleReviewCtx(),
		Finding:       model.Finding{Title: "No issue", Body: "x", Priority: intPtr(3), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}},
		UseJSONSchema: true,
	})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}
	sysPrompt := llmClient.requests[0].Messages[0].Content
	for _, want := range []string{"non-findings", "is NOT a valid finding", "`confidence_score` to `0.0`"} {
		if !strings.Contains(sysPrompt, want) {
			t.Fatalf("verify system prompt missing %q:\n%s", want, sysPrompt)
		}
	}
}

func TestVerifyRetriesMissingVerification(t *testing.T) {
	llmClient := &scriptedVerifyLLM{
		responses: []*llm.ReviewResponse{
			{
				RawResponse: "not verification",
				TokensUsed:  model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
			},
			{
				Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 3, ConfidenceScore: 0.9, Remarks: "not valid"},
				TokensUsed:   model.TokenUsage{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3},
			},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	finding := model.Finding{
		Title:        "x",
		Body:         "x",
		Priority:     intPtr(1),
		CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
	}

	verification, usage, err := engine.Verify(context.Background(), VerifyRequest{
		ReviewCtx:        sampleReviewCtx(),
		Finding:          finding,
		MaxOutputRetries: defaultMaxOutputRetries,
	})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}
	if verification == nil || verification.Verdict != model.VerdictRefuted || verification.Remarks != "not valid" {
		t.Fatalf("verification = %#v", verification)
	}
	if usage.TotalTokens != 5 {
		t.Fatalf("usage total = %d, want 5", usage.TotalTokens)
	}
	if len(llmClient.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(llmClient.requests))
	}
	if len(llmClient.requests[1].Messages) != len(llmClient.requests[0].Messages)+1 {
		t.Fatalf("retry should preserve regular assistant history: first=%d second=%d", len(llmClient.requests[0].Messages), len(llmClient.requests[1].Messages))
	}
	last := llmClient.requests[1].Messages[len(llmClient.requests[1].Messages)-1]
	if last.Role != "assistant" || last.Content != "not verification" {
		t.Fatalf("retry assistant history = %#v", last)
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
	system := llmClient.requests[0].Messages[0].Content
	if !strings.Contains(system, "When validating findings, check the provided styleguides:") {
		t.Fatalf("verify system prompt missing styleguide reminder: %q", system)
	}
	if !strings.Contains(system, "- Go Style Guide") {
		t.Fatalf("verify system prompt missing Go styleguide title: %q", system)
	}
}

func TestVerifyIncludesKubernetesStyleGuide(t *testing.T) {
	llmClient := &scriptedVerifyLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	reviewCtx := sampleReviewCtx()
	reviewCtx.ChangedFiles = []model.ChangedFile{
		{Path: "k8s/deployment.yaml", Status: model.FileModified, Additions: 1},
	}
	reviewCtx.DiffHunks = []model.DiffHunk{
		{FilePath: "k8s/deployment.yaml", Language: "yaml", Content: "+apiVersion: apps/v1\n+kind: Deployment\n"},
	}
	finding := model.Finding{
		Title:        "x",
		Body:         "x",
		Priority:     intPtr(1),
		CodeLocation: model.CodeLocation{FilePath: "k8s/deployment.yaml", LineRange: model.LineRange{Start: 1, End: 1}},
	}
	_, _, err := engine.Verify(context.Background(), VerifyRequest{ReviewCtx: reviewCtx, Finding: finding})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(llmClient.requests[0].Messages[1].Content), &payload); err != nil {
		t.Fatalf("unmarshal user prompt: %v", err)
	}
	styleGuides := payload["style_guides"].([]any)
	if len(styleGuides) != 1 {
		t.Fatalf("style guides = %#v", payload["style_guides"])
	}
	kubernetesStyleGuide := styleGuides[0].(map[string]any)
	if kubernetesStyleGuide["language"] != "kubernetes" {
		t.Fatalf("style guide language = %#v", kubernetesStyleGuide["language"])
	}
	if content, _ := kubernetesStyleGuide["content"].(string); !strings.Contains(content, "# Kubernetes Style Guide") {
		t.Fatalf("style guide content = %.80q", content)
	}
	system := llmClient.requests[0].Messages[0].Content
	if !strings.Contains(system, "- Kubernetes Style Guide") {
		t.Fatalf("verify system prompt missing Kubernetes styleguide title: %q", system)
	}
}

func TestVerifyIncludesToolchainReminder(t *testing.T) {
	llmClient := &scriptedVerifyLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	reviewCtx := sampleReviewCtx()
	reviewCtx.ToolchainVersions = []model.ToolchainVersion{{Language: "go", Source: "go.mod", Field: "go", Version: "1.22"}}
	finding := model.Finding{
		Title:        "x",
		Body:         "x",
		Priority:     intPtr(1),
		CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
	}
	_, _, err := engine.Verify(context.Background(), VerifyRequest{ReviewCtx: reviewCtx, Finding: finding})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}
	if len(llmClient.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(llmClient.requests))
	}
	system := llmClient.requests[0].Messages[0].Content
	if !strings.Contains(system, "provided `toolchain_versions`") {
		t.Fatalf("verify system prompt missing toolchain reminder: %q", system)
	}
}

func TestVerifyFallsBackVerificationIDToFindingID(t *testing.T) {
	const findingID = "11111111-1111-4111-8111-111111111111"
	llmClient := &scriptedVerifyLLM{
		responses: []*llm.ReviewResponse{{
			Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.9, Remarks: "ok"},
		}},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	finding := model.Finding{
		ID:           findingID,
		Title:        "x",
		Body:         "x",
		Priority:     intPtr(1),
		CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
	}
	verification, _, err := engine.Verify(context.Background(), VerifyRequest{ReviewCtx: sampleReviewCtx(), Finding: finding})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}
	if verification.ID != findingID {
		t.Fatalf("verification.ID = %q, want %q", verification.ID, findingID)
	}
}

func TestVerifyPreservesValidVerificationIDFromLLM(t *testing.T) {
	const findingID = "11111111-1111-4111-8111-111111111111"
	const llmID = "22222222-2222-4222-8222-222222222222"
	llmClient := &scriptedVerifyLLM{
		responses: []*llm.ReviewResponse{{
			Verification: &model.FindingVerification{ID: llmID, Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.9, Remarks: "ok"},
		}},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	finding := model.Finding{
		ID:           findingID,
		Title:        "x",
		Body:         "x",
		Priority:     intPtr(1),
		CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}},
	}
	verification, _, err := engine.Verify(context.Background(), VerifyRequest{ReviewCtx: sampleReviewCtx(), Finding: finding})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}
	if verification.ID != llmID {
		t.Fatalf("verification.ID = %q, want %q", verification.ID, llmID)
	}
}

func TestVerifyIncludesSuggestions(t *testing.T) {
	llmClient := &scriptedVerifyLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	finding := model.Finding{
		ID:           "11111111-1111-4111-8111-111111111111",
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
	if verifyFinding["id"] != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("finding id = %#v", verifyFinding["id"])
	}
	suggestions, ok := verifyFinding["suggestions"].([]any)
	if !ok || len(suggestions) != 2 {
		t.Fatalf("suggestions = %#v", verifyFinding["suggestions"])
	}
	if first := suggestions[0].(map[string]any); first["body"] != "replacement one" {
		t.Fatalf("first suggestion = %#v", first)
	}
}
