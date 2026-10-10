package chatgpt

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"maps"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeIssuer struct {
	t      *testing.T
	server *httptest.Server
	key    *rsa.PrivateKey

	mu            sync.Mutex
	nonce         string
	challenge     string
	tokenForms    []url.Values
	refreshCount  int
	subject       string
	expiresIn     int
	revoked       []string
	authorizeArgs url.Values
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIssuer{t: t, key: key, subject: "user-123", expiresIn: 3600}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeIssuer) issuer() string { return f.server.URL }

func (f *fakeIssuer) provider() *Provider {
	return &Provider{Issuer: f.issuer(), HTTPClient: f.server.Client()}
}

func (f *fakeIssuer) serve(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 f.issuer(),
			"authorization_endpoint": f.issuer() + "/authorize",
			"token_endpoint":         f.issuer() + "/token",
			"revocation_endpoint":    f.issuer() + "/revoke",
			"jwks_uri":               f.issuer() + "/jwks",
		})
	case "/jwks":
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{
			"kty": "RSA", "kid": "k1", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(f.key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(f.key.E)).Bytes()),
		}}})
	case "/token":
		_ = r.ParseForm()
		f.mu.Lock()
		f.tokenForms = append(f.tokenForms, r.PostForm)
		f.mu.Unlock()
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			verifier := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(verifier[:]) != f.challenge || r.PostForm.Get("code") != "the-code" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			f.writeTokens(w, r.PostForm.Get("client_id"), f.nonce, "refresh-0")
		case "refresh_token":
			f.mu.Lock()
			f.refreshCount++
			n := f.refreshCount
			f.mu.Unlock()
			if r.PostForm.Get("refresh_token") == "dead" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"refresh_token_reused"}`))
				return
			}
			f.writeTokens(w, r.PostForm.Get("client_id"), "", "refresh-"+string(rune('0'+n)))
		}
	case "/revoke":
		_ = r.ParseForm()
		f.mu.Lock()
		f.revoked = append(f.revoked, r.PostForm.Get("token"))
		f.mu.Unlock()
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeIssuer) writeTokens(w http.ResponseWriter, clientID, nonce, refresh string) {
	claims := map[string]any{
		"iss": f.issuer(), "sub": f.subject, "aud": clientID,
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "dev@example.com", "name": "Dev",
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token":  "access-" + refresh,
		"refresh_token": refresh,
		"id_token":      f.sign(claims),
		"token_type":    "Bearer",
		"expires_in":    f.expiresIn,
		"scope":         Scopes,
	})
}

func (f *fakeIssuer) sign(claims map[string]any) string {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	signing := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		f.t.Fatal(err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// browser plays the user: it follows the authorization URL straight back to
// the loopback callback with a code and a newly issued client ID.
func (f *fakeIssuer) browser(clientID string) func(string) error {
	return func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		q := u.Query()
		f.mu.Lock()
		f.nonce = q.Get("nonce")
		f.challenge = q.Get("code_challenge")
		f.authorizeArgs = q
		f.mu.Unlock()
		callback, _ := url.Parse(q.Get("redirect_uri"))
		cq := url.Values{"code": {"the-code"}, "state": {q.Get("state")}, "client_id": {clientID}}
		callback.RawQuery = cq.Encode()
		go func() {
			resp, err := http.Get(callback.String())
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}
}

func TestLoginRegistersVerifiesAndStoresCredentials(t *testing.T) {
	issuer := newFakeIssuer(t)
	var registered string
	creds, err := issuer.provider().Login(context.Background(), nil, "urn:uuid:host", LoginOptions{
		Port:           0,
		OpenBrowser:    issuer.browser("oaiapp_123"),
		OnRegistration: func(id string) error { registered = id; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if registered != "oaiapp_123" || creds.ClientID != "oaiapp_123" {
		t.Fatalf("registered = %q, client id = %q", registered, creds.ClientID)
	}
	if creds.Subject != "user-123" || creds.Email != "dev@example.com" || !creds.PlanUsage() || !creds.SignedIn() {
		t.Fatalf("credentials = %#v", creds)
	}
	args := issuer.authorizeArgs
	if args.Get("client_id") != DynamicClientID || args.Get("agent_name_hint") != AppName ||
		args.Get("ext_agent_host_id") != "urn:uuid:host" || args.Get("code_challenge_method") != "S256" ||
		args.Get("resource") != Resource || args.Get("scope") != Scopes {
		t.Fatalf("authorize args = %v", args)
	}
	if !strings.HasPrefix(args.Get("redirect_uri"), "http://127.0.0.1:") {
		t.Fatalf("redirect_uri = %q", args.Get("redirect_uri"))
	}
	exchange := issuer.tokenForms[0]
	if exchange.Get("client_id") != "oaiapp_123" || exchange.Get("resource") != Resource || exchange.Get("redirect_uri") != args.Get("redirect_uri") {
		t.Fatalf("token exchange = %v", exchange)
	}
}

func TestLoginReauthorizesWithSavedClientID(t *testing.T) {
	issuer := newFakeIssuer(t)
	previous := &Credentials{ClientID: "oaiapp_123", Subject: "user-123", Email: "dev@example.com"}
	_, err := issuer.provider().Login(context.Background(), previous, "urn:uuid:host", LoginOptions{
		Port:        0,
		OpenBrowser: issuer.browser(""),
	})
	if err != nil {
		t.Fatal(err)
	}
	args := issuer.authorizeArgs
	if args.Get("client_id") != "oaiapp_123" || args.Has("agent_name_hint") || args.Get("login_hint") != "dev@example.com" {
		t.Fatalf("authorize args = %v", args)
	}
}

func TestLoginRejectsDifferentAccount(t *testing.T) {
	issuer := newFakeIssuer(t)
	issuer.subject = "someone-else"
	previous := &Credentials{ClientID: "oaiapp_123", Subject: "user-123"}
	_, err := issuer.provider().Login(context.Background(), previous, "urn:uuid:host", LoginOptions{
		Port:        0,
		OpenBrowser: issuer.browser(""),
	})
	if err == nil || !strings.Contains(err.Error(), "account_mismatch") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifyIDTokenRejectsWrongAudienceAndNonce(t *testing.T) {
	issuer := newFakeIssuer(t)
	p := issuer.provider()
	base := map[string]any{"iss": issuer.issuer(), "sub": "s", "aud": "client", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": "n"}
	if _, err := p.verifyIDToken(context.Background(), issuer.sign(base), "client", "n", time.Now()); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if _, err := p.verifyIDToken(context.Background(), issuer.sign(base), "other", "n", time.Now()); err == nil {
		t.Fatal("wrong audience accepted")
	}
	if _, err := p.verifyIDToken(context.Background(), issuer.sign(base), "client", "x", time.Now()); err == nil {
		t.Fatal("wrong nonce accepted")
	}
	expired := maps.Clone(base)
	expired["exp"] = time.Now().Add(-time.Hour).Unix()
	if _, err := p.verifyIDToken(context.Background(), issuer.sign(expired), "client", "n", time.Now()); err == nil {
		t.Fatal("expired token accepted")
	}
	token := issuer.sign(base)
	tampered := token[:len(token)-4] + "AAAA"
	if _, err := p.verifyIDToken(context.Background(), tampered, "client", "n", time.Now()); err == nil {
		t.Fatal("tampered signature accepted")
	}
}

func TestSessionRefreshesExpiringTokenAndPersistsRotation(t *testing.T) {
	issuer := newFakeIssuer(t)
	store := &Store{Dir: t.TempDir()}
	store.AuthFile = filepath.Join(store.Dir, "auth.json")
	if err := store.Save(&Credentials{
		ClientID: "oaiapp_123", Subject: "user-123",
		AccessToken: "old-access", RefreshToken: "refresh-0", TokenType: "Bearer",
		ExpiresAt: time.Now().Add(30 * time.Second), Scopes: strings.Fields(Scopes),
	}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(store.AuthFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("auth file mode = %v", info.Mode().Perm())
	}

	session, err := NewSession(store, issuer.provider())
	if err != nil {
		t.Fatal(err)
	}
	token, err := session.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token != "access-refresh-1" {
		t.Fatalf("token = %q", token)
	}
	// A second token request reuses the fresh token.
	if token, _ = session.Token(context.Background()); token != "access-refresh-1" || issuer.refreshCount != 1 {
		t.Fatalf("token = %q refreshes = %d", token, issuer.refreshCount)
	}
	stored, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if stored.RefreshToken != "refresh-1" || stored.AccessToken != "access-refresh-1" {
		t.Fatalf("stored = %#v", stored)
	}

	// A second process sees the rotated tokens instead of refreshing again.
	other, err := NewSession(store, issuer.provider())
	if err != nil {
		t.Fatal(err)
	}
	if token, err := other.Token(context.Background()); err != nil || token != "access-refresh-1" {
		t.Fatalf("other token = %q, %v", token, err)
	}
	if issuer.refreshCount != 1 {
		t.Fatalf("refreshes = %d", issuer.refreshCount)
	}
}

func TestSessionClearsTokensOnTerminalRefreshError(t *testing.T) {
	issuer := newFakeIssuer(t)
	store := &Store{Dir: t.TempDir()}
	store.AuthFile = filepath.Join(store.Dir, "auth.json")
	if err := store.Save(&Credentials{
		ClientID: "oaiapp_123", AccessToken: "a", RefreshToken: "dead", TokenType: "Bearer",
		ExpiresAt: time.Now().Add(-time.Minute), Scopes: strings.Fields(Scopes),
	}); err != nil {
		t.Fatal(err)
	}
	session, err := NewSession(store, issuer.provider())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Token(context.Background()); err == nil || !RequiresSignIn(err) {
		t.Fatalf("error = %v", err)
	}
	stored, _ := store.Load()
	if stored.SignedIn() || stored.ClientID != "oaiapp_123" {
		t.Fatalf("stored = %#v", stored)
	}
}

func TestNewSessionRequiresPlanUsageScope(t *testing.T) {
	store := &Store{Dir: t.TempDir()}
	store.AuthFile = filepath.Join(store.Dir, "auth.json")
	if err := store.Save(&Credentials{ClientID: "c", AccessToken: "a", RefreshToken: "r", Scopes: []string{"openid"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSession(store, NewProvider()); err == nil || !strings.Contains(err.Error(), "--reconsent") {
		t.Fatalf("error = %v", err)
	}
	empty := &Store{Dir: t.TempDir()}
	empty.AuthFile = filepath.Join(empty.Dir, "auth.json")
	if _, err := NewSession(empty, NewProvider()); err != ErrNotSignedIn {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreHostIDIsStable(t *testing.T) {
	store := &Store{Dir: t.TempDir()}
	first, err := store.HostID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.HostID()
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !strings.HasPrefix(first, "urn:uuid:") {
		t.Fatalf("host ids = %q, %q", first, second)
	}
}
