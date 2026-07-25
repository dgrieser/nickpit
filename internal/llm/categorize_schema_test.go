package llm

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
)

func schemaRequiredFields(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
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
	return required
}

func categoriesEnum(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	categories := properties["categories"].(map[string]any)
	items := categories["items"].(map[string]any)
	enumAny := items["enum"].([]any)
	out := make([]string, 0, len(enumAny))
	for _, v := range enumAny {
		out = append(out, v.(string))
	}
	return out
}

func TestCategorizeSchemaRequiresAllFields(t *testing.T) {
	required := schemaRequiredFields(t, CategorizeSchema)
	for _, field := range []string{"id", "categories", "remarks"} {
		if !slices.Contains(required, field) {
			t.Fatalf("required missing %q: %v", field, required)
		}
	}
	if slices.Contains(required, "replacement_code_location") {
		t.Fatalf("unscoped schema must not require replacement_code_location: %v", required)
	}
}

// An empty categories array must be impossible to produce validly: treating an
// uncategorized finding as uncategorizable would silently drop it.
func TestCategorizeSchemaForbidsEmptyCategories(t *testing.T) {
	for name, raw := range map[string]json.RawMessage{"unscoped": CategorizeSchema, "scoped": ScopedCategorizeSchema} {
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("%s: unmarshal schema: %v", name, err)
		}
		categories := schema["properties"].(map[string]any)["categories"].(map[string]any)
		if minItems, ok := categories["minItems"].(float64); !ok || minItems != 1 {
			t.Fatalf("%s: categories minItems = %#v, want 1", name, categories["minItems"])
		}
		if unique, ok := categories["uniqueItems"].(bool); !ok || !unique {
			t.Fatalf("%s: categories uniqueItems = %#v, want true", name, categories["uniqueItems"])
		}
	}
}

func TestCategorizeSchemaOffersDiffScopeOnlyWhenScoped(t *testing.T) {
	unscoped := categoriesEnum(t, CategorizeSchema)
	if slices.Contains(unscoped, model.CategoryOutsideDiffScope) {
		t.Fatalf("unscoped enum must not offer the diff-scope category: %v", unscoped)
	}
	for _, want := range []string{model.CategoryConfirmation, model.CategoryCompilation, model.CategoryFinding} {
		if !slices.Contains(unscoped, want) {
			t.Fatalf("unscoped enum missing %q: %v", want, unscoped)
		}
	}
	scoped := categoriesEnum(t, ScopedCategorizeSchema)
	for _, want := range model.FindingCategories() {
		if !slices.Contains(scoped, want) {
			t.Fatalf("scoped enum missing %q: %v", want, scoped)
		}
	}
}

func TestScopedCategorizeSchemaRequiresNullableReplacementLocation(t *testing.T) {
	required := schemaRequiredFields(t, ScopedCategorizeSchema)
	if !slices.Contains(required, "replacement_code_location") {
		t.Fatalf("required = %v", required)
	}
	var schema map[string]any
	if err := json.Unmarshal(ScopedCategorizeSchema, &schema); err != nil {
		t.Fatal(err)
	}
	replacement := schema["properties"].(map[string]any)["replacement_code_location"].(map[string]any)
	if len(replacement["anyOf"].([]any)) != 2 {
		t.Fatalf("replacement schema = %#v", replacement)
	}
	var example map[string]any
	if err := json.Unmarshal([]byte(ScopedCategorizeExamplePromptSnippet()), &example); err != nil {
		t.Fatal(err)
	}
	if value, ok := example["replacement_code_location"]; !ok || value != nil {
		t.Fatalf("replacement example = %#v", value)
	}
}

func TestCategorizeSchemaStripsExamples(t *testing.T) {
	if schemaContainsKey(CategorizeSchema, "examples") {
		t.Fatalf("schema unexpectedly contains examples: %s", CategorizeSchema)
	}
	if schemaContainsKey(ScopedCategorizeSchema, "examples") {
		t.Fatalf("scoped schema unexpectedly contains examples: %s", ScopedCategorizeSchema)
	}
}

