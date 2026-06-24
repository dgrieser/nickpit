package review

import (
	"context"
	"time"

	"github.com/dgrieser/nickpit/internal/workflow"
)

const defaultSpeedupThreshold = 80

type activeTimeBudget struct {
	scope            string
	start            time.Time
	deadline         time.Time
	speedupThreshold int
}

type timeBudgetContextKey struct{}
type timeBudgetThresholdContextKey struct{}

type childTimePlan struct {
	allocated *time.Duration
	optional  bool
}

type timeBudgetStarter struct {
	ctx     context.Context
	cfg     *workflow.TimeBudget
	plan    childTimePlan
	enabled bool
	scope   string
	logf    timeBudgetLogFunc
}

type timeBudgetLogFunc func(context.Context, string, ...any)

func timeBudgetFromContext(ctx context.Context) (activeTimeBudget, bool) {
	budget, ok := ctx.Value(timeBudgetContextKey{}).(activeTimeBudget)
	return budget, ok
}

func newTimeBudgetStarter(ctx context.Context, cfg *workflow.TimeBudget, plan childTimePlan, enabled bool, scope string, logf timeBudgetLogFunc) timeBudgetStarter {
	return timeBudgetStarter{ctx: ctx, cfg: cfg, plan: plan, enabled: enabled, scope: scope, logf: logf}
}

func (s timeBudgetStarter) start() (context.Context, context.CancelFunc, bool) {
	if !s.enabled {
		return s.ctx, func() {}, false
	}
	return withConfiguredTimeBudget(s.ctx, s.cfg, s.plan, s.scope, s.logf)
}

func (s timeBudgetStarter) startOrCanceled() (context.Context, context.CancelFunc) {
	ctx, cancel, skipped := s.start()
	if skipped {
		return alreadyCanceledContext(s.ctx)
	}
	return ctx, cancel
}

func alreadyCanceledContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	cancel()
	return ctx, func() {}
}

func withConfiguredTimeBudget(ctx context.Context, cfg *workflow.TimeBudget, plan childTimePlan, scope string, logf timeBudgetLogFunc) (context.Context, context.CancelFunc, bool) {
	parent, hasParent := timeBudgetFromContext(ctx)
	now := time.Now()
	if plan.optional && hasParent && !parent.deadline.After(now) {
		logTimeBudgetf(ctx, logf, "Workflow time budget skipped: scope=%s reason=parent_deadline_exhausted", budgetScope(scope))
		return ctx, func() {}, true
	}

	threshold := inheritedSpeedupThreshold(ctx)
	if cfg != nil && cfg.SpeedupThreshold != nil {
		threshold = *cfg.SpeedupThreshold
	}

	var deadline time.Time
	if plan.allocated != nil {
		deadline = now.Add(*plan.allocated)
	}
	if cfg != nil && cfg.MaxSeconds != nil {
		maxDeadline := now.Add(time.Duration(*cfg.MaxSeconds) * time.Second)
		if deadline.IsZero() || maxDeadline.Before(deadline) {
			deadline = maxDeadline
		}
	}
	if hasParent && (deadline.IsZero() || parent.deadline.Before(deadline)) {
		deadline = parent.deadline
	}
	if deadline.IsZero() {
		if cfg != nil && cfg.SpeedupThreshold != nil {
			return context.WithValue(ctx, timeBudgetThresholdContextKey{}, threshold), func() {}, false
		}
		return ctx, func() {}, false
	}

	budget := activeTimeBudget{
		scope:            budgetScope(scope),
		start:            now,
		deadline:         deadline,
		speedupThreshold: threshold,
	}
	deadlineCtx, cancel := context.WithDeadline(ctx, deadline)
	deadlineCtx = context.WithValue(deadlineCtx, timeBudgetContextKey{}, budget)
	logTimeBudgetStart(deadlineCtx, logf, budget)
	return deadlineCtx, cancel, false
}

func budgetScope(scope string) string {
	if scope == "" {
		return "unspecified"
	}
	return scope
}

