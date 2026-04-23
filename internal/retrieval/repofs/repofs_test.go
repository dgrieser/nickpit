package repofs

import (
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
	}
	got := SanitizeGitArgs(args)
	want := []string{
		"-c", "http.extraHeader=<redacted>",
		"clone",
		"https://user:%3Credacted%3E@example.com/repo.git",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v", got)
	}
}
