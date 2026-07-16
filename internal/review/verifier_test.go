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

func TestVerifyAddsInlineExampleForBothJSONResponseModes(t *testing.T) {
	for _, disableJSONResponseFormat := range []bool{false, true} {
		llmClient := &scriptedVerifyLLM{}
		engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
		reviewCtx := sampleReviewCtx()
		reviewCtx.DiffScopeHunks = []model.DiffHunk{{FilePath: "main.go", OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1}}
		_, _, err := engine.Verify(context.Background(), VerifyRequest{
			ReviewCtx:                 reviewCtx,
			Finding:                   model.Finding{Title: "x", Body: "x", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}},
			DisableJSONResponseFormat: disableJSONResponseFormat,
		})
		if err != nil {
			t.Fatalf("Verify(disableJSONResponseFormat=%v) returned err: %v", disableJSONResponseFormat, err)
		}
		if len(llmClient.requests) != 1 {
			t.Fatalf("requests = %d, want 1", len(llmClient.requests))
		}
		messages := llmClient.requests[0].Messages
		if len(messages) != 2 {
			t.Fatalf("messages = %#v, want system/user", messages)
		}
		if !strings.Contains(messages[0].Content, strings.TrimSpace(llm.ScopedVerifyExamplePromptSnippet())) {
			t.Fatalf("system prompt missing verify example:\n%s", messages[0].Content)
		}
		if messages[1].Role != "user" || !strings.Contains(messages[1].Content, `"finding"`) {
			t.Fatalf("task message = %#v", messages[1])
		}
	}
}

