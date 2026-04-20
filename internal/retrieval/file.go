package retrieval

import (
	"context"
	"fmt"
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
	return &FileContent{
		Path:     path,
		Content:  normalizeText(string(data)),
		Language: detectLanguage(path),
	}, nil
}

func (e *LocalEngine) ListFiles(_ context.Context, repoRoot, path string) (*DirectoryListing, error) {
	normalizedPath := strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), "./")
	fullPath := filepath.Join(repoRoot, normalizedPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("retrieval: listing %s: %w", normalizedPath, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if normalizedPath != "" {
			name = normalizedPath + "/" + name
		}
		if entry.IsDir() {
			name += "/"
		}
		files = append(files, name)
	}
	sort.Strings(files)
	return &DirectoryListing{
		Path:  normalizedPath,
		Files: files,
	}, nil
}

func (e *LocalEngine) GetFileSlice(ctx context.Context, repoRoot, path string, start, end int) (*FileSlice, error) {
	full, err := e.GetFile(ctx, repoRoot, path)
	if err != nil {
		return nil, err
	}
	lines := splitLines(full.Content)
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return nil, fmt.Errorf("retrieval: invalid line range %d-%d", start, end)
	}
	return &FileSlice{
		Path:      path,
		StartLine: start,
		EndLine:   end,
		Content:   strings.Join(lines[start-1:end], "\n"),
		Language:  full.Language,
	}, nil
}

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.TrimSuffix(text, "\n")
}

func splitLines(text string) []string {
	text = normalizeText(text)
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
