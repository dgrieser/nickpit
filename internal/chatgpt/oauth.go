package chatgpt

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	// Issuer is the ChatGPT OAuth issuer; endpoints come from its discovery
	// document and must stay on its origin.
	Issuer = "https://auth.openai.com"
	// Resource is the API the tokens are minted for.
	Resource = "https://api.openai.com/v1"
	// ScopePlanUsage is the scope that lets requests draw on the ChatGPT plan.
	ScopePlanUsage = "chatgpt.tokens.use.direct"
	// Scopes requests identity, a refresh token, and plan usage.
	Scopes = "openid profile email offline_access resource.invoke " + ScopePlanUsage
	// DynamicClientID registers a new client on the first sign-in; later
	// sign-ins reuse the client ID that registration issued.
	DynamicClientID = "dynamic_agent_client"
	// AppName is the agent name shown on the ChatGPT consent screen.
	AppName = "NickPit"
	// DefaultCallbackPort is the loopback port of the sign-in callback.
	DefaultCallbackPort = 1455
	callbackPath        = "/auth/callback"
)

var clientIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,200}$`)

// Error is a ChatGPT OAuth or API failure carrying the server's
// machine-readable code.
type Error struct {
	Code    string
	Message string
	Status  int
}

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" {
		msg = "request failed"
	}
	if e.Code != "" {
		return fmt.Sprintf("chatgpt: %s (%s)", msg, e.Code)
	}
	return "chatgpt: " + msg
}

// RequiresSignIn reports whether err means the stored refresh token can no
// longer be used and the user has to run the sign-in again.
func RequiresSignIn(err error) bool {
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		return errors.Is(err, ErrNotSignedIn)
	}
	switch apiErr.Code {
	case "invalid_grant", "invalid_refresh_token", "token_expired", "refresh_token_expired",
		"refresh_token_invalidated", "refresh_token_reused", "account_mismatch":
		return true
	}
	return false
}

// ErrNotSignedIn is returned when no usable credentials are stored.
var ErrNotSignedIn = errors.New("chatgpt: not signed in; run `nickpit chatgpt login`")

// Provider talks to the ChatGPT OAuth issuer.
type Provider struct {
	Issuer     string
	HTTPClient *http.Client

	discovery *discoveryDoc
	keys      map[string]*rsa.PublicKey
}

// NewProvider returns a provider for the production issuer.
func NewProvider() *Provider {
	return &Provider{Issuer: Issuer, HTTPClient: noRedirectClient(30 * time.Second)}
}

func noRedirectClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type discoveryDoc struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	RevocationEndpoint    string `json:"revocation_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

func (p *Provider) discover(ctx context.Context) (*discoveryDoc, error) {
	if p.discovery != nil {
		return p.discovery, nil
	}
	var doc discoveryDoc
	if err := p.getJSON(ctx, strings.TrimRight(p.Issuer, "/")+"/.well-known/openid-configuration", "", &doc); err != nil {
		return nil, fmt.Errorf("chatgpt: sign-in configuration could not be loaded: %w", err)
	}
	if doc.Issuer != p.Issuer {
		return nil, fmt.Errorf("chatgpt: sign-in configuration names issuer %q, want %q", doc.Issuer, p.Issuer)
	}
	for name, endpoint := range map[string]string{
		"authorization_endpoint": doc.AuthorizationEndpoint,
		"token_endpoint":         doc.TokenEndpoint,
		"jwks_uri":               doc.JWKSURI,
		"revocation_endpoint":    doc.RevocationEndpoint,
	} {
		if endpoint == "" && name == "revocation_endpoint" {
			continue
		}
		if !sameOrigin(endpoint, p.Issuer) {
			return nil, fmt.Errorf("chatgpt: sign-in configuration %s %q is not on the issuer origin", name, endpoint)
		}
	}
	p.discovery = &doc
	return &doc, nil
}

func sameOrigin(raw, issuer string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	want, err := url.Parse(issuer)
	if err != nil {
		return false
	}
	return u.Scheme == want.Scheme && u.Host == want.Host
}

func (p *Provider) getJSON(ctx context.Context, endpoint, bearer string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return apiError(data, resp.StatusCode)
	}
	return json.Unmarshal(data, out)
}

func apiError(body []byte, status int) *Error {
	out := &Error{Status: status, Message: http.StatusText(status)}
	var envelope map[string]any
	if json.Unmarshal(body, &envelope) != nil {
		return out
	}
	detail := envelope
	for range 4 {
		if nested, ok := detail["error"].(map[string]any); ok {
			detail = nested
		} else if nested, ok := detail["detail"].(map[string]any); ok {
			detail = nested
		} else {
			break
		}
	}
	if code, ok := detail["error"].(string); ok {
		out.Code = code
	} else if code, ok := detail["code"].(string); ok {
		out.Code = code
	}
	for _, key := range []string{"error_description", "message", "detail"} {
		if message, ok := detail[key].(string); ok && message != "" {
			out.Message = message
			break
		}
	}
	return out
}

