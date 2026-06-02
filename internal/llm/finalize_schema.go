package llm

import "slices"

import "encoding/json"

// finalizeSchemaDefinition extends the findings schema with required
// per-finding verifier input and finalizer output. The finalizer preserves
// `verification` and records its own decision in `finalization`.
var finalizeSchemaDefinition = buildFinalizeSchemaDefinition()

// confidence_score is intentionally omitted: it is computed deterministically
// in code (see applyWeightedConfidence in internal/review/finalizer.go) rather
// than emitted by the LLM.
var finalizationSchemaDefinition = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"title":    map[string]any{"type": "string", "examples": []any{"Example final title"}},
		"body":     map[string]any{"type": "string", "examples": []any{"Example final explanation."}},
		"priority": map[string]any{"type": "integer", "minimum": 0, "maximum": 3, "examples": []any{1}},
		"remarks":  map[string]any{"type": "string", "examples": []any{"Example explanation of the final decision."}},
	},
	"required": []string{"title", "body", "priority", "remarks"},
}

func buildFinalizeSchemaDefinition() map[string]any {
	return extendFindingsForFinalize(deepCopySchema(findingsWithIDSchemaDefinition).(map[string]any))
}

func extendFindingsForVerification(root map[string]any) map[string]any {
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
	itemProps["verification"] = deepCopySchema(verifySchemaDefinition)
	required, ok := items["required"].([]string)
	if !ok {
		panic("llm: findings schema findings.items.required is not []string")
	}
	items["required"] = appendRequired(required, "verification")
	return root
}

func extendFindingsForFinalize(root map[string]any) map[string]any {
	root = extendFindingsForVerification(root)
	properties := root["properties"].(map[string]any)
	findings := properties["findings"].(map[string]any)
	items := findings["items"].(map[string]any)
	itemProps := items["properties"].(map[string]any)
	itemProps["finalization"] = deepCopySchema(finalizationSchemaDefinition)
	required, ok := items["required"].([]string)
	if !ok {
		panic("llm: findings schema findings.items.required is not []string")
	}
	items["required"] = appendRequired(required, "finalization")
	return root
}

func appendRequired(required []string, fields ...string) []string {
	out := append([]string{}, required...)
	for _, field := range fields {
		found := slices.Contains(out, field)
		if !found {
			out = append(out, field)
		}
	}
	return out
}

func deepCopySchema(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = deepCopySchema(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = deepCopySchema(value)
		}
		return out
	case []string:
		out := make([]string, len(typed))
		copy(out, typed)
		return out
	default:
		return typed
	}
}

var FinalizeSchema = mustMarshalCleanSchema(finalizeSchemaDefinition)

// FinalizeSchemaWithConstraints returns the finalize schema narrowed by the
// given constraints (priority bounds + allowed overall_correctness values).
func FinalizeSchemaWithConstraints(c ResponseConstraints) json.RawMessage {
	min, max := 0, 3
	if c.MinPriority != nil {
		min = *c.MinPriority
	}
	if c.MaxPriority != nil {
		max = *c.MaxPriority
	}
	root := buildFindingsSchemaDefinition(min, max, c.AllowedCorrectness, true)
	return mustMarshalCleanSchema(extendFindingsForFinalize(root))
}

func FinalizeExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(finalizeSchemaDefinition)))
}
