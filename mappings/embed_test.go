package mappings

import (
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/prompts"
)

func TestEmbeddedMappingsLoad(t *testing.T) {
	loaded, err := load()
	if err != nil {
		t.Fatal(err)
	}

	if loaded.extLang[".go"] != "go" {
		t.Fatalf(".go language = %q", loaded.extLang[".go"])
	}
	if loaded.ctxExt[".tsx"] != "typescript" {
		t.Fatalf(".tsx style guide language = %q", loaded.ctxExt[".tsx"])
	}
	if loaded.styleGuides.StyleGuides["kubernetes"] != "styleguides/kubernetes.md" {
		t.Fatalf("kubernetes style guide = %q", loaded.styleGuides.StyleGuides["kubernetes"])
	}
	if len(loaded.styleGuideDetectors) == 0 {
		t.Fatal("style guide detectors empty")
	}
	if len(loaded.files.GeneratedSuffixes) == 0 {
		t.Fatal("generated suffixes empty")
	}
	for language, name := range loaded.styleGuides.StyleGuides {
		if _, err := prompts.Load(name); err != nil {
			t.Fatalf("style guide for %s points to unreadable file %q: %v", language, name, err)
		}
	}
}

func TestMappingValidationRejectsMissingRequiredSections(t *testing.T) {
	if _, err := parseLanguagesYAML([]byte("extensions: {}\n")); err == nil || !strings.Contains(err.Error(), "missing default") {
		t.Fatalf("languages validation err = %v", err)
	}
	if _, err := parseFilesYAML([]byte("generated_suffixes: []\n")); err == nil || !strings.Contains(err.Error(), "missing generated suffixes") {
		t.Fatalf("files validation err = %v", err)
	}
	if _, err := parseStyleGuidesYAML([]byte("style_guides: {}\n")); err == nil || !strings.Contains(err.Error(), "missing style guides") {
		t.Fatalf("style guides validation err = %v", err)
	}
}

func TestMappingValidationRejectsDuplicateKeys(t *testing.T) {
	languages := LanguageMappings{
		Default: "text",
		Extension: map[string][]string{
			"go":     {".go"},
			"golang": {".go"},
		},
	}
	files := FileMappings{GeneratedSuffixes: []string{"go.sum"}}
	styleGuides := StyleGuideMappings{
		StyleGuides:     map[string]string{"go": "styleguides/go.md"},
		StyleGuideOrder: []string{"go"},
	}

	_, err := buildLoadedMappings(languages, files, styleGuides)
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("duplicate validation err = %v", err)
	}
}

func TestMappingValidationRejectsUnknownOrderedStyleGuide(t *testing.T) {
	languages := LanguageMappings{
		Default:   "text",
		Extension: map[string][]string{"go": {".go"}},
	}
	files := FileMappings{GeneratedSuffixes: []string{"go.sum"}}
	styleGuides := StyleGuideMappings{
		StyleGuides:     map[string]string{"go": "styleguides/go.md"},
		StyleGuideOrder: []string{"go", "python"},
	}

	_, err := buildLoadedMappings(languages, files, styleGuides)
	if err == nil || !strings.Contains(err.Error(), "unknown style guide language") {
		t.Fatalf("unknown order validation err = %v", err)
	}
}

func TestMappingValidationRejectsInvalidDetectorRegex(t *testing.T) {
	styleGuides := StyleGuideMappings{
		StyleGuides:     map[string]string{"go": "styleguides/go.md"},
		StyleGuideOrder: []string{"go"},
		Detectors: []StyleGuideDetector{
			{Language: "go", MatchAny: PatternSet{ContentRegex: []string{"("}}},
		},
	}
	_, err := buildLoadedMappings(
		LanguageMappings{Default: "text", Extension: map[string][]string{"go": {".go"}}},
		FileMappings{GeneratedSuffixes: []string{"go.sum"}},
		styleGuides,
	)
	if err == nil || !strings.Contains(err.Error(), "content_regex") {
		t.Fatalf("invalid detector regex err = %v", err)
	}
}

func TestConfigDrivenStyleGuideDetectorLanguages(t *testing.T) {
	languages := StyleGuideDetectorLanguages("pkg/example.go", "+const q = `SELECT id FROM users`\n")
	if len(languages) != 1 || languages[0] != "sql" {
		t.Fatalf("detector languages = %#v", languages)
	}
}
