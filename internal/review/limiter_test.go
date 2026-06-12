package review

import (
	"context"
	"testing"
	"time"
)

func TestLimiterUnlimitedNeverBlocks(t *testing.T) {
	for _, limit := range []int{0, -1} {
		l := NewLimiter(limit)
		if got := l.Limit(); got != 0 {
			t.Fatalf("NewLimiter(%d).Limit() = %d, want 0", limit, got)
		}
		releases := make([]func(), 0, 100)
		for range 100 {
			_, release, err := l.Acquire(context.Background())
			if err != nil {
				t.Fatalf("NewLimiter(%d).Acquire() = %v, want nil", limit, err)
			}
			releases = append(releases, release)
		}
		for _, release := range releases {
			release()
		}
	}
}

func TestLimiterNilReceiverIsUnlimited(t *testing.T) {
	var l *Limiter
	if got := l.Limit(); got != 0 {
		t.Fatalf("nil limiter Limit() = %d, want 0", got)
	}
	_, release, err := l.Acquire(context.Background())
	if err != nil {
		t.Fatalf("nil limiter Acquire() = %v, want nil", err)
	}
	release()
}

func TestLimiterCapsAndReleases(t *testing.T) {
	l := NewLimiter(2)
	if got := l.Limit(); got != 2 {
		t.Fatalf("Limit() = %d, want 2", got)
	}
	ctx := context.Background()
	_, release1, err := l.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := l.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	// Third acquire must block until a release.
	acquired := make(chan struct{})
	go func() {
		if _, _, err := l.Acquire(ctx); err == nil {
			close(acquired)
		}
	}()
	select {
	case <-acquired:
		t.Fatal("third Acquire succeeded while limiter was full")
	case <-time.After(20 * time.Millisecond):
	}
	release1()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("Acquire did not proceed after Release")
	}
}

func TestLimiterAcquireHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, _, err := NewLimiter(0).Acquire(ctx); err == nil {
		t.Fatal("unlimited Acquire with cancelled ctx = nil, want error")
	}

	full := NewLimiter(1)
	if _, _, err := full.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := full.Acquire(ctx); err == nil {
		t.Fatal("full Acquire with cancelled ctx = nil, want error")
	}
}

// An admitted ctx re-acquiring must pass through without consuming a second
// slot — verify admission happens in the spawn loop, and the agent loop
// acquires again with the same ctx; a second slot would deadlock at limit=1.
func TestLimiterAdmittedContextReentersWithoutSecondSlot(t *testing.T) {
	l := NewLimiter(1)
	ctx, release, err := l.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_, nestedRelease, err := l.Acquire(ctx)
		if err == nil {
			nestedRelease()
			close(done)
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("re-entrant Acquire blocked on its own chain's slot")
	}
	release()
	// The slot must still be held exactly once before release; after release a
	// fresh chain can acquire.
	if _, _, err := l.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLimiterFromContextRoundTrip(t *testing.T) {
	if got := LimiterFromContext(context.Background()); got != nil {
		t.Fatalf("LimiterFromContext(empty) = %v, want nil", got)
	}
	l := NewLimiter(3)
	ctx := WithLimiter(context.Background(), l)
	if got := LimiterFromContext(ctx); got != l {
		t.Fatalf("LimiterFromContext = %v, want installed limiter", got)
	}
}
