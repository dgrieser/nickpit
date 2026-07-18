package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

// maxWebhookBody bounds webhook payloads; GitLab MR events stay far below.
const maxWebhookBody = 10 << 20 // 10 MiB

// commandReplyTimeout bounds the fire-and-forget GitLab calls acknowledging
// and answering command comments.
const commandReplyTimeout = 30 * time.Second

// chatReplyTimeout bounds a discussion-agent reply, which spawns a child running
// an LLM turn (and possibly tool calls), so it is far longer than a plain
// command reply.
const chatReplyTimeout = 10 * time.Minute

// defaultMaxConcurrentChats bounds how many chat children run at once when the
// serve config does not set a limit, so a burst of discussion activity cannot
// spawn unbounded processes, API requests, and log files.
const defaultMaxConcurrentChats = 4

// chatSeenCap bounds the recently-answered note set used to drop webhook
// redeliveries of the same note.
const chatSeenCap = 1024

// HandlerConfig is the static trigger configuration for the webhook endpoint.
type HandlerConfig struct {
	// TriggerEmoji is the award-emoji name requesting a manual review.
	TriggerEmoji string
	// CommandKeyword is the "/<keyword> <command>" note-command keyword.
	CommandKeyword string
	// AckEmoji is awarded on a review command note to acknowledge it; ""
	// disables.
	AckEmoji string
	// AbortEmoji is awarded on an abort command note to acknowledge it; ""
	// disables.
	AbortEmoji string
}

// ChatConfig carries the static invocation details a spawned chat child needs
// (the daemon's config path, GitLab base URL, log dir, and any extra args). The
// per-group token is taken from the matched group at spawn time.
type ChatConfig struct {
	ConfigPath string
	BaseURL    string
	LogDir     string
	ExtraArgs  []string
	// MaxConcurrent caps concurrent chat children; <=0 uses
	// defaultMaxConcurrentChats.
	MaxConcurrent int
}

// Handler is the webhook HTTP endpoint. It only parses, authenticates, and
// classifies — every API call and the review itself happen async, keeping the
// response inside GitLab's ~10s webhook timeout. Command state changes
// (enqueue, abort, status) are synchronous mutex-only dispatcher calls; only
// the GitLab acknowledgements and replies run in goroutines, which are fire
// and forget: a reply may be lost when the daemon shuts down mid-flight.
type Handler struct {
	groups     *GroupSet
	dispatcher *Dispatcher
	cfg        HandlerConfig
	log        *slog.Logger
	// chatRunner spawns a `nickpit chat` child to answer a discussion-thread
	// reply, keeping the daemon free of LLM logic. Nil disables chat, and chat
	// candidate events are then ignored.
	chatRunner ChatRunner
	chatCfg    ChatConfig
	// chatSem bounds concurrent chat work (thread gate + child) so a burst of
	// discussion activity cannot spawn unbounded processes.
	chatSem chan struct{}
	// chatLocks serializes chat replies within a discussion, so two quick replies
	// are answered in order instead of racing and both answering the newest note.
	chatLocks keyedMutex
	// chatSeen drops webhook redeliveries of a note already answered.
	chatSeen *noteDedup
}

func NewHandler(groups *GroupSet, dispatcher *Dispatcher, cfg HandlerConfig, chatRunner ChatRunner, chatCfg ChatConfig, log *slog.Logger) *Handler {
	limit := chatCfg.MaxConcurrent
	if limit <= 0 {
		limit = defaultMaxConcurrentChats
	}
	return &Handler{
		groups:     groups,
		dispatcher: dispatcher,
		cfg:        cfg,
		chatRunner: chatRunner,
		chatCfg:    chatCfg,
		chatSem:    make(chan struct{}, limit),
		chatSeen:   newNoteDedup(chatSeenCap),
		log:        log,
	}
}

// keyedMutex provides a mutex per string key, so callers serialize work for the
// same key while different keys proceed concurrently. Entries are
// reference-counted and removed once idle, so a long-running daemon's memory
// does not grow with the lifetime count of distinct discussions.
type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*keyedLock
}

type keyedLock struct {
	mu sync.Mutex
	// refs counts holders and waiters; the map entry is deleted when it drops
	// to zero, guarded by keyedMutex.mu.
	refs int
}

