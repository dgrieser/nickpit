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
// vector reviewers concurrently, verify, dedupe, merge, finalize, summarize. It
// carries no per-step overrides, so every step inherits the active profile.
//
//go:embed default.yaml
var defaultSpecYAML []byte

// simpleSpecYAML is the "simple" workflow: collect context, run the single
// composite review:simple reviewer (all focus areas in one agent) with one manual
// nudge round, then verify, dedupe, merge, finalize, summarize. A lighter
// alternative to the six-reviewer default.
//
//go:embed simple.yaml
var simpleSpecYAML []byte

// exampleTemplate is the documented form of the default spec: prose plus a
// {{SPEC}} marker that ExampleYAML replaces with default.yaml verbatim.
//
//go:embed example.tmpl
var exampleTemplate string

// Default returns the embedded default workflow spec as YAML bytes. The returned
// slice is a copy, so callers may retain or mutate it freely.
func Default() []byte { return bytes.Clone(defaultSpecYAML) }

// Simple returns the embedded "simple" workflow spec as YAML bytes. The returned
// slice is a copy, so callers may retain or mutate it freely.
func Simple() []byte { return bytes.Clone(simpleSpecYAML) }
