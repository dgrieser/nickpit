package serve

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/testutil"
)

// noteFixture loads a note webhook fixture with its note text replaced, and
// with the note type replaced unless noteType is "keep".
func noteFixture(t *testing.T, fixture, note, noteType string) []byte {
	t.Helper()
	var event map[string]any
	if err := json.Unmarshal(testutil.LoadFixture(t, filepath.Join("testdata", fixture)), &event); err != nil {
		t.Fatal(err)
	}
	attrs := event["object_attributes"].(map[string]any)
	attrs["note"] = note
	if noteType != "keep" {
		attrs["type"] = noteType
	}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestCommandHasExtraText(t *testing.T) {
	for body, want := range map[string]bool{
		"/nickpit review":                         false,
		"\n  /nickpit review  \n\n":               false,
		"/nickpit review check the comments":      true,
		"/nickpit review\nplease look at auth.go": true,
		"/nickpit status\n\n":                     false,
	} {
		if got := commandHasExtraText(body); got != want {
			t.Errorf("commandHasExtraText(%q) = %v, want %v", body, got, want)
		}
	}
}

func TestMentionsUser(t *testing.T) {
	for body, want := range map[string]bool{
		"@nickpit-bot check this":        true,
		"hey @NickPit-Bot.":              true,
		"(@nickpit-bot)":                 true,
		"@nickpit-bot2 check this":       false,
		"@nickpit-bot.other check this":  false,
		"mail@nickpit-bot":               false,
		"no mention here":                false,
		"see @nickpit-bot, @nickpit-bot": true,
	} {
		if got := mentionsUser(body, "nickpit-bot"); got != want {
			t.Errorf("mentionsUser(%q) = %v, want %v", body, got, want)
		}
	}
	if mentionsUser("@anyone", "") {
		t.Error("empty username must never match")
	}
}

func TestDecideCommandMarksIgnoredText(t *testing.T) {
	var event WebhookEvent
	if err := json.Unmarshal(noteFixture(t, "note_command_review.json", "/nickpit review check these comments against the latest commit", "keep"), &event); err != nil {
		t.Fatal(err)
	}
	decision := Decide(&event, "nickpit", "", "nickpit", nil, nil)
	if decision.Command != CommandReview || !decision.IgnoredText {
		t.Fatalf("decision = %+v, want review with ignored text", decision)
	}
}

func TestHandlerReviewCommandWithExtraTextReplies(t *testing.T) {
	env := newHandlerEnv(t)
	postWebhookBody(t, env.handler, noteFixture(t, "note_command_review.json", "/nickpit review check these comments", "keep"), "legacy-secret")
	waitFor(t, 3*time.Second, func() bool {
		for _, post := range env.gitlab.posted() {
			if strings.Contains(post.Body["body"], "Text after `/nickpit review` is ignored") {
				return true
			}
		}
		return false
	})
	if queuedJobs(env.dispatcher) != 1 {
		t.Fatalf("queued = %d, want the review still queued", queuedJobs(env.dispatcher))
	}
}

func TestHandlerStatusCommandWithExtraTextPrefixesNotice(t *testing.T) {
	env := newHandlerEnv(t)
	postWebhookBody(t, env.handler, noteFixture(t, "note_command_status.json", "/nickpit status please", "keep"), "legacy-secret")
	waitFor(t, 3*time.Second, func() bool { return len(env.gitlab.posted()) == 1 })
	body := env.gitlab.posted()[0].Body["body"]
	if !strings.HasPrefix(body, "Text after `/nickpit status` is ignored") || !strings.Contains(body, "No review is queued or running") {
		t.Fatalf("reply = %q", body)
	}
}

func TestHandlerTopLevelMentionGetsHelp(t *testing.T) {
	env := newHandlerEnv(t)
	postWebhookBody(t, env.handler, noteFixture(t, "note_plain.json", "@nickpit-bot can you look at this?", ""), "legacy-secret")
	waitFor(t, 3*time.Second, func() bool { return len(env.gitlab.posted()) == 1 })
	if body := env.gitlab.posted()[0].Body["body"]; !strings.Contains(body, "answers only in its own review threads") {
		t.Fatalf("reply = %q", body)
	}
	// A redelivery of the same note is not answered twice.
	postWebhookBody(t, env.handler, noteFixture(t, "note_plain.json", "@nickpit-bot can you look at this?", ""), "legacy-secret")
	time.Sleep(100 * time.Millisecond)
	if n := len(env.gitlab.posted()); n != 1 {
		t.Fatalf("posts = %d, want 1", n)
	}
}

func TestHandlerTopLevelCommentWithoutMentionStaysSilent(t *testing.T) {
	env := newHandlerEnv(t)
	postWebhookBody(t, env.handler, noteFixture(t, "note_plain.json", "looks good to me", ""), "legacy-secret")
	time.Sleep(100 * time.Millisecond)
	if posts := env.gitlab.posted(); len(posts) != 0 {
		t.Fatalf("posts = %v, want silence", posts)
	}
}

func TestHandlerForeignThreadMentionGetsHelp(t *testing.T) {
	env := newHandlerEnv(t)
	env.group.BotUserID = fakeBotUserID
	env.gitlab.discussionRoot = "just a human thread"
	postWebhookBody(t, env.handler, noteFixture(t, "note_plain.json", "@nickpit-bot check these comments", "keep"), "legacy-secret")
	waitFor(t, 3*time.Second, func() bool {
		for _, post := range env.gitlab.posted() {
			if strings.Contains(post.Body["body"], "answers only in its own review threads") {
				return true
			}
		}
		return false
	})
	select {
	case <-env.chat.calls:
		t.Fatal("chat child spawned for a foreign thread")
	default:
	}
}
