package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestFindingsSchemaOmitsFollowUpRequests(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(FindingsSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	properties := schema["properties"].(map[string]any)
	if _, ok := properties["follow_up_requests"]; ok {
		t.Fatalf("schema unexpectedly contains follow_up_requests: %#v", properties["follow_up_requests"])
	}
}

func TestFindingsSchemaStripsExamples(t *testing.T) {
	if schemaContainsKey(FindingsSchema, "examples") {
		t.Fatalf("schema unexpectedly contains examples: %s", FindingsSchema)
	}
}

func TestFindingsExamplePromptSnippetOmitsFollowUpRequests(t *testing.T) {
	snippet := FindingsExamplePromptSnippet()
	for _, unwanted := range []string{`"follow_up_requests"`, `"type": "file"`, `"path": "file.go"`} {
		if strings.Contains(snippet, unwanted) {
			t.Fatalf("snippet unexpectedly contains %q: %s", unwanted, snippet)
		}
	}
}

func TestFindingsExamplePromptSnippetIsJSON(t *testing.T) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(FindingsExamplePromptSnippet()), &payload); err != nil {
		t.Fatalf("snippet is not valid json: %v", err)
	}
}

func TestFindingsSchemaIncludesSuggestions(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(FindingsSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	properties := schema["properties"].(map[string]any)
	findings := properties["findings"].(map[string]any)
	items := findings["items"].(map[string]any)
	findingProps := items["properties"].(map[string]any)
	suggestions, ok := findingProps["suggestions"].(map[string]any)
	if !ok {
		t.Fatalf("suggestions schema missing: %#v", findingProps["suggestions"])
	}
	if suggestions["type"] != "array" {
		t.Fatalf("suggestions schema type = %#v", suggestions["type"])
	}
	suggestion, ok := suggestions["items"].(map[string]any)
	if !ok {
		t.Fatalf("suggestions.items schema missing: %#v", suggestions["items"])
	}

	suggestionProps := suggestion["properties"].(map[string]any)
	if _, ok := suggestionProps["body"]; !ok {
		t.Fatalf("suggestion.body schema missing: %#v", suggestionProps)
	}
	codeLocation, ok := suggestionProps["code_location"].(map[string]any)
	if !ok {
		t.Fatalf("suggestion.code_location schema missing: %#v", suggestionProps["code_location"])
	}
	codeLocationProps := codeLocation["properties"].(map[string]any)
	lineRange, ok := codeLocationProps["line_range"].(map[string]any)
	if !ok {
		t.Fatalf("suggestion.code_location.line_range schema missing: %#v", codeLocationProps["line_range"])
	}
	if _, ok := codeLocationProps["content"]; !ok {
		t.Fatalf("suggestion.code_location.content schema missing: %#v", codeLocationProps)
	}
	lineRangeProps := lineRange["properties"].(map[string]any)
	if _, ok := lineRangeProps["start"]; !ok {
		t.Fatalf("suggestion.line_range.start schema missing: %#v", lineRangeProps)
	}
	if _, ok := lineRangeProps["end"]; !ok {
		t.Fatalf("suggestion.line_range.end schema missing: %#v", lineRangeProps)
	}
	if _, ok := lineRangeProps["count"]; !ok {
		t.Fatalf("suggestion.line_range.count schema missing: %#v", lineRangeProps)
	}
}

func TestFindingsExamplePromptSnippetIncludesSuggestions(t *testing.T) {
	snippet := FindingsExamplePromptSnippet()
	for _, required := range []string{`"suggestions"`, `"body": "Change the code to so and so"`, `"code_location"`, `"line_range"`, `"count": 6`, `"content"`} {
		if !strings.Contains(snippet, required) {
			t.Fatalf("snippet missing %q: %s", required, snippet)
		}
	}
	if strings.Contains(snippet, `"suggestion"`) {
		t.Fatalf("snippet unexpectedly contains singular suggestion: %s", snippet)
	}
}

func TestFindingsSchemaWithoutSuggestionsOmitsSuggestions(t *testing.T) {
	if schemaContainsKey(FindingsSchemaWithoutSuggestions, "suggestions") {
		t.Fatalf("schema unexpectedly contains suggestions: %s", FindingsSchemaWithoutSuggestions)
	}
	if strings.Contains(FindingsExamplePromptSnippetFor(true), `"suggestions"`) {
		t.Fatalf("snippet unexpectedly contains suggestions: %s", FindingsExamplePromptSnippetFor(true))
	}
}

func TestMergeAndFinalizeSchemasWithoutSuggestionsOmitSuggestions(t *testing.T) {
	if schemaContainsKey(MergeSchemaWithoutSuggestions, "suggestions") {
		t.Fatalf("merge schema unexpectedly contains suggestions: %s", MergeSchemaWithoutSuggestions)
	}
	if schemaContainsKey(FinalizeSchemaWithoutSuggestions, "suggestions") {
		t.Fatalf("finalize schema unexpectedly contains suggestions: %s", FinalizeSchemaWithoutSuggestions)
	}
	for name, snippet := range map[string]string{
		"merge":    MergeExamplePromptSnippetFor(true),
		"finalize": FinalizeExamplePromptSnippetFor(true),
	} {
		if strings.Contains(snippet, `"suggestions"`) {
			t.Fatalf("%s snippet unexpectedly contains suggestions: %s", name, snippet)
		}
	}
}

func TestFindingsSchemaRequiresPriorityWithoutID(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(FindingsSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	findings := properties["findings"].(map[string]any)
	items := findings["items"].(map[string]any)
	requiredAny, ok := items["required"].([]any)
	if !ok {
		t.Fatalf("required is not an array: %#v", items["required"])
	}
	required := make([]string, 0, len(requiredAny))
	for _, r := range requiredAny {
		if s, ok := r.(string); ok {
			required = append(required, s)
		}
	}
	if !slices.Contains(required, "priority") {
		t.Fatalf("required does not include priority: %v", required)
	}
	if slices.Contains(required, "id") {
		t.Fatalf("review schema should not require id: %v", required)
	}
}

func TestMergeSchemaRequiresVerification(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(MergeSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	required := findingRequiredFields(t, schema)
	for _, want := range []string{"id", "verification"} {
		if !slices.Contains(required, want) {
			t.Fatalf("required missing %q: %v", want, required)
		}
	}
	findingProps := findingProperties(t, schema)
	if _, ok := findingProps["verification"].(map[string]any); !ok {
		t.Fatalf("verification schema missing: %#v", findingProps["verification"])
	}
	suggestions := findingProps["suggestions"].(map[string]any)
	if suggestions["maxItems"] != float64(1) {
		t.Fatalf("suggestions.maxItems = %#v, want 1", suggestions["maxItems"])
	}
	requiredJSON := []byte(`"required":["id","title","body","confidence_score","priority","code_location","verification"]`)
	if !bytes.Contains(MergeSchema, requiredJSON) {
		t.Fatalf("raw merge schema missing required verification: %s", MergeSchema)
	}
}

func TestMergeExamplePromptSnippetIncludesVerification(t *testing.T) {
	snippet := MergeExamplePromptSnippet()
	for _, required := range []string{`"id"`, `"verification"`, `"verdict"`, `"confidence_score"`, `"remarks"`} {
		if !strings.Contains(snippet, required) {
			t.Fatalf("snippet missing %q: %s", required, snippet)
		}
	}
}

func TestConstrainedMergeSchemaLimitsSuggestions(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(MergeSchemaWithConstraintsFor(ResponseConstraints{}, false), &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	suggestions := findingProperties(t, schema)["suggestions"].(map[string]any)
	if suggestions["maxItems"] != float64(1) {
		t.Fatalf("suggestions.maxItems = %#v, want 1", suggestions["maxItems"])
	}
}

func TestLimitFindingSuggestionItemsIgnoresMalformedSchema(t *testing.T) {
	tests := []map[string]any{
		nil,
		{},
		{"properties": "invalid"},
		{"properties": map[string]any{}},
		{"properties": map[string]any{"findings": "invalid"}},
		{"properties": map[string]any{"findings": map[string]any{}}},
		{"properties": map[string]any{"findings": map[string]any{"items": "invalid"}}},
		{"properties": map[string]any{"findings": map[string]any{"items": map[string]any{}}}},
		{"properties": map[string]any{"findings": map[string]any{"items": map[string]any{"properties": "invalid"}}}},
		{"properties": map[string]any{"findings": map[string]any{"items": map[string]any{"properties": map[string]any{}}}}},
	}

	for i, schema := range tests {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			limitFindingSuggestionItems(schema, 1)
		})
	}
}

func TestFindingsExamplePromptSnippetOmitsID(t *testing.T) {
	snippet := FindingsExamplePromptSnippet()
	if strings.Contains(snippet, `"id"`) {
		t.Fatalf("review snippet should omit id: %s", snippet)
	}
}

func TestFindingsExamplePromptSnippetHasUnprefixedTitle(t *testing.T) {
	snippet := FindingsExamplePromptSnippet()
	for _, unwanted := range []string{`[P0]`, `[P1]`, `[P2]`, `[P3]`} {
		if strings.Contains(snippet, unwanted) {
			t.Fatalf("snippet unexpectedly contains priority prefix %q: %s", unwanted, snippet)
		}
	}
	if !strings.Contains(snippet, `"title": "Example title"`) {
		t.Fatalf("snippet missing unprefixed title: %s", snippet)
	}
}

func TestFinalizeSchemaRequiresFinalizationAndVerification(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(FinalizeSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	required := findingRequiredFields(t, schema)
	for _, want := range []string{"finalization", "verification"} {
		if !slices.Contains(required, want) {
			t.Fatalf("required missing %q: %v", want, required)
		}
	}
	findingProps := findingProperties(t, schema)
	if _, ok := findingProps["suggestions"]; ok {
		t.Fatalf("finalize schema should put suggestions under finalization, not top-level: %#v", findingProps["suggestions"])
	}
	finalization, ok := findingProps["finalization"].(map[string]any)
	if !ok {
		t.Fatalf("finalization schema missing: %#v", findingProps["finalization"])
	}
	finalizationProps := finalization["properties"].(map[string]any)
	if _, ok := finalizationProps["suggestions"].(map[string]any); !ok {
		t.Fatalf("finalization.suggestions schema missing: %#v", finalizationProps)
	}
	if _, ok := findingProps["verification"].(map[string]any); !ok {
		t.Fatalf("verification schema missing: %#v", findingProps["verification"])
	}
	requiredJSON := []byte(`"required":["id","title","body","confidence_score","priority","code_location","verification","finalization"]`)
	if !bytes.Contains(FinalizeSchema, requiredJSON) {
		t.Fatalf("raw finalize schema missing required verification: %s", FinalizeSchema)
	}
}

func findingRequiredFields(t *testing.T, schema map[string]any) []string {
	t.Helper()
	properties := schema["properties"].(map[string]any)
	findings := properties["findings"].(map[string]any)
	items := findings["items"].(map[string]any)
	requiredAny := items["required"].([]any)
	required := make([]string, 0, len(requiredAny))
	for _, r := range requiredAny {
		if s, ok := r.(string); ok {
			required = append(required, s)
		}
	}
	return required
}

func findingProperties(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()
	properties := schema["properties"].(map[string]any)
	findings := properties["findings"].(map[string]any)
	items := findings["items"].(map[string]any)
	return items["properties"].(map[string]any)
}

func TestFinalizeExamplePromptSnippetIncludesFinalization(t *testing.T) {
	snippet := FinalizeExamplePromptSnippet()
	for _, required := range []string{`"id"`, `"verification"`, `"finalization"`, `"title"`, `"body"`, `"confidence_score"`, `"remarks"`, `"suggestions"`} {
		if !strings.Contains(snippet, required) {
			t.Fatalf("snippet missing %q: %s", required, snippet)
		}
	}
}
