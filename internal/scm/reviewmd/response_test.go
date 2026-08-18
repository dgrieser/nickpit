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

// A footer stamped under earlier settings advertises controls the daemon no
// longer honors. The stamped fingerprint tracks exactly the configuration
// inputs that shape the text, so callers can tell a current footer from a
// stale one without diffing rendered prose — and it never reaches an LLM.
func TestResponseFooterTracksPolicyChanges(t *testing.T) {
	policy := ResponseStatus{Enabled: true, OptIn: true, MuteEmoji: "mute", RequestEmoji: "nickpit", CommandKeyword: "nickpit"}
	body := UpsertResponseFooter("Finding body", policy)

	if !FooterMatchesPolicy(body, policy) {
		t.Fatalf("freshly stamped footer does not match its own policy: %q", body)
	}
	// Live per-thread state is deliberately NOT part of the fingerprint: mute
	// reactions and the command marker reconcile through their own events.
	live := policy
	live.MRMuted, live.ThreadMuted, live.CommandMuted = true, true, true
	if !FooterMatchesPolicy(UpsertResponseFooter("Finding body", live), policy) {
		t.Fatal("live thread state changed the policy fingerprint")
	}
	for name, changed := range map[string]ResponseStatus{
		"chat disabled":   {OptIn: true, MuteEmoji: "mute", RequestEmoji: "nickpit", CommandKeyword: "nickpit"},
		"automatic mode":  {Enabled: true, MuteEmoji: "mute", RequestEmoji: "nickpit", CommandKeyword: "nickpit"},
		"renamed mute":    {Enabled: true, OptIn: true, MuteEmoji: "no_bell", RequestEmoji: "nickpit", CommandKeyword: "nickpit"},
		"renamed request": {Enabled: true, OptIn: true, MuteEmoji: "mute", RequestEmoji: "robot", CommandKeyword: "nickpit"},
		"renamed keyword": {Enabled: true, OptIn: true, MuteEmoji: "mute", RequestEmoji: "nickpit", CommandKeyword: "bot"},
	} {
		if FooterMatchesPolicy(body, changed) {
			t.Fatalf("%s did not change the policy fingerprint", name)
		}
	}
	if got := StripResponseFooter(body); got != "Finding body" {
		t.Fatalf("policy marker survived stripping: %q", got)
	}
}
