package git

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/retrieval/repofs"
)

type Runner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

// LimitedRunner runs git but stops reading after limit bytes and kills the
// command, so a caller with a size cap never has to materialize output it would
// throw away. `git show` on a generated file or a vendored tree can emit
// gigabytes; Run would buffer all of it before the cap is even consulted.
type LimitedRunner interface {
	RunLimited(ctx context.Context, limit int, args ...string) (string, bool, error)
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

// RunLimited streams stdout and stops after limit bytes, reporting whether more
// output was available. Once the limit is hit the command is cancelled instead
// of drained, so neither memory nor git's runtime scales with the size of a diff
// the caller has already decided to discard. A limit of 0 or less reads
// everything, matching Run.
func (r ExecRunner) RunLimited(ctx context.Context, limit int, args ...string) (string, bool, error) {
	if limit <= 0 {
		out, err := r.Run(ctx, args...)
		return out, false, err
	}
	// A cancellable child context is what stops git early; cancelling on every
	// return path also reaps the process when the read fails.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "git", args...)
	if r.RepoRoot != "" {
		cmd.Dir = r.RepoRoot
	}
	// stderr is captured separately here (unlike Run's CombinedOutput) so a huge
	// stdout cannot crowd out the diagnostic git prints on failure.
	var stderr strings.Builder
	cmd.Stderr = &limitedWriter{dst: &stderr, remaining: maxErrorOutputBytes}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", false, fmt.Errorf("git: %s: %w", strings.Join(repofs.SanitizeGitArgs(args), " "), err)
	}
	if err := cmd.Start(); err != nil {
		return "", false, fmt.Errorf("git: %s: %w", strings.Join(repofs.SanitizeGitArgs(args), " "), err)
	}
	// Read one byte past the limit: that byte is the proof that output was cut,
	// and it is dropped from the result.
	out, readErr := io.ReadAll(io.LimitReader(stdout, int64(limit)+1))
	truncated := len(out) > limit
	if truncated {
		out = out[:limit]
		// Kill git before waiting; it would otherwise block writing the rest of
		// the diff into a pipe nobody reads.
		cancel()
	}
	waitErr := cmd.Wait()
	switch {
	case truncated:
		// Wait reports the kill we just caused; the partial output is the result.
		return string(out), true, nil
	case waitErr != nil:
		return "", false, fmt.Errorf("git: %s: %w%s", strings.Join(repofs.SanitizeGitArgs(args), " "), waitErr, outputSnippet([]byte(stderr.String())))
	case readErr != nil:
		return "", false, fmt.Errorf("git: %s: reading output: %w", strings.Join(repofs.SanitizeGitArgs(args), " "), readErr)
	}
	return string(out), false, nil
}

// runLimited applies a size cap through LimitedRunner when the runner supports
// streaming, and falls back to reading everything and clipping otherwise (test
// doubles, alternative runners). The observable result is identical either way.
func runLimited(ctx context.Context, runner Runner, limit int, args ...string) (string, bool, error) {
	if limited, ok := runner.(LimitedRunner); ok {
		return limited.RunLimited(ctx, limit, args...)
	}
	out, err := runner.Run(ctx, args...)
	if err != nil {
		return "", false, err
	}
	if limit > 0 && len(out) > limit {
		return out[:limit], true, nil
	}
	return out, false, nil
}

// limitedWriter keeps only the first remaining bytes written to it, so a command
// that fails after printing a lot on stderr cannot grow the error message
// without bound.
type limitedWriter struct {
	dst       *strings.Builder
	remaining int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.remaining > 0 {
		chunk := p
		if len(chunk) > w.remaining {
			chunk = chunk[:w.remaining]
		}
		w.dst.Write(chunk)
		w.remaining -= len(chunk)
	}
	return len(p), nil
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
