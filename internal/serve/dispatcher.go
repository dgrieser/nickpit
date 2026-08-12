package serve

import (
	"container/list"
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"sync"
	"time"
)

const (
	queueCapacity             = 256
	shaLRUSize                = 512
	ackCleanupQueueCapacity   = 64
	maxAckCleanupWorkers      = 4
	reactionRetryInitial      = time.Second
	reactionRetryMax          = time.Minute
	maxRateLimitRetryDelay    = 5 * time.Minute
	reactionUncertaintyWindow = 30 * time.Second
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
	// Coalesced events contribute their notes too, up to maxAckNotes.
	AckNoteIDs []int
	// UncertainAckNoteIDs are awards whose client request timed out. GitLab may
	// still commit them after the first outcome flip, so settlement keeps these
	// targets alive for another replacement at AckCleanupUntil.
	UncertainAckNoteIDs []int
	AckCleanupUntil     time.Time
	// StartCleanupUntil keeps MR settlement alive after a cancelled or timed-out
	// start-reaction request. GitLab may commit that award after an earlier
	// replacement listed no marker, so one final sweep runs at this deadline.
	StartCleanupUntil time.Time
	// StartEmojis and AckEmojis are every configured name under which this job
	// may have placed its managed reactions. Keeping the names on the event lets
	// journal restore clean markers from an older configuration.
	StartEmojis []string
	AckEmojis   []string
	// SettleMR records that the worker reached the MR-reaction boundary. It is
	// separate from StartEmojis because an empty start emoji still requires
	// revoke-only cleanup of status reactions left by earlier runs.
	SettleMR bool
	// RevokeMROnly preserves disabled-decoration semantics if cleanup survives
	// a restart under a configuration that enables start reactions again.
	RevokeMROnly bool
}

type jobKey struct {
	ProjectID int
	IID       int
}

const (
	stateQueued = iota
	stateRunning
	// stateCleanup has no review running or queued: only durable reaction
	// settlement remains. A later review waits in pending until cleanup wins.
	stateCleanup
)

type reactionCleanup struct {
	event   Event
	outcome reviewOutcome
	// aborted is a newer pending review cancelled while event cleanup was
	// blocked. Its acknowledgements must be revoked, never assigned event's
	// done/failed outcome.
	aborted *Event
}

// jobState coalesces events per MR: while queued the newest event wins;
// while running the newest event is parked in pending and re-queued when the
// active review finishes.
type jobState struct {
	status  int
	latest  Event
	pending *Event
	// cleanup is durable remote-only work. cleanupQueued prevents duplicate
	// cleanup-pool requests while one is queued or running.
	cleanup        *reactionCleanup
	cleanupQueued  bool
	cleanupVersion uint64
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
	Topic        string
	TriggerEmoji string
	// StartEmoji marks the merge request while a review runs; AckEmoji is the
	// same marker on the command note (awarded by the handler). Both are
	// replaced when the review ends: DoneEmoji when it landed, FailEmoji when it
	// did not. Any of them may be "". An empty start emoji makes MR handling
	// revoke-only; an empty ack or outcome emoji suppresses that award.
	StartEmoji string
	AckEmoji   string
	DoneEmoji  string
	FailEmoji  string
	BaseURL    string
	ConfigPath string
	ExtraArgs  []string
	LogDir     string
}

// Dispatcher owns the coalescing queue and worker pools. Review state sits
// behind mu; the independent acknowledgement cleanup pool uses cleanupMu.
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
	// Ack cleanup runs outside the review workers, but its queue and worker
	// count are bounded so a flood of commands over maxAckNotes cannot create
	// unbounded goroutines or GitLab requests. Workers are started lazily and
	// exit when the queue drains.
	cleanupMu      sync.Mutex
	cleanupQueue   []ackCleanup
	cleanupWorkers int
	cleanupClosed  bool
	cleanupWG      sync.WaitGroup
	cleanupCtx     context.Context
	cleanupCancel  context.CancelFunc
	// cleanupWaiting holds durable jobs admitted while cleanupQueue is full.
	// One shared attempt gate admits a bounded wave and carries outage backoff
	// across the whole backlog instead of giving each job a hot loop.
	cleanupWaiting        map[jobKey]struct{}
	cleanupFallback       []ackCleanup
	cleanupAttemptsActive int
	cleanupBackoffGen     uint64
	cleanupRetryFailures  int
	cleanupRetryUntil     time.Time
	cleanupWake           chan struct{}
	cleanupReactionSlots  chan struct{}
	cleanupRetryInitial   time.Duration
	cleanupRetryMax       time.Duration
	cleanupRetryJitter    func(time.Duration, time.Duration) time.Duration
	// startReactionUncertaintyWindow is configurable in tests so delayed-award
	// races do not require production-length waits.
	startReactionUncertaintyWindow time.Duration
	// jobCtx outlives the intake context so in-flight reviews survive
	// shutdown until the grace period expires.
	jobCtx    context.Context
	jobCancel context.CancelFunc
}

