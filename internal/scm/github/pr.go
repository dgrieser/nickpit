package github

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
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
	// PreviousFilename is the pre-change path of a rename or copy. A pure rename
	// has no patch at all, so without it the move is invisible — and for a
	// relative symlink the move alone decides whether the target still resolves.
	PreviousFilename string `json:"previous_filename"`
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
		oldPath := ""
		if file.Status == "renamed" && file.PreviousFilename != file.Filename {
			// A pure rename carries no hunk, so the old path is the only record of
			// the move; for a relative symlink it is what decides whether the
			// target still resolves. GitHub reports previous_filename for a COPY
			// too, where nothing moved — recording it there would show an
			// unmoved file as renamed and, worse, hand a patch-less entry the
			// review scope that metadataOnlySymlinkLocations grants a move.
			oldPath = file.PreviousFilename
		}
		changedFiles = append(changedFiles, model.ChangedFile{
			Path:      file.Filename,
			Status:    status,
			Additions: file.Additions,
			Deletions: file.Deletions,
			OldPath:   oldPath,
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
		// The head SHA identifies the post-change side this diff describes. The
		// files API reports no file modes, so a symlink can only be recognized by
		// asking that exact tree; a base SHA is deliberately not set, because the
		// API diffs against the merge base, which it does not report.
		DiffHeadSHA: pr.Head.SHA,
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

// baseFileResponse is the subset of the contents API payload needed to read a
// small text file.
type baseFileResponse struct {
	Type     string `json:"type"`
	Encoding string `json:"encoding"`
	Size     int    `json:"size"`
	Content  string `json:"content"`
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
	endpoint := fmt.Sprintf("/repos/%s/contents/%s?ref=%s", escapeRepo(baseRepo), escapePath(path), url.QueryEscape(ref))
	var file baseFileResponse
	if err := c.Get(ctx, endpoint, &file); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	if file.Type != "file" {
		return nil, false, fmt.Errorf("github: %s at %s is a %s, not a file", path, ref, file.Type)
	}
	if file.Size > model.MaxBaseFileBytes {
		return nil, false, fmt.Errorf("github: %s at %s exceeds %d bytes", path, ref, model.MaxBaseFileBytes)
	}
	// GitHub serves anything over ~1 MiB with an empty body and encoding
	// "none"; the size check above already rejects those, so any other encoding
	// is an API change rather than a large file.
	if file.Encoding != "base64" {
		return nil, false, fmt.Errorf("github: %s at %s: unsupported content encoding %q", path, ref, file.Encoding)
	}
	// The payload is wrapped at 60 columns, which base64.StdEncoding rejects.
	decoded, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(file.Content), ""))
	if err != nil {
		return nil, false, fmt.Errorf("github: decoding %s at %s: %w", path, ref, err)
	}
	return decoded, true, nil
}

// escapePath escapes a repository-relative path for a URL path segment list,
// keeping the separators intact.
func escapePath(path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
