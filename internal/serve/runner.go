package serve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// logDrainGrace bounds how long Run waits for the output-copy goroutine to
// finish after the child exits. Normally the pipe EOFs immediately; this only
// bites when a leaked descendant holds the write end open, in which case the
// reader is force-closed so cancellation is never held hostage to a stray
// grandchild.
const logDrainGrace = 2 * time.Second

// ReviewSpec describes one review to execute in a child process.
type ReviewSpec struct {
	ProjectPath string
	IID         int
	Token       string
	BaseURL     string
	ConfigPath  string
	ExtraArgs   []string
	LogDir      string
	// HeadSHA and Trigger label the review's log stream (see LogSink); they do
	// not affect the child invocation.
	HeadSHA string
	Trigger string
}

// ReviewRunner executes one review and reports the child's exit code and the
// path of its captured log.
type ReviewRunner interface {
	Run(ctx context.Context, spec ReviewSpec) (exitCode int, logPath string, err error)
}

// ChatSpec describes one discussion-thread reply to execute in a child process
// (`nickpit chat --gitlab ... --reply-discussion`). The child reads the thread,
// gates on the thread's root marker, runs the discussion agent, and posts the
// reply back into the thread, so the daemon itself stays free of LLM logic.
type ChatSpec struct {
	ProjectPath  string
	IID          int
	DiscussionID string
	// NoteID is the triggering note; the child answers only when this note is
	// still the latest reply, so racing/redelivered replies do not double-answer.
	NoteID     int
	Token      string
	BaseURL    string
	ConfigPath string
	ExtraArgs  []string
	LogDir     string
}

// ChatRunner executes one discussion-thread reply in a child process.
type ChatRunner interface {
	RunChat(ctx context.Context, spec ChatSpec) (exitCode int, logPath string, err error)
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
	// sink opens a per-review durable log stream (e.g. Loki) that the child's
	// output is tee'd into alongside the on-disk log. Nil is treated as
	// NoopSink, so a runner built without a sink behaves exactly as before.
	sink LogSink
	// now stamps log file names; injectable for tests.
	now func() time.Time
}

// NewExecRunner resolves the current binary once. scrubValues lists secret
// values that must never reach a review child's environment. sink receives a
// mirror of each review's output; pass NoopSink{} (or nil) to disable shipping.
func NewExecRunner(scrubValues []string, sink LogSink) (*ExecRunner, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("serve: resolving own executable: %w", err)
	}
	if sink == nil {
		sink = NoopSink{}
	}
	runner := &ExecRunner{Executable: executable, scrubValues: make(map[string]bool, len(scrubValues)), sink: sink, now: time.Now}
	for _, value := range scrubValues {
		if value != "" {
			runner.scrubValues[value] = true
		}
	}
	return runner, nil
}

// childEnv is the daemon environment minus every entry whose value is a
// configured secret, plus the credentials for this child's group.
func (r *ExecRunner) childEnv(token, baseURL string) []string {
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
		"NICKPIT_GITLAB_TOKEN="+token,
		"NICKPIT_GITLAB_BASE_URL="+baseURL,
	)
}

func (r *ExecRunner) Run(ctx context.Context, spec ReviewSpec) (int, string, error) {
	logPath, logFile, err := createChildLog("review", spec.ProjectPath, spec.IID, spec.LogDir, r.now())
	if err != nil {
		return -1, "", err
	}
	defer func() { _ = logFile.Close() }()

	// Tee the child's output into a durable log stream alongside the on-disk
	// file. The stream is opened per review and flushed on every exit path
	// (success, failure, SIGTERM/abort) by the deferred Close.
	sink := r.sink
	if sink == nil {
		sink = NoopSink{}
	}
	stream := sink.Open(StreamMeta{
		Project: spec.ProjectPath,
		IID:     spec.IID,
		Trigger: spec.Trigger,
		HeadSHA: spec.HeadSHA,
	})
	defer func() { _ = stream.Close() }()

	args := []string{"gitlab", "mr", "--repo", spec.ProjectPath, "--id", strconv.Itoa(spec.IID), "--publish"}
	if spec.ConfigPath != "" {
		args = append(args, "--config", spec.ConfigPath)
	}
	args = append(args, spec.ExtraArgs...)

	cmd := exec.CommandContext(ctx, r.Executable, args...)
	cmd.Env = r.childEnv(spec.Token, spec.BaseURL)
	return r.runLoggedChild(cmd, logFile, stream, logPath)
}

