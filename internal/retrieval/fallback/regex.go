package fallback

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Symbol struct {
	Name      string
	Path      string
	StartLine int
	EndLine   int
	Source    string
	Language  string
}

var patterns = []struct {
	language   string
	extensions map[string]struct{}
	def        *regexp.Regexp
}{
	{language: "python", extensions: set(".py"), def: regexp.MustCompile(`^\s*def\s+(\w+)\s*\(`)},
	{language: "javascript", extensions: set(".js", ".ts"), def: regexp.MustCompile(`^\s*(?:function\s+(\w+)|const\s+(\w+)\s*=\s*(?:async\s*)?\()`)},
	{language: "rust", extensions: set(".rs"), def: regexp.MustCompile(`^\s*fn\s+(\w+)`)},
	{language: "java", extensions: set(".java"), def: regexp.MustCompile(`^\s*(?:public|private|protected)?\s*(?:static\s+)?[\w<>\[\]]+\s+(\w+)\s*\(`)},
}

func FindSymbol(_ context.Context, repoRoot, name string) (*Symbol, error) {
	var result *Symbol
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := filepath.Ext(path)
		pattern := findPattern(ext)
		if pattern == nil {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for idx, line := range lines {
			matches := pattern.def.FindStringSubmatch(line)
			if len(matches) == 0 {
				continue
			}
			funcName := firstNonEmpty(matches[1:]...)
			if funcName != name {
				continue
			}
			end := idx + 1
			for ; end < len(lines); end++ {
				if strings.TrimSpace(lines[end]) == "" {
					break
				}
			}
			rel, _ := filepath.Rel(repoRoot, path)
			result = &Symbol{
				Name:      name,
				Path:      filepath.ToSlash(rel),
				StartLine: idx + 1,
				EndLine:   end,
				Source:    strings.Join(lines[idx:end], "\n"),
				Language:  pattern.language,
			}
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

func findPattern(ext string) *struct {
	language   string
	extensions map[string]struct{}
	def        *regexp.Regexp
} {
	for i := range patterns {
		if _, ok := patterns[i].extensions[ext]; ok {
			return &patterns[i]
		}
	}
	return nil
}

func set(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
