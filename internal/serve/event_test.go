package serve

import (
	"path/filepath"
	"testing"

	"github.com/dgrieser/nickpit/internal/testutil"
)

func loadEvent(t *testing.T, name string) *WebhookEvent {
	t.Helper()
	event, err := ParseEvent(testutil.LoadFixture(t, filepath.Join("testdata", name)))
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	return event
}

func TestDecide(t *testing.T) {
	botIDs := map[int]bool{999: true}
	cases := []struct {
		fixture     string
		wantKind    TriggerKind
		wantCommand CommandKind
		wantIID     int
		wantSHA     string
	}{
		{"mr_open.json", TriggerAuto, CommandNone, 7, "sha-open-1"},
		{"mr_open_draft.json", TriggerNone, CommandNone, 8, ""},
		// Pushes never auto re-review; re-review is manual (emoji/command) or
		// the ready transition.
		{"mr_update_oldrev.json", TriggerNone, CommandNone, 7, ""},
		{"mr_update_metadata.json", TriggerNone, CommandNone, 7, ""},
		{"mr_update_ready.json", TriggerAuto, CommandNone, 9, "sha-ready-1"},
		{"mr_close.json", TriggerNone, CommandNone, 7, ""},
		{"emoji_award_nickpit.json", TriggerManual, CommandNone, 11, "sha-emoji-1"},
		{"emoji_award_nickpit_draft.json", TriggerManual, CommandNone, 12, "sha-emoji-2"},
		// Revoking the trigger emoji withdraws the request: abort.
		{"emoji_revoke.json", TriggerNone, CommandAbort, 11, ""},
		{"emoji_revoke_eyes.json", TriggerNone, CommandNone, 0, ""},
		{"emoji_award_eyes.json", TriggerNone, CommandNone, 0, ""},
		{"emoji_award_bot.json", TriggerNone, CommandNone, 0, ""},
		{"emoji_award_note.json", TriggerNone, CommandNone, 0, ""},
		// A review command is a manual trigger, exactly like the emoji.
		{"note_command_review.json", TriggerManual, CommandReview, 11, "sha-note-1"},
		{"note_command_abort.json", TriggerNone, CommandAbort, 11, ""},
		{"note_command_status.json", TriggerNone, CommandStatus, 11, ""},
		{"note_command_help.json", TriggerNone, CommandHelp, 11, ""},
		{"note_command_unknown.json", TriggerNone, CommandUnknown, 11, ""},
		// A plain reply inside a discussion thread is a chat candidate; the
		// handler gates it on the thread's root marker before answering.
		{"note_plain.json", TriggerNone, CommandChat, 11, ""},
		// A plain reply with no discussion is a standalone comment, not a thread
		// reply, so it is ignored (no chat).
		{"note_plain_no_discussion.json", TriggerNone, CommandNone, 0, ""},
		{"note_command_bot.json", TriggerNone, CommandNone, 0, ""},
		{"note_system.json", TriggerNone, CommandNone, 0, ""},
		{"note_on_issue.json", TriggerNone, CommandNone, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			decision := Decide(loadEvent(t, tc.fixture), "nickpit", "nickpit", botIDs)
			if decision.Kind != tc.wantKind || decision.Command != tc.wantCommand {
				t.Fatalf("kind = %v, command = %v (%s), want kind=%v command=%v", decision.Kind, decision.Command, decision.Reason, tc.wantKind, tc.wantCommand)
			}
			if decision.Kind == TriggerNone && decision.Command == CommandNone {
				return
			}
			if decision.IID != tc.wantIID || decision.HeadSHA != tc.wantSHA {
				t.Fatalf("decision = %+v, want iid=%d sha=%q", decision, tc.wantIID, tc.wantSHA)
			}
		})
	}
}

// Note commands must carry the note id and discussion id so the handler can
// acknowledge and reply threaded.
func TestDecideNoteCarriesReplyContext(t *testing.T) {
	decision := Decide(loadEvent(t, "note_command_status.json"), "nickpit", "nickpit", nil)
	if decision.NoteID != 303 || decision.DiscussionID != "disc-303" {
		t.Fatalf("decision = %+v, want note id 303, discussion disc-303", decision)
	}
}

