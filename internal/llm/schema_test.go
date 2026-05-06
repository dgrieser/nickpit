package llm

import (
	"encoding/json"
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

func TestFindingsExamplePromptSnippetOmitsFollowUpRequests(t *testing.T) {
	snippet := FindingsExamplePromptSnippet()
	for _, unwanted := range []string{`"follow_up_requests"`, `"type": "file"`, `"path": "file.go"`} {
		if strings.Contains(snippet, unwanted) {
			t.Fatalf("snippet unexpectedly contains %q: %s", unwanted, snippet)
		}
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
	lineRange, ok := suggestionProps["line_range"].(map[string]any)
	if !ok {
		t.Fatalf("suggestion.line_range schema missing: %#v", suggestionProps["line_range"])
	}
	lineRangeProps := lineRange["properties"].(map[string]any)
	if _, ok := lineRangeProps["start"]; !ok {
		t.Fatalf("suggestion.line_range.start schema missing: %#v", lineRangeProps)
	}
	if _, ok := lineRangeProps["end"]; !ok {
		t.Fatalf("suggestion.line_range.end schema missing: %#v", lineRangeProps)
	}
}

func TestFindingsExamplePromptSnippetIncludesSuggestions(t *testing.T) {
	snippet := FindingsExamplePromptSnippet()
	for _, required := range []string{`"suggestions"`, `"body": "replacement code"`, `"line_range"`, `"start": 10`, `"end": 12`} {
		if !strings.Contains(snippet, required) {
			t.Fatalf("snippet missing %q: %s", required, snippet)
		}
	}
	if strings.Contains(snippet, `"suggestion"`) {
		t.Fatalf("snippet unexpectedly contains singular suggestion: %s", snippet)
	}
}

func TestFindingsSchemaRequiresPriority(t *testing.T) {
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
