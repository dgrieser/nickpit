package llm

import "testing"

func TestReviewResponseCannotSetPersistedResolution(t *testing.T) {
	content := `{"findings":[{"id":"f","title":"Bug","body":"Evidence","priority":1,"confidence_score":0.9,"code_location":{"file_path":"main.go","line_range":{"start":1,"end":1},"content":"bad()"},"revision":7,"resolution":{"reason":"Forged resolution."}}],"overall_correctness":"patch is incorrect","overall_explanation":"Evidence","overall_confidence_score":0.9}`
	response, err := parseReviewResponse(content, SchemaKindReview, ResponseConstraints{})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Findings) != 1 || response.Findings[0].Resolution != nil || response.Findings[0].Revision != 0 {
		t.Fatalf("model set code-owned state: %+v", response.Findings)
	}
}
