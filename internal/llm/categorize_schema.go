package llm

import (
	"maps"

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
			"Assign at least one; when the item reports a concrete actionable problem and nothing suppresses it, assign \"" + model.CategoryFinding + "\". " +
			"This is a classification of what kind of item was submitted, never a judgement about whether its claim is technically true.",
		"examples": []any{[]any{model.CategoryFinding}},
	}
}

var categorizeSchemaDefinition = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{"type": "string", "examples": []any{"<uuid-v4>"}},
		"categories": categorizeCategoriesSchema([]any{
			model.CategoryConfirmation,
			model.CategoryCompilation,
			model.CategoryFinding,
		}),
		"remarks": map[string]any{"type": "string", "examples": []any{"Reports a concrete null dereference on the changed path."}},
	},
	"required": []string{"id", "categories", "remarks"},
}

// scopedCategorizeSchemaDefinition adds the diff-scope category and the
// relocation escape hatch: when the submitted location misses the diff but a
// concrete causal location overlaps it, the agent returns that location instead
// of tagging the finding out of scope.
var scopedCategorizeSchemaDefinition = func() map[string]any {
	properties := map[string]any{}
	maps.Copy(properties, categorizeSchemaDefinition["properties"].(map[string]any))
	properties["categories"] = categorizeCategoriesSchema([]any{
		model.CategoryConfirmation,
		model.CategoryCompilation,
		model.CategoryOutsideDiffScope,
		model.CategoryFinding,
	})
	properties["replacement_code_location"] = map[string]any{
		"anyOf": []any{
			codeLocationSchemaDefinition(),
			map[string]any{"type": "null"},
		},
		"examples": []any{nil},
	}
	required := append([]string{}, categorizeSchemaDefinition["required"].([]string)...)
	required = append(required, "replacement_code_location")
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}()

var CategorizeSchema = mustMarshalCleanSchema(categorizeSchemaDefinition)
var ScopedCategorizeSchema = mustMarshalCleanSchema(scopedCategorizeSchemaDefinition)

func CategorizeExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(categorizeSchemaDefinition)))
}

func ScopedCategorizeExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(scopedCategorizeSchemaDefinition)))
}
