//go:build unix

package serve

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

func validateJournalDirOwner(info fs.FileInfo) error {
	return validateJournalDirOwnerAs(info, uint32(os.Geteuid()))
}

func validateJournalDirOwnerAs(info fs.FileInfo, want uint32) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot inspect owner")
	}
	if stat.Uid != want {
		return fmt.Errorf("owned by uid %d, want effective uid %d", stat.Uid, want)
	}
	return nil
}
