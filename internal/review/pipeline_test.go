package review

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
)

type countingLLM struct {
	mu    sync.Mutex
	calls int
}

func (c *countingLLM) Review(context.Context, *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return &llm.ReviewResponse{}, nil
}

type finalizingPriorityDowngradeLLM struct {
	multiAgentLLM
}

func (s *finalizingPriorityDowngradeLLM) Review(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	if req.SchemaKind != llm.SchemaKindFinalize {
		return s.multiAgentLLM.Review(ctx, req)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := *req
	cloned.Messages = cloneTestMessages(req.Messages)
	s.finalizeRequests = append(s.finalizeRequests, &cloned)
	findings := testPayloadFindingsFromJSON(taskMessageContent(req))
	for i := range findings {
		findings[i].Finalization = &model.FindingFinalization{
			Title:           "Final " + findings[i].Title,
			Body:            "FINALIZED_MARKER " + findings[i].Body,
			Priority:        2,
			ConfidenceScore: 0.8,
			Remarks:         "downgraded",
		}
	}
	return &llm.ReviewResponse{
		Findings:   findings,
		TokensUsed: model.TokenUsage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
	}, nil
}

func pipelineTestEngine(client llm.Client) *Engine {
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logging.New(os.Stderr, false, false))
	return engine
}

func writeFindingsFile(t *testing.T, name string, result model.ReviewResult) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A single injected findings file means one merge group; merge passes it through
// without invoking the merge agent (matching len==1 semantics), and the findings
// survive unchanged.
func TestWorkflowMergeSingleFileInjectionPassthrough(t *testing.T) {
	client := &countingLLM{}
	engine := pipelineTestEngine(client)
	path := writeFindingsFile(t, "findings.json", model.ReviewResult{
		Findings: []model.Finding{
			{ID: "11111111-1111-1111-1111-111111111111", Title: "a", Priority: intPtr(1), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 1, End: 1}}},
			{ID: "22222222-2222-2222-2222-222222222222", Title: "b", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "main.go", LineRange: model.LineRange{Start: 2, End: 2}}},
		},
		OverallCorrectness: "patch is incorrect",
	})

	spec := workflow.SingleStepSpec(workflow.StepMerge, []string{path})
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	if pipeline.NeedsSource() {
		t.Fatal("merge-from-file must not need a source")
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 0 {
		t.Fatalf("LLM calls = %d, want 0 for single-group passthrough merge", client.calls)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("findings = %d, want 2 preserved from injection", len(result.Findings))
	}
	if len(result.SegmentRuntimes) != 1 {
		t.Fatalf("segment runtimes = %#v, want one entry per pipeline unit", result.SegmentRuntimes)
	}
	if steps := result.SegmentRuntimes[0].Steps; len(steps) != 1 || steps[0] != workflow.StepMerge {
		t.Fatalf("segment steps = %#v, want [merge]", steps)
	}
}

// Finalize injection with no findings short-circuits (Finalize early-returns),
// so the injected result is emitted without any LLM call and reviewers/verify
// never run.
func TestWorkflowFinalizeInjectionEmptyFindings(t *testing.T) {
	client := &countingLLM{}
	engine := pipelineTestEngine(client)
	path := writeFindingsFile(t, "result.json", model.ReviewResult{
		Findings:           nil,
		OverallCorrectness: "patch is correct",
		OverallExplanation: "nothing to flag",
	})

	spec := workflow.SingleStepSpec(workflow.StepFinalize, []string{path})
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	if pipeline.NeedsSource() {
		t.Fatal("finalize-from-file must not need a source")
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 0 {
		t.Fatalf("LLM calls = %d, want 0 when finalizing empty findings", client.calls)
	}
	if result.OverallExplanation != "nothing to flag" {
		t.Fatalf("overall explanation = %q, want injected value", result.OverallExplanation)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("findings = %d, want 0", len(result.Findings))
	}
}

// Finalize with no merge and no injection must materialize from groups while
// the finalize step holds the state lock — exercising the locked materialize
// path that must not self-deadlock on the non-reentrant mutex.
func TestWorkflowFinalizeWithoutMergeNoDeadlock(t *testing.T) {
	client := &countingLLM{}
	engine := pipelineTestEngine(client)
	pipeline, err := engine.BuildPipeline(workflow.SingleStepSpec(workflow.StepFinalize, nil))
	if err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		result *model.ReviewResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		r, _, e := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal})
		done <- outcome{r, e}
	}()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if len(got.result.Findings) != 0 {
			t.Fatalf("findings = %d, want 0", len(got.result.Findings))
		}
		if client.calls != 0 {
			t.Fatalf("LLM calls = %d, want 0", client.calls)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("finalize-without-merge deadlocked")
	}
}

// Two injected files give merge two groups, so the merge agent must actually
// run (not degrade to the JSON-render fallback) even though no source/context
// populated the enriched prompt.
func TestWorkflowMergeTwoFilesInvokesMergeAgent(t *testing.T) {
	client := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logging.New(os.Stderr, false, false))

	// The two findings are related enough to cluster as dedupe.Possible (same
	// file, 12-line gap, shared body, similar titles), so the cluster merge
	// actually invokes a micro-merge agent instead of passing both through.
	a := writeFindingsFile(t, "a.json", model.ReviewResult{
		Findings:           []model.Finding{{ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Title: "Fix cleanup behavior alpha", Body: "body", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "m.go", LineRange: model.LineRange{Start: 1, End: 1}}}},
		OverallCorrectness: "patch is incorrect",
	})
	b := writeFindingsFile(t, "b.json", model.ReviewResult{
		Findings:           []model.Finding{{ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Title: "Fix cleanup behavior beta", Body: "body", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "m.go", LineRange: model.LineRange{Start: 13, End: 13}}}},
		OverallCorrectness: "patch is incorrect",
	})
	pipeline, err := engine.BuildPipeline(workflow.SingleStepSpec(workflow.StepMerge, []string{a, b}))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.mergeRequests) == 0 {
		t.Fatal("merge agent was not invoked for a two-group injected merge")
	}
	if len(result.Findings) != 2 {
		t.Fatalf("merged findings = %d, want 2", len(result.Findings))
	}
}

