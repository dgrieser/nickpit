package workflow

import (
	"maps"
	"math"
	"reflect"
	"slices"
	"strings"
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
								MaxReasoningSeconds: intPtr(300),
								MineReasoning:       &AgentOverride{TimeBudget: budget(90, 0, 0)},
								CompileFindings:     &AgentOverride{TimeBudget: budget(60, 0, 0)},
								Nudge:               &AgentOverride{TimeBudget: budget(30, 0, 0), MaxReasoningSeconds: intPtr(20)},
							}},
							{Type: "verify:security", Config: &StepOverride{
								TimeBudget: budget(0, 55, 90),
								// categorize accepts no time_budget of its own, but it
								// does accept max_reasoning_seconds.
								Categorize: &AgentOverride{MaxReasoningSeconds: intPtr(40)},
							}},
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
	scaled, report := scaleTestSpec().WithScaledTimeBudgets(2)

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
	// max_reasoning_seconds is the other absolute wall-clock cap: leaving it fixed
	// would move its ratio to the step budget with the factor.
	if got := *review.MaxReasoningSeconds; got != 600 {
		t.Fatalf("step max_reasoning_seconds = %d, want 600", got)
	}
	if got := *review.Nudge.MaxReasoningSeconds; got != 40 {
		t.Fatalf("nudge max_reasoning_seconds = %d, want 40", got)
	}
	verifyCfg := group.Lane[1].Config
	if got := *verifyCfg.Categorize.MaxReasoningSeconds; got != 80 {
		t.Fatalf("categorize max_reasoning_seconds = %d, want 80", got)
	}
	// weight is a share of the parent and speedup_threshold a percentage of it, so
	// both already move with the parent; rescaling them would apply the factor twice.
	verify := verifyCfg.TimeBudget
	if *verify.Weight != 55 || *verify.SpeedupThreshold != 90 || verify.MaxSeconds != nil {
		t.Fatalf("relative budget was rewritten: %+v", verify)
	}
	if report.Factor != 2 || report.Caps != 9 || report.Clamped != 0 {
		t.Fatalf("report = %+v, want factor 2 over 9 caps with none clamped", report)
	}
	if report.MinSeconds != 40 || report.MaxSeconds != 3000 {
		t.Fatalf("report range = %ds..%ds, want 40s..3000s", report.MinSeconds, report.MaxSeconds)
	}
}

// A Spec travels by value while its steps and configs are shared through pointers
// and one slice backing array, so scaling must not reach into the caller's spec.
func TestWithScaledTimeBudgetsLeavesTheOriginalAlone(t *testing.T) {
	original := scaleTestSpec()

	_, _ = original.WithScaledTimeBudgets(4)

	if got := *original.Steps[0].Config.TimeBudget.MaxSeconds; got != 180 {
		t.Fatalf("original context step = %d, want 180", got)
	}
	if got := *original.Steps[1].Parallel[0].Config.TimeBudget.MaxSeconds; got != 1500 {
		t.Fatalf("original lane group = %d, want 1500", got)
	}
	review := original.Steps[1].Parallel[0].Lane[0].Config
	if got := *review.MineReasoning.TimeBudget.MaxSeconds; got != 90 {
		t.Fatalf("original mine_reasoning = %d, want 90", got)
	}
	if got := *review.MaxReasoningSeconds; got != 300 {
		t.Fatalf("original max_reasoning_seconds = %d, want 300", got)
	}
}

func TestWithScaledTimeBudgetsIgnoresUnusableFactors(t *testing.T) {
	for _, factor := range []float64{1, 0, -2, math.Inf(1), math.Inf(-1), math.NaN()} {
		scaled, report := scaleTestSpec().WithScaledTimeBudgets(factor)
		if got := *scaled.Steps[0].Config.TimeBudget.MaxSeconds; got != 180 {
			t.Fatalf("factor %v changed the spec: %d", factor, got)
		}
		if report.Applied() || report.Factor != 1 {
			t.Fatalf("factor %v reported %+v, want an unapplied report", factor, report)
		}
	}
}

