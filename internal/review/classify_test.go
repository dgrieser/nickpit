package review

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// symlinkTreeRunner answers "git ls-tree -z <rev>" from a fixed set of symlink
// paths, so the stamping tests need no real checkout. It records the revisions it
// was asked for, because the marks are only sound when they come from the
// reviewed head rather than from whatever a checkout holds.
type symlinkTreeRunner struct {
	symlinks  []string
	deleted   []string
	target    string
	revs      []string
	blobReads []string
	logCalls  [][]string
}

func (r *symlinkTreeRunner) Run(_ context.Context, args ...string) (string, error) {
	switch {
	case len(args) >= 3 && args[0] == "ls-tree":
		// The revision is the argument right before the pathspec separator, so
		// this stays correct as the command gains flags.
		if sep := slices.Index(args, "--"); sep > 0 {
			r.revs = append(r.revs, args[sep-1])
		}
		var out strings.Builder
		for _, path := range r.symlinks {
			out.WriteString("120000 blob 32f64f4\t" + path + "\x00")
		}
		return out.String(), nil
	case args[0] == "log":
		r.logCalls = append(r.logCalls, args)
		var out strings.Builder
		for _, path := range r.deleted {
			out.WriteString(":120000 000000 32f64f4 0000000 D\x00" + path + "\x00")
		}
		return out.String(), nil
	case len(args) == 3 && args[0] == "cat-file" && args[1] == "blob":
		r.blobReads = append(r.blobReads, args[2])
		return r.target, nil
	}
	return "", nil
}