func TestWorkflowMergeSourcelessIncludesAdditionalStyleGuides(t *testing.T) {
	client := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logging.New(os.Stderr, false, false))
	engine.SetAdditionalStyleGuides([]model.AdditionalStyleGuide{
		{StyleGuide: model.StyleGuide{Language: "team.md", Content: "### Additional styleguide: team.md\n\nNo TODO comments."}},
	})

	// Same clustering setup as TestWorkflowMergeTwoFilesInvokesMergeAgent so
	// the merge LLM actually runs: ensurePrompts never fires in a source-less
	// merge, and the additional guides must still reach the merge prompt.
	a := writeFindingsFile(t, "a.json", model.ReviewResult{
		Findings:           []model.Finding{{ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Title: "Fix cleanup behavior alpha", Body: "body", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "m.go", LineRange: model.LineRange{Start: 1, End: 1}}}},
		OverallCorrectness: "patch is incorrect",
	})
	b := writeFindingsFile(t, "b.json", model.ReviewResult{
		Findings:           []model.Finding{{ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Title: "Fix cleanup behavior beta", Body: "body", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "m.go", LineRange: model.LineRange{Start: 13, End: 13}}}},
		OverallCorrectness: "patch is incorrect",
	})
	pipeline, err := engine.BuildPipeline(workflow.SingleStepSpec(workflow.StepMerge, []string{a, b}))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal}); err != nil {
		t.Fatal(err)
	}
	if len(client.mergeRequests) == 0 {
		t.Fatal("merge agent was not invoked")
	}
	system := client.mergeRequests[0].Messages[0].Content
	if !strings.Contains(system, "### Additional styleguide: team.md") {
		t.Fatalf("merge system prompt misses additional guide: %.200q", system)
	}
}

func TestWorkflowFusedPostMergeFinalizesVerdictsAndSummarizes(t *testing.T) {
	client := &multiAgentLLM{}
	engine := pipelineTestEngine(client)
	firstFinding := verifiedPipelineFinding("11111111-1111-4111-8111-111111111111", "Fix cleanup behavior alpha", "m.go", 1, 1)
	firstFinding.Suggestions = []model.Suggestion{{
		Body:      "Replace the repeated cleanup setup with a shared helper so each test covers one clear scenario.",
		LineRange: model.LineRange{Start: 3, End: 4},
	}}
	a := writeFindingsFile(t, "a.json", model.ReviewResult{
		Findings: []model.Finding{
			firstFinding,
		},
	})
	b := writeFindingsFile(t, "b.json", model.ReviewResult{
		Findings: []model.Finding{
			verifiedPipelineFinding("22222222-2222-4222-8222-222222222222", "Fix cleanup behavior beta", "m.go", 13, 1),
		},
	})
	c := writeFindingsFile(t, "c.json", model.ReviewResult{
		Findings: []model.Finding{
			verifiedPipelineFinding("33333333-3333-4333-8333-333333333333", "Reject malformed config input", "other.go", 90, 2),
		},
	})
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{
		{Pipeline: []workflow.StepEntry{
			{Type: workflow.StepMerge, FindingsFrom: []string{a, b, c}},
			{Type: workflow.StepFinalize},
			{Type: workflow.StepVerdict},
			{Type: workflow.StepSummarize},
		}},
	}}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.mergeRequests) != 1 {
		t.Fatalf("merge requests = %d, want one ambiguous cluster merge", len(client.mergeRequests))
	}
	if len(client.finalizeRequests) != 2 {
		t.Fatalf("finalize requests = %d, want one per resolved cluster", len(client.finalizeRequests))
	}
	if len(client.verdictRequests) != 1 {
		t.Fatalf("verdict requests = %d, want one", len(client.verdictRequests))
	}
	if len(client.summarizeRequests) != 3 {
		t.Fatalf("summarize requests = %d, want two finding shards plus overall", len(client.summarizeRequests))
	}
	if len(result.Findings) != 3 {
		t.Fatalf("findings = %d, want 3", len(result.Findings))
	}
	for _, finding := range result.Findings {
		if finding.Finalization == nil {
			t.Fatalf("finding %s missing finalization: %+v", finding.ID, finding)
		}
		if finding.Summarization == nil || !strings.Contains(finding.Summarization.Body, "SUMMARY_MARKER FINALIZED_MARKER") {
			t.Fatalf("finding %s summarization = %#v, want summarized finalized body", finding.ID, finding.Summarization)
		}
	}
	var summarizedSuggestion string
	for _, finding := range result.Findings {
		if finding.ID == firstFinding.ID && finding.Summarization != nil && len(finding.Summarization.Suggestions) > 0 {
			summarizedSuggestion = finding.Summarization.Suggestions[0].Body
			if !sameTestLineAnchor(finding.Summarization.Suggestions[0].LineRange, firstFinding.Suggestions[0].LineRange) {
				t.Fatalf("suggestion line range = %+v, want %+v", finding.Summarization.Suggestions[0].LineRange, firstFinding.Suggestions[0].LineRange)
			}
		}
	}
	if !strings.Contains(summarizedSuggestion, "SUMMARY_MARKER") {
		t.Fatalf("summarized suggestion = %q, want summarized by fused summarize lane", summarizedSuggestion)
	}
	// overall_confidence_score is code-computed: verdict "patch is incorrect" with
	// floor-1 deciding findings → max finalization confidence (0.6*0.9 + 0.4*0.7 = 0.82).
	if result.OverallCorrectness != "patch is incorrect" || result.OverallConfidenceScore != 0.82 {
		t.Fatalf("overall = %q %.2f, want patch is incorrect / 0.82", result.OverallCorrectness, result.OverallConfidenceScore)
	}
	if !strings.Contains(result.OverallExplanation, "SUMMARY_MARKER VERDICT_MARKER findings=3") {
		t.Fatalf("overall explanation = %q, want summarized verdict text", result.OverallExplanation)
	}
	if result.FinalizeTokensUsed.TotalTokens != 14 {
		t.Fatalf("finalize tokens = %+v, want two shard runs", result.FinalizeTokensUsed)
	}
	if result.VerdictTokensUsed.TotalTokens != 8 {
		t.Fatalf("verdict tokens = %+v, want verdict run tokens", result.VerdictTokensUsed)
	}
	if result.SummarizeTokensUsed.TotalTokens != 15 {
		t.Fatalf("summarize tokens = %+v, want two shard runs plus overall", result.SummarizeTokensUsed)
	}
	if countAgentRuns(result.AgentRuns, "finalize") != 2 || countAgentRuns(result.AgentRuns, "verdict") != 1 || countAgentRuns(result.AgentRuns, "summarize") != 3 {
		t.Fatalf("agent run counts = finalize:%d verdict:%d summarize:%d", countAgentRuns(result.AgentRuns, "finalize"), countAgentRuns(result.AgentRuns, "verdict"), countAgentRuns(result.AgentRuns, "summarize"))
	}
	if len(result.SegmentRuntimes) != 1 || len(result.SegmentRuntimes[0].Steps) != 1 || result.SegmentRuntimes[0].Steps[0] != "merge→finalize→verdict→summarize" {
		t.Fatalf("segment runtimes = %+v, want fused post-merge segment", result.SegmentRuntimes)
	}
}

