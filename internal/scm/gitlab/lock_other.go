//go:build !unix && !windows

package gitlab

import (
	"fmt"
	"os"
)

func tryMRFileLock(f *os.File) (bool, error) {
	return false, fmt.Errorf("GitLab writes require OS file-lock support")
}
