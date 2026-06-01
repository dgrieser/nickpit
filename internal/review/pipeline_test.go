package review

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

	a := writeFindingsFile(t, "a.json", model.ReviewResult{
		Findings:           []model.Finding{{ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Title: "a", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "m.go", LineRange: model.LineRange{Start: 1, End: 1}}}},
		OverallCorrectness: "patch is incorrect",
	})
	b := writeFindingsFile(t, "b.json", model.ReviewResult{
		Findings:           []model.Finding{{ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Title: "b", Priority: intPtr(2), CodeLocation: model.CodeLocation{FilePath: "m.go", LineRange: model.LineRange{Start: 2, End: 2}}}},
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

// When the initial reviewer fails, a following standalone nudge step must error
// cleanly (the failed review left no session) rather than panic on a nil
// response.
func TestWorkflowNudgeAfterFailedReviewErrorsNoPanic(t *testing.T) {
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
	_, _, err = engine.RunSpecPipeline(context.Background(), pipeline, model.ReviewRequest{
		Mode:             model.ModeLocal,
		RepoRoot:         ".",
		MaxContextTokens: 1000,
		MaxToolCalls:     1,
	})
	if err == nil {
		t.Fatal("expected error from nudge after failed review")
	}
	if !strings.Contains(err.Error(), "did not complete successfully") {
		t.Fatalf("error = %q, want a clear 'review did not complete' message", err.Error())
	}
}

// The default review spec executed through the pipeline needs a source and
// reproduces the legacy reviewer/merge run shape that RunWithContext returns.
func TestWorkflowDefaultReviewSpecNeedsSourceAndRuns(t *testing.T) {
	client := &multiAgentLLM{}
	engine := NewEngine(stubSource{}, client, stubRetrieval{}, config.Profile{Model: "test"})
	engine.SetLogger(logging.New(os.Stderr, false, false))

	pipeline, err := engine.BuildPipeline(workflow.DefaultReviewSpec())
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
	expectedAgentRuns := 1 + len(reviewVectors) + (len(reviewVectors) - 1)
	if len(result.AgentRuns) != expectedAgentRuns {
		t.Fatalf("agent runs = %d, want %d", len(result.AgentRuns), expectedAgentRuns)
	}
	if client.verifyCalls != len(reviewVectors) {
		t.Fatalf("verify calls = %d, want %d", client.verifyCalls, len(reviewVectors))
	}
}
