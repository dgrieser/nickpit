package repofs

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func NormalizePath(path string) string {
	normalized := filepath.ToSlash(path)
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimSuffix(normalized, "/")
	if normalized == "." {
		return ""
	}
	return normalized
}

func ResolvePath(repoRoot, path string) (string, string, error) {
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", err
	}
	if path == "" {
		return "", repoAbs, nil
	}
	if filepath.IsAbs(path) {
		return "", "", fmt.Errorf("path %q escapes repository root", path)
	}

	normalized := NormalizePath(path)
	fullPath := filepath.Clean(filepath.Join(repoAbs, filepath.FromSlash(normalized)))
	relPath, err := filepath.Rel(repoAbs, fullPath)
	if err != nil {
		return "", "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path %q escapes repository root", path)
	}
	if relPath == "." {
		return "", repoAbs, nil
	}
	return filepath.ToSlash(relPath), fullPath, nil
}

func RelPath(repoRoot, fullPath string) (string, error) {
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	relPath, err := filepath.Rel(repoAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes repository root", fullPath)
	}
	if relPath == "." {
		return "", nil
	}
	return filepath.ToSlash(relPath), nil
}

type IgnoreMatcher struct {
	repoRoot string
	rules    []ignoreRule
}

func NewIgnoreMatcher(repoRoot string) IgnoreMatcher {
	if repoRoot == "" {
		return IgnoreMatcher{}
	}
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return IgnoreMatcher{}
	}
	rules := loadIgnoreRules(repoAbs)
	return IgnoreMatcher{repoRoot: repoAbs, rules: rules}
}

func (m IgnoreMatcher) IsIgnored(path string, isDir bool) bool {
	if m.repoRoot == "" || path == "" {
		return false
	}
	normalizedPath := NormalizePath(path)
	if normalizedPath == "" {
		return false
	}
	ignored := false
	for _, rule := range m.rules {
		if rule.matches(normalizedPath, isDir) {
			ignored = !rule.negated
		}
	}
	return ignored
}

type ignoreRule struct {
	baseDir       string
	pattern       string
	negated       bool
	directoryOnly bool
	anchored      bool
	matchBase     bool
}

func loadIgnoreRules(repoRoot string) []ignoreRule {
	rules := make([]ignoreRule, 0)
	_ = filepath.WalkDir(repoRoot, func(currentPath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != ".gitignore" {
			return nil
		}
		baseDir, relErr := RelPath(repoRoot, filepath.Dir(currentPath))
		if relErr != nil {
			return nil
		}
		loaded, loadErr := parseIgnoreFile(currentPath, baseDir)
		if loadErr == nil {
			rules = append(rules, loaded...)
		}
		return nil
	})
	return rules
}

func parseIgnoreFile(filePath, baseDir string) ([]ignoreRule, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	rules := make([]ignoreRule, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rule, ok := parseIgnoreRule(baseDir, line)
		if ok {
			rules = append(rules, rule)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func parseIgnoreRule(baseDir, raw string) (ignoreRule, bool) {
	rule := ignoreRule{baseDir: baseDir}
	if strings.HasPrefix(raw, `\!`) || strings.HasPrefix(raw, `\#`) {
		raw = raw[1:]
	}
	if strings.HasPrefix(raw, "!") {
		rule.negated = true
		raw = raw[1:]
	}
	if raw == "" {
		return ignoreRule{}, false
	}
	if strings.HasSuffix(raw, "/") {
		rule.directoryOnly = true
		raw = strings.TrimSuffix(raw, "/")
	}
	if strings.HasPrefix(raw, "/") {
		rule.anchored = true
		raw = strings.TrimPrefix(raw, "/")
	}
	if raw == "" {
		return ignoreRule{}, false
	}
	rule.pattern = raw
	rule.matchBase = !strings.Contains(raw, "/")
	return rule, true
}

func (r ignoreRule) matches(path string, isDir bool) bool {
	rel, ok := relativeToBase(path, r.baseDir)
	if !ok || rel == "" {
		return false
	}
	if r.directoryOnly {
		for _, candidate := range candidateDirectories(rel, isDir) {
			if r.matchesSingle(candidate) {
				return true
			}
		}
		return false
	}
	return r.matchesSingle(rel)
}

func (r ignoreRule) matchesSingle(rel string) bool {
	if rel == "" {
		return false
	}
	if r.matchBase {
		if r.anchored {
			name, _ := path.Match(r.pattern, path.Base(rel))
			return name && path.Dir(rel) == "."
		}
		for _, segment := range strings.Split(rel, "/") {
			matched, err := path.Match(r.pattern, segment)
			if err == nil && matched {
				return true
			}
		}
		return false
	}
	matched, err := path.Match(r.pattern, rel)
	return err == nil && matched
}

func relativeToBase(path, baseDir string) (string, bool) {
	if baseDir == "" {
		return path, true
	}
	if path == baseDir {
		return "", true
	}
	prefix := baseDir + "/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	return strings.TrimPrefix(path, prefix), true
}

func candidateDirectories(path string, isDir bool) []string {
	parts := strings.Split(path, "/")
	if !isDir && len(parts) > 0 {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return nil
	}
	candidates := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		candidates = append(candidates, strings.Join(parts[:i+1], "/"))
	}
	return candidates
}

func SanitizeGitArgs(args []string) []string {
	sanitized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		current := args[i]
		if current == "-c" && i+1 < len(args) {
			sanitized = append(sanitized, current, sanitizeGitArg(args[i+1]))
			i++
			continue
		}
		sanitized = append(sanitized, sanitizeGitArg(current))
	}
	return sanitized
}

func sanitizeGitArg(arg string) string {
	if strings.HasPrefix(arg, "http.extraHeader=") {
		return "http.extraHeader=<redacted>"
	}
	if strings.Contains(arg, "Authorization:") {
		return "<redacted>"
	}
	parsed, err := url.Parse(arg)
	if err == nil && parsed.User != nil {
		username := parsed.User.Username()
		if username == "" {
			username = "<redacted>"
		}
		parsed.User = url.UserPassword(username, "<redacted>")
		return parsed.String()
	}
	return arg
}
