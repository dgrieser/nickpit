package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCompletion(t *testing.T) {
	tests := []struct {
		shell string
		want  []string
	}{
		{shell: "bash", want: []string{"__start_nickpit()", `commands+=("completion")`}},
		{shell: "zsh", want: []string{"#compdef nickpit", "_nickpit"}},
	}
	for _, tc := range tests {
		t.Run(tc.shell, func(t *testing.T) {
			root := newRootCmd()
			var out bytes.Buffer
			if err := generateCompletion(root, tc.shell, &out); err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("%s completion missing %q", tc.shell, want)
				}
			}
		})
	}
}

func TestGenerateCompletionRejectsUnknownShell(t *testing.T) {
	err := generateCompletion(newRootCmd(), "fish", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "expected bash or zsh") {
		t.Fatalf("error = %v", err)
	}
}

func TestInstallCompletion(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("BASH_COMPLETION_USER_DIR", "")

	tests := []struct {
		shell   string
		relPath string
		want    string
	}{
		{shell: "bash", relPath: "bash-completion/completions/nickpit", want: "__start_nickpit()"},
		{shell: "zsh", relPath: "zsh/site-functions/_nickpit", want: "#compdef nickpit"},
	}
	for _, tc := range tests {
		t.Run(tc.shell, func(t *testing.T) {
			path, err := installCompletion(newRootCmd(), tc.shell)
			if err != nil {
				t.Fatal(err)
			}
			wantPath := filepath.Join(dataHome, filepath.FromSlash(tc.relPath))
			if path != wantPath {
				t.Fatalf("path = %q, want %q", path, wantPath)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tc.want) {
				t.Fatalf("installed completion missing %q", tc.want)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != 0o644 {
				t.Fatalf("mode = %o, want 644", got)
			}
		})
	}
}

func TestCompletionInstallPathUsesBashOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BASH_COMPLETION_USER_DIR", dir)
	path, err := completionInstallPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "nickpit"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}
