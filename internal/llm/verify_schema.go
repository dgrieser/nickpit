package llm

var VerifySchema = mustMarshalJSON(map[string]any{
	"type": "object",
	"properties": map[string]any{
		"valid":            map[string]any{"type": "boolean"},
		"priority":         map[string]any{"type": "integer", "minimum": 0, "maximum": 3},
		"confidence_score": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		"remarks":          map[string]any{"type": "string"},
	},
	"required": []string{"valid", "priority", "confidence_score", "remarks"},
})

func VerifyExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchemaJSON(VerifySchema)))
}
