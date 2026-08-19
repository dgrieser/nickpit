package workflow

import (
	"math"

	"github.com/dgrieser/nickpit/internal/config"
)

const (
	// MinScaledTimeBudgetSeconds is the floor for a scaled cap. max_seconds is
	// validated as positive, and a zero would read as "no cap" while meaning
	// "already expired".
	MinScaledTimeBudgetSeconds = 1

	// MaxScaledTimeBudgetSeconds bounds a scaled cap so a runaway factor cannot
	// overflow the int seconds field or produce a deadline far past any plausible
	// run. Ten days is already "no cap" in practice; use
	// --disable-workflow-time-budget to say that outright.
	MaxScaledTimeBudgetSeconds = 10 * 24 * 60 * 60
)

// TimeBudgetScaleReport describes what a scale actually did to a spec, so a caller
// can report the caps the run will honor instead of only the factor that was
// requested. A clamped cap no longer follows the factor at all.
type TimeBudgetScaleReport struct {
	// Factor is the multiplier that was applied, or 1 when none was.
	Factor float64
	// Caps counts the absolute wall-clock caps that were rewritten — both
	// time_budget.max_seconds and max_reasoning_seconds.
	Caps int
	// Clamped counts the caps whose product fell outside
	// [MinScaledTimeBudgetSeconds, MaxScaledTimeBudgetSeconds] and were pinned to a
	// bound instead.
	Clamped int
	// MinSeconds and MaxSeconds bound the rewritten caps. Both are zero when
	// nothing was scaled.
	MinSeconds int
	MaxSeconds int
}

// Applied reports whether the scale rewrote anything.
func (r TimeBudgetScaleReport) Applied() bool { return r.Caps > 0 }

// WithScaledTimeBudgets returns the spec with every absolute wall-clock cap
// multiplied by factor: `time_budget.max_seconds` and `max_reasoning_seconds`, on
// steps, on their internal agent overrides, and on lane/pipeline group configs. It
// is the run-level knob for "give this review more (or less) time": the spec keeps
// the execution shape it declares, only the magnitude of its caps moves. (A
// `parallel:` group takes no `config:` block of its own — only its lane/pipeline
// children do — so the traversal simply finds nothing to scale there.)
//
// `weight` and `speedup_threshold` are deliberately untouched. Both are relative —
// a share of the parent budget and a percentage of it — so they already scale with
// whatever their parent ends up being, and rescaling them would double-apply the
// factor.
//
// A factor of 1, or one config.UsableTimeBudgetScale rejects, returns the spec
// unchanged with an empty report, so an unset knob cannot reshape a spec. Scaled
// caps are clamped into [MinScaledTimeBudgetSeconds, MaxScaledTimeBudgetSeconds],
// and the report says how many were clamped so a caller can warn that those caps
// no longer follow the factor.
//
// Nothing is edited in place. A Spec is passed around by value while its steps and
// configs are shared through pointers and a shared slice backing array, so scaling
// copies every node it rewrites and leaves the original spec — and any other holder
// of it — as it was.
func (s Spec) WithScaledTimeBudgets(factor float64) (Spec, TimeBudgetScaleReport) {
	report := TimeBudgetScaleReport{Factor: 1}
	if factor == 1 || !config.UsableTimeBudgetScale(factor) {
		return s, report
	}
	report.Factor = factor
	s.Steps = scaledEntries(s.Steps, factor, &report)
	return s, report
}

// ScaleReasoningSeconds applies a time-budget scale to a profile-level
// max_reasoning_seconds. A spec's own overrides are scaled with the spec; this is
// the same rule for the value the profile supplies when no step overrides it.
// Zero means "no cap" and stays zero.
func ScaleReasoningSeconds(seconds int, factor float64) int {
	if seconds == 0 || factor == 1 || !config.UsableTimeBudgetScale(factor) {
		return seconds
	}
	var discard TimeBudgetScaleReport
	return scaledSeconds(seconds, factor, &discard)
}

