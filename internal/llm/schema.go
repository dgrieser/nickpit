package llm

import (
	"encoding/json"
	"fmt"

	"github.com/dgrieser/nickpit/prompts"
)

var FindingsSchema = mustMarshalJSON(map[string]any{
	"type": "object",
	"properties": map[string]any{
		"findings": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":            map[string]any{"type": "string"},
					"body":             map[string]any{"type": "string"},
					"confidence_score": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
					"priority":         map[string]any{"type": "integer", "minimum": 0, "maximum": 3},
					"code_location": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"file_path": map[string]any{"type": "string"},
							"line_range": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"start": map[string]any{"type": "integer"},
									"end":   map[string]any{"type": "integer"},
								},
								"required": []string{"start", "end"},
							},
						},
						"required": []string{"file_path", "line_range"},
					},
					"suggestions": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"body": map[string]any{"type": "string"},
								"line_range": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"start": map[string]any{"type": "integer"},
										"end":   map[string]any{"type": "integer"},
									},
									"required": []string{"start", "end"},
								},
							},
							"required": []string{"body", "line_range"},
						},
					},
				},
				"required": []string{"title", "body", "confidence_score", "priority", "code_location"},
			},
		},
		"overall_correctness":      map[string]any{"type": "string", "enum": []string{"patch is correct", "patch is incorrect"}},
		"overall_explanation":      map[string]any{"type": "string"},
		"overall_confidence_score": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
	},
	"required": []string{"findings", "overall_correctness", "overall_explanation", "overall_confidence_score"},
})

func FindingsExamplePromptSnippet() string {
	return mustLoadPrompt("agent_review_result_snippet.tmpl")
}

func mustMarshalJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("llm: marshal schema: %v", err))
	}
	return json.RawMessage(data)
}

func mustLoadPrompt(name string) string {
	content, err := prompts.Load(name)
	if err != nil {
		panic(fmt.Sprintf("llm: load prompt %s: %v", name, err))
	}
	return content
}
