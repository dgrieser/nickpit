package mappings

import (
	"embed"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// FS stores editable mapping data shipped inside the binary.
//
//go:embed *.yaml
var FS embed.FS

type LanguageMappings struct {
	Default   string              `yaml:"default"`
	Extension map[string][]string `yaml:"extensions"`
	Basename  map[string][]string `yaml:"basenames"`
	PathRules []LanguagePathRule  `yaml:"path_rules"`
}

type LanguagePathRule struct {
	Language string     `yaml:"language"`
	MatchAny PatternSet `yaml:"match_any"`
}

type FileMappings struct {
	GeneratedSuffixes []string `yaml:"generated_suffixes"`
}

type StyleGuideMappings struct {
	StyleGuides                  map[string]string    `yaml:"style_guides"`
	StyleGuideOrder              []string             `yaml:"style_guide_order"`
	StyleGuideExtensionOverrides map[string][]string  `yaml:"extension_overrides"`
	Detectors                    []StyleGuideDetector `yaml:"detectors"`
}

type StyleGuideDetector struct {
	Language   string     `yaml:"language"`
	ProbePaths PatternSet `yaml:"probe_paths"`
	MatchAny   PatternSet `yaml:"match_any"`
	MatchAll   PatternSet `yaml:"match_all"`
}

type PatternSet struct {
	Extensions       []string `yaml:"extensions"`
	Basenames        []string `yaml:"basenames"`
	BasenameSuffixes []string `yaml:"basename_suffixes"`
	PathSegments     []string `yaml:"path_segments"`
	PathPrefixes     []string `yaml:"path_prefixes"`
	PathContains     []string `yaml:"path_contains"`
	ContentContains  []string `yaml:"content_contains"`
	ContentRegex     []string `yaml:"content_regex"`
}

var (
	loadOnce sync.Once
	loaded   loadedMappings
	loadErr  error
)

type loadedMappings struct {
	languages           LanguageMappings
	files               FileMappings
	styleGuides         StyleGuideMappings
	extLang             map[string]string
	baseLang            map[string]string
	ctxExt              map[string]string
	languagePathRules   []compiledLanguagePathRule
	styleGuideDetectors []compiledStyleGuideDetector
}

type compiledLanguagePathRule struct {
	language string
	matchAny compiledPatternSet
}

type compiledStyleGuideDetector struct {
	language   string
	probePaths compiledPatternSet
	matchAny   compiledPatternSet
	matchAll   compiledPatternSet
}

type compiledPatternSet struct {
	extensions       []string
	basenames        []string
	basenameSuffixes []string
	pathSegments     []string
	pathPrefixes     []string
	pathContains     []string
	contentContains  []string
	contentRegex     []*regexp.Regexp
}

func DetectLanguage(path string) string {
	m := mustLoadMappings()
	normalized := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(normalized)
	for _, rule := range m.languagePathRules {
		if rule.matchAny.matchesPath(normalized, base) {
			return rule.language
		}
	}
	if language, ok := m.extLang[filepath.Ext(base)]; ok {
		return language
	}
	if language, ok := m.baseLang[base]; ok {
		return language
	}
	return m.languages.Default
}

func IsGeneratedFile(path string) bool {
	m := mustLoadMappings()
	for _, suffix := range m.files.GeneratedSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func StyleGuideFile(language string) (string, bool) {
	m := mustLoadMappings()
	name, ok := m.styleGuides.StyleGuides[language]
	return name, ok
}

func StyleGuideOrder() []string {
	m := mustLoadMappings()
	return append([]string(nil), m.styleGuides.StyleGuideOrder...)
}

func StyleGuideLanguageForPath(path string, fallback func(string) string) string {
	m := mustLoadMappings()
	ext := strings.ToLower(filepath.Ext(path))
	if language, ok := m.ctxExt[ext]; ok {
		return language
	}
	return fallback(path)
}

func StyleGuideDetectorProbePath(path string) bool {
	m := mustLoadMappings()
	normalized := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(normalized)
	for _, detector := range m.styleGuideDetectors {
		if detector.probePaths.matchesPath(normalized, base) {
			return true
		}
	}
	return false
}

func StyleGuideDetectorLanguages(path, content string) []string {
	m := mustLoadMappings()
	normalized := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(normalized)
	content = normalizeSignalContent(content)
	var out []string
	seen := make(map[string]struct{})
	for _, detector := range m.styleGuideDetectors {
		if !detector.matchAny.empty() && !detector.matchAny.matches(normalized, base, content) {
			continue
		}
		if !detector.matchAll.empty() && !detector.matchAll.matchesAll(normalized, base, content) {
			continue
		}
		if detector.matchAny.empty() && detector.matchAll.empty() {
			continue
		}
		if _, ok := seen[detector.language]; ok {
			continue
		}
		seen[detector.language] = struct{}{}
		out = append(out, detector.language)
	}
	return out
}

func Context() StyleGuideMappings {
	m := mustLoadMappings()
	return StyleGuideMappings{
		StyleGuides:                  cloneStringMap(m.styleGuides.StyleGuides),
		StyleGuideOrder:              append([]string(nil), m.styleGuides.StyleGuideOrder...),
		StyleGuideExtensionOverrides: cloneStringSliceMap(m.styleGuides.StyleGuideExtensionOverrides),
		Detectors:                    cloneStyleGuideDetectors(m.styleGuides.Detectors),
	}
}

func mustLoadMappings() loadedMappings {
	loadOnce.Do(func() {
		loaded, loadErr = load()
	})
	if loadErr != nil {
		panic(loadErr)
	}
	return loaded
}

func load() (loadedMappings, error) {
	languages, err := parseLanguagesFile("languages.yaml")
	if err != nil {
		return loadedMappings{}, err
	}
	files, err := parseFilesFile("files.yaml")
	if err != nil {
		return loadedMappings{}, err
	}
	styleGuides, err := parseStyleGuidesFile("styleguides.yaml")
	if err != nil {
		return loadedMappings{}, err
	}
	return buildLoadedMappings(languages, files, styleGuides)
}

func parseLanguagesFile(name string) (LanguageMappings, error) {
	data, err := FS.ReadFile(name)
	if err != nil {
		return LanguageMappings{}, fmt.Errorf("mappings: reading %s: %w", name, err)
	}
	return parseLanguagesYAML(data)
}

func parseFilesFile(name string) (FileMappings, error) {
	data, err := FS.ReadFile(name)
	if err != nil {
		return FileMappings{}, fmt.Errorf("mappings: reading %s: %w", name, err)
	}
	return parseFilesYAML(data)
}

func parseStyleGuidesFile(name string) (StyleGuideMappings, error) {
	data, err := FS.ReadFile(name)
	if err != nil {
		return StyleGuideMappings{}, fmt.Errorf("mappings: reading %s: %w", name, err)
	}
	return parseStyleGuidesYAML(data)
}

func parseLanguagesYAML(data []byte) (LanguageMappings, error) {
	var mappings LanguageMappings
	if err := yaml.Unmarshal(data, &mappings); err != nil {
		return LanguageMappings{}, fmt.Errorf("mappings: parsing languages.yaml: %w", err)
	}
	if mappings.Default == "" {
		return LanguageMappings{}, fmt.Errorf("mappings: languages.yaml missing default")
	}
	if len(mappings.Extension) == 0 {
		return LanguageMappings{}, fmt.Errorf("mappings: languages.yaml missing extensions")
	}
	for i, rule := range mappings.PathRules {
		if rule.Language == "" {
			return LanguageMappings{}, fmt.Errorf("mappings: languages.yaml path_rules[%d] missing language", i)
		}
		if rule.MatchAny.empty() {
			return LanguageMappings{}, fmt.Errorf("mappings: languages.yaml path_rules[%d] missing match_any", i)
		}
	}
	return mappings, nil
}

func parseFilesYAML(data []byte) (FileMappings, error) {
	var mappings FileMappings
	if err := yaml.Unmarshal(data, &mappings); err != nil {
		return FileMappings{}, fmt.Errorf("mappings: parsing files.yaml: %w", err)
	}
	if len(mappings.GeneratedSuffixes) == 0 {
		return FileMappings{}, fmt.Errorf("mappings: files.yaml missing generated suffixes")
	}
	return mappings, nil
}

func parseStyleGuidesYAML(data []byte) (StyleGuideMappings, error) {
	var mappings StyleGuideMappings
	if err := yaml.Unmarshal(data, &mappings); err != nil {
		return StyleGuideMappings{}, fmt.Errorf("mappings: parsing styleguides.yaml: %w", err)
	}
	if len(mappings.StyleGuides) == 0 {
		return StyleGuideMappings{}, fmt.Errorf("mappings: styleguides.yaml missing style guides")
	}
	if len(mappings.StyleGuideOrder) == 0 {
		return StyleGuideMappings{}, fmt.Errorf("mappings: styleguides.yaml missing style guide order")
	}
	for i, detector := range mappings.Detectors {
		if detector.Language == "" {
			return StyleGuideMappings{}, fmt.Errorf("mappings: styleguides.yaml detectors[%d] missing language", i)
		}
		if detector.MatchAny.empty() && detector.MatchAll.empty() {
			return StyleGuideMappings{}, fmt.Errorf("mappings: styleguides.yaml detectors[%d] missing match rules", i)
		}
	}
	return mappings, nil
}

func buildLoadedMappings(languages LanguageMappings, files FileMappings, styleGuides StyleGuideMappings) (loadedMappings, error) {
	extLang, err := flattenStringSlices("languages.yaml extensions", languages.Extension)
	if err != nil {
		return loadedMappings{}, err
	}
	baseLang, err := flattenStringSlices("languages.yaml basenames", languages.Basename)
	if err != nil {
		return loadedMappings{}, err
	}
	ctxExt, err := flattenStringSlices("styleguides.yaml extension overrides", styleGuides.StyleGuideExtensionOverrides)
	if err != nil {
		return loadedMappings{}, err
	}
	for _, language := range styleGuides.StyleGuideOrder {
		if _, ok := styleGuides.StyleGuides[language]; !ok {
			return loadedMappings{}, fmt.Errorf("mappings: styleguides.yaml order references unknown style guide language %q", language)
		}
	}
	for i, detector := range styleGuides.Detectors {
		if _, ok := styleGuides.StyleGuides[detector.Language]; !ok {
			return loadedMappings{}, fmt.Errorf("mappings: styleguides.yaml detector[%d] references unknown style guide language %q", i, detector.Language)
		}
	}
	languageRules, err := compileLanguagePathRules(languages.PathRules)
	if err != nil {
		return loadedMappings{}, err
	}
	styleDetectors, err := compileStyleGuideDetectors(styleGuides.Detectors)
	if err != nil {
		return loadedMappings{}, err
	}
	return loadedMappings{
		languages:           languages,
		files:               files,
		styleGuides:         styleGuides,
		extLang:             extLang,
		baseLang:            baseLang,
		ctxExt:              ctxExt,
		languagePathRules:   languageRules,
		styleGuideDetectors: styleDetectors,
	}, nil
}

func compileLanguagePathRules(rules []LanguagePathRule) ([]compiledLanguagePathRule, error) {
	out := make([]compiledLanguagePathRule, 0, len(rules))
	for i, rule := range rules {
		matchAny, err := compilePatternSet(fmt.Sprintf("languages.yaml path_rules[%d].match_any", i), rule.MatchAny)
		if err != nil {
			return nil, err
		}
		out = append(out, compiledLanguagePathRule{
			language: rule.Language,
			matchAny: matchAny,
		})
	}
	return out, nil
}

func compileStyleGuideDetectors(detectors []StyleGuideDetector) ([]compiledStyleGuideDetector, error) {
	out := make([]compiledStyleGuideDetector, 0, len(detectors))
	for i, detector := range detectors {
		probePaths, err := compilePatternSet(fmt.Sprintf("styleguides.yaml detectors[%d].probe_paths", i), detector.ProbePaths)
		if err != nil {
			return nil, err
		}
		matchAny, err := compilePatternSet(fmt.Sprintf("styleguides.yaml detectors[%d].match_any", i), detector.MatchAny)
		if err != nil {
			return nil, err
		}
		matchAll, err := compilePatternSet(fmt.Sprintf("styleguides.yaml detectors[%d].match_all", i), detector.MatchAll)
		if err != nil {
			return nil, err
		}
		out = append(out, compiledStyleGuideDetector{
			language:   detector.Language,
			probePaths: probePaths,
			matchAny:   matchAny,
			matchAll:   matchAll,
		})
	}
	return out, nil
}

func compilePatternSet(label string, set PatternSet) (compiledPatternSet, error) {
	out := compiledPatternSet{
		extensions:       lowerStrings(set.Extensions),
		basenames:        lowerStrings(set.Basenames),
		basenameSuffixes: lowerStrings(set.BasenameSuffixes),
		pathSegments:     lowerStrings(set.PathSegments),
		pathPrefixes:     lowerSlashStrings(set.PathPrefixes),
		pathContains:     lowerSlashStrings(set.PathContains),
		contentContains:  append([]string(nil), set.ContentContains...),
	}
	for i, expr := range set.ContentRegex {
		compiled, err := regexp.Compile(expr)
		if err != nil {
			return compiledPatternSet{}, fmt.Errorf("mappings: %s.content_regex[%d]: %w", label, i, err)
		}
		out.contentRegex = append(out.contentRegex, compiled)
	}
	return out, nil
}

func flattenStringSlices(label string, values map[string][]string) (map[string]string, error) {
	out := make(map[string]string)
	for language, keys := range values {
		if language == "" {
			return nil, fmt.Errorf("mappings: %s has empty language", label)
		}
		for _, key := range keys {
			normalized := strings.ToLower(key)
			if normalized == "" {
				return nil, fmt.Errorf("mappings: %s has empty key for %q", label, language)
			}
			if existing, ok := out[normalized]; ok {
				return nil, fmt.Errorf("mappings: %s maps %q to both %q and %q", label, normalized, existing, language)
			}
			out[normalized] = language
		}
	}
	return out, nil
}

func (set PatternSet) empty() bool {
	return len(set.Extensions) == 0 &&
		len(set.Basenames) == 0 &&
		len(set.BasenameSuffixes) == 0 &&
		len(set.PathSegments) == 0 &&
		len(set.PathPrefixes) == 0 &&
		len(set.PathContains) == 0 &&
		len(set.ContentContains) == 0 &&
		len(set.ContentRegex) == 0
}

func (set compiledPatternSet) empty() bool {
	return len(set.extensions) == 0 &&
		len(set.basenames) == 0 &&
		len(set.basenameSuffixes) == 0 &&
		len(set.pathSegments) == 0 &&
		len(set.pathPrefixes) == 0 &&
		len(set.pathContains) == 0 &&
		len(set.contentContains) == 0 &&
		len(set.contentRegex) == 0
}

func (set compiledPatternSet) matches(normalizedPath, base, content string) bool {
	return set.matchesPath(normalizedPath, base) || set.matchesContent(content)
}

func (set compiledPatternSet) matchesAll(normalizedPath, base, content string) bool {
	if len(set.extensions) > 0 && !contains(set.extensions, filepath.Ext(base)) {
		return false
	}
	if len(set.basenames) > 0 && !contains(set.basenames, base) {
		return false
	}
	if len(set.basenameSuffixes) > 0 && !hasAnySuffix(base, set.basenameSuffixes) {
		return false
	}
	if len(set.pathSegments) > 0 && !hasAnyPathSegment(normalizedPath, set.pathSegments) {
		return false
	}
	if len(set.pathPrefixes) > 0 && !hasAnyPrefix(normalizedPath, set.pathPrefixes) {
		return false
	}
	if len(set.pathContains) > 0 && !hasAnyContains(normalizedPath, set.pathContains) {
		return false
	}
	if len(set.contentContains) > 0 && !hasAnyContains(content, set.contentContains) {
		return false
	}
	if len(set.contentRegex) > 0 && !matchesAnyRegex(content, set.contentRegex) {
		return false
	}
	return true
}

func (set compiledPatternSet) matchesPath(normalizedPath, base string) bool {
	return contains(set.extensions, filepath.Ext(base)) ||
		contains(set.basenames, base) ||
		hasAnySuffix(base, set.basenameSuffixes) ||
		hasAnyPathSegment(normalizedPath, set.pathSegments) ||
		hasAnyPrefix(normalizedPath, set.pathPrefixes) ||
		hasAnyContains(normalizedPath, set.pathContains)
}

func (set compiledPatternSet) matchesContent(content string) bool {
	return hasAnyContains(content, set.contentContains) || matchesAnyRegex(content, set.contentRegex)
}

func hasPathSegment(path, segment string) bool {
	if path == segment {
		return true
	}
	return strings.HasPrefix(path, segment+"/") ||
		strings.Contains(path, "/"+segment+"/") ||
		strings.HasSuffix(path, "/"+segment)
}

func normalizeSignalContent(content string) string {
	var out strings.Builder
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line[0] == '+' || line[0] == '-' || line[0] == ' ' {
			line = strings.TrimSpace(line[1:])
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}

func lowerStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, value := range in {
		value = strings.ToLower(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func lowerSlashStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, value := range in {
		value = strings.ToLower(filepath.ToSlash(value))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasAnySuffix(value string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func hasAnyContains(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func hasAnyPathSegment(path string, segments []string) bool {
	for _, segment := range segments {
		if hasPathSegment(path, segment) {
			return true
		}
	}
	return false
}

func matchesAnyRegex(content string, expressions []*regexp.Regexp) bool {
	for _, expr := range expressions {
		if expr.MatchString(content) {
			return true
		}
	}
	return false
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneStringSliceMap(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for key, value := range in {
		out[key] = append([]string(nil), value...)
	}
	return out
}

func cloneStyleGuideDetectors(in []StyleGuideDetector) []StyleGuideDetector {
	out := make([]StyleGuideDetector, len(in))
	for i, detector := range in {
		out[i] = StyleGuideDetector{
			Language:   detector.Language,
			ProbePaths: clonePatternSet(detector.ProbePaths),
			MatchAny:   clonePatternSet(detector.MatchAny),
			MatchAll:   clonePatternSet(detector.MatchAll),
		}
	}
	return out
}

func clonePatternSet(in PatternSet) PatternSet {
	return PatternSet{
		Extensions:       append([]string(nil), in.Extensions...),
		Basenames:        append([]string(nil), in.Basenames...),
		BasenameSuffixes: append([]string(nil), in.BasenameSuffixes...),
		PathSegments:     append([]string(nil), in.PathSegments...),
		PathPrefixes:     append([]string(nil), in.PathPrefixes...),
		PathContains:     append([]string(nil), in.PathContains...),
		ContentContains:  append([]string(nil), in.ContentContains...),
		ContentRegex:     append([]string(nil), in.ContentRegex...),
	}
}
