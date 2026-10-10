package chatgpt

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// refreshMargin is how long before expiry an access token is renewed.
const refreshMargin = 2 * time.Minute

// Session hands out access tokens for the stored account, refreshing them as
// they near expiry. It implements llm.TokenSource. Refreshes are serialized
// within the process by a mutex and across processes by a lock file, and the
// credential file is re-read under that lock, so a token another process
// already rotated is picked up instead of spending the old refresh token.
type Session struct {
	Store    *Store
	Provider *Provider

	mu    sync.Mutex
	creds *Credentials
}

// NewSession loads the stored account. It fails with ErrNotSignedIn when no
// usable credentials exist, and when the grant lacks plan usage.
func NewSession(store *Store, provider *Provider) (*Session, error) {
	creds, err := store.Load()
	if err != nil {
		return nil, err
	}
	if !creds.SignedIn() {
		return nil, ErrNotSignedIn
	}
	if !creds.PlanUsage() {
		return nil, fmt.Errorf("chatgpt: the sign-in did not grant ChatGPT plan usage (%s); run `nickpit chatgpt login --reconsent`", ScopePlanUsage)
	}
	return &Session{Store: store, Provider: provider, creds: creds}, nil
}

// Credentials returns a copy of the current credentials.
func (s *Session) Credentials() Credentials {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.creds
}

// Token returns a valid access token, refreshing first when it expires within
// refreshMargin.
func (s *Session) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.needsRefresh(s.creds, time.Now()) {
		return s.creds.AccessToken, nil
	}
	return s.refreshLocked(ctx, false)
}

// ForceRefresh renews the access token regardless of its expiry, unless
// another process already rotated it since this one loaded it.
func (s *Session) ForceRefresh(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshLocked(ctx, true)
}

func (s *Session) needsRefresh(creds *Credentials, now time.Time) bool {
	return creds.AccessToken == "" || !now.Add(refreshMargin).Before(creds.ExpiresAt)
}

func (s *Session) refreshLocked(ctx context.Context, force bool) (string, error) {
	var token string
	err := s.Store.WithLock(func() error {
		var err error
		token, err = s.refreshUnderLock(ctx, force)
		return err
	})
	return token, err
}

// refreshUnderLock runs with the credential lock held: every other writer
// (refresh, login, logout) waits, so what it reads is what it replaces.
func (s *Session) refreshUnderLock(ctx context.Context, force bool) (string, error) {
	// Another process may have rotated the tokens while this one waited.
	stored, err := s.Store.Load()
	if err != nil {
		return "", err
	}
	if !stored.SignedIn() {
		return "", ErrNotSignedIn
	}
	if stored.AccessToken != s.creds.AccessToken {
		s.creds = stored
		if !s.needsRefresh(stored, time.Now()) {
			return stored.AccessToken, nil
		}
	} else if !force && !s.needsRefresh(stored, time.Now()) {
		return stored.AccessToken, nil
	}

	now := time.Now()
	if !stored.EarliestRefreshAt.IsZero() && now.Before(stored.EarliestRefreshAt) {
		if now.Before(stored.ExpiresAt) {
			return stored.AccessToken, nil
		}
		return "", fmt.Errorf("chatgpt: the connection cannot refresh before %s; try again shortly", stored.EarliestRefreshAt.Local().Format(time.Kitchen))
	}

	next, err := s.Provider.Refresh(ctx, stored)
	if err != nil {
		if RequiresSignIn(err) {
			// Clear only the session this refresh spent. Should the file no
			// longer hold it — a writer outside the lock protocol replaced it
			// — the newer credentials stay, and are used.
			current, loadErr := s.Store.Load()
			if loadErr == nil && current.SignedIn() && current.RefreshToken != stored.RefreshToken {
				s.creds = current
				return current.AccessToken, nil
			}
			if loadErr == nil && current != nil {
				current.AccessToken, current.RefreshToken = "", ""
				_ = s.Store.Save(current)
			}
			return "", fmt.Errorf("%w; run `nickpit chatgpt login` again", err)
		}
		return "", err
	}
	if err := s.Store.Save(next); err != nil {
		return "", err
	}
	s.creds = next
	return next.AccessToken, nil
}

// Model is one entry of the account's model catalog.
type Model struct {
	Slug        string
	DisplayName string
}

// ListModels returns the models the signed-in account can use, in catalog
// order.
func (s *Session) ListModels(ctx context.Context) ([]Model, error) {
	token, err := s.Token(ctx)
	if err != nil {
		return nil, err
	}
	var catalog struct {
		Models []struct {
			Slug        string `json:"slug"`
			DisplayName string `json:"display_name"`
			Visibility  string `json:"visibility"`
		} `json:"models"`
	}
	if err := s.Provider.getJSON(ctx, strings.TrimRight(Resource, "/")+"/models", token, &catalog); err != nil {
		return nil, fmt.Errorf("chatgpt: listing models: %w", err)
	}
	models := make([]Model, 0, len(catalog.Models))
	for _, entry := range catalog.Models {
		if entry.Visibility != "list" || strings.TrimSpace(entry.Slug) == "" {
			continue
		}
		models = append(models, Model{Slug: entry.Slug, DisplayName: entry.DisplayName})
	}
	return models, nil
}

// LazySession is a token source that loads the stored account on first use,
// so building a client for a ChatGPT profile never fails up front: commands
// that end up making no LLM call need no sign-in, and a sign-in made while a
// long-running process waits is picked up by its next request.
type LazySession struct {
	Store    *Store
	Provider *Provider

	mu      sync.Mutex
	session *Session
}

func (l *LazySession) get() (*Session, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.session != nil {
		return l.session, nil
	}
	session, err := NewSession(l.Store, l.Provider)
	if err != nil {
		return nil, err
	}
	l.session = session
	return session, nil
}

// Token implements llm.TokenSource.
func (l *LazySession) Token(ctx context.Context) (string, error) {
	session, err := l.get()
	if err != nil {
		return "", err
	}
	return session.Token(ctx)
}

// ForceRefresh implements llm.TokenSource.
func (l *LazySession) ForceRefresh(ctx context.Context) (string, error) {
	session, err := l.get()
	if err != nil {
		return "", err
	}
	return session.ForceRefresh(ctx)
}
