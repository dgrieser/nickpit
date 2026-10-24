package prompts

import (
	"io/fs"
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
		if _, err := fs.Stat(FS, guide); err != nil {
			t.Errorf("%s: no matching guide %s", name, guide)
		}
		data, err := FS.ReadFile(guide)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "\n[^") {
			t.Errorf("%s: footnote definitions belong in %s", guide, name)
		}
	}
}
