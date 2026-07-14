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
	if loaded.styleGuides.StyleGuides["kubernetes"].Default != "styleguides/kubernetes.md" {
		t.Fatalf("kubernetes style guide = %q", loaded.styleGuides.StyleGuides["kubernetes"].Default)
	}
	if loaded.styleGuides.StyleGuides["go"].Versions["1.19"] != "styleguides/go-1.19.md" {
		t.Fatalf("go 1.19 style guide = %q", loaded.styleGuides.StyleGuides["go"].Versions["1.19"])
	}
	if loaded.styleGuides.StyleGuides["python"].Versions["2.7"] != "styleguides/python-2.7.md" {
		t.Fatalf("python 2.7 style guide = %q", loaded.styleGuides.StyleGuides["python"].Versions["2.7"])
	}
	if len(loaded.styleGuideDetectors) == 0 {
		t.Fatal("style guide detectors empty")
	}
	if len(loaded.files.GeneratedSuffixes) == 0 {
		t.Fatal("generated suffixes empty")
	}
	if loaded.extLang[".proto"] != "protobuf" {
		t.Fatalf(".proto language = %q", loaded.extLang[".proto"])
	}
	if len(loaded.languageContentRules) == 0 {
		t.Fatal("language content rules empty")
	}
	if len(loaded.generatedRules) == 0 {
		t.Fatal("generated rules empty")
	}
	if len(loaded.evictionPriorities) == 0 {
		t.Fatal("eviction priorities empty")
	}
	for language, entry := range loaded.styleGuides.StyleGuides {
		files := []string{entry.Default}
		for _, versionFile := range entry.Versions {
			files = append(files, versionFile)
		}
		for _, name := range files {
			if _, err := prompts.Load(name); err != nil {
				t.Fatalf("style guide for %s points to unreadable file %q: %v", language, name, err)
			}
		}
	}
}

func TestEmbeddedVersionSourcePriorityLoad(t *testing.T) {
	loaded, err := load()
	if err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"go", "python", "javascript", "typescript"} {
		if len(loaded.styleGuides.VersionSourcePriority[language]) == 0 {
			t.Fatalf("version_source_priority missing for %s", language)
		}
	}
	if first := loaded.styleGuides.VersionSourcePriority["go"][0]; first != "go.mod" {
		t.Fatalf("go top-priority source = %q, want go.mod", first)
	}
	if first := loaded.styleGuides.VersionSourcePriority["typescript"][0]; first != "package-lock.json" {
		t.Fatalf("typescript top-priority source = %q, want package-lock.json", first)
	}
}

func TestVersionSourceRank(t *testing.T) {
	tests := []struct {
		language string
		source   string
		want     int
	}{
		{"go", "go.mod", 0},
		{"go", ".tool-versions", 1},
		{"go", ".github/workflows/ci.yml", 2},
		{"go", ".gitlab-ci.yml", 3},
		{"go", "Dockerfile", 4},
		{"go", "dockerfile", 4},          // case-insensitive
		{"go", "services/api/go.mod", 0}, // slash-less pattern matches base name
		{"go", "sub/dir/Dockerfile", 4},  // ... anywhere in the tree
		{"go", "some-other-file", 5},     // unlisted ranks after every tier
		{"go", "", 5},
		{"python", "pyproject.toml", 0},
		{"python", "Dockerfile", 9},
		{"typescript", "package-lock.json", 0},
		{"typescript", "package.json", 3},
		{"shell", "Dockerfile", 0}, // no configured priority: everything ties at 0
		{"shell", "go.mod", 0},
	}
	for _, tt := range tests {
		if got := VersionSourceRank(tt.language, tt.source); got != tt.want {
			t.Errorf("VersionSourceRank(%q, %q) = %d, want %d", tt.language, tt.source, got, tt.want)
		}
	}
}

