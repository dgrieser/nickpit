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

func TestFallbackSearchScope(t *testing.T) {
	repoRoot := t.TempDir()
	writeRetrievalFile(t, repoRoot, "src/auth.rb", "def redirect_allowed; end\n")

	tests := []struct {
		name string
		path string
		want string
	}{
		{"file widens to repo-wide", "src/auth.rb", ""},
		{"directory is kept", "src", "src"},
		{"repo-wide scope stays repo-wide", "", ""},
		{"unresolvable path is returned unchanged", "../escape.rb", "../escape.rb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FallbackSearchScope(repoRoot, tt.path); got != tt.want {
				t.Fatalf("FallbackSearchScope(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
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

// TestStaticGraphCacheCapRefusesNewKeysWhenFull exercises the soft cap on a fresh
// store (isolated from the process-global counter): under-cap directory scopes are
// memoized, an over-cap directory scope degrades to rebuild-without-caching, and
// the repo-wide scope is admitted regardless of the cap.
func TestStaticGraphCacheCapRefusesNewKeysWhenFull(t *testing.T) {
	t.Setenv("NICKPIT_GRAPH_CACHE_MAX_ENTRIES", "2")
	c := &staticGraphCacheStore{}

	calls := map[string]int{}
	get := func(key string, repoWide bool) *staticGraph {
		g, err := c.getOrBuild(key, repoWide, func() (*staticGraph, error) {
			calls[key]++
			return &staticGraph{}, nil
		})
		if err != nil {
			t.Fatalf("getOrBuild(%q): %v", key, err)
		}
		if g == nil {
			t.Fatalf("getOrBuild(%q) returned a nil graph", key)
		}
		return g
	}

	// Two directory scopes fit under the cap and are memoized (built once, reused).
	if d1a, d1b := get("d1", false), get("d1", false); d1a != d1b {
		t.Fatal("d1 was rebuilt instead of reused")
	}
	get("d2", false)
	if calls["d1"] != 1 || calls["d2"] != 1 {
		t.Fatalf("under-cap builds: d1=%d d2=%d, want 1 each", calls["d1"], calls["d2"])
	}
	if got := c.count.Load(); got != 2 {
		t.Fatalf("count = %d after two directory scopes, want 2", got)
	}

	// A third directory scope is refused: rebuilt on every call, never cached.
	if d3a, d3b := get("d3", false), get("d3", false); d3a == d3b {
		t.Fatal("over-cap directory scope was cached instead of rebuilt")
	}
	if calls["d3"] != 2 {
		t.Fatalf("over-cap build count = %d, want 2 (rebuilt each call)", calls["d3"])
	}
	if got := c.count.Load(); got != 2 {
		t.Fatalf("count = %d after a refused key, want it unchanged at 2", got)
	}

	// The repo-wide scope is exempt from the cap and stays memoized.
	if rwA, rwB := get("rw", true), get("rw", true); rwA != rwB {
		t.Fatal("repo-wide scope was not cached despite the cap exemption")
	}
	if calls["rw"] != 1 {
		t.Fatalf("repo-wide build count = %d, want 1", calls["rw"])
	}
	if got := c.count.Load(); got != 3 {
		t.Fatalf("count = %d after admitting the exempt repo-wide scope, want 3", got)
	}
}

func TestStaticGraphCacheCap(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{"empty falls back to default", "", defaultMaxStaticGraphCacheEntries},
		{"garbage falls back to default", "abc", defaultMaxStaticGraphCacheEntries},
		{"custom value", "10", 10},
		{"surrounding whitespace", "  32  ", 32},
		{"zero disables the cap", "0", 0},
		{"negative disables the cap", "-1", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NICKPIT_GRAPH_CACHE_MAX_ENTRIES", tt.env)
			if got := staticGraphCacheCap(); got != tt.want {
				t.Errorf("staticGraphCacheCap() = %d, want %d", got, tt.want)
			}
		})
	}
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
