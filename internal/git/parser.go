package git

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dgrieser/nickpit/internal/filetype"
	"github.com/dgrieser/nickpit/internal/model"
)

var hunkHeader = regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@`)

func ParseUnifiedDiff(diff string) ([]model.DiffHunk, []model.ChangedFile, error) {
	_, hunks, files, err := ParseUnifiedDiffFormats(diff)
	return hunks, files, err
}

func ParseUnifiedDiffFormats(diff string) ([]model.DiffFile, []model.DiffHunk, []model.ChangedFile, error) {
	return ParseUnifiedDiffFormatsWithModes(diff, nil)
}

// ParseUnifiedDiffFormatsWithModes parses a patch and, for every file section
// whose header carries no git file mode, takes the mode from modes instead
// (typically "git diff --raw" output for the same revisions). Sections that do
// carry a mode header keep it, so the two entries of a symlink/regular-file
// replacement — which share one path — stay individually correct.
func ParseUnifiedDiffFormatsWithModes(diff string, modes FileModes) ([]model.DiffFile, []model.DiffHunk, []model.ChangedFile, error) {
	var (
		hunks        []model.DiffHunk
		files        []model.ChangedFile
		currentFile  string
		currentHunk  *model.DiffHunk
		currentEntry *model.ChangedFile
		// sectionSawMode records whether the current section's header carried a
		// mode, so the modes fallback only fills sections git left silent.
		sectionSawMode bool
	)
	// applyModeFallback resolves the current section's mode from modes. It runs
	// once the section's header block is complete — at its first hunk or at its
	// end — so a rename's final path is already known.
	applyModeFallback := func() {
		if sectionSawMode || currentEntry == nil {
			return
		}
		sectionSawMode = true
		if modes.Symlink(currentEntry.Path) {
			currentEntry.Symlink = true
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(diff))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
			applyModeFallback()
			if currentEntry != nil {
				files = append(files, *currentEntry)
			}
			currentFile = parseDiffGitPath(line)
			currentEntry = &model.ChangedFile{Path: currentFile, Status: model.FileModified}
			sectionSawMode = false
		case strings.HasPrefix(line, "diff --cc "), strings.HasPrefix(line, "diff --combined "):
			// Combined-diff ("merge state") file boundary. Without it the
			// section's header lines (index, ---/+++ file headers) would bleed
			// into the previous file's open hunk as content. The entry is
			// tracked like the `diff --git` case; its `@@@` hunks are skipped
			// below, so it survives without hunk or line-count data.
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
			applyModeFallback()
			if currentEntry != nil {
				files = append(files, *currentEntry)
			}
			currentFile = parseDiffCCPath(line)
			currentEntry = &model.ChangedFile{Path: currentFile, Status: model.FileModified}
			sectionSawMode = false
		case strings.HasPrefix(line, "new file mode "):
			if currentEntry != nil {
				currentEntry.Status = model.FileAdded
				currentEntry.Symlink = currentEntry.Symlink || diffHeaderMarksSymlink(line)
				sectionSawMode = true
			}
		case strings.HasPrefix(line, "deleted file mode "):
			if currentEntry != nil {
				currentEntry.Status = model.FileDeleted
				currentEntry.Symlink = currentEntry.Symlink || diffHeaderMarksSymlink(line)
				sectionSawMode = true
			}
		case strings.HasPrefix(line, "new mode "), strings.HasPrefix(line, "mode "), strings.HasPrefix(line, "index "):
			// A mode-only change into a symlink ("new mode 120000", or a combined
			// diff's "mode <parents>..<new>") and a changed symlink target
			// ("index <old>..<new> 120000") carry the mode here instead of on a
			// new/deleted-file line. Both appear in the header
			// block before the first hunk, so they cannot collide with body
			// lines (those always carry a ' ', '+', '-', or '\' prefix).
			if currentEntry != nil {
				// An "index" line carries a mode only when the mode is unchanged
				// on both sides; when git omits it there is nothing to learn and
				// the fallback still applies.
				if fileModeSpecFromDiffHeader(line) != "" {
					sectionSawMode = true
				}
				if diffHeaderMarksSymlink(line) {
					currentEntry.Symlink = true
				}
			}
		case strings.HasPrefix(line, "rename to "):
			if currentEntry != nil {
				currentEntry.Status = model.FileRenamed
				// Git C-quotes special/non-ASCII paths here just like on the
				// "diff --git" line; decode so the correct path from that line
				// is not overwritten with an escaped string.
				currentEntry.Path = unquoteGitPath(strings.TrimPrefix(line, "rename to "))
				currentFile = currentEntry.Path
			}
		case strings.HasPrefix(line, "@@@"):
			// Combined-diff ("merge state") hunk header, e.g.
			// "@@@ -1,4 -1,4 +1,4 @@@". The two-way parsing below cannot
			// represent it; previously the header and the whole hunk body were
			// swallowed as content of the preceding hunk. Skip this file's
			// combined hunks instead of misparsing them — the ChangedFile
			// entry survives without hunk or line-count data.
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
		case strings.HasPrefix(line, "@@ "):
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}
			applyModeFallback()
			parsed, err := parseHunkHeader(currentFile, line)
			if err != nil {
				return nil, nil, nil, err
			}
			// The file's mode header lines all precede its first hunk, so the
			// entry's mark is final here. Copying it per hunk keeps a hunk
			// self-describing without a path lookup, which could not tell apart
			// the two same-path entries of a symlink/regular-file replacement.
			if currentEntry != nil {
				parsed.Symlink = currentEntry.Symlink
			}
			currentHunk = parsed
		default:
			if currentHunk != nil {
				currentHunk.Content += line + "\n"
				if currentEntry != nil {
					// Every +/- line inside a hunk body is real content: file
					// headers (`+++ b/...`/`--- a/...`) only appear between the
					// file boundary and the first `@@`, where currentHunk is
					// nil. Excluding `+++`/`---` prefixes here would miscount
					// added lines starting with `++` or deleted lines starting
					// with `--`.
					switch {
					case strings.HasPrefix(line, "+"):
						currentEntry.Additions++
					case strings.HasPrefix(line, "-"):
						currentEntry.Deletions++
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, nil, err
	}
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}
	applyModeFallback()
	if currentEntry != nil {
		files = append(files, *currentEntry)
	}
	diffFiles := diffFilesFromUnifiedDiff(diff, modes)
	// Hunk languages come from path-only detection; refine them with the
	// content-aware per-file classification.
	langByPath := make(map[string]string, len(diffFiles))
	for _, file := range diffFiles {
		langByPath[file.FilePath] = file.Language
	}
	for i := range hunks {
		if language, ok := langByPath[hunks[i].FilePath]; ok {
			hunks[i].Language = language
		}
	}
	return diffFiles, hunks, files, nil
}

func DiffFilesFromUnifiedDiff(diff string) []model.DiffFile {
	return diffFilesFromUnifiedDiff(diff, nil)
}

func diffFilesFromUnifiedDiff(diff string, modes FileModes) []model.DiffFile {
	sections := splitUnifiedDiff(diff)
	files := make([]model.DiffFile, 0, len(sections))
	for _, section := range sections {
		if section.path == "" {
			continue
		}
		classification := filetype.Classify(section.path, section.text)
		symlink, sawMode := diffSectionMarksSymlink(section.text)
		if !sawMode {
			symlink = modes.Symlink(section.path)
		}
		files = append(files, model.DiffFile{
			FilePath:  section.path,
			Language:  classification.Language,
			Content:   section.text,
			Generated: classification.Generated,
			Symlink:   symlink,
		})
	}
	return files
}

type diffSection struct {
	path string
	text string
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
	remaining := diff
	for len(remaining) > 0 {
		idx := strings.IndexByte(remaining, '\n')
		var line string
		if idx == -1 {
			line = remaining
			remaining = ""
		} else {
			line = remaining[:idx+1]
			remaining = remaining[idx+1:]
		}
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			inSection = true
			currentPath = parseDiffGitPath(line)
		case strings.HasPrefix(line, "diff --cc "), strings.HasPrefix(line, "diff --combined "):
			// Combined-diff ("merge state") sections carry no "diff --git"
			// header. Without this boundary a combined patch produced no
			// DiffFile at all, so the raw per-file text of a merge diff was
			// dropped from prompt payloads and tool results alike.
			flush()
			inSection = true
			currentPath = parseDiffCCPath(line)
		}
		if inSection {
			current.WriteString(line)
		}
	}
	flush()
	return sections
}

// SymlinkFileMode is the git blob mode of a symlink: its content is the link
// target path, never file text.
const SymlinkFileMode = "120000"

// SymlinkFromModes reports whether the reviewed side of a change is a symlink,
// given the pre- and post-change git file modes as an SCM API reports them
// ("100644", "120000", and "0" or "" for a side that does not exist). The
// post-change side decides; for a deletion the pre-change side does. This is
// the same rule diffHeaderMarksSymlink applies to raw git diff headers, so both
// diff sources classify a symlink identically.
func SymlinkFromModes(oldMode, newMode string) bool {
	if mode := NormalizeFileMode(newMode); mode != "" {
		return mode == SymlinkFileMode
	}
	return NormalizeFileMode(oldMode) == SymlinkFileMode
}

// NormalizeFileMode trims a reported file mode and maps every "absent side"
// spelling to the empty string: "" and "0" as SCM APIs report it, and the
// all-zero mode ("000000") git prints in raw diff entries.
func NormalizeFileMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" || strings.Trim(mode, "0") == "" {
		return ""
	}
	return mode
}

// diffHeaderMarksSymlink reports whether one per-file diff header line says the
// reviewed side of the file is a symlink. Git spells the mode four ways:
//
//	new file mode 120000            symlink added
//	deleted file mode 120000        symlink removed
//	new mode 120000                 mode-only change into a symlink
//	index 1de5659..27fa349 120000   symlink target changed
//
// "old mode 120000" is deliberately not matched: with a regular "new mode" the
// post-change side is real text and must be reviewed as such.
// Pass header lines only; hunk body lines are not header lines but always carry
// a prefix character, so they cannot match these forms.
func diffHeaderMarksSymlink(line string) bool {
	return modeSpecMarksSymlink(fileModeSpecFromDiffHeader(line))
}

// fileModeSpecFromDiffHeader extracts the mode specification a header line states,
// or "" when it states none. A two-way diff states one mode; a combined ("--cc")
// diff lists one mode per parent, comma-separated, and states a mode change as a
// range whose post-change side follows "..":
//
//	deleted file mode 120000,100644
//	mode 000000,100644..100644
//
// "old mode" is deliberately ignored: it always comes with a "new mode" that
// describes the post-change side.
func fileModeSpecFromDiffHeader(line string) string {
	for _, prefix := range []string{"new file mode ", "deleted file mode ", "new mode ", "mode "} {
		if spec, ok := strings.CutPrefix(line, prefix); ok {
			return strings.TrimSpace(spec)
		}
	}
	if strings.HasPrefix(line, "index ") {
		// Two-way: "index <old>..<new> <mode>". A combined index line
		// ("index <a>,<b>..<c>") carries no mode; the separate "mode" line does.
		fields := strings.Fields(line)
		if len(fields) == 3 {
			return fields[2]
		}
	}
	return ""
}

// modeSpecMarksSymlink reports whether a mode specification describes a symlink on
// the side under review. A range states the post-change mode after "..", so that
// side decides. A comma-separated list has no post-change side — it enumerates the
// parents of a combined add or delete — so any parent that held a symlink makes the
// content the section shows a link target rather than file text.
func modeSpecMarksSymlink(spec string) bool {
	if spec == "" {
		return false
	}
	if pre, post, isRange := strings.Cut(spec, ".."); isRange {
		if NormalizeFileMode(post) != "" {
			return NormalizeFileMode(post) == SymlinkFileMode
		}
		// The post-change side is gone; the parents describe what was there.
		spec = pre
	}
	for mode := range strings.SplitSeq(spec, ",") {
		if NormalizeFileMode(mode) == SymlinkFileMode {
			return true
		}
	}
	return false
}

// diffSectionMarksSymlink reports whether a per-file diff section's header block
// marks the file as a symlink, and whether it stated a mode at all — a section
// that states none (a plain rename, for instance) is the case a caller-supplied
// mode map has to answer. Only lines before the first hunk header are inspected
// so body content can never be mistaken for a mode line.
func diffSectionMarksSymlink(section string) (symlink, sawMode bool) {
	for line := range strings.SplitSeq(section, "\n") {
		if strings.HasPrefix(line, "@@") {
			return symlink, sawMode
		}
		if spec := fileModeSpecFromDiffHeader(line); spec != "" {
			sawMode = true
			if modeSpecMarksSymlink(spec) {
				symlink = true
			}
		}
	}
	return symlink, sawMode
}

func parseDiffGitPath(line string) string {
	const prefix = "diff --git "
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	rest := line[len(prefix):]
	// Git C-quotes a path containing special characters (quotes, backslashes,
	// control chars, and — with the default core.quotePath — non-ASCII bytes
	// as octal escapes): `diff --git "a/x y" "b/x y"`. Either side may be
	// quoted independently.
	if path, ok := parseQuotedDiffGitPath(rest); ok {
		return path
	}
	if idx := strings.LastIndex(rest, " b/"); idx >= 0 {
		return rest[idx+len(" b/"):]
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	value := fields[len(fields)-1]
	value = strings.TrimPrefix(value, "b/")
	value = strings.TrimPrefix(value, "a/")
	return value
}

// parseDiffCCPath extracts the path from a combined-diff header line
// ("diff --cc <path>" or "diff --combined <path>"; no a/-b/ prefixes, but git
// C-quotes paths containing special or non-ASCII characters just like on
// "diff --git" lines).
func parseDiffCCPath(line string) string {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"diff --cc ", "diff --combined "} {
		if strings.HasPrefix(line, prefix) {
			return unquoteGitPath(line[len(prefix):])
		}
	}
	return ""
}

// unquoteGitPath decodes value when git C-quoted it (leading '"'); other
// values are returned verbatim.
func unquoteGitPath(value string) string {
	if strings.HasPrefix(value, `"`) {
		if decoded, _, ok := unquoteCStyle(value); ok {
			return decoded
		}
	}
	return value
}

