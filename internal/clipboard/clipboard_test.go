package clipboard

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestHelpersForPlatformOrder(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		wayland bool
		want    []string
	}{
		{name: "macos", goos: "darwin", want: []string{"pbcopy"}},
		{name: "windows", goos: "windows", want: []string{"clip"}},
		{name: "x11 first without wayland", goos: "linux", want: []string{"xclip", "xsel", "wl-copy", "termux-clipboard-set", "clip.exe"}},
		{name: "wayland first when session is wayland", goos: "linux", wayland: true, want: []string{"wl-copy", "xclip", "xsel", "termux-clipboard-set", "clip.exe"}},
		{name: "bsd shares the unix chain", goos: "freebsd", want: []string{"xclip", "xsel", "wl-copy", "termux-clipboard-set", "clip.exe"}},
		{name: "unknown platform", goos: "plan9", want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := helpersFor(tc.goos, tc.wayland)
			if len(got) != len(tc.want) {
				t.Fatalf("chain = %v, want %v", names(got), tc.want)
			}
			for i, h := range got {
				if h.name != tc.want[i] {
					t.Fatalf("chain = %v, want %v", names(got), tc.want)
				}
			}
		})
	}
}

func TestCopyUsesFirstWorkingHelper(t *testing.T) {
	restore := stub(t)
	defer restore()

	goos = "linux"
	present := map[string]bool{"xsel": true, "wl-copy": true}
	lookPath = func(name string) (string, error) {
		if present[name] {
			return "/usr/bin/" + name, nil
		}
		return "", exec.ErrNotFound
	}
	var used string
	var payload []byte
	run = func(_ context.Context, path string, _ []string, data []byte) error {
		// xsel is present but broken (the headless case): fall through to wl-copy.
		if strings.HasSuffix(path, "xsel") {
			return errors.New("Can't open display")
		}
		used, payload = path, data
		return nil
	}

	helper, err := Copy(context.Background(), []byte("review"))
	if err != nil {
		t.Fatal(err)
	}
	if helper != "wl-copy" || used != "/usr/bin/wl-copy" {
		t.Fatalf("helper = %q via %q, want wl-copy", helper, used)
	}
	if string(payload) != "review" {
		t.Fatalf("payload = %q", payload)
	}
}

func TestCopyErrors(t *testing.T) {
	restore := stub(t)
	defer restore()

	// Nothing installed: the error names the candidates and how to get one.
	goos = "linux"
	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	run = func(context.Context, string, []string, []byte) error { return nil }
	_, err := Copy(context.Background(), []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "no clipboard helper found in PATH") ||
		!strings.Contains(err.Error(), "xclip") || !strings.Contains(err.Error(), "install wl-clipboard") {
		t.Fatalf("missing-helper error = %v", err)
	}

	// Installed but every candidate fails: report each failure.
	lookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	run = func(context.Context, string, []string, []byte) error { return errors.New("boom") }
	_, err = Copy(context.Background(), []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "no clipboard helper succeeded") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("failing-helper error = %v", err)
	}
	if strings.Contains(err.Error(), "not installed") {
		t.Fatalf("nothing was missing, yet the error claims otherwise: %v", err)
	}

	// Mixed: the installed helper fails and the one that would fit the session
	// type is absent, so the install hint must survive alongside the failure.
	getenv = func(string) string { return "wayland-0" }
	lookPath = func(name string) (string, error) {
		if name == "xclip" {
			return "/usr/bin/xclip", nil
		}
		return "", exec.ErrNotFound
	}
	run = func(context.Context, string, []string, []byte) error { return errors.New("Can't open display") }
	_, err = Copy(context.Background(), []byte("x"))
	if err == nil {
		t.Fatal("mixed missing/failing chain should fail")
	}
	for _, want := range []string{"xclip: Can't open display", "not installed: wl-copy", "install wl-clipboard"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("mixed-chain error %v missing %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "install xclip") {
		t.Fatalf("install hint offered for an installed helper: %v", err)
	}
	getenv = func(string) string { return "" }

	// A cancelled caller context stops after the first candidate instead of
	// retrying the whole chain against a dead context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0
	run = func(context.Context, string, []string, []byte) error {
		attempts++
		return context.Canceled
	}
	if _, err = Copy(ctx, []byte("x")); err == nil {
		t.Fatal("cancelled copy should fail")
	}
	if attempts != 1 {
		t.Fatalf("attempts after cancellation = %d, want 1", attempts)
	}

	goos = "plan9"
	if _, err = Copy(context.Background(), []byte("x")); err == nil || !strings.Contains(err.Error(), "no clipboard helper is known for plan9") {
		t.Fatalf("unknown-platform error = %v", err)
	}
}

