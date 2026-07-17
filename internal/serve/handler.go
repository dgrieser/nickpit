package serve

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	gitlab "github.com/dgrieser/nickpit/internal/scm/gitlab"
)

// maxWebhookBody bounds webhook payloads; GitLab MR events stay far below.
const maxWebhookBody = 10 << 20 // 10 MiB

// commandReplyTimeout bounds the fire-and-forget GitLab calls acknowledging
// and answering command comments.
const commandReplyTimeout = 30 * time.Second

// chatReplyTimeout bounds a discussion-agent reply, which runs an LLM turn (and
// possibly tool calls) in-process, so it is far longer than a plain command
// reply.
const chatReplyTimeout = 10 * time.Minute

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

// Handler is the webhook HTTP endpoint. It only parses, authenticates, and
// classifies — every API call and the review itself happen async, keeping the
// response inside GitLab's ~10s webhook timeout. Command state changes
// (enqueue, abort, status) are synchronous mutex-only dispatcher calls; only
// the GitLab acknowledgements and replies run in goroutines, which are fire
// and forget: a reply may be lost when the daemon shuts down mid-flight.
// ChatConfig carries the static invocation details a spawned chat child needs
// (the daemon's config path, GitLab base URL, log dir, and any extra args). The
// per-group token is taken from the matched group at spawn time.
type ChatConfig struct {
	ConfigPath string
	BaseURL    string
	LogDir     string
	ExtraArgs  []string
}

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
}

func NewHandler(groups *GroupSet, dispatcher *Dispatcher, cfg HandlerConfig, chatRunner ChatRunner, chatCfg ChatConfig, log *slog.Logger) *Handler {
	return &Handler{groups: groups, dispatcher: dispatcher, cfg: cfg, chatRunner: chatRunner, chatCfg: chatCfg, log: log}
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
// child, keeping the daemon free of LLM logic. The child self-gates (a thread
// nickpit did not start is a quiet no-op) and posts its reply itself. Runs under
// chatReplyTimeout; a lost reply on shutdown is acceptable, like command replies.
func (h *Handler) handleChat(group *Group, projectPath string, decision Decision) {
	if h.chatRunner == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), chatReplyTimeout)
	defer cancel()
	exitCode, logPath, err := h.chatRunner.RunChat(ctx, ChatSpec{
		ProjectPath:  projectPath,
		IID:          decision.IID,
		DiscussionID: decision.DiscussionID,
		Token:        group.Token,
		BaseURL:      h.chatCfg.BaseURL,
		ConfigPath:   h.chatCfg.ConfigPath,
		ExtraArgs:    h.chatCfg.ExtraArgs,
		LogDir:       h.chatCfg.LogDir,
	})
	switch {
	case err != nil:
		h.log.Warn("chat child failed to run", "iid", decision.IID, "discussion", decision.DiscussionID, "error", err)
	case exitCode != 0:
		h.log.Warn("chat child exited with error", "iid", decision.IID, "discussion", decision.DiscussionID, "exit_code", exitCode, "log", logPath)
	default:
		h.log.Info("chat reply handled", "iid", decision.IID, "discussion", decision.DiscussionID)
	}
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
