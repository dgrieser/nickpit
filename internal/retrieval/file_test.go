package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLocalEngineListFiles(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "a.go"), []byte("package pkg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	got, err := engine.ListFiles(context.Background(), repoRoot, "pkg")
	if err != nil {
		t.Fatal(err)
	}

	want := &DirectoryListing{
		Path:  "pkg",
		Files: []string{"pkg/a.go", "pkg/b.txt", "pkg/sub/"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listing = %#v, want %#v", got, want)
	}
}
