package review

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
)

type codeLocationRepairResult struct {
	Repaired    int
	RetryFields []string
}

type codeLocationRepairQuery struct {
	path string
	code string
}

func (e *Engine) responseCodeLocationRepairer(repoRoot string) func(context.Context, *llm.ReviewResponse) codeLocationRepairResult {
	if e == nil || e.retrieval == nil || strings.TrimSpace(repoRoot) == "" {
		return nil
	}
	return func(ctx context.Context, resp *llm.ReviewResponse) codeLocationRepairResult {
		if ctx == nil {
			ctx = context.Background()
		}
		repairer := &codeLocationRepairer{
			engine:    e,
			repoRoot:  repoRoot,
			retrieval: e.retrieval,
			findCache: make(map[codeLocationRepairQuery]*retrieval.FindLinesResult),
		}
		return repairer.repairResponse(ctx, resp)
	}
}

type codeLocationRepairer struct {
	engine    *Engine
	repoRoot  string
	retrieval retrieval.Engine
	findCache map[codeLocationRepairQuery]*retrieval.FindLinesResult
	retrySeen map[string]struct{}
}

type contentRepairStatus int

const (
	contentRepairNoMatch contentRepairStatus = iota
	contentRepairRepaired
	contentRepairInconclusive
)

func (r *codeLocationRepairer) repairResponse(ctx context.Context, resp *llm.ReviewResponse) codeLocationRepairResult {
	var result codeLocationRepairResult
	if r == nil || resp == nil {
		return result
	}
	for i := range resp.Findings {
		finding := &resp.Findings[i]
		prefix := fmt.Sprintf("findings[%d]", i)
		changed, _ := r.repairLocation(ctx, prefix+".code_location", &finding.CodeLocation, &result)
		result.Repaired += changed
		result.Repaired += r.repairSuggestions(ctx, prefix+".suggestions", finding.Suggestions, &result)
		if finding.Finalization != nil {
			result.Repaired += r.repairSuggestions(ctx, prefix+".finalization.suggestions", finding.Finalization.Suggestions, &result)
		}
		if finding.Summarization != nil {
			result.Repaired += r.repairSuggestions(ctx, prefix+".summarization.suggestions", finding.Summarization.Suggestions, &result)
		}
	}
	if result.Repaired > 0 {
		r.logf(ctx, "Code location repair completed: repaired=%d retry_fields=%d", result.Repaired, len(result.RetryFields))
	}
	return result
}

func (r *codeLocationRepairer) repairSuggestions(ctx context.Context, prefix string, suggestions []model.Suggestion, result *codeLocationRepairResult) int {
	repaired := 0
	for i := range suggestions {
		field := fmt.Sprintf("%s[%d].code_location", prefix, i)
		changed, ok := r.repairLocation(ctx, field, &suggestions[i].CodeLocation, result)
		if ok {
			// Sync the top-level line range from the validated code location
			// unconditionally on success — a location that validated without
			// needing changes can still disagree with a stale LineRange copy.
			suggestions[i].LineRange = suggestions[i].CodeLocation.LineRange
		}
		repaired += changed
	}
	return repaired
}

// repairLocation validates (and where needed repairs) one code location.
// It returns how many locations actually changed (0 or 1) and whether the
// location validated successfully; a false second return means a retry field
// was recorded.
func (r *codeLocationRepairer) repairLocation(ctx context.Context, field string, loc *model.CodeLocation, result *codeLocationRepairResult) (int, bool) {
	if loc == nil {
		r.addRetry(result, field)
		return 0, false
	}
	loc.FilePath = normalizeToolPath(strings.TrimSpace(loc.FilePath))
	loc.Content = normalizeCodeLocationContent(loc.Content)
	if loc.FilePath == "" {
		r.addRetry(result, field+".file_path")
		return 0, false
	}
	original := *loc
	if loc.Content != "" {
		switch r.repairFromContent(ctx, field, loc) {
		case contentRepairRepaired:
			return changedCodeLocation(original, *loc), true
		case contentRepairInconclusive:
			r.addRetry(result, field+".content_or_line_range")
			return 0, false
		}
	}
	if hasAnyLineAnchor(loc.LineRange) {
		if r.repairFromRange(ctx, field, loc) {
			return changedCodeLocation(original, *loc), true
		}
	}
	r.addRetry(result, field+".content_or_line_range")
	return 0, false
}