// A plain thread reply carries the note text and discussion id so the handler
// can pass the author's message to the discussion agent.
func TestDecideNoteChatCarriesText(t *testing.T) {
	decision := Decide(loadEvent(t, "note_plain.json"), "nickpit", "nickpit", nil)
	if decision.Command != CommandChat {
		t.Fatalf("expected chat command, got %v", decision.Command)
	}
	if decision.DiscussionID != "disc-306" || decision.NoteText != "looks good to me" {
		t.Fatalf("decision = %+v, want discussion disc-306 and note text", decision)
	}
}

// The daemon's own thread replies must not be treated as chat (loop guard).
func TestDecideNoteChatBotIgnored(t *testing.T) {
	event := loadEvent(t, "note_plain.json")
	decision := Decide(event, "nickpit", "nickpit", map[int]bool{event.User.ID: true})
	if decision.Command != CommandNone || decision.Kind != TriggerNone {
		t.Fatalf("bot thread reply should be ignored, got %+v", decision)
	}
}

func TestDecideNoteUnknownCarriesArg(t *testing.T) {
	decision := Decide(loadEvent(t, "note_command_unknown.json"), "nickpit", "nickpit", nil)
	if decision.Command != CommandUnknown || decision.UnknownArg != "frobnicate" {
		t.Fatalf("decision = %+v", decision)
	}
}

// Editing an old command comment must not re-trigger it.
func TestDecideNoteEditIgnored(t *testing.T) {
	event := loadEvent(t, "note_command_review.json")
	event.ObjectAttributes.Action = "update"
	decision := Decide(event, "nickpit", "nickpit", nil)
	if decision.Kind != TriggerNone || decision.Command != CommandNone {
		t.Fatalf("decision = %+v", decision)
	}
}

// Older GitLab versions send note events without an action field.
func TestDecideNoteWithoutActionAccepted(t *testing.T) {
	event := loadEvent(t, "note_command_review.json")
	event.ObjectAttributes.Action = ""
	decision := Decide(event, "nickpit", "nickpit", nil)
	if decision.Command != CommandReview {
		t.Fatalf("decision = %+v", decision)
	}
}

// The bot's own revoke (e.g. cleanup automation) must not abort.
func TestDecideBotRevokeIgnored(t *testing.T) {
	event := loadEvent(t, "emoji_revoke.json")
	event.User.ID = 999
	decision := Decide(event, "nickpit", "nickpit", map[int]bool{999: true})
	if decision.Command != CommandNone {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestDecideUnknownObjectKind(t *testing.T) {
	decision := Decide(&WebhookEvent{ObjectKind: "pipeline"}, "nickpit", "nickpit", nil)
	if decision.Kind != TriggerNone {
		t.Fatalf("kind = %v", decision.Kind)
	}
}

// A draft update carrying oldrev (push to a draft MR) must stay ignored: the
// draft skip applies to every auto path.
func TestDecideDraftPushIgnored(t *testing.T) {
	event := loadEvent(t, "mr_update_oldrev.json")
	event.ObjectAttributes.Draft = true
	decision := Decide(event, "nickpit", "nickpit", nil)
	if decision.Kind != TriggerNone || decision.Reason != "draft" {
		t.Fatalf("decision = %+v", decision)
	}
}

// A ready transition triggers even when GitLab omits oldrev (no new commits).
func TestDecideReadyTransitionWithoutOldrev(t *testing.T) {
	event := loadEvent(t, "mr_update_ready.json")
	if event.ObjectAttributes.OldRev != "" {
		t.Fatal("fixture must not carry oldrev")
	}
	decision := Decide(event, "nickpit", "nickpit", nil)
	if decision.Kind != TriggerAuto {
		t.Fatalf("decision = %+v", decision)
	}
}
