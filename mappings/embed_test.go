package mappings

import (
	"slices"
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
		{"kubernetes", "go.mod", 0},
		{"kubernetes", "Makefile", 1},
		{"helm", ".tool-versions", 0},
		{"mysql", "docker-compose.yml", 3},
		{"rust", "crates/api/Cargo.toml", 0},
		{"mysql", "services/db/compose.yaml", 0},
		{"mysql", "Dockerfile", 6},
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
		{"python", []string{"3.16"}, "styleguides/python.md"},
		{"python", []string{"3.7"}, "styleguides/python-3.8.md"},
		{"python", []string{"3.7.17"}, "styleguides/python-3.8.md"},
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
		{"python", []string{"3.9"}, "styleguides/python-3.9.md"},
		{"python", []string{"3.9.25"}, "styleguides/python-3.9.md"},
		{"python", []string{"3.9-slim"}, "styleguides/python-3.9.md"},
		{"python", []string{"3.9", "3.8"}, "styleguides/python-3.8.md"},
		{"python", []string{"3.10"}, "styleguides/python-3.10.md"},
		{"python", []string{"3.10.20"}, "styleguides/python-3.10.md"},
		{"python", []string{"3.10-slim"}, "styleguides/python-3.10.md"},
		{"python", []string{"3.11"}, "styleguides/python-3.11.md"},
		{"python", []string{"3.11.14"}, "styleguides/python-3.11.md"},
		{"python", []string{"3.11-bookworm"}, "styleguides/python-3.11.md"},
		{"python", []string{"3.12"}, "styleguides/python-3.12.md"},
		{"python", []string{"3.12.13"}, "styleguides/python-3.12.md"},
		{"python", []string{"3.12-alpine"}, "styleguides/python-3.12.md"},
		{"python", []string{"3.13"}, "styleguides/python-3.13.md"},
		{"python", []string{"3.13.16"}, "styleguides/python-3.13.md"},
		{"python", []string{"3.13-slim"}, "styleguides/python-3.13.md"},
		{"python", []string{"3.14"}, "styleguides/python-3.14.md"},
		{"python", []string{"3.14.6"}, "styleguides/python-3.14.md"},
		{"python", []string{"3.14-alpine"}, "styleguides/python-3.14.md"},
		{"python", []string{"3.15"}, "styleguides/python-3.15.md"},
		{"python", []string{"3.15.0"}, "styleguides/python-3.15.md"},
		{"python", []string{"3.15-slim"}, "styleguides/python-3.15.md"},
		{"python", []string{"3.14", "3.12"}, "styleguides/python-3.12.md"},
		{"go", []string{"1.19.3"}, "styleguides/go-1.19.md"},
		{"go", []string{"1.26"}, "styleguides/go-1.26.md"},
		{"go", []string{"1.26.10"}, "styleguides/go-1.26.md"},
		{"go", []string{"1.27"}, "styleguides/go-1.27.md"},
		{"go", []string{"1.27.0"}, "styleguides/go-1.27.md"},
		{"go", []string{"1.27.3"}, "styleguides/go-1.27.md"},
		{"go", []string{"1.27-alpine"}, "styleguides/go-1.27.md"},
		{"go", []string{"1.27", "1.26"}, "styleguides/go-1.26.md"},
		{"go", []string{"1.28"}, "styleguides/go.md"},
		{"javascript", nil, "styleguides/javascript.md"},
		{"javascript", []string{"14.21.3"}, "styleguides/javascript-node-16.md"},
		{"javascript", []string{"16"}, "styleguides/javascript-node-16.md"},
		{"javascript", []string{"17.9.1"}, "styleguides/javascript-node-16.md"},
		{"javascript", []string{"v18.20.8"}, "styleguides/javascript-node-18.md"},
		{"javascript", []string{"19"}, "styleguides/javascript-node-18.md"},
		{"javascript", []string{"20-alpine"}, "styleguides/javascript-node-20.md"},
		{"javascript", []string{"21.7.3"}, "styleguides/javascript-node-20.md"},
		{"javascript", []string{"22"}, "styleguides/javascript-node-22.md"},
		{"javascript", []string{"23.11"}, "styleguides/javascript-node-22.md"},
		{"javascript", []string{"24.22.1"}, "styleguides/javascript-node-24.md"},
		{"javascript", []string{"25"}, "styleguides/javascript-node-24.md"},
		{"javascript", []string{"26.12.1"}, "styleguides/javascript-node-26.md"},
		{"javascript", []string{"26", "22"}, "styleguides/javascript-node-22.md"},
		{"javascript", []string{"27"}, "styleguides/javascript.md"},
		{"typescript", nil, "styleguides/typescript.md"},
		{"typescript", []string{"3.9.10"}, "styleguides/typescript.md"},
		{"typescript", []string{"4.2.4"}, "styleguides/typescript-4.9.md"},
		{"typescript", []string{"4.9.5"}, "styleguides/typescript-4.9.md"},
		{"typescript", []string{"5.0.4"}, "styleguides/typescript-5.0.md"},
		{"typescript", []string{"5.4.5"}, "styleguides/typescript-5.4.md"},
		{"typescript", []string{"5.9.3"}, "styleguides/typescript-5.9.md"},
		{"typescript", []string{"6.0.3"}, "styleguides/typescript-6.0.md"},
		{"typescript", []string{"7.0.2"}, "styleguides/typescript-7.0.md"},
		{"typescript", []string{"7.0.2", "5.8.3"}, "styleguides/typescript-5.8.md"},
		{"typescript", []string{"7.1.0"}, "styleguides/typescript.md"},
		{"csharp", nil, "styleguides/csharp.md"},
		{"csharp", []string{"7.3"}, "styleguides/csharp-9.md"},
		{"csharp", []string{"8.0"}, "styleguides/csharp-9.md"},
		{"csharp", []string{"9"}, "styleguides/csharp-9.md"},
		{"csharp", []string{"10.0"}, "styleguides/csharp-10.md"},
		{"csharp", []string{"11"}, "styleguides/csharp-11.md"},
		{"csharp", []string{"12.0"}, "styleguides/csharp-12.md"},
		{"csharp", []string{"13.0"}, "styleguides/csharp-13.md"},
		{"csharp", []string{"14.0"}, "styleguides/csharp-14.md"},
		{"csharp", []string{"15.0"}, "styleguides/csharp-15.md"},
		{"csharp", []string{"14.0", "12.0"}, "styleguides/csharp-12.md"},
		{"csharp", []string{"16.0"}, "styleguides/csharp.md"},
		{"helm", nil, "styleguides/helm.md"},
		{"helm", []string{"2.17.0"}, "styleguides/helm.md"},
		{"helm", []string{"3.15.2"}, "styleguides/helm-3.md"},
		{"helm", []string{"v3.22.1"}, "styleguides/helm-3.md"},
		{"helm", []string{"4.0.1"}, "styleguides/helm-4.md"},
		{"helm", []string{"4.3.1", "3.16.1"}, "styleguides/helm-3.md"},
		{"helm", []string{"5.0.0"}, "styleguides/helm.md"},
		{"kubernetes", nil, "styleguides/kubernetes.md"},
		{"kubernetes", []string{"1.28.9"}, "styleguides/kubernetes.md"},
		{"kubernetes", []string{"1.29"}, "styleguides/kubernetes-1.29.md"},
		{"kubernetes", []string{"1.31.4"}, "styleguides/kubernetes-1.31.md"},
		{"kubernetes", []string{"v1.33.0"}, "styleguides/kubernetes-1.33.md"},
		{"kubernetes", []string{"1.37.1"}, "styleguides/kubernetes-1.37.md"},
		{"kubernetes", []string{"1.36", "1.32"}, "styleguides/kubernetes-1.32.md"},
		{"kubernetes", []string{"1.38.0"}, "styleguides/kubernetes.md"},
		{"rust", nil, "styleguides/rust.md"},
		{"rust", []string{"2015"}, "styleguides/rust-2018.md"},
		{"rust", []string{"2018"}, "styleguides/rust-2018.md"},
		{"rust", []string{"2021"}, "styleguides/rust-2021.md"},
		{"rust", []string{"2024"}, "styleguides/rust-2024.md"},
		{"rust", []string{"2024", "2021"}, "styleguides/rust-2021.md"},
		{"rust", []string{"2027"}, "styleguides/rust.md"},
		{"postgresql", nil, "styleguides/postgresql.md"},
		{"postgresql", []string{"12"}, "styleguides/postgresql-14.md"},
		{"postgresql", []string{"14.24"}, "styleguides/postgresql-14.md"},
		{"postgresql", []string{"15.4.0"}, "styleguides/postgresql-15.md"},
		{"postgresql", []string{"16-alpine"}, "styleguides/postgresql-16.md"},
		{"postgresql", []string{"17-3.5"}, "styleguides/postgresql-17.md"},
		{"postgresql", []string{"18.6"}, "styleguides/postgresql-18.md"},
		{"postgresql", []string{"19rc1"}, "styleguides/postgresql-19.md"},
		{"postgresql", []string{"18", "16"}, "styleguides/postgresql-16.md"},
		{"postgresql", []string{"20"}, "styleguides/postgresql.md"},
		{"mysql", nil, "styleguides/mysql.md"},
		{"mysql", []string{"5.7"}, "styleguides/mysql.md"},
		{"mysql", []string{"8.0"}, "styleguides/mysql-8.0.md"},
		{"mysql", []string{"8.0.46"}, "styleguides/mysql-8.0.md"},
		{"mysql", []string{"8.3.0"}, "styleguides/mysql-8.4.md"},
		{"mysql", []string{"8.4-oracle"}, "styleguides/mysql-8.4.md"},
		{"mysql", []string{"9.6.0"}, "styleguides/mysql-8.4.md"},
		{"mysql", []string{"9.7.4"}, "styleguides/mysql-9.7.md"},
		{"mysql", []string{"26.10.0"}, "styleguides/mysql.md"},
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

func TestUnusedIdentifierDiagnostics(t *testing.T) {
	tests := []struct {
		language string
		want     []string
	}{
		// go rejects both; rustc warns on both by default.
		{"go", []string{"imports", "variables"}},
		{"rust", []string{"imports", "variables"}},
		// zig errors on unused locals only; swiftc/kotlinc/csc warn on unused
		// variables but ignore unused imports.
		{"zig", []string{"variables"}},
		{"swift", []string{"variables"}},
		{"kotlin", []string{"variables"}},
		{"csharp", []string{"variables"}},
		// nothing in the default toolchain reports these.
		{"python", nil},
		{"nodejs", nil},
		{"typescript", nil},
		{"text", nil},
		{"", nil},
	}
	for _, tc := range tests {
		if got := UnusedIdentifierDiagnostics(tc.language); !slices.Equal(got, tc.want) {
			t.Fatalf("UnusedIdentifierDiagnostics(%q) = %v, want %v", tc.language, got, tc.want)
		}
	}
}

func TestUnusedIdentifierDiagnosticsValidation(t *testing.T) {
	base := "default: text\nextensions:\n  go: [.go]\n"
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{"unknown language", "unused_identifier_diagnostics:\n  fortran: [imports]\n", `unknown language "fortran"`},
		{"empty kinds", "unused_identifier_diagnostics:\n  go: []\n", `"go" has no kinds`},
		{"unknown kind", "unused_identifier_diagnostics:\n  go: [functions]\n", `unknown kind "functions"`},
		{"duplicate kind", "unused_identifier_diagnostics:\n  go: [imports, imports]\n", `repeats kind "imports"`},
	}
	for _, tc := range tests {
		languages, err := parseLanguagesYAML([]byte(base + tc.yaml))
		if err != nil {
			t.Fatalf("%s: parse err = %v", tc.name, err)
		}
		if _, err := buildUnusedIdentifierDiagnostics(languages); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("%s: validation err = %v, want %q", tc.name, err, tc.wantErr)
		}
	}
	// Kinds normalize to canonical order regardless of YAML order.
	languages, err := parseLanguagesYAML([]byte(base + "unused_identifier_diagnostics:\n  go: [variables, imports]\n"))
	if err != nil {
		t.Fatal(err)
	}
	built, err := buildUnusedIdentifierDiagnostics(languages)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(built["go"], []string{"imports", "variables"}) {
		t.Fatalf("canonical order = %v", built["go"])
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

func TestMySQLDetectorAndCompanions(t *testing.T) {
	tests := []struct {
		path, content string
		want          bool
	}{
		{"internal/db/db.go", "import _ \"github.com/go-sql-driver/mysql\"\n", true},
		{"migrations/001_init.sql", "CREATE TABLE t (id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY) ENGINE=InnoDB;\n", true},
		{"repo/upsert.py", "cur.execute(\"INSERT INTO t (k, v) VALUES (%s, %s) ON DUPLICATE KEY UPDATE v = VALUES(v)\")\n", true},
		{"migrations/001_init.sql", "CREATE TABLE t (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY);\n", false},
	}
	for _, tt := range tests {
		got := slices.Contains(StyleGuideDetectorLanguages(tt.path, tt.content), "mysql")
		if got != tt.want {
			t.Errorf("mysql detector on %s %q = %v, want %v", tt.path, tt.content, got, tt.want)
		}
	}
	if got := ToolchainCompanions("sql"); !slices.Equal(got, []string{"postgresql", "mysql"}) {
		t.Fatalf("ToolchainCompanions(sql) = %v", got)
	}
	if got := ToolchainCompanions("go"); got != nil {
		t.Fatalf("ToolchainCompanions(go) = %v, want none", got)
	}
}

func TestToolchainCompanionsValidation(t *testing.T) {
	base := "style_guides:\n  sql: styleguides/sql.md\n  mysql: styleguides/mysql.md\nstyle_guide_order: [sql, mysql]\n"
	unknown := base + "toolchain_companions:\n  sql: [oracle]\n"
	if _, err := parseStyleGuidesYAML([]byte(unknown)); err == nil || !strings.Contains(err.Error(), "invalid companion") {
		t.Fatalf("unknown companion validation err = %v", err)
	}
	self := base + "toolchain_companions:\n  sql: [sql]\n"
	if _, err := parseStyleGuidesYAML([]byte(self)); err == nil || !strings.Contains(err.Error(), "invalid companion") {
		t.Fatalf("self companion validation err = %v", err)
	}
	empty := base + "toolchain_companions:\n  sql: []\n"
	if _, err := parseStyleGuidesYAML([]byte(empty)); err == nil || !strings.Contains(err.Error(), "no companions") {
		t.Fatalf("empty companion validation err = %v", err)
	}
}

func TestPostgreSQLDetector(t *testing.T) {
	tests := []struct {
		path, content string
		want          bool
	}{
		{"internal/store/store.go", "import \"github.com/jackc/pgx/v5/pgxpool\"\n", true},
		{"migrations/003_orders.sql", "CREATE TABLE orders (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY);\n", true},
		{"migrations/004_idx.sql", "CREATE INDEX CONCURRENTLY orders_created_idx ON orders (created_at);\n", true},
		{"repo/query.py", "cur.execute(\"SELECT payload::jsonb FROM events\")\n", true},
		{"migrations/001_init.sql", "CREATE TABLE t (id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY) ENGINE=InnoDB;\n", false},
	}
	for _, tt := range tests {
		got := slices.Contains(StyleGuideDetectorLanguages(tt.path, tt.content), "postgresql")
		if got != tt.want {
			t.Errorf("postgresql detector on %s %q = %v, want %v", tt.path, tt.content, got, tt.want)
		}
	}
}

func TestRustManifestDetector(t *testing.T) {
	for _, path := range []string{"Cargo.toml", "crates/core/Cargo.toml", "Cargo.lock", "rust-toolchain.toml"} {
		if !slices.Contains(StyleGuideDetectorLanguages(path, ""), "rust") {
			t.Errorf("%s should select the rust guide", path)
		}
	}
	if slices.Contains(StyleGuideDetectorLanguages("pyproject.toml", ""), "rust") {
		t.Errorf("pyproject.toml must not select the rust guide")
	}
}