func (r *codeLocationRepairer) repairFromContent(ctx context.Context, field string, loc *model.CodeLocation) contentRepairStatus {
	match, status := r.findBestContentMatch(ctx, loc.FilePath, loc.Content, loc.LineRange)
	if status == contentRepairRepaired {
		r.applyFindLinesLocation(ctx, field, loc, match, "content")
		return contentRepairRepaired
	}
	if status == contentRepairInconclusive {
		return contentRepairInconclusive
	}
	lines := splitNormalizedCodeLines(loc.Content)
	if len(lines) == 0 {
		return contentRepairNoMatch
	}
	status = r.repairFromFirstLastLines(ctx, field, loc, lines)
	if status == contentRepairRepaired {
		return contentRepairRepaired
	}
	if status == contentRepairInconclusive {
		return contentRepairInconclusive
	}
	return r.repairFromAnyContentLine(ctx, field, loc, lines)
}

func (r *codeLocationRepairer) findBestContentMatch(ctx context.Context, path, content string, hint model.LineRange) (retrieval.FindLinesLocation, contentRepairStatus) {
	result, err := r.findLines(ctx, path, content)
	if err != nil || result == nil || len(result.Matches) == 0 {
		return retrieval.FindLinesLocation{}, contentRepairNoMatch
	}
	return bestFindLinesMatch(result.Matches, hint)
}

func bestFindLinesMatch(matches []retrieval.FindLinesMatch, hint model.LineRange) (retrieval.FindLinesLocation, contentRepairStatus) {
	if len(matches) == 0 {
		return retrieval.FindLinesLocation{}, contentRepairNoMatch
	}
	if !hasAnyLineAnchor(hint) && len(matches) > 1 {
		return retrieval.FindLinesLocation{}, contentRepairInconclusive
	}
	best := matches[0].CodeLocation
	bestScore := lineRangeScore(best.LineRange.Start, best.LineRange.End, hint)
	tied := false
	for _, match := range matches[1:] {
		loc := match.CodeLocation
		score := lineRangeScore(loc.LineRange.Start, loc.LineRange.End, hint)
		if score < bestScore {
			best = loc
			bestScore = score
			tied = false
			continue
		}
		if score == bestScore {
			tied = true
		}
	}
	if tied {
		return retrieval.FindLinesLocation{}, contentRepairInconclusive
	}
	return best, contentRepairRepaired
}

func lineRangeScore(start, end int, hint model.LineRange) int {
	if hint.Start <= 0 && hint.End <= 0 {
		return 0
	}
	score := 0
	if hint.Start > 0 {
		score += absInt(start - hint.Start)
	}
	if hint.End > 0 {
		score += absInt(end - hint.End)
	}
	return score
}

func (r *codeLocationRepairer) repairFromFirstLastLines(ctx context.Context, field string, loc *model.CodeLocation, lines []string) contentRepairStatus {
	first, firstIdx, ok := firstNonBlankLine(lines)
	if !ok {
		return contentRepairNoMatch
	}
	last, lastIdx, ok := lastNonBlankLine(lines)
	if !ok {
		return contentRepairNoMatch
	}
	if firstIdx == lastIdx {
		return contentRepairNoMatch
	}
	firstMatches, err := r.findLines(ctx, loc.FilePath, first)
	if err != nil || firstMatches == nil || len(firstMatches.Matches) == 0 {
		return contentRepairNoMatch
	}
	lastMatches, err := r.findLines(ctx, loc.FilePath, last)
	if err != nil || lastMatches == nil || len(lastMatches.Matches) == 0 {
		return contentRepairNoMatch
	}
	start, end, ok := bestEndpointPair(firstMatches.Matches, firstIdx, lastMatches.Matches, lastIdx, len(lines), loc.LineRange)
	if !ok {
		return contentRepairInconclusive
	}
	if r.applyFileSlice(ctx, field, loc, start, end, "content_endpoints") {
		return contentRepairRepaired
	}
	return contentRepairNoMatch
}