func TestVerifyWithoutDiffScopeHunksUsesLegacyPromptAndSchema(t *testing.T) {
	llmClient := &scriptedVerifyLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	_, _, err := engine.Verify(context.Background(), VerifyRequest{
		ReviewCtx: sampleReviewCtx(),
		Finding:   model.Finding{Title: "x", Body: "x", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := llmClient.requests[0]
	if string(req.Schema) != string(llm.VerifySchema) {
		t.Fatalf("schema = %s, want legacy verify schema", req.Schema)
	}
	if req.Constraints.RequireReplacementCodeLocation {
		t.Fatal("source-less verifier unexpectedly requires replacement_code_location")
	}
	for _, messages := range [][]llm.Message{req.Messages, req.NoToolsMessages} {
		system := messages[0].Content
		if strings.Contains(system, "Diff-scope gate") || strings.Contains(system, "replacement_code_location") {
			t.Fatalf("source-less verifier prompt contains diff-scope instructions:\n%s", system)
		}
		if !strings.Contains(system, strings.TrimSpace(llm.VerifyExamplePromptSnippet())) {
			t.Fatalf("source-less verifier prompt missing legacy example:\n%s", system)
		}
	}
}

func TestVerifyDisableDiffScopeRestoresLegacyPromptAndSchema(t *testing.T) {
	llmClient := &scriptedVerifyLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	reviewCtx := sampleReviewCtx()
	reviewCtx.DiffScopeHunks = []model.DiffHunk{{FilePath: "main.go", OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1}}
	finding := model.Finding{Title: "x", Body: "x", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "other.go", LineRange: model.LineRange{Start: 9, End: 9}}}
	_, _, err := engine.Verify(context.Background(), VerifyRequest{
		ReviewCtx:        reviewCtx,
		Finding:          finding,
		DisableDiffScope: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := llmClient.requests[0]
	if string(req.Schema) != string(llm.VerifySchema) {
		t.Fatalf("schema = %s, want legacy verify schema", req.Schema)
	}
	for _, messages := range [][]llm.Message{req.Messages, req.NoToolsMessages} {
		if len(messages) == 0 {
			t.Fatal("missing verifier prompt messages")
		}
		system := messages[0].Content
		if strings.Contains(system, "Diff-scope gate") || strings.Contains(system, "replacement_code_location") {
			t.Fatalf("legacy verifier prompt contains diff-scope instructions:\n%s", system)
		}
		if !strings.Contains(system, strings.TrimSpace(llm.VerifyExamplePromptSnippet())) {
			t.Fatalf("legacy verifier prompt missing legacy example:\n%s", system)
		}
		assertVerifierGateNumbering(t, system, []string{
			"1. Non-finding gate:",
			"2. Styleguide contradiction gate:",
			"3. Confirm gate:",
			"4. Refute gate for actual issue claims:",
			"5. Unverified gate:",
		})
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(taskMessageContent(req)), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["finding_diff_scope"]; ok {
		t.Fatalf("legacy payload includes finding_diff_scope: %#v", payload)
	}
}

func TestVerifyAnnotatesDeterministicDiffScopeStatus(t *testing.T) {
	llmClient := &scriptedVerifyLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	reviewCtx := sampleReviewCtx()
	reviewCtx.DiffScopeHunks = []model.DiffHunk{{FilePath: "main.go", OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1}}
	finding := model.Finding{Title: "x", Body: "x", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "other.go", LineRange: model.LineRange{Start: 9, End: 9}}}
	_, _, err := engine.Verify(context.Background(), VerifyRequest{ReviewCtx: reviewCtx, Finding: finding})
	if err != nil {
		t.Fatal(err)
	}
	req := llmClient.requests[0]
	if string(req.Schema) != string(llm.ScopedVerifySchema) {
		t.Fatalf("schema = %s, want scoped verify schema", req.Schema)
	}
	if len(req.NoToolsMessages) == 0 || !strings.Contains(req.NoToolsMessages[0].Content, "Diff-scope gate") ||
		!strings.Contains(req.NoToolsMessages[0].Content, strings.TrimSpace(llm.ScopedVerifyExamplePromptSnippet())) {
		t.Fatalf("no-tools verifier prompt missing scoped guidance: %#v", req.NoToolsMessages)
	}
	assertVerifierGateNumbering(t, req.Messages[0].Content, []string{
		"1. Non-finding gate:",
		"2. Diff-scope gate:",
		"3. Styleguide contradiction gate:",
		"4. Confirm gate:",
		"5. Refute gate for actual issue claims:",
		"6. Unverified gate:",
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(taskMessageContent(req)), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["finding_diff_scope"] != "outside_diff" {
		t.Fatalf("finding_diff_scope = %#v", payload["finding_diff_scope"])
	}
}

func assertVerifierGateNumbering(t *testing.T, prompt string, headings []string) {
	t.Helper()
	for _, heading := range headings {
		if !strings.Contains(prompt, heading) {
			t.Errorf("verifier prompt missing numbered heading %q:\n%s", heading, prompt)
		}
	}
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

func assertFallbackUnverified(t *testing.T, v *model.FindingVerification, priority int) {
	t.Helper()
	if v == nil {
		t.Fatal("verification = nil, want fallback unverified verification")
	}
	if _, err := uuid.Parse(v.ID); err != nil {
		t.Fatalf("verification ID = %q, want valid UUID", v.ID)
	}
	if v.Verdict != model.VerdictUnverified || v.Priority != priority || v.ConfidenceScore != 0 || v.Remarks != "" {
		t.Fatalf("verification = %#v, want unverified priority %d confidence 0 with empty remarks", v, priority)
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
	assertFallbackUnverified(t, verifications[0], 1)
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
	for _, v := range verifications {
		assertFallbackUnverified(t, v, 1)
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
	if err := json.Unmarshal([]byte(taskMessageContent(llmClient.requests[0])), &payload); err != nil {
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

func TestVerifyAndFilterDowngradesLowConfidenceRefuted(t *testing.T) {
	reviewerFindings := []model.Finding{
		{Title: "low refuted", Body: "b", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}},
		{Title: "high refuted", Body: "b", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "b.go", LineRange: model.LineRange{Start: 2, End: 2}}},
		{Title: "unverified", Body: "b", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "c.go", LineRange: model.LineRange{Start: 3, End: 3}}},
	}
	llmClient := &scriptedVerifyLLM{responses: []*llm.ReviewResponse{
		{Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 2, ConfidenceScore: 0.69, Remarks: "contradicted"}},
		{Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 2, ConfidenceScore: 0.7, Remarks: "drop"}},
		{Verification: &model.FindingVerification{Verdict: model.VerdictUnverified, Priority: 2, ConfidenceScore: 0.69, Remarks: "uncertain"}},
	}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: reviewerFindings},
		run:  model.AgentRun{Name: "Reviewer 1", Role: "review", Status: model.AgentRunStatusOK},
	}}
	req := model.ReviewRequest{VerifyDropPolicy: DropPolicyRefutedOnly}
	_, _, err := engine.verifyAndFilterVectorFindings(context.Background(), sampleReviewCtx(), vectorResults, req, NewLimiter(1), "")
	if err != nil {
		t.Fatalf("verifyAndFilterVectorFindings returned err: %v", err)
	}

	kept := vectorResults[0].resp.Findings
	if len(kept) != 1 {
		t.Fatalf("kept findings = %d, want 1", len(kept))
	}
	if kept[0].Title != "unverified" || kept[0].Verification == nil ||
		kept[0].Verification.Verdict != model.VerdictUnverified || kept[0].Verification.Remarks != "uncertain" {
		t.Fatalf("kept unverified = %#v, want unchanged unverified", kept[0])
	}
}

func TestVerifyAndFilterKeepsVerifierFailuresAsUnverified(t *testing.T) {
	reviewerFindings := []model.Finding{
		{Title: "timeout", Body: "b", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "a.go", LineRange: model.LineRange{Start: 1, End: 1}}},
	}
	llmClient := &scriptedVerifyLLM{err: errors.New("context deadline exceeded")}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})

	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: reviewerFindings},
		run:  model.AgentRun{Name: "Reviewer 1", Role: "review", Status: model.AgentRunStatusOK},
	}}
	req := model.ReviewRequest{VerifyDropPolicy: DropPolicyRefutedOnly}
	_, warnings, err := engine.verifyAndFilterVectorFindings(context.Background(), sampleReviewCtx(), vectorResults, req, NewLimiter(1), "")
	if err != nil {
		t.Fatalf("verifyAndFilterVectorFindings returned err: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Verify failed") || !strings.Contains(warnings[0], "context deadline exceeded") {
		t.Fatalf("warnings = %#v, want verifier failure warning", warnings)
	}
	kept := vectorResults[0].resp.Findings
	if len(kept) != 1 {
		t.Fatalf("kept findings = %d, want 1", len(kept))
	}
	assertFallbackUnverified(t, kept[0].Verification, 2)
	if kept[0].Verification.ID != kept[0].ID {
		t.Fatalf("verification ID = %q, finding ID = %q, want matching IDs", kept[0].Verification.ID, kept[0].ID)
	}
}

