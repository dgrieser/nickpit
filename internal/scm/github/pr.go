package github

import (
	"context"
	"fmt"
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

type fileResponse struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
	Contents  string `json:"contents_url"`
}

type reviewResponse struct {
	ID        int       `json:"id"`
	Body      string    `json:"body"`
	User      userRef   `json:"user"`
	Submitted time.Time `json:"submitted_at"`
}

type commentResponse struct {
	Body string `json:"body"`
	Path string `json:"path"`
	// Line is null for comments whose anchor is no longer part of the diff
	// (outdated comments); OriginalLine then still carries the position the
	// comment was made on.
	Line         int       `json:"line"`
	OriginalLine int       `json:"original_line"`
	Side         string    `json:"side"`
	CreatedAt    time.Time `json:"created_at"`
	User         userRef   `json:"user"`
}

type issueCommentResponse struct {
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	User      userRef   `json:"user"`
}

type userRef struct {
	Login string `json:"login"`
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

	var files []fileResponse
	if err := c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/files", escaped, number), &files); err != nil {
		return nil, err
	}

	var comments []model.Comment
	if includeComments {
		// Hidden nickpit markers are stripped before a body enters prompt
		// context: the carrier payloads are large opaque blobs that would waste
		// model tokens and displace real comments during trimming. Comments that
		// were only carriers are dropped entirely.
		var reviews []reviewResponse
		_ = c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews", escaped, number), &reviews)
		for _, item := range reviews {
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

		var lineComments []commentResponse
		_ = c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/comments", escaped, number), &lineComments)
		for _, item := range lineComments {
			body := reviewmd.StripMarkers(item.Body)
			if body == "" {
				continue
			}
			line := item.Line
			if line == 0 {
				// Outdated comments carry a null line; fall back to the line
				// the comment was originally made on instead of Line:0.
				line = item.OriginalLine
			}
			comments = append(comments, model.Comment{
				Author:    item.User.Login,
				Body:      body,
				Path:      item.Path,
				Line:      line,
				Side:      item.Side,
				CreatedAt: item.CreatedAt,
				IsReview:  true,
			})
		}

		var issueComments []issueCommentResponse
		_ = c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/issues/%d/comments", escaped, number), &issueComments)
		for _, item := range issueComments {
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

	changedFiles := make([]model.ChangedFile, 0, len(files))
	for _, file := range files {
		status := model.FileModified
		switch file.Status {
		case "added":
			status = model.FileAdded
		case "removed":
			status = model.FileDeleted
		case "renamed":
			status = model.FileRenamed
		}
		changedFiles = append(changedFiles, model.ChangedFile{
			Path:      file.Filename,
			Status:    status,
			Additions: file.Additions,
			Deletions: file.Deletions,
			PatchURL:  file.Contents,
		})
	}
	diff := framedDiff(files)
	diffFiles, hunks, _, _ := git.ParseUnifiedDiffFormats(diff)
	return &model.ReviewContext{
		Mode:       model.ModeGitHub,
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
	}, nil
}

func (c *Client) FetchPRCheckout(ctx context.Context, repo string, number int) (*model.CheckoutSpec, error) {
	var pr prResponse
	escaped := escapeRepo(repo)
	if err := c.Get(ctx, fmt.Sprintf("/repos/%s/pulls/%d", escaped, number), &pr); err != nil {
		return nil, err
	}
	cloneURL := ""
	if pr.Head.Repo != nil {
		cloneURL = pr.Head.Repo.CloneURL
	}
	if cloneURL == "" && pr.Base.Repo != nil {
		cloneURL = pr.Base.Repo.CloneURL
	}
	return &model.CheckoutSpec{
		Provider: model.ModeGitHub,
		Repo:     repo,
		CloneURL: cloneURL,
		HeadRef:  pr.Head.Ref,
		HeadSHA:  pr.Head.SHA,
	}, nil
}

// PRPositionInfo is the freshly-fetched diff state used when publishing review
// comments back to a PR. It is fetched at post-time (not reused from the review
// context) so the head SHA and hunks match the current diff and GitHub accepts
// the inline comment positions.
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
	var files []fileResponse
	if err := c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/files", escaped, number), &files); err != nil {
		return nil, err
	}
	hunks, _, _ := git.ParseUnifiedDiff(framedDiff(files))
	byPath := make(map[string][]model.DiffHunk, len(files))
	for _, hunk := range hunks {
		byPath[hunk.FilePath] = append(byPath[hunk.FilePath], hunk)
	}
	return &PRPositionInfo{HeadSHA: pr.Head.SHA, Hunks: byPath}, nil
}

// framedDiff reconstructs a unified diff from the PR file patches, re-creating
// the minimal "diff --git" framing so git.ParseUnifiedDiff attributes each hunk
// to its file (the GitHub files API returns per-file patches without it).
func framedDiff(files []fileResponse) string {
	var diff strings.Builder
	for _, file := range files {
		if file.Patch == "" {
			continue
		}
		diff.WriteString("diff --git a/")
		diff.WriteString(file.Filename)
		diff.WriteString(" b/")
		diff.WriteString(file.Filename)
		diff.WriteByte('\n')
		diff.WriteString(file.Patch)
		diff.WriteByte('\n')
	}
	return diff.String()
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