func bestEndpointPair(firstMatches []retrieval.FindLinesMatch, firstIdx int, lastMatches []retrieval.FindLinesMatch, lastIdx int, lineCount int, hint model.LineRange) (int, int, bool) {
	bestStart, bestEnd, bestScore := 0, 0, 0
	found := false
	tied := false
	for _, first := range firstMatches {
		firstLoc := first.CodeLocation
		for _, last := range lastMatches {
			lastLoc := last.CodeLocation
			if firstLoc.FilePath != lastLoc.FilePath {
				continue
			}
			start := max(firstLoc.LineRange.Start-(firstIdx-1), 1)
			end := lastLoc.LineRange.Start + (lineCount - lastIdx)
			if end < start {
				continue
			}
			score := lineRangeScore(start, end, hint)
			if !found || score < bestScore {
				bestStart, bestEnd, bestScore = start, end, score
				found = true
				tied = false
				continue
			}
			if score == bestScore {
				tied = true
			}
		}
	}
	return bestStart, bestEnd, found && !tied
}

func (r *codeLocationRepairer) repairFromAnyContentLine(ctx context.Context, field string, loc *model.CodeLocation, lines []string) contentRepairStatus {
	lineMatches := make([]contentLineMatches, 0, len(lines))
	nonBlank := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		nonBlank++
		result, err := r.findLines(ctx, loc.FilePath, line)
		if err != nil || result == nil || len(result.Matches) == 0 {
			continue
		}
		lineMatches = append(lineMatches, contentLineMatches{Index: i, Matches: result.Matches})
	}
	if len(lineMatches) == 0 || (nonBlank > 1 && len(lineMatches) < 2) {
		if len(lineMatches) == 0 {
			return contentRepairNoMatch
		}
		return contentRepairInconclusive
	}
	start, end, ok := bestCorroboratedLineOffset(lineMatches, len(lines), loc.LineRange)
	if !ok {
		return contentRepairInconclusive
	}
	if r.applyFileSlice(ctx, field, loc, start, end, "content_line_offset") {
		return contentRepairRepaired
	}
	return contentRepairNoMatch
}

type contentLineMatches struct {
	Index   int
	Matches []retrieval.FindLinesMatch
}

func bestCorroboratedLineOffset(lineMatches []contentLineMatches, lineCount int, hint model.LineRange) (int, int, bool) {
	candidateStarts := make(map[int]struct{})
	for _, line := range lineMatches {
		for _, match := range line.Matches {
			start := match.CodeLocation.LineRange.Start - line.Index
			if start > 0 {
				candidateStarts[start] = struct{}{}
			}
		}
	}
	bestStart, bestEnd, bestScore := 0, 0, 0
	found := false
	tied := false
	for start := range candidateStarts {
		if !lineOffsetSupported(lineMatches, start) {
			continue
		}
		end := max(start+lineCount-1, start)
		score := lineRangeScore(start, end, hint)
		if !found || score < bestScore {
			bestStart, bestEnd, bestScore = start, end, score
			found = true
			tied = false
			continue
		}
		if score == bestScore {
			tied = true
		}
	}
	return bestStart, bestEnd, found && !tied
}

func lineOffsetSupported(lineMatches []contentLineMatches, start int) bool {
	for _, line := range lineMatches {
		want := start + line.Index
		if !lineHasMatchAt(line.Matches, want) {
			return false
		}
	}
	return true
}