func NewDispatcher(runner ReviewRunner, lookup TopicLookup, journal *Journal, cfg WorkerConfig, log *slog.Logger) *Dispatcher {
	jobCtx, jobCancel := context.WithCancel(context.Background())
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	return &Dispatcher{
		states:                         make(map[jobKey]*jobState),
		queue:                          make(chan jobKey, queueCapacity),
		recent:                         newSHALRU(shaLRUSize),
		runner:                         runner,
		topics:                         newTopicCache(lookup),
		cfg:                            cfg,
		journal:                        journal,
		log:                            log,
		jobCtx:                         jobCtx,
		jobCancel:                      jobCancel,
		cleanupCtx:                     cleanupCtx,
		cleanupCancel:                  cleanupCancel,
		cleanupWaiting:                 make(map[jobKey]struct{}),
		cleanupWake:                    make(chan struct{}),
		cleanupReactionSlots:           make(chan struct{}, maxAckCleanupWorkers),
		cleanupRetryInitial:            reactionRetryInitial,
		cleanupRetryMax:                reactionRetryMax,
		cleanupRetryJitter:             jitterReactionRetry,
		startReactionUncertaintyWindow: reactionUncertaintyWindow,
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
		event := eventFromJournal(entry, group)
		key := jobKey{ProjectID: event.ProjectID, IID: event.IID}
		if entry.CleanupOutcome != "" {
			outcome, ok := parseCleanupOutcome(entry.CleanupOutcome)
			if !ok {
				d.log.Warn("journal: dropping job with invalid cleanup outcome", "project", entry.ProjectPath, "iid", entry.IID, "outcome", entry.CleanupOutcome)
				d.journal.remove(entry.ProjectID, entry.IID)
				continue
			}
			cleanup := reactionCleanup{event: event, outcome: outcome}
			if entry.Aborted != nil {
				abortedEvent := eventFromJournal(*entry.Aborted, group)
				cleanup.aborted = &abortedEvent
			}
			var pending *Event
			if entry.Pending != nil {
				pendingEvent := eventFromJournal(*entry.Pending, group)
				pending = &pendingEvent
			}
			d.mu.Lock()
			d.states[key] = &jobState{
				status:         stateCleanup,
				latest:         event,
				pending:        pending,
				cleanup:        &cleanup,
				cleanupVersion: 1,
				persisted:      true,
			}
			d.mu.Unlock()
			d.queueDurableCleanup(key)
			resumed++
			continue
		}
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

func eventFromJournal(entry journalEntry, group *Group) Event {
	event := Event{
		Kind:                parseTriggerKind(entry.Kind),
		ProjectID:           entry.ProjectID,
		ProjectPath:         entry.ProjectPath,
		IID:                 entry.IID,
		HeadSHA:             entry.HeadSHA,
		Group:               group,
		AckNoteIDs:          entry.AckNoteIDs,
		UncertainAckNoteIDs: entry.UncertainAckNoteIDs,
		StartEmojis:         entry.StartEmojis,
		AckEmojis:           entry.AckEmojis,
		SettleMR:            entry.SettleMR,
		RevokeMROnly:        entry.RevokeMROnly,
	}
	if entry.AckCleanupUntilUnixMilli != 0 {
		event.AckCleanupUntil = time.UnixMilli(entry.AckCleanupUntilUnixMilli)
	}
	if entry.StartCleanupUntilUnixMilli != 0 {
		event.StartCleanupUntil = time.UnixMilli(entry.StartCleanupUntilUnixMilli)
	}
	return event
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
		var overflows []ackOverflow
		switch {
		case state.cleanup != nil:
			// Abort installs durable cleanup before a running worker reaches
			// finish. Keep new work behind that cleanup even while status still
			// says running; replacing the journal with a runnable snapshot could
			// resurrect the aborted review after a crash.
			pending := event
			if state.pending != nil {
				var overflow ackOverflow
				pending, overflow = mergeEvents(*state.pending, event)
				overflows = appendAckOverflow(overflows, overflow)
			}
			state.pending = &pending
			state.persisted = d.journal.persistCleanup(*state.cleanup, state.pending)
		case state.status == stateRunning:
			pending := event
			if state.pending != nil {
				var overflow ackOverflow
				pending, overflow = mergeEvents(*state.pending, event)
				overflows = appendAckOverflow(overflows, overflow)
			}
			state.pending = &pending
			// The journal holds the job's would-be re-run after a crash: the
			// running review never settled, so its notes AND the pending ones
			// still need their flip. Notes omitted by this second merge would
			// be absent after a crash, so release them now and remove them from
			// the pending rerun's settlement set.
			resume, omitted := mergeEvents(state.latest, pending)
			overflows = appendAckOverflow(overflows, omitted)
			if len(omitted.notes) > 0 {
				state.pending.AckNoteIDs = removeInts(state.pending.AckNoteIDs, omitted.notes)
				state.pending.UncertainAckNoteIDs = removeInts(state.pending.UncertainAckNoteIDs, omitted.notes)
				if len(state.pending.UncertainAckNoteIDs) == 0 {
					state.pending.AckCleanupUntil = time.Time{}
				}
				resume, _ = mergeEvents(state.latest, *state.pending)
			}
			state.persisted = d.journal.persist(resume)
		default:
			var overflow ackOverflow
			state.latest, overflow = mergeEvents(state.latest, event)
			overflows = appendAckOverflow(overflows, overflow)
			state.persisted = d.journal.persist(state.latest)
		}
		for _, overflow := range overflows {
			// Already acknowledged, but over the per-job cap: no settle will
			// ever flip them, so schedule a bounded release (after the timeout
			// uncertainty window when needed).
			d.log.Warn("ack note cap exceeded, releasing dropped notes", "project", event.ProjectPath, "iid", event.IID, "notes", overflow.notes)
			d.queueAckCleanup(overflow.event, overflow.notes)
		}
		return true
	}
	// Durable cleanup no longer occupies the channel, but it still consumes an
	// admitted MR slot until remote settlement succeeds. Count all non-running
	// states here so a prolonged GitLab outage cannot grow cleanup state,
	// journals, and retry timers without bound.
	if len(d.states)-d.running >= queueCapacity {
		d.dropped++
		d.log.Error("job backlog full, dropping event", "project", event.ProjectPath, "iid", event.IID, "dropped_total", d.dropped)
		return false
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
type ackOverflow struct {
	event Event
	notes []int
}

func appendAckOverflow(overflows []ackOverflow, overflow ackOverflow) []ackOverflow {
	if len(overflow.notes) == 0 {
		return overflows
	}
	return append(overflows, overflow)
}

func mergeEvents(existing, incoming Event) (Event, ackOverflow) {
	if existing.Kind == TriggerManual {
		incoming.Kind = TriggerManual
	}
	merged, dropped := mergeAckNotes(existing.AckNoteIDs, incoming.AckNoteIDs)
	overflow := overflowAckCleanup(existing, incoming, dropped)
	incoming.AckNoteIDs = merged
	uncertain := appendUniqueInts(existing.UncertainAckNoteIDs, incoming.UncertainAckNoteIDs...)
	incoming.UncertainAckNoteIDs = slices.DeleteFunc(uncertain, func(noteID int) bool {
		return !slices.Contains(merged, noteID)
	})
	if existing.AckCleanupUntil.After(incoming.AckCleanupUntil) {
		incoming.AckCleanupUntil = existing.AckCleanupUntil
	}
	if len(incoming.UncertainAckNoteIDs) == 0 {
		incoming.AckCleanupUntil = time.Time{}
	}
	if existing.StartCleanupUntil.After(incoming.StartCleanupUntil) {
		incoming.StartCleanupUntil = existing.StartCleanupUntil
	}
	incoming.StartEmojis = appendUniqueStrings(existing.StartEmojis, incoming.StartEmojis...)
	incoming.AckEmojis = appendUniqueStrings(existing.AckEmojis, incoming.AckEmojis...)
	if eventSettlesMR(existing) {
		incoming.RevokeMROnly = existing.RevokeMROnly
	}
	incoming.SettleMR = existing.SettleMR || incoming.SettleMR
	return incoming, overflow
}

// overflowAckCleanup preserves timeout uncertainty separately from the capped
// job event. Otherwise an immediate successful revoke can race a timed-out POST
// that GitLab commits later, leaving the dropped acknowledgement permanent.
func overflowAckCleanup(existing, incoming Event, dropped []int) ackOverflow {
	if len(dropped) == 0 {
		return ackOverflow{}
	}
	event := incoming
	event.AckNoteIDs = slices.Clone(dropped)
	event.AckEmojis = appendUniqueStrings(existing.AckEmojis, incoming.AckEmojis...)
	event.StartEmojis = nil
	event.SettleMR = false
	event.RevokeMROnly = false
	event.StartCleanupUntil = time.Time{}
	event.UncertainAckNoteIDs = nil
	event.AckCleanupUntil = time.Time{}
	for _, source := range []Event{existing, incoming} {
		for _, noteID := range source.UncertainAckNoteIDs {
			if slices.Contains(dropped, noteID) {
				event.UncertainAckNoteIDs = appendUniqueInts(event.UncertainAckNoteIDs, noteID)
				if source.AckCleanupUntil.After(event.AckCleanupUntil) {
					event.AckCleanupUntil = source.AckCleanupUntil
				}
			}
		}
	}
	return ackOverflow{event: event, notes: slices.Clone(dropped)}
}

// trackAcknowledgement stamps the name already awarded by the handler onto the
// event before it is journaled. Reactions are disabled defensively if a caller
// constructs a group without the resolved identity required for safe cleanup.
// Start reaction names are recorded by recordStartReactionAttempt immediately
// before the remote request; queued and precheck-only jobs have not placed a
// start marker yet.
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
// their ack reaction can be queued for best-effort release. That cleanup queue
// is bounded too, preserving resource limits under sustained overload.
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
					result := d.process(jobCtx, event)
					d.finish(key, result)
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
	d.cleanupOnShutdown(settleTimeout)
	d.journal.stopRetirements()
}

// cleanupOnShutdown gives background acknowledgement cleanup half the normal
// reaction budget, then reserves a separate fallback budget. If an unpersisted
// timed-out award is still uncertain, fallback extends through that deadline
// so process exit cannot discard the mandatory final sweep.
func (d *Dispatcher) cleanupOnShutdown(timeout time.Duration) {
	backgroundBudget := timeout / 2
	backgroundCtx, backgroundCancel := context.WithTimeout(context.Background(), backgroundBudget)
	d.stopAckCleanup(backgroundCtx)
	backgroundCancel()

	fallbackBudget := timeout - backgroundBudget
	if uncertaintyDelay := d.unpersistedReactionUncertaintyDelay(); uncertaintyDelay > 0 {
		fallbackBudget = max(fallbackBudget, uncertaintyDelay+timeout/2)
	}
	fallbackCtx, fallbackCancel := context.WithTimeout(context.Background(), fallbackBudget)
	defer fallbackCancel()
	d.releaseUnfinished(fallbackCtx)
}

func (d *Dispatcher) unpersistedReactionUncertaintyDelay() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	var deadline time.Time
	consider := func(event Event) {
		deadline = maxTime(deadline, eventUncertaintyDeadline(event))
	}
	for _, state := range d.states {
		if state.persisted {
			continue
		}
		consider(state.latest)
		if state.cleanup != nil {
			consider(state.cleanup.event)
			if state.cleanup.aborted != nil {
				consider(*state.cleanup.aborted)
			}
		}
		if state.pending != nil {
			consider(*state.pending)
		}
	}
	d.cleanupMu.Lock()
	for _, cleanup := range d.cleanupFallback {
		consider(cleanup.event)
		if cleanup.aborted != nil {
			consider(*cleanup.aborted)
		}
	}
	d.cleanupMu.Unlock()
	return max(time.Until(deadline), 0)
}

// releaseUnfinished handles the jobs shutdown leaves behind: queued jobs that
// never ran and pending re-runs parked behind a review (the workers are done,
// so nothing here will ever settle in this process). With a journal they stay
// on disk and resume after restart; without one, their ack reactions are
// revoked so no command note reads as in progress forever.
func (d *Dispatcher) releaseUnfinished(ctx context.Context) {
	type unfinished struct {
		event     Event
		cleanup   *reactionCleanup
		pending   *Event
		persisted bool
	}
	d.mu.Lock()
	remaining := make([]unfinished, 0, len(d.states))
	for _, state := range d.states {
		if state.cleanup != nil {
			persisted := state.persisted
			if d.journal != nil && !persisted {
				persisted = d.journal.persistCleanup(*state.cleanup, state.pending)
			}
			remaining = append(remaining, unfinished{
				event:     state.cleanup.event,
				cleanup:   state.cleanup,
				pending:   state.pending,
				persisted: persisted,
			})
			continue
		}
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
	d.cleanupMu.Lock()
	for _, cleanup := range d.cleanupFallback {
		fallback := reactionCleanup{
			event:   eventWithReactions(cleanup.event, cleanup.placed),
			outcome: cleanup.outcome,
			aborted: cleanup.aborted,
		}
		remaining = append(remaining, unfinished{event: fallback.event, cleanup: &fallback})
	}
	d.cleanupFallback = nil
	d.cleanupMu.Unlock()
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
	// This fallback runs after the regular cleanup pool has stopped. Keep both
	// its active jobs and their nested reaction requests bounded: settle fans a
	// job's reactions out concurrently, so limiting only the outer jobs would
	// still permit maxAckCleanupWorkers*maxAckNotes live GitLab requests.
	jobs := make(chan unfinished)
	reactionSlots := make(chan struct{}, maxAckCleanupWorkers)
	var wg sync.WaitGroup
	for range min(maxAckCleanupWorkers, len(remaining)) {
		wg.Go(func() {
			for job := range jobs {
				if job.persisted {
					continue
				}
				if job.cleanup != nil {
					cleanup := *job.cleanup
					log := d.log.With("project", cleanup.event.ProjectPath, "iid", cleanup.event.IID)
					remaining := d.settleReactionCleanupThroughUncertainty(ctx, cleanup, log, reactionSlots)
					if cleanupHasReactions(remaining) {
						log.Warn("shutdown: reaction cleanup remains unresolved after fallback")
					}
					if job.pending != nil {
						d.releaseReactionsWithLimit(ctx, *job.pending, reactionSlots)
					}
					continue
				}
				d.releaseReactionsWithLimit(ctx, job.event, reactionSlots)
			}
		})
	}
	for _, job := range remaining {
		jobs <- job
	}
	close(jobs)
	wg.Wait()
}

// releaseReactionsWithLimit revokes every in-progress reaction from a review
// discarded at shutdown without a journal.
func (d *Dispatcher) releaseReactionsWithLimit(ctx context.Context, event Event, reactionSlots chan struct{}) {
	placed := reactions{mr: eventSettlesMR(event), notes: event.AckNoteIDs}
	if !hasReactions(placed) {
		return
	}
	log := d.log.With("project", event.ProjectPath, "iid", event.IID)
	remaining := eventWithReactions(event, d.settleWithLimit(ctx, event, placed, outcomeAborted, log, reactionSlots))
	if waitForReactionUncertainty(ctx, remaining) {
		remaining = eventWithReactions(remaining, d.settleWithLimit(ctx, remaining,
			reactions{mr: eventSettlesMR(remaining), notes: remaining.AckNoteIDs}, outcomeAborted, log, reactionSlots))
	}
	if eventSettlesMR(remaining) || len(remaining.AckNoteIDs) > 0 {
		log.Warn("shutdown: reaction cleanup remains unresolved after fallback", "notes", remaining.AckNoteIDs)
	}
}

type ackCleanup struct {
	event   Event
	placed  reactions
	outcome reviewOutcome
	aborted *Event
	durable *jobKey
	version uint64
	readyAt time.Time
}

// queueAckCleanup schedules best-effort acknowledgement revokes without
// blocking webhook intake. Both queued work and live requests are bounded; if
// the cleanup backlog is saturated, excess work is logged and discarded.
func (d *Dispatcher) queueAckCleanup(event Event, notes []int) {
	if len(notes) == 0 {
		return
	}
	d.cleanupMu.Lock()
	queued := 0
	if !d.cleanupClosed {
		for _, noteID := range notes {
			if len(d.cleanupQueue) >= ackCleanupQueueCapacity {
				break
			}
			cleanup := ackCleanup{
				event:   event,
				placed:  reactions{notes: []int{noteID}},
				outcome: outcomeAborted,
			}
			if slices.Contains(event.UncertainAckNoteIDs, noteID) {
				cleanup.readyAt = event.AckCleanupUntil
			}
			d.cleanupQueue = append(d.cleanupQueue, cleanup)
			queued++
		}
		d.startAckCleanupWorkersLocked()
	}
	d.cleanupMu.Unlock()
	if queued < len(notes) {
		d.log.Warn("ack cleanup backlog full, dropping cleanup requests",
			"project", event.ProjectPath, "iid", event.IID, "dropped", len(notes)-queued)
	}
}

// queueDurableCleanup schedules one complete settlement attempt. Unlike
// overflow ack cleanup, this work is never discarded: saturation reserves it
// in cleanupWaiting, while remote failures re-enter the queue behind the
// dispatcher's shared outage gate.
func (d *Dispatcher) queueDurableCleanup(key jobKey) {
	d.mu.Lock()
	state, ok := d.states[key]
	if !ok || state.status != stateCleanup || state.cleanup == nil || state.cleanupQueued || d.closed {
		d.mu.Unlock()
		return
	}
	cleanup := *state.cleanup
	if delay := time.Until(reactionCleanupUncertaintyDeadline(cleanup)); delay > 0 {
		state.cleanupQueued = true
		version := state.cleanupVersion
		d.mu.Unlock()
		time.AfterFunc(delay, func() { d.startDelayedCleanup(key, version) })
		return
	}
	d.cleanupMu.Lock()
	if !d.cleanupClosed && len(d.cleanupQueue) < ackCleanupQueueCapacity {
		d.cleanupQueue = append(d.cleanupQueue, durableAckCleanup(key, cleanup, state.cleanupVersion))
		state.cleanupQueued = true
		d.startAckCleanupWorkersLocked()
	} else if !d.cleanupClosed {
		// The reservation counts as queued so repeated state transitions cannot
		// add duplicate waiting entries. A worker promotes it when a slot opens.
		d.cleanupWaiting[key] = struct{}{}
		state.cleanupQueued = true
	}
	d.cleanupMu.Unlock()
	d.mu.Unlock()
}

func durableAckCleanup(key jobKey, cleanup reactionCleanup, version uint64) ackCleanup {
	keyCopy := key
	return ackCleanup{
		event: cleanup.event,
		placed: reactions{
			mr:    eventSettlesMR(cleanup.event),
			notes: cleanup.event.AckNoteIDs,
		},
		outcome: cleanup.outcome,
		aborted: cleanup.aborted,
		durable: &keyCopy,
		version: version,
	}
}

// drainWaitingDurableCleanup promotes reservations after a worker frees queue
// capacity. Lock order matches queueDurableCleanup: state, then cleanup.
func (d *Dispatcher) drainWaitingDurableCleanup() {
	d.mu.Lock()
	d.cleanupMu.Lock()
	for key := range d.cleanupWaiting {
		if d.closed || d.cleanupClosed || len(d.cleanupQueue) >= ackCleanupQueueCapacity {
			break
		}
		state, ok := d.states[key]
		if !ok || state.status != stateCleanup || state.cleanup == nil || !state.cleanupQueued {
			delete(d.cleanupWaiting, key)
			continue
		}
		d.cleanupQueue = append(d.cleanupQueue, durableAckCleanup(key, *state.cleanup, state.cleanupVersion))
		delete(d.cleanupWaiting, key)
	}
	d.startAckCleanupWorkersLocked()
	d.cleanupMu.Unlock()
	d.mu.Unlock()
}

// startDelayedCleanup releases the reservation made for an uncertain award
// sweep. A version change means abort/coalescing updated the work while the
// timer waited; queueDurableCleanup reads and schedules that newest snapshot.
func (d *Dispatcher) startDelayedCleanup(key jobKey, version uint64) {
	d.mu.Lock()
	state, ok := d.states[key]
	if !ok || state.status != stateCleanup || state.cleanup == nil || !state.cleanupQueued {
		d.mu.Unlock()
		return
	}
	state.cleanupQueued = false
	closed := d.closed
	changed := state.cleanupVersion != version
	d.mu.Unlock()
	if closed {
		return
	}
	if changed {
		d.log.Debug("acknowledgement cleanup changed while delayed", "project_id", key.ProjectID, "iid", key.IID)
	}
	d.queueDurableCleanup(key)
}

// startAckCleanupWorkersLocked starts enough bounded workers for queued work.
// Caller holds cleanupMu.
func (d *Dispatcher) startAckCleanupWorkersLocked() {
	for d.cleanupWorkers < maxAckCleanupWorkers && d.cleanupWorkers < len(d.cleanupQueue) {
		d.cleanupWorkers++
		d.cleanupWG.Go(d.runAckCleanup)
	}
}

// beginAckCleanupAttempt admits one cleanup job after both its semantic
// deadline and the shared outage backoff. Bounded waves at this boundary keep
// a large durable backlog from multiplying a fast GitLab failure into
// independent retry loops; reactions across admitted jobs remain bounded by
// cleanupReactionSlots.
func (d *Dispatcher) beginAckCleanupAttempt(ctx context.Context, readyAt time.Time) (uint64, bool) {
	for {
		d.cleanupMu.Lock()
		now := time.Now()
		notBefore := readyAt
		if d.cleanupRetryUntil.After(notBefore) {
			notBefore = d.cleanupRetryUntil
		}
		if d.cleanupAttemptsActive < maxAckCleanupWorkers && !now.Before(notBefore) {
			d.cleanupAttemptsActive++
			generation := d.cleanupBackoffGen
			d.cleanupMu.Unlock()
			return generation, true
		}
		wake := d.cleanupWake
		atCapacity := d.cleanupAttemptsActive >= maxAckCleanupWorkers
		d.cleanupMu.Unlock()

		var timer *time.Timer
		var timerC <-chan time.Time
		if !atCapacity {
			if delay := time.Until(notBefore); delay > 0 {
				timer = time.NewTimer(delay)
				timerC = timer.C
			}
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return 0, false
		case <-wake:
			if timer != nil {
				timer.Stop()
			}
		case <-timerC:
		}
	}
}

func (d *Dispatcher) finishAckCleanupAttempt(generation uint64, failed bool, retryAfter time.Time) {
	now := time.Now()
	d.cleanupMu.Lock()
	if d.cleanupAttemptsActive > 0 {
		d.cleanupAttemptsActive--
	}
	if failed {
		d.cleanupBackoffGen++
		d.cleanupRetryFailures++
		delay := exponentialReactionRetry(d.cleanupRetryFailures, d.cleanupRetryInitial, d.cleanupRetryMax)
		if d.cleanupRetryJitter != nil {
			delay = d.cleanupRetryJitter(delay, d.cleanupRetryMax)
		}
		retryUntil := now.Add(delay)
		serverLimit := now.Add(maxRateLimitRetryDelay)
		if retryAfter.After(serverLimit) {
			retryAfter = serverLimit
		}
		if retryAfter.After(retryUntil) {
			retryUntil = retryAfter
		}
		// Concurrent attempts finish out of order. Keep a longer server deadline
		// established by an earlier completion instead of replacing it with this
		// attempt's local backoff.
		if retryUntil.After(d.cleanupRetryUntil) {
			d.cleanupRetryUntil = retryUntil
		}
	} else if generation == d.cleanupBackoffGen {
		d.cleanupRetryFailures = 0
		d.cleanupRetryUntil = time.Time{}
	}
	retryUntil := d.cleanupRetryUntil
	failures := d.cleanupRetryFailures
	close(d.cleanupWake)
	d.cleanupWake = make(chan struct{})
	d.cleanupMu.Unlock()
	if failed {
		d.log.Warn("acknowledgement cleanup backing off", "delay", time.Until(retryUntil).Round(time.Millisecond), "failures", failures)
	}
}

func exponentialReactionRetry(failures int, initial, maximum time.Duration) time.Duration {
	if initial <= 0 || maximum <= 0 {
		return 0
	}
	delay := initial
	for attempt := 1; attempt < failures && delay < maximum; attempt++ {
		if delay > maximum/2 {
			return maximum
		}
		delay *= 2
	}
	return min(delay, maximum)
}

func jitterReactionRetry(delay, maximum time.Duration) time.Duration {
	if delay <= 0 || maximum <= delay {
		return delay
	}
	spread := min(delay/2, maximum-delay)
	if spread <= 0 {
		return delay
	}
	return delay + time.Duration(rand.Int64N(int64(spread)+1))
}

func (d *Dispatcher) runAckCleanup() {
	for {
		d.cleanupMu.Lock()
		if len(d.cleanupQueue) == 0 {
			d.cleanupWorkers--
			d.cleanupMu.Unlock()
			return
		}
		cleanup := d.cleanupQueue[0]
		d.cleanupQueue[0] = ackCleanup{}
		d.cleanupQueue = d.cleanupQueue[1:]
		d.cleanupMu.Unlock()
		d.drainWaitingDurableCleanup()

		generation, admitted := d.beginAckCleanupAttempt(d.cleanupCtx, cleanup.readyAt)
		if !admitted {
			if cleanup.durable != nil {
				remaining := reactionCleanup{event: eventWithReactions(cleanup.event, cleanup.placed), outcome: cleanup.outcome, aborted: cleanup.aborted}
				d.completeDurableCleanup(*cleanup.durable, cleanup.version, remaining)
			} else {
				d.preserveAckCleanupForFallback(cleanup)
			}
			continue
		}

		log := d.log.With("project", cleanup.event.ProjectPath, "iid", cleanup.event.IID)
		attempt := d.settleAttemptWithLimit(d.cleanupCtx, cleanup.event, cleanup.placed, cleanup.outcome, log, d.cleanupReactionSlots)
		remaining := reactionCleanup{
			event:   eventWithReactions(cleanup.event, attempt.remaining),
			outcome: cleanup.outcome,
		}
		if cleanup.aborted != nil {
			abortedAttempt := d.settleAttemptWithLimit(d.cleanupCtx, *cleanup.aborted,
				reactions{notes: cleanup.aborted.AckNoteIDs}, outcomeAborted, log, d.cleanupReactionSlots)
			aborted := eventWithReactions(*cleanup.aborted, abortedAttempt.remaining)
			remaining.aborted = &aborted
			attempt.retryableFailure = attempt.retryableFailure || abortedAttempt.retryableFailure
			if abortedAttempt.retryAfter.After(attempt.retryAfter) {
				attempt.retryAfter = abortedAttempt.retryAfter
			}
		}
		d.finishAckCleanupAttempt(generation, attempt.retryableFailure, attempt.retryAfter)
		if cleanup.durable != nil {
			d.completeDurableCleanup(*cleanup.durable, cleanup.version, remaining)
		} else if d.cleanupCtx.Err() != nil && cleanupHasReactions(remaining) {
			d.preserveAckCleanupForFallback(ackCleanup{
				event: eventWithReactions(remaining.event, reactions{
					mr:    eventSettlesMR(remaining.event),
					notes: remaining.event.AckNoteIDs,
				}),
				placed: reactions{
					mr:    eventSettlesMR(remaining.event),
					notes: remaining.event.AckNoteIDs,
				},
				outcome: remaining.outcome,
				aborted: remaining.aborted,
			})
		}
	}
}

func (d *Dispatcher) preserveAckCleanupForFallback(cleanup ackCleanup) {
	if cleanup.durable != nil {
		return
	}
	cleanup.event = eventWithReactions(cleanup.event, cleanup.placed)
	d.cleanupMu.Lock()
	d.cleanupFallback = append(d.cleanupFallback, cleanup)
	d.cleanupMu.Unlock()
}

func (d *Dispatcher) completeDurableCleanup(key jobKey, version uint64, remaining reactionCleanup) {
	retry := false
	d.mu.Lock()
	state, ok := d.states[key]
	if !ok || state.status != stateCleanup || state.cleanup == nil {
		d.mu.Unlock()
		return
	}
	state.cleanupQueued = false
	if version != state.cleanupVersion {
		retry = !d.closed
		d.mu.Unlock()
		if retry {
			d.queueDurableCleanup(key)
		}
		return
	}
	if cleanupHasReactions(remaining) {
		state.cleanup = &remaining
		state.latest = remaining.event
		// Always rewrite after an attempt: successful or terminal targets were
		// removed, so restart must retry only unresolved targets.
		state.persisted = d.journal.persistCleanup(remaining, state.pending)
		retry = !d.closed
		d.mu.Unlock()
		if retry {
			d.queueDurableCleanup(key)
		}
		return
	}
	state.cleanup = nil
	if state.pending == nil {
		delete(d.states, key)
		d.journal.remove(key.ProjectID, key.IID)
		d.mu.Unlock()
		return
	}
	state.latest = *state.pending
	state.pending = nil
	state.status = stateQueued
	state.persisted = d.journal.persist(state.latest)
	if !d.closed {
		select {
		case d.queue <- key:
		default:
			d.overflow = append(d.overflow, key)
		}
	}
	d.mu.Unlock()
}

// stopAckCleanup prevents new cleanup work and drains until ctx expires. It then
// cancels in-flight requests so a full queue cannot exceed process shutdown
// budget. Server shutdown calls this after HTTP handlers have drained, so no
// producer can race the wait with a new WaitGroup task.
func (d *Dispatcher) stopAckCleanup(ctx context.Context) {
	d.cleanupMu.Lock()
	d.cleanupClosed = true
	d.cleanupMu.Unlock()
	done := make(chan struct{})
	go func() {
		d.cleanupWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		d.cleanupCancel()
		return
	case <-ctx.Done():
		d.log.Warn("shutdown: acknowledgement cleanup grace expired, cancelling requests")
		d.cleanupCancel()
		// Workers already hold at most maxAckCleanupWorkers items. Discard queued
		// best-effort attempts; durable state remains in d.states/the journal and
		// releaseUnfinished handles any required fallback below.
		d.cleanupMu.Lock()
		for _, cleanup := range d.cleanupQueue {
			if cleanup.durable == nil {
				cleanup.event = eventWithReactions(cleanup.event, cleanup.placed)
				d.cleanupFallback = append(d.cleanupFallback, cleanup)
			}
		}
		d.cleanupQueue = nil
		d.cleanupMu.Unlock()
	}
	<-done
}

func cleanupHasReactions(cleanup reactionCleanup) bool {
	if eventSettlesMR(cleanup.event) || len(cleanup.event.AckNoteIDs) > 0 {
		return true
	}
	return cleanup.aborted != nil && (eventSettlesMR(*cleanup.aborted) || len(cleanup.aborted.AckNoteIDs) > 0)
}

func (d *Dispatcher) settleReactionCleanup(ctx context.Context, cleanup reactionCleanup, log *slog.Logger) reactionCleanup {
	return d.settleReactionCleanupWithLimit(ctx, cleanup, log, nil)
}

func (d *Dispatcher) settleReactionCleanupWithLimit(
	ctx context.Context,
	cleanup reactionCleanup,
	log *slog.Logger,
	reactionSlots chan struct{},
) reactionCleanup {
	remaining := reactionCleanup{
		event: eventWithReactions(cleanup.event, d.settleWithLimit(ctx, cleanup.event, reactions{
			mr:    eventSettlesMR(cleanup.event),
			notes: cleanup.event.AckNoteIDs,
		}, cleanup.outcome, log, reactionSlots)),
		outcome: cleanup.outcome,
	}
	if cleanup.aborted != nil {
		aborted := eventWithReactions(*cleanup.aborted, d.settleWithLimit(ctx, *cleanup.aborted,
			reactions{notes: cleanup.aborted.AckNoteIDs}, outcomeAborted, log, reactionSlots))
		remaining.aborted = &aborted
	}
	return remaining
}

func (d *Dispatcher) settleReactionCleanupThroughUncertainty(
	ctx context.Context,
	cleanup reactionCleanup,
	log *slog.Logger,
	reactionSlots chan struct{},
) reactionCleanup {
	remaining := d.settleReactionCleanupWithLimit(ctx, cleanup, log, reactionSlots)
	if waitUntilReactionSweep(ctx, reactionCleanupUncertaintyDeadline(remaining)) {
		remaining = d.settleReactionCleanupWithLimit(ctx, remaining, log, reactionSlots)
	}
	return remaining
}

func waitForReactionUncertainty(ctx context.Context, event Event) bool {
	return waitUntilReactionSweep(ctx, eventUncertaintyDeadline(event))
}

func eventUncertaintyDeadline(event Event) time.Time {
	var deadline time.Time
	if len(event.UncertainAckNoteIDs) > 0 {
		deadline = event.AckCleanupUntil
	}
	if eventSettlesMR(event) {
		deadline = maxTime(deadline, event.StartCleanupUntil)
	}
	return deadline
}

func reactionCleanupUncertaintyDeadline(cleanup reactionCleanup) time.Time {
	deadline := eventUncertaintyDeadline(cleanup.event)
	if cleanup.aborted != nil {
		deadline = maxTime(deadline, eventUncertaintyDeadline(*cleanup.aborted))
	}
	return deadline
}

func maxTime(first, second time.Time) time.Time {
	if second.After(first) {
		return second
	}
	return first
}

// waitUntilReactionSweep reports whether a final sweep is required and its
// deadline was reached. A zero deadline means no uncertain award remains.
func waitUntilReactionSweep(ctx context.Context, deadline time.Time) bool {
	if deadline.IsZero() {
		return false
	}
	if delay := time.Until(deadline); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return false
		}
	}
	return ctx.Err() == nil
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
	for _, state := range d.states {
		if state.status == stateCleanup {
			if state.pending != nil {
				queued++
			}
			continue
		}
		if state.status == stateQueued {
			queued++
		}
	}
	return queued, d.running
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
	ctx, cancel := context.WithCancel(d.jobCtx)
	state.cancel = cancel
	state.startedAt = time.Now()
	d.running++
	return state.latest, ctx, true
}

