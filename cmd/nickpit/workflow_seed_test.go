package main

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/workflow"
)

func TestSeedFindingsTargetsFirstConsumingStep(t *testing.T) {
	// collect-context and review:* do not consume findings; merge does. The
	// files must land on merge, not be dropped on the leading non-consuming step.
	spec := workflow.Spec{Version: 1, Steps: []workflow.StepEntry{
		{Type: workflow.StepCollectContext},
		{Parallel: []workflow.StepEntry{{Type: workflow.StepReviewPrefix + "security"}}},
		{Type: workflow.StepMerge},
		{Type: workflow.StepFinalize},
	}}
	if err := seedFindings(&spec, []string{"a.json", "b.json"}); err != nil {
		t.Fatal(err)
	}
	if len(spec.Steps[0].FindingsFrom) != 0 {
		t.Fatalf("collect-context wrongly seeded: %v", spec.Steps[0].FindingsFrom)
	}
	merge := spec.Steps[2]
	if len(merge.FindingsFrom) != 2 {
		t.Fatalf("merge findings_from = %v, want 2 files", merge.FindingsFrom)
	}
}

func TestSeedFindingsSkipsStepsThatAlreadyDeclare(t *testing.T) {
	spec := workflow.Spec{Version: 1, Steps: []workflow.StepEntry{
		{Type: workflow.StepMerge, FindingsFrom: []string{"explicit.json"}},
		{Type: workflow.StepFinalize},
	}}
	if err := seedFindings(&spec, []string{"seed.json"}); err != nil {
		t.Fatal(err)
	}
	if got := spec.Steps[0].FindingsFrom; len(got) != 1 || got[0] != "explicit.json" {
		t.Fatalf("explicit merge findings_from overwritten: %v", got)
	}
	if got := spec.Steps[1].FindingsFrom; len(got) != 1 || got[0] != "seed.json" {
		t.Fatalf("finalize not seeded: %v", got)
	}
}

func TestSeedFindingsErrorsWhenNoConsumer(t *testing.T) {
	spec := workflow.Spec{Version: 1, Steps: []workflow.StepEntry{
		{Type: workflow.StepCollectContext},
		{Parallel: []workflow.StepEntry{{Type: workflow.StepReviewPrefix + "security"}}},
	}}
	if err := seedFindings(&spec, []string{"a.json"}); err == nil {
		t.Fatal("expected error when no step consumes the findings")
	}
}

func TestSeedFindingsNoopWithoutFindings(t *testing.T) {
	spec := workflow.Spec{Version: 1, Steps: []workflow.StepEntry{{Type: workflow.StepCollectContext}}}
	if err := seedFindings(&spec, nil); err != nil {
		t.Fatalf("no findings should be a no-op: %v", err)
	}
}
