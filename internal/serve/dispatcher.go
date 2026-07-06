package serve

import (
	"container/list"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	queueCapacity = 256
	shaLRUSize    = 512
)

// Event is one review request accepted by the handler and queued for a
// worker.
type Event struct {
	Kind        TriggerKind
	ProjectID   int
	ProjectPath string
	IID         int
	// HeadSHA is the head from the webhook payload; the worker re-reads the
	// authoritative SHA before running.
	HeadSHA string
	Group   *Group
}

type jobKey struct {
	ProjectID int
	IID       int
}

const (
	stateQueued = iota
	stateRunning
)

// jobState coalesces events per MR: while queued the newest event wins;
// while running the newest event is parked in pending and re-queued when the
// active review finishes.
type jobState struct {
	status  int
	latest  Event
	pending *Event
	// cancel aborts the running review's context; set while running.
	cancel context.CancelFunc
	// startedAt stamps the running review's start for status/abort replies.
	startedAt time.Time
}

// WorkerConfig is the static per-review configuration shared by all workers.
type WorkerConfig struct {
	Topic      string
	StartEmoji string
	BaseURL    string
	ConfigPath string
	ExtraArgs  []string
	LogDir     string
}

// Dispatcher owns the coalescing queue and the worker pool. All mutable state
// sits behind one mutex; the daemon is concurrent by construction.
type Dispatcher struct {
	mu      sync.Mutex
	states  map[jobKey]*jobState
	queue   chan jobKey
	recent  *shaLRU
	closed  bool
	running int
	dropped int

	runner ReviewRunner
	topics *topicCache
	cfg    WorkerConfig
	log    *slog.Logger

	workers sync.WaitGroup
	// jobCtx outlives the intake context so in-flight reviews survive
	// shutdown until the grace period expires.
	jobCtx    context.Context
	jobCancel context.CancelFunc
}

func NewDispatcher(runner ReviewRunner, lookup TopicLookup, cfg WorkerConfig, log *slog.Logger) *Dispatcher {
	jobCtx, jobCancel := context.WithCancel(context.Background())
	return &Dispatcher{
		states:    make(map[jobKey]*jobState),
		queue:     make(chan jobKey, queueCapacity),
		recent:    newSHALRU(shaLRUSize),
		runner:    runner,
		topics:    newTopicCache(lookup),
		cfg:       cfg,
		log:       log,
		jobCtx:    jobCtx,
		jobCancel: jobCancel,
	}
}

// Enqueue accepts an event from the webhook handler. Never blocks: when the
// queue is full the event is dropped and counted, keeping the handler's
// fast-ack guarantee.
func (d *Dispatcher) Enqueue(event Event) {
	key := jobKey{ProjectID: event.ProjectID, IID: event.IID}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	// Duplicate auto-trigger for an already-reviewed head (GitLab webhook
	// retries): drop before it occupies a queue slot. Manual triggers always
	// pass — the user asked.
	if event.Kind == TriggerAuto && event.HeadSHA != "" && d.recent.Contains(shaKey(event.ProjectID, event.IID, event.HeadSHA)) {
		d.log.Debug("dropping already-reviewed head", "project", event.ProjectPath, "iid", event.IID, "sha", event.HeadSHA)
		return
	}
	if state, ok := d.states[key]; ok {
		switch state.status {
		case stateRunning:
			pending := event
			if state.pending != nil {
				pending = mergeEvents(*state.pending, event)
			}
			state.pending = &pending
		default:
			state.latest = mergeEvents(state.latest, event)
		}
		return
	}
	select {
	case d.queue <- key:
		d.states[key] = &jobState{status: stateQueued, latest: event}
	default:
		d.dropped++
		d.log.Error("job queue full, dropping event", "project", event.ProjectPath, "iid", event.IID, "dropped_total", d.dropped)
	}
}

// mergeEvents coalesces a newer event onto a not-yet-executed one. The newer
// event wins (freshest head SHA), but a pending manual trigger is never
// downgraded: an explicit user request must not lose its topic/draft/LRU
// bypasses to a later auto event.
func mergeEvents(existing, incoming Event) Event {
	if existing.Kind == TriggerManual {
		incoming.Kind = TriggerManual
	}
	return incoming
}

// Start launches the worker pool. Workers stop picking up new jobs once ctx
// is cancelled; the currently running review continues on the dispatcher's
// job context (see Shutdown).
func (d *Dispatcher) Start(ctx context.Context, workers int) {
	for range workers {
		d.workers.Go(func() {
			for {
				// Check cancellation first: with a backlog, the two-way
				// select below could keep picking queued jobs after shutdown
				// began — only already-running reviews get the grace period.
				select {
				case <-ctx.Done():
					return
				default:
				}
				select {
				case <-ctx.Done():
					return
				case key := <-d.queue:
					// The select above picks randomly when both cases are
					// ready; don't start a job picked after cancellation
					// (Shutdown may not have marked the dispatcher closed
					// yet — the HTTP drain runs first).
					if ctx.Err() != nil {
						return
					}
					event, jobCtx, ok := d.take(key)
					if !ok {
						continue
					}
					d.process(jobCtx, event)
					d.finish(key)
				}
			}
		})
	}
}

