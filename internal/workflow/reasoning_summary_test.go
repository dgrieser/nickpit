package workflow

import "testing"

func TestReasoningSummaryDefaultsAndScaling(t *testing.T) {
	defaults := ReasoningSummaryOverride(nil)
	if *defaults.Model != SmallModelAlias || *defaults.TimeBudget.Weight != 25 || *defaults.TimeBudget.MaxSeconds != 30 || *defaults.TimeBudget.SpeedupThreshold != 100 {
		t.Fatalf("defaults = %+v", defaults)
	}
	zero := 0
	cfg := &StepOverride{SummarizeReasoning: &AgentOverride{TimeBudget: &TimeBudget{Weight: &zero}}}
	resolved := ReasoningSummaryOverride(cfg)
	if *resolved.TimeBudget.Weight != 0 || cfg.SummarizeReasoning.Model != nil || cfg.SummarizeReasoning.TimeBudget.MaxSeconds != nil {
		t.Fatal("defaults overwrote explicit zero or mutated input")
	}
	spec := Spec{Version: SpecVersion, Steps: []StepEntry{{Type: StepCollectContext}, {Type: StepVerify}}}
	scaled, report := spec.WithScaledTimeBudgets(2)
	if report.Caps != 2 || spec.Steps[0].Config != nil {
		t.Fatalf("report = %+v or input mutated", report)
	}
	for _, step := range scaled.Steps {
		o := ReasoningSummaryOverride(step.Config)
		if *o.TimeBudget.MaxSeconds != 60 || *o.TimeBudget.Weight != 25 || *o.TimeBudget.SpeedupThreshold != 100 {
			t.Fatalf("scaled summary = %+v", o.TimeBudget)
		}
	}
}

func TestReasoningSummaryValidation(t *testing.T) {
	for _, step := range []string{StepCollectContext, StepVerify, StepVerifyPrefix + "security", StepMerge, StepReviewPrefix + "security", StepSummarize} {
		for _, weight := range []int{-1, 0, 25, 99, 100} {
			err := validateStepTimeBudgets(StepEntry{Type: step, Config: &StepOverride{SummarizeReasoning: &AgentOverride{TimeBudget: &TimeBudget{Weight: &weight}}}})
			valid := StepSupportsReasoningSummary(step) && weight >= 0 && weight < 100
			if (err == nil) != valid {
				t.Errorf("step=%s weight=%d error=%v", step, weight, err)
			}
		}
	}
	_, err := Load(writeSpec(t, "version: 1\nsteps:\n  - type: collect-context\n    config:\n      summarize_reasoning:\n        model: '@small'\n        time_budget:\n          weight: 0\n"))
	if err != nil {
		t.Fatal(err)
	}
}