type tokenResponse struct {
	AccessToken       string          `json:"access_token"`
	RefreshToken      string          `json:"refresh_token"`
	IDToken           string          `json:"id_token"`
	TokenType         string          `json:"token_type"`
	ExpiresIn         float64         `json:"expires_in"`
	Scope             *string         `json:"scope"`
	EarliestRefreshAt json.RawMessage `json:"earliest_refresh_at"`
}

func (p *Provider) tokenRequest(ctx context.Context, form url.Values) (*tokenResponse, error) {
	doc, err := p.discover(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, doc.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chatgpt: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("chatgpt: token request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apiError(data, resp.StatusCode)
	}
	var tok tokenResponse
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, &Error{Code: "invalid_token_response", Message: "the token response could not be parsed"}
	}
	return &tok, nil
}

// applyTokens validates a token response and copies it onto creds.
func applyTokens(creds *Credentials, tok *tokenResponse, previousScopes []string) error {
	scopes := previousScopes
	if tok.Scope != nil {
		scopes = strings.Fields(*tok.Scope)
	}
	if len(scopes) == 0 {
		return &Error{Code: "invalid_token_response", Message: "ChatGPT did not confirm the granted permissions"}
	}
	if tok.AccessToken == "" || !strings.EqualFold(tok.TokenType, "bearer") || tok.ExpiresIn <= 0 || tok.RefreshToken == "" {
		return &Error{Code: "invalid_token_response", Message: "ChatGPT returned incomplete credentials"}
	}
	creds.AccessToken = tok.AccessToken
	creds.RefreshToken = tok.RefreshToken
	creds.TokenType = tok.TokenType
	creds.ExpiresAt = time.Now().Add(time.Duration(tok.ExpiresIn * float64(time.Second))).UTC()
	creds.EarliestRefreshAt = parseEarliestRefresh(tok.EarliestRefreshAt)
	creds.Scopes = scopes
	return nil
}

