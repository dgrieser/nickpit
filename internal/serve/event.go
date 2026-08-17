// Package serve implements the `nickpit gitlab serve` webhook daemon: it
// receives GitLab group webhooks, decides which merge-request, emoji, and
// comment events warrant a review (or a command such as abort/status), and
// spawns each review as a separate child process of the nickpit binary.
package serve

import "encoding/json"

// TriggerKind classifies how a review was requested.
type TriggerKind int

const (
	// TriggerNone marks events that must be ignored.
	TriggerNone TriggerKind = iota
	// TriggerAuto is a review caused by MR activity (open/reopen/ready
	// transition — deliberately not new commits); it requires the project
	// opt-in topic and is deduplicated by head SHA.
	TriggerAuto
	// TriggerManual is a review explicitly requested by a user awarding the
	// trigger emoji; it bypasses the topic check, the SHA dedup, and the
	// draft skip.
	TriggerManual
)

func (k TriggerKind) String() string {
	switch k {
	case TriggerAuto:
		return "auto"
	case TriggerManual:
		return "manual"
	default:
		return "none"
	}
}

// WebhookEvent is the envelope of the GitLab webhook payloads the daemon
// consumes (merge request events and emoji events). Unknown kinds keep their
// raw ObjectKind for logging.
type WebhookEvent struct {
	ObjectKind       string          `json:"object_kind"`
	User             eventUser       `json:"user"`
	Project          eventProject    `json:"project"`
	ObjectAttributes eventAttributes `json:"object_attributes"`
	Changes          eventChanges    `json:"changes"`
	MergeRequest     *eventMR        `json:"merge_request"`
	Note             *eventNote      `json:"note"`
}

type eventUser struct {
	ID int `json:"id"`
}

