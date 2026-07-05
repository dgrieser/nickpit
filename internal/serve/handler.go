package serve

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

// maxWebhookBody bounds webhook payloads; GitLab MR events stay far below.
const maxWebhookBody = 10 << 20 // 10 MiB

// Handler is the webhook HTTP endpoint. It only parses, authenticates, and
// classifies — every API call and the review itself happen async in the
// dispatcher, keeping the response inside GitLab's ~10s webhook timeout.
type Handler struct {
	groups       *GroupSet
	dispatcher   *Dispatcher
	triggerEmoji string
	log          *slog.Logger
}

func NewHandler(groups *GroupSet, dispatcher *Dispatcher, triggerEmoji string, log *slog.Logger) *Handler {
	return &Handler{groups: groups, dispatcher: dispatcher, triggerEmoji: triggerEmoji, log: log}
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

	decision := Decide(event, h.triggerEmoji, h.groups.BotIDs())
	if decision.Kind == TriggerNone {
		h.log.Debug("ignoring event", "project", event.Project.PathWithNamespace, "reason", decision.Reason)
		writeJSON(w, map[string]string{"status": "ignored", "reason": decision.Reason})
		return
	}
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

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
