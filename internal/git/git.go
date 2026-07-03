package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/retrieval/repofs"
)

type Runner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

type ExecRunner struct {
	RepoRoot string
}

func (r ExecRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if r.RepoRoot != "" {
		cmd.Dir = r.RepoRoot
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Include a truncated tail of the combined output: git prints the
		// actionable reason (missing ref, auth failure, ...) on stderr. The
		// args are sanitized via SanitizeGitArgs and the output itself is
		// token-free (credentials travel via http.extraHeader), so the
		// snippet is safe to surface.
		return "", fmt.Errorf("git: %s: %w%s", strings.Join(repofs.SanitizeGitArgs(args), " "), err, outputSnippet(out))
	}
	return string(out), nil
}

// maxErrorOutputBytes bounds how much command output is attached to an error.
const maxErrorOutputBytes = 2048

// outputSnippet renders the last maxErrorOutputBytes of a failed command's
// combined output as an error suffix, or "" when there is no output.
func outputSnippet(out []byte) string {
	text := strings.TrimSpace(string(out))
	if text == "" {
		return ""
	}
	if len(text) > maxErrorOutputBytes {
		// Advance to a rune boundary so the cut cannot split a multi-byte
		// character (localized git messages are not ASCII-only).
		start := len(text) - maxErrorOutputBytes
		for start < len(text) && !utf8.RuneStart(text[start]) {
			start++
		}
		text = "..." + text[start:]
	}
	return ": " + text
}