func TestCopyEncodesForWindowsHelper(t *testing.T) {
	restore := stub(t)
	defer restore()

	goos = "windows"
	lookPath = func(name string) (string, error) { return `C:\Windows\System32\` + name + ".exe", nil }
	var payload []byte
	run = func(_ context.Context, _ string, _ []string, data []byte) error {
		payload = data
		return nil
	}
	if _, err := Copy(context.Background(), []byte("ä")); err != nil {
		t.Fatal(err)
	}
	// UTF-16LE BOM plus U+00E4 as a single little-endian code unit.
	if want := []byte{0xFF, 0xFE, 0xE4, 0x00}; string(payload) != string(want) {
		t.Fatalf("payload = % x, want % x", payload, want)
	}
}

func TestUTF16LEWithBOMEncodesAstralPlane(t *testing.T) {
	// A surrogate pair must survive as two code units, not as a replacement char.
	got := utf16LEWithBOM([]byte("🎉"))
	want := []byte{0xFF, 0xFE, 0x3C, 0xD8, 0x89, 0xDF}
	if string(got) != string(want) {
		t.Fatalf("encoded = % x, want % x", got, want)
	}
}

func TestRunHelperPipesStdinAndReportsStderr(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	ctx := context.Background()
	if err := runHelper(ctx, "/bin/sh", []string{"-c", `read -r line; [ "$line" = "review" ]`}, []byte("review\n")); err != nil {
		t.Fatalf("stdin was not piped: %v", err)
	}
	err := runHelper(ctx, "/bin/sh", []string{"-c", "echo broken >&2; exit 3"}, nil)
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("stderr not surfaced: %v", err)
	}
}

func TestRunHelperReportsMissingStderrCapture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TMPDIR redirection is POSIX-specific")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	// An uncreatable temp file must not turn a working copy into a failure...
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "does-not-exist"))
	if err := runHelper(context.Background(), "/bin/sh", []string{"-c", "exit 0"}, []byte("review")); err != nil {
		t.Fatalf("copy failed because stderr could not be captured: %v", err)
	}
	// ...and when the helper does fail, the error says why its stderr is missing
	// instead of looking silently mute.
	err := runHelper(context.Background(), "/bin/sh", []string{"-c", "echo broken >&2; exit 3"}, nil)
	if err == nil || !strings.Contains(err.Error(), "stderr not captured") {
		t.Fatalf("error = %v, want a note that stderr capture was unavailable", err)
	}
}

func TestCopyTimeoutBoundsAHangingHelper(t *testing.T) {
	restore := stub(t)
	defer restore()

	goos = "darwin"
	lookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	run = func(ctx context.Context, _ string, _ []string, _ []byte) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("helper context has no deadline")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > copyTimeout {
			t.Fatalf("deadline in %v, want within %v", remaining, copyTimeout)
		}
		return nil
	}
	if _, err := Copy(context.Background(), []byte("x")); err != nil {
		t.Fatal(err)
	}
}

// stub swaps the package seams and returns a restore func.
func stub(t *testing.T) func() {
	t.Helper()
	origLook, origRun, origGOOS, origEnv := lookPath, run, goos, getenv
	getenv = func(string) string { return "" }
	return func() {
		lookPath, run, goos, getenv = origLook, origRun, origGOOS, origEnv
	}
}
