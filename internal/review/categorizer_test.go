package review

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

type scriptedCategorizeLLM struct {
	mu        sync.Mutex
	calls     int
	requests  []*llm.ReviewRequest
	responses []*llm.ReviewResponse
	err       error
}

func (s *scriptedCategorizeLLM) Review(_ context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	cloned := *req
	cloned.Messages = cloneTestMessages(req.Messages)
	cloned.NoToolsMessages = cloneTestMessages(req.NoToolsMessages)
	cloned.Tools = append([]llm.ToolDefinition(nil), req.Tools...)
	s.requests = append(s.requests, &cloned)
	if s.err != nil {
		return nil, s.err
	}
	if len(s.responses) > 0 {
		resp := s.responses[0]
		s.responses = s.responses[1:]
		return resp, nil
	}
	return categorized(model.CategoryFinding), nil
}

func categorized(categories ...string) *llm.ReviewResponse {
	return categorizedWithUsage(model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}, categories...)
}

func categorizedWithUsage(usage model.TokenUsage, categories ...string) *llm.ReviewResponse {
	return &llm.ReviewResponse{
		Categorization: &model.FindingCategorization{Categories: categories, Remarks: "classified"},
		TokensUsed:     usage,
	}
}

func sampleFinding(title, path string, start int) model.Finding {
	return model.Finding{
		Title:        title,
		Body:         "body",
		Priority:     intPtr(2),
		CodeLocation: model.CodeLocation{FilePath: path, LineRange: model.LineRange{Start: start, End: start}},
	}
}

func TestCategorizeIsBlindAndToolFree(t *testing.T) {
	client := &scriptedCategorizeLLM{}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	reviewCtx := sampleReviewCtx()
	reviewCtx.DiffScopeHunks = []model.DiffHunk{{FilePath: "main.go", OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1}}
	reviewCtx.ToolchainVersions = []model.ToolchainVersion{{Language: "go", Version: "1.25"}}

	finding := sampleFinding("runtime bug", "main.go", 1)
	finding.Verification = &model.FindingVerification{Verdict: model.VerdictRefuted}
	finding.Finalization = &model.FindingFinalization{Remarks: "downstream outcome"}
	finding.MergedFrom = []string{"prior-id"}
	_, _, err := engine.Categorize(context.Background(), CategorizeRequest{
		ReviewCtx: reviewCtx,
		Finding:   finding,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := client.requests[0]
	if len(req.Tools) != 0 || req.ParallelToolCalls {
		t.Fatalf("categorizer tools = %v parallel=%v", req.Tools, req.ParallelToolCalls)
	}
	system := req.Messages[0].Content
	for _, banned := range []string{
		"outside-diff-scope", "replacement_code_location", "verify_drop_policy",
		"proceed", "eligible", "secret style rule", "VERDICT DECISION ORDER",
	} {
		if strings.Contains(system, banned) {
			t.Fatalf("categorizer prompt contains %q:\n%s", banned, system)
		}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(taskMessageContent(req)), &payload); err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"diff_hunks", "diff_scope_hunks", "style_guides", "finding_diff_scope"} {
		if _, ok := payload[banned]; ok {
			t.Fatalf("categorizer payload contains %q: %#v", banned, payload)
		}
	}
	if _, ok := payload["finding"]; !ok {
		t.Fatalf("payload missing finding: %#v", payload)
	}
	if _, ok := payload["toolchain_versions"]; !ok {
		t.Fatalf("payload missing toolchain versions: %#v", payload)
	}
	findingPayload, ok := payload["finding"].(map[string]any)
	if !ok {
		t.Fatalf("finding payload = %#v", payload["finding"])
	}
	for _, banned := range []string{"verification", "finalization", "summarization", "merged_from"} {
		if _, ok := findingPayload[banned]; ok {
			t.Fatalf("categorizer finding contains downstream outcome %q: %#v", banned, findingPayload)
		}
	}
}

func TestCategorizeAllAttachesByIndex(t *testing.T) {
	client := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{
		categorized(model.CategoryFinding),
		categorized(model.CategoryConfirmation),
		categorized(model.CategoryCompilation, model.CategoryFinding),
	}}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	findings := []model.Finding{
		sampleFinding("a", "a.go", 1),
		sampleFinding("b", "b.go", 2),
		sampleFinding("c", "c.go", 3),
	}
	got, usage, warnings, err := engine.CategorizeAll(context.Background(), sampleReviewCtx(), findings, CategorizeOptions{Limiter: NewLimiter(1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(got) != 3 {
		t.Fatalf("got=%d warnings=%v", len(got), warnings)
	}
	want := [][]string{{model.CategoryFinding}, {model.CategoryConfirmation}, {model.CategoryCompilation, model.CategoryFinding}}
	for i := range want {
		if !slices.Equal(got[i].Categories, want[i]) {
			t.Fatalf("result %d categories = %v, want %v", i, got[i].Categories, want[i])
		}
	}
	if usage.TotalTokens != 6 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestCategorizeAllAggregatesVariableConcurrentUsage(t *testing.T) {
	client := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{
		categorizedWithUsage(model.TokenUsage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}, model.CategoryFinding),
		categorizedWithUsage(model.TokenUsage{PromptTokens: 7, CompletionTokens: 11, TotalTokens: 18}, model.CategoryFinding),
		categorizedWithUsage(model.TokenUsage{PromptTokens: 13, CompletionTokens: 17, TotalTokens: 30}, model.CategoryFinding),
	}}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	findings := []model.Finding{
		sampleFinding("a", "a.go", 1),
		sampleFinding("b", "b.go", 2),
		sampleFinding("c", "c.go", 3),
	}

	_, usage, warnings, err := engine.CategorizeAll(context.Background(), sampleReviewCtx(), findings, CategorizeOptions{Limiter: NewLimiter(3)})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	want := (model.TokenUsage{PromptTokens: 22, CompletionTokens: 31, TotalTokens: 53})
	if usage != want {
		t.Fatalf("usage = %+v, want %+v", usage, want)
	}
}

func TestCategorizePromptUsesLanguageSpecificUnusedIdentifierMapping(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{language: "go", want: "default toolchain reports unused\n  imports or variables"},
		{language: "typescript", want: "Do not assume unused imports or variables"},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			client := &scriptedCategorizeLLM{}
			engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
			finding := sampleFinding("unused local", "file.ts", 1)
			finding.CodeLocation.Language = tt.language
			if _, _, err := engine.Categorize(context.Background(), CategorizeRequest{
				ReviewCtx: sampleReviewCtx(),
				Finding:   finding,
			}); err != nil {
				t.Fatal(err)
			}
			if system := client.requests[0].Messages[0].Content; !strings.Contains(system, tt.want) {
				t.Fatalf("system prompt missing %q:\n%s", tt.want, system)
			}
		})
	}
}

