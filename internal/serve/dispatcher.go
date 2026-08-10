package serve

import (
	"container/list"
	"context"
	"fmt"
	"log/slog"
	"slices"
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
	// AckNoteIDs are the command notes the handler acknowledged for this review
	// (with the ack emoji). The worker replaces that reaction with the review
	// outcome, so the comment that asked for the review shows how it ended.
	// Coalesced events contribute their notes too — every asker gets an answer.
	AckNoteIDs []int
	// StartEmojis and AckEmojis are every configured name under which this job
	// may have placed its managed reactions. Keeping the names on the event lets
	// journal restore clean markers from an older configuration.
	StartEmojis []string
	AckEmojis   []string
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
	// persisted says the complete unfinished event represented by this state
	// was written successfully. A configured journal is not proof of durability:
	// its storage can become full or read-only after startup.
	persisted bool
	// cancel aborts the running review's context; set while running.
	cancel context.CancelFunc
	// startedAt stamps the running review's start for status/abort replies.
	startedAt time.Time
}

// WorkerConfig is the static per-review configuration shared by all workers.
type WorkerConfig struct {
	Topic string
	// StartEmoji marks the merge request while a review runs; AckEmoji is the
	// same marker on the command note (awarded by the handler). Both are
	// replaced when the review ends: DoneEmoji when it landed, FailEmoji when it
	// did not. Any of them may be "" — the emoji is then never placed, and
	// nothing replaces it either.
	StartEmoji string
	AckEmoji   string
	DoneEmoji  string
	FailEmoji  string
	BaseURL    string
	ConfigPath string
	ExtraArgs  []string
	LogDir     string
}

// Dispatcher owns the coalescing queue and the worker pool. All mutable state
// sits behind one mutex; the daemon is concurrent by construction.
type Dispatcher struct {
	mu     sync.Mutex
	states map[jobKey]*jobState
	queue  chan jobKey
	// overflow parks accepted keys when the queue channel is full. Those events
	// must not be dropped; take moves them into freed queue slots. Restored jobs
	// and re-runs land here — new webhook events still get backpressure via
	// Enqueue.
	overflow []jobKey
	recent   *shaLRU
	closed   bool
	running  int
	dropped  int

	runner ReviewRunner
	topics *topicCache
	cfg    WorkerConfig
	// journal persists accepted-but-unfinished jobs across restarts; nil
	// disables journaling (queued jobs then release their ack reactions at
	// shutdown instead of resuming).
	journal *Journal
	log     *slog.Logger

	workers sync.WaitGroup
	// jobCtx outlives the intake context so in-flight reviews survive
	// shutdown until the grace period expires.
	jobCtx    context.Context
	jobCancel context.CancelFunc
}

func NewDispatcher(runner ReviewRunner, lookup TopicLookup, journal *Journal, cfg WorkerConfig, log *slog.Logger) *Dispatcher {
	jobCtx, jobCancel := context.WithCancel(context.Background())
	return &Dispatcher{
		states:    make(map[jobKey]*jobState),
		queue:     make(chan jobKey, queueCapacity),
		recent:    newSHALRU(shaLRUSize),
		runner:    runner,
		topics:    newTopicCache(lookup),
		cfg:       cfg,
		journal:   journal,
		log:       log,
		jobCtx:    jobCtx,
		jobCancel: jobCancel,
	}
}

// AckEmoji is the reaction the handler awards on an accepted review command
// note; the workers revoke it at settle time. Exposed so both sides read the
// one configured name — two independently plumbed copies could drift, and a
// settle revoking a name that was never awarded silently strands the ack.
func (d *Dispatcher) AckEmoji() string {
	return d.cfg.AckEmoji
}

// Restore re-enqueues the jobs a previous daemon process journaled but never
// finished, so a restart (crash, upgrade) neither loses accepted reviews nor
// strands their acknowledged command notes. Groups are re-resolved from the
// current config; jobs whose group vanished are dropped with a warning. Call
// before Start.
func (d *Dispatcher) Restore(groups *GroupSet) int {
	if d.journal == nil {
		return 0
	}
	resumed := 0
	for _, entry := range d.journal.load() {
		group := groups.Match(entry.ProjectPath)
		if group == nil {
			d.log.Warn("journal: dropping job, no configured group matches", "project", entry.ProjectPath, "iid", entry.IID)
			d.journal.remove(entry.ProjectID, entry.IID)
			continue
		}
		event := Event{
			Kind:        parseTriggerKind(entry.Kind),
			ProjectID:   entry.ProjectID,
			ProjectPath: entry.ProjectPath,
			IID:         entry.IID,
			HeadSHA:     entry.HeadSHA,
			Group:       group,
			AckNoteIDs:  entry.AckNoteIDs,
			StartEmojis: entry.StartEmojis,
			AckEmojis:   entry.AckEmojis,
		}
		key := jobKey{ProjectID: event.ProjectID, IID: event.IID}
		d.mu.Lock()
		d.states[key] = &jobState{status: stateQueued, latest: event, persisted: true}
		select {
		case d.queue <- key:
		default:
			d.overflow = append(d.overflow, key)
		}
		d.mu.Unlock()
		resumed++
	}
	return resumed
}

