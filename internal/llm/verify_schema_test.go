package llm

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
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
	for _, field := range []string{"id", "verdict", "gate", "priority", "confidence_score", "remarks"} {
		if !slices.Contains(required, field) {
			t.Fatalf("required missing %q: %v", field, required)
		}
	}
}

func TestScopedVerifySchemaRequiresNullableReplacementLocation(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(ScopedVerifySchema, &schema); err != nil {
		t.Fatal(err)
	}
	requiredAny, _ := schema["required"].([]any)
	required := make([]string, 0, len(requiredAny))
	for _, value := range requiredAny {
		if field, ok := value.(string); ok {
			required = append(required, field)
		}
	}
	if !slices.Contains(required, "replacement_code_location") {
		t.Fatalf("required = %v", required)
	}
	properties := schema["properties"].(map[string]any)
	replacement := properties["replacement_code_location"].(map[string]any)
	if len(replacement["anyOf"].([]any)) != 2 {
		t.Fatalf("replacement schema = %#v", replacement)
	}
	var example map[string]any
	if err := json.Unmarshal([]byte(ScopedVerifyExamplePromptSnippet()), &example); err != nil {
		t.Fatal(err)
	}
	if value, ok := example["replacement_code_location"]; !ok || value != nil {
		t.Fatalf("replacement example = %#v", value)
	}
}

func TestVerifySchemaStripsExamples(t *testing.T) {
	if schemaContainsKey(VerifySchema, "examples") {
		t.Fatalf("schema unexpectedly contains examples: %s", VerifySchema)
	}
}

func TestVerifyExamplePromptSnippetIncludesAllFields(t *testing.T) {
	snippet := VerifyExamplePromptSnippet()
	for _, required := range []string{`"id"`, `"verdict"`, `"gate"`, `"priority"`, `"confidence_score"`, `"remarks"`} {
		if !strings.Contains(snippet, required) {
			t.Fatalf("snippet missing %q: %s", required, snippet)
		}
	}
	if !strings.Contains(snippet, `"id": "<uuid-v4>"`) {
		t.Fatalf("snippet should use UUID placeholder: %s", snippet)
	}
	if !strings.Contains(snippet, `"verdict": "confirmed"`) {
		t.Fatalf("snippet should default verdict example to confirmed: %s", snippet)
	}
	if !strings.Contains(snippet, `"gate": "confirm"`) {
		t.Fatalf("snippet gate example must match the confirmed verdict example: %s", snippet)
	}
}

func TestVerifyExamplePromptSnippetIsJSON(t *testing.T) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(VerifyExamplePromptSnippet()), &payload); err != nil {
		t.Fatalf("snippet is not valid json: %v", err)
	}
}