// parseQuotedDiffGitPath extracts the b-side path from the remainder of a
// "diff --git " line when at least one side is C-quoted. It returns ok=false
// when neither side is quoted (the caller then uses the unquoted heuristics).
func parseQuotedDiffGitPath(rest string) (string, bool) {
	// a-side quoted: `"a/..." <b-side>`.
	if strings.HasPrefix(rest, `"`) {
		aPath, consumed, ok := unquoteCStyle(rest)
		if !ok {
			return "", false
		}
		remainder := strings.TrimPrefix(rest[consumed:], " ")
		if strings.HasPrefix(remainder, `"`) {
			bPath, _, ok := unquoteCStyle(remainder)
			if !ok {
				return "", false
			}
			return strings.TrimPrefix(bPath, "b/"), true
		}
		if remainder != "" {
			return strings.TrimPrefix(remainder, "b/"), true
		}
		// Only one token; fall back to the a-side path.
		return strings.TrimPrefix(aPath, "a/"), true
	}
	// a-side unquoted, b-side quoted: `a/... "b/..."`. The quoted b token is
	// the one that ends exactly at the end of the line.
	for idx := strings.Index(rest, ` "`); idx >= 0; {
		candidate := rest[idx+1:]
		if bPath, consumed, ok := unquoteCStyle(candidate); ok && consumed == len(candidate) {
			return strings.TrimPrefix(bPath, "b/"), true
		}
		next := strings.Index(rest[idx+1:], ` "`)
		if next < 0 {
			break
		}
		idx += 1 + next
	}
	return "", false
}

