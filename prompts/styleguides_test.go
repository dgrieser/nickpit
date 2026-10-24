package prompts

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

func TestStyleGuideMarkdownStartsWithSectionHeading(t *testing.T) {
	names, err := fs.Glob(FS, "styleguides/*.md")
	if err != nil {
		t.Fatalf("glob styleguides: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no styleguides found")
	}

	for _, name := range names {
		data, err := FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		content := string(data)
		if !strings.HasPrefix(content, "### ") {
			t.Errorf("%s: styleguide must start with a level-3 markdown heading", name)
			continue
		}
		firstLine, rest, ok := strings.Cut(content, "\n")
		if !ok {
			t.Errorf("%s: heading must be followed by body content", name)
			continue
		}
		title := strings.TrimPrefix(firstLine, "### ")
		if title == "" || strings.TrimSpace(title) != title {
			t.Errorf("%s: heading title must be non-empty and trimmed", name)
		}
		if !strings.HasPrefix(rest, "\n") {
			t.Errorf("%s: heading must be followed by a blank line", name)
		}
	}
}

var (
	citationDef     = regexp.MustCompile(`^\[\^(\d+)\]:`)
	citationMarker  = regexp.MustCompile(`\[\^(\d+)\]`)
	inlineCodeSpan  = regexp.MustCompile("`+[^`]*`+")
	codeFenceOpener = regexp.MustCompile("^\\s*(`{3,}|~{3,})(.*)$")
)

// TestStyleGuideCitationsSitNextToTheirGuide keeps sources out of the prompt:
// each guide's citations live in <guide>.citations.md, every entry says where
// the guide relies on it, and the guide itself carries no footnote markers.
func TestStyleGuideCitationsSitNextToTheirGuide(t *testing.T) {
	names, err := fs.Glob(FS, "styleguides/*.citations.md")
	if err != nil {
		t.Fatalf("glob citations: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no citation files found")
	}
	for _, name := range names {
		guide := strings.TrimSuffix(name, ".citations.md") + ".md"
		data, err := FS.ReadFile(guide)
		if err != nil {
			t.Errorf("%s: no matching guide %s", name, guide)
			continue
		}
		citations, err := FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		defined := map[string]bool{}
		lines := strings.Split(string(citations), "\n")
		for i, line := range lines {
			m := citationDef.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			defined[m[1]] = true
			if i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], "    - ") {
				t.Errorf("%s: [^%s] lacks a back-reference into %s", name, m[1], guide)
			}
		}
		for n, line := range proseLines(string(data)) {
			if citationDef.MatchString(line) {
				t.Errorf("%s:%d: footnote definition belongs in %s", guide, n, name)
			}
			for _, m := range citationMarker.FindAllStringSubmatch(inlineCodeSpan.ReplaceAllString(line, ""), -1) {
				if defined[m[1]] {
					t.Errorf("%s:%d: footnote marker [^%s] belongs in %s", guide, n, m[1], name)
				}
			}
		}
	}
}

// proseLines returns the guide's lines outside fenced code blocks, keyed by
// 1-based line number.
func proseLines(content string) map[int]string {
	out := map[int]string{}
	fence := ""
	for i, line := range strings.Split(content, "\n") {
		if m := codeFenceOpener.FindStringSubmatch(line); m != nil {
			if fence == "" {
				fence = m[1]
				continue
			}
			if m[1][0] == fence[0] && len(m[1]) >= len(fence) && strings.TrimSpace(m[2]) == "" {
				fence = ""
				continue
			}
		}
		if fence == "" {
			out[i+1] = line
		}
	}
	return out
}
