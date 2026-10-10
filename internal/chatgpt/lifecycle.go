package chatgpt

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// WithLock runs fn holding the credential lock: the advisory lock every
// writer of the credential file — login, logout, and token refresh, in this
// process or another — takes, so none of them acts on a token another has
// already rotated, revoked, or replaced.
func (s *Store) WithLock(fn func() error) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("chatgpt: creating %s: %w", s.Dir, err)
	}
	unlock, err := lockFile(s.lockFile())
	if err != nil {
		return fmt.Errorf("chatgpt: locking credentials: %w", err)
	}
	defer unlock()
	return fn()
}

// SaveRegistration records a client ID issued during sign-in before its code
// is exchanged, so a failed exchange reauthorizes the same registration
// instead of registering again. A record already holding that client ID —
// signed in or not — is left alone.
func (s *Store) SaveRegistration(clientID string) error {
	return s.WithLock(func() error {
		current, err := s.Load()
		if err != nil {
			return err
		}
		if current != nil && current.ClientID == clientID {
			return nil
		}
		return s.Save(&Credentials{ClientID: clientID})
	})
}

// Replace stores the credentials of a completed sign-in.
func (s *Store) Replace(creds *Credentials) error {
	return s.WithLock(func() error { return s.Save(creds) })
}

// LogoutResult says what a logout did.
type LogoutResult struct {
	// SignedOut is false when no credentials were stored.
	SignedOut bool
	// RevokeErr reports a remote revocation that could not be confirmed; the
	// local credentials are removed regardless.
	RevokeErr error
}

// Logout revokes the stored session and removes its tokens. It holds the
// credential lock throughout, so a refresh running in another process either
// finishes first — and the token it rotated in is the one revoked — or starts
// after and finds no session to renew. With forget, the registration and
// account are removed as well; otherwise they are kept so the next sign-in
// reauthorizes the same client.
func Logout(ctx context.Context, store *Store, provider *Provider, forget bool) (LogoutResult, error) {
	var result LogoutResult
	err := store.WithLock(func() error {
		creds, err := store.Load()
		if err != nil {
			return err
		}
		if creds == nil {
			return nil
		}
		result.SignedOut = true
		if creds.RefreshToken != "" {
			result.RevokeErr = provider.Revoke(ctx, creds)
		}
		if forget {
			if err := os.Remove(store.AuthFile); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return nil
		}
		return store.Save(&Credentials{ClientID: creds.ClientID, Issuer: creds.Issuer, Subject: creds.Subject, Email: creds.Email, Name: creds.Name})
	})
	return result, err
}
