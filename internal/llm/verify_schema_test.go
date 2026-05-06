package llm

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestVerifySchemaRequiresAllFields(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(VerifySchema, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	requiredAny, ok := schema["required"].([]any)
	if !ok {
		t.Fatalf("required not array: %#v", schema["required"])
	}
	required := make([]string, 0, len(requiredAny))
	for _, r := range requiredAny {
		if s, ok := r.(string); ok {
			required = append(required, s)
		}
	}
	for _, field := range []string{"valid", "priority", "confidence_score", "remarks"} {
		if !slices.Contains(required, field) {
			t.Fatalf("required missing %q: %v", field, required)
		}
	}
}

func TestVerifyExamplePromptSnippetIncludesAllFields(t *testing.T) {
	snippet := VerifyExamplePromptSnippet()
	for _, required := range []string{`"valid"`, `"priority"`, `"confidence_score"`, `"remarks"`} {
		if !strings.Contains(snippet, required) {
			t.Fatalf("snippet missing %q: %s", required, snippet)
		}
	}
}

func TestParseVerifyResponseRequiresFields(t *testing.T) {
	content := `{"valid": true, "priority": 1}`
	_, err := parseReviewResponse(content, SchemaKindVerify)
	if err == nil {
		t.Fatalf("expected InvalidResponseError")
	}
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) {
		t.Fatalf("err type = %T", err)
	}
	if !slices.Contains(invalid.MissingFields, "confidence_score") || !slices.Contains(invalid.MissingFields, "remarks") {
		t.Fatalf("missing fields = %v", invalid.MissingFields)
	}
}

func TestParseVerifyResponseHappyPath(t *testing.T) {
	content := `{"valid": false, "priority": 2, "confidence_score": 0.81, "remarks": "not reachable"}`
	resp, err := parseReviewResponse(content, SchemaKindVerify)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Verification == nil {
		t.Fatalf("verification nil")
	}
	if resp.Verification.Valid {
		t.Fatalf("valid should be false")
	}
	if resp.Verification.Priority != 2 {
		t.Fatalf("priority = %d", resp.Verification.Priority)
	}
	if resp.Verification.ConfidenceScore != 0.81 {
		t.Fatalf("confidence = %f", resp.Verification.ConfidenceScore)
	}
	if resp.Verification.Remarks != "not reachable" {
		t.Fatalf("remarks = %q", resp.Verification.Remarks)
	}
}

func TestParseVerifyResponseRejectsOutOfRangePriority(t *testing.T) {
	content := `{"valid": true, "priority": 9, "confidence_score": 0.5, "remarks": "x"}`
	_, err := parseReviewResponse(content, SchemaKindVerify)
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) {
		t.Fatalf("expected InvalidResponseError, got %v", err)
	}
	if len(invalid.MissingFields) == 0 {
		t.Fatalf("missing fields empty")
	}
}

func asErr(err error, target **InvalidResponseError) bool {
	if err == nil {
		return false
	}
	if v, ok := err.(*InvalidResponseError); ok {
		*target = v
		return true
	}
	return false
}
