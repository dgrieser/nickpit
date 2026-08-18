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

// stampSymlinkFlags propagates symlink marks between the two file views and
// closes the gap for SCM sources whose diff carries no file mode. Diff-derived
// flags come from the git mode header (local diffs and, since the modes are
// plumbed through, GitLab MRs); GitHub's files API reports neither a mode nor a
// mode line in `patch`, so there a symlink blob is indistinguishable from a
// one-line text file unless the checkout is asked directly.
//
// A symlink's blob content is the link target path, so reviewing it as text
// produces findings whose fix would rewrite or break the link.
func stampSymlinkFlags(reviewCtx *model.ReviewContext) {
	if reviewCtx == nil {
		return
	}
	symlink := make(map[string]bool, len(reviewCtx.DiffFiles)+len(reviewCtx.ChangedFiles))
	for _, file := range reviewCtx.DiffFiles {
		if file.Symlink {
			symlink[normalizeReviewPath(file.FilePath)] = true
		}
	}
	for _, file := range reviewCtx.ChangedFiles {
		if file.Symlink {
			symlink[normalizeReviewPath(file.Path)] = true
		}
	}
	// The checkout probe cannot see a deleted symlink (the path is gone), so
	// mode-less sources keep missing that case; the diff header covers it
	// wherever the SCM reports modes.
	mark := func(path string) bool {
		key := normalizeReviewPath(path)
		if symlink[key] {
			return true
		}
		if isCheckoutSymlink(reviewCtx.CheckoutRoot, path) {
			symlink[key] = true
			return true
		}
		return false
	}
	for i := range reviewCtx.ChangedFiles {
		file := &reviewCtx.ChangedFiles[i]
		file.Symlink = mark(file.Path)
	}
	for i := range reviewCtx.DiffFiles {
		file := &reviewCtx.DiffFiles[i]
		file.Symlink = mark(file.FilePath)
	}
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