func TestVersionSourcePriorityValidation(t *testing.T) {
	base := "style_guides:\n  go: styleguides/go.md\nstyle_guide_order: [go]\n"
	unknownLanguage := base + "version_source_priority:\n  rust: [Cargo.toml]\n"
	if _, err := parseStyleGuidesYAML([]byte(unknownLanguage)); err == nil || !strings.Contains(err.Error(), "unknown style guide language") {
		t.Fatalf("unknown language validation err = %v", err)
	}
	emptyList := base + "version_source_priority:\n  go: []\n"
	if _, err := parseStyleGuidesYAML([]byte(emptyList)); err == nil || !strings.Contains(err.Error(), "no source patterns") {
		t.Fatalf("empty list validation err = %v", err)
	}
	emptyPattern := base + "version_source_priority:\n  go: [\"\"]\n"
	if _, err := parseStyleGuidesYAML([]byte(emptyPattern)); err == nil || !strings.Contains(err.Error(), "empty pattern") {
		t.Fatalf("empty pattern validation err = %v", err)
	}
	badGlob := base + "version_source_priority:\n  go: [\"[\"]\n"
	if _, err := parseStyleGuidesYAML([]byte(badGlob)); err == nil || !strings.Contains(err.Error(), "syntax error in pattern") {
		t.Fatalf("bad glob validation err = %v", err)
	}
}

