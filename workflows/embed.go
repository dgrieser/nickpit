// Package workflows holds the embedded default review workflow spec and the
// generator for its documented example file. The binary parses Default() through
// the same loader as any user-supplied spec; workflow.yaml.example is generated
// from the same bytes (see ExampleYAML).
package workflows

import (
	"bytes"
	_ "embed"
)

// defaultSpecYAML is the canonical default workflow: collect context, run the six
// vector reviewers concurrently, verify, dedupe, merge, finalize, verdict,
// summarize.
//
//go:embed default.yaml
var defaultSpecYAML []byte

// exampleTemplate is the documented form of the default spec: prose plus a
// {{SPEC}} marker that ExampleYAML replaces with default.yaml verbatim.
//
//go:embed example.tmpl
var exampleTemplate string

// Default returns the embedded default workflow spec as YAML bytes. The returned
// slice is a copy, so callers may retain or mutate it freely.
func Default() []byte { return bytes.Clone(defaultSpecYAML) }
