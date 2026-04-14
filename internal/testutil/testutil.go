package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func LoadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

func AssertGolden(t *testing.T, got string, goldenPath string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", goldenPath, err)
		}
	}
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", goldenPath, err)
	}
	if string(expected) != got {
		t.Fatalf("golden mismatch for %s\nexpected:\n%s\ngot:\n%s", filepath.Base(goldenPath), string(expected), got)
	}
}