// recordStartReactionAttempt persists MR settlement intent and any configured
// start name immediately before the worker makes the remote request. Therefore
// restored state means a request may truly have reached GitLab; a crash during
// earlier policy checks cannot make the resumed worker touch an unstarted MR.
func (d *Dispatcher) recordStartReactionAttempt(event *Event) {
	event.SettleMR = true
	event.RevokeMROnly = d.cfg.StartEmoji == ""
	event.StartEmojis = appendUniqueStrings(event.StartEmojis, d.cfg.StartEmoji)
	key := jobKey{ProjectID: event.ProjectID, IID: event.IID}
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.states[key]
	if !ok {
		return
	}
	if state.cleanup != nil {
		state.cleanup.event.SettleMR = true
		state.cleanup.event.RevokeMROnly = d.cfg.StartEmoji == ""
		state.cleanup.event.StartEmojis = appendUniqueStrings(state.cleanup.event.StartEmojis, d.cfg.StartEmoji)
		state.latest = state.cleanup.event
		state.cleanupVersion++
		state.persisted = d.journal.persistCleanup(*state.cleanup, state.pending)
		return
	}
	state.latest.SettleMR = true
	state.latest.RevokeMROnly = d.cfg.StartEmoji == ""
	state.latest.StartEmojis = appendUniqueStrings(state.latest.StartEmojis, d.cfg.StartEmoji)
	resume := state.latest
	if state.pending != nil {
		resume, _ = mergeEvents(resume, *state.pending)
	}
	state.persisted = d.journal.persist(resume)
}