func TestCategorizeExamplePromptSnippetIncludesAllFields(t *testing.T) {
	snippet := CategorizeExamplePromptSnippet()
	var payload map[string]any
	if err := json.Unmarshal([]byte(snippet), &payload); err != nil {
		t.Fatalf("snippet is not valid json: %v", err)
	}
	for _, required := range []string{"id", "categories", "remarks"} {
		if _, ok := payload[required]; !ok {
			t.Fatalf("snippet missing %q: %s", required, snippet)
		}
	}
	if !strings.Contains(snippet, `"id": "<uuid-v4>"`) {
		t.Fatalf("snippet should use UUID placeholder: %s", snippet)
	}
	// The example must show the surviving set, so a model copying it verbatim
	// keeps the finding rather than dropping it.
	if !strings.Contains(snippet, `"finding"`) {
		t.Fatalf("snippet categories example should be [finding]: %s", snippet)
	}
}

func TestParseCategorizeResponseHappyPath(t *testing.T) {
	content := `{"id":"11111111-1111-4111-8111-111111111111","categories":["finding"],"remarks":"concrete nil deref"}`
	resp, err := parseReviewResponse(content, SchemaKindCategorize, ResponseConstraints{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Categorization == nil {
		t.Fatal("categorization nil")
	}
	if got := resp.Categorization.Categories; !slices.Equal(got, []string{model.CategoryFinding}) {
		t.Fatalf("categories = %v", got)
	}
	if resp.Categorization.Remarks != "concrete nil deref" {
		t.Fatalf("remarks = %q", resp.Categorization.Remarks)
	}
}

func TestParseCategorizeResponseNormalizesAndDedupes(t *testing.T) {
	content := `{"id":"11111111-1111-4111-8111-111111111111","categories":["FINDING"," compilation ","finding"],"remarks":"x"}`
	resp, err := parseReviewResponse(content, SchemaKindCategorize, ResponseConstraints{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := []string{model.CategoryCompilation, model.CategoryFinding}
	if got := resp.Categorization.Categories; !slices.Equal(got, want) {
		t.Fatalf("categories = %v, want %v", got, want)
	}
}

func TestParseCategorizeResponseRequiresFields(t *testing.T) {
	content := `{"id":"11111111-1111-4111-8111-111111111111","categories":["finding"]}`
	_, err := parseReviewResponse(content, SchemaKindCategorize, ResponseConstraints{})
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) {
		t.Fatalf("err type = %T", err)
	}
	if !slices.Contains(invalid.MissingFields, "remarks") {
		t.Fatalf("missing fields = %v", invalid.MissingFields)
	}
}

// An empty category list is a retry, never a silent drop.
func TestParseCategorizeResponseRetriesEmptyCategories(t *testing.T) {
	for _, content := range []string{
		`{"id":"11111111-1111-4111-8111-111111111111","categories":[],"remarks":"unsure"}`,
		`{"id":"11111111-1111-4111-8111-111111111111","categories":["  "],"remarks":"unsure"}`,
	} {
		_, err := parseReviewResponse(content, SchemaKindCategorize, ResponseConstraints{})
		var invalid *InvalidResponseError
		if !asErr(err, &invalid) {
			t.Fatalf("content %s: expected InvalidResponseError, got %v", content, err)
		}
		if invalid.RetryGuidanceTemplate != "categorize_validation_retry_guidance.tmpl" {
			t.Fatalf("content %s: retry guidance = %q", content, invalid.RetryGuidanceTemplate)
		}
		guidance, ok := invalid.RetryGuidanceData.(CategorizeRetryGuidance)
		if !ok || !guidance.EmptyCategories {
			t.Fatalf("content %s: guidance = %#v, want EmptyCategories", content, invalid.RetryGuidanceData)
		}
		if !slices.Contains(guidance.AllowedCategories, model.CategoryFinding) {
			t.Fatalf("content %s: allowed = %v", content, guidance.AllowedCategories)
		}
	}
}

func TestParseCategorizeResponseRejectsUnknownCategory(t *testing.T) {
	content := `{"id":"11111111-1111-4111-8111-111111111111","categories":["finding","vibes"],"remarks":"x"}`
	_, err := parseReviewResponse(content, SchemaKindCategorize, ResponseConstraints{})
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) {
		t.Fatalf("expected InvalidResponseError, got %v", err)
	}
	guidance, ok := invalid.RetryGuidanceData.(CategorizeRetryGuidance)
	if !ok || !slices.Contains(guidance.UnknownCategories, "vibes") {
		t.Fatalf("guidance = %#v", invalid.RetryGuidanceData)
	}
}

// Without diff hunks the agent is never offered the diff-scope category, so it
// must not appear in the retry guidance either.
func TestParseCategorizeResponseGuidanceOmitsDiffScopeWhenUnscoped(t *testing.T) {
	content := `{"id":"11111111-1111-4111-8111-111111111111","categories":[],"remarks":"x"}`
	_, err := parseReviewResponse(content, SchemaKindCategorize, ResponseConstraints{})
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) {
		t.Fatalf("expected InvalidResponseError, got %v", err)
	}
	guidance := invalid.RetryGuidanceData.(CategorizeRetryGuidance)
	if slices.Contains(guidance.AllowedCategories, model.CategoryOutsideDiffScope) {
		t.Fatalf("allowed = %v, must omit the diff-scope category", guidance.AllowedCategories)
	}
}

func TestParseScopedCategorizeResponseRequiresAndParsesReplacementLocation(t *testing.T) {
	constraints := ResponseConstraints{RequireReplacementCodeLocation: true}
	missing := `{"id":"11111111-1111-4111-8111-111111111111","categories":["finding"],"remarks":"real"}`
	_, err := parseReviewResponse(missing, SchemaKindCategorize, constraints)
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) || !slices.Contains(invalid.MissingFields, "replacement_code_location") {
		t.Fatalf("err = %#v, want missing replacement_code_location", err)
	}

	content := `{"id":"11111111-1111-4111-8111-111111111111","categories":["finding"],"remarks":"real","replacement_code_location":{"file_path":"f.go","line_range":{"start":7,"end":7,"count":1},"content":"changed"}}`
	resp, err := parseReviewResponse(content, SchemaKindCategorize, constraints)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ReplacementCodeLocation == nil || resp.ReplacementCodeLocation.FilePath != "f.go" || resp.ReplacementCodeLocation.LineRange.Start != 7 {
		t.Fatalf("replacement = %#v", resp.ReplacementCodeLocation)
	}

	nullContent := `{"id":"11111111-1111-4111-8111-111111111111","categories":["finding"],"remarks":"real","replacement_code_location":null}`
	resp, err = parseReviewResponse(nullContent, SchemaKindCategorize, constraints)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ReplacementCodeLocation != nil {
		t.Fatalf("null replacement = %#v", resp.ReplacementCodeLocation)
	}
}

