package serve

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/testutil"
)

// newSigningHandler builds a handler whose single group ("platform") verifies
// webhooks via a signing token derived from signingKey.
func newSigningHandler(t *testing.T) *Handler {
	t.Helper()
	token := "whsec_" + base64.StdEncoding.EncodeToString(signingKey)
	set, warnings := NewGroupSet(context.Background(), []config.ServeGroup{
		{Path: "platform", Token: "t1", SigningToken: token},
	}, "https://gitlab.example.com", nil)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	dispatcher := NewDispatcher(&fakeRunner{}, (&countingTopicLookup{}).fn(), nil, WorkerConfig{Topic: "nickpit"}, discardLogger())
	return NewHandler(set, dispatcher, HandlerConfig{TriggerEmoji: "nickpit", CommandKeyword: "nickpit"}, nil, ChatConfig{}, discardLogger())
}

func postSignedWebhook(t *testing.T, handler *Handler, body, key []byte) *httptest.ResponseRecorder {
	t.Helper()
	id := "msg_1"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab", bytes.NewReader(body))
	req.Header.Set("Webhook-Id", id)
	req.Header.Set("Webhook-Timestamp", ts)
	req.Header.Set("Webhook-Signature", signHeader(key, id, ts, body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestHandlerSigningTokenAccepts(t *testing.T) {
	handler := newSigningHandler(t)
	body := testutil.LoadFixture(t, filepath.Join("testdata", "mr_open.json"))
	rec := postSignedWebhook(t, handler, body, signingKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerSigningTokenRejectsBadSignature(t *testing.T) {
	handler := newSigningHandler(t)
	body := testutil.LoadFixture(t, filepath.Join("testdata", "mr_open.json"))
	rec := postSignedWebhook(t, handler, body, []byte("wrong-key"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}

// A group built (defensively, bypassing config validation) from an
// unparseable signing token with no webhook_secret fallback has no usable
// credential and must reject every request — in particular one without any
// X-Gitlab-Token header, which previously slipped through via "" == "".
func TestHandlerUnparseableSigningTokenFailsClosed(t *testing.T) {
	set, warnings := NewGroupSet(context.Background(), []config.ServeGroup{
		{Path: "platform", Token: "t1", SigningToken: "whsec_not!!base64"},
	}, "https://gitlab.example.com", nil)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want the unparseable-token warning", warnings)
	}
	dispatcher := NewDispatcher(&fakeRunner{}, (&countingTopicLookup{}).fn(), nil, WorkerConfig{Topic: "nickpit"}, discardLogger())
	handler := NewHandler(set, dispatcher, HandlerConfig{TriggerEmoji: "nickpit", CommandKeyword: "nickpit"}, nil, ChatConfig{}, discardLogger())

	body := testutil.LoadFixture(t, filepath.Join("testdata", "mr_open.json"))
	for name, header := range map[string]string{"no token": "", "empty-ish token": "anything"} {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab", bytes.NewReader(body))
		if header != "" {
			req.Header.Set("X-Gitlab-Token", header)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: code = %d, want 401 (group without usable credential must fail closed)", name, rec.Code)
		}
	}
}

// A signing group must reject the legacy X-Gitlab-Token path entirely.
func TestHandlerSigningTokenIgnoresPlaintextToken(t *testing.T) {
	handler := newSigningHandler(t)
	body := testutil.LoadFixture(t, filepath.Join("testdata", "mr_open.json"))
	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab", bytes.NewReader(body))
	req.Header.Set("X-Gitlab-Token", "whsec_"+base64.StdEncoding.EncodeToString(signingKey))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401 (plaintext token must not satisfy a signing group)", rec.Code)
	}
}
