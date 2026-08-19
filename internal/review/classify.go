package review

import (
	"context"

	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/git"
	"github.com/dgrieser/nickpit/internal/model"
)

// stampGeneratedFlags marks generated changed files. DiffFiles carry
// parser-stamped, content-aware flags; ChangedFiles built directly from SCM
// APIs (GitHub/GitLab adapters) are stamped here so every source is covered.
func stampGeneratedFlags(reviewCtx *model.ReviewContext) {
	if reviewCtx == nil {
		return
	}
	byPath := make(map[string]bool, len(reviewCtx.DiffFiles))
	for _, file := range reviewCtx.DiffFiles {
		byPath[normalizeReviewPath(file.FilePath)] = file.Generated
	}
	for i := range reviewCtx.ChangedFiles {
		file := &reviewCtx.ChangedFiles[i]
		if generated, ok := byPath[normalizeReviewPath(file.Path)]; ok {
			file.Generated = generated
			continue
		}
		file.Generated = filetype.IsGenerated(file.Path, "")
	}
}

// stampSymlinkFlags fills in symlink marks for review sources whose diff carries
// no git file mode. Local diffs carry mode headers (with a "git diff --raw"
// fallback for the sections git leaves silent) and GitLab MRs report
// a_mode/b_mode, so their entries are already marked per entry by the diff parser
// and the adapter. Those marks stay untouched here, and deliberately are not
// spread across a path: a symlink replaced by a regular file at the same path
// arrives as two entries — a mode-120000 deletion plus a mode-100644 addition —
// whose marks legitimately differ, and marking the addition would hide real text
// from review.
//
// GitHub's pull-request files API reports neither a mode field nor a mode line
// inside `patch`, so there the reviewed head commit's tree is asked. The lookup
// is addressed by SHA, never "whatever the checkout currently holds": a chat
// session resumed with --repo-root points at a user-selected working copy that
// may carry local edits or another revision entirely, and a tree that a checkout
// does not have simply fails the lookup.
//
// A checkout is not always present, and a deleted path is not in the head tree,
// so GitHub keeps blind spots; the trade is a missing mark, never a wrong one.
// A symlink's blob content is the link target path, so reviewing it as text
// produces findings whose fix would rewrite or break the link.
func stampSymlinkFlags(ctx context.Context, reviewCtx *model.ReviewContext, runner git.Runner) {
	if reviewCtx == nil || reviewCtx.CheckoutRoot == "" || reviewCtx.DiffHeadSHA == "" ||
		!sourceOmitsFileModes(reviewCtx.Mode) {
		return
	}
	paths := make([]string, 0, len(reviewCtx.ChangedFiles)+len(reviewCtx.DiffFiles))
	seen := make(map[string]bool, cap(paths))
	collect := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	for _, file := range reviewCtx.ChangedFiles {
		if !file.Symlink {
			collect(file.Path)
		}
	}
	for _, file := range reviewCtx.DiffFiles {
		if !file.Symlink {
			collect(file.FilePath)
		}
	}
	for _, hunk := range reviewCtx.DiffHunks {
		if !hunk.Symlink {
			collect(hunk.FilePath)
		}
	}
	// The error is deliberately dropped: an unreadable tree means no marks, and
	// the change is reviewed either way.
	marks, _ := git.SymlinkPathsAtRev(ctx, runner, reviewCtx.DiffHeadSHA, paths)
	if len(marks) == 0 {
		return
	}
	// The keys stay literal git paths, exactly as ls-tree and the SCM payload
	// spell them. Normalizing would fold distinct legal names together — a
	// symlink named `a\b` and a regular file `a/b` are two different files on
	// Unix — and the symlink's mark would then suppress the other file's text.
	for i := range reviewCtx.ChangedFiles {
		file := &reviewCtx.ChangedFiles[i]
		if !file.Symlink {
			file.Symlink = marks[file.Path]
		}
	}
	for i := range reviewCtx.DiffFiles {
		file := &reviewCtx.DiffFiles[i]
		if !file.Symlink {
			file.Symlink = marks[file.FilePath]
		}
	}
	// The git-json diff format drops DiffFiles, so an unstamped hunk would carry
	// a link target with no marker at all.
	for i := range reviewCtx.DiffHunks {
		hunk := &reviewCtx.DiffHunks[i]
		if !hunk.Symlink {
			hunk.Symlink = marks[hunk.FilePath]
		}
	}
}

// sourceOmitsFileModes reports whether a review source's diff carries no git file
// mode at all, so a symlink cannot be recognized from the diff alone and the
// reviewed tree has to be asked. Sources that do report modes are never
// second-guessed: their marks describe the reviewed revision, which a checkout
// need not match.
func sourceOmitsFileModes(mode model.ReviewMode) bool {
	return mode == model.ModeGitHub
}
