package review

import "context"

// VerifyLimiter caps concurrent verifier agent calls across the whole pipeline
// run — one limiter is shared by every verify step (global or per-vector). A
// zero or negative limit means unlimited: Acquire/Release go through the same
// methods and simply never block, so callers need no mode-specific branching.
type VerifyLimiter struct {
	sem chan struct{} // nil = unlimited
}

// NewVerifyLimiter returns a limiter admitting at most limit concurrent
// holders; limit <= 0 means unlimited.
func NewVerifyLimiter(limit int) *VerifyLimiter {
	if limit <= 0 {
		return &VerifyLimiter{}
	}
	return &VerifyLimiter{sem: make(chan struct{}, limit)}
}

// Acquire blocks until a slot frees or ctx is done. Nil and unlimited limiters
// admit immediately (still honoring an already-cancelled ctx).
func (l *VerifyLimiter) Acquire(ctx context.Context) error {
	if l == nil || l.sem == nil {
		return ctx.Err()
	}
	select {
	case l.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees a slot. It must be paired with a successful Acquire. Safe on
// nil/unlimited limiters.
func (l *VerifyLimiter) Release() {
	if l == nil || l.sem == nil {
		return
	}
	<-l.sem
}

// Limit returns the configured cap; 0 means unlimited.
func (l *VerifyLimiter) Limit() int {
	if l == nil || l.sem == nil {
		return 0
	}
	return cap(l.sem)
}