func lineHasMatchAt(matches []retrieval.FindLinesMatch, line int) bool {
	for _, match := range matches {
		if match.CodeLocation.LineRange.Start == line {
			return true
		}
	}
	return false
}

func (r *codeLocationRepairer) repairFromRange(ctx context.Context, field string, loc *model.CodeLocation) bool {
	start := loc.LineRange.Start
	end := loc.LineRange.End
	if start <= 0 && end <= 0 {
		return false
	}
	start = max(start, 1)
	end = max(end, start)
	return r.applyFileSlice(ctx, field, loc, start, end, "line_range")
}

func (r *codeLocationRepairer) applyFindLinesLocation(ctx context.Context, field string, loc *model.CodeLocation, match retrieval.FindLinesLocation, method string) {
	loc.FilePath = match.FilePath
	loc.LineRange = model.LineRange{
		Start: match.LineRange.Start,
		End:   match.LineRange.End,
		Count: match.LineRange.Count,
	}
	loc.Language = match.Language
	loc.Content = normalizeCodeLocationContent(match.Content)
	r.logf(ctx, "Code location repair: field=%s method=%s path=%s start=%d end=%d", field, method, loc.FilePath, loc.LineRange.Start, loc.LineRange.End)
}

func (r *codeLocationRepairer) applyFileSlice(ctx context.Context, field string, loc *model.CodeLocation, start, end int, method string) bool {
	content, err := r.retrieval.GetFileSlice(ctx, r.repoRoot, loc.FilePath, start, end)
	if err != nil || content == nil {
		return false
	}
	loc.FilePath = content.Path
	loc.LineRange = model.LineRange{
		Start: content.StartLine,
		End:   content.EndLine,
		Count: content.EndLine - content.StartLine + 1,
	}
	loc.Language = content.Language
	loc.Content = normalizeCodeLocationContent(content.Content)
	r.logf(ctx, "Code location repair: field=%s method=%s path=%s start=%d end=%d", field, method, loc.FilePath, loc.LineRange.Start, loc.LineRange.End)
	return true
}

func (r *codeLocationRepairer) findLines(ctx context.Context, path, code string) (*retrieval.FindLinesResult, error) {
	code = retrieval.NormalizeFindLinesCode(code)
	query := codeLocationRepairQuery{path: path, code: code}
	if result, ok := r.findCache[query]; ok {
		return result, nil
	}
	result, err := r.retrieval.FindLines(ctx, r.repoRoot, path, code)
	if err != nil {
		return nil, err
	}
	r.findCache[query] = result
	return result, nil
}

func (r *codeLocationRepairer) addRetry(result *codeLocationRepairResult, field string) {
	if result == nil {
		return
	}
	if r.retrySeen == nil {
		r.retrySeen = make(map[string]struct{})
	}
	if _, ok := r.retrySeen[field]; ok {
		return
	}
	r.retrySeen[field] = struct{}{}
	result.RetryFields = append(result.RetryFields, field)
}

func (r *codeLocationRepairer) logf(ctx context.Context, format string, args ...any) {
	if r != nil && r.engine != nil {
		r.engine.logf(ctx, format, args...)
	}
}

func splitNormalizedCodeLines(content string) []string {
	content = retrieval.NormalizeFindLinesCode(content)
	return retrieval.SplitFindLines(content)
}

func normalizeCodeLocationContent(content string) string {
	return retrieval.NormalizeLineEndings(content)
}

func firstNonBlankLine(lines []string) (string, int, bool) {
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			return line, i + 1, true
		}
	}
	return "", 0, false
}

func lastNonBlankLine(lines []string) (string, int, bool) {
	for i, line := range slices.Backward(lines) {
		if strings.TrimSpace(line) != "" {
			return line, i + 1, true
		}
	}
	return "", 0, false
}

func hasAnyLineAnchor(r model.LineRange) bool {
	return r.Start > 0 || r.End > 0
}

func changedCodeLocation(before, after model.CodeLocation) int {
	if before == after {
		return 0
	}
	return 1
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
