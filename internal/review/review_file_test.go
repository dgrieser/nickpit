package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadReviewFileReadsRegularFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config", "app.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("apiVersion: v1\nkind: Service\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := readReviewFile(root, "config/app.yaml"); got != "apiVersion: v1\nkind: Service\n" {
		t.Fatalf("readReviewFile() = %q", got)
	}
}

func TestReadReviewFileRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if got := readReviewFile(root, "../outside.yaml"); got != "" {
		t.Fatalf("readReviewFile traversal = %q", got)
	}
}

func TestReadReviewFileRejectsOversizedFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "large.yaml")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxStyleGuideProbeBytes+1)), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := readReviewFile(root, "large.yaml"); got != "" {
		t.Fatalf("readReviewFile oversized = %q", got[:min(len(got), 80)])
	}
}
