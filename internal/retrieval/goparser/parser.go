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
	var result *Symbol
	if path != "" {
		fullPath := filepath.Join(repoRoot, path)
		result = findSymbolInFile(repoRoot, fullPath, name)
		if result == nil {
			return nil, fmt.Errorf("symbol %q not found in %q", name, path)
		}
		return result, nil
	}

	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		result = findSymbolInFile(repoRoot, path, name)
		if result != nil {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("symbol %q not found", name)
	}
	return result, nil
}

func findSymbolInFile(repoRoot, path, name string) *Symbol {
	fset := token.NewFileSet()
	file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if parseErr != nil {
		return nil
	}
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
		return &Symbol{
			Name:      fn.Name.Name,
			Path:      filepath.ToSlash(rel),
			StartLine: fset.Position(fn.Pos()).Line,
			EndLine:   fset.Position(fn.End()).Line,
			Source:    buf.String(),
		}
	}
	return nil
}
