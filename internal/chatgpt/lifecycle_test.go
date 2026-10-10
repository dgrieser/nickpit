package chatgpt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const helperEnv = "NICKPIT_TEST_CHATGPT_HELPER"

// expiringStore writes credentials whose access token is about to expire, so
// the next Token call refreshes.
func expiringStore(t *testing.T, refreshToken string) *Store {
	t.Helper()
	store := &Store{Dir: t.TempDir()}
	store.AuthFile = filepath.Join(store.Dir, "auth.json")
	if err := store.Save(&Credentials{
		ClientID: "oaiapp_123", Subject: "user-123", Email: "dev@example.com",
		AccessToken: "old-access", RefreshToken: refreshToken, TokenType: "Bearer",
		ExpiresAt: time.Now().Add(30 * time.Second), Scopes: strings.Fields(Scopes),
	}); err != nil {
		t.Fatal(err)
	}
	return store
}

// TestHelperRefreshProcess is a child process of
// TestConcurrentProcessesRefreshOnce, not a test of its own.
func TestHelperRefreshProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "refresh" {
		t.Skip("helper process")
	}
	store := &Store{Dir: os.Getenv("HELPER_DIR"), AuthFile: os.Getenv("HELPER_AUTH_FILE")}
	session, err := NewSession(store, &Provider{Issuer: os.Getenv("HELPER_ISSUER"), HTTPClient: http.DefaultClient})
	if err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
	token, err := session.Token(context.Background())
	if err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
	fmt.Println("TOKEN", token)
}

// Separate processes share one rotating refresh token. The credential lock
// must let exactly one of them spend it; the others pick up what it stored.
// The issuer refuses a reused refresh token, so a missing lock fails here.
func TestConcurrentProcessesRefreshOnce(t *testing.T) {
	issuer := newFakeIssuer(t)
	issuer.refreshDelay = 200 * time.Millisecond
	store := expiringStore(t, "refresh-0")

	const processes = 4
	commands := make([]*exec.Cmd, processes)
	outputs := make([]*bytes.Buffer, processes)
	for i := range commands {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHelperRefreshProcess$", "-test.count=1")
		cmd.Env = append(os.Environ(), helperEnv+"=refresh", "HELPER_DIR="+store.Dir, "HELPER_AUTH_FILE="+store.AuthFile, "HELPER_ISSUER="+issuer.issuer())
		outputs[i] = &bytes.Buffer{}
		cmd.Stdout, cmd.Stderr = outputs[i], outputs[i]
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands[i] = cmd
	}
	tokens := map[string]bool{}
	for i, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("process %d failed: %v\n%s", i, err, outputs[i])
		}
		for line := range strings.SplitSeq(outputs[i].String(), "\n") {
			if token, ok := strings.CutPrefix(line, "TOKEN "); ok {
				tokens[token] = true
			}
		}
	}
	if len(tokens) != 1 || !tokens["access-refresh-1"] {
		t.Fatalf("tokens = %v, want every process on access-refresh-1", tokens)
	}
	if issuer.refreshCount != 1 {
		t.Fatalf("refreshes = %d, want 1", issuer.refreshCount)
	}
	stored, err := store.Load()
	if err != nil || stored.RefreshToken != "refresh-1" {
		t.Fatalf("stored = %+v, %v", stored, err)
	}
}