func (k *keyedMutex) lock(key string) func() {
	k.mu.Lock()
	if k.locks == nil {
		k.locks = make(map[string]*keyedLock)
	}
	l, ok := k.locks[key]
	if !ok {
		l = &keyedLock{}
		k.locks[key] = l
	}
	l.refs++
	k.mu.Unlock()
	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		k.mu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(k.locks, key)
		}
		k.mu.Unlock()
	}
}

// noteDedup remembers recently answered note ids (bounded, FIFO eviction) so a
// redelivered webhook for the same note is not answered twice.
type noteDedup struct {
	mu    sync.Mutex
	seen  map[int]struct{}
	order []int
	cap   int
}

func newNoteDedup(capacity int) *noteDedup {
	return &noteDedup{seen: make(map[int]struct{}), cap: capacity}
}

// markNew records id and reports whether it was newly seen. A zero id (unknown)
// is always treated as new so it is never silently dropped.
func (d *noteDedup) markNew(id int) bool {
	if id == 0 {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.seen[id]; ok {
		return false
	}
	d.seen[id] = struct{}{}
	d.order = append(d.order, id)
	if len(d.order) > d.cap {
		oldest := d.order[0]
		d.order = d.order[1:]
		delete(d.seen, oldest)
	}
	return true
}

// forget removes id so a redelivery can retry. Called when an attempt fails
// before a reply could have been posted — keeping the mark would discard the
// redelivered webhook and leave the question permanently unanswered. The stale
// entry in the eviction order is harmless (its later eviction is a no-op).
func (d *noteDedup) forget(id int) {
	if id == 0 {
		return
	}
	d.mu.Lock()
	delete(d.seen, id)
	d.mu.Unlock()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "reading body", http.StatusBadRequest)
		return
	}
	event, err := ParseEvent(body)
	if err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}
	group := h.groups.Match(event.Project.PathWithNamespace)
	if group == nil {
		h.log.Warn("webhook for unconfigured project", "project", event.Project.PathWithNamespace)
		http.Error(w, "unknown group", http.StatusUnauthorized)
		return
	}
	if !h.authenticate(group, r, body) {
		h.log.Warn("webhook authentication failed", "project", event.Project.PathWithNamespace, "group", group.Path, "method", authMethod(group))
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	decision := Decide(event, h.cfg.TriggerEmoji, h.cfg.CommandKeyword, h.groups.BotIDs())
	switch {
	case decision.Command == CommandChat:
		if h.chatRunner == nil {
			h.log.Debug("ignoring chat reply (chat disabled)", "project", event.Project.PathWithNamespace, "iid", decision.IID)
			writeJSON(w, map[string]string{"status": "ignored", "reason": "chat disabled"})
			return
		}
		go h.handleChat(group, event.Project.PathWithNamespace, decision)
		h.log.Debug("chat reply candidate", "project", event.Project.PathWithNamespace, "iid", decision.IID, "discussion", decision.DiscussionID)
		writeJSON(w, map[string]string{"status": "chat"})
	case decision.Command != CommandNone:
		h.handleCommand(event, group, decision)
		h.log.Info("command received", "project", event.Project.PathWithNamespace, "iid", decision.IID, "command", decision.Command.String(), "reason", decision.Reason)
		writeJSON(w, map[string]string{"status": "command", "command": decision.Command.String()})
	case decision.Kind == TriggerNone:
		h.log.Debug("ignoring event", "project", event.Project.PathWithNamespace, "reason", decision.Reason)
		writeJSON(w, map[string]string{"status": "ignored", "reason": decision.Reason})
	default:
		h.dispatcher.Enqueue(Event{
			Kind:        decision.Kind,
			ProjectID:   event.Project.ID,
			ProjectPath: event.Project.PathWithNamespace,
			IID:         decision.IID,
			HeadSHA:     decision.HeadSHA,
			Group:       group,
		})
		h.log.Info("event received", "project", event.Project.PathWithNamespace, "iid", decision.IID, "trigger", decision.Kind.String(), "reason", decision.Reason)
		writeJSON(w, map[string]string{"status": "queued"})
	}
}

// authenticate verifies the webhook against the group's configured credential:
// a GitLab signing token (HMAC over the raw body) when present, otherwise the
// legacy plaintext secret token.
func (h *Handler) authenticate(group *Group, r *http.Request, body []byte) bool {
	if group.UsesSigning() {
		return group.CheckSignature(
			r.Header.Get("Webhook-Id"),
			r.Header.Get("Webhook-Timestamp"),
			r.Header.Get("Webhook-Signature"),
			body,
			time.Now(),
		)
	}
	return group.CheckSecret(r.Header.Get("X-Gitlab-Token"))
}

