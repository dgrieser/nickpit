package retrieval

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/retrieval/repofs"
)

func collectFilesByExt(repoRoot string, scope lookupScope, exts map[string]struct{}) ([]string, error) {
	if scope.IsFile {
		if _, ok := exts[strings.ToLower(filepath.Ext(scope.Path))]; !ok {
			return nil, nil
		}
		_, fullPath, err := repofs.ResolvePath(repoRoot, scope.Path)
		if err != nil {
			return nil, err
		}
		return []string{fullPath}, nil
	}

	root := repoRoot
	if scope.IsDir && scope.Path != "" {
		_, resolvedRoot, err := repofs.ResolvePath(repoRoot, scope.Path)
		if err != nil {
			return nil, err
		}
		root = resolvedRoot
	}
	files := make([]string, 0)
	ignores := repofs.NewIgnoreMatcher(repoRoot)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			relPath, relErr := repofs.RelPath(repoRoot, path)
			if relErr == nil && relPath != "" && ignores.IsIgnored(relPath, true) {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, relErr := repofs.RelPath(repoRoot, path)
		if relErr != nil {
			return relErr
		}
		if ignores.IsIgnored(relPath, false) {
			return nil
		}
		if _, ok := exts[strings.ToLower(filepath.Ext(path))]; ok {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// stripLineComment returns line with a trailing single-line comment removed,
// ignoring any marker that appears inside a string literal (e.g. the // in
// "http://x"). It is a best-effort single-line scanner: it tracks the quote
// chars in quoteChars (honoring \-escapes) and cuts at the first marker found
// outside a string. It does NOT model multi-line strings, template/f-string
// interpolation, regex literals, or /* */ block comments — those need state
// carried across lines and remain accepted best-effort gaps. Byte iteration is
// safe because all delimiters are ASCII and never appear inside a UTF-8
// multibyte sequence.
func stripLineComment(line, marker, quoteChars string) string {
	var quote byte // 0 = outside a string; otherwise the open quote char
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++ // skip the escaped char
			} else if c == quote {
				quote = 0
			}
		case strings.IndexByte(quoteChars, c) >= 0:
			quote = c
		case strings.HasPrefix(line[i:], marker):
			return line[:i]
		}
	}
	return line
}
