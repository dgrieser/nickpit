package llm

// summarizeSchemaDefinition is the minimal output contract for the summarize
// pass: one entry per finding carrying only its id and a shortened
// `summarization.body`. Every other summarization field (title, priority,
// confidence_score, remarks) is copied in code from the finding's finalization
// (see applySummarizedFinding in internal/review/summarizer.go), so the model is
// never asked to re-emit them — keeping the output small and hard to get wrong.
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
