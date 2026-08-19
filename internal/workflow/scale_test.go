package workflow

import (
	"math"
	"testing"
)

func intPtr(v int) *int { return &v }

func budget(maxSeconds, weight, threshold int) *TimeBudget {
	tb := &TimeBudget{}
	if maxSeconds > 0 {
		tb.MaxSeconds = intPtr(maxSeconds)
	}
	if weight > 0 {
		tb.Weight = intPtr(weight)
	}
	if threshold > 0 {
		tb.SpeedupThreshold = intPtr(threshold)
	}
	return tb
}

func scaleTestSpec() Spec {
	return Spec{
		Version: 1,
		Steps: []StepEntry{
			{Type: StepCollectContext, Config: &StepOverride{TimeBudget: budget(180, 0, 0)}},
			{
				Parallel: []StepEntry{
					{
						Lane: []StepEntry{
							{Type: "review:security", Config: &StepOverride{
								MineReasoning:   &AgentOverride{TimeBudget: budget(90, 0, 0)},
								CompileFindings: &AgentOverride{TimeBudget: budget(60, 0, 0)},
								Nudge:           &AgentOverride{TimeBudget: budget(30, 0, 0)},
							}},
							{Type: "verify:security", Config: &StepOverride{TimeBudget: budget(0, 55, 90)}},
						},
						Config: &StepOverride{TimeBudget: budget(1500, 0, 0)},
					},
				},
			},
			{
				Pipeline: []StepEntry{{Type: StepMerge}},
				Config:   &StepOverride{TimeBudget: budget(1200, 0, 0)},
			},
		},
	}
}

func TestWithScaledTimeBudgetsScalesEveryAbsoluteCap(t *testing.T) {
	scaled := scaleTestSpec().WithScaledTimeBudgets(2)

	if got := *scaled.Steps[0].Config.TimeBudget.MaxSeconds; got != 360 {
		t.Fatalf("context step = %d, want 360", got)
	}
	group := scaled.Steps[1].Parallel[0]
	if got := *group.Config.TimeBudget.MaxSeconds; got != 3000 {
		t.Fatalf("lane group = %d, want 3000", got)
	}
	review := group.Lane[0].Config
	if got := *review.MineReasoning.TimeBudget.MaxSeconds; got != 180 {
		t.Fatalf("mine_reasoning = %d, want 180", got)
	}
	if got := *review.CompileFindings.TimeBudget.MaxSeconds; got != 120 {
		t.Fatalf("compile_findings = %d, want 120", got)
	}
	if got := *review.Nudge.TimeBudget.MaxSeconds; got != 60 {
		t.Fatalf("nudge = %d, want 60", got)
	}
	if got := *scaled.Steps[2].Config.TimeBudget.MaxSeconds; got != 2400 {
		t.Fatalf("pipeline group = %d, want 2400", got)
	}
	// weight is a share of the parent and speedup_threshold a percentage of it, so
	// both already move with the parent; rescaling them would apply the factor twice.
	verify := group.Lane[1].Config.TimeBudget
	if *verify.Weight != 55 || *verify.SpeedupThreshold != 90 || verify.MaxSeconds != nil {
		t.Fatalf("relative budget was rewritten: %+v", verify)
	}
}

// A Spec travels by value while its steps and configs are shared through pointers
// and one slice backing array, so scaling must not reach into the caller's spec.
func TestWithScaledTimeBudgetsLeavesTheOriginalAlone(t *testing.T) {
	original := scaleTestSpec()

	_ = original.WithScaledTimeBudgets(4)

	if got := *original.Steps[0].Config.TimeBudget.MaxSeconds; got != 180 {
		t.Fatalf("original context step = %d, want 180", got)
	}
	if got := *original.Steps[1].Parallel[0].Config.TimeBudget.MaxSeconds; got != 1500 {
		t.Fatalf("original lane group = %d, want 1500", got)
	}
	if got := *original.Steps[1].Parallel[0].Lane[0].Config.MineReasoning.TimeBudget.MaxSeconds; got != 90 {
		t.Fatalf("original mine_reasoning = %d, want 90", got)
	}
}

func TestWithScaledTimeBudgetsIgnoresUnusableFactors(t *testing.T) {
	for _, factor := range []float64{1, 0, -2, math.Inf(1), math.NaN()} {
		scaled := scaleTestSpec().WithScaledTimeBudgets(factor)
		if got := *scaled.Steps[0].Config.TimeBudget.MaxSeconds; got != 180 {
			t.Fatalf("factor %v changed the spec: %d", factor, got)
		}
	}
}

// max_seconds is validated as positive, and a zero would read as "no cap" while
// meaning "already expired"; a runaway factor must not overflow the field either.
func TestWithScaledTimeBudgetsClampsToUsableSeconds(t *testing.T) {
	tiny := scaleTestSpec().WithScaledTimeBudgets(0.0001)
	if got := *tiny.Steps[0].Config.TimeBudget.MaxSeconds; got != 1 {
		t.Fatalf("tiny factor = %d, want 1", got)
	}
	huge := scaleTestSpec().WithScaledTimeBudgets(1e12)
	if got := *huge.Steps[0].Config.TimeBudget.MaxSeconds; got != maxScaledTimeBudgetSeconds {
		t.Fatalf("huge factor = %d, want the clamp %d", got, maxScaledTimeBudgetSeconds)
	}
	if err := validateTimeBudget(huge.Steps[0].Config.TimeBudget); err != nil {
		t.Fatalf("clamped budget is invalid: %v", err)
	}
	if err := validateTimeBudget(tiny.Steps[0].Config.TimeBudget); err != nil {
		t.Fatalf("clamped budget is invalid: %v", err)
	}
}

// The default workflow is the spec an ordinary review runs, so the knob has to
// move its caps and keep it valid.
func TestWithScaledTimeBudgetsKeepsTheDefaultSpecValid(t *testing.T) {
	scaled := DefaultSpec().WithScaledTimeBudgets(3)

	found := 0
	var walk func(entries []StepEntry)
	walk = func(entries []StepEntry) {
		for _, entry := range entries {
			if err := validateStepTimeBudgets(entry); err != nil {
				t.Fatalf("scaled step %q has an invalid budget: %v", entry.Type, err)
			}
			if entry.Config != nil && entry.Config.TimeBudget != nil && entry.Config.TimeBudget.MaxSeconds != nil {
				found++
			}
			walk(entry.Parallel)
			walk(entry.Lane)
			walk(entry.Pipeline)
		}
	}
	walk(scaled.Steps)
	if found == 0 {
		t.Fatal("no absolute budgets found in the default spec; the test no longer covers anything")
	}
	if got := *scaled.Steps[0].Config.TimeBudget.MaxSeconds; got != 540 {
		t.Fatalf("default context budget = %d, want 3x180", got)
	}
}
