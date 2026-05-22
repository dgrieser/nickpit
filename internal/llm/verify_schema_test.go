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
	for _, field := range []string{"id", "valid", "priority", "confidence_score", "remarks"} {
		if !slices.Contains(required, field) {
			t.Fatalf("required missing %q: %v", field, required)
		}
	}
}

func TestVerifySchemaStripsExamples(t *testing.T) {
	if schemaContainsKey(VerifySchema, "examples") {
		t.Fatalf("schema unexpectedly contains examples: %s", VerifySchema)
	}
}

func TestVerifyExamplePromptSnippetIncludesAllFields(t *testing.T) {
	snippet := VerifyExamplePromptSnippet()
	for _, required := range []string{`"id"`, `"valid"`, `"priority"`, `"confidence_score"`, `"remarks"`} {
		if !strings.Contains(snippet, required) {
			t.Fatalf("snippet missing %q: %s", required, snippet)
		}
	}
	if !strings.Contains(snippet, `"id": "<uuid-v4>"`) {
		t.Fatalf("snippet should use UUID placeholder: %s", snippet)
	}
}

func TestVerifyExamplePromptSnippetIsJSON(t *testing.T) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(VerifyExamplePromptSnippet()), &payload); err != nil {
		t.Fatalf("snippet is not valid json: %v", err)
	}
}

func TestParseVerifyResponseRequiresFields(t *testing.T) {
	content := `{"id": "11111111-1111-4111-8111-111111111111", "valid": true, "priority": 1}`
	_, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
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
	content := `{"id": "11111111-1111-4111-8111-111111111111", "valid": false, "priority": 2, "confidence_score": 0.81, "remarks": "not reachable"}`
	resp, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Verification == nil {
		t.Fatalf("verification nil")
	}
	if resp.Verification.ID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("id = %q", resp.Verification.ID)
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

func TestParseVerifyResponseSkipsMarkdownBeforeJSON(t *testing.T) {
	content := "# Verify Findings\n\n[P1] Finding summary\n\n```json\n" +
		`{"id": "11111111-1111-4111-8111-111111111111", "valid": true, "priority": 1, "confidence_score": 0.91, "remarks": "confirmed"}` +
		"\n```\n"
	resp, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Verification == nil {
		t.Fatalf("verification nil")
	}
	if resp.Verification.ID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("id = %q", resp.Verification.ID)
	}
}

func TestParseVerifyResponseMergesMultipleBlocksLastFieldsWin(t *testing.T) {
	content := "First draft:\n```json\n" +
		`{"id":"11111111-1111-4111-8111-111111111111","valid":true,"priority":2,"confidence_score":0.91,"remarks":"draft"}` +
		"\n```\n\nFinal:\n```json\n" +
		`{"id":"11111111-1111-4111-8111-111111111111","valid":false,"priority":0,"confidence_score":0.1,"remarks":"reconsidered"}` +
		"\n```\n"
	resp, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Verification == nil {
		t.Fatalf("verification nil")
	}
	if resp.Verification.Valid != false {
		t.Fatalf("Valid = %v, want false (last block must overwrite true)", resp.Verification.Valid)
	}
	if resp.Verification.Priority != 0 {
		t.Fatalf("Priority = %d, want 0 (last block must overwrite 2)", resp.Verification.Priority)
	}
	if resp.Verification.ConfidenceScore != 0.1 {
		t.Fatalf("ConfidenceScore = %v, want 0.1", resp.Verification.ConfidenceScore)
	}
	if resp.Verification.Remarks != "reconsidered" {
		t.Fatalf("Remarks = %q", resp.Verification.Remarks)
	}
}

func TestParseVerifyResponsePartialSecondBlockPreservesEarlierFields(t *testing.T) {
	content := "```json\n" +
		`{"id":"11111111-1111-4111-8111-111111111111","valid":true,"priority":2,"confidence_score":0.91,"remarks":"first"}` +
		"\n```\n\nAddendum:\n```json\n" +
		`{"remarks":"actually, see line 42"}` +
		"\n```\n"
	resp, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Verification == nil {
		t.Fatalf("verification nil")
	}
	if resp.Verification.ID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("ID = %q, partial block must not blank earlier ID", resp.Verification.ID)
	}
	if resp.Verification.Valid != true {
		t.Fatalf("Valid = %v, partial block must not blank earlier value", resp.Verification.Valid)
	}
	if resp.Verification.Priority != 2 {
		t.Fatalf("Priority = %d, partial block must not blank earlier value", resp.Verification.Priority)
	}
	if resp.Verification.ConfidenceScore != 0.91 {
		t.Fatalf("ConfidenceScore = %v, partial block must not blank earlier value", resp.Verification.ConfidenceScore)
	}
	if resp.Verification.Remarks != "actually, see line 42" {
		t.Fatalf("Remarks = %q, addendum should win", resp.Verification.Remarks)
	}
}

func TestParseVerifyResponseMissingFieldsDetectionAcrossBlocks(t *testing.T) {
	content := "```json\n" +
		`{"id":"11111111-1111-4111-8111-111111111111","valid":true,"priority":2}` +
		"\n```\n\nMore:\n```json\n" +
		`{"confidence_score":0.91,"remarks":"ok"}` +
		"\n```\n"
	resp, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	if err != nil {
		t.Fatalf("expected no missing-field error, got: %v (verify = %+v)", err, resp.Verification)
	}
	if resp.Verification == nil || resp.Verification.Remarks != "ok" || resp.Verification.ConfidenceScore != 0.91 {
		t.Fatalf("verify = %+v, want merged across blocks", resp.Verification)
	}
}

func TestParseVerifyResponseRejectsOutOfRangePriority(t *testing.T) {
	content := `{"id": "11111111-1111-4111-8111-111111111111", "valid": true, "priority": 9, "confidence_score": 0.5, "remarks": "x"}`
	_, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
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
