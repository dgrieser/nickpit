package gitlab

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
)

type mrResponse struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	WebURL          string `json:"web_url"`
	SHA             string `json:"sha"`
	State           string `json:"state"`
	Draft           bool   `json:"draft"`
	SourceBranch    string `json:"source_branch"`
	TargetBranch    string `json:"target_branch"`
	SourceProjectID int    `json:"source_project_id"`
	TargetProjectID int    `json:"target_project_id"`
	DiffRefs        struct {
		BaseSHA  string `json:"base_sha"`
		HeadSHA  string `json:"head_sha"`
		StartSHA string `json:"start_sha"`
	} `json:"diff_refs"`
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
	// Overflow is set by GitLab when the MR exceeds the diff size/file-count
	// limits and the returned change set is truncated.
	Overflow bool `json:"overflow"`
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

type projectResponse struct {
	HTTPURLToRepo string `json:"http_url_to_repo"`
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
				// Strip hidden nickpit markers before the body enters prompt
				// context: the carrier payloads are large opaque blobs that would
				// waste model tokens and displace real comments during trimming.
				// Notes that were only carriers are dropped entirely.
				body := reviewmd.StripMarkers(note.Body)
				if body == "" {
					continue
				}
				comment := model.Comment{
					Author:    note.Author.Username,
					Body:      body,
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
	diffText := diff.String()
	diffFiles, hunks, _, _ := git.ParseUnifiedDiffFormats(diffText)
	var omitted []string
	if changes.Overflow {
		omitted = append(omitted, "WARNING: GitLab reported this MR's diff as truncated (overflow); the review is based on a partial diff and may miss changes")
	}
	return &model.ReviewContext{
		Mode:       model.ModeGitLab,
		Identifier: iid,
		Repository: model.RepositoryInfo{
			FullName: project,
			BaseRef:  mr.TargetBranch,
			HeadRef:  mr.SourceBranch,
			URL:      mr.WebURL,
		},
		Title:           mr.Title,
		Description:     mr.Description,
		Commits:         normalizeMRCommits(commits),
		ChangedFiles:    changedFiles,
		Diff:            diffText,
		DiffFiles:       diffFiles,
		DiffHunks:       hunks,
		Comments:        comments,
		OmittedSections: omitted,
	}, nil
}

func (c *Client) FetchMRCheckout(ctx context.Context, project string, iid int) (*model.CheckoutSpec, error) {
	escaped := escapeProject(project)
	var mr mrResponse
	if err := c.Get(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d", escaped, iid), &mr); err != nil {
		return nil, err
	}
	sourceProject := mr.SourceProjectID
	if sourceProject == 0 {
		sourceProject = mr.TargetProjectID
	}
	cloneURL, err := c.projectCloneURL(ctx, sourceProject, escaped)
	if err != nil {
		return nil, err
	}
	return &model.CheckoutSpec{
		Provider: model.ModeGitLab,
		Repo:     project,
		CloneURL: cloneURL,
		HeadRef:  mr.SourceBranch,
		HeadSHA:  mr.SHA,
	}, nil
}

func (c *Client) projectCloneURL(ctx context.Context, projectID int, fallbackProject string) (string, error) {
	var path string
	if projectID > 0 {
		path = fmt.Sprintf("/projects/%d", projectID)
	} else {
		path = fmt.Sprintf("/projects/%s", fallbackProject)
	}
	var project projectResponse
	if err := c.Get(ctx, path, &project); err != nil {
		return "", err
	}
	return project.HTTPURLToRepo, nil
}

// MRStatus is the minimal live state of an MR, fetched by the serve daemon
// right before starting a review so closed/merged/draft MRs are skipped on
// authoritative data rather than a possibly stale webhook payload.
type MRStatus struct {
	State   string
	Draft   bool
	HeadSHA string
}

// FetchMRStatus fetches an MR's current state by numeric project ID.
func (c *Client) FetchMRStatus(ctx context.Context, projectID, iid int) (*MRStatus, error) {
	return c.fetchMRStatus(ctx, strconv.Itoa(projectID), iid)
}

// FetchMRStatusByPath fetches an MR's current state by project path (group/name).
// The chat command uses it to detect that an MR gained commits since a session's
// cached context was built, so the diff can be recreated against the new head.
func (c *Client) FetchMRStatusByPath(ctx context.Context, project string, iid int) (*MRStatus, error) {
	return c.fetchMRStatus(ctx, escapeProject(project), iid)
}

func (c *Client) fetchMRStatus(ctx context.Context, escapedProject string, iid int) (*MRStatus, error) {
	var mr mrResponse
	if err := c.Get(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d", escapedProject, iid), &mr); err != nil {
		return nil, err
	}
	return &MRStatus{State: mr.State, Draft: mr.Draft, HeadSHA: mr.SHA}, nil
}

// DiffRefs holds the three commit SHAs GitLab requires in a diff-note position.
type DiffRefs struct {
	BaseSHA  string
	HeadSHA  string
	StartSHA string
}

// MRChange is one changed file plus its parsed hunks, used to anchor review
// comments to diff lines.
type MRChange struct {
	NewPath string
	OldPath string
	NewFile bool
	Deleted bool
	Renamed bool
	Hunks   []model.DiffHunk
}

// MRPositionInfo is the freshly-fetched diff state used when publishing review
// comments back to an MR. It is fetched at post-time (not reused from the review
// context) so the SHAs match the current diff and GitLab accepts the positions.
type MRPositionInfo struct {
	DiffRefs DiffRefs
	Changes  []MRChange
}

// FetchMRPositionInfo fetches the MR's current diff_refs and per-file changes so
// review findings can be anchored to exact diff positions.
func (c *Client) FetchMRPositionInfo(ctx context.Context, project string, iid int) (*MRPositionInfo, error) {
	escaped := escapeProject(project)
	var mr mrResponse
	if err := c.Get(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d", escaped, iid), &mr); err != nil {
		return nil, err
	}
	var changes changesResponse
	if err := c.Get(ctx, fmt.Sprintf("/projects/%s/merge_requests/%d/changes", escaped, iid), &changes); err != nil {
		return nil, err
	}
	out := &MRPositionInfo{
		DiffRefs: DiffRefs{
			BaseSHA:  mr.DiffRefs.BaseSHA,
			HeadSHA:  mr.DiffRefs.HeadSHA,
			StartSHA: mr.DiffRefs.StartSHA,
		},
		Changes: make([]MRChange, 0, len(changes.Changes)),
	}
	for _, change := range changes.Changes {
		newPath := change.NewPath
		if newPath == "" {
			newPath = change.OldPath
		}
		oldPath := change.OldPath
		if oldPath == "" {
			oldPath = newPath
		}
		// Re-create the minimal "diff --git" framing FetchMR uses so the parser
		// attributes hunks to this file; parse per file so rename/old-path stays
		// local to the change.
		var framed strings.Builder
		framed.WriteString("diff --git a/")
		framed.WriteString(newPath)
		framed.WriteString(" b/")
		framed.WriteString(newPath)
		framed.WriteByte('\n')
		framed.WriteString(change.Diff)
		framed.WriteByte('\n')
		hunks, _, _ := git.ParseUnifiedDiff(framed.String())
		out.Changes = append(out.Changes, MRChange{
			NewPath: newPath,
			OldPath: oldPath,
			NewFile: change.NewFile,
			Deleted: change.Deleted,
			Renamed: change.Renamed,
			Hunks:   hunks,
		})
	}
	return out, nil
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
