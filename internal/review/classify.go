package review

import (
	"os"

	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval/repofs"
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
// no git file mode. Local diffs carry mode header lines and GitLab MRs report
// a_mode/b_mode, so their entries are already marked per entry by the diff parser
// and the adapter. Those marks stay untouched here, and deliberately are not
// spread across a path: a symlink replaced by a regular file at the same path
// arrives as two entries — a mode-120000 deletion plus a mode-100644 addition —
// whose marks legitimately differ, and marking the addition would hide real text
// from review.
//
// GitHub's pull-request files API reports neither a mode field nor a mode line
// inside `patch`, so there the checkout is the only remaining source of truth.
// It is a temporary clone of the reviewed head revision, i.e. the post-change
// side the marks describe. A checkout is not always present, and a deleted
// symlink cannot be probed at all (the path is gone), so GitHub keeps blind
// spots; the trade is a missing mark, never a wrong one.
//
// A symlink's blob content is the link target path, so reviewing it as text
// produces findings whose fix would rewrite or break the link.
func stampSymlinkFlags(reviewCtx *model.ReviewContext) {
	if reviewCtx == nil || reviewCtx.CheckoutRoot == "" || !sourceOmitsFileModes(reviewCtx.Mode) {
		return
	}
	probed := make(map[string]bool, len(reviewCtx.ChangedFiles))
	probe := func(path string) bool {
		key := normalizeReviewPath(path)
		symlink, ok := probed[key]
		if !ok {
			symlink = isCheckoutSymlink(reviewCtx.CheckoutRoot, path)
			probed[key] = symlink
		}
		return symlink
	}
	for i := range reviewCtx.ChangedFiles {
		file := &reviewCtx.ChangedFiles[i]
		if !file.Symlink {
			file.Symlink = probe(file.Path)
		}
	}
	for i := range reviewCtx.DiffFiles {
		file := &reviewCtx.DiffFiles[i]
		if !file.Symlink {
			file.Symlink = probe(file.FilePath)
		}
	}
}

// sourceOmitsFileModes reports whether a review source's diff carries no git file
// mode at all, so a symlink cannot be recognized from the diff alone and the
// checkout has to be asked. Probing a source that does report modes would be
// worse than useless: the worktree can hold a different revision than the diff
// (staged content, or a base..head range that is not checked out), so an
// authoritative regular mode could be overwritten with an unrelated current
// symlink and suppress valid text review.
func sourceOmitsFileModes(mode model.ReviewMode) bool {
	return mode == model.ModeGitHub
}

// isCheckoutSymlink reports whether path is a symlink inside the checkout.
// The path is resolved lexically so an escaping path is rejected, and lstat
// deliberately does not follow the link: following it would resolve the target
// and hide exactly the fact being probed.
func isCheckoutSymlink(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	_, fullPath, err := repofs.ResolvePath(root, path)
	if err != nil || fullPath == "" {
		return false
	}
	info, err := os.Lstat(fullPath)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
