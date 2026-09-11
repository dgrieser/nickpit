package git

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

func TestLocalSourceReadBaseFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".nickpit"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "deployment: cli\n"
	if err := os.WriteFile(filepath.Join(dir, ".nickpit", "context.yaml"), []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}

	source := NewLocalSource(dir)
	data, found, err := source.ReadBaseFile(context.Background(), model.ReviewRequest{}, ".nickpit/context.yaml")
	if err != nil {
		t.Fatalf("ReadBaseFile returned err: %v", err)
	}
	if !found || string(data) != want {
		t.Fatalf("found = %v, data = %q; want the file contents", found, data)
	}
}

func TestLocalSourceReadBaseFileMissing(t *testing.T) {
	source := NewLocalSource(t.TempDir())
	data, found, err := source.ReadBaseFile(context.Background(), model.ReviewRequest{}, ".nickpit/context.yaml")
	if err != nil {
		t.Fatalf("ReadBaseFile returned err: %v, want a missing file to be silent", err)
	}
	if found || data != nil {
		t.Fatalf("found = %v, data = %q; want neither", found, data)
	}
}

func TestLocalSourceReadBaseFileNoRepoRoot(t *testing.T) {
	source := NewLocalSource("")
	_, found, err := source.ReadBaseFile(context.Background(), model.ReviewRequest{}, ".nickpit/context.yaml")
	if err != nil || found {
		t.Fatalf("ReadBaseFile() = %v, %v; want no file and no error", found, err)
	}
}

// A checked-out symlink is lexically inside the root but reads whatever it
// targets, so containment alone is not enough.
func TestLocalSourceReadBaseFileRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".nickpit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, ".nickpit", "context.yaml")); err != nil {
		t.Fatal(err)
	}

	_, found, err := NewLocalSource(dir).ReadBaseFile(context.Background(), model.ReviewRequest{}, ".nickpit/context.yaml")
	if err == nil {
		t.Fatal("ReadBaseFile succeeded through a symlink pointing outside the repository")
	}
	if found {
		t.Fatal("found = true, want the escaping read refused")
	}
}

func TestLocalSourceReadBaseFileRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{"../outside.yaml", filepath.Join(dir, "abs.yaml")} {
		_, found, err := NewLocalSource(dir).ReadBaseFile(context.Background(), model.ReviewRequest{}, path)
		if err == nil || !strings.Contains(err.Error(), "escapes repository root") {
			t.Fatalf("path %q: error = %v, want an escape error", path, err)
		}
		if found {
			t.Fatalf("path %q: found = true, want the read refused", path)
		}
	}
}

func TestLocalSourceImplementsBaseFileSource(t *testing.T) {
	var _ model.BaseFileSource = NewLocalSource(t.TempDir())
}
