package llm

import (
	"github.com/dgrieser/nickpit/internal/model"
)

// verifyGateSchema builds the schema for the required gate field. The verifier
// must name the decision-order gate that decided; forcing the choice makes the
// model walk the gate list instead of free-judging whether the issue is real.
//
// Eligibility is resolved before verify by private descriptive classification
// and deterministic diff-scope validation, so every gate left here is an
// evidence judgement.
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
			"description": "Decided by the VERDICT DECISION ORDER: apply the gates in order, the first gate that applies decides. " +
				"confirmed: the confirm gate applied. " +
				"refuted: a refuting gate applied. " +
				"unverified: no gate can prove or refute the claim.",
			"examples": []any{"confirmed"},
		},
		"gate": verifyGateSchema([]any{
			model.GateStyleguideContradiction,
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

var VerifySchema = mustMarshalCleanSchema(verifySchemaDefinition)

func VerifyExamplePromptSnippet() string {
	return mustIndentJSON(mustMarshalJSON(exampleFromSchema(verifySchemaDefinition)))
}
