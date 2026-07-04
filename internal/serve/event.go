// Package serve implements the `nickpit gitlab serve` webhook daemon: it
// receives GitLab group webhooks, decides which merge-request and emoji
// events warrant a review, and spawns each review as a separate child
// process of the nickpit binary.
package serve

import "encoding/json"

// TriggerKind classifies how a review was requested.
type TriggerKind int

const (
	// TriggerNone marks events that must be ignored.
	TriggerNone TriggerKind = iota
	// TriggerAuto is a review caused by MR activity (open/reopen/new
	// commits/ready transition); it requires the project opt-in topic and is
	// deduplicated by head SHA.
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
}

type eventUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type eventProject struct {
	ID                int    `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
}

// eventAttributes covers both merge_request events (action, iid, draft,
// oldrev) and emoji events (action, name, awardable_type, awardable_id).
type eventAttributes struct {
	Action string `json:"action"`
	IID    int    `json:"iid"`
	Draft  bool   `json:"draft"`
	State  string `json:"state"`
	OldRev string `json:"oldrev"`
	// last_commit.id is the MR head SHA in merge_request events.
	LastCommit struct {
		ID string `json:"id"`
	} `json:"last_commit"`
	// Emoji-event fields.
	Name          string `json:"name"`
	AwardableType string `json:"awardable_type"`
	AwardableID   int    `json:"awardable_id"`
}

type eventChanges struct {
	Draft *struct {
		Previous bool `json:"previous"`
		Current  bool `json:"current"`
	} `json:"draft"`
}

// eventMR is the merge_request object embedded in emoji events.
type eventMR struct {
	IID        int    `json:"iid"`
	Draft      bool   `json:"draft"`
	State      string `json:"state"`
	LastCommit struct {
		ID string `json:"id"`
	} `json:"last_commit"`
}

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
}

// Decide classifies a webhook event. Pure function — no I/O — so the trigger
// policy is exhaustively unit-testable. triggerEmoji is the award-emoji name
// requesting a manual review; botIDs are the daemon's own user IDs whose
// emoji events are ignored to prevent reaction loops.
func Decide(event *WebhookEvent, triggerEmoji string, botIDs map[int]bool) Decision {
	switch event.ObjectKind {
	case "merge_request":
		return decideMR(event)
	case "emoji":
		return decideEmoji(event, triggerEmoji, botIDs)
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
		if attrs.OldRev == "" {
			return ignore("metadata update")
		}
		return trigger("new commits")
	default:
		return ignore("action " + attrs.Action)
	}
}

func readyTransition(changes eventChanges) bool {
	return changes.Draft != nil && changes.Draft.Previous && !changes.Draft.Current
}

func decideEmoji(event *WebhookEvent, triggerEmoji string, botIDs map[int]bool) Decision {
	attrs := event.ObjectAttributes
	if attrs.Action != "award" {
		return Decision{Kind: TriggerNone, Reason: "emoji " + attrs.Action}
	}
	if attrs.Name != triggerEmoji {
		return Decision{Kind: TriggerNone, Reason: "emoji name " + attrs.Name}
	}
	if attrs.AwardableType != "MergeRequest" {
		return Decision{Kind: TriggerNone, Reason: "awardable " + attrs.AwardableType}
	}
	if botIDs[event.User.ID] {
		return Decision{Kind: TriggerNone, Reason: "own award"}
	}
	if event.MergeRequest == nil {
		return Decision{Kind: TriggerNone, Reason: "emoji event without merge_request"}
	}
	// Draft is deliberately NOT checked: an explicit human request reviews
	// draft MRs too.
	return Decision{
		Kind:    TriggerManual,
		Reason:  "emoji " + triggerEmoji,
		IID:     event.MergeRequest.IID,
		HeadSHA: event.MergeRequest.LastCommit.ID,
	}
}
