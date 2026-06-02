package workflows

import (
	"bytes"
	"os"
	"testing"
)

// TestExampleMatchesCheckedIn pins the committed workflow.yaml.example to the
// generator output, so the example stays derived from default.yaml + example.tmpl.
// Line endings are normalized to LF first: with no .gitattributes pinning the
// file, core.autocrlf can check it out as CRLF, which would otherwise spuriously
// fail against the generator's LF output.
func TestExampleMatchesCheckedIn(t *testing.T) {
	want, err := ExampleYAML()
	if err != nil {
		t.Fatalf("ExampleYAML: %v", err)
	}
	got, err := os.ReadFile("../workflow.yaml.example")
	if err != nil {
		t.Fatalf("reading checked-in example: %v", err)
	}
	normalize := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	if !bytes.Equal(normalize(got), normalize(want)) {
		t.Fatalf("workflow.yaml.example is stale; run `make generate` (or `go generate ./workflows`)")
	}
}
