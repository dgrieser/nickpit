package llm

import (
	"encoding/json"
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

func TestFindingsSchemaIncludesSuggestionBlock(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(FindingsSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	properties := schema["properties"].(map[string]any)
	findings := properties["findings"].(map[string]any)
	items := findings["items"].(map[string]any)
	findingProps := items["properties"].(map[string]any)
	suggestion, ok := findingProps["suggestion"].(map[string]any)
	if !ok {
		t.Fatalf("suggestion schema missing: %#v", findingProps["suggestion"])
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

func TestFindingsExamplePromptSnippetIncludesSuggestionBlock(t *testing.T) {
	snippet := FindingsExamplePromptSnippet()
	for _, required := range []string{`"suggestion"`, `"body": "replacement code"`, `"line_range"`, `"start": 10`, `"end": 12`} {
		if !strings.Contains(snippet, required) {
			t.Fatalf("snippet missing %q: %s", required, snippet)
		}
	}
}
