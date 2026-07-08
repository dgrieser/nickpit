package serve

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
)

// signingKey is the shared HMAC key used across signature tests.
var signingKey = []byte("standard-webhooks-secret-key")

// newSigningGroup builds a one-group set verifying via a signing token derived
// from signingKey and returns the matched group.
func newSigningGroup(t *testing.T) *Group {
	t.Helper()
	token := "whsec_" + base64.StdEncoding.EncodeToString(signingKey)
	set := newTestGroupSet(t, []config.ServeGroup{
		{Path: "platform", Token: "t", SigningToken: token},
	})
	g := set.Match("platform/api")
	if g == nil {
		t.Fatal("group not matched")
	}
	if !g.UsesSigning() {
		t.Fatal("group should use signing")
	}
	return g
}

// signHeader produces the webhook-signature header value for the given key.
func signHeader(key []byte, id, ts string, body []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "."))
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestGroupUsesSigningFalseForSecret(t *testing.T) {
	set := newTestGroupSet(t, []config.ServeGroup{
		{Path: "platform", Token: "t", WebhookSecret: "s"},
	})
	if set.Match("platform/api").UsesSigning() {
		t.Fatal("secret-token group must not report UsesSigning")
	}
}

func TestCheckSignature(t *testing.T) {
	g := newSigningGroup(t)
	now := time.Unix(1_700_000_000, 0)
	ts := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{"object_kind":"merge_request"}`)
	id := "msg_abc"
	valid := signHeader(signingKey, id, ts, body)

	cases := []struct {
		name           string
		id, ts, header string
		body           []byte
		now            time.Time
		want           bool
	}{
		{"valid", id, ts, valid, body, now, true},
		{"tampered body", id, ts, valid, []byte(`{"object_kind":"push"}`), now, false},
		{"wrong key", id, ts, signHeader([]byte("other"), id, ts, body), body, now, false},
		{"wrong id", "msg_other", ts, valid, body, now, false},
		{"missing headers", "", "", "", body, now, false},
		{"non-v1 version", id, ts, "v2," + valid[3:], body, now, false},
		{"stale timestamp", id, ts, valid, body, now.Add(10 * time.Minute), false},
		{"future timestamp", id, ts, valid, body, now.Add(-10 * time.Minute), false},
		{"within tolerance", id, ts, valid, body, now.Add(4 * time.Minute), true},
		{"non-numeric timestamp", id, "abc", valid, body, now, false},
		{"multiple sigs one valid", id, ts, "v1,deadbeef " + valid, body, now, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.CheckSignature(tc.id, tc.ts, tc.header, tc.body, tc.now); got != tc.want {
				t.Fatalf("CheckSignature = %v, want %v", got, tc.want)
			}
		})
	}
}
