package review

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
