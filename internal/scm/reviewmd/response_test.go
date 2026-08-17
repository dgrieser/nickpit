package reviewmd

import (
	"strings"
	"testing"
)

func TestResponseFooterUpsertAndStrip(t *testing.T) {
	body := SummaryMarker + "\nvisible review"
	status := ResponseStatus{
		Enabled: true, MuteEmoji: "mute", RequestEmoji: "nickpit", CommandKeyword: "nickpit",
	}
	withFooter := UpsertResponseFooter(body, status)
	if !strings.Contains(withFooter, "NickPit will respond") || !strings.Contains(withFooter, ":mute:") || !strings.Contains(withFooter, "/nickpit mute") {
		t.Fatalf("footer = %q", withFooter)
	}
	if got := StripMarkers(withFooter); got != "visible review" {
		t.Fatalf("StripMarkers = %q", got)
	}
	if got := UpsertResponseFooter(withFooter, status); got != withFooter {
		t.Fatalf("upsert not idempotent:\n%s", got)
	}
}

func TestResponseFooterPersistsCommandMute(t *testing.T) {
	status := ResponseStatus{Enabled: true, CommandMuted: true, MuteEmoji: "mute", CommandKeyword: "bot"}
	body := UpsertResponseFooter("review", status)
	if !ThreadCommandMuted(body) || !strings.Contains(body, "/bot resume") {
		t.Fatalf("muted footer = %q", body)
	}
	status.CommandMuted = false
	resumed := UpsertResponseFooter(body, status)
	if ThreadCommandMuted(resumed) || !strings.Contains(resumed, "will respond") {
		t.Fatalf("resumed footer = %q", resumed)
	}
}

func TestResponseFooterOptInAndBlockers(t *testing.T) {
	optIn := UpsertResponseFooter("review", ResponseStatus{
		Enabled: true, OptIn: true, MuteEmoji: "mute", RequestEmoji: "nickpit", CommandKeyword: "nickpit",
	})
	if !strings.Contains(optIn, "only when requested") || !strings.Contains(optIn, ":nickpit:") {
		t.Fatalf("opt-in footer = %q", optIn)
	}
	blocked := UpsertResponseFooter("review", ResponseStatus{
		Enabled: true, MRMuted: true, ThreadMuted: true, CommandMuted: true,
		MuteEmoji: "mute", CommandKeyword: "nickpit",
	})
	for _, want := range []string{"merge request", "this post", "/nickpit resume"} {
		if !strings.Contains(blocked, want) {
			t.Fatalf("blocked footer missing %q: %s", want, blocked)
		}
	}
}
