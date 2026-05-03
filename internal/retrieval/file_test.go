package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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
		ResultCount:   1,
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
		ResultCount:   1,
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
		ResultCount:   1,
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

func TestLocalEngineSearchFallsBackToUnescapedQuery(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "func (cache *RedisSSHSessionCache) sessionPodKey() string {\n\treturn \"x\"\n}"
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "a.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	got, err := engine.Search(context.Background(), repoRoot, "pkg", `func (cache \*RedisSSHSessionCache) sessionPodKey`, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	want := &SearchResults{
		Path:          "pkg",
		Query:         "func (cache *RedisSSHSessionCache) sessionPodKey",
		ContextLines:  0,
		CaseSensitive: false,
		ResultCount:   1,
		Results: []SearchResult{
			{
				Path:      "pkg/a.go",
				StartLine: 1,
				EndLine:   1,
				Language:  "go",
				Content:   "func (cache *RedisSSHSessionCache) sessionPodKey() string {",
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("search = %#v, want %#v", got, want)
	}
}

func TestLocalEngineListFilesSkipsGitIgnoredAndDotGit(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repoRoot, "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".gitignore"), []byte("ignored/\n*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "a.go"), []byte("package pkg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "debug.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "ignored", "tmp.go"), []byte("package ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	got, err := engine.ListFiles(context.Background(), repoRoot, "", 2)
	if err != nil {
		t.Fatal(err)
	}

	want := &DirectoryListing{
		Path:  "",
		Files: []string{".gitignore", "pkg/", "pkg/a.go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listing = %#v, want %#v", got, want)
	}
}

func TestLocalEngineRejectsPathsOutsideRepo(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, "file.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	if _, err := engine.GetFile(context.Background(), repoRoot, "../secret.txt"); err == nil {
		t.Fatal("expected GetFile error")
	}
	if _, err := engine.GetFileSlice(context.Background(), repoRoot, "../secret.txt", 1, 1); err == nil {
		t.Fatal("expected GetFileSlice error")
	}
	if _, err := engine.ListFiles(context.Background(), repoRoot, "../", 1); err == nil {
		t.Fatal("expected ListFiles error")
	}
	if _, err := engine.Search(context.Background(), repoRoot, "../", "ok", 0, 0, false); err == nil {
		t.Fatal("expected Search error")
	}
}

func TestLocalEngineSearchSkipsGitIgnored(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repoRoot, "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".gitignore"), []byte("ignored/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "a.go"), []byte("needle"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "ignored", "tmp.go"), []byte("needle"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	got, err := engine.Search(context.Background(), repoRoot, "", "needle", 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.ResultCount != 1 || len(got.Results) != 1 || got.Results[0].Path != "pkg/a.go" {
		t.Fatalf("search = %#v", got)
	}
}

func TestLocalEngineSearchRegex(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "package pkg\n\nfunc NewClient() {}\nfunc oldClient() {}\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "a.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	pattern := regexp.MustCompile(`^func\s+New\w+\(`)
	got, err := engine.SearchRegex(context.Background(), repoRoot, "pkg", pattern, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.ResultCount != 1 {
		t.Fatalf("result_count = %d, want 1", got.ResultCount)
	}
	if got.Results[0].Path != "pkg/a.go" || got.Results[0].StartLine != 3 {
		t.Fatalf("results = %#v", got.Results)
	}
	if got.Query != pattern.String() {
		t.Fatalf("query = %q, want %q", got.Query, pattern.String())
	}
}

func TestLocalEngineSearchRegexCaseInsensitive(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "a.go"), []byte("HelloWorld\nGoodbye\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	pattern := regexp.MustCompile(`(?i)hello\w+`)
	got, err := engine.SearchRegex(context.Background(), repoRoot, "pkg", pattern, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.ResultCount != 1 {
		t.Fatalf("result_count = %d, want 1", got.ResultCount)
	}
	if got.Results[0].Content != "HelloWorld" {
		t.Fatalf("content = %q", got.Results[0].Content)
	}
}

func TestLocalEngineSearchRegexRespectsMaxResults(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "a.go"), []byte("foo\nfoo\nfoo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	pattern := regexp.MustCompile(`^foo$`)
	got, err := engine.SearchRegex(context.Background(), repoRoot, "pkg", pattern, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.ResultCount != 2 {
		t.Fatalf("result_count = %d, want 2", got.ResultCount)
	}
}

func TestLocalEngineSearchRegexSkipsBinaryAndIgnored(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repoRoot, "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".gitignore"), []byte("ignored/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "a.go"), []byte("needle"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "ignored", "tmp.go"), []byte("needle"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "pkg", "blob.bin"), []byte{0x00, 'n', 'e', 'e', 'd', 'l', 'e'}, 0o644); err != nil {
		t.Fatal(err)
	}

	engine := NewLocalEngine()
	pattern := regexp.MustCompile(`needle`)
	got, err := engine.SearchRegex(context.Background(), repoRoot, "", pattern, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.ResultCount != 1 || got.Results[0].Path != "pkg/a.go" {
		t.Fatalf("search = %#v", got)
	}
}
