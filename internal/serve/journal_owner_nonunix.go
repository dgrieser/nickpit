//go:build !unix

package serve

import (
	"fmt"
	"io/fs"
)

func validateJournalDirOwner(fs.FileInfo) error {
	return fmt.Errorf("ownership validation is unsupported on this platform")
}
