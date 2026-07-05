package serve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ReviewSpec describes one review to execute in a child process.
type ReviewSpec struct {
	ProjectPath string
	IID         int
	Token       string
	BaseURL     string
	ConfigPath  string
	ExtraArgs   []string
	LogDir      string
}

// ReviewRunner executes one review and reports the child's exit code and the
// path of its captured log.
type ReviewRunner interface {
	Run(ctx context.Context, spec ReviewSpec) (exitCode int, logPath string, err error)
}

// ExecRunner spawns the nickpit binary itself (`gitlab mr ... --publish`) so
// every review runs isolated in its own process; a crashing or leaking review
// can never take the daemon down.
type ExecRunner struct {
	Executable string
	// scrubValues are secret values (all group tokens and webhook secrets)
	// removed from the child environment. The daemon's environment typically
	// holds every group's credentials via the ${VAR} references in
	// server.yaml; a review child must only receive the one token injected
	// for its group. Matching by value works regardless of variable naming.
	scrubValues map[string]bool
	// now stamps log file names; injectable for tests.
	now func() time.Time
}

// NewExecRunner resolves the current binary once. scrubValues lists secret
// values that must never reach a review child's environment.
func NewExecRunner(scrubValues []string) (*ExecRunner, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("serve: resolving own executable: %w", err)
	}
	runner := &ExecRunner{Executable: executable, scrubValues: make(map[string]bool, len(scrubValues)), now: time.Now}
	for _, value := range scrubValues {
		if value != "" {
			runner.scrubValues[value] = true
		}
	}
	return runner, nil
}

// childEnv is the daemon environment minus every entry whose value is a
// configured secret, plus the credentials for this review's group.
func (r *ExecRunner) childEnv(spec ReviewSpec) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if _, value, ok := strings.Cut(entry, "="); ok && r.scrubValues[value] {
			continue
		}
		env = append(env, entry)
	}
	// Later entries win in the child's environment, so these override any
	// daemon-level token while the LLM key etc. pass through untouched.
	return append(env,
		"NICKPIT_GITLAB_TOKEN="+spec.Token,
		"NICKPIT_GITLAB_BASE_URL="+spec.BaseURL,
	)
}

func (r *ExecRunner) Run(ctx context.Context, spec ReviewSpec) (int, string, error) {
	logPath, logFile, err := createReviewLog(spec, r.now())
	if err != nil {
		return -1, "", err
	}
	defer func() { _ = logFile.Close() }()

	args := []string{"gitlab", "mr", "--repo", spec.ProjectPath, "--id", strconv.Itoa(spec.IID), "--publish"}
	if spec.ConfigPath != "" {
		args = append(args, "--config", spec.ConfigPath)
	}
	args = append(args, spec.ExtraArgs...)

	cmd := exec.CommandContext(ctx, r.Executable, args...)
	cmd.Env = r.childEnv(spec)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// On context cancel: SIGTERM so the child's own signal handling cleans up
	// its clones; SIGKILL after WaitDelay if it lingers.
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 30 * time.Second

	err = cmd.Run()
	if err == nil {
		return 0, logPath, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), logPath, nil
	}
	return -1, logPath, err
}

func createReviewLog(spec ReviewSpec, now time.Time) (string, *os.File, error) {
	// Private permissions: review logs carry prompts, diffs, and model
	// output. MkdirAll leaves an existing directory's mode untouched.
	if err := os.MkdirAll(spec.LogDir, 0o700); err != nil {
		return "", nil, fmt.Errorf("serve: creating log dir: %w", err)
	}
	slug := strings.ReplaceAll(spec.ProjectPath, "/", "-")
	name := fmt.Sprintf("review-%s-%d-%s.log", slug, spec.IID, now.UTC().Format("2006-01-02-15-04-05"))
	logPath := filepath.Join(spec.LogDir, name)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", nil, fmt.Errorf("serve: creating review log: %w", err)
	}
	return logPath, logFile, nil
}
