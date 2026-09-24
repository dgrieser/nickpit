package forgejo

import "context"

// User is the authenticated token owner. Forgejo identifies comment authors
// by login (and id) in every comment payload, so the login is what a
// carrier's author check compares against.
type User struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
}

// CurrentUser returns the user the client's token authenticates as.
func (c *Client) CurrentUser(ctx context.Context) (*User, error) {
	var user User
	if err := c.Get(ctx, "/user", &user); err != nil {
		return nil, err
	}
	return &user, nil
}
