package workflow

import (
	"strings"
	"testing"
)

func TestEmbeddedImportSources(t *testing.T) {
	file, ok := ImportSourceDefaultsFor(ImportSourceFile)
	if !ok || file != (ImportSourceDefaults{}) {
		t.Fatalf("file defaults = %+v, %v", file, ok)
	}
	published, ok := ImportSourceDefaultsFor(ImportSourcePublishedReview)
	if !ok || !published.ExemptDiffScope || !published.PreferInMerge || !published.SkipDedupe || !strings.Contains(published.Note, "might be outdated") {
		t.Fatalf("published-review defaults = %+v, %v", published, ok)
	}
	if _, ok := ImportSourceDefaultsFor("scanner"); ok {
		t.Fatal("unknown source must not resolve")
	}
}

func TestParseImportSourcesRejects(t *testing.T) {
	for name, body := range map[string]string{
		"unknown key":    "sources:\n  file: {}\n  published-review: {skip_dedupes: true}\n",
		"missing source": "sources:\n  file: {}\n",
		"extra source":   "sources:\n  file: {}\n  published-review: {}\n  scanner: {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseImportSources([]byte(body)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