func sameTestLineAnchor(a, b model.LineRange) bool {
	return a.SameAnchor(b)
}

func TestWorkflowFusedPostMergeVerdictConfidenceFilterOwnsFinalFindings(t *testing.T) {
	client := &multiAgentLLM{}
	engine := pipelineTestEngine(client)
	path := writeFindingsFile(t, "single.json", model.ReviewResult{
		Findings: []model.Finding{
			verifiedPipelineFinding("11111111-1111-4111-8111-111111111111", "Fix low confidence issue", "a.go", 1, 1),
		},
	})
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{
		{Pipeline: []workflow.StepEntry{
			{Type: workflow.StepMerge, FindingsFrom: []string{path}},
			{Type: workflow.StepFinalize},
			{Type: workflow.StepVerdict},
			{Type: workflow.StepSummarize},
		}},
	}}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal, ConfidenceThreshold: 0.83})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("findings = %d, want none after verdict confidence filter: %#v", len(result.Findings), result.Findings)
	}
	if result.OverallCorrectness != "patch is correct" || result.OverallExplanation != "No findings remained after confidence filtering." {
		t.Fatalf("overall = %q / %q, want confidence-filtered clean verdict", result.OverallCorrectness, result.OverallExplanation)
	}
	if len(client.verdictRequests) != 0 {
		t.Fatalf("verdict requests = %d, want skipped verdict agent after filter removed all findings", len(client.verdictRequests))
	}
}

func TestWorkflowFusedPostMergePriorityFilterUsesFinalizedPriority(t *testing.T) {
	client := &finalizingPriorityDowngradeLLM{}
	engine := pipelineTestEngine(client)
	path := writeFindingsFile(t, "single.json", model.ReviewResult{
		Findings: []model.Finding{
			verifiedPipelineFinding("11111111-1111-4111-8111-111111111111", "Fix downgraded issue", "a.go", 1, 1),
		},
	})
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{
		{Pipeline: []workflow.StepEntry{
			{Type: workflow.StepMerge, FindingsFrom: []string{path}},
			{Type: workflow.StepFinalize},
			{Type: workflow.StepVerdict},
			{Type: workflow.StepSummarize},
		}},
	}}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal, PriorityThreshold: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("findings = %d, want none after finalized priority filter: %#v", len(result.Findings), result.Findings)
	}
	if result.OverallCorrectness != "patch is correct" || result.OverallExplanation != "No findings remained after priority filtering." {
		t.Fatalf("overall = %q / %q, want priority-filtered clean verdict", result.OverallCorrectness, result.OverallExplanation)
	}
	if len(client.verdictRequests) != 0 {
		t.Fatalf("verdict requests = %d, want skipped verdict after priority filter removed all findings", len(client.verdictRequests))
	}
	if len(client.summarizeRequests) != 0 {
		t.Fatalf("summarize requests = %d, want no summarize request for filtered finding", len(client.summarizeRequests))
	}
}

func TestWorkflowFlatTailRunsUnfused(t *testing.T) {
	// Without a pipeline: group there is no auto-fusion: the tail runs as four
	// separate sequential steps, and finalize/summarize each run once over the
	// whole finding set rather than per-cluster shards.
	client := &multiAgentLLM{}
	engine := pipelineTestEngine(client)
	a := writeFindingsFile(t, "a.json", model.ReviewResult{
		Findings: []model.Finding{
			verifiedPipelineFinding("11111111-1111-4111-8111-111111111111", "Fix cleanup behavior alpha", "m.go", 1, 1),
		},
	})
	b := writeFindingsFile(t, "b.json", model.ReviewResult{
		Findings: []model.Finding{
			verifiedPipelineFinding("22222222-2222-4222-8222-222222222222", "Fix cleanup behavior beta", "m.go", 13, 1),
		},
	})
	c := writeFindingsFile(t, "c.json", model.ReviewResult{
		Findings: []model.Finding{
			verifiedPipelineFinding("33333333-3333-4333-8333-333333333333", "Reject malformed config input", "other.go", 90, 2),
		},
	})
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{
		{Type: workflow.StepMerge, FindingsFrom: []string{a, b, c}},
		{Type: workflow.StepFinalize},
		{Type: workflow.StepVerdict},
		{Type: workflow.StepSummarize},
	}}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SegmentRuntimes) != 4 {
		t.Fatalf("segments = %d, want 4 unfused tail segments: %+v", len(result.SegmentRuntimes), result.SegmentRuntimes)
	}
	if len(client.finalizeRequests) != 1 {
		t.Fatalf("finalize requests = %d, want one whole-set pass", len(client.finalizeRequests))
	}
	if len(client.summarizeRequests) != 1 {
		t.Fatalf("summarize requests = %d, want one whole-set pass", len(client.summarizeRequests))
	}
	if len(client.verdictRequests) != 1 {
		t.Fatalf("verdict requests = %d, want one", len(client.verdictRequests))
	}
}

