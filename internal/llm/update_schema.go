package llm

// UpdateFindingsSchema uses the standard finding fields, but deliberately
// excludes agent provenance and SCM revision fields, which are owned by Go.
func UpdateFindingsSchema(disableSuggestions, reviewOnly bool) []byte {
	if reviewOnly {
		return mustMarshalCleanSchema(map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{
				"updates": map[string]any{"type": "array", "maxItems": 0, "items": map[string]any{"type": "object"}},
				"review": map[string]any{
					"type": "object", "additionalProperties": false,
					"properties": map[string]any{
						"action": map[string]any{"type": "string", "enum": []string{"unchanged", "correction_warranted"}},
						"reason": map[string]any{"type": "string"},
					}, "required": []string{"action", "reason"},
				},
			}, "required": []string{"updates", "review"},
		})
	}
	base := buildFindingsSchemaDefinition(0, 3, nil, true, !disableSuggestions)
	finding := base["properties"].(map[string]any)["findings"].(map[string]any)["items"].(map[string]any)
	return mustMarshalCleanSchema(map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{"updates": map[string]any{
			"type": "array", "items": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{
					"id":      map[string]any{"type": "string"},
					"action":  map[string]any{"type": "string", "enum": []string{"unchanged", "updated", "resolved"}},
					"reason":  map[string]any{"type": "string"},
					"finding": finding,
				},
				"required": []string{"id", "action", "reason"},
			},
		}}, "required": []string{"updates"},
	})
}
