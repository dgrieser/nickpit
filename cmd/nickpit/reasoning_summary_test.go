package main

import (
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/workflow"
)

func TestReasoningSummaryCapabilityRouting(t *testing.T) {
	zero := 0
	concrete := "concrete-model"
	alias := workflow.SmallModelAlias
	for _, tc := range []struct {
		name      string
		cfg       *workflow.StepOverride
		req       model.ReviewRequest
		wantSmall bool
	}{
		{name: "implicit text helper", wantSmall: true},
		{name: "budgets disabled", req: model.ReviewRequest{DisableWorkflowTimeBudget: true}},
		{name: "extraction disabled", req: model.ReviewRequest{DisableReasoningExtract: true}},
		{name: "zero weight", cfg: &workflow.StepOverride{SummarizeReasoning: &workflow.AgentOverride{TimeBudget: &workflow.TimeBudget{Weight: &zero}}}},
		{name: "concrete primary", cfg: &workflow.StepOverride{SummarizeReasoning: &workflow.AgentOverride{Model: &concrete}}},
		{name: "concrete inherits small", cfg: &workflow.StepOverride{Model: &alias, SummarizeReasoning: &workflow.AgentOverride{Model: &concrete}}, wantSmall: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := workflow.Spec{Version: workflow.SpecVersion, Steps: []workflow.StepEntry{{Type: workflow.StepCollectContext, Config: tc.cfg}}}
			got := modelRequirementsForSpec(spec, tc.req, true)
			if got.Uses() != tc.wantSmall {
				t.Fatalf("small=%+v, want use=%v", got, tc.wantSmall)
			}
			if tc.cfg == nil && tc.wantSmall && got != textModelRequirements() {
				t.Fatalf("helper requires more than text: %+v", got)
			}
			if tc.cfg != nil && tc.cfg.Model == &alias && modelRequirementsForSpec(spec, tc.req, false).Uses() {
				t.Fatal("concrete model moved to primary endpoint")
			}
		})
	}
}
