package llm

// finalizeSchemaDefinition extends the findings schema with a per-finding
// `verification` object whose shape matches the verifier output. The finalizer
// agent uses this object to record what it did to each surviving finding
// (decision + remarks).
var finalizeSchemaDefinition = buildFinalizeSchemaDefinition()

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
	required, _ := items["required"].([]string)
	items["required"] = append(append([]string{}, required...), "verification")
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
