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
	if req.SchemaKind != llm.SchemaKindCategorize {
		return nil, errors.New("expected categorize schema kind")
	}
	if len(s.responses) == 0 {
		return &llm.ReviewResponse{
			Categorization: &model.FindingCategorization{Categories: []string{model.CategoryFinding}, Remarks: "ok"},
			TokensUsed:     model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		}, nil
	}
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
}

func categorized(categories ...string) *llm.ReviewResponse {
	return &llm.ReviewResponse{
		Categorization: &model.FindingCategorization{Categories: categories, Remarks: "r"},
	}
}

func sampleFinding(title, path string, start int) model.Finding {
	return model.Finding{
		Title:        title,
		Body:         "b",
		Priority:     intPtr(2),
		CodeLocation: model.CodeLocation{FilePath: path, LineRange: model.LineRange{Start: start, End: start}},
	}
}

func scopedReviewCtx() *model.ReviewContext {
	ctx := sampleReviewCtx()
	ctx.DiffScopeHunks = []model.DiffHunk{{FilePath: "f.go", OldStart: 10, OldLines: 3, NewStart: 10, NewLines: 2}}
	return ctx
}

func TestCategorizeAnnotatesDeterministicDiffScopeStatus(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	finding := sampleFinding("x", "other.go", 9)
	_, _, err := engine.Categorize(context.Background(), CategorizeRequest{ReviewCtx: scopedReviewCtx(), Finding: finding})
	if err != nil {
		t.Fatal(err)
	}
	req := llmClient.requests[0]
	if string(req.Schema) != string(llm.ScopedCategorizeSchema) {
		t.Fatalf("schema = %s, want scoped categorize schema", req.Schema)
	}
	if !req.Constraints.RequireReplacementCodeLocation {
		t.Fatal("scoped categorize must require replacement_code_location")
	}
	for name, messages := range map[string][]llm.Message{"tools": req.Messages, "no_tools": req.NoToolsMessages} {
		if len(messages) == 0 {
			t.Fatalf("missing %s categorize prompt", name)
		}
		system := messages[0].Content
		if !strings.Contains(system, "`outside-diff-scope`") || !strings.Contains(system, "replacement_code_location") {
			t.Fatalf("%s categorize prompt missing scoped guidance:\n%s", name, system)
		}
		if !strings.Contains(system, strings.TrimSpace(llm.ScopedCategorizeExamplePromptSnippet())) {
			t.Fatalf("%s categorize prompt missing scoped example:\n%s", name, system)
		}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(taskMessageContent(req)), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["finding_diff_scope"] != "outside_diff" {
		t.Fatalf("finding_diff_scope = %#v", payload["finding_diff_scope"])
	}
}

func TestCategorizeWithoutDiffScopeHunksUsesUnscopedPromptAndSchema(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	_, _, err := engine.Categorize(context.Background(), CategorizeRequest{
		ReviewCtx: sampleReviewCtx(),
		Finding:   sampleFinding("x", "main.go", 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	req := llmClient.requests[0]
	if string(req.Schema) != string(llm.CategorizeSchema) {
		t.Fatalf("schema = %s, want unscoped categorize schema", req.Schema)
	}
	if req.Constraints.RequireReplacementCodeLocation {
		t.Fatal("source-less categorize unexpectedly requires replacement_code_location")
	}
	for name, messages := range map[string][]llm.Message{"tools": req.Messages, "no_tools": req.NoToolsMessages} {
		system := messages[0].Content
		if strings.Contains(system, "`outside-diff-scope`") || strings.Contains(system, "replacement_code_location") {
			t.Fatalf("%s categorize prompt contains diff-scope instructions:\n%s", name, system)
		}
		if !strings.Contains(system, strings.TrimSpace(llm.CategorizeExamplePromptSnippet())) {
			t.Fatalf("%s categorize prompt missing example:\n%s", name, system)
		}
	}
}

func TestCategorizeDisableDiffScopeUsesUnscopedPromptAndSchema(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	_, _, err := engine.Categorize(context.Background(), CategorizeRequest{
		ReviewCtx:        scopedReviewCtx(),
		Finding:          sampleFinding("x", "other.go", 9),
		DisableDiffScope: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := llmClient.requests[0]
	if string(req.Schema) != string(llm.CategorizeSchema) {
		t.Fatalf("schema = %s, want unscoped categorize schema", req.Schema)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(taskMessageContent(req)), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["finding_diff_scope"]; ok {
		t.Fatalf("payload includes finding_diff_scope with --disable-diff-scope: %#v", payload)
	}
}

// The categorize prompt must classify, never verify. Styleguides are evidence
// for the verifier, so injecting them here would invite the agent to start
// judging whether the claim is true.
func TestCategorizeSystemPromptOmitsStyleguidesAndVerdicts(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	_, _, err := engine.Categorize(context.Background(), CategorizeRequest{
		ReviewCtx: sampleReviewCtx(),
		Finding:   sampleFinding("x", "main.go", 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	system := llmClient.requests[0].Messages[0].Content
	for _, banned := range []string{"## STYLEGUIDES", "VERDICT DECISION ORDER", `"verdict"`, "confidence_score"} {
		if strings.Contains(system, banned) {
			t.Errorf("categorize prompt must not contain %q:\n%s", banned, system)
		}
	}
	for _, want := range []string{
		"You classify what KIND of item was submitted.",
		"`categories` must never be empty",
		"DO NOT call tools to judge whether the finding's claim is true",
	} {
		if !strings.Contains(system, want) {
			t.Errorf("categorize prompt missing %q:\n%s", want, system)
		}
	}
}

// The confirmation category inherits the old non-finding gate's guidance
// verbatim; these pins move with it.
func TestCategorizeSystemPromptHasConfirmationRule(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	_, _, err := engine.Categorize(context.Background(), CategorizeRequest{
		ReviewCtx: sampleReviewCtx(),
		Finding:   sampleFinding("No issue", "main.go", 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	system := llmClient.requests[0].Messages[0].Content
	for _, want := range []string{
		"`confirmation`",
		"Judge the finding AS A WHOLE",
		"Identify the finding's FINAL CONCLUSION",
		"DO NOT check whether the positive statement is true",
		"often contain phrases similar to these",
		"request optional hardening, extra tests, compatibility, cleanup, or optimization",
		"uncovered changed behavior",
		"claim performance cost without evidence",
	} {
		if !strings.Contains(system, want) {
			t.Fatalf("categorize prompt missing %q:\n%s", want, system)
		}
	}
}

// The compilation category renders only the unused-identifier kinds the
// finding's language reports through its default toolchain (per
// unused_identifier_diagnostics in languages.yaml); elsewhere those are ordinary
// lint findings and the bullet is omitted.
func TestCategorizeSystemPromptFlagsUnusedIdentifierFindings(t *testing.T) {
	tests := []struct {
		name       string
		filePath   string
		wantBullet string
	}{
		{name: "go renders imports and variables", filePath: "main.go", wantBullet: "findings alleging unused imports or variables that the default toolchain reports"},
		{name: "rust renders imports and variables", filePath: "lib.rs", wantBullet: "findings alleging unused imports or variables that the default toolchain reports"},
		{name: "csharp renders variables only", filePath: "Program.cs", wantBullet: "findings alleging unused variables that the default toolchain reports"},
		{name: "python omits bullet", filePath: "script.py"},
		{name: "typescript omits bullet", filePath: "app.ts"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			llmClient := &scriptedCategorizeLLM{}
			engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
			_, _, err := engine.Categorize(context.Background(), CategorizeRequest{
				ReviewCtx: sampleReviewCtx(),
				Finding: model.Finding{
					Title:        "Remove unused import",
					Body:         "Remove the unused import to maintain code cleanliness and avoid lint errors.",
					Priority:     intPtr(3),
					CodeLocation: model.CodeLocation{FilePath: tc.filePath, LineRange: model.LineRange{Start: 1, End: 1}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, messages := range [][]llm.Message{llmClient.requests[0].Messages, llmClient.requests[0].NoToolsMessages} {
				system := messages[0].Content
				if tc.wantBullet == "" {
					if strings.Contains(system, "findings alleging unused") {
						t.Fatalf("unused-identifier bullet unexpectedly present:\n%s", system)
					}
					continue
				}
				// Stop each pin short of a line wrap so reflowing the prompt does
				// not fail the test on its own.
				for _, want := range []string{
					tc.wantBullet,
					"calling a compiler-reported problem a lint error or maintainability issue",
					"it does NOT ask whether the compiler allegation is technically accurate",
					"Do NOT assign it to errors that only surface at runtime",
				} {
					if !strings.Contains(system, want) {
						t.Fatalf("categorize prompt missing %q:\n%s", want, system)
					}
				}
			}
		})
	}
}

func TestCategorizeAllAttachesByIndex(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{
		categorized(model.CategoryFinding),
		categorized(model.CategoryConfirmation),
		categorized(model.CategoryCompilation, model.CategoryFinding),
	}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	findings := []model.Finding{
		sampleFinding("first", "a.go", 1),
		sampleFinding("second", "b.go", 1),
		sampleFinding("third", "c.go", 1),
	}
	model.EnsureFindingIDs(findings)
	got, _, warnings, err := engine.CategorizeAll(context.Background(), sampleReviewCtx(), findings, CategorizeOptions{Limiter: NewLimiter(1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	want := [][]string{
		{model.CategoryFinding},
		{model.CategoryConfirmation},
		{model.CategoryCompilation, model.CategoryFinding},
	}
	for i := range want {
		if got[i] == nil {
			t.Fatalf("categorization %d nil", i)
		}
		if strings.Join(got[i].Categories, ",") != strings.Join(want[i], ",") {
			t.Fatalf("categorization %d = %v, want %v", i, got[i].Categories, want[i])
		}
		if got[i].ID != findings[i].ID {
			t.Fatalf("categorization %d id = %q, finding id = %q", i, got[i].ID, findings[i].ID)
		}
	}
}

func TestCategorizeAllCancelledContextWarnsOnceAndStops(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	findings := []model.Finding{
		sampleFinding("first", "a.go", 1),
		sampleFinding("second", "b.go", 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, _, warnings, err := engine.CategorizeAll(ctx, sampleReviewCtx(), findings, CategorizeOptions{Limiter: NewLimiter(1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Categorize cancelled") {
		t.Fatalf("warnings = %#v, want one cancellation warning", warnings)
	}
	// Every finding still gets a fail-open categorization so nothing is lost.
	for i := range got {
		if got[i] == nil || model.VerdictForCategories(got[i].Categories) != model.VerdictConfirmed {
			t.Fatalf("categorization %d = %#v, want fail-open [finding]", i, got[i])
		}
	}
}

// A categorize agent that never returns a usable answer must not cost the run a
// finding: after its retries the finding is treated as a plain finding and
// continues to verification.
func TestCategorizeExhaustedRetriesKeepsFinding(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{
		{Categorization: &model.FindingCategorization{Categories: nil, Remarks: "unsure"}},
		{Categorization: &model.FindingCategorization{Categories: []string{}, Remarks: "still unsure"}},
	}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	findings := []model.Finding{sampleFinding("real bug", "a.go", 1)}
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: findings},
		run:  model.AgentRun{Name: "Reviewer 1", Role: "review"},
	}}
	_, warnings, err := engine.categorizeAndFilterVectorFindings(context.Background(), sampleReviewCtx(), vectorResults, model.ReviewRequest{MaxOutputRetries: 1}, NewLimiter(1), "")
	if err != nil {
		t.Fatal(err)
	}
	if llmClient.calls < 2 {
		t.Fatalf("calls = %d, want the empty categories retried", llmClient.calls)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Categorize failed") {
		t.Fatalf("warnings = %#v, want one categorize failure warning", warnings)
	}
	if kept := vectorResults[0].resp.Findings; len(kept) != 1 || kept[0].Title != "real bug" {
		t.Fatalf("kept = %#v, want the finding preserved despite categorize failure", kept)
	}
}

func TestCategorizeAgentErrorKeepsFinding(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{err: errors.New("context deadline exceeded")}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: []model.Finding{sampleFinding("real bug", "a.go", 1)}},
		run:  model.AgentRun{Name: "Reviewer 1", Role: "review"},
	}}
	_, warnings, err := engine.categorizeAndFilterVectorFindings(context.Background(), sampleReviewCtx(), vectorResults, model.ReviewRequest{}, NewLimiter(1), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "context deadline exceeded") {
		t.Fatalf("warnings = %#v", warnings)
	}
	if kept := vectorResults[0].resp.Findings; len(kept) != 1 {
		t.Fatalf("kept = %#v, want the finding preserved", kept)
	}
}

// Categorization maps onto a verdict and the one --verify-drop-policy decides,
// exactly as it does for the verifier.
func TestCategorizeAndFilterAppliesDropPolicy(t *testing.T) {
	cases := []struct {
		name       string
		categories []string
		// keptBy lists the policies under which the finding survives.
		keptBy []string
	}{
		{
			// Sole finding = confirmed: nothing ever drops it.
			name:       "sole finding is confirmed",
			categories: []string{model.CategoryFinding},
			keptBy:     []string{model.DropPolicyNone, model.DropPolicyRefutedOnly, model.DropPolicyRefutedAndUnverified},
		},
		{
			// A compile error bundled with a distinct runtime bug is not a CLEAR
			// suppression, so refuted-only keeps it.
			name:       "finding plus compilation is unverified",
			categories: []string{model.CategoryCompilation, model.CategoryFinding},
			keptBy:     []string{model.DropPolicyNone, model.DropPolicyRefutedOnly},
		},
		{
			name:       "finding plus confirmation is unverified",
			categories: []string{model.CategoryConfirmation, model.CategoryFinding},
			keptBy:     []string{model.DropPolicyNone, model.DropPolicyRefutedOnly},
		},
		{
			// No finding at all = refuted: clearly suppressible.
			name:       "pure confirmation is refuted",
			categories: []string{model.CategoryConfirmation},
			keptBy:     []string{model.DropPolicyNone},
		},
		{
			name:       "pure compilation is refuted",
			categories: []string{model.CategoryCompilation},
			keptBy:     []string{model.DropPolicyNone},
		},
	}
	for _, tc := range cases {
		for _, policy := range []string{model.DropPolicyNone, model.DropPolicyRefutedOnly, model.DropPolicyRefutedAndUnverified} {
			t.Run(tc.name+"/"+policy, func(t *testing.T) {
				llmClient := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{categorized(tc.categories...)}}
				engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
				vectorResults := []agentResult{{
					resp: &llm.ReviewResponse{Findings: []model.Finding{sampleFinding("f", "a.go", 1)}},
					run:  model.AgentRun{Name: "Reviewer 1", Role: "review"},
				}}
				_, _, err := engine.categorizeAndFilterVectorFindings(context.Background(), sampleReviewCtx(), vectorResults, model.ReviewRequest{VerifyDropPolicy: policy}, NewLimiter(1), "")
				if err != nil {
					t.Fatal(err)
				}
				wantKept := slices.Contains(tc.keptBy, policy)
				gotKept := len(vectorResults[0].resp.Findings) == 1
				if gotKept != wantKept {
					t.Fatalf("categories %v policy %q: kept = %v, want %v", tc.categories, policy, gotKept, wantKept)
				}
			})
		}
	}
}

// Scope is outside the policy, so even `none` drops a finding that cannot be
// anchored to the patch.
func TestCategorizeAndFilterDropsOutOfScopeUnderEveryPolicy(t *testing.T) {
	for _, policy := range []string{model.DropPolicyNone, model.DropPolicyRefutedOnly, model.DropPolicyRefutedAndUnverified} {
		llmClient := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{categorized(model.CategoryFinding)}}
		engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
		vectorResults := []agentResult{{
			resp: &llm.ReviewResponse{Findings: []model.Finding{sampleFinding("out of scope", "other.go", 99)}},
			run:  model.AgentRun{Name: "Reviewer 1", Role: "review"},
		}}
		_, _, err := engine.categorizeAndFilterVectorFindings(context.Background(), scopedReviewCtx(), vectorResults, model.ReviewRequest{VerifyDropPolicy: policy}, NewLimiter(1), "")
		if err != nil {
			t.Fatal(err)
		}
		if kept := vectorResults[0].resp.Findings; len(kept) != 0 {
			t.Fatalf("policy %q: kept = %#v, want the unanchorable finding dropped", policy, kept)
		}
	}
}

func TestCategorizeAndFilterRelocatesOrDropsWhollyOutOfScopeFindings(t *testing.T) {
	findings := []model.Finding{
		sampleFinding("already scoped", "f.go", 10),
		sampleFinding("relocate", "f.go", 100),
		sampleFinding("deleted anchor", "f.go", 12),
		sampleFinding("drop", "other.go", 1),
	}
	llmClient := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{
		{
			Categorization:          &model.FindingCategorization{Categories: []string{model.CategoryFinding}},
			ReplacementCodeLocation: &model.CodeLocation{FilePath: "other.go", LineRange: model.LineRange{Start: 1, End: 1}},
		},
		{
			Categorization:          &model.FindingCategorization{Categories: []string{model.CategoryFinding}},
			ReplacementCodeLocation: &model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 11, End: 11}, Content: "changed"},
		},
		{Categorization: &model.FindingCategorization{Categories: []string{model.CategoryFinding}}},
		{Categorization: &model.FindingCategorization{Categories: []string{model.CategoryFinding}}},
	}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: findings},
		run:  model.AgentRun{Name: "Reviewer 1", Role: "review"},
	}}
	_, _, err := engine.categorizeAndFilterVectorFindings(context.Background(), scopedReviewCtx(), vectorResults, model.ReviewRequest{}, NewLimiter(1), "")
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

func TestCategorizeAndFilterDisableDiffScopeKeepsOutsideFinding(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{{
		Categorization:          &model.FindingCategorization{Categories: []string{model.CategoryFinding}},
		ReplacementCodeLocation: &model.CodeLocation{FilePath: "f.go", LineRange: model.LineRange{Start: 10, End: 10}, Content: "changed"},
	}}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: []model.Finding{sampleFinding("outside", "other.go", 99)}},
		run:  model.AgentRun{Name: "Reviewer 1", Role: "review"},
	}}
	_, _, err := engine.categorizeAndFilterVectorFindings(context.Background(), scopedReviewCtx(), vectorResults, model.ReviewRequest{DisableDiffScope: true}, NewLimiter(1), "")
	if err != nil {
		t.Fatal(err)
	}
	kept := vectorResults[0].resp.Findings
	if len(kept) != 1 {
		t.Fatalf("outside finding dropped with --disable-diff-scope: %#v", kept)
	}
	if got := kept[0].CodeLocation.FilePath; got != "other.go" {
		t.Fatalf("outside finding relocated with --disable-diff-scope: %q", got)
	}
}

// The agent's outside-diff-scope tag is its own drop signal, independent of
// --verify-drop-policy: it read the whole claim, not just the cited line, so it
// can see that the substance is about unchanged code even when the location
// happens to touch a hunk.
func TestCategorizeAndFilterDropsAgentTaggedOutOfScope(t *testing.T) {
	categorySets := map[string][]string{
		"scope only":         {model.CategoryOutsideDiffScope},
		"scope plus finding": {model.CategoryFinding, model.CategoryOutsideDiffScope},
	}
	for name, categories := range categorySets {
		for _, policy := range []string{model.DropPolicyNone, model.DropPolicyRefutedOnly, model.DropPolicyRefutedAndUnverified} {
			t.Run(name+"/"+policy, func(t *testing.T) {
				llmClient := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{categorized(categories...)}}
				engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
				// f.go:10 is inside the scopedReviewCtx hunk, so only the agent's
				// tag can drop this one.
				vectorResults := []agentResult{{
					resp: &llm.ReviewResponse{Findings: []model.Finding{sampleFinding("in diff", "f.go", 10)}},
					run:  model.AgentRun{Name: "Reviewer 1", Role: "review"},
				}}
				_, _, err := engine.categorizeAndFilterVectorFindings(context.Background(), scopedReviewCtx(), vectorResults, model.ReviewRequest{VerifyDropPolicy: policy}, NewLimiter(1), "")
				if err != nil {
					t.Fatal(err)
				}
				if kept := vectorResults[0].resp.Findings; len(kept) != 0 {
					t.Fatalf("policy %q: kept = %#v, want the out-of-scope tag to drop it", policy, kept)
				}
			})
		}
	}
}

// --disable-diff-scope turns scope filtering off entirely, so an agent that
// emits the category anyway (reachable without schema enforcement) must not
// resurrect the filter the user disabled.
func TestCategorizeAndFilterDisableDiffScopeIgnoresAgentOutOfScope(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{categorized(model.CategoryOutsideDiffScope)}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	vectorResults := []agentResult{{
		resp: &llm.ReviewResponse{Findings: []model.Finding{sampleFinding("outside", "other.go", 99)}},
		run:  model.AgentRun{Name: "Reviewer 1", Role: "review"},
	}}
	_, _, err := engine.categorizeAndFilterVectorFindings(context.Background(), scopedReviewCtx(), vectorResults, model.ReviewRequest{DisableDiffScope: true}, NewLimiter(1), "")
	if err != nil {
		t.Fatal(err)
	}
	if kept := vectorResults[0].resp.Findings; len(kept) != 1 {
		t.Fatalf("kept = %#v, want the finding preserved with --disable-diff-scope", kept)
	}
}

func TestCategorizeExecutesToolCallsThroughAgentLoop(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{
		{
			ToolCalls:  []llm.ToolCall{{ID: "call_1", Name: "inspect_file", Arguments: `{"path":"main.go"}`}},
			TokensUsed: model.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		},
		{
			Categorization: &model.FindingCategorization{Categories: []string{model.CategoryFinding}, Remarks: "real"},
			TokensUsed:     model.TokenUsage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
		},
	}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	got, usage, err := engine.Categorize(context.Background(), CategorizeRequest{
		ReviewCtx: sampleReviewCtx(),
		Finding:   sampleFinding("x", "main.go", 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if llmClient.calls != 2 {
		t.Fatalf("calls = %d, want the tool call followed by the answer", llmClient.calls)
	}
	if len(llmClient.requests[0].Tools) == 0 {
		t.Fatal("categorize must expose the retrieval tools for diff-scope relocation")
	}
	if got == nil || model.VerdictForCategories(got.Categories) != model.VerdictConfirmed {
		t.Fatalf("categorization = %#v", got)
	}
	if usage.TotalTokens != 7 {
		t.Fatalf("usage = %#v, want both calls accounted", usage)
	}
}

func TestCategorizeRetriesMissingCategorization(t *testing.T) {
	llmClient := &scriptedCategorizeLLM{responses: []*llm.ReviewResponse{
		{RawResponse: "not a categorization", TokensUsed: model.TokenUsage{TotalTokens: 2}},
		{Categorization: &model.FindingCategorization{Categories: []string{model.CategoryConfirmation}, Remarks: "praise"}},
	}}
	engine := NewEngine(stubSource{}, llmClient, stubRetrieval{}, config.Profile{Model: "test"})
	got, _, err := engine.Categorize(context.Background(), CategorizeRequest{
		ReviewCtx:        sampleReviewCtx(),
		Finding:          sampleFinding("x", "main.go", 1),
		MaxOutputRetries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if llmClient.calls != 2 {
		t.Fatalf("calls = %d, want one retry", llmClient.calls)
	}
	if got == nil || got.Categories[0] != model.CategoryConfirmation {
		t.Fatalf("categorization = %#v", got)
	}
}