func TestWorkflowFusedPostMergeSingleInputSkipsMergeLLM(t *testing.T) {
	client := &multiAgentLLM{}
	engine := pipelineTestEngine(client)
	path := writeFindingsFile(t, "single.json", model.ReviewResult{
		Findings: []model.Finding{
			verifiedPipelineFinding("44444444-4444-4444-8444-444444444444", "Fix first issue", "a.go", 1, 1),
			verifiedPipelineFinding("55555555-5555-4555-8555-555555555555", "Fix second issue", "b.go", 20, 2),
		},
	})
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{
		{Pipeline: []workflow.StepEntry{
			{Type: workflow.StepMerge, FindingsFrom: []string{path}},
			{Type: workflow.StepFinalize},
			{Type: workflow.StepVerdict},
			{Type: workflow.StepSummarize},
		}},
	}}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.mergeRequests) != 0 {
		t.Fatalf("merge requests = %d, want none for one input group", len(client.mergeRequests))
	}
	if len(client.finalizeRequests) != 2 {
		t.Fatalf("finalize requests = %d, want one per singleton finding", len(client.finalizeRequests))
	}
	if len(result.Findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(result.Findings))
	}
}

func TestWorkflowFusedPostMergeEmptyBranchSkipsLLMs(t *testing.T) {
	client := &multiAgentLLM{}
	engine := pipelineTestEngine(client)
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{
		{Pipeline: []workflow.StepEntry{
			{Type: workflow.StepMerge},
			{Type: workflow.StepFinalize},
			{Type: workflow.StepVerdict},
			{Type: workflow.StepSummarize},
		}},
	}}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(client.mergeRequests) + len(client.finalizeRequests) + len(client.verdictRequests) + len(client.summarizeRequests); got != 0 {
		t.Fatalf("LLM calls = %d, want none for empty merge branch", got)
	}
	if len(result.Findings) != 0 || result.OverallCorrectness != "patch is correct" {
		t.Fatalf("result = %+v, want empty correct result", result)
	}
	if countAgentRuns(result.AgentRuns, "verdict") != 1 {
		t.Fatalf("verdict run count = %d, want skipped verdict telemetry", countAgentRuns(result.AgentRuns, "verdict"))
	}
}

func TestWorkflowFusedPostMergeVerdictFailureFallsBack(t *testing.T) {
	client := &multiAgentLLM{verdictFailErr: errors.New("verdict down")}
	engine := pipelineTestEngine(client)
	path := writeFindingsFile(t, "input.json", model.ReviewResult{
		Findings: []model.Finding{
			verifiedPipelineFinding("66666666-6666-4666-8666-666666666666", "Fix fallback issue", "a.go", 1, 1),
		},
	})
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{
		{Pipeline: []workflow.StepEntry{
			{Type: workflow.StepMerge, FindingsFrom: []string{path}},
			{Type: workflow.StepFinalize},
			{Type: workflow.StepVerdict},
		}},
	}}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if result.OverallCorrectness != "patch is incorrect" {
		t.Fatalf("overall correctness = %q, want merge-derived fallback", result.OverallCorrectness)
	}
	// Confidence is code-computed even on verdict failure: floor-1 deciding finding,
	// max finalization confidence 0.6*0.9 + 0.4*0.7 = 0.82.
	if result.OverallConfidenceScore != 0.82 {
		t.Fatalf("overall confidence = %.2f, want code-computed 0.82", result.OverallConfidenceScore)
	}
	if !strings.Contains(result.OverallExplanation, "Merged 1 reviewer finding lists") {
		t.Fatalf("overall explanation = %q, want merge-derived fallback", result.OverallExplanation)
	}
	if !slices.ContainsFunc(result.AgentRuns, func(run model.AgentRun) bool {
		return run.Role == "verdict" && run.Status == model.AgentRunStatusFailed && strings.Contains(run.Error, "verdict down")
	}) {
		t.Fatalf("agent runs missing failed verdict: %+v", result.AgentRuns)
	}
	if !slices.ContainsFunc(result.Warnings, func(w string) bool {
		return strings.Contains(w, "Verdict failed")
	}) {
		t.Fatalf("warnings missing verdict failure: %+v", result.Warnings)
	}
}

func TestWorkflowFusedPostMergeVerdictFailureCoercesNonBlocking(t *testing.T) {
	// Only-P2 findings: verdictConstraintsFor requires "patch is correct". The
	// merge-derived fallback is "patch is incorrect" (findings present), so a
	// verdict failure must NOT emit a blocking verdict for non-blocking findings.
	client := &multiAgentLLM{verdictFailErr: errors.New("verdict down")}
	engine := pipelineTestEngine(client)
	path := writeFindingsFile(t, "input.json", model.ReviewResult{
		Findings: []model.Finding{
			verifiedPipelineFinding("66666666-6666-4666-8666-666666666666", "Fix fallback issue", "a.go", 1, 2),
		},
	})
	spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{
		{Pipeline: []workflow.StepEntry{
			{Type: workflow.StepMerge, FindingsFrom: []string{path}},
			{Type: workflow.StepFinalize},
			{Type: workflow.StepVerdict},
		}},
	}}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal})
	if err != nil {
		t.Fatal(err)
	}
	if result.OverallCorrectness != "patch is correct" {
		t.Fatalf("overall correctness = %q, want constraint-coerced \"patch is correct\"", result.OverallCorrectness)
	}
	// Code-computed confidence for a non-blocking verdict: the only finding is a
	// floor-2, which never tempers a "patch is correct" verdict => 1.0.
	if result.OverallConfidenceScore != 1.0 {
		t.Fatalf("overall confidence = %.2f, want code-computed 1.0", result.OverallConfidenceScore)
	}
	// Coercing the verdict to "patch is correct" must drop the merge-derived
	// explanation, which would otherwise read as a "patch is incorrect" rationale.
	if strings.Contains(result.OverallExplanation, "Merged") {
		t.Fatalf("overall explanation = %q, want stale merge text replaced after coercion", result.OverallExplanation)
	}
	if !slices.ContainsFunc(result.AgentRuns, func(run model.AgentRun) bool {
		return run.Role == "verdict" && run.Status == model.AgentRunStatusFailed
	}) {
		t.Fatalf("agent runs missing failed verdict: %+v", result.AgentRuns)
	}
}

