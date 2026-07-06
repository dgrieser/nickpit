package filetype

import (
	"github.com/dgrieser/nickpit/mappings"
)

// Classification is the unified label set for a repo path: the syntax
// language and whether the file is generated or lockfile-like noise.
type Classification struct {
	Language  string
	Generated bool
}

// Classify labels a repo path, optionally using file or unified-diff content
// for content-rule detection (shebangs, generated-code markers).
func Classify(path, content string) Classification {
	return Classification{
		Language:  mappings.DetectLanguageContent(path, content),
		Generated: mappings.IsGenerated(path, content),
	}
}

// DetectLanguage returns a syntax-oriented language label for a repo path.
func DetectLanguage(path string) string {
	return mappings.DetectLanguage(path)
}

// IsGenerated reports whether a repo path is generated or lockfile-like
// noise, optionally consulting file or unified-diff content markers.
func IsGenerated(path, content string) bool {
	return mappings.IsGenerated(path, content)
}

// EvictionClass ranks a path for context trimming: lower classes are evicted
// earlier; paths matching no configured class are evicted last.
func EvictionClass(path string) int {
	return mappings.EvictionClass(path)
}
