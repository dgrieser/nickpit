package git

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgrieser/nickpit/internal/model"
)

type CheckoutOptions struct {
	Workdir string
	Token     string
}

type CheckoutManager struct {
	newRunner func(repoRoot string) Runner
	mkdirTemp func(dir, pattern string) (string, error)
	removeAll func(path string) error
	stat      func(path string) (os.FileInfo, error)
}

func NewCheckoutManager() *CheckoutManager {
	return &CheckoutManager{
		newRunner: func(repoRoot string) Runner {
			return ExecRunner{RepoRoot: repoRoot}
		},
		mkdirTemp: os.MkdirTemp,
		removeAll: os.RemoveAll,
		stat:      os.Stat,
	}
}

func (m *CheckoutManager) Prepare(ctx context.Context, spec model.CheckoutSpec, opts CheckoutOptions) (string, func(), error) {
	if spec.CloneURL == "" {
		return "", nil, fmt.Errorf("git: missing clone URL for %s", spec.Repo)
	}
	if spec.HeadRef == "" && spec.HeadSHA == "" {
		return "", nil, fmt.Errorf("git: missing head revision for %s", spec.Repo)
	}
	if opts.Workdir != "" {
		return m.prepareWorktree(ctx, spec, opts)
	}
	return m.prepareClone(ctx, spec, opts)
}

func (m *CheckoutManager) prepareClone(ctx context.Context, spec model.CheckoutSpec, opts CheckoutOptions) (string, func(), error) {
	repoRoot, err := m.mkdirTemp("", "nickpit-clone-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = m.removeAll(repoRoot) }

	args := append(m.authArgs(spec.Provider, opts.Token), "clone", "--no-checkout", spec.CloneURL, repoRoot)
	if _, err := m.newRunner("").Run(ctx, args...); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := m.fetchRevision(ctx, repoRoot, spec, opts); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := m.checkoutDetached(ctx, repoRoot, spec); err != nil {
		cleanup()
		return "", nil, err
	}
	return repoRoot, cleanup, nil
}

func (m *CheckoutManager) prepareWorktree(ctx context.Context, spec model.CheckoutSpec, opts CheckoutOptions) (string, func(), error) {
	localRepo, err := filepath.Abs(opts.Workdir)
	if err != nil {
		return "", nil, err
	}
	if _, err := m.stat(localRepo); err != nil {
		return "", nil, fmt.Errorf("git: local repo %s: %w", localRepo, err)
	}
	repoRunner := m.newRunner(localRepo)
	if _, err := repoRunner.Run(ctx, "rev-parse", "--is-inside-work-tree"); err != nil {
		return "", nil, fmt.Errorf("git: validating local repo %s: %w", localRepo, err)
	}
	if err := m.fetchRevision(ctx, localRepo, spec, opts); err != nil {
		return "", nil, err
	}

	worktreeRoot, err := m.mkdirTemp("", "nickpit-worktree-*")
	if err != nil {
		return "", nil, err
	}
	worktreePath := filepath.Join(worktreeRoot, "repo")
	if _, err := repoRunner.Run(ctx, "worktree", "add", "--detach", worktreePath, m.checkoutTarget(spec)); err != nil {
		_ = m.removeAll(worktreeRoot)
		return "", nil, err
	}
	cleanup := func() {
		_, _ = repoRunner.Run(context.Background(), "worktree", "remove", "--force", worktreePath)
		_ = m.removeAll(worktreeRoot)
	}
	return worktreePath, cleanup, nil
}

func (m *CheckoutManager) fetchRevision(ctx context.Context, repoRoot string, spec model.CheckoutSpec, opts CheckoutOptions) error {
	args := append(m.authArgs(spec.Provider, opts.Token), "fetch", "--depth", "1", spec.CloneURL)
	ref := spec.HeadRef
	if ref == "" {
		ref = spec.HeadSHA
	}
	args = append(args, ref)
	_, err := m.newRunner(repoRoot).Run(ctx, args...)
	return err
}

func (m *CheckoutManager) checkoutDetached(ctx context.Context, repoRoot string, spec model.CheckoutSpec) error {
	_, err := m.newRunner(repoRoot).Run(ctx, "checkout", "--detach", m.checkoutTarget(spec))
	return err
}

func (m *CheckoutManager) checkoutTarget(spec model.CheckoutSpec) string {
	if spec.HeadSHA != "" {
		return spec.HeadSHA
	}
	return "FETCH_HEAD"
}

func (m *CheckoutManager) authArgs(provider model.ReviewMode, token string) []string {
	if token == "" {
		return nil
	}
	creds := "x-access-token:" + token
	if provider == model.ModeGitLab {
		creds = "oauth2:" + token
	}
	header := "http.extraHeader=Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
	return []string{"-c", header}
}
