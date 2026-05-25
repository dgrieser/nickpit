package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

func buildFindingsSchemaDefinition(minPriority, maxPriority int, allowedCorrectness []string, requireID bool) map[string]any {
	if len(allowedCorrectness) == 0 {
		allowedCorrectness = []string{"patch is correct", "patch is incorrect"}
	}
	findingProperties := map[string]any{
		"title":            map[string]any{"type": "string", "examples": []any{"Example title"}},
		"body":             map[string]any{"type": "string", "examples": []any{"Example explanation of why this is a problem."}},
		"confidence_score": map[string]any{"type": "number", "minimum": 0, "maximum": 1, "examples": []any{0.85}},
		"priority":         map[string]any{"type": "integer", "minimum": minPriority, "maximum": maxPriority, "examples": []any{minPriority}},
		"code_location": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{"type": "string", "examples": []any{"file.go"}},
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
					"body": map[string]any{"type": "string", "examples": []any{"replacement code"}},
					"line_range": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"start": map[string]any{"type": "integer", "examples": []any{10}},
							"end":   map[string]any{"type": "integer", "examples": []any{12}},
						},
						"required": []string{"start", "end"},
					},
				},
				"required": []string{"body", "line_range"},
			},
		},
	}
	requiredFindingFields := []string{"title", "body", "confidence_score", "priority", "code_location"}
	if requireID {
		findingProperties["id"] = map[string]any{"type": "string", "examples": []any{"<uuid-v4>"}}
		requiredFindingFields = append([]string{"id"}, requiredFindingFields...)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"findings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":       "object",
					"properties": findingProperties,
					"required":   requiredFindingFields,
				},
			},
			"overall_correctness":      map[string]any{"type": "string", "enum": allowedCorrectness},
			"overall_explanation":      map[string]any{"type": "string", "examples": []any{"The patch is incorrect because it introduces a correctness issue."}},
			"overall_confidence_score": map[string]any{"type": "number", "minimum": 0, "maximum": 1, "examples": []any{0.85}},
		},
		"required": []string{"findings", "overall_correctness", "overall_explanation", "overall_confidence_score"},
	}
}

var findingsSchemaDefinition = buildFindingsSchemaDefinition(0, 3, nil, false)

var findingsWithIDSchemaDefinition = buildFindingsSchemaDefinition(0, 3, nil, true)

var FindingsSchema = mustMarshalCleanSchema(findingsSchemaDefinition)

// FindingsSchemaWithConstraints returns a findings schema narrowed by the given constraints.
func FindingsSchemaWithConstraints(c ResponseConstraints) json.RawMessage {
	min, max := 0, 3
	if c.MinPriority != nil {
		min = *c.MinPriority
	}
	if c.MaxPriority != nil {
		max = *c.MaxPriority
	}
	return mustMarshalCleanSchema(buildFindingsSchemaDefinition(min, max, c.AllowedCorrectness, false))
}

func FindingsExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(findingsSchemaDefinition)))
}

func mustMarshalJSON(v any) json.RawMessage {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic(fmt.Sprintf("llm: marshal schema: %v", err))
	}
	data := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	return json.RawMessage(data)
}

func mustMarshalCleanSchema(v any) json.RawMessage {
	return mustMarshalJSON(stripSchemaExamples(v))
}

func mustIndentJSON(data []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		panic(fmt.Sprintf("llm: indent schema example: %v", err))
	}
	return buf.String()
}

func stripSchemaExamples(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(typed))
		for key, value := range typed {
			if key == "examples" {
				continue
			}
			cleaned[key] = stripSchemaExamples(value)
		}
		return cleaned
	case []any:
		cleaned := make([]any, len(typed))
		for i, value := range typed {
			cleaned[i] = stripSchemaExamples(value)
		}
		return cleaned
	case []string:
		cleaned := make([]string, len(typed))
		copy(cleaned, typed)
		return cleaned
	default:
		return typed
	}
}

func exampleFromSchema(schema map[string]any) any {
	if examples, ok := schema["examples"].([]any); ok && len(examples) > 0 {
		return examples[0]
	}
	if enumValues, ok := schema["enum"].([]any); ok && len(enumValues) > 0 {
		return enumValues[0]
	}
	if enumValues, ok := schema["enum"].([]string); ok && len(enumValues) > 0 {
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
			example[key] = exampleFromSchema(propertySchema)
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

func schemaContainsKey(data []byte, key string) bool {
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return false
	}
	return containsMapKey(decoded, key)
}

func containsMapKey(v any, key string) bool {
	switch typed := v.(type) {
	case map[string]any:
		if _, ok := typed[key]; ok {
			return true
		}
		for _, value := range typed {
			if containsMapKey(value, key) {
				return true
			}
		}
	case []any:
		for _, value := range typed {
			if containsMapKey(value, key) {
				return true
			}
		}
	}
	return false
}
