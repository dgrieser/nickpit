package review

import (
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/model"
)

type testingDuplicateFileGuidance struct {
	File           string
	ExistingTitles []string
	ResponseTitles []string
}

type testingDuplicateFileDrop struct {
	File  string
	Title string
}

func validateTestingDuplicateFileResponse(existing []model.Finding, resp *llm.ReviewResponse) *llm.InvalidResponseError {
	if resp == nil {
		return nil
	}
	duplicates := testingDuplicateFileDiagnostics(existing, resp.Findings)
	if len(duplicates) == 0 {
		return nil
	}
	files := make([]string, 0, len(duplicates))
	hasExisting := false
	for _, duplicate := range duplicates {
		files = append(files, duplicate.File)
		if len(duplicate.ExistingTitles) > 0 {
			hasExisting = true
		}
	}
	return &llm.InvalidResponseError{
		RawContent:            resp.RawResponse,
		Reason:                fmt.Sprintf("testing_duplicate_file_findings files=%s", strings.Join(files, ", ")),
		ReasoningEffort:       resp.ReasoningEffort,
		ValidationFailure:     true,
		RetryGuidanceTemplate: "testing_duplicate_file_retry_guidance.tmpl",
		RetryGuidanceData: struct {
			Files       []testingDuplicateFileGuidance
			HasExisting bool
		}{
			Files:       duplicates,
			HasExisting: hasExisting,
		},
	}
}

func testingDuplicateFileDiagnostics(existing, response []model.Finding) []testingDuplicateFileGuidance {
	existingByFile := make(map[string][]string)
	for _, finding := range existing {
		file := testingFindingFileKey(finding)
		if file == "" {
			continue
		}
		existingByFile[file] = append(existingByFile[file], testingFindingTitle(finding))
	}

	appendable := appendableFindings(existing, response)
	responseByFile := make(map[string][]string)
	fileOrder := make([]string, 0)
	for _, finding := range appendable {
		file := testingFindingFileKey(finding)
		if file == "" {
			continue
		}
		if _, ok := responseByFile[file]; !ok {
			fileOrder = append(fileOrder, file)
		}
		responseByFile[file] = append(responseByFile[file], testingFindingTitle(finding))
	}

	out := make([]testingDuplicateFileGuidance, 0)
	for _, file := range fileOrder {
		responseTitles := responseByFile[file]
		existingTitles := existingByFile[file]
		if len(existingTitles) == 0 && len(responseTitles) < 2 {
			continue
		}
		out = append(out, testingDuplicateFileGuidance{
			File:           file,
			ExistingTitles: existingTitles,
			ResponseTitles: responseTitles,
		})
	}
	return out
}

func pruneTestingDuplicateFileFindings(existing, candidates []model.Finding) ([]model.Finding, []testingDuplicateFileDrop) {
	seenFiles := make(map[string]struct{})
	seenIDTitles := make(map[string]struct{})
	seenTitleLocations := make(map[string]struct{})
	for _, finding := range existing {
		if file := testingFindingFileKey(finding); file != "" {
			seenFiles[file] = struct{}{}
		}
		recordFindingKeys(seenIDTitles, seenTitleLocations, finding)
	}

	kept := make([]model.Finding, 0, len(candidates))
	dropped := make([]testingDuplicateFileDrop, 0)
	for _, finding := range candidates {
		if testingFindingAlreadySeen(seenIDTitles, seenTitleLocations, finding) {
			continue
		}
		file := testingFindingFileKey(finding)
		if file == "" {
			kept = append(kept, finding)
			recordFindingKeys(seenIDTitles, seenTitleLocations, finding)
			continue
		}
		if _, exists := seenFiles[file]; exists {
			dropped = append(dropped, testingDuplicateFileDrop{
				File:  file,
				Title: testingFindingTitle(finding),
			})
			recordFindingKeys(seenIDTitles, seenTitleLocations, finding)
			continue
		}
		seenFiles[file] = struct{}{}
		kept = append(kept, finding)
		recordFindingKeys(seenIDTitles, seenTitleLocations, finding)
	}
	return kept, dropped
}

// appendableFindings returns the candidates that appendNewFindings would
// actually add to the session (i.e. not duplicates of existing findings or of
// an earlier candidate), preserving response order.
func appendableFindings(existing, candidates []model.Finding) []model.Finding {
	seenIDTitles := make(map[string]struct{})
	seenTitleLocations := make(map[string]struct{})
	for _, finding := range existing {
		recordFindingKeys(seenIDTitles, seenTitleLocations, finding)
	}
	out := make([]model.Finding, 0, len(candidates))
	for _, finding := range candidates {
		if testingFindingAlreadySeen(seenIDTitles, seenTitleLocations, finding) {
			continue
		}
		out = append(out, finding)
		recordFindingKeys(seenIDTitles, seenTitleLocations, finding)
	}
	return out
}

func testingFindingAlreadySeen(seenIDTitles, seenTitleLocations map[string]struct{}, finding model.Finding) bool {
	idTitleKey, titleLocationKey := findingDedupKeys(finding)
	if idTitleKey != "" {
		if _, ok := seenIDTitles[idTitleKey]; ok {
			return true
		}
	}
	if titleLocationKey != "" {
		if _, ok := seenTitleLocations[titleLocationKey]; ok {
			return true
		}
	}
	return false
}

func enforceTestingDuplicateFileResponse(agentName string, existing []model.Finding, resp *llm.ReviewResponse) string {
	if resp == nil {
		return ""
	}
	kept, dropped := pruneTestingDuplicateFileFindings(existing, resp.Findings)
	if len(kept) != len(resp.Findings) {
		resp.Findings = kept
	}
	if len(dropped) == 0 {
		return ""
	}
	return testingDuplicateFileDropMessage(agentName, dropped)
}

func testingDuplicateFileDropMessage(agentName string, dropped []testingDuplicateFileDrop) string {
	if len(dropped) == 0 {
		return ""
	}
	parts := make([]string, 0, len(dropped))
	for _, drop := range dropped {
		parts = append(parts, fmt.Sprintf("%s %q", drop.File, drop.Title))
	}
	return fmt.Sprintf("%s duplicate-file Testing findings dropped after retry exhaustion: %s", agentName, strings.Join(parts, "; "))
}

func testingFindingFileKey(finding model.Finding) string {
	return normalizeReviewPath(finding.CodeLocation.FilePath)
}

func testingFindingTitle(finding model.Finding) string {
	title := strings.TrimSpace(finding.Title)
	if title == "" {
		return "(untitled)"
	}
	return title
}
