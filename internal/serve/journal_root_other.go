//go:build !unix

package serve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// openJournalRoot rejects symlinks before opening on platforms without the
// Unix descriptor-relative no-follow walk.
func openJournalRoot(dir string) (*os.Root, error) {
	if err := rejectJournalPathSymlinks(dir); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := rejectJournalPathSymlinks(dir); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	pathInfo, err := os.Lstat(dir)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	rootInfo, err := root.Stat(".")
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	if !os.SameFile(pathInfo, rootInfo) {
		_ = root.Close()
		return nil, fmt.Errorf("directory changed while opening")
	}
	return root, nil
}

func rejectJournalPathSymlinks(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	volume := filepath.VolumeName(abs)
	current := volume + string(filepath.Separator)
	rest := strings.TrimPrefix(abs, current)
	for component := range strings.SplitSeq(rest, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("state directory component %q is a symlink", current)
		}
	}
	return nil
}
