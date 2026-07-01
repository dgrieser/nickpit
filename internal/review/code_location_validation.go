package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/retrieval"
)

type codeLocationValidationIssue struct {
	Field  string
	Reason string
}

type codeLocationValidationQuery struct {
	path string
	code string
}

func (e *Engine) responseCodeLocationValidator(repoRoot string) func(*llm.ReviewResponse) *llm.InvalidResponseError {
	if e == nil || e.retrieval == nil || strings.TrimSpace(repoRoot) == "" {
		return nil
	}
	return func(resp *llm.ReviewResponse) *llm.InvalidResponseError {
		return e.validateResponseCodeLocations(context.Background(), repoRoot, resp)
	}
}

func (e *Engine) validateResponseCodeLocations(ctx context.Context, repoRoot string, resp *llm.ReviewResponse) *llm.InvalidResponseError {
	if e == nil || e.retrieval == nil || strings.TrimSpace(repoRoot) == "" || resp == nil {
		return nil
	}
	validator := codeLocationValidator{
		ctx:       ctx,
		repoRoot:  repoRoot,
		retrieval: e.retrieval,
		cache:     make(map[codeLocationValidationQuery]*retrieval.FindLinesResult),
	}
	var issues []codeLocationValidationIssue
	for i, finding := range resp.Findings {
		prefix := fmt.Sprintf("findings[%d]", i)
		issues = validator.validateLocation(issues, prefix+".code_location", finding.CodeLocation)
		issues = validator.validateSuggestions(issues, prefix+".suggestions", finding.Suggestions)
		if finding.Finalization != nil {
			issues = validator.validateSuggestions(issues, prefix+".finalization.suggestions", finding.Finalization.Suggestions)
		}
		if finding.Summarization != nil {
			issues = validator.validateSuggestions(issues, prefix+".summarization.suggestions", finding.Summarization.Suggestions)
		}
	}
	if len(issues) == 0 {
		return nil
	}
	fields := make([]string, 0, len(issues))
	reasons := make([]string, 0, len(issues))
	for _, issue := range issues {
		fields = append(fields, issue.Field)
		reasons = append(reasons, issue.Field+": "+issue.Reason)
	}
	return &llm.InvalidResponseError{
		RawContent:            resp.RawResponse,
		Reason:                "code_location_validation_failed: " + strings.Join(reasons, "; "),
		MissingFields:         fields,
		ReasoningEffort:       resp.ReasoningEffort,
		ValidationFailure:     true,
		RetryGuidanceTemplate: "code_location_validation_retry_guidance.tmpl",
		RetryGuidanceData: struct {
			Issues []codeLocationValidationIssue
		}{
			Issues: issues,
		},
	}
}

func codeLocationValidatorForReviewer(e *Engine, agent agentSpec, repoRoot string) func(*llm.ReviewResponse) *llm.InvalidResponseError {
	if !agent.hasTools {
		return nil
	}
	return e.responseCodeLocationValidator(repoRoot)
}

type codeLocationValidator struct {
	ctx       context.Context
	repoRoot  string
	retrieval retrieval.Engine
	cache     map[codeLocationValidationQuery]*retrieval.FindLinesResult
}

func (v *codeLocationValidator) validateSuggestions(issues []codeLocationValidationIssue, prefix string, suggestions []model.Suggestion) []codeLocationValidationIssue {
	for i, suggestion := range suggestions {
		issues = v.validateLocation(issues, fmt.Sprintf("%s[%d].code_location", prefix, i), suggestion.CodeLocation)
	}
	return issues
}

func (v *codeLocationValidator) validateLocation(issues []codeLocationValidationIssue, field string, loc model.CodeLocation) []codeLocationValidationIssue {
	if strings.TrimSpace(loc.FilePath) == "" {
		return append(issues, codeLocationValidationIssue{Field: field, Reason: "missing file_path"})
	}
	if strings.TrimSpace(loc.Content) == "" {
		return append(issues, codeLocationValidationIssue{Field: field, Reason: "missing content"})
	}
	if loc.LineRange.Start <= 0 || loc.LineRange.End < loc.LineRange.Start || loc.LineRange.Count != loc.LineRange.End-loc.LineRange.Start+1 {
		return append(issues, codeLocationValidationIssue{Field: field, Reason: "invalid line_range"})
	}
	result, err := v.findLines(loc.FilePath, loc.Content)
	if err != nil {
		return append(issues, codeLocationValidationIssue{Field: field, Reason: err.Error()})
	}
	if result == nil || len(result.Matches) == 0 {
		return append(issues, codeLocationValidationIssue{Field: field, Reason: "content was not found by find_lines in the referenced file"})
	}
	for _, match := range result.Matches {
		matchLoc := match.CodeLocation
		if matchLoc.FilePath == loc.FilePath &&
			matchLoc.LineRange.Start == loc.LineRange.Start &&
			matchLoc.LineRange.End == loc.LineRange.End &&
			matchLoc.LineRange.Count == loc.LineRange.Count {
			if matchLoc.Content != loc.Content {
				return append(issues, codeLocationValidationIssue{Field: field, Reason: "content differs from the find_lines match at the exact line range"})
			}
			if loc.Language != "" && matchLoc.Language != "" && loc.Language != matchLoc.Language {
				return append(issues, codeLocationValidationIssue{Field: field, Reason: "language differs from the find_lines match"})
			}
			return issues
		}
	}
	return append(issues, codeLocationValidationIssue{
		Field:  field,
		Reason: fmt.Sprintf("content was found by find_lines, but not at %s:%d-%d", loc.FilePath, loc.LineRange.Start, loc.LineRange.End),
	})
}

func (v *codeLocationValidator) findLines(path, code string) (*retrieval.FindLinesResult, error) {
	query := codeLocationValidationQuery{path: path, code: code}
	if result, ok := v.cache[query]; ok {
		return result, nil
	}
	result, err := v.retrieval.FindLines(v.ctx, v.repoRoot, path, code)
	if err != nil {
		return nil, fmt.Errorf("find_lines failed: %w", err)
	}
	v.cache[query] = result
	return result, nil
}

func composeResponseValidators(validators ...func(*llm.ReviewResponse) *llm.InvalidResponseError) func(*llm.ReviewResponse) *llm.InvalidResponseError {
	nonNil := make([]func(*llm.ReviewResponse) *llm.InvalidResponseError, 0, len(validators))
	for _, validator := range validators {
		if validator != nil {
			nonNil = append(nonNil, validator)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	return func(resp *llm.ReviewResponse) *llm.InvalidResponseError {
		for _, validator := range nonNil {
			if invalid := validator(resp); invalid != nil {
				return invalid
			}
		}
		return nil
	}
}