// unquoteCStyle decodes a git C-style quoted string starting at s[0] == '"'.
// It handles doubled backslashes, escaped quotes, the usual control escapes,
// and 1-3 digit octal escapes (git's default encoding for non-ASCII bytes).
// It returns the decoded value and the number of bytes consumed including both
// quotes.
func unquoteCStyle(s string) (string, int, bool) {
	if len(s) < 2 || s[0] != '"' {
		return "", 0, false
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		c := s[i]
		if c == '"' {
			return b.String(), i + 1, true
		}
		if c != '\\' {
			b.WriteByte(c)
			i++
			continue
		}
		i++
		if i >= len(s) {
			return "", 0, false
		}
		switch e := s[i]; e {
		case '"', '\\':
			b.WriteByte(e)
			i++
		case 'a':
			b.WriteByte('\a')
			i++
		case 'b':
			b.WriteByte('\b')
			i++
		case 'f':
			b.WriteByte('\f')
			i++
		case 'n':
			b.WriteByte('\n')
			i++
		case 'r':
			b.WriteByte('\r')
			i++
		case 't':
			b.WriteByte('\t')
			i++
		case 'v':
			b.WriteByte('\v')
			i++
		default:
			if e < '0' || e > '7' {
				return "", 0, false
			}
			value := 0
			digits := 0
			for i < len(s) && digits < 3 && s[i] >= '0' && s[i] <= '7' {
				value = value*8 + int(s[i]-'0')
				i++
				digits++
			}
			b.WriteByte(byte(value))
		}
	}
	return "", 0, false
}

func parseHunkHeader(path, line string) (*model.DiffHunk, error) {
	matches := hunkHeader.FindStringSubmatch(line)
	if len(matches) != 5 {
		return nil, fmt.Errorf("git: invalid hunk header %q", line)
	}
	oldStart, _ := strconv.Atoi(matches[1])
	oldLines := toCount(matches[2])
	newStart, _ := strconv.Atoi(matches[3])
	newLines := toCount(matches[4])
	return &model.DiffHunk{
		FilePath: path,
		Language: filetype.DetectLanguage(path),
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
	}, nil
}

func toCount(value string) int {
	if value == "" {
		return 1
	}
	count, _ := strconv.Atoi(value)
	return count
}
