package llm

var verdictSchemaDefinition = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"overall_correctness": map[string]any{"type": "string", "enum": []string{"patch is correct", "patch is incorrect"}, "examples": []any{"patch is incorrect"}},
		"overall_explanation": map[string]any{"type": "string", "examples": []any{"The patch is incorrect because it introduces a correctness issue."}},
	},
	"required": []string{"overall_correctness", "overall_explanation"},
}

var VerdictSchema = mustMarshalCleanSchema(verdictSchemaDefinition)

func VerdictSchemaWithConstraints(c ResponseConstraints) []byte {
	root := deepCopySchema(verdictSchemaDefinition).(map[string]any)
	allowed := c.AllowedCorrectness
	if len(allowed) == 0 {
		allowed = []string{"patch is correct", "patch is incorrect"}
	}
	props, ok := root["properties"].(map[string]any)
	if !ok {
		panic("llm: verdict schema missing properties")
	}
	correctness, ok := props["overall_correctness"].(map[string]any)
	if !ok {
		panic("llm: verdict schema missing overall_correctness")
	}
	correctness["enum"] = append([]string(nil), allowed...)
	return mustMarshalCleanSchema(root)
}

func VerdictExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(verdictSchemaDefinition)))
}
