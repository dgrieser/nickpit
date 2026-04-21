package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

type LocalSource struct {
	repoRoot string
	git      Runner
}

func NewLocalSource(repoRoot string) *LocalSource {
	return &LocalSource{
		repoRoot: repoRoot,
		git:      ExecRunner{RepoRoot: repoRoot},
	}
}

func (s *LocalSource) ResolveContext(ctx context.Context, req model.ReviewRequest) (*model.ReviewContext, error) {
	resolvedReq, err := s.resolveDefaults(ctx, req)
	if err != nil {
		return nil, err
	}

	diff, err := s.diffForRequest(ctx, resolvedReq)
	if err != nil {
		return nil, err
	}
	hunks, files, err := ParseUnifiedDiff(diff)
	if err != nil {
		return nil, err
	}
	commits, err := s.commitSummaries(ctx, resolvedReq)
	if err != nil {
		return nil, err
	}
	repoName := filepath.Base(s.repoRoot)
	return &model.ReviewContext{
		Mode: resolvedReq.Mode,
		Repository: model.RepositoryInfo{
			FullName: repoName,
			BaseRef:  resolvedReq.BaseRef,
			HeadRef:  resolvedReq.HeadRef,
		},
		Title:        localTitle(resolvedReq),
		Description:  localDescription(resolvedReq),
		Commits:      commits,
		ChangedFiles: files,
		Diff:         diff,
		DiffHunks:    hunks,
	}, nil
}

func (s *LocalSource) resolveDefaults(ctx context.Context, req model.ReviewRequest) (model.ReviewRequest, error) {
	if req.Submode != "branch" {
		return req, nil
	}

	if req.BaseRef == "" {
		baseRef, err := s.defaultBranch(ctx)
		if err != nil {
			return req, err
		}
		req.BaseRef = baseRef
	}
	if req.HeadRef == "HEAD" {
		headRef, err := s.currentBranch(ctx)
		if err != nil {
			return req, err
		}
		req.HeadRef = headRef
	}
	return req, nil
}

func (s *LocalSource) defaultBranch(ctx context.Context) (string, error) {
	out, err := s.git.Run(ctx, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(strings.TrimSpace(out), "origin/"), nil
}

func (s *LocalSource) currentBranch(ctx context.Context) (string, error) {
	out, err := s.git.Run(ctx, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *LocalSource) diffForRequest(ctx context.Context, req model.ReviewRequest) (string, error) {
	switch req.Submode {
	case "uncommitted":
		return s.git.Run(ctx, "diff", "HEAD")
	case "commits":
		if req.BaseRef == "" || req.HeadRef == "" {
			return "", fmt.Errorf("git: commits mode requires --from and --to")
		}
		return s.git.Run(ctx, "diff", req.BaseRef+".."+req.HeadRef)
	case "branch":
		if req.BaseRef == "" || req.HeadRef == "" {
			return "", fmt.Errorf("git: branch mode requires --base and --head")
		}
		return s.git.Run(ctx, "diff", req.BaseRef+"..."+req.HeadRef)
	default:
		return "", fmt.Errorf("git: unknown submode %q", req.Submode)
	}
}

func (s *LocalSource) commitSummaries(ctx context.Context, req model.ReviewRequest) ([]model.CommitSummary, error) {
	if !req.IncludeCommits {
		return nil, nil
	}
	var rangeArg string
	switch req.Submode {
	case "commits", "branch":
		rangeArg = req.BaseRef + ".." + req.HeadRef
	default:
		rangeArg = "-5"
	}
	args := []string{"log", "--format=%H%x1f%an%x1f%aI%x1f%s"}
	if strings.HasPrefix(rangeArg, "-") {
		args = append(args, rangeArg)
	} else {
		args = append(args, rangeArg)
	}
	out, err := s.git.Run(ctx, args...)
	if err != nil {
		return nil, nil
	}
	return parseCommits(out), nil
}

func parseCommits(out string) []model.CommitSummary {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	commits := make([]model.CommitSummary, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\x1f")
		if len(parts) != 4 {
			continue
		}
		commits = append(commits, model.CommitSummary{
			SHA:     parts[0],
			Author:  parts[1],
			Message: parts[3],
		})
	}
	return commits
}

func localTitle(req model.ReviewRequest) string {
	switch req.Submode {
	case "uncommitted":
		return "Local uncommitted changes"
	case "commits":
		return fmt.Sprintf("Local review for %s..%s", req.BaseRef, req.HeadRef)
	default:
		return fmt.Sprintf("Local branch review for %s...%s", req.BaseRef, req.HeadRef)
	}
}

func localDescription(req model.ReviewRequest) string {
	return fmt.Sprintf("Local %s review generated from git diff.", req.Submode)
}
