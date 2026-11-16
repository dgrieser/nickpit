package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runRoot runs nickpit with args and returns what it wrote.
func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestCompletionScripts(t *testing.T) {
	for shell, want := range map[string]string{
		"bash":       "__start_nickpit()",
		"zsh":        "#compdef nickpit",
		"fish":       "complete -c nickpit",
		"powershell": "Register-ArgumentCompleter",
	} {
		t.Run(shell, func(t *testing.T) {
			out, err := runRoot(t, "completion", shell)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, want) {
				t.Fatalf("%s completion missing %q", shell, want)
			}
		})
	}
}

func TestCompletionRejectsUnknownShell(t *testing.T) {
	if _, err := runRoot(t, "completion", "tcsh"); err == nil {
		t.Fatal("completion for an unknown shell succeeded")
	}
}

func TestCompletionInstall(t *testing.T) {
	dataHome := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("BASH_COMPLETION_USER_DIR", "")

	for _, tc := range []struct {
		shell, path, want string
	}{
		{"bash", filepath.Join(dataHome, "bash-completion", "completions", "nickpit"), "__start_nickpit()"},
		{"zsh", filepath.Join(dataHome, "zsh", "site-functions", "_nickpit"), "#compdef nickpit"},
		{"fish", filepath.Join(configHome, "fish", "completions", "nickpit.fish"), "complete -c nickpit"},
	} {
		t.Run(tc.shell, func(t *testing.T) {
			out, err := runRoot(t, "completion", tc.shell, "--install")
			if err != nil {
				t.Fatal(err)
			}
			if want := "Installed " + tc.shell + " completion to " + tc.path; strings.TrimSpace(out) != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tc.want) {
				t.Fatalf("installed completion missing %q", tc.want)
			}
		})
	}
}