func parseEarliestRefresh(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Time{}
	}
	var seconds float64
	if json.Unmarshal(raw, &seconds) == nil && seconds > 0 {
		return time.Unix(int64(seconds), 0).UTC()
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		if parsed, err := time.Parse(time.RFC3339, text); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

// LoginOptions configures one interactive sign-in.
type LoginOptions struct {
	// Port of the 127.0.0.1 callback listener; 0 picks a free port. The
	// registration accepts any loopback port, so only the port may vary.
	Port int
	// OpenBrowser opens the authorization URL; when nil or failing, the URL is
	// only printed through ShowURL.
	OpenBrowser func(url string) error
	// ShowURL receives the authorization URL before the browser opens.
	ShowURL func(url string)
	// Reconsent asks ChatGPT to show the consent screen again, to grant plan
	// usage after it was declined.
	Reconsent bool
	// OnRegistration persists the issued client ID before the code exchange,
	// so a failed exchange never registers a second client.
	OnRegistration func(clientID string) error
}

// Login runs the browser sign-in and returns verified credentials. previous
// is the stored record, whose client ID and account are reused; it may be nil.
func (p *Provider) Login(ctx context.Context, previous *Credentials, hostID string, opts LoginOptions) (*Credentials, error) {
	doc, err := p.discover(ctx)
	if err != nil {
		return nil, err
	}
	state, nonce, verifier := randomValue(), randomValue(), randomValue()
	challenge := sha256.Sum256([]byte(verifier))

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", opts.Port))
	if err != nil {
		return nil, fmt.Errorf("chatgpt: callback port %d is unavailable (another sign-in running? try --port 0): %w", opts.Port, err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d%s", port, callbackPath)

	savedClientID := ""
	if previous != nil {
		savedClientID = previous.ClientID
	}
	clientID := savedClientID
	if clientID == "" {
		clientID = DynamicClientID
	}
	query := url.Values{
		"client_id":             {clientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {Scopes},
		"resource":              {Resource},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge_method": {"S256"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"ext_agent_host_id":     {hostID},
	}
	if savedClientID == "" {
		query.Set("agent_name_hint", AppName)
	}
	if previous != nil && previous.Email != "" {
		query.Set("login_hint", previous.Email)
	}
	if opts.Reconsent {
		query.Set("prompt", "consent")
	}
	authURL := doc.AuthorizationEndpoint + "?" + query.Encode()

	results := make(chan callbackResult, 1)
	server := &http.Server{
		Handler:           callbackHandler(port, state, savedClientID, results),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if opts.ShowURL != nil {
		opts.ShowURL(authURL)
	}
	if opts.OpenBrowser != nil {
		_ = opts.OpenBrowser(authURL)
	}

	var callback callbackResult
	select {
	case callback = <-results:
	case <-ctx.Done():
		return nil, fmt.Errorf("chatgpt: sign-in cancelled: %w", ctx.Err())
	}
	if callback.err != nil {
		return nil, callback.err
	}
	if opts.OnRegistration != nil {
		if err := opts.OnRegistration(callback.clientID); err != nil {
			return nil, err
		}
	}

	tok, err := p.tokenRequest(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {callback.clientID},
		"code":          {callback.code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
		"resource":      {Resource},
	})
	if err != nil {
		return nil, err
	}
	if tok.IDToken == "" {
		return nil, &Error{Code: "invalid_id_token", Message: "ChatGPT did not return a verifiable identity"}
	}
	claims, err := p.verifyIDToken(ctx, tok.IDToken, callback.clientID, nonce, time.Now())
	if err != nil {
		return nil, err
	}
	if previous != nil && previous.Subject != "" && previous.ClientID == callback.clientID && claims.Subject != previous.Subject {
		return nil, &Error{Code: "account_mismatch", Message: "this sign-in returned a different ChatGPT account; run `nickpit chatgpt logout --forget` first to switch accounts"}
	}
	creds := &Credentials{
		ClientID: callback.clientID,
		Issuer:   p.Issuer,
		Subject:  claims.Subject,
		Email:    claims.Email,
		Name:     claims.Name,
		IDToken:  tok.IDToken,
	}
	if err := applyTokens(creds, tok, nil); err != nil {
		return nil, err
	}
	return creds, nil
}

type callbackResult struct {
	code     string
	clientID string
	err      error
}

func callbackHandler(port int, state, savedClientID string, results chan<- callbackResult) http.Handler {
	host := fmt.Sprintf("127.0.0.1:%d", port)
	var mu sync.Mutex
	done := false
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'")
		mu.Lock()
		defer mu.Unlock()
		if r.Method != http.MethodGet || r.Host != host || r.URL.Path != callbackPath || done {
			http.NotFound(w, r)
			return
		}
		query := r.URL.Query()
		got := query.Get("state")
		if len(query["state"]) != 1 || subtle.ConstantTimeCompare([]byte(got), []byte(state)) != 1 {
			// Unrelated loopback requests must not consume the pending sign-in.
			http.Error(w, "Invalid sign-in state. Return to the browser tab that started sign-in.", http.StatusBadRequest)
			return
		}
		done = true
		result := callbackResult{}
		if errCode := query.Get("error"); errCode != "" {
			result.err = &Error{Code: errCode, Message: "sign-in was not completed"}
			if errCode == "access_denied" {
				result.err = &Error{Code: errCode, Message: "sign-in was declined in the browser"}
			}
		} else {
			code := query.Get("code")
			returned := query.Get("client_id")
			clientID := returned
			if clientID == "" {
				clientID = savedClientID
			}
			switch {
			case code == "" || len(query["code"]) != 1 || len(query["client_id"]) > 1:
				result.err = &Error{Code: "registration_incomplete", Message: "ChatGPT did not complete the sign-in; try again"}
			case !clientIDPattern.MatchString(clientID) || clientID == DynamicClientID:
				result.err = &Error{Code: "registration_incomplete", Message: "ChatGPT did not complete app registration; try again"}
			case savedClientID != "" && returned != "" && returned != savedClientID:
				result.err = &Error{Code: "registration_mismatch", Message: "ChatGPT returned a different client registration"}
			default:
				result.code, result.clientID = code, clientID
			}
		}
		title, body := "Signed in", "NickPit is finishing the ChatGPT sign-in. You can close this tab and return to the terminal."
		if result.err != nil {
			title, body = "Sign-in failed", result.err.Error()
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<!doctype html><html lang="en"><meta charset="utf-8"><title>%s</title><style>body{font:17px system-ui;max-width:32rem;margin:18vh auto;padding:24px}</style><h1>%s</h1><p>%s</p></html>`,
			html.EscapeString(title), html.EscapeString(title), html.EscapeString(body))
		results <- result
	})
}

// Refresh renews creds with its refresh token. The refresh token rotates, so
// the caller must persist the result before anything else can use the old one.
func (p *Provider) Refresh(ctx context.Context, creds *Credentials) (*Credentials, error) {
	if creds == nil || creds.RefreshToken == "" {
		return nil, ErrNotSignedIn
	}
	tok, err := p.tokenRequest(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {creds.ClientID},
		"refresh_token": {creds.RefreshToken},
		"resource":      {Resource},
	})
	if err != nil {
		return nil, err
	}
	next := *creds
	next.Scopes = append([]string(nil), creds.Scopes...)
	if err := applyTokens(&next, tok, creds.Scopes); err != nil {
		return nil, err
	}
	if tok.IDToken != "" {
		claims, err := p.verifyIDToken(ctx, tok.IDToken, creds.ClientID, "", time.Now())
		if err != nil {
			return nil, err
		}
		if creds.Subject != "" && claims.Subject != creds.Subject {
			return nil, &Error{Code: "account_mismatch", Message: "the refreshed identity does not match the signed-in account"}
		}
		next.IDToken = tok.IDToken
		next.Subject = claims.Subject
		if claims.Email != "" {
			next.Email = claims.Email
		}
		if claims.Name != "" {
			next.Name = claims.Name
		}
	}
	return &next, nil
}

// Revoke ends the remote session of creds' refresh token.
func (p *Provider) Revoke(ctx context.Context, creds *Credentials) error {
	if creds == nil || creds.RefreshToken == "" {
		return nil
	}
	doc, err := p.discover(ctx)
	if err != nil {
		return err
	}
	if doc.RevocationEndpoint == "" {
		return errors.New("chatgpt: the issuer offers no revocation endpoint; disconnect NickPit in ChatGPT settings")
	}
	form := url.Values{"token": {creds.RefreshToken}, "token_type_hint": {"refresh_token"}, "client_id": {creds.ClientID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, doc.RevocationEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("chatgpt: revoking session: %w", err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("chatgpt: revoking session: %s", resp.Status)
	}
	return nil
}

type idClaims struct {
	Subject string
	Email   string
	Name    string
}

// verifyIDToken checks the RS256 signature against the issuer's JWKS and the
// issuer, audience, expiry, and nonce claims. now is when the token was
// received.
func (p *Provider) verifyIDToken(ctx context.Context, token, clientID, nonce string, now time.Time) (*idClaims, error) {
	invalid := &Error{Code: "invalid_id_token", Message: "the ChatGPT identity could not be verified; sign in again"}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, invalid
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &header); err != nil || header.Alg != "RS256" {
		return nil, invalid
	}
	key, err := p.signingKey(ctx, header.Kid)
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, invalid
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
		return nil, invalid
	}

	var claims struct {
		Issuer   string          `json:"iss"`
		Subject  string          `json:"sub"`
		Audience json.RawMessage `json:"aud"`
		Azp      *string         `json:"azp"`
		Expiry   float64         `json:"exp"`
		IssuedAt float64         `json:"iat"`
		Nonce    string          `json:"nonce"`
		Email    string          `json:"email"`
		Name     string          `json:"name"`
	}
	if err := decodeSegment(parts[1], &claims); err != nil {
		return nil, invalid
	}
	var audiences []string
	var single string
	if json.Unmarshal(claims.Audience, &single) == nil {
		audiences = []string{single}
	} else if json.Unmarshal(claims.Audience, &audiences) != nil {
		return nil, invalid
	}
	const skew = 5 * time.Second
	expiry := time.Unix(int64(claims.Expiry), 0)
	switch {
	case claims.Issuer != p.Issuer, claims.Subject == "", claims.IssuedAt == 0, claims.Expiry == 0:
		return nil, invalid
	case !slices.Contains(audiences, clientID):
		return nil, invalid
	case claims.Azp != nil && *claims.Azp != clientID:
		return nil, invalid
	case len(audiences) > 1 && claims.Azp == nil:
		return nil, invalid
	case now.After(expiry.Add(skew)):
		return nil, invalid
	case nonce != "" && claims.Nonce != nonce:
		return nil, invalid
	}
	return &idClaims{Subject: claims.Subject, Email: claims.Email, Name: claims.Name}, nil
}

func (p *Provider) signingKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if key, ok := p.keys[kid]; ok {
		return key, nil
	}
	doc, err := p.discover(ctx)
	if err != nil {
		return nil, err
	}
	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := p.getJSON(ctx, doc.JWKSURI, "", &jwks); err != nil {
		return nil, fmt.Errorf("chatgpt: identity verification is temporarily unavailable: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" || (jwk.Use != "" && jwk.Use != "sig") {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(jwk.N)
		e, errE := base64.RawURLEncoding.DecodeString(jwk.E)
		if errN != nil || errE != nil || len(e) == 0 || len(e) > 4 {
			continue
		}
		keys[jwk.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	p.keys = keys
	key, ok := keys[kid]
	if !ok {
		return nil, &Error{Code: "invalid_id_token", Message: "the ChatGPT identity is signed with an unknown key"}
	}
	return key, nil
}

func decodeSegment(segment string, out any) error {
	data, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func randomValue() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}
