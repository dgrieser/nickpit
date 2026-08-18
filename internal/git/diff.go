package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/model"
)

// stableDiffContextLines pins how many context lines every patch nickpit reads
// carries. git's own default is 3, but `diff.context` in the user's global
// configuration or in the reviewed repository silently overrides it — and the
// hunk windows of that patch are what the diff-scope gate, the finding
// fingerprints and the SCM comment positions are all derived from, so a
// machine-local setting would decide which findings survive a review. 3 is also
// exactly what the GitHub and GitLab diff APIs serve, which are not
// configurable at all; pinning it here is what keeps a local review and a
// remote review of the same change comparable.
const stableDiffContextLines = 3

// stableDiffArgs make a patch independent of configuration. Every flag
// neutralizes one setting that would otherwise reach ParseUnifiedDiffFormats:
//
//   - -U pins the context window against `diff.context`.
//   - --no-color: `color.ui = always` emits ANSI escapes even when stdout is a
//     pipe, which the parser would read as part of the content.
//   - --no-ext-diff: `diff.external` replaces the patch with a foreign tool's
//     output, which need not be a unified diff at all.
//   - --no-textconv: a gitattributes textconv filter rewrites file content
//     before it is diffed.
//   - --src-prefix/--dst-prefix: `diff.noprefix` and `diff.mnemonicPrefix`
//     change the "diff --git a/x b/x" framing the parser attributes hunks by.
//
// They belong on every command that emits a patch, and only on those: the
// `--raw --numstat` listings carry plain paths and no content, so none of these
// settings can reach them.
var stableDiffArgs = []string{
	fmt.Sprintf("-U%d", stableDiffContextLines),
	"--no-color",
	"--no-ext-diff",
	"--no-textconv",
	"--src-prefix=a/",
	"--dst-prefix=b/",
}

// patchArgs builds a patch-emitting git invocation: the subcommand, the
// configuration-independence flags, then the caller's own arguments.
func patchArgs(subcommand string, rest ...string) []string {
	args := make([]string, 0, 1+len(stableDiffArgs)+len(rest))
	args = append(args, subcommand)
	args = append(args, stableDiffArgs...)
	return append(args, rest...)
}

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
	diffFiles, hunks, files, err := ParseUnifiedDiffFormatsWithModes(diff, s.fileModesForRequest(ctx, resolvedReq))
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
		DiffFiles:    diffFiles,
		DiffHunks:    hunks,
	}, nil
}

func (s *LocalSource) resolveDefaults(ctx context.Context, req model.ReviewRequest) (model.ReviewRequest, error) {
	if req.Submode == "uncommitted" || req.Submode == "staged" || req.Submode == "unstaged" {
		req.HeadRef = req.Submode
		if req.BaseRef == "" {
			if branch, err := s.currentBranch(ctx); err == nil && branch != "" {
				req.BaseRef = branch
			} else {
				req.BaseRef = "HEAD"
			}
		}
		return req, nil
	}
	if req.Submode != "branch" {
		return req, nil
	}

	if req.BaseRef == "" {
		baseRef, err := s.defaultBranch(ctx)
		if err != nil {
			return req, err
		}
		req.BaseRef = baseRef
	} else if remoteRef, ok := s.originRemoteRef(ctx, req.BaseRef); ok {
		req.BaseRef = remoteRef
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
	return strings.TrimSpace(out), nil
}

func (s *LocalSource) originRemoteRef(ctx context.Context, ref string) (string, bool) {
	if ref == "" || strings.Contains(ref, "/") {
		return "", false
	}
	remoteRef := "origin/" + ref
	_, err := s.git.Run(ctx, "show-ref", "--verify", "--quiet", "refs/remotes/"+remoteRef)
	return remoteRef, err == nil
}

func (s *LocalSource) currentBranch(ctx context.Context) (string, error) {
	out, err := s.git.Run(ctx, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// diffRevArgs renders the revision selection of a request: what the patch and
// the raw listing below must both diff, so their entries describe one comparison.
func diffRevArgs(req model.ReviewRequest) ([]string, error) {
	switch req.Submode {
	case "uncommitted":
		return []string{"HEAD"}, nil
	case "staged":
		return []string{"--cached"}, nil
	case "unstaged":
		return nil, nil
	case "commits":
		if req.BaseRef == "" || req.HeadRef == "" {
			return nil, fmt.Errorf("git: commits mode requires --from and --to")
		}
		return []string{req.BaseRef + ".." + req.HeadRef}, nil
	case "branch":
		if req.BaseRef == "" || req.HeadRef == "" {
			return nil, fmt.Errorf("git: branch mode requires --base and --head")
		}
		return []string{req.BaseRef + "..." + req.HeadRef}, nil
	default:
		return nil, fmt.Errorf("git: unknown submode %q", req.Submode)
	}
}

func (s *LocalSource) diffForRequest(ctx context.Context, req model.ReviewRequest) (string, error) {
	revArgs, err := diffRevArgs(req)
	if err != nil {
		return "", err
	}
	return s.git.Run(ctx, patchArgs("diff", revArgs...)...)
}

// fileModesForRequest lists the post-change file mode of every changed path of
// the same comparison the patch covers. A patch states a mode only when it is
// new, gone, or changed, so a plain rename of an unchanged symlink carries no
// 120000 anywhere — and moving a relative symlink is exactly the kind of change
// whose target can break. The raw listing carries paths and modes but no content,
// so none of the stableDiffArgs settings can reach it.
//
// A failing listing yields no modes rather than a guess: the patch's own mode
// headers still stand, only the sections git left silent stay unmarked.
func (s *LocalSource) fileModesForRequest(ctx context.Context, req model.ReviewRequest) FileModes {
	revArgs, err := diffRevArgs(req)
	if err != nil {
		return nil
	}
	args := append([]string{"diff", "--raw", "-z"}, revArgs...)
	out, err := s.git.Run(ctx, args...)
	if err != nil {
		return nil
	}
	return ParseRawFileModes(out)
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
	args := []string{"log", "--format=%H%x1f%an%x1f%aI%x1f%s", rangeArg}
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
		date, _ := time.Parse(time.RFC3339, parts[2])
		commits = append(commits, model.CommitSummary{
			SHA:     parts[0],
			Author:  parts[1],
			Date:    date,
			Message: parts[3],
		})
	}
	return commits
}

func localTitle(req model.ReviewRequest) string {
	switch req.Submode {
	case "uncommitted":
		return "Local uncommitted changes"
	case "staged":
		return "Local staged changes"
	case "unstaged":
		return "Local unstaged changes"
	case "commits":
		return fmt.Sprintf("Local review for %s..%s", req.BaseRef, req.HeadRef)
	default:
		return fmt.Sprintf("Local branch review for %s...%s", req.BaseRef, req.HeadRef)
	}
}

func localDescription(req model.ReviewRequest) string {
	return fmt.Sprintf("Local %s review generated from git diff.", req.Submode)
}
