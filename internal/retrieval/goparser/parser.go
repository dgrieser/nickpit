package goparser

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Symbol struct {
	Name      string
	Path      string
	StartLine int
	EndLine   int
	Source    string
}

func FindSymbol(_ context.Context, repoRoot, name, path string) (*Symbol, error) {
	results, err := FindSymbols(context.Background(), repoRoot, name, path)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		if path != "" {
			return nil, fmt.Errorf("symbol %q not found in %q", name, path)
		}
		return nil, fmt.Errorf("symbol %q not found", name)
	}
	return &results[0], nil
}

func FindSymbols(_ context.Context, repoRoot, name, path string) ([]Symbol, error) {
	normalizedPath := normalizeLookupPath(path)
	files, err := collectGoFiles(repoRoot, normalizedPath)
	if err != nil {
		return nil, err
	}
	var results []Symbol
	for _, file := range files {
		results = append(results, findSymbolsInFile(repoRoot, file, name)...)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Path == results[j].Path {
			return results[i].StartLine < results[j].StartLine
		}
		return results[i].Path < results[j].Path
	})
	return results, nil
}

func findSymbolInFile(repoRoot, path, name string) *Symbol {
	results := findSymbolsInFile(repoRoot, path, name)
	if len(results) == 0 {
		return nil
	}
	return &results[0]
}

func findSymbolsInFile(repoRoot, path, name string) []Symbol {
	fset := token.NewFileSet()
	file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if parseErr != nil {
		return nil
	}
	var results []Symbol
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name {
			continue
		}
		var buf strings.Builder
		if printErr := printer.Fprint(&buf, fset, fn); printErr != nil {
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, path)
		results = append(results, Symbol{
			Name:      fn.Name.Name,
			Path:      filepath.ToSlash(rel),
			StartLine: fset.Position(fn.Pos()).Line,
			EndLine:   fset.Position(fn.End()).Line,
			Source:    buf.String(),
		})
	}
	return results
}

func collectGoFiles(repoRoot, path string) ([]string, error) {
	if path != "" {
		fullPath := filepath.Join(repoRoot, filepath.FromSlash(path))
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if filepath.Ext(fullPath) != ".go" {
				return nil, nil
			}
			return []string{fullPath}, nil
		}
	}

	root := repoRoot
	if path != "" {
		root = filepath.Join(repoRoot, filepath.FromSlash(path))
	}
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		files = append(files, path)
		return nil
	})
	return files, err
}
