package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

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
		return "", fmt.Errorf("git: %s: %w", strings.Join(repofs.SanitizeGitArgs(args), " "), err)
	}
	return string(out), nil
}