func verifiedPipelineFinding(id, title, file string, line int, priority int) model.Finding {
	return model.Finding{
		ID:              id,
		Title:           title,
		Body:            "body " + title,
		ConfidenceScore: 0.7,
		Priority:        intPtr(priority),
		CodeLocation:    model.CodeLocation{FilePath: file, LineRange: model.LineRange{Start: line, End: line}},
		Verification:    &model.FindingVerification{ID: id, Verdict: model.VerdictConfirmed, Priority: priority, ConfidenceScore: 0.9, Remarks: "verified"},
	}
}

func countAgentRuns(runs []model.AgentRun, role string) int {
	count := 0
	for _, run := range runs {
		if run.Role == role {
			count++
		}
	}
	return count
}

// Findings injected into finalize must still be priority-filtered, so
// --priority-threshold stays effective for finalize-from-file workflows.
func TestWorkflowFinalizeInjectionRespectsPriorityThreshold(t *testing.T) {
	client := &countingLLM{}
	engine := pipelineTestEngine(client)
	path := writeFindingsFile(t, "f.json", model.ReviewResult{
		Findings: []model.Finding{
			{ID: "11111111-1111-1111-1111-111111111111", Title: "p2", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "m.go", LineRange: model.LineRange{Start: 1, End: 1}}},
			{ID: "22222222-2222-2222-2222-222222222222", Title: "p3", Priority: intPtr(3), CodeLocation: model.CodeLocation{FilePath: "m.go", LineRange: model.LineRange{Start: 2, End: 2}}},
		},
		OverallCorrectness: "patch is incorrect",
	})
	pipeline, err := engine.BuildPipeline(workflow.SingleStepSpec(workflow.StepFinalize, []string{path}))
	if err != nil {
		t.Fatal(err)
	}
	// p1 threshold drops both p2 and p3; finalize then short-circuits on empty.
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{Mode: model.ModeLocal, PriorityThreshold: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("findings = %d, want 0 after p1 filter", len(result.Findings))
	}
	if client.calls != 0 {
		t.Fatalf("LLM calls = %d, want 0 (all findings filtered out)", client.calls)
	}
}

// A standalone nudge step advances the session left open by a review step
// (nudge_count: 0), driving an extra reviewer pass through the same
// reviewer-session machinery the default flow uses.
func TestWorkflowStandaloneNudgeStep(t *testing.T) {
	client := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logging.New(os.Stderr, false, false))

	zero := 0
	spec := workflow.Spec{
		Version: workflow.SpecVersion,
		Steps: []workflow.StepEntry{
			{Type: workflow.StepReviewPrefix + "security", Config: &workflow.StepOverride{NudgeCount: &zero}},
			{Type: workflow.StepNudgePrefix + "security"},
		},
	}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Initial pass: tool call + final response (2 calls). Standalone nudge: 1 more.
	if got := client.vectorCalls["Security"]; got != 3 {
		t.Fatalf("Security reviewer calls = %d, want 3 (init tool + init final + nudge)", got)
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected at least one finding from the security reviewer")
	}
}

// A standalone nudge step is skipped outright when the reviewer session already
// reached its max_findings limit: the round could only return findings destined
// to be cut.
func TestWorkflowStandaloneNudgeStepSkippedAtFindingLimit(t *testing.T) {
	client := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logging.New(os.Stderr, false, false))

	zero := 0
	one := 1
	spec := workflow.Spec{
		Version: workflow.SpecVersion,
		Steps: []workflow.StepEntry{
			{Type: workflow.StepReviewPrefix + "security", Config: &workflow.StepOverride{NudgeCount: &zero, MaxFindings: &one}},
			{Type: workflow.StepNudgePrefix + "security"},
		},
	}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Initial pass: tool call + final response (2 calls). The nudge step must
	// not add a third: the single finding already fills the limit.
	if got := client.vectorCalls["Security"]; got != 2 {
		t.Fatalf("Security reviewer calls = %d, want 2 (nudge skipped at finding limit)", got)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %d, want the initial finding kept", len(result.Findings))
	}
}

// When the initial reviewer fails, a following standalone nudge step must error
// cleanly (the failed review left no session) rather than panic on a nil
// response.
func TestWorkflowNudgeAfterFailedReviewSkipsGracefully(t *testing.T) {
	client := &multiAgentLLM{
		vectorFailErr: map[string]error{"Security": errors.New("security upstream fail")},
	}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logging.New(os.Stderr, false, false))

	zero := 0
	spec := workflow.Spec{
		Version: workflow.SpecVersion,
		Steps: []workflow.StepEntry{
			{Type: workflow.StepReviewPrefix + "security", Config: &workflow.StepOverride{NudgeCount: &zero}},
			{Type: workflow.StepNudgePrefix + "security"},
		},
	}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	// A soft-failed review must not turn the standalone nudge step into a
	// hard whole-run failure (verify:/dedupe: no-op on the same condition);
	// the skip is surfaced as a warning instead.
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatalf("expected graceful skip, got error: %v", err)
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "Skipped nudge:security") {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %v, want a 'Skipped nudge:security' warning", result.Warnings)
	}
}