func logTimeBudgetStart(ctx context.Context, logf timeBudgetLogFunc, budget activeTimeBudget) {
	limit := budget.deadline.Sub(budget.start)
	speedupIn := "disabled"
	if budget.speedupThreshold < 100 {
		speedupIn = budgetDuration(time.Duration(float64(limit) * float64(budget.speedupThreshold) / 100))
	}
	logTimeBudgetf(ctx, logf,
		"Workflow time budget started: scope=%s limit=%s deadline_in=%s speedup_threshold=%d%% speedup_in=%s",
		budget.scope, budgetDuration(limit), budgetDuration(time.Until(budget.deadline)), budget.speedupThreshold, speedupIn)
}

func logTimeBudgetf(ctx context.Context, logf timeBudgetLogFunc, format string, args ...any) {
	if logf == nil {
		return
	}
	logf(ctx, format, args...)
}

func timeBudgetLimit(budget activeTimeBudget) time.Duration {
	return budget.deadline.Sub(budget.start)
}

func timeBudgetElapsed(budget activeTimeBudget, now time.Time) time.Duration {
	return now.Sub(budget.start)
}

func timeBudgetRemaining(budget activeTimeBudget, now time.Time) time.Duration {
	return budget.deadline.Sub(now)
}

func timeBudgetOverrun(budget activeTimeBudget, now time.Time) time.Duration {
	overrun := now.Sub(budget.deadline)
	if overrun < 0 {
		return 0
	}
	return overrun
}

func budgetDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Truncate(time.Second).String()
}

func inheritedSpeedupThreshold(ctx context.Context) int {
	if parent, ok := timeBudgetFromContext(ctx); ok {
		return parent.speedupThreshold
	}
	if threshold, ok := ctx.Value(timeBudgetThresholdContextKey{}).(int); ok {
		return threshold
	}
	return defaultSpeedupThreshold
}

func childTimePlans(ctx context.Context, budgets []*workflow.TimeBudget) []childTimePlan {
	plans := make([]childTimePlan, len(budgets))
	parent, ok := timeBudgetFromContext(ctx)
	if !ok {
		for i, budget := range budgets {
			plans[i].optional = budget != nil && budget.Weight != nil && *budget.Weight == 0
		}
		return plans
	}
	remaining := time.Until(parent.deadline)
	weights := resolvedTimeWeights(budgets)
	for i, weight := range weights {
		budget := budgets[i]
		if budget != nil && budget.Weight != nil && *budget.Weight == 0 {
			plans[i].optional = true
			continue
		}
		duration := time.Duration(float64(remaining) * float64(weight) / 100)
		plans[i].allocated = &duration
	}
	return plans
}

func resolvedTimeWeights(budgets []*workflow.TimeBudget) []float64 {
	weights := make([]float64, len(budgets))
	if len(budgets) == 0 {
		return weights
	}
	explicitSum := 0
	unset := 0
	for i, budget := range budgets {
		if budget != nil && budget.Weight != nil {
			weights[i] = float64(*budget.Weight)
			explicitSum += *budget.Weight
			continue
		}
		unset++
	}
	if unset == len(budgets) {
		each := 100.0 / float64(len(budgets))
		for i := range weights {
			weights[i] = each
		}
		return weights
	}
	if unset > 0 {
		remaining := max(100-explicitSum, 0)
		each := float64(remaining) / float64(unset)
		for i, budget := range budgets {
			if budget == nil || budget.Weight == nil {
				weights[i] = each
			}
		}
	}
	return weights
}

func timeBudgetSpeedupDeadline(ctx context.Context) (time.Time, bool) {
	budget, ok := timeBudgetFromContext(ctx)
	if !ok || budget.speedupThreshold >= 100 {
		return time.Time{}, false
	}
	total := budget.deadline.Sub(budget.start)
	if total <= 0 {
		return time.Time{}, false
	}
	soft := budget.start.Add(time.Duration(float64(total) * float64(budget.speedupThreshold) / 100))
	now := time.Now()
	if !soft.After(now) {
		return time.Time{}, false
	}
	return soft, true
}

func timeBudgetUrgentNow(ctx context.Context) bool {
	budget, ok := timeBudgetFromContext(ctx)
	if !ok || budget.speedupThreshold >= 100 {
		return false
	}
	total := budget.deadline.Sub(budget.start)
	if total <= 0 {
		return false
	}
	soft := budget.start.Add(time.Duration(float64(total) * float64(budget.speedupThreshold) / 100))
	now := time.Now()
	return !now.Before(soft) && now.Before(budget.deadline)
}
