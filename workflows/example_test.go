package workflows

import (
	"os"
	"testing"
)

// TestExampleMatchesCheckedIn pins the committed workflow.yaml.example to the
// generator output, so the example stays derived from default.yaml + example.tmpl.
func TestExampleMatchesCheckedIn(t *testing.T) {
	want, err := ExampleYAML()
	if err != nil {
		t.Fatalf("ExampleYAML: %v", err)
	}
	got, err := os.ReadFile("../workflow.yaml.example")
	if err != nil {
		t.Fatalf("reading checked-in example: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("workflow.yaml.example is stale; run `make generate` (or `go generate ./workflows`)")
	}
}