// A run whose root context is cancelled must surface the cancellation instead
// of soft-failing every stage into an empty-findings "patch is correct"
// verdict at confidence 1.0 (every stage fails soft by design, so only the
// pipeline's root-context guard can distinguish SIGINT from a clean run).
func TestWorkflowCancelledRunErrorsInsteadOfFakeSuccess(t *testing.T) {
	client := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logging.New(os.Stderr, false, false))

	pipeline, err := engine.BuildPipeline(reviewerOnlySpec())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, _, err := engine.RunSpecPipeline(ctx, pipeline, model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err == nil {
		t.Fatalf("expected cancellation error, got result: %+v", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled in chain", err)
	}
}

// The reviewer-only spec executed through the pipeline needs a source and
// produces the reviewer/merge run shape (collect → reviewers → verify → dedupe
// → merge) without finalize/summarize.
func TestWorkflowReviewerSpecNeedsSourceAndRuns(t *testing.T) {
	client := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logging.New(os.Stderr, false, false))

	pipeline, err := engine.BuildPipeline(reviewerOnlySpec())
	if err != nil {
		t.Fatal(err)
	}
	if !pipeline.NeedsSource() {
		t.Fatal("default review spec must need a source")
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != len(reviewVectors) {
		t.Fatalf("findings = %d, want %d", len(result.Findings), len(reviewVectors))
	}
	// One micro-merge run: the synthetic vector findings form a single
	// Possible cluster.
	expectedAgentRuns := 1 + len(reviewVectors) + 1
	if len(result.AgentRuns) != expectedAgentRuns {
		t.Fatalf("agent runs = %d, want %d", len(result.AgentRuns), expectedAgentRuns)
	}
	if client.verifyCalls != len(reviewVectors) {
		t.Fatalf("verify calls = %d, want %d", client.verifyCalls, len(reviewVectors))
	}
}

// Runtime stamping is wall-clock based; back-date the anchors so the assertion
// is deterministic instead of racing a sub-millisecond stub run.
func TestReviewerSessionStampsRuntime(t *testing.T) {
	s := &reviewerSession{agent: agentSpec{name: "Security", role: "review"}, started: time.Now().Add(-3 * time.Second)}
	result := s.partialResult(model.ReviewRequest{})
	if result.run.RuntimeSeconds < 3 {
		t.Fatalf("runtime_seconds = %v, want >= 3", result.run.RuntimeSeconds)
	}
}

func TestReviewStepInternalOverridesRouteSubagentModels(t *testing.T) {
	alias := workflow.SmallModelAlias
	nudgeCount := 1
	client := &reasoningExtractLLM{
		reviewerReasoning: []string{"initial reasoning"},
		collectOutputs:    []string{"collected issue"},
		updateOutputs:     []string{"delta issue"},
	}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{
		Model:           "large-model",
		Small:           config.SmallModelConfig{Model: "small-model", ReasoningEffort: "low"},
		ReasoningEffort: "high",
	})
	engine.SetLogger(logging.New(os.Stderr, false, false))
	spec := workflow.Spec{
		Version: workflow.SpecVersion,
		Steps: []workflow.StepEntry{{
			Type: workflow.StepReviewPrefix + "security",
			Config: &workflow.StepOverride{
				NudgeCount:      &nudgeCount,
				MineReasoning:   &workflow.AgentOverride{Model: &alias},
				CompileFindings: &workflow.AgentOverride{Model: &alias},
				Nudge:           &workflow.AgentOverride{Model: &alias},
			},
		}},
	}
	pipeline, err := engine.BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{
		Mode:                model.ModeLocal,
		ModelEmitsReasoning: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	client.mu.Lock()
	reviewerModels := append([]string(nil), client.reviewerModels...)
	collectModels := append([]string(nil), client.collectModels...)
	updateModels := append([]string(nil), client.updateModels...)
	client.mu.Unlock()

	if got := strings.Join(reviewerModels, ","); got != "large-model,small-model" {
		t.Fatalf("reviewer/nudge models = %q, want large-model,small-model", got)
	}
	if got := strings.Join(collectModels, ","); got != "small-model" {
		t.Fatalf("mine reasoning models = %q, want small-model", got)
	}
	if got := strings.Join(updateModels, ","); got != "small-model" {
		t.Fatalf("compile findings models = %q, want small-model", got)
	}
}

// --- lane pipeline tests ---

// laneSpec builds collect-context → parallel lanes (review→verify→dedupe per
// vector) → merge, the shape of the embedded default workflow without
// finalize/verdict/summarize.
func laneSpec(vectors ...string) workflow.Spec {
	lanes := make([]workflow.StepEntry, len(vectors))
	for i, id := range vectors {
		lanes[i] = workflow.StepEntry{Lane: []workflow.StepEntry{
			{Type: workflow.StepReviewPrefix + id},
			{Type: workflow.StepVerifyPrefix + id},
			{Type: workflow.StepDedupePrefix + id},
		}}
	}
	return workflow.Spec{
		Version: workflow.SpecVersion,
		Steps: []workflow.StepEntry{
			{Type: workflow.StepCollectContext},
			{Parallel: lanes},
			{Type: workflow.StepMerge},
		},
	}
}

// laneEventLLM wraps multiAgentLLM, recording one classified event per LLM call
// ("review:Security", "verify:Security", "dedupe:Security", "context", "merge")
// so tests can assert cross-stage ordering within and across lanes. It can also
// fail or stall verify calls to exercise lane error and limiter semantics.
type laneEventLLM struct {
	inner *multiAgentLLM

	verifyErrFor     string        // fail verify calls for this vector's findings
	verifyHold       time.Duration // sleep inside each verify call
	verifyRendezvous int           // wait until this many verifies are in flight (1s cap)

	mu                sync.Mutex
	events            []string
	verifyInFlight    int
	maxVerifyInFlight int
}

func (l *laneEventLLM) Review(ctx context.Context, req *llm.ReviewRequest) (*llm.ReviewResponse, error) {
	label := classifyLaneCall(req)
	l.mu.Lock()
	l.events = append(l.events, label)
	l.mu.Unlock()
	if !strings.HasPrefix(label, "verify:") {
		return l.inner.Review(ctx, req)
	}
	if l.verifyErrFor != "" && label == "verify:"+l.verifyErrFor {
		return nil, errors.New("verify upstream down")
	}
	l.mu.Lock()
	l.verifyInFlight++
	if l.verifyInFlight > l.maxVerifyInFlight {
		l.maxVerifyInFlight = l.verifyInFlight
	}
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		l.verifyInFlight--
		l.mu.Unlock()
	}()
	if l.verifyRendezvous > 0 {
		deadline := time.Now().Add(time.Second)
		for {
			l.mu.Lock()
			// maxVerifyInFlight: once the rendezvous has been observed, later
			// (possibly lone) verify calls need not wait out the deadline.
			reached := l.maxVerifyInFlight >= l.verifyRendezvous
			l.mu.Unlock()
			if reached || time.Now().After(deadline) {
				break
			}
			time.Sleep(time.Millisecond)
		}
	}
	if l.verifyHold > 0 {
		time.Sleep(l.verifyHold)
	}
	return l.inner.Review(ctx, req)
}

