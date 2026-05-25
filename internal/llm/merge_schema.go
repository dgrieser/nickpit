package llm

import "encoding/json"

var mergeSchemaDefinition = buildMergeSchemaDefinition()

func buildMergeSchemaDefinition() map[string]any {
	return extendFindingsForVerification(deepCopySchema(findingsWithIDSchemaDefinition).(map[string]any))
}

var MergeSchema = mustMarshalCleanSchema(mergeSchemaDefinition)

// MergeSchemaWithConstraints returns the merge schema narrowed by the given
// constraints (priority bounds + allowed overall_correctness values).
func MergeSchemaWithConstraints(c ResponseConstraints) json.RawMessage {
	min, max := 0, 3
	if c.MinPriority != nil {
		min = *c.MinPriority
	}
	if c.MaxPriority != nil {
		max = *c.MaxPriority
	}
	root := buildFindingsSchemaDefinition(min, max, c.AllowedCorrectness, true)
	return mustMarshalCleanSchema(extendFindingsForVerification(root))
}

func MergeExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(mergeSchemaDefinition)))
}
