package llm

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestParseSummarizeResponseAcceptsSummarization(t *testing.T) {
	content := `{"findings":[{"id":"11111111-1111-4111-8111-111111111111","summarization":{"body":"Short body.\nSecond line."}}]}`
	resp, err := parseReviewResponse(content, SchemaKindSummarize, ResponseConstraints{})
	if err != nil {
		t.Fatalf("parseReviewResponse: %v", err)
	}
	if len(resp.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(resp.Findings))
	}
	if resp.Findings[0].Summarization == nil {
		t.Fatal("summarization nil")
	}
	if resp.Findings[0].Summarization.Body != "Short body.\nSecond line." {
		t.Fatalf("summarization.body = %q", resp.Findings[0].Summarization.Body)
	}
}

func TestParseSummarizeResponseRequiresBody(t *testing.T) {
	content := `{"findings":[{"id":"11111111-1111-4111-8111-111111111111","summarization":{}}]}`
	_, err := parseReviewResponse(content, SchemaKindSummarize, ResponseConstraints{})
	var invalid *InvalidResponseError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want InvalidResponseError", err)
	}
	found := slices.Contains(invalid.MissingFields, "findings[0].summarization.body")
	if !found {
		t.Fatalf("missing fields = %v, want findings[0].summarization.body", invalid.MissingFields)
	}
}

func TestParseSummarizeResponseDoesNotRequireOverallOrPriority(t *testing.T) {
	// Minimal output: no overall_*, priority, verification, or finalization.
	content := `{"findings":[{"id":"11111111-1111-4111-8111-111111111111","summarization":{"body":"x"}}]}`
	if _, err := parseReviewResponse(content, SchemaKindSummarize, ResponseConstraints{}); err != nil {
		t.Fatalf("parseReviewResponse rejected minimal summarize output: %v", err)
	}
}

func TestSummarizeSchemaShape(t *testing.T) {
	if !schemaContainsKey(SummarizeSchema, "summarization") {
		t.Fatalf("SummarizeSchema missing summarization: %s", SummarizeSchema)
	}
	if !schemaContainsKey(SummarizeSchema, "body") {
		t.Fatalf("SummarizeSchema missing body: %s", SummarizeSchema)
	}
	if !schemaContainsKey(SummarizeSchema, "overall_explanation") {
		t.Fatalf("SummarizeSchema missing overall_explanation: %s", SummarizeSchema)
	}
	snippet := SummarizeExamplePromptSnippet()
	if !strings.Contains(snippet, "summarization") || !strings.Contains(snippet, "body") {
		t.Fatalf("SummarizeExamplePromptSnippet missing summarization/body: %s", snippet)
	}
	if !strings.Contains(snippet, "overall_explanation") {
		t.Fatalf("SummarizeExamplePromptSnippet missing overall_explanation: %s", snippet)
	}
}

func TestParseSummarizeResponseCapturesOverallExplanation(t *testing.T) {
	content := `{"findings":[{"id":"11111111-1111-4111-8111-111111111111","summarization":{"body":"Short body."}}],"overall_explanation":"Short overall summary."}`
	resp, err := parseReviewResponse(content, SchemaKindSummarize, ResponseConstraints{})
	if err != nil {
		t.Fatalf("parseReviewResponse: %v", err)
	}
	if resp.OverallExplanation != "Short overall summary." {
		t.Fatalf("resp.OverallExplanation = %q, want shortened overall", resp.OverallExplanation)
	}
}