// recordStartReactionUncertainty persists the deadline for a final sweep after
// a start-reaction request returned an ambiguous cancellation/timeout.
func (d *Dispatcher) recordStartReactionUncertainty(event *Event, until time.Time) {
	if until.After(event.StartCleanupUntil) {
		event.StartCleanupUntil = until
	}
	key := jobKey{ProjectID: event.ProjectID, IID: event.IID}
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.states[key]
	if !ok {
		return
	}
	if state.cleanup != nil {
		if until.After(state.cleanup.event.StartCleanupUntil) {
			state.cleanup.event.StartCleanupUntil = until
		}
		state.latest = state.cleanup.event
		state.cleanupVersion++
		state.persisted = d.journal.persistCleanup(*state.cleanup, state.pending)
		return
	}
	if until.After(state.latest.StartCleanupUntil) {
		state.latest.StartCleanupUntil = until
	}
	resume := state.latest
	if state.pending != nil {
		resume, _ = mergeEvents(resume, *state.pending)
	}
	state.persisted = d.journal.persist(resume)
}

// beginSettlement replaces the runnable crash snapshot with cleanup-only work
// after the review process has finished, before any remote reaction settlement
// begins. A crash in that I/O window can therefore repeat harmless reaction
// replacement, but can never rerun and republish the completed review.
//
// The version lets finish distinguish this snapshot from cleanup installed by
// an Abort racing the settlement. Direct process callers without a running
// dispatcher state have no journal lifecycle to transition and return zero.
func (d *Dispatcher) beginSettlement(event Event, placed reactions, outcome reviewOutcome) uint64 {
	key := jobKey{ProjectID: event.ProjectID, IID: event.IID}
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.states[key]
	if !ok || state.status != stateRunning || state.cleanup != nil {
		return 0
	}
	cleanup := reactionCleanup{
		event:   eventWithReactions(event, placed),
		outcome: outcome,
	}
	state.latest = cleanup.event
	state.cleanup = &cleanup
	state.cleanupVersion++
	state.persisted = d.journal.persistCleanup(cleanup, state.pending)
	return state.cleanupVersion
}

