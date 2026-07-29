package llm

import (
	"github.com/dgrieser/nickpit/internal/model"
)

// categorizeCategoriesSchema builds the schema for the required categories
// field. minItems is 1 on purpose: an empty array is never a valid answer, and
// the parser turns one into a retry rather than treating the finding as
// uncategorized (and droppable).
func categorizeCategoriesSchema(categories []any) map[string]any {
	return map[string]any{
		"type":        "array",
		"minItems":    1,
		"uniqueItems": true,
		"items": map[string]any{
			"type": "string",
			"enum": categories,
		},
		"description": "Every category that applies to this finding. The categories are not mutually exclusive — assign all that fit, not just the first. " +
			"Assign at least one; when the item reports a concrete actionable problem, assign \"" + model.CategoryFinding + "\". " +
			"This is a classification of what kind of item was submitted, never a judgement about whether its claim is technically true.",
		"examples": []any{[]any{model.CategoryFinding}},
	}
}

var categorizeSchemaDefinition = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id":         map[string]any{"type": "string", "examples": []any{"<uuid-v4>"}},
		"categories": categorizeCategoriesSchema(findingCategorySchemaValues()),
		"remarks":    map[string]any{"type": "string", "examples": []any{"Reports a concrete null dereference on the changed path."}},
	},
	"required": []string{"id", "categories", "remarks"},
}

func findingCategorySchemaValues() []any {
	categories := model.FindingCategories()
	values := make([]any, len(categories))
	for i, category := range categories {
		values[i] = category
	}
	return values
}

var CategorizeSchema = mustMarshalCleanSchema(categorizeSchemaDefinition)

func CategorizeExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(categorizeSchemaDefinition)))
}
