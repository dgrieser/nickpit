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
		fixture  string
		wantKind TriggerKind
		wantIID  int
		wantSHA  string
	}{
		{"mr_open.json", TriggerAuto, 7, "sha-open-1"},
		{"mr_open_draft.json", TriggerNone, 8, ""},
		{"mr_update_oldrev.json", TriggerAuto, 7, "sha-new-2"},
		{"mr_update_metadata.json", TriggerNone, 7, ""},
		{"mr_update_ready.json", TriggerAuto, 9, "sha-ready-1"},
		{"mr_close.json", TriggerNone, 7, ""},
		{"emoji_award_nickpit.json", TriggerManual, 11, "sha-emoji-1"},
		{"emoji_award_nickpit_draft.json", TriggerManual, 12, "sha-emoji-2"},
		{"emoji_revoke.json", TriggerNone, 0, ""},
		{"emoji_award_eyes.json", TriggerNone, 0, ""},
		{"emoji_award_bot.json", TriggerNone, 0, ""},
		{"emoji_award_note.json", TriggerNone, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			decision := Decide(loadEvent(t, tc.fixture), "nickpit", botIDs)
			if decision.Kind != tc.wantKind {
				t.Fatalf("kind = %v (%s), want %v", decision.Kind, decision.Reason, tc.wantKind)
			}
			if decision.Kind == TriggerNone {
				return
			}
			if decision.IID != tc.wantIID || decision.HeadSHA != tc.wantSHA {
				t.Fatalf("decision = %+v, want iid=%d sha=%q", decision, tc.wantIID, tc.wantSHA)
			}
		})
	}
}

func TestDecideUnknownObjectKind(t *testing.T) {
	decision := Decide(&WebhookEvent{ObjectKind: "pipeline"}, "nickpit", nil)
	if decision.Kind != TriggerNone {
		t.Fatalf("kind = %v", decision.Kind)
	}
}

// A draft update carrying oldrev (push to a draft MR) must stay ignored: the
// draft skip applies to every auto path.
func TestDecideDraftPushIgnored(t *testing.T) {
	event := loadEvent(t, "mr_update_oldrev.json")
	event.ObjectAttributes.Draft = true
	decision := Decide(event, "nickpit", nil)
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
	decision := Decide(event, "nickpit", nil)
	if decision.Kind != TriggerAuto {
		t.Fatalf("decision = %+v", decision)
	}
}