// Enqueue accepts an event from the webhook handler. Never blocks, keeping the
// handler's fast-ack guarantee. It reports whether the event was accepted:
// queued, coalesced onto an existing job, or deliberately dropped as an
// already-reviewed duplicate all count as accepted. False means the event was
// LOST — the dispatcher is closed (shutdown) or the queue is full — and the
// webhook must be answered with a non-2xx status so GitLab redelivers it.
func (d *Dispatcher) Enqueue(event Event) bool {
	d.trackAcknowledgement(&event)
	key := jobKey{ProjectID: event.ProjectID, IID: event.IID}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return false
	}
	// Duplicate auto-trigger for an already-reviewed head (GitLab webhook
	// retries): drop before it occupies a queue slot. Manual triggers always
	// pass — the user asked. The drop is intentional, so it counts as
	// accepted: a redelivery would only be dropped again.
	if event.Kind == TriggerAuto && event.HeadSHA != "" && d.recent.Contains(shaKey(event.ProjectID, event.IID, event.HeadSHA)) {
		d.log.Debug("dropping already-reviewed head", "project", event.ProjectPath, "iid", event.IID, "sha", event.HeadSHA)
		return true
	}
	if state, ok := d.states[key]; ok {
		var dropped []int
		var releaseEvent Event
		switch state.status {
		case stateRunning:
			pending := event
			if state.pending != nil {
				pending, dropped = mergeEvents(*state.pending, event)
			}
			state.pending = &pending
			// The journal holds the job's would-be re-run after a crash: the
			// running review never settled, so its notes AND the pending ones
			// still need their flip. Notes omitted by this second merge would
			// be absent after a crash, so release them now and remove them from
			// the pending rerun's settlement set.
			resume, omitted := mergeEvents(state.latest, pending)
			releaseEvent = pending
			if len(omitted) > 0 {
				dropped = appendUniqueInts(dropped, omitted...)
				state.pending.AckNoteIDs = removeInts(state.pending.AckNoteIDs, omitted)
				resume, _ = mergeEvents(state.latest, *state.pending)
			}
			state.persisted = d.journal.persist(resume)
		default:
			state.latest, dropped = mergeEvents(state.latest, event)
			releaseEvent = state.latest
			state.persisted = d.journal.persist(state.latest)
		}
		if len(dropped) > 0 {
			// Already acknowledged, but over the per-job cap: no settle will
			// ever flip them, so take the ack back now instead of leaving the
			// notes reading as in progress forever.
			d.log.Warn("ack note cap exceeded, releasing dropped notes", "project", event.ProjectPath, "iid", event.IID, "notes", dropped)
			go d.releaseAcks(releaseEvent, dropped)
		}
		return true
	}
	select {
	case d.queue <- key:
		d.states[key] = &jobState{
			status:    stateQueued,
			latest:    event,
			persisted: d.journal.persist(event),
		}
		return true
	default:
		d.dropped++
		d.log.Error("job queue full, dropping event", "project", event.ProjectPath, "iid", event.IID, "dropped_total", d.dropped)
		return false
	}
}

// mergeEvents coalesces a newer event onto a not-yet-executed one. The newer
// event wins (freshest head SHA), but a pending manual trigger is never
// downgraded: an explicit user request must not lose its topic/draft/LRU
// bypasses to a later auto event. Acknowledged command notes accumulate instead
// of being replaced: each one wears the in-progress reaction and must be flipped
// to the outcome when the coalesced review ends. Notes dropped over the per-job
// cap are returned so the caller can release their ack reaction.
func mergeEvents(existing, incoming Event) (Event, []int) {
	if existing.Kind == TriggerManual {
		incoming.Kind = TriggerManual
	}
	merged, dropped := mergeAckNotes(existing.AckNoteIDs, incoming.AckNoteIDs)
	incoming.AckNoteIDs = merged
	incoming.StartEmojis = appendUniqueStrings(existing.StartEmojis, incoming.StartEmojis...)
	incoming.AckEmojis = appendUniqueStrings(existing.AckEmojis, incoming.AckEmojis...)
	return incoming, dropped
}