func TestVerifyAndFilterRelocatesOrDropsWhollyOutOfScopeFindings(t *testing.T) {
	reviewCtx := sampleReviewCtx()
	reviewCtx.DiffScopeHunks = []model.DiffHunk{{
		FilePath: "f.go",
		OldStart: 10,
		OldLines: 3,
		NewStart: 10,
		NewLines: 2,
	}}
	findings := []model.Finding{
		{Title: "already scoped", Body: "b", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 10, End: 10}}},
		{Title: "relocate", Body: "b", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 100, End: 100}}},
		{Title: "deleted anchor", Body: "b", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 12, End: 12}}},
		{Title: "drop", Body: "b", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "other.go", LineRange: model.LineRange{Start: 1, End: 1}}},
	}
	confirmed := func() *model.FindingVerification {
		return &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.9, Remarks: "confirmed"}
	}
	llmClient := &scriptedVerifyLLM{responses: []*llm.ReviewResponse{
		{Verification: confirmed(), ReplacementCodeLocation: &model.CodeLocation{FilePath: "other.go", LineRange: model.LineRange{Start: 1, End: 1}}},
		{Verification: confirmed(), ReplacementCodeLocation: &model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 11, End: 11}, Content: "changed"}},
		{Verification: confirmed()},
		{Verification: confirmed()},
	}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: findings},
		run:  model.AgentRun{Name: "Reviewer 1", Role: "review"},
	}}
	req := model.ReviewRequest{VerifyDropPolicy: DropPolicyNone}
	_, _, err := engine.verifyAndFilterVectorFindings(context.Background(), reviewCtx, vectorResults, req, NewLimiter(1), "")
	if err != nil {
		t.Fatal(err)
	}

	kept := vectorResults[0].resp.Findings
	if len(kept) != 3 {
		t.Fatalf("kept findings = %#v, want three", kept)
	}
	if kept[0].Title != "already scoped" || kept[0].CodeLocation.LineRange.Start != 10 {
		t.Fatalf("valid original location changed: %#v", kept[0])
	}
	if kept[1].Title != "relocate" || kept[1].CodeLocation.LineRange != (model.LineRange{Start: 11, End: 11, Count: 1}) {
		t.Fatalf("relocated finding = %#v", kept[1])
	}
	if kept[2].Title != "deleted anchor" || kept[2].CodeLocation.LineRange.Start != 12 {
		t.Fatalf("deleted anchor not preserved: %#v", kept[2])
	}
}

