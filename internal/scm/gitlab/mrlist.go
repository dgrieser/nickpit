package gitlab

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
)

// openMRResponse is the subset of the merge request listing the interactive
// picker shows. It is deliberately narrower than mrResponse: no diff refs, no
// project ids — nothing that would tempt a caller to review from the listing
// instead of fetching the MR.
type openMRResponse struct {
	IID          int       `json:"iid"`
	Title        string    `json:"title"`
	WebURL       string    `json:"web_url"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	Draft        bool      `json:"draft"`
	UpdatedAt    time.Time `json:"updated_at"`
	Author       struct {
		Username string `json:"username"`
	} `json:"author"`
}

// maxListedOpenMRs bounds one listing. A project with more open MRs than this
// is not usefully browsable in a picker, and the cap keeps a single request
// enough — GitLab's per_page maximum is 100.
const maxListedOpenMRs = 100

// ListOpenMRs returns the open merge requests of a project, most recently
// updated first. Drafts are included: they are the ones most likely to want a
// review, and the picker labels them.
func (c *Client) ListOpenMRs(ctx context.Context, project string) ([]model.OpenRequest, error) {
	return c.listMRs(ctx, project, "opened")
}

// ListMRs is ListOpenMRs over every state — merged and closed requests too —
// for a caller reading what was published on them rather than looking for
// something to review.
func (c *Client) ListMRs(ctx context.Context, project string) ([]model.OpenRequest, error) {
	return c.listMRs(ctx, project, "all")
}

func (c *Client) listMRs(ctx context.Context, project, state string) ([]model.OpenRequest, error) {
	path := fmt.Sprintf("/projects/%s/merge_requests?state=%s&order_by=updated_at&sort=desc&per_page=%d",
		escapeProject(project), state, maxListedOpenMRs)
	var response []openMRResponse
	if err := c.Get(ctx, path, &response); err != nil {
		return nil, err
	}
	requests := make([]model.OpenRequest, 0, len(response))
	for _, mr := range response {
		if mr.IID <= 0 {
			continue
		}
		requests = append(requests, model.OpenRequest{
			Identifier:   mr.IID,
			Title:        mr.Title,
			Author:       mr.Author.Username,
			SourceBranch: mr.SourceBranch,
			TargetBranch: mr.TargetBranch,
			Draft:        mr.Draft,
			UpdatedAt:    mr.UpdatedAt,
			WebURL:       mr.WebURL,
		})
	}
	// GitLab already sorts by update time; re-sorting locally makes the order
	// independent of a server that ignores order_by, and ties break on the IID
	// so two runs never disagree about which row comes first.
	sort.SliceStable(requests, func(i, j int) bool {
		if !requests[i].UpdatedAt.Equal(requests[j].UpdatedAt) {
			return requests[i].UpdatedAt.After(requests[j].UpdatedAt)
		}
		return requests[i].Identifier > requests[j].Identifier
	})
	return requests, nil
}
