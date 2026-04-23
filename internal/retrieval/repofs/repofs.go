package repofs

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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
	repoRoot       string
	rulesByBaseDir map[string][]ignoreRule
	statusCache    *sync.Map
}

type ignoreMatcherEntry struct {
	once    sync.Once
	matcher IgnoreMatcher
}

var ignoreMatcherCache sync.Map

func NewIgnoreMatcher(repoRoot string) IgnoreMatcher {
	if repoRoot == "" {
		return IgnoreMatcher{}
	}
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return IgnoreMatcher{}
	}
	actual, _ := ignoreMatcherCache.LoadOrStore(repoAbs, &ignoreMatcherEntry{})
	entry := actual.(*ignoreMatcherEntry)
	entry.once.Do(func() {
		rulesByBaseDir := loadIgnoreRules(repoAbs)
		entry.matcher = IgnoreMatcher{
			repoRoot:       repoAbs,
			rulesByBaseDir: rulesByBaseDir,
			statusCache:    &sync.Map{},
		}
	})
	return entry.matcher
}

func (m IgnoreMatcher) IsIgnored(path string, isDir bool) bool {
	if m.repoRoot == "" || path == "" {
		return false
	}
	normalizedPath := NormalizePath(path)
	if normalizedPath == "" {
		return false
	}
	return m.isIgnoredCached(normalizedPath, isDir)
}

type ignoreRule struct {
	baseDir       string
	pattern       string
	negated       bool
	directoryOnly bool
	anchored      bool
	matchBase     bool
	patternParts  []ignorePatternPart
}

type ignorePatternPart struct {
	globstar bool
	regex    *regexp.Regexp
}

func (m IgnoreMatcher) matchStatus(path string, isDir bool) bool {
	ignored := false
	for _, baseDir := range applicableBaseDirs(path, isDir) {
		for _, rule := range m.rulesByBaseDir[baseDir] {
			if rule.matches(path, isDir) {
				ignored = !rule.negated
			}
		}
	}
	return ignored
}

func (m IgnoreMatcher) isIgnoredCached(path string, isDir bool) bool {
	cacheKey := fmt.Sprintf("%t:%s", isDir, path)
	if m.statusCache != nil {
		if cached, ok := m.statusCache.Load(cacheKey); ok {
			return cached.(bool)
		}
	}
	ignored := m.matchStatus(path, isDir)
	if !isDir {
		for _, ancestor := range candidateDirectories(path, false) {
			if m.isIgnoredCached(ancestor, true) {
				ignored = true
				break
			}
		}
	}
	if m.statusCache != nil {
		m.statusCache.Store(cacheKey, ignored)
	}
	return ignored
}

func loadIgnoreRules(repoRoot string) map[string][]ignoreRule {
	rulesByBaseDir := make(map[string][]ignoreRule)
	matcher := IgnoreMatcher{repoRoot: repoRoot, rulesByBaseDir: rulesByBaseDir, statusCache: &sync.Map{}}
	walkIgnoreRules(repoRoot, "", matcher)
	return rulesByBaseDir
}

func walkIgnoreRules(fullDir, relDir string, matcher IgnoreMatcher) {
	entries, err := os.ReadDir(fullDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() != ".gitignore" {
			continue
		}
		loaded, loadErr := parseIgnoreFile(filepath.Join(fullDir, entry.Name()), relDir)
		if loadErr == nil && len(loaded) > 0 {
			matcher.rulesByBaseDir[relDir] = append(matcher.rulesByBaseDir[relDir], loaded...)
		}
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == ".git" {
			continue
		}
		childRel := name
		if relDir != "" {
			childRel = relDir + "/" + name
		}
		if matcher.IsIgnored(childRel, true) {
			continue
		}
		walkIgnoreRules(filepath.Join(fullDir, name), childRel, matcher)
	}
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
	parts, err := compilePatternParts(raw)
	if err != nil {
		return ignoreRule{}, false
	}
	rule.patternParts = parts
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
			return matchCompiledComponent(r.patternParts[0], pathBase(rel)) && pathDir(rel) == "."
		}
		for _, segment := range strings.Split(rel, "/") {
			if matchCompiledComponent(r.patternParts[0], segment) {
				return true
			}
		}
		return false
	}
	pathParts := strings.Split(rel, "/")
	return matchParts(r.patternParts, pathParts)
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

func applicableBaseDirs(path string, isDir bool) []string {
	dir := pathDir(path)
	if dir == "." || dir == "" {
		return []string{""}
	}
	parts := strings.Split(dir, "/")
	baseDirs := make([]string, 0, len(parts)+1)
	baseDirs = append(baseDirs, "")
	for i := 0; i < len(parts); i++ {
		baseDirs = append(baseDirs, strings.Join(parts[:i+1], "/"))
	}
	return baseDirs
}

func matchParts(patternParts []ignorePatternPart, pathParts []string) bool {
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}
	if patternParts[0].globstar {
		if len(patternParts) == 1 {
			return true
		}
		for i := 0; i <= len(pathParts); i++ {
			if matchParts(patternParts[1:], pathParts[i:]) {
				return true
			}
		}
		return false
	}
	if len(pathParts) == 0 || !matchCompiledComponent(patternParts[0], pathParts[0]) {
		return false
	}
	return matchParts(patternParts[1:], pathParts[1:])
}

func compilePatternParts(pattern string) ([]ignorePatternPart, error) {
	rawParts := strings.Split(pattern, "/")
	parts := make([]ignorePatternPart, 0, len(rawParts))
	for _, part := range rawParts {
		if part == "**" {
			parts = append(parts, ignorePatternPart{globstar: true})
			continue
		}
		compiled, err := compileComponentPattern(part)
		if err != nil {
			return nil, err
		}
		parts = append(parts, ignorePatternPart{regex: compiled})
	}
	return parts, nil
}

func compileComponentPattern(pattern string) (*regexp.Regexp, error) {
	var regex strings.Builder
	regex.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch ch {
		case '*':
			regex.WriteString(`[^/]*`)
		case '?':
			regex.WriteString(`[^/]`)
		case '[':
			end := i + 1
			escaped := false
			for end < len(pattern) {
				if escaped {
					escaped = false
					end++
					continue
				}
				if pattern[end] == '\\' {
					escaped = true
					end++
					continue
				}
				if pattern[end] == ']' {
					break
				}
				end++
			}
			if end >= len(pattern) {
				regex.WriteString(`\[`)
				continue
			}
			class := pattern[i : end+1]
			regex.WriteString(class)
			i = end
		case '.', '+', '^', '$', '{', '}', '(', ')', '|', '\\':
			regex.WriteByte('\\')
			regex.WriteByte(ch)
		default:
			regex.WriteByte(ch)
		}
	}
	regex.WriteString("$")
	return regexp.Compile(regex.String())
}

func matchCompiledComponent(part ignorePatternPart, value string) bool {
	if part.globstar || part.regex == nil {
		return false
	}
	return part.regex.MatchString(value)
}

func pathBase(value string) string {
	parts := strings.Split(value, "/")
	return parts[len(parts)-1]
}

func pathDir(value string) string {
	if idx := strings.LastIndex(value, "/"); idx >= 0 {
		return value[:idx]
	}
	return "."
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
		parsed.User = url.UserPassword("<redacted>", "<redacted>")
		return parsed.String()
	}
	return arg
}