// trackAcknowledgement stamps the name already awarded by the handler onto the
// event before it is journaled. Reactions are disabled defensively if a caller
// constructs a group without the resolved identity required for safe cleanup.
// Start reaction names are recorded later by take, immediately before a worker
// can place them; queued jobs have not placed a start marker yet.
func (d *Dispatcher) trackAcknowledgement(event *Event) {
	if event.Group == nil || event.Group.BotUserID == 0 {
		return
	}
	if len(event.AckNoteIDs) > 0 && d.cfg.AckEmoji != "" {
		event.AckEmojis = appendUniqueStrings(event.AckEmojis, d.cfg.AckEmoji)
	}
}

func appendUniqueStrings(existing []string, incoming ...string) []string {
	merged := slices.Clone(existing)
	for _, value := range incoming {
		if value != "" && !slices.Contains(merged, value) {
			merged = append(merged, value)
		}
	}
	return merged
}

func appendUniqueInts(existing []int, incoming ...int) []int {
	merged := slices.Clone(existing)
	for _, value := range incoming {
		if !slices.Contains(merged, value) {
			merged = append(merged, value)
		}
	}
	return merged
}

func removeInts(values, removed []int) []int {
	return slices.DeleteFunc(slices.Clone(values), func(value int) bool {
		return slices.Contains(removed, value)
	})
}

// maxAckNotes bounds how many command notes one coalesced review carries, so a
// flood of "/<keyword> review" comments on a single MR cannot grow the event or
// the number of settle requests without limit. The oldest notes are kept: they
// have worn the in-progress reaction the longest. Dropped notes are reported so
// their ack reaction can be released — every asker gets an answer, or at least
// no comment reads as in progress forever.
const maxAckNotes = 32