// authMethod labels the group's verification method for logs.
func authMethod(group *Group) string {
	if group.UsesSigning() {
		return "signing_token"
	}
	return "secret_token"
}

// handleCommand executes one note command (or trigger-emoji revoke). Review
// requests re-enter the normal enqueue path with their manual-trigger
// bypasses intact; the reply policy is deliberately quiet — a reaction emoji
// acknowledges review/abort, a comment reply appears only when there is
// something to say.
func (h *Handler) handleCommand(event *WebhookEvent, group *Group, decision Decision) {
	projectID := event.Project.ID
	switch decision.Command {
	case CommandReview:
		h.dispatcher.Enqueue(Event{
			Kind:        decision.Kind,
			ProjectID:   projectID,
			ProjectPath: event.Project.PathWithNamespace,
			IID:         decision.IID,
			HeadSHA:     decision.HeadSHA,
			Group:       group,
		})
		go h.ackNote(group, projectID, decision, h.cfg.AckEmoji)
	case CommandAbort:
		outcome := h.dispatcher.Abort(projectID, decision.IID)
		go func() {
			h.ackNote(group, projectID, decision, h.cfg.AbortEmoji)
			// The emoji revoke has no note to answer; it only gets a
			// confirmation when something was actually aborted — revoking a
			// stale award is routine cleanup, not a question.
			if decision.NoteID == 0 && !outcome.Found {
				return
			}
			h.reply(group, projectID, decision, abortText(outcome))
		}()
	case CommandStatus:
		info := h.dispatcher.JobInfo(projectID, decision.IID)
		go h.reply(group, projectID, decision, statusText(info))
	case CommandHelp:
		go h.reply(group, projectID, decision, helpText(h.cfg.CommandKeyword))
	case CommandUnknown:
		go h.reply(group, projectID, decision, unknownText(h.cfg.CommandKeyword, decision.UnknownArg))
	}
}

// handleChat answers a discussion-thread reply by spawning a `nickpit chat`
// child, keeping the daemon free of LLM logic. It guards several hazards before
// spawning:
//   - a group whose bot identity is unresolved is skipped, because the daemon
//     cannot filter its own replies and would answer them in a loop;
//   - webhook redeliveries of an already-answered note are dropped;
//   - concurrent chat work is bounded by a semaphore, and replies within one
//     discussion are serialized so two quick replies do not race;
//   - a cheap thread gate confirms the thread was started by nickpit before a
//     child is spawned, so unrelated comments never launch a process.
//
// The child self-gates too and posts its reply. A lost reply on shutdown is
// acceptable, like command replies.
func (h *Handler) handleChat(group *Group, projectPath string, decision Decision) {
	if h.chatRunner == nil {
		return
	}
	// Without a resolved bot identity, the daemon cannot recognize (and skip) its
	// own posted replies, so answering would recurse. Disable chat for the group.
	if group.BotUserID == 0 {
		h.log.Warn("skipping chat: bot user id unresolved", "group", group.Path, "iid", decision.IID)
		return
	}
	if !h.chatSeen.markNew(decision.NoteID) {
		h.log.Debug("skipping duplicate chat note", "iid", decision.IID, "note", decision.NoteID)
		return
	}
	// Bound concurrent chat work; drop under load rather than pile up processes.
	// The dropped note is forgotten so a webhook redelivery can retry it.
	select {
	case h.chatSem <- struct{}{}:
	default:
		h.chatSeen.forget(decision.NoteID)
		h.log.Warn("chat busy, dropping reply", "iid", decision.IID, "discussion", decision.DiscussionID)
		return
	}
	defer func() { <-h.chatSem }()

	// Serialize replies within a discussion so ordering is preserved.
	unlock := h.chatLocks.lock(fmt.Sprintf("%s!%s", projectPath, decision.DiscussionID))
	defer unlock()

	ctx, cancel := context.WithTimeout(context.Background(), chatReplyTimeout)
	defer cancel()

	// Inexpensive gate: only spawn a child for a thread nickpit actually started.
	// A genuinely foreign thread keeps its dedup mark (redeliveries would re-gate
	// identically); a transient read failure forgets it so a redelivery retries.
	ours, err := h.isNickpitThread(ctx, group, projectPath, decision.IID, decision.DiscussionID)
	if err != nil {
		h.chatSeen.forget(decision.NoteID)
		h.log.Warn("chat gate: reading thread failed", "iid", decision.IID, "discussion", decision.DiscussionID, "error", err)
		return
	}
	if !ours {
		h.log.Debug("ignoring non-nickpit thread reply", "iid", decision.IID, "discussion", decision.DiscussionID)
		return
	}

	exitCode, logPath, err := h.chatRunner.RunChat(ctx, ChatSpec{
		ProjectPath:  projectPath,
		IID:          decision.IID,
		DiscussionID: decision.DiscussionID,
		NoteID:       decision.NoteID,
		Token:        group.Token,
		BaseURL:      h.chatCfg.BaseURL,
		ConfigPath:   h.chatCfg.ConfigPath,
		ExtraArgs:    h.chatCfg.ExtraArgs,
		LogDir:       h.chatCfg.LogDir,
	})
	switch {
	// A failed attempt posted no reply: forget the note so a redelivered webhook
	// is not discarded as a duplicate, which would leave the question unanswered
	// until eviction or daemon restart.
	case err != nil:
		h.chatSeen.forget(decision.NoteID)
		h.log.Warn("chat child failed to run", "iid", decision.IID, "discussion", decision.DiscussionID, "error", err)
	case exitCode != 0:
		h.chatSeen.forget(decision.NoteID)
		h.log.Warn("chat child exited with error", "iid", decision.IID, "discussion", decision.DiscussionID, "exit_code", exitCode, "log", logPath)
	default:
		h.log.Info("chat reply handled", "iid", decision.IID, "discussion", decision.DiscussionID)
	}
}

