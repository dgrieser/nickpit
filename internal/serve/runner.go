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
	// now stamps log file names; injectable for tests.
	now func() time.Time
}

// NewExecRunner resolves the current binary once.
func NewExecRunner() (*ExecRunner, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("serve: resolving own executable: %w", err)
	}
	return &ExecRunner{Executable: executable, now: time.Now}, nil
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
	// Later entries win in the child's environment, so these override any
	// daemon-level token while the LLM key etc. pass through untouched.
	cmd.Env = append(os.Environ(),
		"NICKPIT_GITLAB_TOKEN="+spec.Token,
		"NICKPIT_GITLAB_BASE_URL="+spec.BaseURL,
	)
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
	if err := os.MkdirAll(spec.LogDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("serve: creating log dir: %w", err)
	}
	slug := strings.ReplaceAll(spec.ProjectPath, "/", "-")
	name := fmt.Sprintf("review-%s-%d-%s.log", slug, spec.IID, now.UTC().Format("2006-01-02-15-04-05"))
	logPath := filepath.Join(spec.LogDir, name)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", nil, fmt.Errorf("serve: creating review log: %w", err)
	}
	return logPath, logFile, nil
}
