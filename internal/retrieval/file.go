package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