func TestVerifyAndFilterDisableDiffScopeKeepsOutsideFinding(t *testing.T) {
	reviewCtx := sampleReviewCtx()
	reviewCtx.DiffScopeHunks = []model.DiffHunk{{FilePath: "main.go", OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1}}
	finding := model.Finding{Title: "outside", Body: "b", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "other.go", LineRange: model.LineRange{Start: 99, End: 99}}}
	llmClient := &scriptedVerifyLLM{responses: []*llm.ReviewResponse{{
		Verification:            &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 2, ConfidenceScore: 0.9, Remarks: "confirmed"},
		ReplacementCodeLocation: &model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}, Content: "changed"},
	}}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	vectorResults := []agentResult{{resp: &llm.ReviewResponse{Findings: []model.Finding{finding}}, run: model.AgentRun{Name: "Reviewer 1", Role: "review"}}}
	req := model.ReviewRequest{DisableDiffScope: true, VerifyDropPolicy: DropPolicyNone}
	_, _, err := engine.verifyAndFilterVectorFindings(context.Background(), reviewCtx, vectorResults, req, NewLimiter(1), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(vectorResults[0].resp.Findings) != 1 {
		t.Fatalf("outside finding dropped with --disable-diff-scope: %#v", vectorResults[0].resp.Findings)
	}
	if got := vectorResults[0].resp.Findings[0].CodeLocation.FilePath; got != "other.go" {
		t.Fatalf("outside finding relocated with --disable-diff-scope: %q", got)
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
		ReviewCtx:    sampleReviewCtx(),
		Finding:      finding,
		MaxToolCalls: 2,
		RepoRoot:     "/repo",
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

// TestVerifySystemPromptHasNonFindingRule pins the detection guidance: the
// verifier is told to return non-finding "findings" (affirmations / "no issue"
// items) as `refuted` so the code demotes or drops them (never blocking).
func TestVerifySystemPromptHasNonFindingRule(t *testing.T) {
	llmClient := &scriptedVerifyLLM{
		responses: []*llm.ReviewResponse{
			{Verification: &model.FindingVerification{Verdict: model.VerdictRefuted, Priority: 3, ConfidenceScore: 0.0, Remarks: "no issue"}},
		},
	}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	_, _, err := engine.Verify(context.Background(), VerifyRequest{
		ReviewCtx: sampleReviewCtx(),
		Finding:   model.Finding{Title: "No issue", Body: "x", Priority: intPtr(3), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}},
	})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}
	sysPrompt := llmClient.requests[0].Messages[0].Content
	for _, want := range []string{
		"Non-finding gate",
		"Judge the finding AS A WHOLE",
		"no issue",
		"Do NOT verify whether the positive statement is true",
		"Never use the phrase \"no issue\" anywhere in `remarks` except",
		"When `refuted` because the input is a non-finding",
		"often contain phrases similar to these",
	} {
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

// TestVerifyOuterRetryReusesToolDedupState pins that the outer
// missing-verification retry loop shares one agent-loop state: a retry attempt
// repeating an earlier attempt's tool call must hit the duplicate detector
// instead of re-fetching the same file with a fresh budget.
func TestVerifyOuterRetryReusesToolDedupState(t *testing.T) {
	llmClient := &scriptedVerifyLLM{
		responses: []*llm.ReviewResponse{
			// Attempt 1: fetch main.go, then finish without a verification.
			{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "inspect_file", Arguments: `{"path":"main.go"}`}}},
			{RawResponse: "still no verification"},
			// Attempt 2 (outer retry): repeats the same fetch — must dedupe.
			{ToolCalls: []llm.ToolCall{{ID: "c2", Name: "inspect_file", Arguments: `{"path":"main.go"}`}}},
			{Verification: &model.FindingVerification{Verdict: model.VerdictConfirmed, Priority: 1, ConfidenceScore: 0.9, Remarks: "confirmed"}},
		},
	}
	counting := &countingRetrieval{}
	engine := NewEngine(stubSource{}, llmClient, counting, config.Profile{Model: "test"})

	verification, _, err := engine.Verify(context.Background(), VerifyRequest{
		ReviewCtx:        sampleReviewCtx(),
		Finding:          model.Finding{Title: "x", Body: "x", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}},
		MaxToolCalls:     4,
		MaxOutputRetries: 2,
		RepoRoot:         "/repo",
	})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}
	if verification == nil || verification.Remarks != "confirmed" {
		t.Fatalf("verification = %#v", verification)
	}
	fullReads := 0
	for _, p := range counting.paths {
		if p == "main.go" {
			fullReads++
		}
	}
	if fullReads != 1 {
		t.Fatalf("GetFile(main.go) calls = %d (%v), want 1: retry must reuse the tool-dedup state", fullReads, counting.paths)
	}
	if len(llmClient.requests) != 4 {
		t.Fatalf("llm calls = %d, want 4", len(llmClient.requests))
	}
	// The deduped repeat must surface as the standard already_requested error.
	lastMessages := llmClient.requests[3].Messages
	dupSeen := false
	for _, msg := range lastMessages {
		if msg.Role == "tool" && msg.ToolCallID == "c2" && strings.Contains(msg.Content, "already_requested") {
			dupSeen = true
		}
	}
	if !dupSeen {
		t.Fatalf("expected already_requested tool result for repeated call, messages = %#v", lastMessages)
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
	if err := json.Unmarshal([]byte(taskMessageContent(llmClient.requests[0])), &payload); err != nil {
		t.Fatalf("unmarshal user prompt: %v", err)
	}
	if _, ok := payload["style_guides"]; ok {
		t.Fatalf("user prompt should not include style_guides: %#v", payload["style_guides"])
	}
	system := llmClient.requests[0].Messages[0].Content
	if !strings.Contains(system, "## STYLEGUIDES") {
		t.Fatalf("verify system prompt missing styleguide section: %q", system)
	}
	if !strings.Contains(system, "Treat styleguides as verification evidence") || !strings.Contains(system, "They are rules to follow") {
		t.Fatalf("verify system prompt missing styleguide evidence rule: %q", system)
	}
	if !strings.Contains(system, "Styleguide contradiction gate") || !strings.Contains(system, "Do NOT confirm a plausible-sounding finding when it conflicts with a styleguide") {
		t.Fatalf("verify system prompt missing styleguide contradiction gate: %q", system)
	}
	if !strings.Contains(system, "# Go — Common Developer Guideline") || strings.Contains(system, "### Go — Common Developer Guideline (go)") {
		t.Fatalf("verify system prompt missing Go styleguide content: %q", system)
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
	if err := json.Unmarshal([]byte(taskMessageContent(llmClient.requests[0])), &payload); err != nil {
		t.Fatalf("unmarshal user prompt: %v", err)
	}
	if _, ok := payload["style_guides"]; ok {
		t.Fatalf("user prompt should not include style_guides: %#v", payload["style_guides"])
	}
	system := llmClient.requests[0].Messages[0].Content
	if !strings.Contains(system, "# Kubernetes Style Guide") || strings.Contains(system, "### Kubernetes Style Guide (kubernetes)") {
		t.Fatalf("verify system prompt missing Kubernetes styleguide content: %q", system)
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
	if err := json.Unmarshal([]byte(taskMessageContent(llmClient.requests[0])), &payload); err != nil {
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

func TestVerifyDisableSuggestionsOmitsSuggestions(t *testing.T) {
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
		},
	}
	_, _, err := engine.Verify(context.Background(), VerifyRequest{ReviewCtx: sampleReviewCtx(), Finding: finding, DisableSuggestions: true})
	if err != nil {
		t.Fatalf("Verify returned err: %v", err)
	}

	var payload map[string]any
	userPrompt := taskMessageContent(llmClient.requests[0])
	if err := json.Unmarshal([]byte(userPrompt), &payload); err != nil {
		t.Fatalf("unmarshal user prompt: %v", err)
	}
	verifyFinding := payload["finding"].(map[string]any)
	if _, ok := verifyFinding["suggestions"]; ok {
		t.Fatalf("suggestions should be omitted from verify payload: %#v", verifyFinding)
	}
	if strings.Contains(userPrompt, "replacement one") {
		t.Fatalf("verify user prompt should not contain suggestion text:\n%s", userPrompt)
	}
}
