package mappings

import (
	"embed"
	"fmt"
	"path/filepath"
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
	Helm      HelmMappings        `yaml:"helm"`
}

type HelmMappings struct {
	Language         string   `yaml:"language"`
	BasenameMatches  []string `yaml:"basename_matches"`
	BasenameSuffixes []string `yaml:"basename_suffixes"`
	PathSegments     []string `yaml:"path_segments"`
}

type FileMappings struct {
	GeneratedSuffixes []string `yaml:"generated_suffixes"`
}

type ContextMappings struct {
	StyleGuides                  map[string]string   `yaml:"style_guides"`
	StyleGuideOrder              []string            `yaml:"style_guide_order"`
	StyleGuideExtensionOverrides map[string][]string `yaml:"style_guide_extension_overrides"`
}

var (
	loadOnce sync.Once
	loaded   loadedMappings
	loadErr  error
)

type loadedMappings struct {
	languages LanguageMappings
	files     FileMappings
	context   ContextMappings
	extLang   map[string]string
	baseLang  map[string]string
	ctxExt    map[string]string
}

func DetectLanguage(path string) string {
	m := mustLoadMappings()
	normalized := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(normalized)

	if helmLanguage(normalized, base, m.languages.Helm) {
		return m.languages.Helm.Language
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
	name, ok := m.context.StyleGuides[language]
	return name, ok
}

func StyleGuideOrder() []string {
	m := mustLoadMappings()
	return append([]string(nil), m.context.StyleGuideOrder...)
}

func StyleGuideLanguageForPath(path string, fallback func(string) string) string {
	m := mustLoadMappings()
	ext := strings.ToLower(filepath.Ext(path))
	if language, ok := m.ctxExt[ext]; ok {
		return language
	}
	return fallback(path)
}

func Context() ContextMappings {
	m := mustLoadMappings()
	return ContextMappings{
		StyleGuides:                  cloneStringMap(m.context.StyleGuides),
		StyleGuideOrder:              append([]string(nil), m.context.StyleGuideOrder...),
		StyleGuideExtensionOverrides: cloneStringSliceMap(m.context.StyleGuideExtensionOverrides),
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
	context, err := parseContextFile("context.yaml")
	if err != nil {
		return loadedMappings{}, err
	}
	return buildLoadedMappings(languages, files, context)
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

func parseContextFile(name string) (ContextMappings, error) {
	data, err := FS.ReadFile(name)
	if err != nil {
		return ContextMappings{}, fmt.Errorf("mappings: reading %s: %w", name, err)
	}
	return parseContextYAML(data)
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
	if mappings.Helm.Language == "" {
		return LanguageMappings{}, fmt.Errorf("mappings: languages.yaml missing helm language")
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

func parseContextYAML(data []byte) (ContextMappings, error) {
	var mappings ContextMappings
	if err := yaml.Unmarshal(data, &mappings); err != nil {
		return ContextMappings{}, fmt.Errorf("mappings: parsing context.yaml: %w", err)
	}
	if len(mappings.StyleGuides) == 0 {
		return ContextMappings{}, fmt.Errorf("mappings: context.yaml missing style guides")
	}
	if len(mappings.StyleGuideOrder) == 0 {
		return ContextMappings{}, fmt.Errorf("mappings: context.yaml missing style guide order")
	}
	return mappings, nil
}

func buildLoadedMappings(languages LanguageMappings, files FileMappings, context ContextMappings) (loadedMappings, error) {
	extLang, err := flattenStringSlices("languages.yaml extensions", languages.Extension)
	if err != nil {
		return loadedMappings{}, err
	}
	baseLang, err := flattenStringSlices("languages.yaml basenames", languages.Basename)
	if err != nil {
		return loadedMappings{}, err
	}
	ctxExt, err := flattenStringSlices("context.yaml style guide extension overrides", context.StyleGuideExtensionOverrides)
	if err != nil {
		return loadedMappings{}, err
	}
	for _, language := range context.StyleGuideOrder {
		if _, ok := context.StyleGuides[language]; !ok {
			return loadedMappings{}, fmt.Errorf("mappings: context.yaml order references unknown style guide language %q", language)
		}
	}
	return loadedMappings{
		languages: languages,
		files:     files,
		context:   context,
		extLang:   extLang,
		baseLang:  baseLang,
		ctxExt:    ctxExt,
	}, nil
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

func helmLanguage(path, base string, helm HelmMappings) bool {
	for _, suffix := range helm.BasenameSuffixes {
		if strings.HasSuffix(base, strings.ToLower(suffix)) {
			return true
		}
	}
	for _, match := range helm.BasenameMatches {
		if base == strings.ToLower(match) {
			return true
		}
	}
	for _, segment := range helm.PathSegments {
		if hasPathSegment(path, strings.ToLower(segment)) {
			return true
		}
	}
	return false
}

func hasPathSegment(path, segment string) bool {
	if path == segment {
		return true
	}
	return strings.HasPrefix(path, segment+"/") ||
		strings.Contains(path, "/"+segment+"/") ||
		strings.HasSuffix(path, "/"+segment)
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