// isNickpitThread reports whether the discussion's root note carries a nickpit
// review marker AND was authored by this group's bot user, i.e. the thread was
// started by a nickpit review this daemon posted. The author check matters
// because markers are only encoded, not authenticated: without it any commenter
// could open a marker-bearing thread and route paid chat calls through the
// daemon. A read failure is returned as an error so the caller can distinguish
// "not ours" from "could not check".
func (h *Handler) isNickpitThread(ctx context.Context, group *Group, projectPath string, iid int, discussionID string) (bool, error) {
	notes, err := group.Client.DiscussionNotes(ctx, projectPath, iid, discussionID)
	if err != nil {
		return false, err
	}
	if len(notes) == 0 {
		return false, nil
	}
	if notes[0].AuthorID != group.BotUserID {
		return false, nil
	}
	_, _, ok := reviewmd.DetectThreadReview(notes[0].Body)
	return ok, nil
}

// ackNote awards the given acknowledgement emoji on the command note ("" skips).
// Failures are logged, never fatal: the command itself has already been executed.
func (h *Handler) ackNote(group *Group, projectID int, decision Decision, emoji string) {
	if emoji == "" || decision.NoteID == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandReplyTimeout)
	defer cancel()
	if err := group.Client.AwardNoteEmoji(ctx, projectID, decision.IID, decision.NoteID, emoji); err != nil {
		h.log.Warn("acknowledging command note failed", "iid", decision.IID, "note", decision.NoteID, "emoji", emoji, "error", err)
	}
}

// reply answers a command, threaded under its note when the payload carried a
// discussion id and GitLab accepts the reply, as a plain MR note otherwise.
func (h *Handler) reply(group *Group, projectID int, decision Decision, body string) {
	ctx, cancel := context.WithTimeout(context.Background(), commandReplyTimeout)
	defer cancel()
	if decision.DiscussionID != "" {
		err := group.Client.ReplyToMRDiscussion(ctx, projectID, decision.IID, decision.DiscussionID, body)
		if err == nil {
			return
		}
		// Some GitLab versions reject replies to individual-note discussions
		// with a 4xx; fall back to a plain note. 5xx and transport errors are
		// not retried against another endpoint.
		var apiErr *gitlab.APIError
		if !errors.As(err, &apiErr) || apiErr.Status >= 500 {
			h.log.Warn("posting command reply failed", "iid", decision.IID, "error", err)
			return
		}
	}
	if err := group.Client.CreateMRNote(ctx, projectID, decision.IID, body); err != nil {
		h.log.Warn("posting command reply failed", "iid", decision.IID, "error", err)
	}
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
