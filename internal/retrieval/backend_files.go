package retrieval

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func collectFilesByExt(repoRoot string, scope lookupScope, exts map[string]struct{}) ([]string, error) {
	if scope.IsFile {
		if _, ok := exts[strings.ToLower(filepath.Ext(scope.Path))]; !ok {
			return nil, nil
		}
		return []string{filepath.Join(repoRoot, filepath.FromSlash(scope.Path))}, nil
	}

	root := repoRoot
	if scope.IsDir && scope.Path != "" {
		root = filepath.Join(repoRoot, filepath.FromSlash(scope.Path))
	}
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
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