func TestShouldDropCategoriesPreservesPolicySemantics(t *testing.T) {
	tests := []struct {
		name       string
		categories []string
		policy     string
		want       bool
	}{
		{"finding none", []string{model.CategoryFinding}, model.DropPolicyNone, false},
		{"finding default", []string{model.CategoryFinding}, model.DropPolicyRefutedOnly, false},
		{"confirmation none", []string{model.CategoryConfirmation}, model.DropPolicyNone, false},
		{"confirmation default", []string{model.CategoryConfirmation}, model.DropPolicyRefutedOnly, true},
		{"compilation default", []string{model.CategoryCompilation}, model.DropPolicyRefutedOnly, true},
		{"mixed default", []string{model.CategoryCompilation, model.CategoryFinding}, model.DropPolicyRefutedOnly, false},
		{"mixed strict", []string{model.CategoryCompilation, model.CategoryFinding}, model.DropPolicyRefutedAndUnverified, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := shouldDropCategories(tc.categories, tc.policy)
			if got != tc.want {
				t.Fatalf("shouldDropCategories(%v, %q) = %v, want %v", tc.categories, tc.policy, got, tc.want)
			}
		})
	}
}

func TestCategorizeFailureFailsOpenToVerification(t *testing.T) {
	client := &scriptedCategorizeLLM{err: errors.New("classification unavailable")}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: []model.Finding{sampleFinding("real bug", "main.go", 1)}},
		run:  model.AgentRun{Name: "Reviewer", Role: "review"},
	}}
	_, warnings, err := engine.categorizeAndFilterVectorFindings(
		context.Background(), sampleReviewCtx(), vectorResults,
		model.ReviewRequest{VerifyDropPolicy: model.DropPolicyRefutedOnly}, NewLimiter(1), "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(vectorResults[0].resp.Findings) != 1 {
		t.Fatal("categorization failure dropped finding")
	}
}

func TestCategorizeRetriesMissingCategorization(t *testing.T) {
	client := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{
		{RawResponse: "not a categorization", TokensUsed: model.TokenUsage{TotalTokens: 2}},
		categorized(model.CategoryConfirmation),
	}}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	got, usage, err := engine.Categorize(context.Background(), CategorizeRequest{
		ReviewCtx:        sampleReviewCtx(),
		Finding:          sampleFinding("x", "main.go", 1),
		MaxOutputRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !slices.Equal(got.Categories, []string{model.CategoryConfirmation}) {
		t.Fatalf("categorization = %#v", got)
	}
	if client.calls != 2 || usage.TotalTokens != 4 {
		t.Fatalf("calls=%d usage=%+v", client.calls, usage)
	}
}