type eventProject struct {
	ID                int    `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
}

// eventAttributes covers merge_request events (action, iid, draft, oldrev),
// emoji events (action, name, awardable_type), and note events
// (id, note, noteable_type, system, discussion_id).
type eventAttributes struct {
	Action string `json:"action"`
	IID    int    `json:"iid"`
	Draft  bool   `json:"draft"`
	OldRev string `json:"oldrev"`
	// last_commit.id is the MR head SHA in merge_request events.
	LastCommit struct {
		ID string `json:"id"`
	} `json:"last_commit"`
	// Emoji-event fields.
	Name          string `json:"name"`
	AwardableType string `json:"awardable_type"`
	// Note-event fields. ID is the note's id (present on other kinds too,
	// unused there). Type distinguishes threaded notes ("DiscussionNote",
	// "DiffNote") from top-level comments (null) — GitLab sends discussion_id
	// on EVERY note (a top-level comment opens its own individual-note
	// discussion), so discussion_id alone cannot tell a thread reply apart.
	ID           int    `json:"id"`
	Note         string `json:"note"`
	NoteableType string `json:"noteable_type"`
	NoteType     string `json:"type"`
	System       bool   `json:"system"`
	DiscussionID string `json:"discussion_id"`
}

type eventChanges struct {
	Draft *struct {
		Previous bool `json:"previous"`
		Current  bool `json:"current"`
	} `json:"draft"`
}

// eventMR is the merge_request object embedded in emoji and note events. The
// worker does not trust its draft/state snapshot: the authoritative values are
// re-read from the API (gitlab.MRStatus) before a review runs, so only the
// fields the daemon actually consumes are decoded here.
type eventMR struct {
	IID        int `json:"iid"`
	LastCommit struct {
		ID string `json:"id"`
	} `json:"last_commit"`
}

// eventNote is the target note embedded in Emoji Hook payloads.
type eventNote struct {
	ID           int    `json:"id"`
	Note         string `json:"note"`
	NoteableType string `json:"noteable_type"`
	AuthorID     int    `json:"author_id"`
	DiscussionID string `json:"discussion_id"`
	Type         string `json:"type"`
	System       bool   `json:"system"`
}

// ResponseAction changes or refreshes discussion-response state.
type ResponseAction int

const (
	ResponseNone ResponseAction = iota
	ResponseMuteThread
	ResponseResumeThread
	ResponseSyncThread
	ResponseSyncMR
)

// ParseEvent decodes a webhook body into the envelope. It does not validate
// semantics; Decide does.
func ParseEvent(body []byte) (*WebhookEvent, error) {
	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// Decision is the outcome of classifying one webhook event.
type Decision struct {
	Kind   TriggerKind
	Reason string
	// IID and HeadSHA identify the MR to review; HeadSHA may be empty when
	// the payload does not carry one (open events, emoji events).
	IID     int
	HeadSHA string
	// Command is set for note commands and the trigger-emoji revoke;
	// CommandNone otherwise. CommandReview additionally sets Kind to
	// TriggerManual so it flows through the normal enqueue path.
	Command CommandKind
	// NoteID is the command note to acknowledge; 0 when there is none (emoji
	// revoke).
	NoteID int
	// DiscussionID threads replies under the command note; empty falls back
	// to a plain MR note.
	DiscussionID string
	// UnknownArg is the raw subcommand for CommandUnknown replies.
	UnknownArg string
	// Requested marks an explicit opt-in request (positive command or trigger
	// emoji on the question note).
	Requested bool
	// PromptBody is the triggering comment with control/skip lines removed.
	PromptBody string
	// Response requests a persistent thread change or footer reconciliation.
	Response ResponseAction
}

// Decide classifies a webhook event. Pure function — no I/O — so the trigger
// policy is exhaustively unit-testable. triggerEmoji is the award-emoji name
// requesting a manual review; commandKeyword is the "/<keyword> <command>"
// note-command keyword; botIDs are the daemon's own user IDs whose emoji and
// note events are ignored to prevent reaction loops.
func Decide(event *WebhookEvent, triggerEmoji, muteEmoji, commandKeyword string, skipPhrases []string, botIDs map[int]bool) Decision {
	switch event.ObjectKind {
	case "merge_request":
		return decideMR(event)
	case "emoji":
		return decideEmoji(event, triggerEmoji, muteEmoji, botIDs)
	case "note":
		return decideNote(event, commandKeyword, skipPhrases, botIDs)
	default:
		return Decision{Kind: TriggerNone, Reason: "object_kind " + event.ObjectKind}
	}
}

func decideMR(event *WebhookEvent) Decision {
	attrs := event.ObjectAttributes
	ignore := func(reason string) Decision {
		return Decision{Kind: TriggerNone, Reason: reason, IID: attrs.IID}
	}
	trigger := func(reason string) Decision {
		return Decision{Kind: TriggerAuto, Reason: reason, IID: attrs.IID, HeadSHA: attrs.LastCommit.ID}
	}
	switch attrs.Action {
	case "open", "reopen":
		if attrs.Draft {
			return ignore("draft")
		}
		return trigger(attrs.Action)
	case "update":
		if readyTransition(event.Changes) {
			// The ready transition is the author asking for eyes; the draft
			// flag in the same payload is already false.
			return trigger("marked ready")
		}
		if attrs.Draft {
			return ignore("draft")
		}
		if attrs.OldRev != "" {
			// Pushes deliberately do not re-review: every commit to an active
			// MR would re-run a full review. Re-review is on request only
			// (trigger emoji, review command) or the ready transition.
			return ignore("new commits (re-review on request only)")
		}
		return ignore("metadata update")
	default:
		return ignore("action " + attrs.Action)
	}
}

func readyTransition(changes eventChanges) bool {
	return changes.Draft != nil && changes.Draft.Previous && !changes.Draft.Current
}

func decideEmoji(event *WebhookEvent, triggerEmoji, muteEmoji string, botIDs map[int]bool) Decision {
	attrs := event.ObjectAttributes
	if attrs.Name != triggerEmoji && (muteEmoji == "" || attrs.Name != muteEmoji) {
		return Decision{Kind: TriggerNone, Reason: "emoji name " + attrs.Name}
	}
	if botIDs[event.User.ID] {
		return Decision{Kind: TriggerNone, Reason: "own " + attrs.Action}
	}
	if event.MergeRequest == nil {
		return Decision{Kind: TriggerNone, Reason: "emoji event without merge_request"}
	}
	if attrs.Name == muteEmoji {
		switch attrs.AwardableType {
		case "MergeRequest":
			return Decision{Reason: "MR response emoji " + attrs.Action, IID: event.MergeRequest.IID, Response: ResponseSyncMR}
		case "Note":
			if event.Note == nil || event.Note.NoteableType != "MergeRequest" || event.Note.DiscussionID == "" {
				return Decision{Kind: TriggerNone, Reason: "mute emoji without MR note"}
			}
			return Decision{
				Reason: "thread response emoji " + attrs.Action, IID: event.MergeRequest.IID,
				NoteID: event.Note.ID, DiscussionID: event.Note.DiscussionID,
				Response: ResponseSyncThread,
			}
		default:
			return Decision{Kind: TriggerNone, Reason: "awardable " + attrs.AwardableType}
		}
	}
	if attrs.AwardableType == "Note" {
		if attrs.Action != "award" || event.Note == nil || event.Note.NoteableType != "MergeRequest" || event.Note.DiscussionID == "" {
			return Decision{Kind: TriggerNone, Reason: "request emoji on unsupported note"}
		}
		// A request reaction is only meaningful on a human question. In
		// particular, reacting to a nickpit root or one of its replies must not
		// recursively invoke another response.
		if event.Note.AuthorID <= 0 || botIDs[event.Note.AuthorID] {
			return Decision{Kind: TriggerNone, Reason: "request emoji on bot or unknown note author"}
		}
		return Decision{
			Command: CommandChat, Requested: true, Reason: "chat request emoji " + triggerEmoji,
			IID: event.MergeRequest.IID, NoteID: event.Note.ID,
			DiscussionID: event.Note.DiscussionID, PromptBody: event.Note.Note,
		}
	}
	if attrs.AwardableType != "MergeRequest" {
		return Decision{Kind: TriggerNone, Reason: "awardable " + attrs.AwardableType}
	}
	switch attrs.Action {
	case "award":
		// Draft is deliberately NOT checked: an explicit human request
		// reviews draft MRs too.
		return Decision{
			Kind:    TriggerManual,
			Reason:  "emoji " + triggerEmoji,
			IID:     event.MergeRequest.IID,
			HeadSHA: event.MergeRequest.LastCommit.ID,
		}
	case "revoke":
		// Taking the trigger emoji back withdraws the request: abort the
		// MR's queued or running review.
		return Decision{
			Command: CommandAbort,
			Reason:  "emoji " + triggerEmoji + " revoked",
			IID:     event.MergeRequest.IID,
		}
	default:
		return Decision{Kind: TriggerNone, Reason: "emoji " + attrs.Action}
	}
}

// decideNote classifies comment (Note Hook) events: new MR comments whose
// first line is "/<keyword> <command>" control the daemon.
func decideNote(event *WebhookEvent, commandKeyword string, skipPhrases []string, botIDs map[int]bool) Decision {
	attrs := event.ObjectAttributes
	if attrs.NoteableType != "MergeRequest" {
		return Decision{Kind: TriggerNone, Reason: "noteable " + attrs.NoteableType}
	}
	if attrs.System {
		return Decision{Kind: TriggerNone, Reason: "system note"}
	}
	// Note Hooks fire on creation; older GitLab versions send no action
	// field. Anything else (e.g. "update" edits) is ignored so editing an old
	// command comment cannot re-trigger it.
	if attrs.Action != "" && attrs.Action != "create" {
		return Decision{Kind: TriggerNone, Reason: "note " + attrs.Action}
	}
	// The daemon's own replies fire note webhooks back at it; drop them
	// before parsing.
	if botIDs[event.User.ID] {
		return Decision{Kind: TriggerNone, Reason: "own note"}
	}
	command, arg := ParseCommand(attrs.Note, commandKeyword)
	if event.MergeRequest == nil {
		return Decision{Kind: TriggerNone, Reason: "note event without merge_request"}
	}
	threaded := attrs.DiscussionID != "" && (attrs.NoteType == "DiscussionNote" || attrs.NoteType == "DiffNote")
	if threaded && (command == CommandNone || command == CommandUnknown) {
		directives := ParseChatDirectives(attrs.Note, commandKeyword, skipPhrases)
		if directives.Mute {
			return Decision{
				Reason: "command mute", IID: event.MergeRequest.IID, NoteID: attrs.ID,
				DiscussionID: attrs.DiscussionID, Response: ResponseMuteThread,
			}
		}
		if directives.Resume {
			decision := Decision{
				Reason: "command resume", IID: event.MergeRequest.IID, NoteID: attrs.ID,
				DiscussionID: attrs.DiscussionID, Response: ResponseResumeThread,
				Requested: true, PromptBody: directives.Remaining,
			}
			if !directives.Skip && directives.Remaining != "" {
				decision.Command = CommandChat
			}
			return decision
		}
		if directives.Skip {
			return Decision{Kind: TriggerNone, Reason: "chat skip phrase"}
		}
	}
	if command == CommandNone {
		// A plain reply inside a discussion thread is a discussion-agent
		// candidate. Thread membership is decided by the note TYPE
		// (DiscussionNote for threaded replies, DiffNote for diff-anchored
		// threads) — NOT by discussion_id, which GitLab sends on every note
		// including top-level comments. Without the type check every human MR
		// comment would consume a chat admission slot, an authenticated thread
		// read, and a permanent dedup entry. The handler further requires the
		// thread's root note to carry a bot-authored nickpit review marker
		// before answering, so unrelated thread replies are dropped without a
		// reply.
		if threaded {
			return Decision{
				Command:      CommandChat,
				Reason:       "chat reply",
				IID:          event.MergeRequest.IID,
				NoteID:       attrs.ID,
				DiscussionID: attrs.DiscussionID,
				PromptBody:   attrs.Note,
			}
		}
		return Decision{Kind: TriggerNone, Reason: "no command"}
	}
	decision := Decision{
		Command:      command,
		Reason:       "command " + command.String(),
		IID:          event.MergeRequest.IID,
		NoteID:       attrs.ID,
		DiscussionID: attrs.DiscussionID,
		UnknownArg:   arg,
	}
	if command == CommandReview {
		// A comment command is an explicit human request, exactly like the
		// trigger emoji: manual kind, drafts included.
		decision.Kind = TriggerManual
		decision.HeadSHA = event.MergeRequest.LastCommit.ID
	}
	return decision
}
