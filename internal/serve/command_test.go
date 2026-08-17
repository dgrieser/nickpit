package serve

import (
	"strings"
	"testing"
	"time"
)

func TestParseCommand(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		keyword  string
		wantKind CommandKind
		wantArg  string
	}{
		{"review", "/nickpit review", "nickpit", CommandReview, ""},
		{"abort", "/nickpit abort", "nickpit", CommandAbort, ""},
		{"status", "/nickpit status", "nickpit", CommandStatus, ""},
		{"help", "/nickpit help", "nickpit", CommandHelp, ""},
		{"bare keyword is help", "/nickpit", "nickpit", CommandHelp, ""},
		{"case insensitive", "/NickPit REVIEW", "nickpit", CommandReview, ""},
		{"leading whitespace", "   /nickpit review", "nickpit", CommandReview, ""},
		{"trailing args ignored", "/nickpit review please, now", "nickpit", CommandReview, ""},
		{"trailing word ignored", "/nickpit review me", "nickpit", CommandReview, ""},
		{"leading blank lines skipped", "\n\n/nickpit review", "nickpit", CommandReview, ""},
		{"leading blank line then text", "\n  \nhey\n/nickpit review", "nickpit", CommandNone, ""},
		{"multiline command first", "/nickpit abort\nthis is taking too long", "nickpit", CommandAbort, ""},
		{"command on second line ignored", "hey\n/nickpit review", "nickpit", CommandNone, ""},
		{"unknown subcommand", "/nickpit frobnicate", "nickpit", CommandUnknown, "frobnicate"},
		{"keyword mismatch", "/nickpitx review", "nickpit", CommandNone, ""},
		{"missing slash", "nickpit review", "nickpit", CommandNone, ""},
		{"plain note", "looks good to me", "nickpit", CommandNone, ""},
		{"empty note", "", "nickpit", CommandNone, ""},
		{"custom keyword", "/bot review", "bot", CommandReview, ""},
		{"default keyword not custom", "/nickpit review", "bot", CommandNone, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, arg := ParseCommand(tc.body, tc.keyword)
			if kind != tc.wantKind || arg != tc.wantArg {
				t.Fatalf("ParseCommand(%q, %q) = (%v, %q), want (%v, %q)", tc.body, tc.keyword, kind, arg, tc.wantKind, tc.wantArg)
			}
		})
	}
}

func TestParseChatDirectives(t *testing.T) {
	cases := []struct {
		name                  string
		body                  string
		wantMute, wantResume  bool
		wantRequest, wantSkip bool
		wantRemaining         string
	}{
		{"mute alias anywhere", "please stop\n /NickPit   SHUTUP \nthanks", true, false, false, false, "please stop\nthanks"},
		{"positive requests same comment", "Why?\n/nickpit resume", false, true, true, false, "Why?"},
		{"positive only has no prompt", "/nickpit comment", false, true, true, false, ""},
		{"skip normalized", "question\n NO    BOT ", false, false, false, true, "question"},
		{"substring does not match", "prefix no bot suffix", false, false, false, false, "prefix no bot suffix"},
		{"arguments reject command", "/nickpit mute now", false, false, false, false, "/nickpit mute now"},
		{"mute wins conflict", "/nickpit respond\nquestion\n/nickpit ignore", true, false, false, false, "question"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseChatDirectives(tc.body, "nickpit", []string{"no bot"})
			if got.Mute != tc.wantMute || got.Resume != tc.wantResume || got.Request != tc.wantRequest || got.Skip != tc.wantSkip || got.Remaining != tc.wantRemaining {
				t.Fatalf("directives = %+v", got)
			}
		})
	}
}

// Replies must never start with the command prefix, or the daemon's own notes
// could read as commands.
func TestReplyTextsNeverStartWithSlash(t *testing.T) {
	texts := []string{
		helpText("nickpit"),
		unknownText("nickpit", "frobnicate"),
		statusText(JobInfo{}),
		statusText(JobInfo{Running: true, Since: 3 * time.Second}),
		abortText(AbortOutcome{}),
		abortText(AbortOutcome{Found: true, Running: true, Since: time.Minute}),
	}
	for _, text := range texts {
		if strings.HasPrefix(strings.TrimSpace(text), "/") {
			t.Fatalf("reply text starts with '/': %q", text)
		}
	}
}

func TestStatusText(t *testing.T) {
	cases := []struct {
		name string
		info JobInfo
		want string
	}{
		{"idle", JobInfo{}, "No review is queued or running"},
		{"queued", JobInfo{Queued: true}, "queued and will start shortly"},
		{"running", JobInfo{Running: true, Since: 90 * time.Second}, "running for 1m30s"},
		{"running with pending", JobInfo{Running: true, Since: time.Second, Pending: true}, "queued behind it"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusText(tc.info); !strings.Contains(got, tc.want) {
				t.Fatalf("statusText(%+v) = %q, want containing %q", tc.info, got, tc.want)
			}
		})
	}
}

func TestAbortText(t *testing.T) {
	cases := []struct {
		name    string
		outcome AbortOutcome
		want    string
	}{
		{"nothing", AbortOutcome{}, "No review is queued or running"},
		{"queued", AbortOutcome{Found: true}, "Removed the queued review"},
		{"running", AbortOutcome{Found: true, Running: true, Since: 30 * time.Second}, "Aborted the running review (after 30s)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := abortText(tc.outcome); !strings.Contains(got, tc.want) {
				t.Fatalf("abortText(%+v) = %q, want containing %q", tc.outcome, got, tc.want)
			}
		})
	}
}

func TestHelpTextListsAllCommands(t *testing.T) {
	text := helpText("mybot")
	for _, want := range []string{"/mybot review", "/mybot abort", "/mybot status", "/mybot help", "/mybot mute", "/mybot respond", "resume"} {
		if !strings.Contains(text, want) {
			t.Fatalf("help text missing %q:\n%s", want, text)
		}
	}
}
