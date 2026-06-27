package review

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

const allChangedFilesFilteredWarning = "all changed files omitted by filters"

type reviewContextFilter struct {
	includePaths    []*regexp.Regexp
	excludePaths    []*regexp.Regexp
	includeContent  []*regexp.Regexp
	excludeContent  []*regexp.Regexp
	needsFileReads  bool
	hasPathMatchers bool
	enabled         bool
}

func newReviewContextFilter(req model.ReviewRequest) (*reviewContextFilter, error) {
	filter := &reviewContextFilter{}
	var err error
	if filter.includePaths, err = compileFilterRegexes("include_paths", req.IncludePaths); err != nil {
		return nil, err
	}
	if filter.excludePaths, err = compileFilterRegexes("exclude_paths", req.ExcludePaths); err != nil {
		return nil, err
	}
	if filter.includeContent, err = compileFilterRegexes("include_content", req.IncludeContent); err != nil {
		return nil, err
	}
	if filter.excludeContent, err = compileFilterRegexes("exclude_content", req.ExcludeContent); err != nil {
		return nil, err
	}
	filter.needsFileReads = len(filter.includeContent) > 0 || len(filter.excludeContent) > 0
	filter.hasPathMatchers = len(filter.includePaths) > 0 || len(filter.excludePaths) > 0
	filter.enabled = filter.hasPathMatchers || filter.needsFileReads
	return filter, nil
}

func compileFilterRegexes(key string, patterns []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for i, pattern := range patterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("review: %s[%d] invalid regex %q: %w", key, i, pattern, err)
		}
		out = append(out, compiled)
	}
	return out, nil
}

func (e *Engine) applyReviewContextFilter(ctx context.Context, reviewCtx *model.ReviewContext, req model.ReviewRequest, filter *reviewContextFilter) (bool, error) {
	if reviewCtx == nil || filter == nil || !filter.enabled {
		return false, nil
	}
	if filter.needsFileReads {
		if e.retrieval == nil {
			return false, fmt.Errorf("review: content filters require file retrieval")
		}
		if req.RepoRoot == "" {
			return false, fmt.Errorf("review: content filters require a repository checkout")
		}
	}

	paths := collectFilterCandidatePaths(reviewCtx)
	if len(paths) == 0 {
		return false, nil
	}
	statusByPath := make(map[string]model.FileStatus, len(reviewCtx.ChangedFiles))
	for _, file := range reviewCtx.ChangedFiles {
		statusByPath[normalizeReviewPath(file.Path)] = file.Status
	}

	keepByPath := make(map[string]bool, len(paths))
	omitted := make([]string, 0)
	for _, candidate := range paths {
		keep, err := e.keepFilteredPath(ctx, req, filter, candidate, statusByPath[candidate])
		if err != nil {
			return false, err
		}
		keepByPath[candidate] = keep
		if !keep {
			omitted = append(omitted, candidate)
		}
	}
	if len(omitted) == 0 {
		return false, nil
	}

	reviewCtx.ChangedFiles = filterChangedFiles(reviewCtx.ChangedFiles, keepByPath)
	reviewCtx.DiffFiles = filterDiffFiles(reviewCtx.DiffFiles, keepByPath)
	reviewCtx.DiffHunks = filterDiffHunks(reviewCtx.DiffHunks, keepByPath)
	reviewCtx.Comments = filterComments(reviewCtx.Comments, keepByPath)
	reviewCtx.Diff = filterUnifiedDiff(reviewCtx.Diff, keepByPath)
	reviewCtx.OmittedSections = append(reviewCtx.OmittedSections, fmt.Sprintf("files omitted by filters: %s", strings.Join(omitted, ", ")))

	allFiltered := len(reviewCtx.ChangedFiles) == 0 && strings.TrimSpace(reviewCtx.Diff) == "" && len(reviewCtx.DiffFiles) == 0 && len(reviewCtx.DiffHunks) == 0
	if allFiltered {
		reviewCtx.OmittedSections = append(reviewCtx.OmittedSections, allChangedFilesFilteredWarning)
	}
	return allFiltered, nil
}

func (e *Engine) keepFilteredPath(ctx context.Context, req model.ReviewRequest, filter *reviewContextFilter, filePath string, status model.FileStatus) (bool, error) {
	if len(filter.includePaths) > 0 && !matchesAnyFilter(filter.includePaths, filePath) {
		return false, nil
	}
	if matchesAnyFilter(filter.excludePaths, filePath) {
		return false, nil
	}
	if !filter.needsFileReads {
		return true, nil
	}

	content := ""
	contentOK := false
	if status != model.FileDeleted {
		file, err := e.retrieval.GetFile(ctx, req.RepoRoot, filePath)
		if err != nil {
			e.logf(ctx, "Skipping filter content read: path=%s error=%v", filePath, err)
		} else if file != nil {
			content = file.Content
			contentOK = true
		}
	}
	if len(filter.includeContent) > 0 {
		if !contentOK || !matchesAnyFilter(filter.includeContent, content) {
			return false, nil
		}
	}
	if contentOK && matchesAnyFilter(filter.excludeContent, content) {
		return false, nil
	}
	return true, nil
}

