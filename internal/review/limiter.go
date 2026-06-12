package review

import "context"

// Limiter caps concurrent LLM agent loops across the whole pipeline run — one
// limiter is shared by every step (reviewers, verifiers, dedupe, merge,
// finalize, summarize). A zero or negative limit means unlimited: Acquire and
// Release go through the same methods and simply never block, so callers need
// no mode-specific branching.
type Limiter struct {
	sem chan struct{} // nil = unlimited
}

// NewLimiter returns a limiter admitting at most limit concurrent holders;
// limit <= 0 means unlimited.
func NewLimiter(limit int) *Limiter {
	if limit <= 0 {
		return &Limiter{}
	}
	return &Limiter{sem: make(chan struct{}, limit)}
}

type limiterCtxKey struct{}
type limiterAdmittedKey struct{}

// WithLimiter installs the run's limiter into ctx so every agent loop started
// under this ctx acquires admission from the same run-global cap.
func WithLimiter(ctx context.Context, l *Limiter) context.Context {
	return context.WithValue(ctx, limiterCtxKey{}, l)
}

// LimiterFromContext returns the limiter installed by WithLimiter, or nil. A
// nil limiter admits immediately, so callers can Acquire unconditionally.
func LimiterFromContext(ctx context.Context) *Limiter {
	l, _ := ctx.Value(limiterCtxKey{}).(*Limiter)
	return l
}

// Acquire blocks until a slot frees or ctx is done. It returns a ctx that
// carries the admission and the matching release func. A ctx that already
// carries admission is admitted immediately with a no-op release: an agent
// chain holds exactly one slot, so a pre-admitted agent loop (verify acquires
// in its ordered spawn loop before the loop runs) cannot deadlock the chain
// against itself. Nil and unlimited limiters admit immediately. An
// already-cancelled ctx is honored before attempting the channel send, which
// would otherwise be chosen pseudo-randomly by select when a slot is free.
func (l *Limiter) Acquire(ctx context.Context) (context.Context, func(), error) {
	if err := ctx.Err(); err != nil {
		return ctx, nil, err
	}
	noop := func() {}
	if l == nil || l.sem == nil {
		return ctx, noop, nil
	}
	if admitted, _ := ctx.Value(limiterAdmittedKey{}).(bool); admitted {
		return ctx, noop, nil
	}
	select {
	case l.sem <- struct{}{}:
		release := func() { <-l.sem }
		return context.WithValue(ctx, limiterAdmittedKey{}, true), release, nil
	case <-ctx.Done():
		return ctx, nil, ctx.Err()
	}
}

// Limit returns the configured cap; 0 means unlimited.
func (l *Limiter) Limit() int {
	if l == nil || l.sem == nil {
		return 0
	}
	return cap(l.sem)
}
