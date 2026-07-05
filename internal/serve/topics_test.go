package serve

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTopicCacheTTL(t *testing.T) {
	var calls atomic.Int64
	cache := newTopicCache(func(ctx context.Context, group *Group, projectID int) ([]string, error) {
		calls.Add(1)
		return []string{"nickpit"}, nil
	})
	now := time.Now()
	cache.now = func() time.Time { return now }

	for range 3 {
		ok, err := cache.HasTopic(context.Background(), nil, 42, "nickpit")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 (cached)", calls.Load())
	}

	now = now.Add(topicCacheTTL + time.Second)
	if _, err := cache.Topics(context.Background(), nil, 42); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2 (TTL expired)", calls.Load())
	}
}

func TestTopicCacheErrorNotCached(t *testing.T) {
	var calls atomic.Int64
	cache := newTopicCache(func(ctx context.Context, group *Group, projectID int) ([]string, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("boom")
		}
		return []string{"go"}, nil
	})
	if _, err := cache.Topics(context.Background(), nil, 42); err == nil {
		t.Fatal("expected error")
	}
	topics, err := cache.Topics(context.Background(), nil, 42)
	if err != nil || len(topics) != 1 {
		t.Fatalf("topics=%v err=%v", topics, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d", calls.Load())
	}
}

func TestTopicCacheSingleflight(t *testing.T) {
	var calls atomic.Int64
	release := make(chan struct{})
	cache := newTopicCache(func(ctx context.Context, group *Group, projectID int) ([]string, error) {
		calls.Add(1)
		<-release
		return []string{"nickpit"}, nil
	})

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := cache.Topics(context.Background(), nil, 42); err != nil {
				t.Error(err)
			}
		})
	}
	// Give the goroutines a moment to pile up on the same entry, then let the
	// single fetch finish.
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1 (singleflight)", calls.Load())
	}
}

func TestTopicCacheDistinctProjects(t *testing.T) {
	var calls atomic.Int64
	cache := newTopicCache(func(ctx context.Context, group *Group, projectID int) ([]string, error) {
		calls.Add(1)
		return nil, nil
	})
	_, _ = cache.Topics(context.Background(), nil, 1)
	_, _ = cache.Topics(context.Background(), nil, 2)
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
}
