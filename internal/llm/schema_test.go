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
