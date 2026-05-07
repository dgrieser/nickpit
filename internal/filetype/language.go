package filetype

import (
	"github.com/dgrieser/nickpit/mappings"
)

// DetectLanguage returns a syntax-oriented language label for a repo path.
func DetectLanguage(path string) string {
	return mappings.DetectLanguage(path)
}