// Finding a causal line inside the diff proves the finding IS anchorable, so
// the two answers cannot both stand.
func TestParseCategorizeResponseRejectsOutOfScopeWithReplacement(t *testing.T) {
	constraints := ResponseConstraints{RequireReplacementCodeLocation: true}
	content := `{"id":"11111111-1111-4111-8111-111111111111","categories":["outside-diff-scope"],"remarks":"x","replacement_code_location":{"file_path":"f.go","line_range":{"start":7,"end":7,"count":1},"content":"changed"}}`
	_, err := parseReviewResponse(content, SchemaKindCategorize, constraints)
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) {
		t.Fatalf("expected InvalidResponseError, got %v", err)
	}
	if len(invalid.MissingFields) != 1 || !strings.Contains(invalid.MissingFields[0], model.CategoryOutsideDiffScope) {
		t.Fatalf("missing fields = %v", invalid.MissingFields)
	}
}

func TestParseCategorizeResponseMergesMultipleBlocksLastFieldsWin(t *testing.T) {
	content := "First:\n```json\n" +
		`{"id":"11111111-1111-4111-8111-111111111111","categories":["confirmation"],"remarks":"draft"}` +
		"\n```\n\nFinal:\n```json\n" +
		`{"id":"11111111-1111-4111-8111-111111111111","categories":["finding"],"remarks":"reconsidered"}` +
		"\n```\n"
	resp, err := parseReviewResponse(content, SchemaKindCategorize, ResponseConstraints{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got := resp.Categorization.Categories; !slices.Equal(got, []string{model.CategoryFinding}) {
		t.Fatalf("categories = %v, last block must win", got)
	}
	if resp.Categorization.Remarks != "reconsidered" {
		t.Fatalf("remarks = %q", resp.Categorization.Remarks)
	}
}