func TestParseVerifyResponseRequiresFields(t *testing.T) {
	content := `{"id": "11111111-1111-4111-8111-111111111111", "verdict": "confirmed", "gate": "confirm", "priority": 1}`
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
	content := `{"id": "11111111-1111-4111-8111-111111111111", "verdict": "refuted", "gate": "refute", "priority": 2, "confidence_score": 0.81, "remarks": "not reachable"}`
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
	if resp.Verification.Verdict != "refuted" {
		t.Fatalf("verdict = %q, want refuted", resp.Verification.Verdict)
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

func TestParseScopedVerifyResponseRequiresAndParsesReplacementLocation(t *testing.T) {
	constraints := ResponseConstraints{RequireReplacementCodeLocation: true}
	missing := `{"id":"11111111-1111-4111-8111-111111111111","verdict":"confirmed","gate":"confirm","priority":1,"confidence_score":0.9,"remarks":"real"}`
	_, err := parseReviewResponse(missing, SchemaKindVerify, constraints)
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) || !slices.Contains(invalid.MissingFields, "replacement_code_location") {
		t.Fatalf("err = %#v, want missing replacement_code_location", err)
	}

	content := `{"id":"11111111-1111-4111-8111-111111111111","verdict":"confirmed","gate":"confirm","priority":1,"confidence_score":0.9,"remarks":"real","replacement_code_location":{"file_path":"f.go","line_range":{"start":7,"end":7,"count":1},"content":"changed"}}`
	resp, err := parseReviewResponse(content, SchemaKindVerify, constraints)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ReplacementCodeLocation == nil || resp.ReplacementCodeLocation.FilePath != "f.go" || resp.ReplacementCodeLocation.LineRange.Start != 7 {
		t.Fatalf("replacement = %#v", resp.ReplacementCodeLocation)
	}

	nullContent := `{"id":"11111111-1111-4111-8111-111111111111","verdict":"confirmed","gate":"confirm","priority":1,"confidence_score":0.9,"remarks":"real","replacement_code_location":null}`
	resp, err = parseReviewResponse(nullContent, SchemaKindVerify, constraints)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ReplacementCodeLocation != nil {
		t.Fatalf("null replacement = %#v", resp.ReplacementCodeLocation)
	}
}

func TestParseVerifyResponseRescalesMisscaledConfidence(t *testing.T) {
	content := `{"id": "11111111-1111-4111-8111-111111111111", "verdict": "confirmed", "gate": "confirm", "priority": 1, "confidence_score": 95, "remarks": "real"}`
	resp, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Verification == nil {
		t.Fatalf("verification nil")
	}
	if resp.Verification.ConfidenceScore != 0.95 {
		t.Fatalf("confidence = %f, want 0.95 (rescaled from 95)", resp.Verification.ConfidenceScore)
	}
}

func TestParseVerifyResponseSkipsMarkdownBeforeJSON(t *testing.T) {
	content := "# Verify Findings\n\n[P1] Finding summary\n\n```json\n" +
		`{"id": "11111111-1111-4111-8111-111111111111", "verdict": "confirmed", "gate": "confirm", "priority": 1, "confidence_score": 0.91, "remarks": "confirmed"}` +
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
		`{"id":"11111111-1111-4111-8111-111111111111","verdict":"confirmed","gate":"confirm","priority":2,"confidence_score":0.91,"remarks":"draft"}` +
		"\n```\n\nFinal:\n```json\n" +
		`{"id":"11111111-1111-4111-8111-111111111111","verdict":"refuted","gate":"refute","priority":0,"confidence_score":0.1,"remarks":"reconsidered"}` +
		"\n```\n"
	resp, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Verification == nil {
		t.Fatalf("verification nil")
	}
	if resp.Verification.Verdict != "refuted" {
		t.Fatalf("Verdict = %q, want refuted (last block must overwrite confirmed)", resp.Verification.Verdict)
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
		`{"id":"11111111-1111-4111-8111-111111111111","verdict":"confirmed","gate":"confirm","priority":2,"confidence_score":0.91,"remarks":"first"}` +
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
	if resp.Verification.Verdict != "confirmed" {
		t.Fatalf("Verdict = %q, partial block must not blank earlier value", resp.Verification.Verdict)
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
		`{"id":"11111111-1111-4111-8111-111111111111","verdict":"confirmed","gate":"confirm","priority":2}` +
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

func TestParseVerifyResponseRejectsMalformedTypedBlockAcrossBlocks(t *testing.T) {
	content := "```json\n" +
		`{"id":"11111111-1111-4111-8111-111111111111","verdict":"confirmed","gate":"confirm","priority":"high"}` +
		"\n```\n\nMore:\n```json\n" +
		`{"confidence_score":0.91,"remarks":"ok"}` +
		"\n```\n"
	_, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) {
		t.Fatalf("expected InvalidResponseError, got %v", err)
	}
	for _, want := range []string{"id", "verdict", "priority"} {
		if !slices.Contains(invalid.MissingFields, want) {
			t.Fatalf("missing fields = %v, want %q", invalid.MissingFields, want)
		}
	}
}

func TestParseVerifyResponseRejectsOutOfRangePriority(t *testing.T) {
	content := `{"id": "11111111-1111-4111-8111-111111111111", "verdict": "confirmed", "gate": "confirm", "priority": 9, "confidence_score": 0.5, "remarks": "x"}`
	_, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) {
		t.Fatalf("expected InvalidResponseError, got %v", err)
	}
	if len(invalid.MissingFields) == 0 {
		t.Fatalf("missing fields empty")
	}
}

func TestParseVerifyResponseRejectsGateVerdictMismatch(t *testing.T) {
	content := `{"id": "11111111-1111-4111-8111-111111111111", "verdict": "confirmed", "gate": "compile-error", "priority": 1, "confidence_score": 0.9, "remarks": "compile error"}`
	_, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) {
		t.Fatalf("expected InvalidResponseError, got %v", err)
	}
	if len(invalid.MissingFields) != 1 || !strings.Contains(invalid.MissingFields[0], `dictates "refuted"`) {
		t.Fatalf("missing fields = %v, want verdict dictated by gate", invalid.MissingFields)
	}
}

func TestParseVerifyResponseAcceptsCompileErrorRefutation(t *testing.T) {
	content := `{"id": "11111111-1111-4111-8111-111111111111", "verdict": "refuted", "gate": "compile-error", "priority": 1, "confidence_score": 0.9, "remarks": "compiler-caught"}`
	resp, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Verification.Gate != model.GateCompileError {
		t.Fatalf("verification = %+v", resp.Verification)
	}
}

func TestParseVerifyResponseRejectsUnknownGate(t *testing.T) {
	content := `{"id": "11111111-1111-4111-8111-111111111111", "verdict": "confirmed", "gate": "vibes", "priority": 1, "confidence_score": 0.9, "remarks": "x"}`
	_, err := parseReviewResponse(content, SchemaKindVerify, ResponseConstraints{})
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) {
		t.Fatalf("expected InvalidResponseError, got %v", err)
	}
	if len(invalid.MissingFields) != 1 || !strings.Contains(invalid.MissingFields[0], "gate") {
		t.Fatalf("missing fields = %v, want unknown gate rejection", invalid.MissingFields)
	}
}

func TestVerdictForGateCoversAllGates(t *testing.T) {
	cases := map[string]string{
		model.GateNonFinding:              model.VerdictRefuted,
		model.GateDiffScope:               model.VerdictRefuted,
		model.GateStyleguideContradiction: model.VerdictRefuted,
		model.GateCompileError:            model.VerdictRefuted,
		model.GateConfirm:                 model.VerdictConfirmed,
		model.GateRefute:                  model.VerdictRefuted,
		model.GateUnverified:              model.VerdictUnverified,
	}
	for gate, want := range cases {
		got, ok := model.VerdictForGate(gate)
		if !ok || got != want {
			t.Fatalf("VerdictForGate(%q) = %q/%v, want %q", gate, got, ok, want)
		}
	}
	if _, ok := model.VerdictForGate(""); ok {
		t.Fatal("empty gate must be unknown")
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
