package workflow

import "math"

// maxScaledTimeBudgetSeconds bounds a scaled budget so a runaway factor cannot
// overflow the int seconds field or produce a deadline far past any plausible run.
// Ten days is already "no cap" in practice; use --disable-workflow-time-budget to
// say that outright.
const maxScaledTimeBudgetSeconds = 10 * 24 * 60 * 60

// WithScaledTimeBudgets returns the spec with every absolute
// `time_budget.max_seconds` — on steps, on their internal agent overrides, and on
// lane/pipeline/parallel group configs — multiplied by factor. It is the run-level
// knob for "give this review more (or less) time": the spec keeps the execution
// shape it declares, only the magnitude of its caps moves.
//
// `weight` and `speedup_threshold` are deliberately untouched. Both are relative —
// a share of the parent budget and a percentage of it — so they already scale with
// whatever their parent ends up being, and rescaling them would double-apply the
// factor.
//
// A factor of 1 (or anything not positive and finite) returns the spec unchanged,
// so an unset knob cannot reshape a spec. Scaled values are clamped to at least one
// second: max_seconds is validated as positive, and a zero would read as "no cap"
// while meaning "already expired".
//
// Nothing is edited in place. A Spec is passed around by value while its steps and
// configs are shared through pointers and a shared slice backing array, so scaling
// copies every node it rewrites and leaves the original spec — and any other holder
// of it — as it was.
func (s Spec) WithScaledTimeBudgets(factor float64) Spec {
	if factor == 1 || factor <= 0 || math.IsInf(factor, 0) || math.IsNaN(factor) {
		return s
	}
	s.Steps = scaledEntries(s.Steps, factor)
	return s
}

func scaledEntries(entries []StepEntry, factor float64) []StepEntry {
	if len(entries) == 0 {
		return entries
	}
	scaled := make([]StepEntry, len(entries))
	for i, entry := range entries {
		entry.Config = scaledStepOverride(entry.Config, factor)
		entry.Parallel = scaledEntries(entry.Parallel, factor)
		entry.Lane = scaledEntries(entry.Lane, factor)
		entry.Pipeline = scaledEntries(entry.Pipeline, factor)
		scaled[i] = entry
	}
	return scaled
}

func scaledStepOverride(override *StepOverride, factor float64) *StepOverride {
	if override == nil {
		return nil
	}
	copied := *override
	copied.TimeBudget = scaledTimeBudget(override.TimeBudget, factor)
	copied.MineReasoning = scaledAgentOverride(override.MineReasoning, factor)
	copied.CompileFindings = scaledAgentOverride(override.CompileFindings, factor)
	copied.Nudge = scaledAgentOverride(override.Nudge, factor)
	return &copied
}

func scaledAgentOverride(override *AgentOverride, factor float64) *AgentOverride {
	if override == nil {
		return nil
	}
	copied := *override
	copied.TimeBudget = scaledTimeBudget(override.TimeBudget, factor)
	return &copied
}

func scaledTimeBudget(tb *TimeBudget, factor float64) *TimeBudget {
	if tb == nil || tb.MaxSeconds == nil {
		return tb
	}
	seconds := int(math.Round(float64(*tb.MaxSeconds) * factor))
	seconds = min(max(seconds, 1), maxScaledTimeBudgetSeconds)
	scaled := *tb
	scaled.MaxSeconds = &seconds
	return &scaled
}