func scaledEntries(entries []StepEntry, factor float64, report *TimeBudgetScaleReport) []StepEntry {
	if len(entries) == 0 {
		return entries
	}
	scaled := make([]StepEntry, len(entries))
	for i, entry := range entries {
		entry.Config = scaledStepOverride(entry.Config, factor, report)
		entry.Parallel = scaledEntries(entry.Parallel, factor, report)
		entry.Lane = scaledEntries(entry.Lane, factor, report)
		entry.Pipeline = scaledEntries(entry.Pipeline, factor, report)
		scaled[i] = entry
	}
	return scaled
}

func scaledStepOverride(override *StepOverride, factor float64, report *TimeBudgetScaleReport) *StepOverride {
	if override == nil {
		return nil
	}
	copied := *override
	copied.TimeBudget = scaledTimeBudget(override.TimeBudget, factor, report)
	copied.MaxReasoningSeconds = scaledReasoningSeconds(override.MaxReasoningSeconds, factor, report)
	copied.MineReasoning = scaledAgentOverride(override.MineReasoning, factor, report)
	copied.CompileFindings = scaledAgentOverride(override.CompileFindings, factor, report)
	copied.Nudge = scaledAgentOverride(override.Nudge, factor, report)
	// Categorize accepts no time_budget of its own — it shares the verify step's —
	// but it does accept max_reasoning_seconds, so it carries an absolute cap like
	// any other agent override.
	copied.Categorize = scaledAgentOverride(override.Categorize, factor, report)
	return &copied
}

func scaledAgentOverride(override *AgentOverride, factor float64, report *TimeBudgetScaleReport) *AgentOverride {
	if override == nil {
		return nil
	}
	copied := *override
	copied.TimeBudget = scaledTimeBudget(override.TimeBudget, factor, report)
	copied.MaxReasoningSeconds = scaledReasoningSeconds(override.MaxReasoningSeconds, factor, report)
	return &copied
}

func scaledTimeBudget(tb *TimeBudget, factor float64, report *TimeBudgetScaleReport) *TimeBudget {
	if tb == nil || tb.MaxSeconds == nil {
		return tb
	}
	seconds := scaledSeconds(*tb.MaxSeconds, factor, report)
	scaled := *tb
	scaled.MaxSeconds = &seconds
	return &scaled
}

// scaledReasoningSeconds scales max_reasoning_seconds alongside the budget that
// contains it. It is the other absolute wall-clock cap a spec can set — the limit
// on one call's reasoning stream, past which the agent falls back to a lower
// effort — so leaving it fixed would move its ratio to the step budget with the
// factor: a halved lane would die on its own deadline instead of degrading effort,
// and a doubled one would still degrade at the same second. Zero means "no cap"
// and stays zero.
func scaledReasoningSeconds(seconds *int, factor float64, report *TimeBudgetScaleReport) *int {
	if seconds == nil || *seconds == 0 {
		return seconds
	}
	scaled := scaledSeconds(*seconds, factor, report)
	return &scaled
}

// scaledSeconds multiplies an absolute wall-clock cap and clamps the product into
// [MinScaledTimeBudgetSeconds, MaxScaledTimeBudgetSeconds] *before* converting to
// int. An out-of-range float-to-int conversion is implementation-defined — it
// yields minInt on amd64 and arm64 — so clamping after the conversion would turn a
// runaway factor into a one-second deadline, the exact inverse of what was asked
// for. The 32-bit release targets reach that point at a far smaller factor than
// the 64-bit ones, which is why the clamp cannot rely on the width of int at all.
func scaledSeconds(seconds int, factor float64, report *TimeBudgetScaleReport) int {
	exact := math.Round(float64(seconds) * factor)
	clamped := math.Min(math.Max(exact, MinScaledTimeBudgetSeconds), MaxScaledTimeBudgetSeconds)
	scaled := int(clamped)

	report.Caps++
	if clamped != exact {
		report.Clamped++
	}
	if report.MinSeconds == 0 || scaled < report.MinSeconds {
		report.MinSeconds = scaled
	}
	if scaled > report.MaxSeconds {
		report.MaxSeconds = scaled
	}
	return scaled
}
