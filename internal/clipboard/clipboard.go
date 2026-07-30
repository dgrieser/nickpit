// Package clipboard copies text to the host clipboard. There is no portable
// system call for this, so every platform gets a small chain of helper commands
// tried in order: the first one present in PATH that exits cleanly wins. That
// keeps the binary dependency-free (no cgo, no X11 headers) and works the same
// way on macOS, Windows, WSL, Wayland, X11, and Termux.
package clipboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
)

// copyTimeout bounds the whole helper chain, not one attempt: a clipboard helper
// that hangs (X server not answering, wl-copy with an unreachable compositor
// socket) must not hang the command that asked for a copy, and five hanging
// candidates in a row must not multiply the wait.
const copyTimeout = 10 * time.Second

// waitDelay bounds how long a helper's stdin pipe is drained after the process
// is killed. xclip/xsel/wl-copy fork a daemon that inherits the pipe, so without
// it a killed helper could still block on the stdin copy.
const waitDelay = 2 * time.Second

// helper is one clipboard command: the executable, its args, and an optional
// encoder for platforms whose helper does not read UTF-8.
type helper struct {
	name    string
	args    []string
	encode  func([]byte) []byte
	install string // hint printed when nothing worked
}

// Seams for tests: the real implementations shell out.
var (
	lookPath = exec.LookPath
	run      = runHelper
	goos     = runtime.GOOS
	getenv   = os.Getenv
)

// Copy writes data to the system clipboard and returns the name of the helper
// that accepted it. An empty helper chain (unknown platform) or a chain where
// every candidate is missing or failing yields an error naming what to install.
func Copy(ctx context.Context, data []byte) (string, error) {
	helpers := helpersFor(goos, getenv("WAYLAND_DISPLAY") != "")
	if len(helpers) == 0 {
		return "", fmt.Errorf("clipboard: no clipboard helper is known for %s", goos)
	}
	var (
		missing []string
		errs    []error
	)
	runCtx, cancel := context.WithTimeout(ctx, copyTimeout)
	defer cancel()
	for _, h := range helpers {
		path, err := lookPath(h.name)
		if err != nil {
			missing = append(missing, h.name)
			continue
		}
		payload := data
		if h.encode != nil {
			payload = h.encode(data)
		}
		if err = run(runCtx, path, h.args, payload); err == nil {
			return h.name, nil
		}
		// A present-but-failing helper is the common headless case (xclip with
		// no DISPLAY): keep going, and report every failure if none works.
		errs = append(errs, fmt.Errorf("%s: %w", h.name, err))
		// Cancellation or an exhausted chain budget is not this helper's fault
		// and would hit every remaining candidate the same way.
		if runCtx.Err() != nil {
			break
		}
	}
	if len(errs) > 0 {
		return "", fmt.Errorf("clipboard: no clipboard helper succeeded: %w", errors.Join(errs...))
	}
	return "", fmt.Errorf("clipboard: no clipboard helper found in PATH (tried %s); %s",
		strings.Join(missing, ", "), installHint(helpers))
}

// helpersFor returns the helper chain for a GOOS. wayland reports whether
// WAYLAND_DISPLAY is set, which only reorders the Unix chain: both wl-copy and
// xclip are commonly installed, and under XWayland xclip "works" while writing
// to the wrong clipboard, so the session type decides who goes first.
func helpersFor(goos string, wayland bool) []helper {
	switch goos {
	case "darwin":
		return []helper{{name: "pbcopy", install: "pbcopy is part of macOS"}}
	case "windows":
		// clip.exe reads the console codepage for 8-bit input, which mangles
		// anything non-ASCII; UTF-16LE with a BOM is interpreted correctly.
		return []helper{{name: "clip", encode: utf16LEWithBOM, install: "clip.exe is part of Windows"}}
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "illumos", "aix":
		x11 := []helper{
			{name: "xclip", args: []string{"-selection", "clipboard", "-in"}, install: "install xclip"},
			{name: "xsel", args: []string{"--clipboard", "--input"}, install: "install xsel"},
		}
		wl := []helper{{name: "wl-copy", install: "install wl-clipboard"}}
		chain := make([]helper, 0, len(x11)+len(wl)+2)
		if wayland {
			chain = append(append(chain, wl...), x11...)
		} else {
			chain = append(append(chain, x11...), wl...)
		}
		// Termux (Android) and WSL, where the Windows helper is reachable from
		// the Linux side. clip.exe needs the same UTF-16LE treatment as native.
		chain = append(chain,
			helper{name: "termux-clipboard-set", install: "install the Termux:API package"},
			helper{name: "clip.exe", encode: utf16LEWithBOM, install: "clip.exe is part of Windows"},
		)
		return chain
	default:
		return nil
	}
}

// installHint joins the distinct install hints of a helper chain.
func installHint(helpers []helper) string {
	seen := make(map[string]bool, len(helpers))
	hints := make([]string, 0, len(helpers))
	for _, h := range helpers {
		if h.install == "" || seen[h.install] {
			continue
		}
		seen[h.install] = true
		hints = append(hints, h.install)
	}
	return strings.Join(hints, ", or ")
}

// utf16LEWithBOM re-encodes UTF-8 text as little-endian UTF-16 with a byte-order
// mark, the encoding Windows clipboard helpers read losslessly.
func utf16LEWithBOM(data []byte) []byte {
	units := utf16.Encode([]rune(string(data)))
	out := make([]byte, 0, 2*len(units)+2)
	out = append(out, 0xFF, 0xFE)
	for _, u := range units {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

// runHelper pipes data into a clipboard helper and returns its stderr on
// failure.
//
// Stderr goes to a temp *os.File rather than a buffer on purpose: xclip, xsel and
// wl-copy fork a daemon that keeps serving the selection after the parent exits,
// and that daemon inherits whatever fds it was given. With an in-memory writer
// os/exec would hand the child a pipe and wait for EOF on it, which the daemon
// never delivers — so a successful copy would look like a hang (or, with
// WaitDelay, like a failure). A real file needs no reader goroutine, so Wait
// returns as soon as the direct child exits.
func runHelper(ctx context.Context, path string, args []string, data []byte) error {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.WaitDelay = waitDelay
	cmd.Stdin = bytes.NewReader(data)
	if errFile, err := os.CreateTemp("", "nickpit-clipboard-*.err"); err == nil {
		defer func() {
			name := errFile.Name()
			_ = errFile.Close()
			_ = os.Remove(name)
		}()
		cmd.Stderr = errFile
	}
	if err := cmd.Run(); err != nil {
		if msg := helperStderr(cmd.Stderr); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// maxStderrBytes bounds how much helper stderr is attached to an error.
const maxStderrBytes = 2048

// helperStderr reads back what the helper wrote to its stderr file.
func helperStderr(w io.Writer) string {
	f, ok := w.(*os.File)
	if !ok || f == nil {
		return ""
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	out, err := io.ReadAll(io.LimitReader(f, maxStderrBytes))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