// RunChat spawns `nickpit chat --gitlab ... --reply-discussion` so a discussion
// reply runs isolated in its own process; the daemon never loads the LLM engine.
// The child self-gates (a thread nickpit did not start is a quiet no-op) and
// posts its answer back into the thread itself.
func (r *ExecRunner) RunChat(ctx context.Context, spec ChatSpec) (int, string, error) {
	logPath, logFile, err := createChildLog("chat", spec.ProjectPath, spec.IID, spec.LogDir, r.now())
	if err != nil {
		return -1, "", err
	}
	defer func() { _ = logFile.Close() }()

	args := []string{
		"chat", "--gitlab",
		"--repo", spec.ProjectPath,
		"--id", strconv.Itoa(spec.IID),
		"--reply-discussion", spec.DiscussionID,
	}
	if spec.NoteID > 0 {
		args = append(args, "--reply-note", strconv.Itoa(spec.NoteID))
	}
	if spec.ConfigPath != "" {
		args = append(args, "--config", spec.ConfigPath)
	}
	args = append(args, spec.ExtraArgs...)

	cmd := exec.CommandContext(ctx, r.Executable, args...)
	cmd.Env = r.childEnv(spec.Token, spec.BaseURL)
	// Chat children are not shipped to the durable stream; the on-disk log is
	// enough for a short reply. Discard the stream side of the tee.
	return r.runLoggedChild(cmd, logFile, noopWriteCloser{}, logPath)
}

// runLoggedChild runs cmd with its stdout/stderr fanned out to logFile and
// stream, returning the child's exit code (0 or the process exit code) and only
// a non-nil error for a spawn/transport failure. Shared by Run and RunChat.
func (r *ExecRunner) runLoggedChild(cmd *exec.Cmd, logFile *os.File, stream io.WriteCloser, logPath string) (int, string, error) {
	// The child's stdout/stderr go to a real pipe (an *os.File): os/exec dups
	// an *os.File straight to the child and neither spawns a copy goroutine nor
	// makes Wait block on it, so cancellation stays as prompt as writing to a
	// plain file. We own the read end and fan it out to the on-disk log and the
	// durable stream ourselves. bestEffortWriter guarantees a misbehaving sink
	// can never fail the authoritative file write.
	pr, pw, err := os.Pipe()
	if err != nil {
		return -1, logPath, fmt.Errorf("serve: creating log pipe: %w", err)
	}
	copyDone := make(chan struct{})
	go func() {
		defer close(copyDone)
		_, _ = io.Copy(io.MultiWriter(logFile, bestEffortWriter{stream}), pr)
	}()

	cmd.Stdout = pw
	cmd.Stderr = pw
	// On context cancel: SIGTERM so the child's own signal handling cleans up
	// its clones; SIGKILL after WaitDelay if it lingers.
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 30 * time.Second

	err = cmd.Run()
	// Close the parent's write end so the reader sees EOF once the child and
	// its descendants release their dups. A leaked grandchild (e.g. a stray
	// clone) can hold the pipe open, so bound the drain and force the reader
	// closed rather than block cancellation on it; the deferred logFile/stream
	// Close must not race the copy goroutine, hence the wait for copyDone.
	_ = pw.Close()
	select {
	case <-copyDone:
		_ = pr.Close()
	case <-time.After(logDrainGrace):
		// Force the reader closed so a leaked grandchild holding the write end
		// cannot keep the copy goroutine (and thus cancellation) blocked.
		_ = pr.Close()
		<-copyDone
	}
	if err == nil {
		return 0, logPath, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), logPath, nil
	}
	return -1, logPath, err
}

// bestEffortWriter wraps a log-sink stream so a misbehaving sink can never
// break a review: Write always reports full success and discards any error
// from the wrapped writer. The Loki stream already never errors; this is
// belt-and-braces so the runner's "logging never fails a review" guarantee
// does not rely on the sink's good behavior.
type bestEffortWriter struct{ w io.Writer }

func (b bestEffortWriter) Write(p []byte) (int, error) {
	_, _ = b.w.Write(p)
	return len(p), nil
}

func createChildLog(prefix, projectPath string, iid int, logDir string, now time.Time) (string, *os.File, error) {
	// Private permissions: child logs carry prompts, diffs, and model
	// output. MkdirAll leaves an existing directory's mode untouched.
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return "", nil, fmt.Errorf("serve: creating log dir: %w", err)
	}
	slug := strings.ReplaceAll(projectPath, "/", "-")
	name := fmt.Sprintf("%s-%s-%d-%s.log", prefix, slug, iid, now.UTC().Format("2006-01-02-15-04-05"))
	logPath := filepath.Join(logDir, name)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", nil, fmt.Errorf("serve: creating child log: %w", err)
	}
	return logPath, logFile, nil
}
