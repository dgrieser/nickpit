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