func (l *laneEventLLM) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events...)
}

func classifyLaneCall(req *llm.ReviewRequest) string {
	system, user := "", ""
	if len(req.Messages) > 0 {
		system = req.Messages[0].Content
	}
	if len(req.Messages) > 1 {
		user = taskMessageContent(req)
	}
	switch {
	case req.SchemaKind == llm.SchemaKindVerify:
		return "verify:" + laneVectorFromContent(user)
	case strings.Contains(system, "DO NOT produce review findings yourself"):
		return "context"
	case strings.Contains(system, "## FOCUS ON "):
		return "review:" + vectorNameFromSystem(system)
	case strings.Contains(user, `"review_findings"`):
		return "dedupe:" + laneVectorFromContent(user)
	default:
		return "merge"
	}
}

// laneVectorFromContent identifies the vector a verify/dedupe payload belongs
// to via the stub finding titles ("Fix <vector name>").
func laneVectorFromContent(content string) string {
	for _, v := range reviewVectors {
		if strings.Contains(content, "Fix "+v.name) {
			return v.name
		}
	}
	return ""
}

func firstEventIndex(events []string, label string) int {
	for i, e := range events {
		if e == label {
			return i
		}
	}
	return -1
}

func lastEventIndex(events []string, label string) int {
	last := -1
	for i, e := range events {
		if e == label {
			last = i
		}
	}
	return last
}

func laneTestRequest() model.ReviewRequest {
	return model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	}
}

// Each lane runs review → verify → dedupe in order for its own vector, and the
// merge step only runs after every lane finished. The unit's segment runtime
// records the lane chains.
func TestWorkflowLaneRunsPerVectorVerifyAndDedupeInOrder(t *testing.T) {
	inner := &multiAgentLLM{vectorFindings: map[string]int{"Security": 2, "Performance": 2}}
	client := &laneEventLLM{inner: inner}
	engine := pipelineTestEngine(client)
	pipeline, err := engine.BuildPipeline(laneSpec("security", "performance"))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, laneTestRequest())
	if err != nil {
		t.Fatal(err)
	}
	events := client.snapshot()
	mergeIdx := firstEventIndex(events, "merge")
	if mergeIdx < 0 {
		t.Fatalf("no merge event in %v", events)
	}
	for _, name := range []string{"Security", "Performance"} {
		lastReview := lastEventIndex(events, "review:"+name)
		firstVerify := firstEventIndex(events, "verify:"+name)
		lastVerify := lastEventIndex(events, "verify:"+name)
		dedupeIdx := firstEventIndex(events, "dedupe:"+name)
		if lastReview < 0 || firstVerify < 0 || dedupeIdx < 0 {
			t.Fatalf("missing %s lane events in %v", name, events)
		}
		if lastReview > firstVerify || lastVerify > dedupeIdx || dedupeIdx > mergeIdx {
			t.Fatalf("%s lane out of order: review@%d verify@[%d,%d] dedupe@%d merge@%d", name, lastReview, firstVerify, lastVerify, dedupeIdx, mergeIdx)
		}
	}
	if len(result.Findings) != 4 {
		t.Fatalf("findings = %d, want 4", len(result.Findings))
	}
	wantSegment := "review:security→verify:security→dedupe:security"
	found := false
	for _, seg := range result.SegmentRuntimes {
		for _, step := range seg.Steps {
			if step == wantSegment {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("segment %q not recorded in %+v", wantSegment, result.SegmentRuntimes)
	}
}

func TestWorkflowGroupNamesLabelRuntimeSegments(t *testing.T) {
	spec := laneSpec("security")
	spec.Steps[1].Parallel[0].Name = "Security review"
	spec.Steps[2] = workflow.StepEntry{Name: "Review synthesis", Pipeline: []workflow.StepEntry{
		{Type: workflow.StepMerge},
		{Type: workflow.StepFinalize},
		{Type: workflow.StepVerdict},
	}}
	client := &laneEventLLM{inner: &multiAgentLLM{vectorFindings: map[string]int{"Security": 1}}}
	pipeline, err := pipelineTestEngine(client).BuildPipeline(spec)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := pipeline.Run(context.Background(), &model.ReviewContext{}, laneTestRequest())
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(result.SegmentRuntimes))
	for _, segment := range result.SegmentRuntimes {
		got = append(got, segment.Steps...)
	}
	if !slices.Contains(got, "Security review") || !slices.Contains(got, "Review synthesis") {
		t.Fatalf("runtime segment labels = %v, want named lane and pipeline", got)
	}
}

func TestWorkflowTestingVectorWiresDuplicateFileValidation(t *testing.T) {
	client := &multiAgentLLM{vectorFindings: map[string]int{"Testing": 2}}
	engine := pipelineTestEngine(client)
	pipeline, err := engine.BuildPipeline(laneSpec("testing"))
	if err != nil {
		t.Fatal(err)
	}
	req := laneTestRequest()
	req.MaxOutputRetries = 1
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, req)
	if err != nil {
		t.Fatal(err)
	}
	if got := client.vectorCalls["Testing"]; got != 3 {
		t.Fatalf("Testing reviewer calls = %d, want tool call, invalid response, retry", got)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %d, want duplicate same-file Testing findings pruned to 1", len(result.Findings))
	}
	foundTestingRun := false
	for _, run := range result.AgentRuns {
		if run.Name != "Testing" || run.Role != "review" {
			continue
		}
		foundTestingRun = true
		if got, want := run.Status, model.AgentRunStatusPartial; got != want {
			t.Fatalf("Testing run status = %q, want %q", got, want)
		}
		if !strings.Contains(run.Error, "duplicate-file Testing findings dropped") {
			t.Fatalf("Testing run error = %q, want duplicate-file prune message", run.Error)
		}
	}
	if !foundTestingRun {
		t.Fatalf("Testing review run missing from %#v", result.AgentRuns)
	}
}

