package workflow

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/dgrieser/nickpit/workflows"
	"gopkg.in/yaml.v3"
)

// ImportSourceDefaults is the provenance an import-findings group starts from,
// per source, as workflows/import_sources.yaml defines it.
type ImportSourceDefaults struct {
	Note            string `yaml:"note"`
	ExemptDiffScope bool   `yaml:"exempt_diff_scope"`
	PreferInMerge   bool   `yaml:"prefer_in_merge"`
	SkipDedupe      bool   `yaml:"skip_dedupe"`
}

// importSources lists the sources the engine knows how to read; the embedded
// file must define exactly these.
var importSources = []string{ImportSourceFile, ImportSourcePublishedReview}

var (
	importSourcesOnce   sync.Once
	importSourceEntries map[string]ImportSourceDefaults
)

// ImportSourceDefaultsFor returns the defaults of an import source, and
// whether the source exists.
func ImportSourceDefaultsFor(source string) (ImportSourceDefaults, bool) {
	importSourcesOnce.Do(func() {
		entries, err := parseImportSources(workflows.ImportSources())
		if err != nil {
			panic(fmt.Sprintf("workflow: invalid embedded import sources: %v", err))
		}
		importSourceEntries = entries
	})
	defaults, ok := importSourceEntries[source]
	return defaults, ok
}

// parseImportSources decodes the import-source file strictly: unknown keys
// are errors, and the sources must be exactly the ones the engine reads.
func parseImportSources(data []byte) (map[string]ImportSourceDefaults, error) {
	var file struct {
		Sources map[string]ImportSourceDefaults `yaml:"sources"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return nil, err
	}
	var names []string
	for name := range file.Sources {
		names = append(names, name)
	}
	sort.Strings(names)
	want := slices.Clone(importSources)
	sort.Strings(want)
	if !slices.Equal(names, want) {
		return nil, fmt.Errorf("sources are [%s], want [%s]", strings.Join(names, ", "), strings.Join(want, ", "))
	}
	for name, defaults := range file.Sources {
		defaults.Note = strings.TrimSpace(defaults.Note)
		file.Sources[name] = defaults
	}
	return file.Sources, nil
}
