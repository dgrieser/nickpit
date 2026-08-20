package review

import (
	"cmp"
	"slices"
	"strings"

	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/model"
)

// allowedDiffCodeLocations returns the authoritative old- and new-side hunk
// windows as complete code_location values, plus one location per metadata-only
// symlink change (see metadataOnlySymlinkLocations). The same values drive scope
// validation, deterministic repair, and reviewer retry guidance.
func allowedDiffCodeLocations(hunks []model.DiffHunk, changed []model.ChangedFile) []model.CodeLocation {
	locations := make([]model.CodeLocation, 0, len(hunks)*2)
	for _, hunk := range hunks {
		// The path stays the literal git path: normalizing would fold distinct
		// legal names together (`a\b` and `a/b` are two files on Unix) and could
		// then authorize a finding against the wrong file. Findings are matched
		// against it by allowedPathMatches, which tolerates model-added noise on
		// the finding's side only.
		path := hunk.FilePath
		if strings.TrimSpace(path) == "" {
			continue
		}
		language := hunk.Language
		if language == "" {
			language = filetype.DetectLanguage(path)
		}
		oldContent, newContent, oldCount, newCount := splitDiffHunkSides(hunk.Content)
		if hunk.OldLines > 0 {
			oldCount = hunk.OldLines
		}
		if hunk.NewLines > 0 {
			newCount = hunk.NewLines
		}
		if oldCount > 0 && hunk.OldStart > 0 {
			locations = append(locations, model.CodeLocation{
				FilePath: path,
				LineRange: model.LineRange{
					Start: hunk.OldStart,
					End:   hunk.OldStart + oldCount - 1,
					Count: oldCount,
				},
				Language: language,
				Content:  oldContent,
			})
		}
		if newCount > 0 && hunk.NewStart > 0 {
			locations = append(locations, model.CodeLocation{
				FilePath: path,
				LineRange: model.LineRange{
					Start: hunk.NewStart,
					End:   hunk.NewStart + newCount - 1,
					Count: newCount,
				},
				Language: language,
				Content:  newContent,
			})
		}
	}
	locations = append(locations, metadataOnlySymlinkLocations(hunks, changed)...)
	slices.SortFunc(locations, func(a, b model.CodeLocation) int {
		if n := cmp.Compare(a.FilePath, b.FilePath); n != 0 {
			return n
		}
		if n := cmp.Compare(a.LineRange.Start, b.LineRange.Start); n != 0 {
			return n
		}
		if n := cmp.Compare(a.LineRange.End, b.LineRange.End); n != 0 {
			return n
		}
		return cmp.Compare(a.Content, b.Content)
	})
	return slices.CompactFunc(locations, func(a, b model.CodeLocation) bool {
		return a.FilePath == b.FilePath &&
			a.LineRange.SameAnchor(b.LineRange) &&
			a.Content == b.Content
	})
}

func splitDiffHunkSides(content string) (oldContent, newContent string, oldCount, newCount int) {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return "", "", 0, 0
	}
	var oldLines, newLines []string
	for raw := range strings.SplitSeq(content, "\n") {
		marker := byte(' ')
		line := raw
		if raw != "" {
			marker = raw[0]
			if marker == ' ' || marker == '+' || marker == '-' || marker == '\\' {
				line = raw[1:]
			}
		}
		switch marker {
		case '+':
			newLines = append(newLines, line)
		case '-':
			oldLines = append(oldLines, line)
		case '\\':
			continue
		default:
			oldLines = append(oldLines, line)
			newLines = append(newLines, line)
		}
	}
	return strings.Join(oldLines, "\n"), strings.Join(newLines, "\n"), len(oldLines), len(newLines)
}

