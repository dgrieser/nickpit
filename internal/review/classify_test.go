package review

import (
	"context"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// symlinkTreeRunner answers "git ls-tree -z <rev>" from a fixed set of symlink
// paths, so the stamping tests need no real checkout. It records the revisions it
// was asked for, because the marks are only sound when they come from the
// reviewed head rather than from whatever a checkout holds.
type symlinkTreeRunner struct {
	symlinks []string
	revs     []string
}

func (r *symlinkTreeRunner) Run(_ context.Context, args ...string) (string, error) {
	if len(args) < 3 || args[0] != "ls-tree" {
		return "", nil
	}
	r.revs = append(r.revs, args[2])
	var out strings.Builder
	for _, path := range r.symlinks {
		out.WriteString("120000 blob 32f64f4\t" + path + "\x00")
	}
	return out.String(), nil
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
		Mode:         model.ModeGitHub,
		CheckoutRoot: "/checkout",
		DiffHeadSHA:  "head111",
		ChangedFiles: []model.ChangedFile{{Path: "templates"}, {Path: "main.go"}},
		DiffFiles:    []model.DiffFile{{FilePath: "templates"}, {FilePath: "main.go"}},
		DiffHunks:    []model.DiffHunk{{FilePath: "templates"}, {FilePath: "main.go"}},
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

// Without the reviewed head SHA there is nothing to address the tree by, and a
// chat session resumed with --repo-root points at a user-selected working copy
// that may hold local edits or another revision entirely. Guessing from it would
// stamp a locally symlinked path that is regular text in the reviewed change.
func TestStampSymlinkFlagsSkipsWithoutReviewedHead(t *testing.T) {
	reviewCtx := &model.ReviewContext{
		Mode:         model.ModeGitHub,
		CheckoutRoot: "/checkout",
		ChangedFiles: []model.ChangedFile{{Path: "templates"}},
		DiffFiles:    []model.DiffFile{{FilePath: "templates"}},
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
				Mode:         mode,
				CheckoutRoot: "/checkout",
				DiffHeadSHA:  "head111",
				ChangedFiles: []model.ChangedFile{{Path: "templates"}},
				DiffFiles:    []model.DiffFile{{FilePath: "templates"}},
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
		Mode:         model.ModeGitHub,
		DiffHeadSHA:  "head111",
		ChangedFiles: []model.ChangedFile{{Path: "templates"}},
		DiffFiles:    []model.DiffFile{{FilePath: "templates"}},
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
