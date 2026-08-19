package git

import (
	"context"
	"slices"
	"strings"
)

// maxTreeQueryPaths caps how many pathspecs one "ls-tree" call receives so a
// large change set cannot overflow the command line.
const maxTreeQueryPaths = 100

// literalPathspec wraps a path so git takes it verbatim. "--" stops option
// parsing but not pathspec magic, and these paths come from SCM payloads: a file
// literally named ":(literal)link" would otherwise be reparsed as magic (missing
// the real file), and one named ":!foo" would turn into an exclusion that
// enumerates the rest of the tree.
func literalPathspec(path string) string {
	return ":(literal)" + path
}

// SymlinkPathsAtRev asks git which of paths are stored as symlinks (mode 120000)
// in rev's tree, keyed by the path as git reports it.
//
// The tree of the reviewed commit — not the index and not the worktree — is the
// authority, for two independent reasons. A checkout can hold a different
// revision than the diff describes (a user-selected repo root for a remote
// review, staged content, a range that is not checked out), and with
// core.symlinks=false (the default on Windows, and a valid setting on Unix) git
// materializes a symlink blob as a regular file holding the target path, so an
// lstat would report a plain file. Addressing the tree by SHA sidesteps both: a
// checkout that does not have that commit simply fails the lookup.
//
// Paths absent from the tree (a deletion, an untracked file) are absent from the
// result, and a failing git call yields no marks rather than a guess: a missing
// mark, never a wrong one. The error is returned so a caller that can log it may,
// but it never invalidates the marks already collected.
func SymlinkPathsAtRev(ctx context.Context, runner Runner, rev string, paths []string) (map[string]bool, error) {
	if runner == nil || rev == "" || len(paths) == 0 {
		return nil, nil
	}
	symlinks := make(map[string]bool, len(paths))
	var firstErr error
	for chunk := range slices.Chunk(paths, maxTreeQueryPaths) {
		args := make([]string, 0, 5+len(chunk))
		args = append(args, "ls-tree", "-z", rev, "--")
		for _, path := range chunk {
			args = append(args, literalPathspec(path))
		}
		out, err := runner.Run(ctx, args...)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		collectTreeSymlinks(out, symlinks)
	}
	return symlinks, firstErr
}

// collectTreeSymlinks parses "ls-tree -z" output. Each NUL-terminated entry is
// "<mode> <type> <object>\t<path>"; -z keeps the path literal, so it needs no
// unquoting.
func collectTreeSymlinks(out string, symlinks map[string]bool) {
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

// FileModes maps a repo-relative path to the post-change git file mode of that
// path, as "git diff --raw" reports it. It fills the gap a patch leaves: git
// prints a mode header only when the mode is new, gone, or changed, so a plain
// rename of an unchanged symlink carries no 120000 anywhere in its patch.
type FileModes map[string]string

// Symlink reports whether path is stored as a symlink according to these modes.
func (m FileModes) Symlink(path string) bool {
	if len(m) == 0 {
		return false
	}
	mode, ok := m[path]
	return ok && NormalizeFileMode(mode) == SymlinkFileMode
}

// RawEntryModes parses the meta token of a "git diff --raw" entry:
//
//	:<srcmode> <dstmode> <srcsha> <dstsha> <status>
//	::<mode1> <mode2> <dstmode> <sha1> <sha2> <dstsha> <status>
//
// The second form is a combined (merge) entry: one leading colon and one source
// column per parent. Returned modes are normalized, so an absent side ("000000")
// comes back as "". ok is false for anything that is not a raw entry.
func RawEntryModes(meta string) (parents []string, dst, status string, ok bool) {
	rest := strings.TrimLeft(meta, ":")
	count := len(meta) - len(rest)
	fields := strings.Fields(rest)
	// One source mode per parent, one destination mode, the same number of object
	// names, and one status column; the paths are separate tokens.
	if count < 1 || len(fields) != 2*(count+1)+1 {
		return nil, "", "", false
	}
	parents = make([]string, 0, count)
	for _, mode := range fields[:count] {
		parents = append(parents, NormalizeFileMode(mode))
	}
	return parents, NormalizeFileMode(fields[count]), fields[len(fields)-1], true
}

// SymlinkModeFromRawEntry picks the mode that describes the side under review of a
// raw entry: the destination when the entry has one, else the parent side. A
// combined deletion has no destination and one source column per parent, and any
// parent that held a symlink means the patch shows a link target — so a symlink
// parent wins over a regular one. Returns "" when no side states a mode.
func SymlinkModeFromRawEntry(parents []string, dst string) string {
	if dst != "" {
		return dst
	}
	fallback := ""
	for _, mode := range parents {
		if mode == SymlinkFileMode {
			return mode
		}
		if fallback == "" {
			fallback = mode
		}
	}
	return fallback
}

// ParseRawFileModes reads the file mode of every entry in "git diff --raw -z"
// output. An entry is "<meta>\0<path>[\0<newpath>]"; a rename or copy adds the
// second path — the destination, which is the one keyed here.
func ParseRawFileModes(out string) FileModes {
	tokens := strings.Split(out, "\x00")
	modes := FileModes{}
	for i := 0; i < len(tokens); i++ {
		if !strings.HasPrefix(tokens[i], ":") {
			continue
		}
		parents, dst, status, ok := RawEntryModes(tokens[i])
		if !ok {
			continue
		}
		paths := 1
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			paths = 2
		}
		if i+paths >= len(tokens) {
			break
		}
		mode := SymlinkModeFromRawEntry(parents, dst)
		if path := tokens[i+paths]; path != "" && mode != "" {
			modes[path] = mode
		}
		i += paths
	}
	return modes
}
