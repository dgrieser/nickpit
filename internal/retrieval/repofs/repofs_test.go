package repofs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolvePathRejectsEscapingInputs(t *testing.T) {
	repoRoot := t.TempDir()
	tests := []string{"../secret.txt", "../../x", "/tmp/secret.txt"}
	for _, path := range tests {
		if _, _, err := ResolvePath(repoRoot, path); err == nil {
			t.Fatalf("ResolvePath(%q) succeeded", path)
		}
	}
}

func TestResolvePathNormalizesInRepoInputs(t *testing.T) {
	repoRoot := t.TempDir()
	normalized, _, err := ResolvePath(repoRoot, "pkg/../pkg/file.go")
	if err != nil {
		t.Fatal(err)
	}
	if normalized != "pkg/file.go" {
		t.Fatalf("normalized = %q", normalized)
	}

	normalized, fullPath, err := ResolvePath(repoRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	if normalized != "" || fullPath == "" {
		t.Fatalf("empty path resolution failed: normalized=%q fullPath=%q", normalized, fullPath)
	}
}

func TestSanitizeGitArgsRedactsSecrets(t *testing.T) {
	args := []string{
		"-c", "http.extraHeader=Authorization: Basic abc123",
		"clone",
		"https://user:secret@example.com/repo.git",
		"https://token@example.com/repo.git",
	}
	got := SanitizeGitArgs(args)
	want := []string{
		"-c", "http.extraHeader=<redacted>",
		"clone",
		"https://%3Credacted%3E:%3Credacted%3E@example.com/repo.git",
		"https://%3Credacted%3E:%3Credacted%3E@example.com/repo.git",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v", got)
	}
}

func TestIgnoreMatcherSupportsDirectoryGlobAndNegation(t *testing.T) {
	repoRoot := t.TempDir()
	writeFile(t, repoRoot, ".gitignore", "ignored/\n*.log\n!important.log\n")
	writeFile(t, repoRoot, "ignored/tmp.go", "package ignored")
	writeFile(t, repoRoot, "debug.log", "x")
	writeFile(t, repoRoot, "important.log", "x")
	writeFile(t, repoRoot, "pkg/a.go", "package pkg")

	matcher := NewIgnoreMatcher(repoRoot)
	if !matcher.IsIgnored("ignored", true) {
		t.Fatal("expected ignored dir to match")
	}
	if !matcher.IsIgnored("ignored/tmp.go", false) {
		t.Fatal("expected file in ignored dir to match")
	}
	if !matcher.IsIgnored("debug.log", false) {
		t.Fatal("expected glob match")
	}
	if matcher.IsIgnored("important.log", false) {
		t.Fatal("expected negation to unignore")
	}
	if matcher.IsIgnored("pkg/a.go", false) {
		t.Fatal("unexpected ignore match")
	}
}

func TestIgnoreMatcherSupportsNestedGitignoreAndAnchoredRules(t *testing.T) {
	repoRoot := t.TempDir()
	writeFile(t, repoRoot, ".gitignore", "/root-only.txt\n")
	writeFile(t, repoRoot, "pkg/.gitignore", "tmp/\n/root.txt\n")
	writeFile(t, repoRoot, "root-only.txt", "x")
	writeFile(t, repoRoot, "pkg/root.txt", "x")
	writeFile(t, repoRoot, "pkg/tmp/a.go", "package tmp")
	writeFile(t, repoRoot, "pkg/nested/root.txt", "x")

	matcher := NewIgnoreMatcher(repoRoot)
	if !matcher.IsIgnored("root-only.txt", false) {
		t.Fatal("expected anchored root rule to match")
	}
	if !matcher.IsIgnored("pkg/root.txt", false) {
		t.Fatal("expected nested anchored rule to match")
	}
	if matcher.IsIgnored("pkg/nested/root.txt", false) {
		t.Fatal("expected nested anchored rule not to match descendant")
	}
	if !matcher.IsIgnored("pkg/tmp/a.go", false) {
		t.Fatal("expected nested dir rule to match descendant")
	}
}

func TestIgnoreMatcherSupportsGlobstarPatterns(t *testing.T) {
	repoRoot := t.TempDir()
	writeFile(t, repoRoot, ".gitignore", "**/node_modules\n")
	writeFile(t, repoRoot, "node_modules/a.js", "x")
	writeFile(t, repoRoot, "pkg/node_modules/b.js", "x")
	writeFile(t, repoRoot, "pkg/src/app.js", "x")

	matcher := NewIgnoreMatcher(repoRoot)
	if !matcher.IsIgnored("node_modules", true) {
		t.Fatal("expected root node_modules to match")
	}
	if !matcher.IsIgnored("pkg/node_modules", true) {
		t.Fatal("expected nested node_modules to match")
	}
	if !matcher.IsIgnored("pkg/node_modules/b.js", false) {
		t.Fatal("expected descendant of nested node_modules to match")
	}
	if matcher.IsIgnored("pkg/src/app.js", false) {
		t.Fatal("unexpected ignore match")
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	fullPath := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
