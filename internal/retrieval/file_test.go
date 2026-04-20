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
	got, err := engine.ListFiles(context.Background(), repoRoot, "pkg", 1)
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

func TestLocalEngineListFilesRepoRoot(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	got, err := engine.ListFiles(context.Background(), repoRoot, "", 1)
	if err != nil {
		t.Fatal(err)
	}

	want := &DirectoryListing{
		Path:  "",
		Files: []string{"README.md", "pkg/"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listing = %#v, want %#v", got, want)
	}
}

func TestLocalEngineListFilesWithDepth(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "sub", "c.go"), []byte("package sub"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	got, err := engine.ListFiles(context.Background(), repoRoot, "pkg", 2)
	if err != nil {
		t.Fatal(err)
	}

	want := &DirectoryListing{
		Path:  "pkg",
		Files: []string{"pkg/a.go", "pkg/sub/", "pkg/sub/c.go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listing = %#v, want %#v", got, want)
	}
}

func TestLocalEngineSearch(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "a.go"), []byte("one\nttlExtenders\nthree\nfour"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "b.go"), []byte("ttlExtenders\nx"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	got, err := engine.Search(context.Background(), repoRoot, "pkg", "ttlExtenders", 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}

	want := &SearchResults{
		Path:          "pkg",
		Query:         "ttlExtenders",
		ContextLines:  1,
		MaxResults:    1,
		CaseSensitive: false,
		Results: []SearchResult{
			{
				Path:      "pkg/a.go",
				StartLine: 1,
				EndLine:   3,
				Language:  "go",
				Content:   "one\nttlExtenders\nthree",
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("search = %#v, want %#v", got, want)
	}
}

func TestLocalEngineSearchSkipsBinaryFiles(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "a.go"), []byte("ttlExtenders\nx"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "blob.bin"), []byte{0x00, 0xff, 0x01, 't', 't', 'l'}, 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	got, err := engine.Search(context.Background(), repoRoot, "pkg", "ttlExtenders", 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	want := &SearchResults{
		Path:          "pkg",
		Query:         "ttlExtenders",
		ContextLines:  0,
		CaseSensitive: false,
		Results: []SearchResult{
			{
				Path:      "pkg/a.go",
				StartLine: 1,
				EndLine:   1,
				Language:  "go",
				Content:   "ttlExtenders",
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("search = %#v, want %#v", got, want)
	}
}

func TestLocalEngineSearchCaseSensitive(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "a.go"), []byte("ttlExtenders\nTTLEXTENDERS"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	got, err := engine.Search(context.Background(), repoRoot, "pkg", "TTLEXTENDERS", 0, 0, true)
	if err != nil {
		t.Fatal(err)
	}

	want := &SearchResults{
		Path:          "pkg",
		Query:         "TTLEXTENDERS",
		ContextLines:  0,
		CaseSensitive: true,
		Results: []SearchResult{
			{
				Path:      "pkg/a.go",
				StartLine: 2,
				EndLine:   2,
				Language:  "go",
				Content:   "TTLEXTENDERS",
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("search = %#v, want %#v", got, want)
	}
}
