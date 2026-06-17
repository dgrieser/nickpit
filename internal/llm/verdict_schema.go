package llm

var verdictSchemaDefinition = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"overall_correctness":      map[string]any{"type": "string", "enum": []string{"patch is correct", "patch is incorrect"}, "examples": []any{"patch is incorrect"}},
		"overall_explanation":      map[string]any{"type": "string", "examples": []any{"The patch is incorrect because it introduces a correctness issue."}},
		"overall_confidence_score": map[string]any{"type": "number", "minimum": 0, "maximum": 1, "examples": []any{0.85}},
	},
	"required": []string{"overall_correctness", "overall_explanation", "overall_confidence_score"},
}

var VerdictSchema = mustMarshalCleanSchema(verdictSchemaDefinition)

func VerdictExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(verdictSchemaDefinition)))
}