// A logout issued while a refresh is in flight waits for it, then revokes the
// token the refresh rotated in, so the refresh cannot bring the sign-in back.
func TestLogoutWaitsForRunningRefresh(t *testing.T) {
	issuer := newFakeIssuer(t)
	release := issuer.gateRefreshes(t)
	store := expiringStore(t, "refresh-0")
	session, err := NewSession(store, issuer.provider())
	if err != nil {
		t.Fatal(err)
	}

	refreshed := make(chan error, 1)
	go func() {
		_, err := session.Token(context.Background())
		refreshed <- err
	}()
	<-issuer.refreshStarted

	loggedOut := make(chan error, 1)
	go func() {
		_, err := Logout(context.Background(), store, issuer.provider(), false)
		loggedOut <- err
	}()
	select {
	case err := <-loggedOut:
		t.Fatalf("logout finished while a refresh held the credentials: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	release()
	if err := <-refreshed; err != nil {
		t.Fatal(err)
	}
	if err := <-loggedOut; err != nil {
		t.Fatal(err)
	}

	if len(issuer.revoked) != 1 || issuer.revoked[0] != "refresh-1" {
		t.Fatalf("revoked = %v, want the rotated refresh-1", issuer.revoked)
	}
	stored, err := store.Load()
	if err != nil || stored.SignedIn() || stored.ClientID != "oaiapp_123" {
		t.Fatalf("stored after logout = %+v, %v", stored, err)
	}
	// The session in memory still holds a token; renewing it finds the
	// account signed out instead of restoring it.
	if _, err := session.ForceRefresh(context.Background()); !errors.Is(err, ErrNotSignedIn) {
		t.Fatalf("refresh after logout = %v, want ErrNotSignedIn", err)
	}
}

// A refresh token that turns out to be spent clears only that session: when
// the file meanwhile holds newer credentials, they stay and are used.
func TestTerminalRefreshErrorKeepsNewerCredentials(t *testing.T) {
	issuer := newFakeIssuer(t)
	release := issuer.gateRefreshes(t)
	store := expiringStore(t, "dead")
	session, err := NewSession(store, issuer.provider())
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	var token string
	go func() {
		var err error
		token, err = session.Token(context.Background())
		result <- err
	}()
	<-issuer.refreshStarted
	// A writer outside the lock protocol (an older nickpit, a copied file)
	// stores a fresh sign-in while the doomed refresh is in flight.
	newer := &Credentials{ClientID: "oaiapp_123", Subject: "user-123", AccessToken: "newer-access", RefreshToken: "newer-refresh",
		TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour), Scopes: strings.Fields(Scopes)}
	if err := store.Save(newer); err != nil {
		t.Fatal(err)
	}
	release()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if token != "newer-access" {
		t.Fatalf("token = %q", token)
	}
	if stored, _ := store.Load(); !stored.SignedIn() || stored.RefreshToken != "newer-refresh" {
		t.Fatalf("newer credentials were cleared: %+v", stored)
	}
}

func TestLockFileExcludesConcurrentHolders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json.lock")
	unlock, err := lockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	acquired := make(chan struct{})
	go func() {
		release, err := lockFile(path)
		if err != nil {
			t.Error(err)
			close(acquired)
			return
		}
		close(acquired)
		release()
	}()
	select {
	case <-acquired:
		t.Fatal("a second holder acquired the lock while it was held")
	case <-time.After(100 * time.Millisecond):
	}
	unlock()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("the lock was not handed over after release")
	}
}

// TestHelperLogoutProcess is a child process of
// TestSessionStopsAfterLogoutInAnotherProcess, not a test of its own.
func TestHelperLogoutProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "logout" {
		t.Skip("helper process")
	}
	store := &Store{Dir: os.Getenv("HELPER_DIR"), AuthFile: os.Getenv("HELPER_AUTH_FILE")}
	provider := &Provider{Issuer: os.Getenv("HELPER_ISSUER"), HTTPClient: http.DefaultClient}
	if _, err := Logout(context.Background(), store, provider, false); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
}

func validStore(t *testing.T, accessToken, subject string) *Store {
	t.Helper()
	store := &Store{Dir: t.TempDir()}
	store.AuthFile = filepath.Join(store.Dir, "auth.json")
	if err := store.Save(&Credentials{
		ClientID: "oaiapp_123", Subject: subject, AccessToken: accessToken, RefreshToken: "refresh-" + accessToken,
		TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour), Scopes: strings.Fields(Scopes),
	}); err != nil {
		t.Fatal(err)
	}
	return store
}

