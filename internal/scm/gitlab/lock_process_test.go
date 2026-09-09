package gitlab

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestUpdateExecutionLockAcrossProcesses(t *testing.T) {
	const envKey = "NICKPIT_TEST_UPDATE_LOCK"
	if project := os.Getenv(envKey); project != "" {
		client := NewClient("https://lock-process.example", "")
		_, release, err := client.TryLockMR(context.Background(), project, 1)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		fmt.Println("locked")
		_, _ = io.Copy(io.Discard, os.Stdin)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	project := "update-execution/" + t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestUpdateExecutionLockAcrossProcesses$")
	cmd.Env = append(os.Environ(), envKey+"="+project)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("child readiness: %q %v", line, err)
	}
	client := NewClient("https://lock-process.example", "")
	if _, _, err := client.TryLockMR(context.Background(), project, 1); !errors.Is(err, ErrLockBusy) {
		t.Fatalf("competing process admitted: %v", err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	_, release, err := client.TryLockMR(context.Background(), project, 1)
	if err != nil {
		t.Fatalf("exit retained lock: %v", err)
	}
	release()
}
