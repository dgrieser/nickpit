package serve

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeReviewScript echoes its argv and the nickpit env vars, so tests can
// assert the exact child invocation without running a real review.
const fakeReviewScript = `#!/bin/sh
echo "args:$@"
echo "token:$NICKPIT_GITLAB_TOKEN"
echo "base_url:$NICKPIT_GITLAB_BASE_URL"
exit ${FAKE_REVIEW_EXIT:-0}
`

func writeFakeReview(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake not portable to windows")
	}
	path := filepath.Join(t.TempDir(), "fake-nickpit")
	if err := os.WriteFile(path, []byte(fakeReviewScript), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func testSpec(t *testing.T) ReviewSpec {
	return ReviewSpec{
		ProjectPath: "platform/api",
		IID:         7,
		Token:       "group-token",
		BaseURL:     "https://gitlab.example.com",
		ConfigPath:  ".nickpit.yaml",
		ExtraArgs:   []string{"--profile", "default"},
		LogDir:      t.TempDir(),
	}
}

func TestExecRunnerInvocation(t *testing.T) {
	runner := &ExecRunner{Executable: writeFakeReview(t), now: time.Now}
	spec := testSpec(t)

	exitCode, logPath, err := runner.Run(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	if !strings.Contains(log, "args:gitlab mr --repo platform/api --id 7 --publish --config .nickpit.yaml --profile default") {
		t.Fatalf("argv wrong:\n%s", log)
	}
	if !strings.Contains(log, "token:group-token") || !strings.Contains(log, "base_url:https://gitlab.example.com") {
		t.Fatalf("env wrong:\n%s", log)
	}
	if !strings.Contains(filepath.Base(logPath), "review-platform-api-7-") {
		t.Fatalf("log name = %s", filepath.Base(logPath))
	}
}

// The child environment must not contain other groups' tokens or any
// webhook secret, regardless of which env var names carry them.
func TestExecRunnerScrubsSecretsFromChildEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake not portable to windows")
	}
	path := filepath.Join(t.TempDir(), "envdump")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nenv\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SMOKE_OTHER_GROUP_TOKEN", "other-secret-token")
	t.Setenv("SMOKE_HOOK_SECRET", "hook-secret-value")
	t.Setenv("SMOKE_HARMLESS", "keep-me")

	runner := &ExecRunner{
		Executable:  path,
		scrubValues: map[string]bool{"other-secret-token": true, "hook-secret-value": true},
		now:         time.Now,
	}
	_, logPath, err := runner.Run(context.Background(), testSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	env := string(data)
	if strings.Contains(env, "other-secret-token") || strings.Contains(env, "hook-secret-value") {
		t.Fatalf("child env leaks daemon secrets:\n%s", env)
	}
	if !strings.Contains(env, "SMOKE_HARMLESS=keep-me") {
		t.Fatal("unrelated env vars must pass through")
	}
	if !strings.Contains(env, "NICKPIT_GITLAB_TOKEN=group-token") {
		t.Fatal("own group token must be injected")
	}
}

func TestExecRunnerPrivateLogPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permissions")
	}
	runner := &ExecRunner{Executable: writeFakeReview(t), now: time.Now}
	spec := testSpec(t)
	spec.LogDir = filepath.Join(t.TempDir(), "logs")

	_, logPath, err := runner.Run(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	dirInfo, err := os.Stat(spec.LogDir)
	if err != nil {
		t.Fatal(err)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		t.Fatalf("log dir mode = %o, want 700", mode)
	}
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if mode := fileInfo.Mode().Perm(); mode != 0o600 {
		t.Fatalf("log file mode = %o, want 600", mode)
	}
}

func TestExecRunnerChildFailureExitCode(t *testing.T) {
	runner := &ExecRunner{Executable: writeFakeReview(t), now: time.Now}
	spec := testSpec(t)
	t.Setenv("FAKE_REVIEW_EXIT", "1")

	exitCode, _, err := runner.Run(context.Background(), spec)
	if err != nil {
		t.Fatalf("child exit 1 must not be a runner error, got %v", err)
	}
	if exitCode != 1 {
		t.Fatalf("exit code = %d", exitCode)
	}
}

func TestExecRunnerCancelTerminatesChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signals not portable")
	}
	path := filepath.Join(t.TempDir(), "sleeper")
	script := "#!/bin/sh\ntrap 'exit 143' TERM\nsleep 30 &\nwait $!\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &ExecRunner{Executable: path, now: time.Now}
	spec := testSpec(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var exitCode int
	go func() {
		defer close(done)
		exitCode, _, _ = runner.Run(ctx, spec)
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("child did not terminate after cancel")
	}
	if exitCode == 0 {
		t.Fatalf("exit code = %d, want non-zero after SIGTERM", exitCode)
	}
}
