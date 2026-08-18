package review

import (
	"context"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// symlinkIndexRunner answers "git ls-files --stage -z" from a fixed set of
// symlink paths, so the stamping tests do not need a real checkout.
type symlinkIndexRunner struct {
	symlinks []string
	calls    int
}

func (r *symlinkIndexRunner) Run(_ context.Context, args ...string) (string, error) {
	r.calls++
	if len(args) == 0 || args[0] != "ls-files" {
		return "", nil
	}
	var out strings.Builder
	for _, path := range r.symlinks {
		out.WriteString("120000 32f64f4 0\t" + path + "\x00")
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
		ChangedFiles: []model.ChangedFile{
			{Path: "link", Status: model.FileDeleted, Symlink: true},
			{Path: "link", Status: model.FileAdded},
		},
		DiffFiles: []model.DiffFile{
			{FilePath: "link", Content: "deleted file mode 120000\n", Symlink: true},
			{FilePath: "link", Content: "new file mode 100644\n"},
		},
	}

	stampSymlinkFlags(context.Background(), reviewCtx, &symlinkIndexRunner{symlinks: []string{"link"}})

	if !reviewCtx.ChangedFiles[0].Symlink || !reviewCtx.DiffFiles[0].Symlink {
		t.Fatalf("symlink deletion lost its mark: %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0])
	}
	if reviewCtx.ChangedFiles[1].Symlink || reviewCtx.DiffFiles[1].Symlink {
		t.Fatalf("regular-file addition inherited the symlink mark: %#v / %#v", reviewCtx.ChangedFiles[1], reviewCtx.DiffFiles[1])
	}
}

// GitHub's files API reports neither a mode nor a mode header line in `patch`,
// so the checkout's git index — recording the reviewed head revision — is the
// only source of truth left for that provider.
func TestStampSymlinkFlagsProbesIndexForModelessSource(t *testing.T) {
	reviewCtx := &model.ReviewContext{
		Mode:         model.ModeGitHub,
		CheckoutRoot: "/checkout",
		ChangedFiles: []model.ChangedFile{{Path: "templates"}, {Path: "main.go"}},
		DiffFiles:    []model.DiffFile{{FilePath: "templates"}, {FilePath: "main.go"}},
	}

	runner := &symlinkIndexRunner{symlinks: []string{"templates"}}
	stampSymlinkFlags(context.Background(), reviewCtx, runner)

	if !reviewCtx.ChangedFiles[0].Symlink || !reviewCtx.DiffFiles[0].Symlink {
		t.Fatalf("index symlink not detected: %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0])
	}
	if reviewCtx.ChangedFiles[1].Symlink || reviewCtx.DiffFiles[1].Symlink {
		t.Fatalf("regular file marked as symlink: %#v / %#v", reviewCtx.ChangedFiles[1], reviewCtx.DiffFiles[1])
	}
	// Both file views share one index query.
	if runner.calls != 1 {
		t.Fatalf("git calls = %d, want a single batched query", runner.calls)
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
				ChangedFiles: []model.ChangedFile{{Path: "templates"}},
				DiffFiles:    []model.DiffFile{{FilePath: "templates"}},
			}

			runner := &symlinkIndexRunner{symlinks: []string{"templates"}}
			stampSymlinkFlags(context.Background(), reviewCtx, runner)

			if reviewCtx.ChangedFiles[0].Symlink || reviewCtx.DiffFiles[0].Symlink {
				t.Fatalf("the checkout overrode the diff's mode: %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0])
			}
			if runner.calls != 0 {
				t.Fatalf("git calls = %d, want none for a source that reports modes", runner.calls)
			}
		})
	}
}

// Without a checkout there is no index to ask, so no git call may be made.
func TestStampSymlinkFlagsSkipsWithoutCheckout(t *testing.T) {
	reviewCtx := &model.ReviewContext{
		Mode:         model.ModeGitHub,
		ChangedFiles: []model.ChangedFile{{Path: "templates"}},
		DiffFiles:    []model.DiffFile{{FilePath: "templates"}},
	}

	runner := &symlinkIndexRunner{symlinks: []string{"templates"}}
	stampSymlinkFlags(context.Background(), reviewCtx, runner)

	if reviewCtx.ChangedFiles[0].Symlink || reviewCtx.DiffFiles[0].Symlink {
		t.Fatalf("marked without a checkout: %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0])
	}
	if runner.calls != 0 {
		t.Fatalf("git calls = %d, want none without a checkout", runner.calls)
	}
}