// Shutdown waits for running reviews up to grace, then cancels their context
// (children receive SIGTERM). Call after the Start context is cancelled.
func (d *Dispatcher) Shutdown(grace time.Duration) {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()

	done := make(chan struct{})
	go func() {
		d.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(grace):
		d.log.Warn("shutdown grace expired, terminating running reviews")
	}
	d.jobCancel()
	<-done
}

// AbortOutcome reports what Abort found and did.
type AbortOutcome struct {
	// Found is true when a queued or running job existed for the MR.
	Found bool
	// Running is true when a running review was cancelled (else it was only
	// queued).
	Running bool
	// Since is the elapsed run time when Running.
	Since time.Duration
}

// JobInfo reports one MR's review state for the status command.
type JobInfo struct {
	Queued  bool
	Running bool
	// Since is the elapsed run time when Running.
	Since time.Duration
	// Pending is true when a re-run is parked behind the running review.
	Pending bool
}

// Stats reports queue depth for /healthz.
func (d *Dispatcher) Stats() (queued, running int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.states) - d.running, d.running
}

// take claims a queued job for execution. The returned context is the job's
// own child of jobCtx so Abort can cancel this one review without touching
// the rest of the pool.
func (d *Dispatcher) take(key jobKey) (Event, context.Context, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	// Once shutdown began, no new review may start: only already-running
	// reviews get the grace period. The worker loop's cancellation checks
	// alone can lose the select race against a non-empty queue.
	if d.closed {
		return Event{}, nil, false
	}
	state, ok := d.states[key]
	if !ok || state.status != stateQueued {
		return Event{}, nil, false
	}
	state.status = stateRunning
	ctx, cancel := context.WithCancel(d.jobCtx)
	state.cancel = cancel
	state.startedAt = time.Now()
	d.running++
	return state.latest, ctx, true
}

// finish completes a job; a pending event received mid-run re-queues the MR.
func (d *Dispatcher) finish(key jobKey) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.running--
	state, ok := d.states[key]
	if !ok {
		return
	}
	if state.cancel != nil {
		state.cancel()
		state.cancel = nil
	}
	state.startedAt = time.Time{}
	if state.pending == nil || d.closed {
		delete(d.states, key)
		return
	}
	state.latest = *state.pending
	state.pending = nil
	state.status = stateQueued
	select {
	case d.queue <- key:
	default:
		d.dropped++
		delete(d.states, key)
		d.log.Error("job queue full, dropping pending re-run", "project", state.latest.ProjectPath, "iid", state.latest.IID)
	}
}

// Abort cancels the MR's review: a queued job is removed (its stale queue key
// is skipped by take), a running job's context is cancelled (the child
// process receives SIGTERM), and any pending re-run is cleared. An abort that
// races a finishing review may find nothing — the reply then says no review
// was running even though one just completed, which is accurate enough.
func (d *Dispatcher) Abort(projectID, iid int) AbortOutcome {
	key := jobKey{ProjectID: projectID, IID: iid}
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.states[key]
	if !ok {
		return AbortOutcome{}
	}
	state.pending = nil
	if state.status == stateRunning {
		if state.cancel != nil {
			state.cancel()
		}
		return AbortOutcome{Found: true, Running: true, Since: time.Since(state.startedAt)}
	}
	delete(d.states, key)
	return AbortOutcome{Found: true}
}

// JobInfo reports the MR's review state for the status command.
func (d *Dispatcher) JobInfo(projectID, iid int) JobInfo {
	key := jobKey{ProjectID: projectID, IID: iid}
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.states[key]
	if !ok {
		return JobInfo{}
	}
	if state.status == stateRunning {
		return JobInfo{Running: true, Since: time.Since(state.startedAt), Pending: state.pending != nil}
	}
	return JobInfo{Queued: true}
}

// markReviewed records an authoritative head SHA so duplicate auto-triggers
// are dropped at enqueue time.
func (d *Dispatcher) markReviewed(projectID, iid int, sha string) {
	if sha == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.recent.Add(shaKey(projectID, iid, sha))
}

func (d *Dispatcher) alreadyReviewed(projectID, iid int, sha string) bool {
	if sha == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.recent.Contains(shaKey(projectID, iid, sha))
}

func shaKey(projectID, iid int, sha string) string {
	return fmt.Sprintf("%d:%d:%s", projectID, iid, sha)
}

// shaLRU is a fixed-size set with least-recently-added eviction. Not
// self-locking: the dispatcher mutex guards it.
type shaLRU struct {
	capacity int
	order    *list.List
	entries  map[string]*list.Element
}

func newSHALRU(capacity int) *shaLRU {
	return &shaLRU{
		capacity: capacity,
		order:    list.New(),
		entries:  make(map[string]*list.Element),
	}
}

func (l *shaLRU) Contains(key string) bool {
	_, ok := l.entries[key]
	return ok
}

func (l *shaLRU) Add(key string) {
	if element, ok := l.entries[key]; ok {
		l.order.MoveToFront(element)
		return
	}
	l.entries[key] = l.order.PushFront(key)
	for l.order.Len() > l.capacity {
		oldest := l.order.Back()
		l.order.Remove(oldest)
		delete(l.entries, oldest.Value.(string))
	}
}
