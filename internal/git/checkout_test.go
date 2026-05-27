package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

type recordedCall struct {
	repoRoot string
	args     []string
}

type stubRunnerFactory struct {
	calls []recordedCall
	errs  map[string]error
	fail  func(repoRoot string, args []string) error
	out   func(repoRoot string, args []string) string
}

func (f *stubRunnerFactory) runner(repoRoot string) Runner {
	return stubRunner{repoRoot: repoRoot, factory: f}
}

func (f *stubRunnerFactory) key(repoRoot string, args []string) string {
	return repoRoot + "|" + strings.Join(args, "\x00")
}

type stubRunner struct {
	repoRoot string
	factory  *stubRunnerFactory
}

func (r stubRunner) Run(_ context.Context, args ...string) (string, error) {
	r.factory.calls = append(r.factory.calls, recordedCall{repoRoot: r.repoRoot, args: append([]string(nil), args...)})
	if r.factory.fail != nil {
		if err := r.factory.fail(r.repoRoot, args); err != nil {
			return "", err
		}
	}
	if err := r.factory.errs[r.factory.key(r.repoRoot, args)]; err != nil {
		return "", err
	}
	if r.factory.out != nil {
		return r.factory.out(r.repoRoot, args), nil
	}
	return "", nil
}

func hasSubcommand(args []string, name string) int {
	for i, a := range args {
		if a == name {
			return i
		}
	}
	return -1
}

func lsRemoteFound(_ string, args []string) string {
	if hasSubcommand(args, "ls-remote") >= 0 {
		return "abc123\trefs/heads/" + args[len(args)-1] + "\n"
	}
	return ""
}

