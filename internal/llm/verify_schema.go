package llm

var verifySchemaDefinition = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{"type": "string", "examples": []any{"<uuid-v4>"}},
		"verdict": map[string]any{
			"type":        "string",
			"enum":        []any{"confirmed", "refuted", "unverified"},
			"description": "confirmed: finding is a real issue. refuted: concrete evidence contradicts it. unverified: insufficient evidence to confirm or refute.",
			"examples":    []any{"confirmed"},
		},
		"priority":         map[string]any{"type": "integer", "minimum": 0, "maximum": 3, "examples": []any{1}},
		"confidence_score": map[string]any{"type": "number", "minimum": 0, "maximum": 1, "examples": []any{0.85}},
		"remarks":          map[string]any{"type": "string", "examples": []any{"Example explanation of why this is a problem."}},
	},
	"required": []string{"id", "verdict", "priority", "confidence_score", "remarks"},
}

var scopedVerifySchemaDefinition = func() map[string]any {
	properties := map[string]any{}
	for key, value := range verifySchemaDefinition["properties"].(map[string]any) {
		properties[key] = value
	}
	properties["replacement_code_location"] = map[string]any{
		"anyOf": []any{
			codeLocationSchemaDefinition(),
			map[string]any{"type": "null"},
		},
		"examples": []any{nil},
	}
	required := append([]string{}, verifySchemaDefinition["required"].([]string)...)
	required = append(required, "replacement_code_location")
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}()

var VerifySchema = mustMarshalCleanSchema(verifySchemaDefinition)
var ScopedVerifySchema = mustMarshalCleanSchema(scopedVerifySchemaDefinition)

func VerifyExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(verifySchemaDefinition)))
}

func ScopedVerifyExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(scopedVerifySchemaDefinition)))
}
