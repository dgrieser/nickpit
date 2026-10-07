package forgejo

import (
	"context"
	"fmt"
)

// PRStatus is the minimal live state of a pull request, for a webhook daemon to
// check right before starting a review so closed/merged/draft pull requests
// are skipped on authoritative data rather than a possibly stale payload.
// State is "open" or "closed": Forgejo has no merged state, a merged pull
// request is closed with Merged set. BaseSHA is the merge base the diff is
// computed against, not base.sha (the base branch's tip, which moves with every
// push there): together with HeadSHA it identifies the pull request's current
// diff, so a retargeted one (base moved, head unchanged) is detectable, and it
// is the value FetchPR records as the context's DiffBaseSHA.
type PRStatus struct {
	State   string
	Merged  bool
	Draft   bool
	HeadSHA string
	BaseSHA string
}

// FetchPRStatus fetches a pull request's current state.
func (c *Client) FetchPRStatus(ctx context.Context, repo string, number int) (*PRStatus, error) {
	var pr prResponse
	if err := c.Get(ctx, fmt.Sprintf("/repos/%s/pulls/%d", escapeRepo(repo), number), &pr); err != nil {
		return nil, err
	}
	return &PRStatus{
		State:   pr.State,
		Merged:  pr.Merged,
		Draft:   pr.Draft,
		HeadSHA: pr.Head.SHA,
		BaseSHA: pr.MergeBase,
	}, nil
}
