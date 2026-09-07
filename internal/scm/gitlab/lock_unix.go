//go:build unix

package gitlab

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
)

func tryMRFileLock(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return false, nil
	}
	return err == nil, err
}
