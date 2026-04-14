package retrieval

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LocalEngine struct{}

func NewLocalEngine() *LocalEngine {
	return &LocalEngine{}
}

func (e *LocalEngine) GetFile(_ context.Context, repoRoot, path string) (*FileContent, error) {
	fullPath := filepath.Join(repoRoot, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("retrieval: reading %s: %w", path, err)
	}
	lines := splitLines(string(data))
	lineMap := make(map[int]int, len(lines))
	offset := 0
	for i, line := range lines {
		lineMap[i+1] = offset
		offset += len(line) + 1
	}
	return &FileContent{
		Path:     path,
		Lines:    lines,
		LineMap:  lineMap,
		Language: detectLanguage(path),
	}, nil
}

func (e *LocalEngine) GetFileSlice(ctx context.Context, repoRoot, path string, start, end int) (*FileSlice, error) {
	full, err := e.GetFile(ctx, repoRoot, path)
	if err != nil {
		return nil, err
	}
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > len(full.Lines) {
		end = len(full.Lines)
	}
	if start > end {
		return nil, fmt.Errorf("retrieval: invalid line range %d-%d", start, end)
	}
	return &FileSlice{
		Path:      path,
		StartLine: start,
		EndLine:   end,
		Lines:     append([]string(nil), full.Lines[start-1:end]...),
		Language:  full.Language,
	}, nil
}

func (e *LocalEngine) GetAdjacentFiles(_ context.Context, repoRoot, path string, mode AdjacencyMode) ([]FileRef, error) {
	fullPath := filepath.Join(repoRoot, path)
	dir := filepath.Dir(fullPath)
	switch mode {
	case SameDir:
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		out := make([]FileRef, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			rel, _ := filepath.Rel(repoRoot, filepath.Join(dir, entry.Name()))
			if rel == path {
				continue
			}
			out = append(out, FileRef{Path: filepath.ToSlash(rel)})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
		return out, nil
	case Siblings:
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		var out []FileRef
		for _, entry := range entries {
			name := entry.Name()
			if name == filepath.Base(path) {
				continue
			}
			if strings.HasPrefix(name, base) {
				rel, _ := filepath.Rel(repoRoot, filepath.Join(dir, name))
				out = append(out, FileRef{Path: filepath.ToSlash(rel)})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
		return out, nil
	case Imports:
		if filepath.Ext(path) != ".go" {
			return nil, nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, fullPath, nil, parser.ImportsOnly)
		if err != nil {
			return nil, err
		}
		var out []FileRef
		for _, imp := range file.Imports {
			name := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(name, ".") {
				out = append(out, FileRef{Path: name})
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("retrieval: unknown adjacency mode")
	}
}

func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return []string{}
	}
	return strings.Split(text, "\n")
}

func detectLanguage(path string) string {
	switch filepath.Ext(path) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	default:
		return "text"
	}
}
