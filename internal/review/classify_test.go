package review

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

// A mark on either file view must reach the other: the two views are built by
// different code paths (SCM adapter vs diff parser) and every downstream agent
// reads whichever one its prompt format selects.
func TestStampSymlinkFlagsPropagatesBetweenFileViews(t *testing.T) {
	reviewCtx := &model.ReviewContext{
		ChangedFiles: []model.ChangedFile{
			{Path: "deploy/chart/templates", Symlink: true},
			{Path: "cmd/main.go"},
			{Path: "docs/link"},
		},
		DiffFiles: []model.DiffFile{
			{FilePath: "deploy/chart/templates"},
			{FilePath: "cmd/main.go"},
			{FilePath: "docs/link", Symlink: true},
		},
	}

	stampSymlinkFlags(reviewCtx)

	if !reviewCtx.DiffFiles[0].Symlink {
		t.Fatalf("changed-file mark did not reach the diff file: %#v", reviewCtx.DiffFiles[0])
	}
	if !reviewCtx.ChangedFiles[2].Symlink {
		t.Fatalf("diff-file mark did not reach the changed file: %#v", reviewCtx.ChangedFiles[2])
	}
	if reviewCtx.ChangedFiles[1].Symlink || reviewCtx.DiffFiles[1].Symlink {
		t.Fatalf("regular file marked as symlink: %#v / %#v", reviewCtx.ChangedFiles[1], reviewCtx.DiffFiles[1])
	}
}

// GitHub's files API reports neither a mode nor a mode header line in `patch`,
// so the checkout is the only source left for that provider.
func TestStampSymlinkFlagsFallsBackToCheckout(t *testing.T) {
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
	reviewCtx := &model.ReviewContext{
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
