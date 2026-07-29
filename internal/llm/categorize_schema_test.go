package llm

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/prompts"
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
	var schema map[string]any
	if err := json.Unmarshal(CategorizeSchema, &schema); err != nil {
		t.Fatal(err)
	}
	categories := schema["properties"].(map[string]any)["categories"].(map[string]any)
	if minItems, ok := categories["minItems"].(float64); !ok || minItems != 1 {
		t.Fatalf("categories minItems = %#v, want 1", categories["minItems"])
	}
	if unique, ok := categories["uniqueItems"].(bool); !ok || !unique {
		t.Fatalf("categories uniqueItems = %#v, want true", categories["uniqueItems"])
	}
}

func TestCategorizeSchemaOffersOnlyDescriptiveCategories(t *testing.T) {
	categories := categoriesEnum(t, CategorizeSchema)
	for _, want := range []string{model.CategoryConfirmation, model.CategoryCompilation, model.CategoryFinding} {
		if !slices.Contains(categories, want) {
			t.Fatalf("enum missing %q: %v", want, categories)
		}
	}
}

func TestCategorizeSchemaStripsExamples(t *testing.T) {
	if schemaContainsKey(CategorizeSchema, "examples") {
		t.Fatalf("schema unexpectedly contains examples: %s", CategorizeSchema)
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

func TestParseCategorizeResponseGuidanceListsDescriptiveCategories(t *testing.T) {
	content := `{"id":"11111111-1111-4111-8111-111111111111","categories":[],"remarks":"x"}`
	_, err := parseReviewResponse(content, SchemaKindCategorize, ResponseConstraints{})
	var invalid *InvalidResponseError
	if !asErr(err, &invalid) {
		t.Fatalf("expected InvalidResponseError, got %v", err)
	}
	guidance := invalid.RetryGuidanceData.(CategorizeRetryGuidance)
	if !slices.Equal(guidance.AllowedCategories, model.FindingCategories()) {
		t.Fatalf("allowed = %v, want %v", guidance.AllowedCategories, model.FindingCategories())
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

// The retry-guidance template is only ever rendered when a categorize response
// was already rejected, so a bad field name or syntax error would surface at
// the worst possible moment. Render it here instead.
func TestRenderCategorizeRetryGuidance(t *testing.T) {
	tmpl, err := prompts.Load(categorizeRetryGuidanceTemplate)
	if err != nil {
		t.Fatal(err)
	}
	allowed := model.FindingCategories()
	cases := []struct {
		name     string
		data     CategorizeRetryGuidance
		contains []string
		absent   []string
	}{
		{
			name:     "empty categories",
			data:     CategorizeRetryGuidance{EmptyCategories: true, AllowedCategories: allowed},
			contains: []string{"must never be empty", `["finding"]`, "`" + model.CategoryConfirmation + "`"},
			absent:   []string{"not allowed"},
		},
		{
			name:     "unknown categories",
			data:     CategorizeRetryGuidance{UnknownCategories: []string{"vibes"}, AllowedCategories: allowed},
			contains: []string{"not allowed", "`vibes`", "`" + model.CategoryFinding + "`"},
			absent:   []string{"must never be empty"},
		},
		{
			name: "both",
			data: CategorizeRetryGuidance{
				EmptyCategories:   true,
				UnknownCategories: []string{"vibes", "hunch"},
				AllowedCategories: allowed,
			},
			contains: []string{"must never be empty", "`vibes`", "`hunch`", "2 category names"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RenderPrompt(tmpl, tc.data)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if strings.TrimSpace(out) == "" {
				t.Fatal("rendered guidance is empty")
			}
			for _, want := range tc.contains {
				if !strings.Contains(out, want) {
					t.Errorf("guidance missing %q:\n%s", want, out)
				}
			}
			for _, banned := range tc.absent {
				if strings.Contains(out, banned) {
					t.Errorf("guidance unexpectedly contains %q:\n%s", banned, out)
				}
			}
		})
	}
}

// A rejection carrying no guidance data at all must still render cleanly rather
// than erroring inside the retry path.
func TestRenderCategorizeRetryGuidanceZeroValue(t *testing.T) {
	tmpl, err := prompts.Load(categorizeRetryGuidanceTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RenderPrompt(tmpl, CategorizeRetryGuidance{}); err != nil {
		t.Fatalf("render: %v", err)
	}
}
