package workflow

import (
	"fmt"

	"github.com/dgrieser/nickpit/workflows"
)

// UpdateSpec uses the shared YAML parser, but is only available as an embedded
// workflow. The update step is deliberately not part of public spec validation.
func UpdateSpec() Spec {
	spec, err := parseSpec(workflows.Update())
	if err != nil {
		panic(fmt.Sprintf("workflow: invalid embedded update workflow: %v", err))
	}
	// These stages depend on their predecessors; reject an invalid built-in
	// definition before running any agents.
	want := []string{"update", StepVerdict, StepSummarize}
	if spec.Version != SpecVersion || spec.Profile != "" || len(spec.Steps) != len(want) {
		panic("workflow: invalid embedded update workflow structure")
	}
	for i, step := range spec.Steps {
		if step.Type != want[i] || step.IsParallel() || step.IsLane() || step.IsPipeline() || len(step.FindingsFrom) != 0 {
			panic("workflow: invalid embedded update workflow stage")
		}
	}
	return spec
}

type UpdateWorkflowStages struct {
	Update    StepEntry
	Verdict   StepEntry
	Summarize StepEntry
}

// UpdateStages names the fixed stages so correction orchestration does not need
// dynamic dispatch or positional indexes.
func UpdateStages() UpdateWorkflowStages {
	steps := UpdateSpec().Steps
	return UpdateWorkflowStages{Update: steps[0], Verdict: steps[1], Summarize: steps[2]}
}
