package repofs

import (
	"fmt"
	"net/url"
	"os/exec"
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
	enabled  bool
}

func NewIgnoreMatcher(repoRoot string) IgnoreMatcher {
	if repoRoot == "" {
		return IgnoreMatcher{}
	}
	if _, err := exec.LookPath("git"); err != nil {
		return IgnoreMatcher{}
	}
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--show-toplevel")
	if err := cmd.Run(); err != nil {
		return IgnoreMatcher{}
	}
	return IgnoreMatcher{repoRoot: repoRoot, enabled: true}
}

func (m IgnoreMatcher) IsIgnored(path string, isDir bool) bool {
	if !m.enabled || path == "" {
		return false
	}
	checkPath := filepath.ToSlash(path)
	if isDir {
		checkPath += "/"
	}
	cmd := exec.Command("git", "-C", m.repoRoot, "check-ignore", "-q", "--no-index", "--stdin")
	cmd.Stdin = strings.NewReader(checkPath + "\n")
	return cmd.Run() == nil
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
