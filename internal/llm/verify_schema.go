package llm

var verifySchemaDefinition = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id":               map[string]any{"type": "string", "examples": []any{"11111111-1111-4111-8111-111111111111"}},
		"valid":            map[string]any{"type": "boolean", "examples": []any{false}},
		"priority":         map[string]any{"type": "integer", "minimum": 0, "maximum": 3, "examples": []any{1}},
		"confidence_score": map[string]any{"type": "number", "minimum": 0, "maximum": 1, "examples": []any{0.85}},
		"remarks":          map[string]any{"type": "string", "examples": []any{"Example explanation of why this is a problem."}},
	},
	"required": []string{"id", "valid", "priority", "confidence_score", "remarks"},
}

var VerifySchema = mustMarshalCleanSchema(verifySchemaDefinition)

func VerifyExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(verifySchemaDefinition)))
}