func mergeAckNotes(existing, incoming []int) (merged, dropped []int) {
	merged = slices.Clone(existing)
	for _, noteID := range incoming {
		if slices.Contains(merged, noteID) {
			continue
		}
		if len(merged) >= maxAckNotes {
			dropped = append(dropped, noteID)
			continue
		}
		merged = append(merged, noteID)
	}
	return merged, dropped
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
					// (CloseIntake may not have run yet if the daemon is
					// stopping because the listener failed).
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

// CloseIntake marks the dispatcher closed so Enqueue rejects every further
// event. Call it before draining in-flight HTTP handlers: the workers stop
// with the intake context, so an event accepted by a still-draining handler
// after that point would be acknowledged with a 2xx and never processed.
func (d *Dispatcher) CloseIntake() {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
}

// Shutdown waits for running reviews up to grace, then cancels their context
// (children receive SIGTERM). Call after the Start context is cancelled;
// idempotent with an earlier CloseIntake.
func (d *Dispatcher) Shutdown(grace time.Duration) {
	d.CloseIntake()

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
	d.releaseUnfinished()
}

// releaseUnfinished handles the jobs shutdown leaves behind: queued jobs that
// never ran and pending re-runs parked behind a review (the workers are done,
// so nothing here will ever settle in this process). With a journal they stay
// on disk and resume after restart; without one, their ack reactions are
// revoked so no command note reads as in progress forever.
func (d *Dispatcher) releaseUnfinished() {
	type unfinished struct {
		event     Event
		persisted bool
	}
	d.mu.Lock()
	remaining := make([]unfinished, 0, len(d.states))
	for _, state := range d.states {
		event := state.latest
		if state.pending != nil {
			event, _ = mergeEvents(event, *state.pending)
		}
		persisted := state.persisted
		if d.journal != nil && !persisted {
			// Retry once at shutdown: transient storage failures need not force
			// journal-less degradation if the state directory recovered.
			persisted = d.journal.persist(event)
		}
		remaining = append(remaining, unfinished{event: event, persisted: persisted})
	}
	d.mu.Unlock()
	if len(remaining) == 0 {
		return
	}
	if d.journal != nil {
		persisted := 0
		for _, job := range remaining {
			if job.persisted {
				persisted++
			}
		}
		if persisted > 0 {
			d.log.Info("shutdown: unfinished review jobs journaled for resume", "count", persisted, "dir", d.journal.Dir())
		}
		if persisted == len(remaining) {
			return
		}
		d.log.Warn("shutdown: releasing acknowledgements for jobs not persisted", "count", len(remaining)-persisted)
	}
	var wg sync.WaitGroup
	for _, job := range remaining {
		if job.persisted {
			continue
		}
		event := job.event
		if len(event.AckNoteIDs) == 0 {
			continue
		}
		wg.Go(func() { d.releaseAcks(event, event.AckNoteIDs) })
	}
	wg.Wait()
}

// releaseAcks revokes the ack reaction from command notes whose review will
// never settle them (aborted while queued, dropped over the ack-note cap, or
// discarded at shutdown without a journal).
func (d *Dispatcher) releaseAcks(event Event, notes []int) {
	if len(notes) == 0 {
		return
	}
	log := d.log.With("project", event.ProjectPath, "iid", event.IID)
	d.settle(context.Background(), event, reactions{notes: notes}, outcomeAborted, log)
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
	// The receive that delivered key freed a queue slot; move parked re-run
	// keys into freed slots so the overflow drains as workers cycle.
	d.drainOverflowLocked()
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
	if state.latest.Group != nil && state.latest.Group.BotUserID != 0 && d.cfg.StartEmoji != "" && !slices.Contains(state.latest.StartEmojis, d.cfg.StartEmoji) {
		// Persist the name before the worker can place it. A crash after the
		// remote award but before any later local update must still leave the
		// next process enough information to clean the marker.
		state.latest.StartEmojis = appendUniqueStrings(state.latest.StartEmojis, d.cfg.StartEmoji)
		state.persisted = d.journal.persist(state.latest)
	}
	ctx, cancel := context.WithCancel(d.jobCtx)
	state.cancel = cancel
	state.startedAt = time.Now()
	d.running++
	return state.latest, ctx, true
}

// finish completes a job. A pending event received mid-run re-queues the MR
// at the back of the queue — behind other waiting MRs, so a busy MR cannot
// monopolize a worker. The re-run was already acknowledged with a 2xx at
// enqueue time, so a full queue must not drop it: it parks in the overflow
// and take moves it into the next freed slot. During shutdown it cannot run
// in this process; it stays journaled for the restart, or — without a journal
// — releaseUnfinished revokes its ack reactions on the way out.
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
	if state.pending == nil {
		delete(d.states, key)
		d.journal.remove(key.ProjectID, key.IID)
		return
	}
	state.latest = *state.pending
	state.pending = nil
	state.status = stateQueued
	state.persisted = d.journal.persist(state.latest)
	if d.closed {
		if d.journal == nil {
			d.dropped++
			d.log.Warn("shutdown: dropping pending re-run (no state_dir configured)", "project", state.latest.ProjectPath, "iid", state.latest.IID)
		}
		return
	}
	select {
	case d.queue <- key:
	default:
		d.overflow = append(d.overflow, key)
	}
}

// drainOverflowLocked moves parked re-run keys into free queue slots,
// preserving their order. Callers hold d.mu.
func (d *Dispatcher) drainOverflowLocked() {
	for len(d.overflow) > 0 {
		select {
		case d.queue <- d.overflow[0]:
			d.overflow = d.overflow[1:]
		default:
			return
		}
	}
}

// Abort cancels the MR's review: a queued job is removed (its stale queue key
// is skipped by take), a running job's context is cancelled (the child
// process receives SIGTERM), and any pending re-run is cleared. An abort that
// races a finishing review may find nothing — the reply then says no review
// was running even though one just completed, which is accurate enough.
//
// Ack reactions that no settle will ever flip — on the notes of a queued job,
// or of a cleared pending re-run — are released here: only a running review
// settles its own notes when its cancelled process winds down.
func (d *Dispatcher) Abort(projectID, iid int) AbortOutcome {
	key := jobKey{ProjectID: projectID, IID: iid}
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.states[key]
	if !ok {
		return AbortOutcome{}
	}
	// Nothing of this job should resume after a restart either.
	d.journal.remove(projectID, iid)
	if state.pending != nil {
		pending := *state.pending
		state.pending = nil
		go d.releaseAcks(pending, pending.AckNoteIDs)
	}
	if state.status == stateRunning {
		if state.cancel != nil {
			state.cancel()
		}
		return AbortOutcome{Found: true, Running: true, Since: time.Since(state.startedAt)}
	}
	latest := state.latest
	delete(d.states, key)
	go d.releaseAcks(latest, latest.AckNoteIDs)
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