// codeLocationOverlapsAllowed reports whether any line in loc overlaps any of
// the allowed code locations (for diff scoping: the old-side or new-side line
// ranges of the diff hunks, see allowedDiffCodeLocations). Ranges are
// intentionally accepted as-is when only part overlaps or when they span gaps
// between hunks.
func codeLocationOverlapsAllowed(loc model.CodeLocation, allowed []model.CodeLocation) bool {
	start := loc.LineRange.Start
	if strings.TrimSpace(loc.FilePath) == "" || start <= 0 {
		return false
	}
	end := max(loc.LineRange.End, start)
	for _, candidate := range allowed {
		if !allowedPathMatches(loc.FilePath, candidate.FilePath) {
			continue
		}
		if rangesOverlap(start, end, candidate.LineRange.Start, candidate.LineRange.EffectiveCount()) {
			return true
		}
	}
	return false
}

// allowedPathMatches reports whether a model-supplied file path designates the same
// file as an allowed location's git path. The allowed side is a literal git path and
// is compared as such; only the finding's side is also tried normalized, because a
// model may prefix "./" or spell a separator loosely. Folding both sides would
// merge distinct legal names — a symlink `a\b` and a regular file `a/b` — and let
// one file's scope authorize a finding about the other.
func allowedPathMatches(findingPath, candidatePath string) bool {
	if findingPath == candidatePath {
		return true
	}
	return normalizeReviewPath(findingPath) == candidatePath
}

func rangesOverlap(start, end, hunkStart, hunkLines int) bool {
	if hunkStart <= 0 || hunkLines <= 0 {
		return false
	}
	hunkEnd := hunkStart + hunkLines - 1
	return start <= hunkEnd && end >= hunkStart
}

// metadataOnlySymlinkLocations grants a scopeable location to every symlink change
// whose patch has no hunk at all. Renaming a symlink emits only "rename from/to"
// lines, yet moving a relative symlink is precisely what can break its target — so
// without this the reviewer is told the entry is a symlink and then has every
// finding about it dropped for pointing outside the (empty) diff scope. The single
// line of a symlink blob is its target, so line 1 is the whole file.
func metadataOnlySymlinkLocations(hunks []model.DiffHunk, changed []model.ChangedFile) []model.CodeLocation {
	if len(changed) == 0 {
		return nil
	}
	// Literal git paths throughout: a hunk for `a/b` must not cancel the location
	// of a symlink named `a\b`, nor lend its scope to that other file.
	hasHunk := make(map[string]bool, len(hunks))
	for _, hunk := range hunks {
		hasHunk[hunk.FilePath] = true
	}
	var locations []model.CodeLocation
	seen := make(map[string]bool, len(changed))
	for _, file := range changed {
		path := file.Path
		if !file.Symlink || strings.TrimSpace(path) == "" || hasHunk[path] || seen[path] {
			continue
		}
		// Scope without evidence would invite a finding the prompt cannot ground:
		// with no hunk, the target and the old path are the entire change. When
		// neither reached the context, the change stays unreviewable rather than
		// reviewable on nothing.
		if file.SymlinkTarget == "" && file.OldPath == "" {
			continue
		}
		seen[path] = true
		locations = append(locations, model.CodeLocation{
			FilePath:  path,
			LineRange: model.LineRange{Start: 1, End: 1, Count: 1},
			Language:  filetype.DetectLanguage(path),
			Content:   file.SymlinkTarget,
		})
	}
	return locations
}

func filterFindingsByDiffScope(findings []model.Finding, hunks []model.DiffHunk, changed []model.ChangedFile) ([]model.Finding, []model.Finding) {
	if len(findings) == 0 {
		return findings, nil
	}
	allowed := allowedDiffCodeLocations(hunks, changed)
	kept := make([]model.Finding, 0, len(findings))
	dropped := make([]model.Finding, 0)
	for _, finding := range findings {
		if codeLocationOverlapsAllowed(finding.CodeLocation, allowed) {
			kept = append(kept, finding)
			continue
		}
		dropped = append(dropped, finding)
	}
	return kept, dropped
}