func matchesAnyFilter(patterns []*regexp.Regexp, value string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func collectFilterCandidatePaths(ctx *model.ReviewContext) []string {
	seen := make(map[string]struct{})
	add := func(value string) {
		normalized := normalizeReviewPath(value)
		if normalized == "" || normalized == "." {
			return
		}
		seen[normalized] = struct{}{}
	}
	for _, file := range ctx.ChangedFiles {
		add(file.Path)
	}
	for _, hunk := range ctx.DiffHunks {
		add(hunk.FilePath)
	}
	for _, file := range ctx.DiffFiles {
		add(file.FilePath)
	}
	for _, section := range splitUnifiedDiff(ctx.Diff) {
		add(section.path)
	}
	for _, comment := range ctx.Comments {
		add(comment.Path)
	}
	paths := make([]string, 0, len(seen))
	for value := range seen {
		paths = append(paths, value)
	}
	sort.Strings(paths)
	return paths
}

func normalizeReviewPath(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "./")
	value = path.Clean(strings.ReplaceAll(value, "\\", "/"))
	if value == "." {
		return ""
	}
	for strings.HasPrefix(value, "../") {
		value = strings.TrimPrefix(value, "../")
	}
	return strings.TrimPrefix(value, "/")
}

func filterChangedFiles(files []model.ChangedFile, keepByPath map[string]bool) []model.ChangedFile {
	filtered := make([]model.ChangedFile, 0, len(files))
	for _, file := range files {
		if keepByPath[normalizeReviewPath(file.Path)] {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func filterDiffHunks(hunks []model.DiffHunk, keepByPath map[string]bool) []model.DiffHunk {
	filtered := make([]model.DiffHunk, 0, len(hunks))
	for _, hunk := range hunks {
		if keepByPath[normalizeReviewPath(hunk.FilePath)] {
			filtered = append(filtered, hunk)
		}
	}
	return filtered
}

func filterDiffFiles(files []model.DiffFile, keepByPath map[string]bool) []model.DiffFile {
	filtered := make([]model.DiffFile, 0, len(files))
	for _, file := range files {
		if keepByPath[normalizeReviewPath(file.FilePath)] {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func filterComments(comments []model.Comment, keepByPath map[string]bool) []model.Comment {
	filtered := make([]model.Comment, 0, len(comments))
	for _, comment := range comments {
		if strings.TrimSpace(comment.Path) == "" || keepByPath[normalizeReviewPath(comment.Path)] {
			filtered = append(filtered, comment)
		}
	}
	return filtered
}

type diffSection struct {
	path string
	text string
}

func filterUnifiedDiff(diff string, keepByPath map[string]bool) string {
	sections := splitUnifiedDiff(diff)
	if len(sections) == 0 {
		return diff
	}
	var out strings.Builder
	for _, section := range sections {
		if keepByPath[normalizeReviewPath(section.path)] {
			out.WriteString(section.text)
		}
	}
	return out.String()
}

func splitUnifiedDiff(diff string) []diffSection {
	var sections []diffSection
	var current strings.Builder
	currentPath := ""
	inSection := false
	flush := func() {
		if !inSection {
			return
		}
		sections = append(sections, diffSection{path: currentPath, text: current.String()})
		current.Reset()
	}
	for _, line := range strings.SplitAfter(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			inSection = true
			currentPath = parseDiffGitPath(line)
		}
		if inSection {
			current.WriteString(line)
		}
	}
	flush()
	return sections
}

func parseDiffGitPath(line string) string {
	const prefix = "diff --git "
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	// The header is "a/<old> b/<new>". Git leaves spaces unquoted here, so
	// splitting on whitespace mangles paths that contain them. Instead take
	// everything after the last " b/", which is the post-change path unless a
	// path literally contains " b/" (rare enough to fall back on).
	rest := line[len(prefix):]
	if idx := strings.LastIndex(rest, " b/"); idx >= 0 {
		return normalizeReviewPath(rest[idx+len(" b/"):])
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	value := fields[len(fields)-1]
	value = strings.TrimPrefix(value, "b/")
	value = strings.TrimPrefix(value, "a/")
	return normalizeReviewPath(value)
}

func reviewContextAllFiltered(ctx *model.ReviewContext) bool {
	if ctx == nil {
		return false
	}
	return slices.Contains(ctx.OmittedSections, allChangedFilesFilteredWarning)
}
