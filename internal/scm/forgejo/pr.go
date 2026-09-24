package forgejo

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

type prResponse struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Base  struct {
		Ref  string `json:"ref"`
		SHA  string `json:"sha"`
		Repo *struct {
			FullName string `json:"full_name"`
			CloneURL string `json:"clone_url"`
		} `json:"repo"`
	} `json:"base"`
	Head struct {
		Ref  string `json:"ref"`
		SHA  string `json:"sha"`
		Repo *struct {
			FullName string `json:"full_name"`
			CloneURL string `json:"clone_url"`
		} `json:"repo"`
	} `json:"head"`
	HTMLURL string `json:"html_url"`
	// MergeBase is the commit the pull request diff is computed against.
	MergeBase string `json:"merge_base"`
}

type commitResponse struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

type reviewResponse struct {
	ID        int       `json:"id"`
	Body      string    `json:"body"`
	User      userRef   `json:"user"`
	Submitted time.Time `json:"submitted_at"`
	// CommentsCount says whether the review carries inline comments at all,
	// which is what decides whether they are fetched (one request per review).
	CommentsCount int `json:"comments_count"`
}

// reviewCommentResponse is one inline review comment. Forgejo reports the
// new-side line as "position" and the old-side line as "original_position";
// a comment on a deleted line has no new-side position.
type reviewCommentResponse struct {
	Body             string    `json:"body"`
	Path             string    `json:"path"`
	Position         int       `json:"position"`
	OriginalPosition int       `json:"original_position"`
	CreatedAt        time.Time `json:"created_at"`
	User             userRef   `json:"user"`
}