// max_seconds is validated as positive, and a zero would read as "no cap" while
// meaning "already expired". The upper clamp has to hold for any factor: the
// float-to-int conversion is implementation-defined once the product leaves the
// range of int, so clamping after it would collapse a runaway factor to the floor.
func TestWithScaledTimeBudgetsClampsToUsableSeconds(t *testing.T) {
	tiny, tinyReport := scaleTestSpec().WithScaledTimeBudgets(0.0001)
	if got := *tiny.Steps[0].Config.TimeBudget.MaxSeconds; got != MinScaledTimeBudgetSeconds {
		t.Fatalf("tiny factor = %d, want %d", got, MinScaledTimeBudgetSeconds)
	}
	if tinyReport.Clamped != tinyReport.Caps || tinyReport.Caps == 0 {
		t.Fatalf("tiny report = %+v, want every cap clamped", tinyReport)
	}
	if err := validateTimeBudget(tiny.Steps[0].Config.TimeBudget); err != nil {
		t.Fatalf("clamped budget is invalid: %v", err)
	}

	// 1e12 stays inside int64 after multiplication; the larger factors do not, and
	// on the 32-bit release targets even 1e8 does not.
	for _, factor := range []float64{1e8, 1e12, 1e17, 1e20, 1e300, math.MaxFloat64} {
		huge, hugeReport := scaleTestSpec().WithScaledTimeBudgets(factor)
		if got := *huge.Steps[0].Config.TimeBudget.MaxSeconds; got != MaxScaledTimeBudgetSeconds {
			t.Fatalf("factor %v = %d, want the clamp %d", factor, got, MaxScaledTimeBudgetSeconds)
		}
		if got := *huge.Steps[1].Parallel[0].Lane[0].Config.MaxReasoningSeconds; got != MaxScaledTimeBudgetSeconds {
			t.Fatalf("factor %v max_reasoning_seconds = %d, want the clamp %d", factor, got, MaxScaledTimeBudgetSeconds)
		}
		if hugeReport.Clamped != hugeReport.Caps || hugeReport.Caps == 0 {
			t.Fatalf("factor %v report = %+v, want every cap clamped", factor, hugeReport)
		}
		if err := validateTimeBudget(huge.Steps[0].Config.TimeBudget); err != nil {
			t.Fatalf("clamped budget is invalid: %v", err)
		}
	}
}

// The default workflow is the spec an ordinary review runs, so the knob has to
// move its caps and keep it valid.
func TestWithScaledTimeBudgetsKeepsTheDefaultSpecValid(t *testing.T) {
	scaled, report := DefaultSpec().WithScaledTimeBudgets(3)

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
	if report.Caps < found || report.Clamped != 0 {
		t.Fatalf("report = %+v, want at least %d caps and none clamped", report, found)
	}
}

func TestScaleReasoningSeconds(t *testing.T) {
	// Zero is "no cap" and a scaled no-cap is still no cap.
	if got := ScaleReasoningSeconds(0, 2); got != 0 {
		t.Fatalf("no cap scaled to %d, want 0", got)
	}
	for _, factor := range []float64{1, 0, -2, math.Inf(1), math.NaN()} {
		if got := ScaleReasoningSeconds(300, factor); got != 300 {
			t.Fatalf("factor %v changed 300 to %d", factor, got)
		}
	}
	if got := ScaleReasoningSeconds(300, 2); got != 600 {
		t.Fatalf("300 scaled by 2 = %d, want 600", got)
	}
	if got := ScaleReasoningSeconds(300, 1e300); got != MaxScaledTimeBudgetSeconds {
		t.Fatalf("runaway factor = %d, want the clamp %d", got, MaxScaledTimeBudgetSeconds)
	}
	if got := ScaleReasoningSeconds(300, 1e-9); got != MinScaledTimeBudgetSeconds {
		t.Fatalf("tiny factor = %d, want the floor %d", got, MinScaledTimeBudgetSeconds)
	}
}

// absoluteCapFields returns the fields of an override struct that carry an
// absolute wall-clock cap, either directly (a time_budget or a *_seconds value) or
// through a nested agent override.
func absoluteCapFields(typ reflect.Type) []string {
	timeBudget := reflect.TypeFor[*TimeBudget]()
	agent := reflect.TypeFor[*AgentOverride]()
	var fields []string
	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.Type == timeBudget || field.Type == agent || strings.HasSuffix(field.Name, "Seconds") {
			fields = append(fields, field.Name)
		}
	}
	return fields
}

// The scale enumerates the cap carriers it rewrites, so a new one added to either
// override struct would silently escape --time-budget-scale: no compile error, and
// every existing test still passes. Pin the set here instead, so adding a field
// fails until WithScaledTimeBudgets covers it.
func TestScaleCoversEveryAbsoluteCapField(t *testing.T) {
	want := map[string][]string{
		"StepOverride":  {"TimeBudget", "MaxReasoningSeconds", "MineReasoning", "CompileFindings", "Nudge", "Categorize"},
		"AgentOverride": {"TimeBudget", "MaxReasoningSeconds"},
	}
	got := map[string][]string{
		"StepOverride":  absoluteCapFields(reflect.TypeFor[StepOverride]()),
		"AgentOverride": absoluteCapFields(reflect.TypeFor[AgentOverride]()),
	}
	for _, name := range slices.Sorted(maps.Keys(want)) {
		if !slices.Equal(got[name], want[name]) {
			t.Fatalf("%s carries caps in %v, want %v; scale the new field in WithScaledTimeBudgets and extend this list",
				name, got[name], want[name])
		}
	}
}
