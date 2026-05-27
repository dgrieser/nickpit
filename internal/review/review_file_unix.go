//go:build unix

package review

import (
	"os"
	"syscall"
)

func openReviewFileNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
}