// finish completes a job. A pending event received mid-run re-queues the MR
// at the back of the queue — behind other waiting MRs, so a busy MR cannot
// monopolize a worker. The re-run was already acknowledged with a 2xx at
// enqueue time, so a full queue must not drop it: it parks in the overflow
// and take moves it into the next freed slot. During shutdown it cannot run
// in this process; it stays journaled for the restart, or — without a journal
// — releaseUnfinished revokes its ack reactions on the way out.
func (d *Dispatcher) finish(key jobKey, result settlementResult) {
	d.mu.Lock()
	d.running--
	state, ok := d.states[key]
	if !ok {
		d.mu.Unlock()
		return
	}
	if state.cancel != nil {
		state.cancel()
		state.cancel = nil
	}
	state.startedAt = time.Time{}
	if result.resume {
		// Root shutdown interrupted accepted work. Keep runnable state instead of
		// converting it to failed cleanup; restart must execute the review again.
		if state.cleanup != nil {
			state.status = stateCleanup
			state.persisted = d.journal.persistCleanup(*state.cleanup, state.pending)
		} else {
			state.latest = result.event
			state.status = stateQueued
			resume := state.latest
			if state.pending != nil {
				resume, _ = mergeEvents(resume, *state.pending)
			}
			state.persisted = d.journal.persist(resume)
		}
		d.mu.Unlock()
		return
	}
	// Abort may have replaced the worker's completion snapshot while reaction
	// settlement was in flight. Preserve that newer cleanup as one unit,
	// including notes from a cleared pending run.
	ownsCleanup := state.cleanup != nil && result.cleanupVersion != 0 &&
		result.cleanupVersion == state.cleanupVersion
	if state.cleanup != nil && !ownsCleanup {
		state.status = stateCleanup
		if d.journal != nil && !state.persisted {
			state.persisted = d.journal.persistCleanup(*state.cleanup, state.pending)
		}
		queueCleanup := !d.closed
		d.mu.Unlock()
		if queueCleanup {
			d.queueDurableCleanup(key)
		}
		return
	}
	if ownsCleanup {
		state.cleanup = nil
	}
	if !result.settled {
		cleanup := reactionCleanup{event: result.event, outcome: result.outcome}
		state.latest = result.event
		state.cleanup = &cleanup
		state.cleanupVersion++
		state.status = stateCleanup
		state.persisted = d.journal.persistCleanup(cleanup, state.pending)
		queueCleanup := !d.closed
		d.mu.Unlock()
		if queueCleanup {
			d.queueDurableCleanup(key)
		}
		return
	}
	if state.pending == nil {
		delete(d.states, key)
		d.journal.remove(key.ProjectID, key.IID)
		d.mu.Unlock()
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
		d.mu.Unlock()
		return
	}
	select {
	case d.queue <- key:
	default:
		d.overflow = append(d.overflow, key)
	}
	d.mu.Unlock()
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
// Before cancellation becomes visible, Abort replaces the runnable journal
// entry with cleanup-only state. A crash can therefore never resurrect the
// aborted review or lose the reactions that still need revocation.
func (d *Dispatcher) Abort(projectID, iid int) AbortOutcome {
	key := jobKey{ProjectID: projectID, IID: iid}
	d.mu.Lock()
	state, ok := d.states[key]
	if !ok {
		d.mu.Unlock()
		return AbortOutcome{}
	}
	if state.status == stateCleanup {
		// Cleanup itself is not an active review. An abort only has work to do
		// when a newer review is waiting behind it.
		if state.pending == nil {
			d.mu.Unlock()
			return AbortOutcome{}
		}
		aborted := *state.pending
		var overflow ackOverflow
		if state.cleanup.aborted != nil {
			aborted, overflow = mergeEvents(*state.cleanup.aborted, aborted)
		}
		// Pending reviews have not placed an MR start reaction. Even malformed
		// restored input must not let their abort affect old cleanup's MR target.
		aborted.StartEmojis = nil
		aborted.SettleMR = false
		aborted.RevokeMROnly = false
		state.cleanup.aborted = &aborted
		state.pending = nil
		state.cleanupVersion++
		state.persisted = d.journal.persistCleanup(*state.cleanup, nil)
		d.mu.Unlock()
		if len(overflow.notes) > 0 {
			d.log.Warn("ack note cap exceeded while aborting pending review, releasing dropped notes",
				"project", aborted.ProjectPath, "iid", aborted.IID, "notes", overflow.notes)
			d.queueAckCleanup(overflow.event, overflow.notes)
		}
		return AbortOutcome{Found: true}
	}
	cleanupEvent := state.latest
	if state.pending != nil {
		cleanupEvent, _ = mergeEvents(cleanupEvent, *state.pending)
	}
	cleanup := reactionCleanup{event: cleanupEvent, outcome: outcomeAborted}
	state.latest = cleanupEvent
	state.pending = nil
	state.cleanup = &cleanup
	state.cleanupVersion++
	// Persist before cancellation or async cleanup: webhook acceptance can now
	// safely outlive this process.
	state.persisted = d.journal.persistCleanup(cleanup, nil)
	if state.status == stateRunning {
		outcome := AbortOutcome{Found: true, Running: true, Since: time.Since(state.startedAt)}
		if state.cancel != nil {
			state.cancel()
		}
		d.mu.Unlock()
		return outcome
	}
	state.status = stateCleanup
	d.mu.Unlock()
	d.queueDurableCleanup(key)
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
	if state.status == stateCleanup {
		return JobInfo{Queued: state.pending != nil}
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
