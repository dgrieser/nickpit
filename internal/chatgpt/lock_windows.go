//go:build windows

package chatgpt

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFile takes an exclusive lock on path, so concurrent nickpit processes
// (a serve daemon and its chat workers) never spend the same rotating refresh
// token twice. LockFileEx blocks until the lock is granted and is released
// with the handle, so a crashed holder cannot leave it locked.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	handle := windows.Handle(f.Fd())
	overlapped := new(windows.Overlapped)
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = windows.UnlockFileEx(handle, 0, 1, 0, overlapped)
		_ = f.Close()
	}, nil
}