func TestStyleGuideFileVersionSelection(t *testing.T) {
	tests := []struct {
		language string
		detected []string
		want     string
	}{
		{"python", nil, "styleguides/python.md"},
		{"python", []string{"3.11"}, "styleguides/python.md"},
		{"python", []string{"3.7"}, "styleguides/python.md"},
		{"python", []string{"2.7"}, "styleguides/python-2.7.md"},
		{"python", []string{"2.7.18"}, "styleguides/python-2.7.md"},
		{"python", []string{"2.7-slim"}, "styleguides/python-2.7.md"},
		{"python", []string{"3.0"}, "styleguides/python-3.6.md"},
		{"python", []string{"3.4.10"}, "styleguides/python-3.6.md"},
		{"python", []string{"3.6"}, "styleguides/python-3.6.md"},
		{"python", []string{"3.6.15"}, "styleguides/python-3.6.md"},
		{"python", []string{"3.6-alpine"}, "styleguides/python-3.6.md"},
		{"python", []string{"3.6", "2.7"}, "styleguides/python-2.7.md"},
		{"python", []string{"3.8"}, "styleguides/python-3.8.md"},
		{"python", []string{"3.8.18"}, "styleguides/python-3.8.md"},
		{"python", []string{"3.8-alpine"}, "styleguides/python-3.8.md"},
		{"python", []string{"3.9"}, "styleguides/python.md"},
		{"go", []string{"1.19.3"}, "styleguides/go-1.19.md"},
	}
	for _, tt := range tests {
		got, ok := StyleGuideFile(tt.language, tt.detected)
		if !ok || got != tt.want {
			t.Errorf("StyleGuideFile(%q, %v) = (%q, %v), want %q", tt.language, tt.detected, got, ok, tt.want)
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

func TestMappingValidationRejectsRulesWithoutMatchers(t *testing.T) {
	pathRule := "default: text\nextensions:\n  go: [.go]\npath_rules:\n  - language: helm\n"
	if _, err := parseLanguagesYAML([]byte(pathRule)); err == nil || !strings.Contains(err.Error(), "path_rules[0] missing match rules") {
		t.Fatalf("path rule validation err = %v", err)
	}
	contentRule := "default: text\nextensions:\n  go: [.go]\ncontent_rules:\n  - language: python\n"
	if _, err := parseLanguagesYAML([]byte(contentRule)); err == nil || !strings.Contains(err.Error(), "content_rules[0] missing match rules") {
		t.Fatalf("content rule validation err = %v", err)
	}
	generatedRule := "generated_suffixes: [go.sum]\ngenerated_rules:\n  - reason: empty\n"
	if _, err := parseFilesYAML([]byte(generatedRule)); err == nil || !strings.Contains(err.Error(), "generated_rules[0] missing match rules") {
		t.Fatalf("generated rule validation err = %v", err)
	}
	evictionRule := "generated_suffixes: [go.sum]\neviction_priorities:\n  - name: docs\n"
	if _, err := parseFilesYAML([]byte(evictionRule)); err == nil || !strings.Contains(err.Error(), "eviction_priorities[0] missing match rules") {
		t.Fatalf("eviction priority validation err = %v", err)
	}
	unnamedEviction := "generated_suffixes: [go.sum]\neviction_priorities:\n  - match_any:\n      extensions: [.md]\n"
	if _, err := parseFilesYAML([]byte(unnamedEviction)); err == nil || !strings.Contains(err.Error(), "eviction_priorities[0] missing name") {
		t.Fatalf("eviction priority name validation err = %v", err)
	}
}

func TestIsGenerated(t *testing.T) {
	generated := []struct {
		path    string
		content string
	}{
		{"Cargo.lock", ""},
		{"sub/Gemfile.lock", ""},
		{"web/pnpm-lock.yaml", ""},
		{"api/zz_generated.deepcopy.go", ""},
		{"assets/app.min.js", ""},
		{"src/__snapshots__/Button.test.js.snap", ""},
		{"GO.SUM", ""},
		{"Web/Package-Lock.JSON", ""},
		{"api/service.pb.go", ""},
		{"api/service.go", "+// Code generated by protoc-gen-go. DO NOT EDIT.\n+package api\n"},
		{"web/bundle.js", " * @generated\n"},
		// Marker on an unchanged context line still marks the file.
		{"db/queries.sql.go", " // Code generated by sqlc. DO NOT EDIT.\n+func New() {}\n"},
	}
	for _, tc := range generated {
		if !IsGenerated(tc.path, tc.content) {
			t.Fatalf("IsGenerated(%q) = false, want true", tc.path)
		}
	}
	notGenerated := []struct {
		path    string
		content string
	}{
		{"main.go", "+package main\n"},
		{"docs/guide.md", "+Never edit files that say DO NOT EDIT in their header.\n"},
		{"internal/locker/lock.go", ""},
		{"web/minify.js", ""},
		// A patch that removes the marker means the post-change file is no
		// longer generated.
		{"api/service.go", "-// Code generated by protoc-gen-go. DO NOT EDIT.\n+// handwritten now\n"},
		{"web/bundle.js", "- * @generated\n+ * handwritten\n"},
	}
	for _, tc := range notGenerated {
		if IsGenerated(tc.path, tc.content) {
			t.Fatalf("IsGenerated(%q) = true, want false", tc.path)
		}
	}
}

func TestDetectLanguageContentRules(t *testing.T) {
	tests := []struct {
		path    string
		content string
		want    string
	}{
		{"bin/deploy", "#!/usr/bin/env python3\nimport sys\n", "python"},
		{"bin/run", "+#!/bin/bash\n+set -euo pipefail\n", "shell"},
		{"scripts/cli", "#!/usr/bin/env node\n", "nodejs"},
		{"run.py", "#!/bin/bash\n", "python"},
		{"bin/deploy", "", "text"},
		{"bin/deploy", "echo no shebang\n", "text"},
	}
	for _, tc := range tests {
		if got := DetectLanguageContent(tc.path, tc.content); got != tc.want {
			t.Fatalf("DetectLanguageContent(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestEvictionClass(t *testing.T) {
	docs := EvictionClass("docs/guide.md")
	tests := EvictionClass("pkg/service_test.go")
	source := EvictionClass("pkg/service.go")
	if docs >= tests || tests >= source {
		t.Fatalf("eviction classes docs=%d tests=%d source=%d, want docs < tests < source", docs, tests, source)
	}
	if EvictionClass("README.md") != docs {
		t.Fatalf("README.md class = %d, want %d", EvictionClass("README.md"), docs)
	}
	if EvictionClass("web/src/App.spec.ts") != tests {
		t.Fatalf("spec class = %d, want %d", EvictionClass("web/src/App.spec.ts"), tests)
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
		StyleGuides:     map[string]StyleGuideEntry{"go": {Default: "styleguides/go.md"}},
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
		StyleGuides:     map[string]StyleGuideEntry{"go": {Default: "styleguides/go.md"}},
		StyleGuideOrder: []string{"go", "python"},
	}

	_, err := buildLoadedMappings(languages, files, styleGuides)
	if err == nil || !strings.Contains(err.Error(), "unknown style guide language") {
		t.Fatalf("unknown order validation err = %v", err)
	}
}

func TestMappingValidationRejectsInvalidDetectorRegex(t *testing.T) {
	styleGuides := StyleGuideMappings{
		StyleGuides:     map[string]StyleGuideEntry{"go": {Default: "styleguides/go.md"}},
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

func TestContextReturnsIsolatedDetectorSlices(t *testing.T) {
	first := Context()
	if len(first.Detectors) == 0 || len(first.Detectors[0].MatchAny.Extensions) == 0 {
		t.Fatalf("detectors missing match_any extensions: %#v", first.Detectors)
	}
	original := first.Detectors[0].MatchAny.Extensions[0]
	first.Detectors[0].MatchAny.Extensions[0] = ".mutated"

	second := Context()
	if got := second.Detectors[0].MatchAny.Extensions[0]; got != original {
		t.Fatalf("detector extension mutated shared config: got %q want %q", got, original)
	}
}