func TestCheckoutManagerPrepareClone(t *testing.T) {
	factory := &stubRunnerFactory{errs: map[string]error{}, out: lsRemoteFound}
	manager := NewCheckoutManager()
	manager.newRunner = factory.runner

	repoRoot, cleanup, err := manager.Prepare(context.Background(), model.CheckoutSpec{
		Provider: model.ModeGitHub,
		Repo:     "owner/repo",
		CloneURL: "https://github.com/owner/repo.git",
		HeadRef:  "feature",
		HeadSHA:  "deadbeef",
	}, CheckoutOptions{Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if repoRoot == "" {
		t.Fatal("expected repo root")
	}
	if cleanup == nil {
		t.Fatal("expected cleanup")
	}
	t.Cleanup(cleanup)

	if len(factory.calls) != 4 {
		t.Fatalf("calls = %d", len(factory.calls))
	}
	if got := factory.calls[0].args; len(got) < 5 || got[2] != "clone" {
		t.Fatalf("clone args = %#v", got)
	}
	if got := factory.calls[1].args; len(got) < 5 || got[2] != "ls-remote" || got[len(got)-1] != "feature" {
		t.Fatalf("ls-remote args = %#v", got)
	}
	if got := factory.calls[2].args; len(got) < 7 || got[2] != "fetch" || got[len(got)-1] != "feature" {
		t.Fatalf("fetch args = %#v", got)
	}
	if got := factory.calls[3].args; len(got) != 3 || got[0] != "checkout" || got[2] != "deadbeef" {
		t.Fatalf("checkout args = %#v", got)
	}
}

func TestCheckoutManagerPrepareWorktree(t *testing.T) {
	localRepo := t.TempDir()
	factory := &stubRunnerFactory{errs: map[string]error{}, out: lsRemoteFound}
	manager := NewCheckoutManager()
	manager.newRunner = factory.runner

	repoRoot, cleanup, err := manager.Prepare(context.Background(), model.CheckoutSpec{
		Provider: model.ModeGitLab,
		Repo:     "group/project",
		CloneURL: "https://gitlab.com/group/project.git",
		HeadRef:  "feature",
		HeadSHA:  "cafebabe",
	}, CheckoutOptions{Workdir: localRepo, Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if repoRoot == "" {
		t.Fatal("expected repo root")
	}
	if cleanup == nil {
		t.Fatal("expected cleanup")
	}
	cleanup()

	if len(factory.calls) != 5 {
		t.Fatalf("calls = %d", len(factory.calls))
	}
	if got := factory.calls[0].args; len(got) != 2 || got[0] != "rev-parse" {
		t.Fatalf("validate args = %#v", got)
	}
	if got := factory.calls[1].args; len(got) < 5 || got[2] != "ls-remote" || got[len(got)-1] != "feature" {
		t.Fatalf("ls-remote args = %#v", got)
	}
	if got := factory.calls[2].args; len(got) < 7 || got[2] != "fetch" || got[len(got)-1] != "feature" {
		t.Fatalf("fetch args = %#v", got)
	}
	if got := factory.calls[3].args; len(got) != 5 || got[0] != "worktree" || got[1] != "add" || got[4] != "cafebabe" {
		t.Fatalf("worktree args = %#v", got)
	}
	if got := factory.calls[4].args; len(got) != 4 || got[0] != "worktree" || got[1] != "remove" {
		t.Fatalf("cleanup args = %#v", got)
	}
}

func TestCheckoutManagerPrepareRequiresExistingWorkdir(t *testing.T) {
	manager := NewCheckoutManager()
	_, _, err := manager.Prepare(context.Background(), model.CheckoutSpec{
		Provider: model.ModeGitHub,
		Repo:     "owner/repo",
		CloneURL: "https://github.com/owner/repo.git",
		HeadRef:  "feature",
	}, CheckoutOptions{Workdir: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckoutManagerPrepareCleansUpCloneOnFailure(t *testing.T) {
	factory := &stubRunnerFactory{errs: map[string]error{}}
	manager := NewCheckoutManager()
	manager.newRunner = factory.runner
	tempParent := t.TempDir()
	var repoRoot string
	manager.mkdirTemp = func(dir, pattern string) (string, error) {
		path, err := os.MkdirTemp(tempParent, pattern)
		repoRoot = path
		return path, err
	}
	manager.removeAll = os.RemoveAll
	factory.fail = func(_ string, args []string) error {
		if len(args) > 2 && args[2] == "clone" {
			return errors.New("clone failed")
		}
		return nil
	}

	_, _, err := manager.Prepare(context.Background(), model.CheckoutSpec{
		Provider: model.ModeGitHub,
		Repo:     "owner/repo",
		CloneURL: "https://github.com/owner/repo.git",
		HeadRef:  "feature",
	}, CheckoutOptions{Token: "secret"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, statErr := os.Stat(repoRoot); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("repo root still exists: %v", statErr)
	}
}

func TestCheckoutManagerFallsBackToHeadSHAWhenRefMissing(t *testing.T) {
	factory := &stubRunnerFactory{errs: map[string]error{}}
	manager := NewCheckoutManager()
	manager.newRunner = factory.runner

	_, cleanup, err := manager.Prepare(context.Background(), model.CheckoutSpec{
		Provider: model.ModeGitLab,
		Repo:     "group/project",
		CloneURL: "https://gitlab.com/group/project.git",
		HeadRef:  "missing-branch",
		HeadSHA:  "f970cb9b",
	}, CheckoutOptions{Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	if len(factory.calls) != 4 {
		t.Fatalf("calls = %d", len(factory.calls))
	}
	if got := factory.calls[1].args; got[2] != "ls-remote" || got[len(got)-1] != "missing-branch" {
		t.Fatalf("ls-remote args = %#v", got)
	}
	if got := factory.calls[2].args; got[2] != "fetch" || got[len(got)-1] != "f970cb9b" {
		t.Fatalf("fallback fetch args = %#v", got)
	}
	if got := factory.calls[3].args; got[0] != "checkout" || got[2] != "f970cb9b" {
		t.Fatalf("checkout args = %#v", got)
	}
}

func TestCheckoutManagerErrorsWhenRefMissingAndNoHeadSHA(t *testing.T) {
	factory := &stubRunnerFactory{errs: map[string]error{}}
	manager := NewCheckoutManager()
	manager.newRunner = factory.runner
	tempParent := t.TempDir()
	manager.mkdirTemp = func(dir, pattern string) (string, error) {
		return os.MkdirTemp(tempParent, pattern)
	}
	manager.removeAll = os.RemoveAll

	_, _, err := manager.Prepare(context.Background(), model.CheckoutSpec{
		Provider: model.ModeGitLab,
		Repo:     "group/project",
		CloneURL: "https://gitlab.com/group/project.git",
		HeadRef:  "missing-branch",
	}, CheckoutOptions{Token: "secret"})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "missing-branch") || !strings.Contains(msg, "no head_sha") {
		t.Fatalf("error missing context: %q", msg)
	}
	// clone + ls-remote only; no fetch attempted.
	if len(factory.calls) != 2 {
		t.Fatalf("calls = %d", len(factory.calls))
	}
	if got := factory.calls[1].args; got[2] != "ls-remote" {
		t.Fatalf("expected ls-remote, got %#v", got)
	}
}

func TestCheckoutManagerWrapsCombinedErrorWhenSHAFallbackFails(t *testing.T) {
	factory := &stubRunnerFactory{errs: map[string]error{}}
	factory.fail = func(_ string, args []string) error {
		idx := hasSubcommand(args, "fetch")
		if idx >= 0 && args[len(args)-1] == "f970cb9b" {
			return errors.New("remote rejected SHA")
		}
		return nil
	}
	manager := NewCheckoutManager()
	manager.newRunner = factory.runner
	tempParent := t.TempDir()
	manager.mkdirTemp = func(dir, pattern string) (string, error) {
		return os.MkdirTemp(tempParent, pattern)
	}
	manager.removeAll = os.RemoveAll

	_, _, err := manager.Prepare(context.Background(), model.CheckoutSpec{
		Provider: model.ModeGitLab,
		Repo:     "group/project",
		CloneURL: "https://gitlab.com/group/project.git",
		HeadRef:  "missing-branch",
		HeadSHA:  "f970cb9b",
	}, CheckoutOptions{Token: "secret"})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"missing-branch", "f970cb9b", "remote rejected SHA"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error missing %q: %s", want, msg)
		}
	}
}

func TestCheckoutManagerSkipsLsRemoteWhenHeadRefEmpty(t *testing.T) {
	factory := &stubRunnerFactory{errs: map[string]error{}}
	manager := NewCheckoutManager()
	manager.newRunner = factory.runner

	_, cleanup, err := manager.Prepare(context.Background(), model.CheckoutSpec{
		Provider: model.ModeGitHub,
		Repo:     "owner/repo",
		CloneURL: "https://github.com/owner/repo.git",
		HeadSHA:  "deadbeef",
	}, CheckoutOptions{Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	if len(factory.calls) != 3 {
		t.Fatalf("calls = %d", len(factory.calls))
	}
	for _, call := range factory.calls {
		if hasSubcommand(call.args, "ls-remote") >= 0 {
			t.Fatalf("unexpected ls-remote call: %#v", call.args)
		}
	}
	if got := factory.calls[1].args; got[2] != "fetch" || got[len(got)-1] != "deadbeef" {
		t.Fatalf("fetch args = %#v", got)
	}
}

func TestExecRunnerRedactsAuthHeadersInErrors(t *testing.T) {
	runner := ExecRunner{}
	_, err := runner.Run(context.Background(),
		"-c", "http.extraHeader=Authorization: Basic c2VjcmV0",
		"definitely-not-a-git-subcommand",
		"https://user:secret@example.com/repo.git",
	)
	if err == nil {
		t.Fatal("expected error")
	}
	message := err.Error()
	for _, forbidden := range []string{"c2VjcmV0", "secret@example.com", "Authorization: Basic"} {
		if strings.Contains(message, forbidden) {
			t.Fatalf("error leaked secret: %q", message)
		}
	}
	if !strings.Contains(message, "http.extraHeader=<redacted>") {
		t.Fatalf("error did not redact header: %q", message)
	}
}
