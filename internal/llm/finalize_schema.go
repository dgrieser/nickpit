package llm

// finalizeSchemaDefinition extends the findings schema with per-finding
// verifier input and finalizer output. The finalizer preserves `verification`
// and records its own decision in `finalization`.
var finalizeSchemaDefinition = buildFinalizeSchemaDefinition()

var finalizationSchemaDefinition = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"title":            map[string]any{"type": "string", "examples": []any{"Example final title"}},
		"body":             map[string]any{"type": "string", "examples": []any{"Example final explanation."}},
		"priority":         map[string]any{"type": "integer", "minimum": 0, "maximum": 3, "examples": []any{1}},
		"confidence_score": map[string]any{"type": "number", "minimum": 0, "maximum": 1, "examples": []any{0.85}},
		"remarks":          map[string]any{"type": "string", "examples": []any{"Example explanation of the final decision."}},
	},
	"required": []string{"title", "body", "priority", "confidence_score", "remarks"},
}

func buildFinalizeSchemaDefinition() map[string]any {
	root := deepCopySchema(findingsSchemaDefinition).(map[string]any)
	properties, ok := root["properties"].(map[string]any)
	if !ok {
		panic("llm: findingsSchemaDefinition missing properties")
	}
	findings, ok := properties["findings"].(map[string]any)
	if !ok {
		panic("llm: findingsSchemaDefinition missing findings property")
	}
	items, ok := findings["items"].(map[string]any)
	if !ok {
		panic("llm: findingsSchemaDefinition findings.items malformed")
	}
	itemProps, ok := items["properties"].(map[string]any)
	if !ok {
		panic("llm: findingsSchemaDefinition findings.items.properties malformed")
	}
	itemProps["verification"] = deepCopySchema(verifySchemaDefinition)
	itemProps["finalization"] = deepCopySchema(finalizationSchemaDefinition)
	required, _ := items["required"].([]string)
	items["required"] = append(append([]string{}, required...), "verification", "finalization")
	return root
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

func FinalizeExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(finalizeSchemaDefinition)))
}
