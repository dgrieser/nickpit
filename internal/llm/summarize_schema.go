package llm

// summarizeSchemaDefinition is the minimal output contract for the summarize
// pass: one entry per text item carrying only its id and a shortened
// `summarization.body`. Finding metadata is copied in code; the verdict
// explanation is summarized by passing it as another text item.
var summarizeSchemaDefinition = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"findings": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string", "examples": []any{"<uuid-v4>"}},
					"summarization": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"body": map[string]any{"type": "string", "examples": []any{"Concise explanation of the issue.\n\nKey detail preserved on its own line."}},
						},
						"required": []string{"body"},
					},
				},
				"required": []string{"id", "summarization"},
			},
		},
	},
	"required": []string{"findings"},
}

// SummarizeSchema is the JSON schema enforced when --use-json-schema is set.
var SummarizeSchema = mustMarshalCleanSchema(summarizeSchemaDefinition)

// SummarizeExamplePromptSnippet renders the example output used both in the
// summarize system prompt and in JSON output-retry guidance.
func SummarizeExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(summarizeSchemaDefinition)))
}
