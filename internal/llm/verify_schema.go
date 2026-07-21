package llm

import (
	"maps"

	"github.com/dgrieser/nickpit/internal/model"
)

// verifyGateSchema builds the schema for the required gate field. The verifier
// must name the decision-order gate that decided; forcing the choice makes the
// model walk the gate list instead of free-judging whether the issue is real.
func verifyGateSchema(gates []any) map[string]any {
	return map[string]any{
		"type": "string",
		"enum": gates,
		"description": "The VERDICT DECISION ORDER gate that decided: walk the gates in order and name the first one that applied. " +
			"The gate dictates the verdict: confirm gate confirms, unverified gate leaves unverified, every other gate refutes.",
		"examples": []any{model.GateConfirm},
	}
}

var verifySchemaDefinition = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"id": map[string]any{"type": "string", "examples": []any{"<uuid-v4>"}},
		"verdict": map[string]any{
			"type": "string",
			"enum": []any{"confirmed", "refuted", "unverified"},
			"description": "Decided by the VERDICT DECISION ORDER: apply the gates in order, the first gate that applies decides — never judge by whether the issue is real. " +
				"confirmed: the confirm gate applied. " +
				"refuted: a refuting gate applied, even when the claim is technically real. " +
				"unverified: no gate can prove or refute the claim.",
			"examples": []any{"confirmed"},
		},
		"gate": verifyGateSchema([]any{
			model.GateNonFinding,
			model.GateStyleguideContradiction,
			model.GateCompileError,
			model.GateConfirm,
			model.GateRefute,
			model.GateUnverified,
		}),
		"priority":         map[string]any{"type": "integer", "minimum": 0, "maximum": 3, "examples": []any{1}},
		"confidence_score": map[string]any{"type": "number", "minimum": 0, "maximum": 1, "examples": []any{0.85}},
		"remarks":          map[string]any{"type": "string", "examples": []any{"Example explanation of why this is a problem."}},
	},
	"required": []string{"id", "verdict", "gate", "priority", "confidence_score", "remarks"},
}

var scopedVerifySchemaDefinition = func() map[string]any {
	properties := map[string]any{}
	maps.Copy(properties, verifySchemaDefinition["properties"].(map[string]any))
	properties["gate"] = verifyGateSchema([]any{
		model.GateNonFinding,
		model.GateDiffScope,
		model.GateStyleguideContradiction,
		model.GateCompileError,
		model.GateConfirm,
		model.GateRefute,
		model.GateUnverified,
	})
	properties["replacement_code_location"] = map[string]any{
		"anyOf": []any{
			codeLocationSchemaDefinition(),
			map[string]any{"type": "null"},
		},
		"examples": []any{nil},
	}
	required := append([]string{}, verifySchemaDefinition["required"].([]string)...)
	required = append(required, "replacement_code_location")
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}()

var VerifySchema = mustMarshalCleanSchema(verifySchemaDefinition)
var ScopedVerifySchema = mustMarshalCleanSchema(scopedVerifySchemaDefinition)

func VerifyExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(verifySchemaDefinition)))
}

func ScopedVerifyExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(scopedVerifySchemaDefinition)))
}
