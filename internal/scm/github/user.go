package github

import "context"

// User is the authenticated token owner. GitHub identifies comment authors by
// login in every comment payload, so the login is what a carrier's author check
// compares against; the numeric id is decoded for callers that log it.
type User struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
}

// CurrentUser returns the user the client's token authenticates as. A GitHub App
// installation token has no user behind it and gets a 403 here, so callers that
// only need best-effort identity must tolerate the error rather than fail on it.
// A successful answer is memoized for the client's lifetime: it identifies the
// token, which cannot change under it, and callers that verify carrier
// authorship ask once per request they read.
func (c *Client) CurrentUser(ctx context.Context) (*User, error) {
	c.userMu.Lock()
	defer c.userMu.Unlock()
	if c.user != nil {
		return c.user, nil
	}
	var user User
	if err := c.Get(ctx, "/user", &user); err != nil {
		return nil, err
	}
	c.user = &user
	return c.user, nil
}
