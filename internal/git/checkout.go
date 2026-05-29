package git

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

type CheckoutOptions struct {
	Workdir string
	Token   string
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
	// CloneURL/HeadRef/HeadSHA come straight from SCM API responses and, for
	// fork PRs/MRs, are attacker-controlled. Reject values that git would parse
	// as options (leading "-") and non-network clone-URL schemes (ext::, file://)
	// that can execute commands or read local files, before any value reaches
	// the git command line.
	if err := validateCloneURL(spec.CloneURL); err != nil {
		return "", nil, err
	}
	if err := validateGitRef("head ref", spec.HeadRef); err != nil {
		return "", nil, err
	}
	if err := validateGitRef("head sha", spec.HeadSHA); err != nil {
		return "", nil, err
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

	args := append(m.authArgs(spec.Provider, opts.Token), "clone", "--no-checkout", "--", spec.CloneURL, repoRoot)
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
	runner := m.newRunner(repoRoot)
	auth := m.authArgs(spec.Provider, opts.Token)

	if spec.HeadRef == "" {
		return m.fetchTarget(ctx, runner, auth, spec.CloneURL, spec.HeadSHA)
	}

	present, err := m.remoteHasRef(ctx, runner, auth, spec.CloneURL, spec.HeadRef)
	if err != nil {
		return err
	}
	if present {
		return m.fetchTarget(ctx, runner, auth, spec.CloneURL, spec.HeadRef)
	}
	if spec.HeadSHA == "" {
		return fmt.Errorf("git: ref %q not found on remote %s and no head_sha to fall back to", spec.HeadRef, spec.CloneURL)
	}
	if err := m.fetchTarget(ctx, runner, auth, spec.CloneURL, spec.HeadSHA); err != nil {
		return fmt.Errorf("git: ref %q missing on %s; fallback fetch of head_sha %q failed: %w", spec.HeadRef, spec.CloneURL, spec.HeadSHA, err)
	}
	return nil
}

func (m *CheckoutManager) remoteHasRef(ctx context.Context, runner Runner, auth []string, cloneURL, ref string) (bool, error) {
	args := make([]string, 0, len(auth)+3)
	args = append(args, auth...)
	args = append(args, "ls-remote", "--", cloneURL, ref)
	out, err := runner.Run(ctx, args...)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (m *CheckoutManager) fetchTarget(ctx context.Context, runner Runner, auth []string, cloneURL, target string) error {
	args := make([]string, 0, len(auth)+5)
	args = append(args, auth...)
	args = append(args, "fetch", "--depth", "1", "--", cloneURL, target)
	_, err := runner.Run(ctx, args...)
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

// validateGitRef rejects a ref/SHA that git would interpret as an option. Git
// itself forbids branch names beginning with "-", so a legitimate ref never
// trips this; an injected value like "--upload-pack=..." does. Empty is allowed
// (the caller decides whether a value is required).
func validateGitRef(field, value string) error {
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("git: refusing %s that starts with '-': %q", field, value)
	}
	return nil
}

// validateCloneURL rejects clone URLs that begin with "-" (option injection)
// and the local/transport-helper schemes git supports that can run commands or
// read local files (ext::, file://). Network transports (http/https/ssh/git)
// and scp-like syntax (git@host:path, which has no URL scheme) are allowed.
func validateCloneURL(raw string) error {
	if strings.HasPrefix(raw, "-") {
		return fmt.Errorf("git: refusing clone URL that starts with '-': %q", raw)
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "ext::") || strings.HasPrefix(lower, "file:") {
		return fmt.Errorf("git: refusing non-network clone URL: %q", raw)
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" {
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https", "ssh", "git":
		default:
			return fmt.Errorf("git: unsupported clone URL scheme %q", parsed.Scheme)
		}
	}
	return nil
}
