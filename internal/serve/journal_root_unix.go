//go:build unix

package serve

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// openJournalRoot creates and opens dir without following any symlink in the
// configured path. The descriptor walk prevents a writable ancestor from
// redirecting traversal between a pathname check and the open. OpenRoot then
// pins the directory used by all later journal operations; the identity check
// ensures it opened the same directory as the no-follow walk.
func openJournalRoot(dir string) (*os.Root, error) {
	clean := filepath.Clean(dir)
	base := "."
	if filepath.IsAbs(clean) {
		base = string(filepath.Separator)
		clean = strings.TrimPrefix(clean, base)
	}

	current, err := os.Open(base)
	if err != nil {
		return nil, err
	}
	defer func() { _ = current.Close() }()

	for _, component := range strings.Split(clean, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		fd, openErr := unix.Openat(
			int(current.Fd()),
			component,
			unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
			0,
		)
		if errors.Is(openErr, unix.ENOENT) {
			if mkdirErr := unix.Mkdirat(int(current.Fd()), component, 0o700); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				return nil, fmt.Errorf("create component %q: %w", component, mkdirErr)
			}
			fd, openErr = unix.Openat(
				int(current.Fd()),
				component,
				unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
				0,
			)
		}
		if openErr != nil {
			return nil, fmt.Errorf("open component %q without symlinks: %w", component, openErr)
		}
		next := os.NewFile(uintptr(fd), component)
		if next == nil {
			_ = unix.Close(fd)
			return nil, fmt.Errorf("open component %q: invalid file descriptor", component)
		}
		if err := current.Close(); err != nil {
			_ = next.Close()
			return nil, fmt.Errorf("close parent of component %q: %w", component, err)
		}
		current = next
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	openedInfo, err := root.Stat(".")
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	walkedInfo, err := current.Stat()
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	if !os.SameFile(openedInfo, walkedInfo) {
		_ = root.Close()
		return nil, fmt.Errorf("directory changed while opening")
	}
	return root, nil
}
