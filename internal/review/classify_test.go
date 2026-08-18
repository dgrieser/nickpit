package review

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// A symlink replaced by a regular file at the same path arrives as two entries
// with legitimately different marks. Spreading the deletion's mark over the path
// would mark the regular-file addition as a symlink, and agents would then skip
// reviewing its real text.
func TestStampSymlinkFlagsKeepsReplacementEntriesDistinct(t *testing.T) {
	root := t.TempDir()
	reviewCtx := &model.ReviewContext{
		Mode:         model.ModeLocal,
		CheckoutRoot: root,
		ChangedFiles: []model.ChangedFile{
			{Path: "link", Status: model.FileDeleted, Symlink: true},
			{Path: "link", Status: model.FileAdded},
		},
		DiffFiles: []model.DiffFile{
			{FilePath: "link", Content: "deleted file mode 120000\n", Symlink: true},
			{FilePath: "link", Content: "new file mode 100644\n"},
		},
	}

	stampSymlinkFlags(reviewCtx)

	if !reviewCtx.ChangedFiles[0].Symlink || !reviewCtx.DiffFiles[0].Symlink {
		t.Fatalf("symlink deletion lost its mark: %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0])
	}
	if reviewCtx.ChangedFiles[1].Symlink || reviewCtx.DiffFiles[1].Symlink {
		t.Fatalf("regular-file addition inherited the symlink mark: %#v / %#v", reviewCtx.ChangedFiles[1], reviewCtx.DiffFiles[1])
	}
}

// GitHub's files API reports neither a mode nor a mode header line in `patch`,
// so the checkout — a clone of the reviewed head revision — is the only source
// of truth left for that provider.
func TestStampSymlinkFlagsProbesCheckoutForModelessSource(t *testing.T) {
	root := symlinkCheckout(t)
	reviewCtx := &model.ReviewContext{
		Mode:         model.ModeGitHub,
		CheckoutRoot: root,
		ChangedFiles: []model.ChangedFile{{Path: "templates"}, {Path: "main.go"}},
		DiffFiles:    []model.DiffFile{{FilePath: "templates"}, {FilePath: "main.go"}},
	}

	stampSymlinkFlags(reviewCtx)

	if !reviewCtx.ChangedFiles[0].Symlink || !reviewCtx.DiffFiles[0].Symlink {
		t.Fatalf("checkout symlink not detected: %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0])
	}
	if reviewCtx.ChangedFiles[1].Symlink || reviewCtx.DiffFiles[1].Symlink {
		t.Fatalf("regular file marked as symlink: %#v / %#v", reviewCtx.ChangedFiles[1], reviewCtx.DiffFiles[1])
	}
}

// The worktree can hold a different revision than the reviewed diff (staged
// content, or a base..head range that is not checked out), so a source that does
// report modes must never be second-guessed by the current checkout.
func TestStampSymlinkFlagsTrustsModesOverCheckout(t *testing.T) {
	root := symlinkCheckout(t)
	for _, mode := range []model.ReviewMode{model.ModeLocal, model.ModeGitLab} {
		t.Run(string(mode), func(t *testing.T) {
			reviewCtx := &model.ReviewContext{
				Mode:         mode,
				CheckoutRoot: root,
				ChangedFiles: []model.ChangedFile{{Path: "templates"}},
				DiffFiles:    []model.DiffFile{{FilePath: "templates"}},
			}

			stampSymlinkFlags(reviewCtx)

			if reviewCtx.ChangedFiles[0].Symlink || reviewCtx.DiffFiles[0].Symlink {
				t.Fatalf("current worktree overrode the diff's mode: %#v / %#v", reviewCtx.ChangedFiles[0], reviewCtx.DiffFiles[0])
			}
		})
	}
}

// Without a checkout the probe must stay silent instead of guessing, and a path
// escaping the checkout must never be probed at all.
func TestIsCheckoutSymlinkRejectsMissingRootAndEscapingPath(t *testing.T) {
	if isCheckoutSymlink("", "templates") {
		t.Fatal("probe ran without a checkout root")
	}
	root := t.TempDir()
	if isCheckoutSymlink(root, "../outside") {
		t.Fatal("escaping path was probed")
	}
	if isCheckoutSymlink(root, "/etc/passwd") {
		t.Fatal("absolute path was probed")
	}
	if isCheckoutSymlink(root, "missing") {
		t.Fatal("missing path reported as symlink")
	}
}

// symlinkCheckout builds a checkout holding one symlink ("templates") and one
// regular file ("main.go").
func symlinkCheckout(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config", "crd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("config", "crd"), filepath.Join(root, "templates")); err != nil {
		t.Fatal(err)
	}
	return root
}
