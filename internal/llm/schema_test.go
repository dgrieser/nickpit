package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFindingsSchemaLimitsFollowUpRequestsToFiles(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(FindingsSchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	properties := schema["properties"].(map[string]any)
	followUps := properties["follow_up_requests"].(map[string]any)
	items := followUps["items"].(map[string]any)
	itemProperties := items["properties"].(map[string]any)

	if len(itemProperties) != 3 {
		t.Fatalf("follow_up_requests properties = %#v", itemProperties)
	}
	if _, ok := itemProperties["type"]; !ok {
		t.Fatalf("follow_up_requests missing type property: %#v", itemProperties)
	}
	if _, ok := itemProperties["path"]; !ok {
		t.Fatalf("follow_up_requests missing path property: %#v", itemProperties)
	}
	if _, ok := itemProperties["reason"]; !ok {
		t.Fatalf("follow_up_requests missing reason property: %#v", itemProperties)
	}
	if _, ok := itemProperties["start_line"]; ok {
		t.Fatalf("follow_up_requests unexpectedly contains start_line: %#v", itemProperties)
	}
	if _, ok := itemProperties["end_line"]; ok {
		t.Fatalf("follow_up_requests unexpectedly contains end_line: %#v", itemProperties)
	}
	if _, ok := itemProperties["symbol"]; ok {
		t.Fatalf("follow_up_requests unexpectedly contains symbol: %#v", itemProperties)
	}
	if _, ok := itemProperties["depth"]; ok {
		t.Fatalf("follow_up_requests unexpectedly contains depth: %#v", itemProperties)
	}

	typeProperty := itemProperties["type"].(map[string]any)
	enumValues := typeProperty["enum"].([]any)
	if len(enumValues) != 1 || enumValues[0] != "file" {
		t.Fatalf("follow_up_requests type enum = %#v", enumValues)
	}

	required := items["required"].([]any)
	if len(required) != 3 || required[0] != "type" || required[1] != "path" || required[2] != "reason" {
		t.Fatalf("follow_up_requests required = %#v", required)
	}
}

func TestFindingsExamplePromptSnippetUsesFilenameOnlyFollowUps(t *testing.T) {
	snippet := FindingsExamplePromptSnippet()
	if want := `"type": "file"`; !strings.Contains(snippet, want) {
		t.Fatalf("snippet missing %q: %s", want, snippet)
	}
	if want := `"path": "file.go"`; !strings.Contains(snippet, want) {
		t.Fatalf("snippet missing %q: %s", want, snippet)
	}
	for _, unwanted := range []string{`"start_line":`, `"end_line":`, `"symbol":`, `"depth":`, `"lines"`, `"function"`, `"callers"`, `"callees"`} {
		if strings.Contains(snippet, unwanted) {
			t.Fatalf("snippet unexpectedly contains %q: %s", unwanted, snippet)
		}
	}
}
