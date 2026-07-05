package serve

import (
	"context"
	"slices"
	"sync"
	"time"
)

const topicCacheTTL = 5 * time.Minute

// TopicLookup fetches the current topics of a project using the group whose
// token has access to it.
type TopicLookup func(ctx context.Context, group *Group, projectID int) ([]string, error)

// GitLabTopicLookup queries the project via the group's API client.
func GitLabTopicLookup(ctx context.Context, group *Group, projectID int) ([]string, error) {
	project, err := group.Client.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return project.Topics, nil
}

type topicEntry struct {
	topics  []string
	fetched time.Time
	// pending serializes concurrent lookups for the same project: the first
	// caller closes it after filling the entry (singleflight).
	pending chan struct{}
}

// topicCache caches project topics with a TTL so a burst of webhook events
// for one project costs a single GET /projects/:id.
type topicCache struct {
	mu      sync.Mutex
	entries map[int]*topicEntry
	lookup  TopicLookup
	now     func() time.Time
}

func newTopicCache(lookup TopicLookup) *topicCache {
	return &topicCache{
		entries: make(map[int]*topicEntry),
		lookup:  lookup,
		now:     time.Now,
	}
}

// Topics returns the project's topics, fetching at most once per TTL window.
// Errors are not cached: the next call retries.
func (c *topicCache) Topics(ctx context.Context, group *Group, projectID int) ([]string, error) {
	for {
		c.mu.Lock()
		entry, ok := c.entries[projectID]
		if ok && entry.pending == nil && c.now().Sub(entry.fetched) < topicCacheTTL {
			topics := entry.topics
			c.mu.Unlock()
			return topics, nil
		}
		if ok && entry.pending != nil {
			pending := entry.pending
			c.mu.Unlock()
			select {
			case <-pending:
				// The fetching caller finished; loop to read the result (or
				// retry if it errored and removed the entry).
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}
		entry = &topicEntry{pending: make(chan struct{})}
		c.entries[projectID] = entry
		c.mu.Unlock()

		topics, err := c.lookup(ctx, group, projectID)
		c.mu.Lock()
		if err != nil {
			delete(c.entries, projectID)
		} else {
			entry.topics = topics
			entry.fetched = c.now()
		}
		close(entry.pending)
		entry.pending = nil
		if err == nil {
			c.mu.Unlock()
			return topics, nil
		}
		c.mu.Unlock()
		return nil, err
	}
}

// HasTopic reports whether the project carries the exact topic.
func (c *topicCache) HasTopic(ctx context.Context, group *Group, projectID int, topic string) (bool, error) {
	topics, err := c.Topics(ctx, group, projectID)
	if err != nil {
		return false, err
	}
	return slices.Contains(topics, topic), nil
}
