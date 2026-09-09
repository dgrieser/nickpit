package llm

// updateFindingsSchemaDefinition uses the standard finding fields, but
// deliberately excludes agent provenance and SCM revision fields, which are
// owned by Go.
func updateFindingsSchemaDefinition(disableSuggestions, reviewOnly bool) map[string]any {
	if reviewOnly {
		return map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{
				"updates": map[string]any{"type": "array", "maxItems": 0, "items": map[string]any{"type": "object"}},
				"review": map[string]any{
					"type": "object", "additionalProperties": false,
					"properties": map[string]any{
						"action": map[string]any{"type": "string", "enum": []string{"unchanged", "correction_warranted"}},
						"reason": map[string]any{"type": "string", "examples": []any{"Example evidence that justifies this assessment."}},
					}, "required": []string{"action", "reason"},
				},
			}, "required": []string{"updates", "review"},
		}
	}
	base := buildFindingsSchemaDefinition(0, 3, nil, true, !disableSuggestions)
	finding := base["properties"].(map[string]any)["findings"].(map[string]any)["items"].(map[string]any)
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{"updates": map[string]any{
			"type": "array", "items": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{
					"id":      map[string]any{"type": "string", "examples": []any{"<uuid-v4>"}},
					"action":  map[string]any{"type": "string", "enum": []string{"unchanged", "updated", "resolved"}, "examples": []any{"updated"}},
					"reason":  map[string]any{"type": "string", "examples": []any{"Example evidence that justifies this decision."}},
					"finding": finding,
				},
				"required": []string{"id", "action", "reason"},
			},
		}}, "required": []string{"updates"},
	}
}

func UpdateFindingsSchema(disableSuggestions, reviewOnly bool) []byte {
	return mustMarshalCleanSchema(updateFindingsSchemaDefinition(disableSuggestions, reviewOnly))
}

// UpdateExamplePromptSnippetFor renders the example output shape for the update
// prompt. A review-only run returns no decisions, which the schema expresses as
// `maxItems: 0` and the example must show as an empty array.
func UpdateExamplePromptSnippetFor(disableSuggestions, reviewOnly bool) string {
	example, ok := exampleFromSchema(updateFindingsSchemaDefinition(disableSuggestions, reviewOnly)).(map[string]any)
	if !ok {
		panic("llm: update schema example is not an object")
	}
	if reviewOnly {
		example["updates"] = []any{}
	}
	return mustIndentJSON(mustMarshalJSON(example))
}
