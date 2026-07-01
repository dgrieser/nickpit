package review

import (
	"context"
	"fmt"
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

func (r *codeLocationRepairer) repairResponse(ctx context.Context, resp *llm.ReviewResponse) codeLocationRepairResult {
	var result codeLocationRepairResult
	if r == nil || resp == nil {
		return result
	}
	for i := range resp.Findings {
		finding := &resp.Findings[i]
		prefix := fmt.Sprintf("findings[%d]", i)
		result.Repaired += r.repairLocation(ctx, prefix+".code_location", &finding.CodeLocation, &result)
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
		if r.repairLocation(ctx, field, &suggestions[i].CodeLocation, result) > 0 {
			suggestions[i].LineRange = suggestions[i].CodeLocation.LineRange
			repaired++
		}
	}
	return repaired
}

func (r *codeLocationRepairer) repairLocation(ctx context.Context, field string, loc *model.CodeLocation, result *codeLocationRepairResult) int {
	if loc == nil {
		r.addRetry(result, field)
		return 0
	}
	loc.FilePath = normalizeToolPath(strings.TrimSpace(loc.FilePath))
	loc.Content = normalizeCodeLocationContent(loc.Content)
	if loc.FilePath == "" {
		r.addRetry(result, field+".file_path")
		return 0
	}
	original := *loc
	if loc.Content != "" {
		if r.repairFromContent(ctx, field, loc) {
			return changedCodeLocation(original, *loc)
		}
	}
	if hasAnyLineAnchor(loc.LineRange) {
		if r.repairFromRange(ctx, field, loc) {
			return changedCodeLocation(original, *loc)
		}
	}
	r.addRetry(result, field+".content_or_line_range")
	return 0
}

func (r *codeLocationRepairer) repairFromContent(ctx context.Context, field string, loc *model.CodeLocation) bool {
	if match, ok := r.findBestContentMatch(ctx, loc.FilePath, loc.Content, loc.LineRange); ok {
		r.applyFindLinesLocation(ctx, field, loc, match, "content")
		return true
	}
	lines := splitNormalizedCodeLines(loc.Content)
	if len(lines) == 0 {
		return false
	}
	if r.repairFromFirstLastLines(ctx, field, loc, lines) {
		return true
	}
	return r.repairFromAnyContentLine(ctx, field, loc, lines)
}

func (r *codeLocationRepairer) findBestContentMatch(ctx context.Context, path, content string, hint model.LineRange) (retrieval.FindLinesLocation, bool) {
	result, err := r.findLines(ctx, path, content)
	if err != nil || result == nil || len(result.Matches) == 0 {
		return retrieval.FindLinesLocation{}, false
	}
	return bestFindLinesMatch(result.Matches, hint), true
}

func bestFindLinesMatch(matches []retrieval.FindLinesMatch, hint model.LineRange) retrieval.FindLinesLocation {
	best := matches[0].CodeLocation
	bestScore := lineRangeScore(best.LineRange.Start, best.LineRange.End, hint)
	for _, match := range matches[1:] {
		loc := match.CodeLocation
		score := lineRangeScore(loc.LineRange.Start, loc.LineRange.End, hint)
		if score < bestScore {
			best = loc
			bestScore = score
		}
	}
	return best
}

func lineRangeScore(start, end int, hint model.LineRange) int {
	if hint.Start <= 0 && hint.End <= 0 {
		return start
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

func (r *codeLocationRepairer) repairFromFirstLastLines(ctx context.Context, field string, loc *model.CodeLocation, lines []string) bool {
	first, firstIdx, ok := firstNonBlankLine(lines)
	if !ok {
		return false
	}
	last, lastIdx, ok := lastNonBlankLine(lines)
	if !ok {
		return false
	}
	firstMatches, err := r.findLines(ctx, loc.FilePath, first)
	if err != nil || firstMatches == nil || len(firstMatches.Matches) == 0 {
		return false
	}
	lastMatches, err := r.findLines(ctx, loc.FilePath, last)
	if err != nil || lastMatches == nil || len(lastMatches.Matches) == 0 {
		return false
	}
	start, end, ok := bestEndpointPair(firstMatches.Matches, firstIdx, lastMatches.Matches, lastIdx, len(lines), loc.LineRange)
	if !ok {
		return false
	}
	return r.applyFileSlice(ctx, field, loc, start, end, "content_endpoints")
}

func bestEndpointPair(firstMatches []retrieval.FindLinesMatch, firstIdx int, lastMatches []retrieval.FindLinesMatch, lastIdx int, lineCount int, hint model.LineRange) (int, int, bool) {
	bestStart, bestEnd, bestScore := 0, 0, 0
	found := false
	for _, first := range firstMatches {
		firstLoc := first.CodeLocation
		for _, last := range lastMatches {
			lastLoc := last.CodeLocation
			if firstLoc.FilePath != lastLoc.FilePath {
				continue
			}
			start := firstLoc.LineRange.Start - (firstIdx - 1)
			if start < 1 {
				start = 1
			}
			end := lastLoc.LineRange.Start + (lineCount - lastIdx)
			if end < start {
				continue
			}
			score := lineRangeScore(start, end, hint)
			if !found || score < bestScore {
				bestStart, bestEnd, bestScore = start, end, score
				found = true
			}
		}
	}
	return bestStart, bestEnd, found
}

func (r *codeLocationRepairer) repairFromAnyContentLine(ctx context.Context, field string, loc *model.CodeLocation, lines []string) bool {
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		match, ok := r.findBestContentMatch(ctx, loc.FilePath, line, lineHint(loc.LineRange, i))
		if !ok {
			continue
		}
		start := match.LineRange.Start - i
		if start < 1 {
			start = 1
		}
		end := match.LineRange.Start + (len(lines) - i - 1)
		if end < start {
			end = start
		}
		return r.applyFileSlice(ctx, field, loc, start, end, "content_line_offset")
	}
	return false
}

func lineHint(hint model.LineRange, zeroBasedContentIndex int) model.LineRange {
	if hint.Start <= 0 {
		return model.LineRange{}
	}
	line := hint.Start + zeroBasedContentIndex
	return model.LineRange{Start: line, End: line, Count: 1}
}

func (r *codeLocationRepairer) repairFromRange(ctx context.Context, field string, loc *model.CodeLocation) bool {
	start := loc.LineRange.Start
	end := loc.LineRange.End
	if start <= 0 && end <= 0 {
		return false
	}
	if start <= 0 {
		start = 1
	}
	if end > 0 && end < start {
		end = start
	}
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
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func normalizeCodeLocationContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.ReplaceAll(content, "\r", "\n")
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
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i], i + 1, true
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