// A symlink replaced by a regular file at the same path arrives as two entries
// with legitimately different marks. Spreading the deletion's mark over the path
// would mark the regular-file addition as a symlink, and agents would then skip
// reviewing its real text.
func TestStampSymlinkFlagsKeepsReplacementEntriesDistinct(t *testing.T) {
	reviewCtx := &model.ReviewContext{
		Mode:         model.ModeLocal,
		CheckoutRoot: "/checkout",
		DiffHeadSHA:  "head111",
		ChangedFiles: []model.ChangedFile{
			{Path: "link", Status: model.FileDeleted, Symlink: true},
			{Path: "link", Status: model.FileAdded},
		},
		DiffFiles: []model.DiffFile{
			{FilePath: "link", Content: "deleted file mode 120000\n", Symlink: true},
			{FilePath: "link", Content: "new file mode 100644\n"},
		},
	}

	stampSymlinkFlags(context.Background(), reviewCtx, &symlinkTreeRunner{symlinks: []string{"link"}})

	if !reviewCtx.ChangedFiles[0].Symlink || !reviewCtx.DiffFiles[0].Symlink {
		t.Fatalf("symlink deletion lost its mark: %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0])
	}
	if reviewCtx.ChangedFiles[1].Symlink || reviewCtx.DiffFiles[1].Symlink {
		t.Fatalf("regular-file addition inherited the symlink mark: %#v / %#v", reviewCtx.ChangedFiles[1], reviewCtx.DiffFiles[1])
	}
}

// GitHub's files API reports neither a mode nor a mode header line in `patch`,
// so the reviewed head commit's tree is the only source of truth left for that
// provider — and every representation an agent may read has to be stamped, since
// the git-json diff format drops DiffFiles entirely.
func TestStampSymlinkFlagsMarksEveryViewFromTheHeadTree(t *testing.T) {
	reviewCtx := &model.ReviewContext{
		Mode:               model.ModeGitHub,
		DiffOmitsFileModes: true,
		CheckoutRoot:       "/checkout",
		DiffHeadSHA:        "head111",
		ChangedFiles:       []model.ChangedFile{{Path: "templates"}, {Path: "main.go"}},
		DiffFiles:          []model.DiffFile{{FilePath: "templates"}, {FilePath: "main.go"}},
		DiffHunks:          []model.DiffHunk{{FilePath: "templates"}, {FilePath: "main.go"}},
	}

	runner := &symlinkTreeRunner{symlinks: []string{"templates"}}
	stampSymlinkFlags(context.Background(), reviewCtx, runner)

	if !reviewCtx.ChangedFiles[0].Symlink || !reviewCtx.DiffFiles[0].Symlink || !reviewCtx.DiffHunks[0].Symlink {
		t.Fatalf("symlink not marked in every view: %#v / %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0], reviewCtx.DiffHunks[0])
	}
	if reviewCtx.ChangedFiles[1].Symlink || reviewCtx.DiffFiles[1].Symlink || reviewCtx.DiffHunks[1].Symlink {
		t.Fatalf("regular file marked as symlink: %#v / %#v / %#v", reviewCtx.ChangedFiles[1], reviewCtx.DiffFiles[1], reviewCtx.DiffHunks[1])
	}
	// All views share one lookup, and it targets the reviewed head.
	if len(runner.revs) != 1 || runner.revs[0] != "head111" {
		t.Fatalf("tree lookups = %v, want a single query for head111", runner.revs)
	}
}

// A deleted path cannot be in the reviewed tree, so asking only that tree leaves a
// removed symlink unmarked and its target reviewed as ordinary text. The deletion
// in the change's own commits is what states the mode.
func TestStampSymlinkFlagsMarksDeletedSymlinksFromTheirDeletion(t *testing.T) {
	reviewCtx := &model.ReviewContext{
		Mode:               model.ModeGitHub,
		DiffOmitsFileModes: true,
		CheckoutRoot:       "/checkout",
		DiffHeadSHA:        "head111",
		Commits:            []model.CommitSummary{{SHA: "c1"}},
		ChangedFiles: []model.ChangedFile{
			{Path: "dir/link", Status: model.FileDeleted},
			{Path: "main.go", Status: model.FileDeleted},
		},
		DiffFiles: []model.DiffFile{{FilePath: "dir/link"}, {FilePath: "main.go"}},
		DiffHunks: []model.DiffHunk{{FilePath: "dir/link"}, {FilePath: "main.go"}},
	}

	runner := &symlinkTreeRunner{deleted: []string{"dir/link"}}
	stampSymlinkFlags(context.Background(), reviewCtx, runner)

	if !reviewCtx.ChangedFiles[0].Symlink || !reviewCtx.DiffFiles[0].Symlink || !reviewCtx.DiffHunks[0].Symlink {
		t.Fatalf("deleted symlink not marked in every view: %#v / %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0], reviewCtx.DiffHunks[0])
	}
	if reviewCtx.ChangedFiles[1].Symlink || reviewCtx.DiffFiles[1].Symlink || reviewCtx.DiffHunks[1].Symlink {
		t.Fatalf("deleted regular file marked as a symlink: %#v / %#v / %#v", reviewCtx.ChangedFiles[1], reviewCtx.DiffFiles[1], reviewCtx.DiffHunks[1])
	}
	// The deleted paths go to the deletion listing, not to the head tree.
	if len(runner.logCalls) != 1 {
		t.Fatalf("deletion listings = %d, want one", len(runner.logCalls))
	}
	if len(runner.revs) != 0 {
		t.Fatalf("head-tree lookups = %v, want none for a change that only deletes", runner.revs)
	}
}

// Without the reviewed head SHA there is nothing to address the tree by, and a
// chat session resumed with --repo-root points at a user-selected working copy
// that may hold local edits or another revision entirely. Guessing from it would
// stamp a locally symlinked path that is regular text in the reviewed change.
func TestStampSymlinkFlagsSkipsWithoutReviewedHead(t *testing.T) {
	reviewCtx := &model.ReviewContext{
		Mode:               model.ModeGitHub,
		DiffOmitsFileModes: true,
		CheckoutRoot:       "/checkout",
		ChangedFiles:       []model.ChangedFile{{Path: "templates"}},
		DiffFiles:          []model.DiffFile{{FilePath: "templates"}},
	}

	runner := &symlinkTreeRunner{symlinks: []string{"templates"}}
	stampSymlinkFlags(context.Background(), reviewCtx, runner)

	if reviewCtx.ChangedFiles[0].Symlink || reviewCtx.DiffFiles[0].Symlink {
		t.Fatalf("marked without a reviewed head SHA: %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0])
	}
	if len(runner.revs) != 0 {
		t.Fatalf("tree lookups = %v, want none without a head SHA", runner.revs)
	}
}

// The checkout can hold a different revision than the reviewed diff (staged
// content, or a base..head range that is not checked out), so a source that does
// report modes must never be second-guessed by it.
func TestStampSymlinkFlagsTrustsModesOverCheckout(t *testing.T) {
	for _, mode := range []model.ReviewMode{model.ModeLocal, model.ModeGitLab} {
		t.Run(string(mode), func(t *testing.T) {
			reviewCtx := &model.ReviewContext{
				Mode:               mode,
				DiffOmitsFileModes: mode == model.ModeGitHub,
				CheckoutRoot:       "/checkout",
				DiffHeadSHA:        "head111",
				ChangedFiles:       []model.ChangedFile{{Path: "templates"}},
				DiffFiles:          []model.DiffFile{{FilePath: "templates"}},
			}

			runner := &symlinkTreeRunner{symlinks: []string{"templates"}}
			stampSymlinkFlags(context.Background(), reviewCtx, runner)

			if reviewCtx.ChangedFiles[0].Symlink || reviewCtx.DiffFiles[0].Symlink {
				t.Fatalf("the checkout overrode the diff's mode: %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0])
			}
			if len(runner.revs) != 0 {
				t.Fatalf("tree lookups = %v, want none for a source that reports modes", runner.revs)
			}
		})
	}
}

// Without a checkout there is no index to ask, so no git call may be made.
func TestStampSymlinkFlagsSkipsWithoutCheckout(t *testing.T) {
	reviewCtx := &model.ReviewContext{
		Mode:               model.ModeGitHub,
		DiffOmitsFileModes: true,
		DiffHeadSHA:        "head111",
		ChangedFiles:       []model.ChangedFile{{Path: "templates"}},
		DiffFiles:          []model.DiffFile{{FilePath: "templates"}},
	}

	runner := &symlinkTreeRunner{symlinks: []string{"templates"}}
	stampSymlinkFlags(context.Background(), reviewCtx, runner)

	if reviewCtx.ChangedFiles[0].Symlink || reviewCtx.DiffFiles[0].Symlink {
		t.Fatalf("marked without a checkout: %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0])
	}
	if len(runner.revs) != 0 {
		t.Fatalf("tree lookups = %v, want none without a checkout", runner.revs)
	}
}

// Git paths must stay literal when the marks are propagated. On Unix a symlink
// named `a\b` and a regular file `a/b` are two different files, and folding them
// onto one key would let the symlink's mark suppress the other file's text.
func TestStampSymlinkFlagsKeepsPathsLiteral(t *testing.T) {
	reviewCtx := &model.ReviewContext{
		Mode:               model.ModeGitHub,
		DiffOmitsFileModes: true,
		CheckoutRoot:       "/checkout",
		DiffHeadSHA:        "head111",
		ChangedFiles:       []model.ChangedFile{{Path: `a\b`}, {Path: "a/b"}},
		DiffFiles:          []model.DiffFile{{FilePath: `a\b`}, {FilePath: "a/b"}},
		DiffHunks:          []model.DiffHunk{{FilePath: `a\b`}, {FilePath: "a/b"}},
	}

	stampSymlinkFlags(context.Background(), reviewCtx, &symlinkTreeRunner{symlinks: []string{`a\b`}})

	if !reviewCtx.ChangedFiles[0].Symlink || !reviewCtx.DiffFiles[0].Symlink || !reviewCtx.DiffHunks[0].Symlink {
		t.Fatalf("symlink not marked: %#v / %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0], reviewCtx.DiffHunks[0])
	}
	if reviewCtx.ChangedFiles[1].Symlink || reviewCtx.DiffFiles[1].Symlink || reviewCtx.DiffHunks[1].Symlink {
		t.Fatalf("a distinct path inherited the mark: %#v / %#v / %#v", reviewCtx.ChangedFiles[1], reviewCtx.DiffFiles[1], reviewCtx.DiffHunks[1])
	}
}

// A pure symlink rename emits no hunk and no content in any source, so the target
// is nowhere in the patch — yet whether a relative target still resolves from the
// new directory is the whole question the change raises. It is read from the blob
// the reviewed head tree names, for every source, not just the mode-less ones.
func TestStampSymlinkFlagsReadsTargetForHunklessRename(t *testing.T) {
	for _, mode := range []model.ReviewMode{model.ModeGitLab, model.ModeGitHub} {
		t.Run(string(mode), func(t *testing.T) {
			reviewCtx := &model.ReviewContext{
				Mode:               mode,
				DiffOmitsFileModes: mode == model.ModeGitHub,
				CheckoutRoot:       "/checkout",
				DiffHeadSHA:        "head111",
				ChangedFiles: []model.ChangedFile{
					{Path: "dir/link2", Status: model.FileRenamed, OldPath: "dir/sub/link", Symlink: true},
				},
			}

			runner := &symlinkTreeRunner{symlinks: []string{"dir/link2"}, target: "../target"}
			stampSymlinkFlags(context.Background(), reviewCtx, runner)

			if reviewCtx.ChangedFiles[0].SymlinkTarget != "../target" {
				t.Fatalf("target not recovered: %#v", reviewCtx.ChangedFiles[0])
			}
			if len(runner.blobReads) != 1 || runner.blobReads[0] != "32f64f4" {
				t.Fatalf("blob reads = %v, want the object the tree named once", runner.blobReads)
			}
		})
	}
}

// A symlink whose patch carries a hunk already shows its target, so the blob must
// not be read again — the diff is what the reviewer sees.
func TestStampSymlinkFlagsLeavesVisibleTargetsAlone(t *testing.T) {
	reviewCtx := &model.ReviewContext{
		Mode:         model.ModeGitLab,
		CheckoutRoot: "/checkout",
		DiffHeadSHA:  "head111",
		ChangedFiles: []model.ChangedFile{{Path: "link", Status: model.FileAdded, Symlink: true}},
		DiffHunks:    []model.DiffHunk{{FilePath: "link", NewStart: 1, NewLines: 1, Content: "+target", Symlink: true}},
	}

	runner := &symlinkTreeRunner{symlinks: []string{"link"}, target: "target"}
	stampSymlinkFlags(context.Background(), reviewCtx, runner)

	if reviewCtx.ChangedFiles[0].SymlinkTarget != "" {
		t.Fatalf("target duplicated although the hunk shows it: %#v", reviewCtx.ChangedFiles[0])
	}
	if len(runner.revs) != 0 || len(runner.blobReads) != 0 {
		t.Fatalf("git ran anyway: revs=%v blobs=%v", runner.revs, runner.blobReads)
	}
}
