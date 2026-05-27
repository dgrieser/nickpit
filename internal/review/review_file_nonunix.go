//go:build !unix

package review

import "os"

func openReviewFileNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