type issueCommentResponse struct {
	ID        int       `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	User      userRef   `json:"user"`
}

type userRef struct {
	Login string `json:"login"`
}

// surfaces are the three places a review's comments live: review bodies,
// the inline comments of those reviews, and plain issue comments.
type surfaces struct {
	reviews  []reviewResponse
	inline   []reviewCommentResponse
	comments []issueCommentResponse
}

// fetchSurfaces reads every comment surface of a pull request. With tolerant
// set, a surface that cannot be read is left empty rather than failing the
// read: a publisher deduplicating against prior comments would rather risk a
// duplicate than post nothing, whereas review reassembly needs every surface
// because a partial read would silently look like "no review here".
func (c *Client) fetchSurfaces(ctx context.Context, repo string, number int, tolerant bool) (surfaces, error) {
	escaped := escapeRepo(repo)
	var out surfaces
	fail := func(err error) (surfaces, error) {
		if tolerant {
			return out, nil
		}
		return surfaces{}, err
	}
	if err := c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews", escaped, number), &out.reviews); err != nil {
		return fail(err)
	}
	for _, review := range out.reviews {
		if review.CommentsCount == 0 {
			continue
		}
		var comments []reviewCommentResponse
		if err := c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews/%d/comments", escaped, number, review.ID), &comments); err != nil {
			if tolerant {
				continue
			}
			return surfaces{}, err
		}
		out.inline = append(out.inline, comments...)
	}
	if err := c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/issues/%d/comments", escaped, number), &out.comments); err != nil {
		return fail(err)
	}
	return out, nil
}

func (c *Client) FetchPR(ctx context.Context, repo string, number int, includeComments bool) (*model.ReviewContext, error) {
	var pr prResponse
	escaped := escapeRepo(repo)
	if err := c.Get(ctx, fmt.Sprintf("/repos/%s/pulls/%d", escaped, number), &pr); err != nil {
		return nil, err
	}

	var commits []commitResponse
	if err := c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/commits", escaped, number), &commits); err != nil {
		return nil, err
	}

	// The files listing carries no patch, so the diff is downloaded whole: the
	// same `git diff` text a local review parses, with file modes and rename
	// headers, from which the changed files are derived as well.
	diff, err := c.fetchDiff(ctx, escaped, number)
	if err != nil {
		return nil, err
	}
	diffFiles, hunks, changedFiles, err := git.ParseUnifiedDiffFormats(diff)
	if err != nil {
		return nil, fmt.Errorf("forgejo: parsing pull request diff: %w", err)
	}

	var comments []model.Comment
	if includeComments {
		// Hidden nickpit markers are stripped before a body enters prompt
		// context: the carrier payloads are large opaque blobs that would waste
		// model tokens and displace real comments during trimming. Comments that
		// were only carriers are dropped entirely.
		found, _ := c.fetchSurfaces(ctx, repo, number, true)
		for _, item := range found.reviews {
			body := reviewmd.StripMarkers(item.Body)
			if body == "" {
				continue
			}
			comments = append(comments, model.Comment{
				Author:    item.User.Login,
				Body:      body,
				CreatedAt: item.Submitted,
				IsReview:  true,
			})
		}
		for _, item := range found.inline {
			body := reviewmd.StripMarkers(item.Body)
			if body == "" {
				continue
			}
			line := item.Position
			side := "RIGHT"
			if line == 0 {
				// A comment on a removed line has no new-side position; the
				// old-side line is the only anchor left.
				line = item.OriginalPosition
				side = "LEFT"
			}
			comments = append(comments, model.Comment{
				Author:    item.User.Login,
				Body:      body,
				Path:      item.Path,
				Line:      line,
				Side:      side,
				CreatedAt: item.CreatedAt,
				IsReview:  true,
			})
		}
		for _, item := range found.comments {
			body := reviewmd.StripMarkers(item.Body)
			if body == "" {
				continue
			}
			comments = append(comments, model.Comment{
				Author:    item.User.Login,
				Body:      body,
				CreatedAt: item.CreatedAt,
			})
		}
	}

	return &model.ReviewContext{
		Mode:       model.ModeForgejo,
		Identifier: number,
		Repository: model.RepositoryInfo{
			FullName: repo,
			BaseRef:  pr.Base.Ref,
			HeadRef:  pr.Head.Ref,
			URL:      pr.HTMLURL,
		},
		Title:        pr.Title,
		Description:  pr.Body,
		Commits:      normalizeCommits(commits),
		ChangedFiles: changedFiles,
		Diff:         diff,
		DiffFiles:    diffFiles,
		DiffHunks:    hunks,
		Comments:     comments,
		// The diff is computed between the merge base and the head, and unlike
		// GitHub the API reports both, so a cached context's freshness can be
		// checked against the live request.
		DiffBaseSHA: pr.MergeBase,
		DiffHeadSHA: pr.Head.SHA,
	}, nil
}

// fetchDiff downloads the pull request's unified diff.
func (c *Client) fetchDiff(ctx context.Context, escapedRepo string, number int) (string, error) {
	diff, err := c.GetRaw(ctx, fmt.Sprintf("/repos/%s/pulls/%d.diff", escapedRepo, number))
	if err != nil {
		return "", err
	}
	return string(diff), nil
}

func (c *Client) FetchPRCheckout(ctx context.Context, repo string, number int) (*model.CheckoutSpec, error) {
	var pr prResponse
	escaped := escapeRepo(repo)
	if err := c.Get(ctx, fmt.Sprintf("/repos/%s/pulls/%d", escaped, number), &pr); err != nil {
		return nil, err
	}
	cloneURL := ""
	// Once the head branch is gone Forgejo reports head.ref as the pull ref,
	// refs/pull/N/head, while head.repo still names the fork. That ref only
	// exists in the base repository, so it is fetched from there.
	if pr.Head.Repo != nil && !strings.HasPrefix(pr.Head.Ref, "refs/pull/") {
		cloneURL = pr.Head.Repo.CloneURL
	}
	if cloneURL == "" && pr.Base.Repo != nil {
		cloneURL = pr.Base.Repo.CloneURL
	}
	return &model.CheckoutSpec{
		Provider: model.ModeForgejo,
		Repo:     repo,
		CloneURL: cloneURL,
		HeadRef:  pr.Head.Ref,
		HeadSHA:  pr.Head.SHA,
	}, nil
}

// PRPositionInfo is the freshly-fetched diff state used when publishing review
// comments back to a PR. It is fetched at post-time (not reused from the review
// context) so the head SHA and hunks match the current diff and the server
// accepts the inline comment positions.
type PRPositionInfo struct {
	HeadSHA string
	// Hunks maps a file's new-side path to its parsed diff hunks.
	Hunks map[string][]model.DiffHunk
}

// FetchPRPositionInfo fetches the PR's head SHA and per-file diff hunks so review
// findings can be anchored to exact diff lines.
func (c *Client) FetchPRPositionInfo(ctx context.Context, repo string, number int) (*PRPositionInfo, error) {
	escaped := escapeRepo(repo)
	var pr prResponse
	if err := c.Get(ctx, fmt.Sprintf("/repos/%s/pulls/%d", escaped, number), &pr); err != nil {
		return nil, err
	}
	diff, err := c.fetchDiff(ctx, escaped, number)
	if err != nil {
		return nil, err
	}
	hunks, _, err := git.ParseUnifiedDiff(diff)
	if err != nil {
		return nil, fmt.Errorf("forgejo: parsing pull request diff: %w", err)
	}
	byPath := make(map[string][]model.DiffHunk)
	for _, hunk := range hunks {
		byPath[hunk.FilePath] = append(byPath[hunk.FilePath], hunk)
	}
	return &PRPositionInfo{HeadSHA: pr.Head.SHA, Hunks: byPath}, nil
}

func normalizeCommits(in []commitResponse) []model.CommitSummary {
	out := make([]model.CommitSummary, 0, len(in))
	for _, item := range in {
		out = append(out, model.CommitSummary{
			SHA:     item.SHA,
			Message: item.Commit.Message,
			Author:  item.Commit.Author.Name,
			Date:    item.Commit.Author.Date,
		})
	}
	return out
}

// FetchBaseFile reads path from the PR's BASE repository at the base commit.
//
// The base, never the head, is the point: for a fork PR the head repository
// belongs to the contributor, so a file read from it is attacker-controlled.
// The base commit SHA is preferred over the base branch name because it is
// immutable and is what the reviewed diff is computed against; the branch name
// is only a fallback for a payload that omits the SHA.
func (c *Client) FetchBaseFile(ctx context.Context, repo string, number int, path string) ([]byte, bool, error) {
	var pr prResponse
	if err := c.Get(ctx, fmt.Sprintf("/repos/%s/pulls/%d", escapeRepo(repo), number), &pr); err != nil {
		return nil, false, err
	}
	baseRepo := repo
	if pr.Base.Repo != nil && pr.Base.Repo.FullName != "" {
		baseRepo = pr.Base.Repo.FullName
	}
	ref := pr.Base.SHA
	if ref == "" {
		ref = pr.Base.Ref
	}
	if ref == "" {
		return nil, false, nil
	}
	// The raw endpoint answers with the file bytes themselves, so there is no
	// encoding to undo and no size field to trust: the body is bounded here.
	endpoint := fmt.Sprintf("/repos/%s/raw/%s?ref=%s", escapeRepo(baseRepo), escapePath(path), url.QueryEscape(ref))
	data, err := c.GetRaw(ctx, endpoint)
	if err != nil {
		if IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(data) > model.MaxBaseFileBytes {
		return nil, false, fmt.Errorf("forgejo: %s at %s exceeds %d bytes", path, ref, model.MaxBaseFileBytes)
	}
	return data, true, nil
}
