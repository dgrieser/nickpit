package retrieval

import (
	"errors"
	"sync"
	"testing"
)

func TestSupportsStructuralAnalysis(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "main.go", "package main\n\nfunc Run() {}\n")
	writeRetrievalFile(t, repoRoot, "app.py", "def run():\n    return 1\n")
	writeRetrievalFile(t, repoRoot, "app.ts", "export function run() { return 1 }\n")
	writeRetrievalFile(t, repoRoot, "lib.rs", "pub fn run() {}\n")
	writeRetrievalFile(t, repoRoot, "notes.txt", "hello\n")
	writeRetrievalFile(t, repoRoot, "Main.java", "class Main {}\n")
	writeRetrievalFile(t, repoRoot, "pkg/mod.rs", "fn helper() {}\n")

	tests := []struct {
		path string
		want bool
	}{
		{"main.go", true},
		{"app.py", true},
		{"app.ts", true},
		{"lib.rs", true}, // Rust is supported by rustBackend
		{"notes.txt", false},
		{"Main.java", false},         // genuinely unsupported language
		{"", true},                   // repo-wide scope: a backend can still attempt a search
		{"pkg", true},                // directory scope
		{"../escape.go", false},      // path escaping the repo resolves to an error
		{"does-not-exist.go", false}, // missing file cannot be stat'd
	}
	for _, tt := range tests {
		if got := SupportsStructuralAnalysis(repoRoot, tt.path); got != tt.want {
			t.Errorf("SupportsStructuralAnalysis(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// TestBuildStaticGraphCachedReusesAndIsConcurrencySafe mirrors the goparser
// cache test for the shared regex-backend memoizer: the same graph is reused
// across calls for one (language, repoRoot, scope) key, errors are not cached,
// and concurrent access is race-free (run under -race).
func TestBuildStaticGraphCachedReusesAndIsConcurrencySafe(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "src/sso.rs", rustFixture)
	scope := scopeForHierarchy(lookupScope{Path: "src/sso.rs", IsFile: true})

	calls := 0
	build := func() (*staticGraph, error) {
		calls++
		return buildRustGraph(repoRoot, scope)
	}

	first, err := buildStaticGraphCached("rust-reuse", repoRoot, scope, build)
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildStaticGraphCached("rust-reuse", repoRoot, scope, build)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("buildStaticGraphCached rebuilt the graph instead of reusing the cached value")
	}
	if calls != 1 {
		t.Fatalf("build invoked %d times, want 1", calls)
	}

	// Errors are not cached: a failed build can be retried successfully.
	wantErr := errors.New("boom")
	_, err = buildStaticGraphCached("rust-retry", repoRoot, scope, func() (*staticGraph, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	retried, err := buildStaticGraphCached("rust-retry", repoRoot, scope, build)
	if err != nil || retried == nil {
		t.Fatalf("retry after error failed: graph=%v err=%v", retried, err)
	}

	// Concurrent callers share the cached value without data races.
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := buildStaticGraphCached("rust-reuse", repoRoot, scope, build); err != nil {
				t.Errorf("concurrent buildStaticGraphCached: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestCandidateBackendsReturnsUnsupportedLanguageError(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "Main.java", "class Main {}\n")

	scope, err := resolveLookupScope(repoRoot, "Main.java")
	if err != nil {
		t.Fatal(err)
	}
	_, err = candidateBackends(scope)
	var unsupported *UnsupportedLanguageError
	if !errors.As(err, &unsupported) {
		t.Fatalf("candidateBackends error = %v, want *UnsupportedLanguageError", err)
	}
}