// A session whose cached token is still valid for an hour must stop as soon
// as any process signs out: the token stays accepted remotely until it
// expires, so only the client can honour the logout.
func TestSessionStopsAfterLogoutInAnotherProcess(t *testing.T) {
	issuer := newFakeIssuer(t)
	store := validStore(t, "cached-access", "user-123")
	session, err := NewSession(store, issuer.provider())
	if err != nil {
		t.Fatal(err)
	}
	if token, err := session.Token(context.Background()); err != nil || token != "cached-access" {
		t.Fatalf("token before logout = %q, %v", token, err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperLogoutProcess$", "-test.count=1")
	cmd.Env = append(os.Environ(), helperEnv+"=logout", "HELPER_DIR="+store.Dir, "HELPER_AUTH_FILE="+store.AuthFile, "HELPER_ISSUER="+issuer.issuer())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("logout process: %v\n%s", err, out)
	}

	if token, err := session.Token(context.Background()); !errors.Is(err, ErrNotSignedIn) {
		t.Fatalf("token after logout = %q, %v; want ErrNotSignedIn", token, err)
	}
	lazy := &LazySession{Store: store, Provider: issuer.provider()}
	if _, err := lazy.Token(context.Background()); !errors.Is(err, ErrNotSignedIn) {
		t.Fatalf("lazy token after logout = %v; want ErrNotSignedIn", err)
	}
}

// A new sign-in elsewhere replaces what the session sends, and its account
// identity changes with it.
func TestSessionFollowsAnotherSignIn(t *testing.T) {
	issuer := newFakeIssuer(t)
	store := validStore(t, "access-a", "user-a")
	session, err := NewSession(store, issuer.provider())
	if err != nil {
		t.Fatal(err)
	}
	before := session.AccountID()
	if err := store.Replace(&Credentials{
		ClientID: "oaiapp_123", Subject: "user-b", AccessToken: "access-b", RefreshToken: "refresh-b",
		TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour), Scopes: strings.Fields(Scopes),
	}); err != nil {
		t.Fatal(err)
	}
	if token, err := session.Token(context.Background()); err != nil || token != "access-b" {
		t.Fatalf("token = %q, %v; want the new sign-in's", token, err)
	}
	if after := session.AccountID(); after == before || !strings.Contains(after, "user-b") || strings.Contains(after, "access") {
		t.Fatalf("account id %q -> %q", before, after)
	}
}

// TestHelperHostIDProcess is a child process of
// TestConcurrentFirstSignInsShareOneHostID, not a test of its own.
func TestHelperHostIDProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "hostid" {
		t.Skip("helper process")
	}
	store := &Store{Dir: os.Getenv("HELPER_DIR"), AuthFile: os.Getenv("HELPER_AUTH_FILE")}
	id, err := store.HostID()
	if err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
	fmt.Println("HOSTID", id)
}

// Simultaneous first-time sign-ins in separate processes all present, and
// keep, the same host ID.
func TestConcurrentFirstSignInsShareOneHostID(t *testing.T) {
	dir := t.TempDir()
	const processes = 6
	commands := make([]*exec.Cmd, processes)
	outputs := make([]*bytes.Buffer, processes)
	for i := range commands {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHelperHostIDProcess$", "-test.count=1")
		// Each process names its own credential file in the shared
		// directory: the host ID is shared all the same.
		cmd.Env = append(os.Environ(), helperEnv+"=hostid", "HELPER_DIR="+dir, "HELPER_AUTH_FILE="+filepath.Join(dir, fmt.Sprintf("account-%d.json", i)))
		outputs[i] = &bytes.Buffer{}
		cmd.Stdout, cmd.Stderr = outputs[i], outputs[i]
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands[i] = cmd
	}
	ids := map[string]bool{}
	for i, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("process %d failed: %v\n%s", i, err, outputs[i])
		}
		for line := range strings.SplitSeq(outputs[i].String(), "\n") {
			if id, ok := strings.CutPrefix(line, "HOSTID "); ok {
				ids[id] = true
			}
		}
	}
	stored, err := (&Store{Dir: dir}).HostID()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || !ids[stored] {
		t.Fatalf("host ids = %v, stored %q; want one shared id", ids, stored)
	}
}

// The lock always lives beside the credentials, also for a store that names
// only its directory.
func TestLockFileStaysInTheStoreDirectory(t *testing.T) {
	dir := t.TempDir()
	if got := (&Store{Dir: dir}).lockFile(); filepath.Dir(got) != dir {
		t.Fatalf("lock file = %q, want it in %q", got, dir)
	}
	if got := (&Store{Dir: dir, AuthFile: filepath.Join(dir, "x.json")}).lockFile(); got != filepath.Join(dir, "x.json.lock") {
		t.Fatalf("lock file = %q", got)
	}
}
