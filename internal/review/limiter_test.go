package review

import (
	"context"
	"testing"
	"time"
)

func TestVerifyLimiterUnlimitedNeverBlocks(t *testing.T) {
	for _, limit := range []int{0, -1} {
		l := NewVerifyLimiter(limit)
		if got := l.Limit(); got != 0 {
			t.Fatalf("NewVerifyLimiter(%d).Limit() = %d, want 0", limit, got)
		}
		for range 100 {
			if err := l.Acquire(context.Background()); err != nil {
				t.Fatalf("NewVerifyLimiter(%d).Acquire() = %v, want nil", limit, err)
			}
		}
		for range 100 {
			l.Release()
		}
	}
}

func TestVerifyLimiterNilReceiverIsUnlimited(t *testing.T) {
	var l *VerifyLimiter
	if got := l.Limit(); got != 0 {
		t.Fatalf("nil limiter Limit() = %d, want 0", got)
	}
	if err := l.Acquire(context.Background()); err != nil {
		t.Fatalf("nil limiter Acquire() = %v, want nil", err)
	}
	l.Release()
}

func TestVerifyLimiterCapsAndReleases(t *testing.T) {
	l := NewVerifyLimiter(2)
	if got := l.Limit(); got != 2 {
		t.Fatalf("Limit() = %d, want 2", got)
	}
	ctx := context.Background()
	if err := l.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if err := l.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	// Third acquire must block until a release.
	acquired := make(chan struct{})
	go func() {
		if err := l.Acquire(ctx); err == nil {
			close(acquired)
		}
	}()
	select {
	case <-acquired:
		t.Fatal("third Acquire succeeded while limiter was full")
	case <-time.After(20 * time.Millisecond):
	}
	l.Release()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("Acquire did not proceed after Release")
	}
}

func TestVerifyLimiterAcquireHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := NewVerifyLimiter(0).Acquire(ctx); err == nil {
		t.Fatal("unlimited Acquire with cancelled ctx = nil, want error")
	}

	full := NewVerifyLimiter(1)
	if err := full.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := full.Acquire(ctx); err == nil {
		t.Fatal("full Acquire with cancelled ctx = nil, want error")
	}
}
