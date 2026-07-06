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

// HandlerConfig is the static trigger configuration for the webhook endpoint.
type HandlerConfig struct {
	// TriggerEmoji is the award-emoji name requesting a manual review.
	TriggerEmoji string
	// CommandKeyword is the "/<keyword> <command>" note-command keyword.
	CommandKeyword string
	// AckEmoji is awarded on command notes to acknowledge them; "" disables.
	AckEmoji string
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
}

func NewHandler(groups *GroupSet, dispatcher *Dispatcher, cfg HandlerConfig, log *slog.Logger) *Handler {
	return &Handler{groups: groups, dispatcher: dispatcher, cfg: cfg, log: log}
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
	if !group.CheckSecret(r.Header.Get("X-Gitlab-Token")) {
		h.log.Warn("webhook secret mismatch", "project", event.Project.PathWithNamespace, "group", group.Path)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	decision := Decide(event, h.cfg.TriggerEmoji, h.cfg.CommandKeyword, h.groups.BotIDs())
	switch {
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
		h.log.Info("review queued", "project", event.Project.PathWithNamespace, "iid", decision.IID, "trigger", decision.Kind.String(), "reason", decision.Reason)
		writeJSON(w, map[string]string{"status": "queued"})
	}
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
		go h.ackNote(group, projectID, decision)
	case CommandAbort:
		outcome := h.dispatcher.Abort(projectID, decision.IID)
		go func() {
			h.ackNote(group, projectID, decision)
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

// ackNote awards the acknowledgement emoji on the command note. Failures are
// logged, never fatal: the command itself has already been executed.
func (h *Handler) ackNote(group *Group, projectID int, decision Decision) {
	if h.cfg.AckEmoji == "" || decision.NoteID == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandReplyTimeout)
	defer cancel()
	if err := group.Client.AwardNoteEmoji(ctx, projectID, decision.IID, decision.NoteID, h.cfg.AckEmoji); err != nil {
		h.log.Warn("acknowledging command note failed", "iid", decision.IID, "note", decision.NoteID, "error", err)
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
