package github

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
)

// openPRResponse is the subset of the pull request listing the interactive
// picker shows.
type openPRResponse struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	HTMLURL   string    `json:"html_url"`
	Draft     bool      `json:"draft"`
	UpdatedAt time.Time `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

// maxListedOpenPRs bounds one listing to a single request (GitHub's per_page
// maximum); more open PRs than this are not usefully browsable in a picker.
const maxListedOpenPRs = 100

// ListOpenPRs returns the open pull requests of a repo, most recently updated
// first. Drafts are included and labeled by the picker.
func (c *Client) ListOpenPRs(ctx context.Context, repo string) ([]model.OpenRequest, error) {
	path := fmt.Sprintf("/repos/%s/pulls?state=open&sort=updated&direction=desc&per_page=%d",
		escapeRepo(repo), maxListedOpenPRs)
	var response []openPRResponse
	if err := c.Get(ctx, path, &response); err != nil {
		return nil, err
	}
	requests := make([]model.OpenRequest, 0, len(response))
	for _, pr := range response {
		if pr.Number <= 0 {
			continue
		}
		requests = append(requests, model.OpenRequest{
			Identifier:   pr.Number,
			Title:        pr.Title,
			Author:       pr.User.Login,
			SourceBranch: pr.Head.Ref,
			TargetBranch: pr.Base.Ref,
			Draft:        pr.Draft,
			UpdatedAt:    pr.UpdatedAt,
			WebURL:       pr.HTMLURL,
		})
	}
	// Sorting locally keeps the order independent of a server that ignores the
	// sort parameters, with the number breaking ties for determinism.
	sort.SliceStable(requests, func(i, j int) bool {
		if !requests[i].UpdatedAt.Equal(requests[j].UpdatedAt) {
			return requests[i].UpdatedAt.After(requests[j].UpdatedAt)
		}
		return requests[i].Identifier > requests[j].Identifier
	})
	return requests, nil
}
