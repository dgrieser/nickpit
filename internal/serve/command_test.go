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
	for _, want := range []string{"/mybot review", "/mybot abort", "/mybot status", "/mybot help"} {
		if !strings.Contains(text, want) {
			t.Fatalf("help text missing %q:\n%s", want, text)
		}
	}
}
