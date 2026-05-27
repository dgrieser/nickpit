//go:build unix

package review

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadReviewFileRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.yaml")
	link := filepath.Join(root, "link.yaml")
	if err := os.WriteFile(target, []byte("apiVersion: v1\nkind: Service\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if got := readReviewFile(root, "link.yaml"); got != "" {
		t.Fatalf("readReviewFile symlink = %q", got)
	}
}
