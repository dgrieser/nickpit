package git

import (
	"context"
	"slices"
	"strings"
)

// maxLsFilesPaths caps how many pathspecs one "ls-files" call receives so a
// large change set cannot overflow the command line.
const maxLsFilesPaths = 100

// SymlinkPaths asks a checkout's git index which of paths are stored as symlinks
// (mode 120000), keyed by the path as git reports it.
//
// The index — not the worktree — is the authority: with core.symlinks=false (the
// default on Windows, and a valid setting on Unix) git materializes a symlink
// blob as a regular file holding the target path, so lstat would report a plain
// file and the link target would be reviewed as ordinary text.
//
// Paths the index does not track (a deletion, an untracked file) are simply
// absent from the result, and a failing git call yields no marks rather than a
// guess: a missing mark, never a wrong one. The error is returned so a caller
// that can log it may, but it never invalidates the marks already collected.
func SymlinkPaths(ctx context.Context, runner Runner, paths []string) (map[string]bool, error) {
	if runner == nil || len(paths) == 0 {
		return nil, nil
	}
	symlinks := make(map[string]bool, len(paths))
	var firstErr error
	for chunk := range slices.Chunk(paths, maxLsFilesPaths) {
		args := append([]string{"ls-files", "--stage", "-z", "--"}, chunk...)
		out, err := runner.Run(ctx, args...)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		collectSymlinkEntries(out, symlinks)
	}
	return symlinks, firstErr
}

// collectSymlinkEntries parses "ls-files --stage -z" output. Each NUL-terminated
// entry is "<mode> <object> <stage>\t<path>"; -z keeps the path literal, so it
// needs no unquoting.
func collectSymlinkEntries(out string, symlinks map[string]bool) {
	for entry := range strings.SplitSeq(out, "\x00") {
		meta, path, ok := strings.Cut(entry, "\t")
		if !ok || path == "" {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) == 0 {
			continue
		}
		if NormalizeFileMode(fields[0]) == SymlinkFileMode {
			symlinks[path] = true
		}
	}
}
