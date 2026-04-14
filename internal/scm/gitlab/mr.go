package gitlab

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
)

type mrResponse struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	WebURL       string `json:"web_url"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
}

type commitResponse struct {
	ID         string    `json:"id"`
	Message    string    `json:"message"`
	AuthorName string    `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type changesResponse struct {
	Changes []struct {
		NewPath string `json:"new_path"`
		OldPath string `json:"old_path"`
		NewFile bool   `json:"new_file"`
		Deleted bool   `json:"deleted_file"`
		Renamed bool   `json:"renamed_file"`
		Diff    string `json:"diff"`
	} `json:"changes"`
}

type discussionsResponse []struct {
	ID    string `json:"id"`
	Notes []struct {
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"created_at"`
		Author    struct {
			Username string `json:"username"`
		} `json:"author"`
		Position *struct {
			NewPath string `json:"new_path"`
			NewLine int    `json:"new_line"`
		} `json:"position"`
	} `json:"notes"`
}

func (c *Client) FetchMR(ctx context.Context, project string, iid int, includeComments bool) (*model.ReviewContext, error) {
	escaped := escapeProject(project)
	var mr mrResponse
	if err := c.Get(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d", escaped, iid), &mr); err != nil {
		return nil, err
	}
	var commits []commitResponse
	if err := c.GetPaginated(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/commits", escaped, iid), &commits); err != nil {
		return nil, err
	}
	var changes changesResponse
	if err := c.Get(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/changes", escaped, iid), &changes); err != nil {
		return nil, err
	}

	var comments []model.Comment
	if includeComments {
		var discussions discussionsResponse
		_ = c.GetPaginated(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/discussions", escaped, iid), &discussions)
		for _, discussion := range discussions {
			for _, note := range discussion.Notes {
				comment := model.Comment{
					Author:    note.Author.Username,
					Body:      note.Body,
					CreatedAt: note.CreatedAt,
					ThreadID:  discussion.ID,
				}
				if note.Position != nil {
					comment.Path = note.Position.NewPath
					comment.Line = note.Position.NewLine
				}
				comments = append(comments, comment)
			}
		}
	}

	var diff strings.Builder
	changedFiles := make([]model.ChangedFile, 0, len(changes.Changes))
	for _, change := range changes.Changes {
		status := model.FileModified
		if change.NewFile {
			status = model.FileAdded
		} else if change.Deleted {
			status = model.FileDeleted
		} else if change.Renamed {
			status = model.FileRenamed
		}
		path := change.NewPath
		if path == "" {
			path = change.OldPath
		}
		changedFiles = append(changedFiles, model.ChangedFile{
			Path:   path,
			Status: status,
		})
		diff.WriteString("diff --git a/")
		diff.WriteString(path)
		diff.WriteString(" b/")
		diff.WriteString(path)
		diff.WriteByte('\n')
		diff.WriteString(change.Diff)
		diff.WriteByte('\n')
	}
	hunks, _, _ := git.ParseUnifiedDiff(diff.String())
	return &model.ReviewContext{
		Mode: model.ModeGitLab,
		Repository: model.RepositoryInfo{
			FullName: project,
			BaseRef:  mr.TargetBranch,
			HeadRef:  mr.SourceBranch,
			URL:      mr.WebURL,
		},
		Title:        mr.Title,
		Description:  mr.Description,
		Commits:      normalizeMRCommits(commits),
		ChangedFiles: changedFiles,
		Diff:         diff.String(),
		DiffHunks:    hunks,
		Comments:     comments,
	}, nil
}

func normalizeMRCommits(in []commitResponse) []model.CommitSummary {
	out := make([]model.CommitSummary, 0, len(in))
	for _, item := range in {
		out = append(out, model.CommitSummary{
			SHA:     item.ID,
			Message: item.Message,
			Author:  item.AuthorName,
			Date:    item.CreatedAt,
		})
	}
	return out
}
