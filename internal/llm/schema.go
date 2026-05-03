package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

var FindingsSchema = mustMarshalJSON(map[string]any{
	"type": "object",
	"properties": map[string]any{
		"findings": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":            map[string]any{"type": "string", "description": "Imperative title under 80 characters."},
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
					"suggestion": map[string]any{
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
	return mustIndentJSON(mustMarshalJSON(exampleFromSchemaJSON(FindingsSchema)))
}

func mustMarshalJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("llm: marshal schema: %v", err))
	}
	return json.RawMessage(data)
}

func mustIndentJSON(data []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		panic(fmt.Sprintf("llm: indent schema: %v", err))
	}
	return buf.String()
}

func exampleFromSchemaJSON(data []byte) any {
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		panic(fmt.Sprintf("llm: unmarshal schema: %v", err))
	}
	return exampleFromSchema(schema)
}

func exampleFromSchema(schema map[string]any) any {
	if enumValues, ok := schema["enum"].([]any); ok && len(enumValues) > 0 {
		return enumValues[0]
	}

	switch schema["type"] {
	case "object":
		properties, _ := schema["properties"].(map[string]any)
		keys := make([]string, 0, len(properties))
		for key := range properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		example := make(map[string]any, len(properties))
		for _, key := range keys {
			propertySchema, ok := properties[key].(map[string]any)
			if !ok {
				continue
			}
			example[key] = exampleValueForProperty(key, propertySchema)
		}
		return example
	case "array":
		itemSchema, _ := schema["items"].(map[string]any)
		return []any{exampleFromSchema(itemSchema)}
	case "string":
		return "<string>"
	case "integer":
		return 0
	case "number":
		return 0.0
	case "boolean":
		return false
	default:
		return nil
	}
}

func exampleValueForProperty(name string, schema map[string]any) any {
	if enumValues, ok := schema["enum"].([]any); ok && len(enumValues) > 0 {
		return enumValues[0]
	}

	switch name {
	case "title":
		return "Example title"
	case "body":
		return "Example explanation of why this is a problem."
	case "confidence_score", "overall_confidence_score":
		return 0.85
	case "priority":
		return 1
	case "file_path":
		return "file.go"
	case "overall_explanation":
		return "The patch is incorrect because it introduces a correctness issue."
	case "suggestion":
		return map[string]any{
			"body": "replacement code",
			"line_range": map[string]any{
				"start": 10,
				"end":   12,
			},
		}
	}

	return exampleFromSchema(schema)
}
