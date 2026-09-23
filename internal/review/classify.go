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

// stampSymlinkFlags marks symlink entries and fills in the metadata a symlink
// change needs to be reviewable, for the sources whose diff cannot supply it.
//
// Marks: local diffs carry mode headers (with a "git diff --raw" fallback for the
// sections git leaves silent) and GitLab MRs report a_mode/b_mode, so their entries
// arrive already marked per entry by the diff parser and the adapter. Those marks
// stay untouched, and deliberately are not spread across a path: a symlink replaced
// by a regular file at the same path arrives as two entries — a mode-120000
// deletion plus a mode-100644 addition — whose marks legitimately differ, and
// marking the addition would hide real text from review. GitHub's pull-request
// files API reports neither a mode field nor a mode line inside `patch`, so there
// the reviewed head commit's tree is asked.
//
// A deleted path is not in that tree, and the removed side of its patch is exactly
// where a removed link target sits — so for those entries the change's own commits
// are asked instead, and only a mode they all agree on is taken (see
// git.StableFileModes). GitHub reports one entry per path, so a deletion mark
// cannot bleed into a same-path addition the way a diff-derived mark could.
//
// Targets: a pure symlink rename emits no hunk and no content in ANY source, so the
// link target is nowhere in the patch — yet whether a relative target still resolves
// from the new directory is the whole question such a change raises. The target is
// therefore read from the blob the head tree names.
//
// Every lookup is addressed by SHA, never "whatever the checkout currently holds":
// a chat session resumed with --repo-root points at a user-selected working copy
// that may carry local edits or another revision entirely, and a tree that a
// checkout does not have simply fails the lookup. A checkout is not always present
// either, so blind spots remain; the trade is missing metadata, never wrong
// metadata.
func stampSymlinkFlags(ctx context.Context, reviewCtx *model.ReviewContext, runner git.Runner) {
	if reviewCtx == nil || reviewCtx.CheckoutRoot == "" || reviewCtx.DiffHeadSHA == "" {
		return
	}
	markFromTree := reviewCtx.DiffOmitsFileModes
	hasHunk := make(map[string]bool, len(reviewCtx.DiffHunks))
	for _, hunk := range reviewCtx.DiffHunks {
		hasHunk[hunk.FilePath] = true
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
	var deletedPaths []string
	deletedSeen := make(map[string]bool, len(reviewCtx.ChangedFiles))
	collectDeleted := func(path string) {
		if path == "" || deletedSeen[path] {
			return
		}
		deletedSeen[path] = true
		deletedPaths = append(deletedPaths, path)
	}
	for _, file := range reviewCtx.ChangedFiles {
		switch {
		case markFromTree && !file.Symlink && file.Status == model.FileDeleted:
			// The reviewed tree no longer holds this path; its deletion does.
			collectDeleted(file.Path)
		case markFromTree && !file.Symlink:
			collect(file.Path)
		case file.Symlink && file.SymlinkTarget == "" && !hasHunk[file.Path]:
			// Nothing in the patch shows this symlink's target.
			collect(file.Path)
		}
	}
	if markFromTree {
		// The diff views carry no status, so a deleted path is recognized by the
		// entry that named it above. Asking the reviewed tree for it would be
		// wasted work: the deletion listing is what answers for those.
		for _, file := range reviewCtx.DiffFiles {
			if !file.Symlink && !deletedSeen[file.FilePath] {
				collect(file.FilePath)
			}
		}
		for _, hunk := range reviewCtx.DiffHunks {
			if !hunk.Symlink && !deletedSeen[hunk.FilePath] {
				collect(hunk.FilePath)
			}
		}
	}
	// The errors are deliberately dropped: unreadable history means no metadata, and
	// the change is reviewed either way.
	blobs, _ := git.SymlinkPathsAtRev(ctx, runner, reviewCtx.DiffHeadSHA, paths)
	deletedLinks := deletedSymlinkPaths(ctx, runner, reviewCtx, deletedPaths)
	if len(blobs) == 0 && len(deletedLinks) == 0 {
		return
	}
	// The keys stay literal git paths, exactly as ls-tree and the SCM payload
	// spell them. Normalizing would fold distinct legal names together — a
	// symlink named `a\b` and a regular file `a/b` are two different files on
	// Unix — and the symlink's mark would then suppress the other file's text.
	if markFromTree {
		for i := range reviewCtx.ChangedFiles {
			file := &reviewCtx.ChangedFiles[i]
			if file.Symlink {
				continue
			}
			if file.Status == model.FileDeleted {
				file.Symlink = deletedLinks[file.Path]
				continue
			}
			_, file.Symlink = blobs[file.Path]
		}
		// The diff views carry no status, so both listings answer for them.
		for i := range reviewCtx.DiffFiles {
			file := &reviewCtx.DiffFiles[i]
			if !file.Symlink {
				file.Symlink = markedSymlink(blobs, deletedLinks, file.FilePath)
			}
		}
		// The git-json diff format drops DiffFiles, so an unstamped hunk would
		// carry a link target with no marker at all.
		for i := range reviewCtx.DiffHunks {
			hunk := &reviewCtx.DiffHunks[i]
			if !hunk.Symlink {
				hunk.Symlink = markedSymlink(blobs, deletedLinks, hunk.FilePath)
			}
		}
	}
	// Same reading and same skip rules as a local diff; only the blob names come
	// from the tree lookup above instead of a raw listing.
	git.AttachSymlinkTargets(ctx, runner, reviewCtx.ChangedFiles, reviewCtx.DiffHunks, func(path string) string {
		return blobs[path]
	})
}

// deletedSymlinkPaths reports which of paths were symlinks on the pre-change side,
// for paths the change deletes. The commit list is the bound on the lookup: the
// deletion under review is in one of them, and nothing else is examined. A path
// whose mode is not the same throughout that range is left out rather than guessed
// at — see git.StableFileModes.
func deletedSymlinkPaths(ctx context.Context, runner git.Runner, reviewCtx *model.ReviewContext, paths []string) map[string]bool {
	if len(paths) == 0 || len(reviewCtx.Commits) == 0 {
		return nil
	}
	commits := make([]string, 0, len(reviewCtx.Commits))
	for _, commit := range reviewCtx.Commits {
		if commit.SHA != "" {
			commits = append(commits, commit.SHA)
		}
	}
	modes, _ := git.StableFileModes(ctx, runner, commits, paths)
	if len(modes) == 0 {
		return nil
	}
	links := make(map[string]bool, len(modes))
	for path := range modes {
		if modes.Symlink(path) {
			links[path] = true
		}
	}
	return links
}

// markedSymlink reports whether either listing marks path as a symlink.
func markedSymlink(blobs map[string]string, deletedLinks map[string]bool, path string) bool {
	if _, ok := blobs[path]; ok {
		return true
	}
	return deletedLinks[path]
}
