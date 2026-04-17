package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
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
	Body      string    `json:"body"`
	Path      string    `json:"path"`
	Line      int       `json:"line"`
	Side      string    `json:"side"`
	CreatedAt time.Time `json:"created_at"`
	User      userRef   `json:"user"`
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
		var reviews []reviewResponse
		_ = c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews", escaped, number), &reviews)
		for _, item := range reviews {
			if strings.TrimSpace(item.Body) == "" {
				continue
			}
			comments = append(comments, model.Comment{
				Author:    item.User.Login,
				Body:      item.Body,
				CreatedAt: item.Submitted,
				IsReview:  true,
			})
		}

		var lineComments []commentResponse
		_ = c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/pulls/%d/comments", escaped, number), &lineComments)
		for _, item := range lineComments {
			comments = append(comments, model.Comment{
				Author:    item.User.Login,
				Body:      item.Body,
				Path:      item.Path,
				Line:      item.Line,
				Side:      item.Side,
				CreatedAt: item.CreatedAt,
				IsReview:  true,
			})
		}

		var issueComments []issueCommentResponse
		_ = c.GetPaginated(ctx, fmt.Sprintf("/repos/%s/issues/%d/comments", escaped, number), &issueComments)
		for _, item := range issueComments {
			comments = append(comments, model.Comment{
				Author:    item.User.Login,
				Body:      item.Body,
				CreatedAt: item.CreatedAt,
			})
		}
	}

	var diff strings.Builder
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
		if file.Patch != "" {
			diff.WriteString("diff --git a/")
			diff.WriteString(file.Filename)
			diff.WriteString(" b/")
			diff.WriteString(file.Filename)
			diff.WriteByte('\n')
			diff.WriteString(file.Patch)
			diff.WriteByte('\n')
		}
	}
	hunks, _, _ := git.ParseUnifiedDiff(diff.String())
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
		Diff:         diff.String(),
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
