package llm

import "encoding/json"

var mergeSchemaDefinition = buildMergeSchemaDefinition()

func buildMergeSchemaDefinition() map[string]any {
	root := deepCopySchema(findingsWithIDSchemaDefinition).(map[string]any)
	limitFindingSuggestionItems(root, 1)
	return extendFindingsForMergeProvenance(extendFindingsForVerification(root))
}

var mergeWithoutSuggestionsSchemaDefinition = buildMergeWithoutSuggestionsSchemaDefinition()

func buildMergeWithoutSuggestionsSchemaDefinition() map[string]any {
	return extendFindingsForMergeProvenance(extendFindingsForVerification(deepCopySchema(findingsWithIDWithoutSuggestionsSchemaDefinition).(map[string]any)))
}

// extendFindingsForMergeProvenance adds the optional merged_from array the
// cluster merge agent uses to account for absorbed duplicates: a finding that
// folds others into itself lists their ids. Intentionally not required —
// findings kept unchanged carry no provenance.
func extendFindingsForMergeProvenance(root map[string]any) map[string]any {
	properties, ok := root["properties"].(map[string]any)
	if !ok {
		panic("llm: findings schema missing properties")
	}
	findings, ok := properties["findings"].(map[string]any)
	if !ok {
		panic("llm: findings schema missing findings property")
	}
	items, ok := findings["items"].(map[string]any)
	if !ok {
		panic("llm: findings schema findings.items malformed")
	}
	itemProps, ok := items["properties"].(map[string]any)
	if !ok {
		panic("llm: findings schema findings.items.properties malformed")
	}
	itemProps["merged_from"] = map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string", "examples": []any{"<uuid-v4>"}},
	}
	return root
}

var MergeSchema = mustMarshalCleanSchema(mergeSchemaDefinition)

var MergeSchemaWithoutSuggestions = mustMarshalCleanSchema(mergeWithoutSuggestionsSchemaDefinition)

// MergeSchemaWithConstraints returns the merge schema narrowed by the given
// constraints (priority bounds + allowed overall_correctness values).
func MergeSchemaWithConstraints(c ResponseConstraints) json.RawMessage {
	return MergeSchemaWithConstraintsFor(c, false)
}

// MergeSchemaWithConstraintsFor returns the merge schema narrowed by the given
// constraints (priority bounds + allowed overall_correctness values).
func MergeSchemaWithConstraintsFor(c ResponseConstraints, disableSuggestions bool) json.RawMessage {
	min, max := 0, 3
	if c.MinPriority != nil {
		min = *c.MinPriority
	}
	if c.MaxPriority != nil {
		max = *c.MaxPriority
	}
	root := buildFindingsSchemaDefinition(min, max, c.AllowedCorrectness, true, !disableSuggestions)
	if !disableSuggestions {
		limitFindingSuggestionItems(root, 1)
	}
	return mustMarshalCleanSchema(extendFindingsForMergeProvenance(extendFindingsForVerification(root)))
}

func limitFindingSuggestionItems(root map[string]any, maxItems int) {
	properties, ok := root["properties"].(map[string]any)
	if !ok {
		return
	}
	findings, ok := properties["findings"].(map[string]any)
	if !ok {
		return
	}
	items, ok := findings["items"].(map[string]any)
	if !ok {
		return
	}
	findingProperties, ok := items["properties"].(map[string]any)
	if !ok {
		return
	}
	suggestions, ok := findingProperties["suggestions"].(map[string]any)
	if !ok {
		return
	}
	suggestions["maxItems"] = maxItems
}

func MergeExamplePromptSnippet() string {
	return MergeExamplePromptSnippetFor(false)
}

func MergeExamplePromptSnippetFor(disableSuggestions bool) string {
	if disableSuggestions {
		return mustIndentJSON(mustMarshalJSON(exampleFromSchema(mergeWithoutSuggestionsSchemaDefinition)))
	}
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(mergeSchemaDefinition)))
}