// The verify limiter is shared across lanes: with a cap of 1, the four verify
// calls (two lanes × two findings) never overlap.
func TestLimiterCapsAcrossLanes(t *testing.T) {
	inner := &multiAgentLLM{vectorFindings: map[string]int{"Security": 2, "Performance": 2}}
	client := &laneEventLLM{inner: inner, verifyHold: 20 * time.Millisecond}
	engine := pipelineTestEngine(client)
	pipeline, err := engine.BuildPipeline(laneSpec("security", "performance"))
	if err != nil {
		t.Fatal(err)
	}
	req := laneTestRequest()
	req.Concurrency = 1
	if _, _, err := engine.RunSpecPipeline(context.Background(), pipeline, req); err != nil {
		t.Fatal(err)
	}
	if client.maxVerifyInFlight != 1 {
		t.Fatalf("max verify in flight = %d, want 1", client.maxVerifyInFlight)
	}
	if got := len(client.snapshot()); got == 0 {
		t.Fatal("no events recorded")
	}
}

// The default cap of 0 is unlimited: verify calls from different lanes overlap.
func TestVerifyUnlimitedRunsLanesConcurrently(t *testing.T) {
	inner := &multiAgentLLM{vectorFindings: map[string]int{"Security": 2, "Performance": 2}}
	client := &laneEventLLM{inner: inner, verifyRendezvous: 2}
	engine := pipelineTestEngine(client)
	pipeline, err := engine.BuildPipeline(laneSpec("security", "performance"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := engine.RunSpecPipeline(context.Background(), pipeline, laneTestRequest()); err != nil {
		t.Fatal(err)
	}
	if client.maxVerifyInFlight < 2 {
		t.Fatalf("max verify in flight = %d, want >= 2", client.maxVerifyInFlight)
	}
}

// A per-finding verify failure is downgraded to an unverified finding, so the
// lane continues through dedupe and still reaches merge with its siblings.
func TestWorkflowLaneVerifyFailureContinuesAsUnverified(t *testing.T) {
	inner := &multiAgentLLM{vectorFindings: map[string]int{"Security": 2, "Performance": 2}}
	client := &laneEventLLM{inner: inner, verifyErrFor: "Security"}
	engine := pipelineTestEngine(client)
	pipeline, err := engine.BuildPipeline(laneSpec("security", "performance"))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, laneTestRequest())
	if err != nil {
		t.Fatalf("RunSpecPipeline returned err: %v", err)
	}
	events := client.snapshot()
	if firstEventIndex(events, "dedupe:Security") < 0 {
		t.Fatalf("security dedupe did not run after soft verify failure: %v", events)
	}
	if firstEventIndex(events, "dedupe:Performance") < 0 {
		t.Fatalf("sibling performance lane did not complete: %v", events)
	}
	if firstEventIndex(events, "merge") < 0 {
		t.Fatalf("merge did not run after soft verify failure: %v", events)
	}
	foundWarning := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "Verify failed") && strings.Contains(warning, "verify upstream down") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Fatalf("warnings = %#v, want verify failure warning", result.Warnings)
	}
}

// A soft-failed reviewer leaves its lane's verify and dedupe as graceful no-ops;
// a single-finding reviewer skips dedupe.
func TestWorkflowLaneSkipsVerifyAndDedupeNoOps(t *testing.T) {
	inner := &multiAgentLLM{
		vectorFailErr:  map[string]error{"Security": errors.New("review down")},
		vectorFindings: map[string]int{"Performance": 1},
	}
	client := &laneEventLLM{inner: inner}
	engine := pipelineTestEngine(client)
	pipeline, err := engine.BuildPipeline(laneSpec("security", "performance"))
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, laneTestRequest())
	if err != nil {
		t.Fatal(err)
	}
	events := client.snapshot()
	if firstEventIndex(events, "verify:Security") >= 0 || firstEventIndex(events, "dedupe:Security") >= 0 {
		t.Fatalf("failed security review must skip verify/dedupe: %v", events)
	}
	if firstEventIndex(events, "verify:Performance") < 0 {
		t.Fatalf("performance verify missing: %v", events)
	}
	if firstEventIndex(events, "dedupe:Performance") >= 0 {
		t.Fatalf("single-finding performance dedupe must be skipped: %v", events)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(result.Findings))
	}
}

// The six-lane shape of the embedded default workflow (minus finalize/verdict/
// summarize) produces the same result shape as the global-step spec.
func TestWorkflowSixLaneSpecMatchesGlobalResultShape(t *testing.T) {
	inner := &multiAgentLLM{}
	client := &laneEventLLM{inner: inner}
	engine := pipelineTestEngine(client)
	pipeline, err := engine.BuildPipeline(laneSpec(workflow.ReviewVectorIDs...))
	if err != nil {
		t.Fatal(err)
	}
	if !pipeline.NeedsSource() {
		t.Fatal("lane review spec must need a source")
	}
	result, _, err := engine.RunSpecPipeline(context.Background(), pipeline, laneTestRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != len(reviewVectors) {
		t.Fatalf("findings = %d, want %d", len(result.Findings), len(reviewVectors))
	}
	if inner.verifyCalls != len(reviewVectors) {
		t.Fatalf("verify calls = %d, want %d", inner.verifyCalls, len(reviewVectors))
	}
}

func TestLiveLaneLabelPrefersName(t *testing.T) {
	// A configured name wins (this is how a named plain step, e.g. collect-context
	// with name: Context, surfaces in live progress).
	if got := liveLaneLabel(boundLane{name: "Context"}, "collect-context", 0); got != "Context" {
		t.Fatalf("named lane should use its name, got %q", got)
	}
	// Unnamed plain step (no ":" in the label) falls back to laneN.
	if got := liveLaneLabel(boundLane{}, "collect-context", 0); got != "lane0" {
		t.Fatalf("unnamed plain step should fall back to laneN, got %q", got)
	}
	// Unnamed vector step uses the suffix after ":".
	if got := liveLaneLabel(boundLane{}, "review:security", 1); got != "security" {
		t.Fatalf("unnamed vector step should use its suffix, got %q", got)
	}
}
