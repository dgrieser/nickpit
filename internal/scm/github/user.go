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
func (c *Client) CurrentUser(ctx context.Context) (*User, error) {
	var user User
	if err := c.Get(ctx, "/user", &user); err != nil {
		return nil, err
	}
	return &user, nil
}
