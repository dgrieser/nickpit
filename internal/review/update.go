package review

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/tokenestimate"
)

type ReviewUpdateStatus string

const (
	ReviewUpdateScheduled   ReviewUpdateStatus = "scheduled"
	ReviewUpdateQueueFailed ReviewUpdateStatus = "queue_failed"
	ReviewUpdateError       ReviewUpdateStatus = "error"
)

type ReviewUpdateToolResult struct {
	Status  ReviewUpdateStatus `json:"status"`
	Message string             `json:"message,omitempty"`
}

type ReviewUpdateSignal struct {
	FindingIDs []string `json:"finding_ids"`
	Reason     string   `json:"reason"`
}

func (s ReviewUpdateSignal) Validate(result *model.ReviewResult) error {
	if strings.TrimSpace(s.Reason) == "" {
		return fmt.Errorf("provide a concrete reason for disputing the findings or review")
	}
	seen := map[string]bool{}
	for _, id := range s.FindingIDs {
		if seen[id] {
			return fmt.Errorf("duplicate finding ID %q", id)
		}
		seen[id] = true
		found := false
		for _, f := range result.Findings {
			if f.ID == id {
				if f.Resolution != nil {
					return fmt.Errorf("finding %q is already resolved", id)
				}
				found = true
			}
		}
		if !found {
			return fmt.Errorf("unknown finding ID %q", id)
		}
	}
	return nil
}

type ReviewUpdateOutcome struct {
	TokensUsed  model.TokenUsage     `json:"-"`
	Checks      []FindingUpdateCheck `json:"checks"`
	ReviewCheck *ReviewUpdateCheck   `json:"review_check,omitempty"`
	// BlockedBy lists active findings whose priority floor pins the verdict to
	// "patch is incorrect"; an overall correction cannot change it while they
	// remain active, so it is reported instead of published.
	BlockedBy          []string        `json:"blocked_by,omitempty"`
	Changed            []model.Finding `json:"changed_findings"`
	OverallCorrectness string          `json:"overall_correctness"`
	OverallExplanation string          `json:"overall_explanation"`
}

type FindingUpdateCheck struct {
	ID         string `json:"id"`
	Action     string `json:"action"`
	Basis      string `json:"basis,omitempty"`
	Reason     string `json:"reason"`
	Resolution string `json:"resolution,omitempty"`
}

type FindingUpdateReport struct {
	model.AgentRun
	Checks      []FindingUpdateCheck
	ReviewCheck *ReviewUpdateCheck
}

// ReviewUpdateCheck assesses evidence, not which agents or SCM writes to run.
type ReviewUpdateCheck struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

func (r FindingUpdateReport) ReviewCorrectionWarranted() bool {
	return r.ReviewCheck != nil && r.ReviewCheck.Action == "correction_warranted"
}

type UpdateFindingsRequest struct {
	DiscussRequest
	Signal ReviewUpdateSignal
}

// Resolution bases for a resolved decision. Author acceptance cannot waive a
// blocking finding: only P2/P3 findings may be resolved as accepted.
const (
	resolutionBasisFixed    = "fixed"
	resolutionBasisInvalid  = "invalid"
	resolutionBasisAccepted = "accepted"
	minAcceptedPriority     = 2
)

// findingUpdateDecision keeps the free-form evidence (reason) separate from the
// published one-sentence resolution so the length limit on the latter never
// shapes which action the model picks.
type findingUpdateDecision struct {
	ID         string         `json:"id"`
	Action     string         `json:"action"`
	Reason     string         `json:"reason"`
	Basis      string         `json:"basis,omitempty"`
	Resolution string         `json:"resolution,omitempty"`
	Finding    *model.Finding `json:"finding,omitempty"`
}

// UpdateFindings checks the selected batch. No SCM writes or revision/history
// decisions occur here; unchanged findings retain their exact original data.
func (e *Engine) UpdateFindings(ctx context.Context, req UpdateFindingsRequest) (*model.ReviewResult, FindingUpdateReport, error) {
	run := FindingUpdateReport{AgentRun: model.AgentRun{Name: "Update Findings", Role: "update"}}
	if req.Result == nil || req.ReviewCtx == nil {
		return nil, run, fmt.Errorf("update: missing review or context")
	}
	if err := req.Signal.Validate(req.Result); err != nil {
		return nil, run, err
	}
	out, err := req.Result.Clone()
	if err != nil {
		return nil, run, err
	}
	reviewOnly := len(req.Signal.FindingIDs) == 0
	selected := &model.ReviewResult{ReviewID: out.ReviewID}
	for _, id := range req.Signal.FindingIDs {
		for _, f := range out.Findings {
			if f.ID == id {
				selected.Findings = append(selected.Findings, currentFinding(f))
			}
		}
	}
	promptResult := selected
	if reviewOnly {
		promptResult = &model.ReviewResult{ReviewID: out.ReviewID, OverallCorrectness: out.OverallCorrectness,
			OverallExplanation: out.OverallExplanation, OverallConfidenceScore: out.OverallConfidenceScore}
		for _, finding := range out.Findings {
			if finding.Resolution == nil {
				promptResult.Findings = append(promptResult.Findings, currentFinding(finding))
			}
		}
	}
	guides, err := e.styleGuidesFor(req.ReviewCtx)
	if err != nil {
		return nil, run, err
	}
	style, err := e.renderStyleGuideToolchainSnippet("update", guides, len(req.ReviewCtx.ToolchainVersions) > 0, req.ReviewCtx.ProjectContext)
	if err != nil {
		return nil, run, err
	}
	tools := req.Tools
	if tools == nil {
		tools = reviewerToolDefinitions()
	}
	if req.MaxToolCalls < 0 {
		tools = nil
	}
	hasTools := len(tools) > 0
	var instructions string
	if hasTools {
		instructions, err = e.renderToolInstructions(toolInstructionsConfig{agentRole: "update", parallelToolCallGuidance: !req.DisableParallelToolCalls})
		if err != nil {
			return nil, run, err
		}
	}
	systemTemplate, err := e.loadPrompt("agent_update_system_prompt.tmpl")
	if err != nil {
		return nil, run, err
	}
	outputSchemaSnippet := llm.UpdateExamplePromptSnippetFor(req.DisableSuggestions, reviewOnly)
	commonSnippets, err := agentCommonSystemPromptSnippetsForTools("update", outputSchemaSnippet, req.DisableSuggestions, hasTools)
	if err != nil {
		return nil, run, err
	}
	system, err := llm.RenderPrompt(systemTemplate, struct {
		ReviewOnly                 bool
		DisableSuggestions         bool
		HasTools                   bool
		ToolInstructions           string
		StyleGuideToolchainSnippet string
		PrioritySnippet            string
		OutputSchemaSnippet        string
		OutputFormatSnippet        string
	}{
		ReviewOnly:                 reviewOnly,
		DisableSuggestions:         req.DisableSuggestions,
		HasTools:                   hasTools,
		ToolInstructions:           instructions,
		StyleGuideToolchainSnippet: strings.TrimSpace(style),
		PrioritySnippet:            commonSnippets.priority,
		OutputSchemaSnippet:        outputSchemaSnippet,
		OutputFormatSnippet:        commonSnippets.outputFormat,
	})
	if err != nil {
		return nil, run, fmt.Errorf("update: rendering system prompt: %w", err)
	}
	maxContext := e.config.MaxContextTokens
	if maxContext <= 0 {
		maxContext = config.DefaultMaxContextToken
	}
	estimator := tokenestimate.SimpleEstimator{}
	// The rendered system prompt is the real fixed overhead: styleguides, tool
	// instructions, and the output example all ship with it.
	messages := boundDiscussTranscript(req.Messages, discussTranscriptBudget(maxContext, discussFixedOverheadTokens(promptResult, req.DisableSuggestions, system+req.Signal.Reason, estimator)), estimator)
	trimmed, err := e.trimForDiscuss(req.ReviewCtx, promptResult, messages, system+req.Signal.Reason, req.DisableSuggestions, req.DiffFormat)
	if err != nil {
		return nil, run, err
	}
	contextJSON, err := e.buildDiscussContext(trimmed, promptResult, "", req.DisableSuggestions, req.DiffFormat)
	if err != nil {
		return nil, run, err
	}
	schema := llm.UpdateFindingsSchema(req.DisableSuggestions, reviewOnly)
	payload, err := json.Marshal(map[string]any{"context": json.RawMessage(contextJSON), "conversation": messages, "reason": req.Signal.Reason, "head_sha": req.ReviewCtx.DiffHeadSHA})
	if err != nil {
		return nil, run, err
	}
	var decisions []findingUpdateDecision
	allowed := allowedDiffCodeLocations(req.ReviewCtx.DiffHunks, req.ReviewCtx.ChangedFiles)
	validate := func(resp *llm.ReviewResponse) *llm.InvalidResponseError {
		raw := ""
		if resp != nil {
			raw = resp.RawResponse
		}
		parsed, parseErr := parseFindingUpdates(raw, selected, req.DisableSuggestions)
		if parseErr != nil {
			return &llm.InvalidResponseError{RawContent: raw, Reason: parseErr.Error()}
		}
		if reviewOnly {
			var payload struct {
				Review *ReviewUpdateCheck `json:"review"`
			}
			if err := llm.LenientUnmarshal(raw, &payload); err != nil || payload.Review == nil ||
				(payload.Review.Action != "unchanged" && payload.Review.Action != "correction_warranted") || strings.TrimSpace(payload.Review.Reason) == "" {
				return &llm.InvalidResponseError{RawContent: raw, Reason: "return a review assessment with action unchanged or correction_warranted and a concrete evidence-based reason"}
			}
			run.ReviewCheck = payload.Review
		}
		for _, d := range parsed {
			if d.Action == "resolved" && d.Basis == resolutionBasisAccepted {
				// The floor keeps a verifier-confirmed P0 blocking even when its
				// displayed priority was lowered.
				for _, f := range out.Findings {
					if f.ID == d.ID && priorityFloor(f, model.PriorityThresholdRank("")) < minAcceptedPriority {
						return &llm.InvalidResponseError{RawContent: raw, Reason: fmt.Sprintf("finding %s is blocking (P0/P1): author acceptance can only resolve P2 or P3 findings; return unchanged unless the current code fixes it", d.ID)}
					}
				}
			}
			if d.Action != "updated" {
				continue
			}
			var original model.Finding
			for _, f := range selected.Findings {
				if f.ID == d.ID {
					original = f
				}
			}
			if err := e.checkUpdateLocations(ctx, req.RepoRoot, original, d.Finding, allowed); err != nil {
				return &llm.InvalidResponseError{RawContent: raw, Reason: err.Error()}
			}
		}
		decisions = parsed
		return nil
	}
	start := time.Now()
	responseSchema := schema
	if e.config.DisableJSONResponseFormat {
		responseSchema = nil
	}
	e.logProgress(logging.StageChat, logging.StateStart, "Checking finding updates")
	loop, err := e.runAgentLoop(ctx, agentLoopRequest{
		AgentName: run.Name, AgentKind: "update", Progress: e.progressInfo("update", run.Name, ""),
		Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(payload)}},
		Tools:    tools, Schema: responseSchema, SchemaKind: llm.SchemaKindJSON, Model: e.config.Model,
		MaxTokens: e.config.MaxTokens, Temperature: e.config.Temperature, TopP: e.config.TopP,
		TopK: e.config.TopK, MinP: e.config.MinP, PresencePenalty: e.config.PresencePenalty,
		RepetitionPenalty: e.config.RepetitionPenalty, ExtraBody: e.config.ExtraBody,
		ReasoningEffort: e.config.ReasoningEffort, ParallelToolCalls: !req.DisableParallelToolCalls,
		RepoRoot: req.RepoRoot, MaxToolCalls: req.MaxToolCalls, MaxDuplicateToolCalls: req.MaxDuplicateToolCalls,
		MaxOutputRetries: req.MaxOutputRetries, MaxReasoningSeconds: req.MaxReasoningSeconds,
		State: newAgentLoopState(), ValidateResponse: validate, NoToolsSystem: system,
		NoToolsMessages: func(messages []llm.Message) ([]llm.Message, error) {
			return noToolsMessagesFromRendered(system, messages)
		},
	})
	run.TokensUsed, run.ToolCalls, run.RuntimeSeconds = loop.tokensUsed, loop.toolCalls, time.Since(start).Seconds()
	e.logProgress(logging.StageChat, logging.StateDone, "Finding update check finished")
	if err != nil {
		return nil, run, err
	}
	// Fallback completions also require validation before any change is applied.
	if invalid := validate(loop.resp); invalid != nil {
		return nil, run, invalid
	}
	for _, decision := range decisions {
		run.Checks = append(run.Checks, FindingUpdateCheck{ID: decision.ID, Action: decision.Action, Basis: decision.Basis, Reason: decision.Reason, Resolution: decision.Resolution})
		e.logProgress(logging.StageChat, logging.StateDone, updateDecisionLogLine(decision))
		for i := range out.Findings {
			f := &out.Findings[i]
			if f.ID != decision.ID {
				continue
			}
			switch decision.Action {
			case "resolved":
				resolved := currentFinding(*f)
				resolved.Revision = f.Revision
				resolved.Resolution = &model.FindingResolution{Reason: decision.Resolution}
				resolved.Body, resolved.Suggestions = resolved.Resolution.Reason, nil
				*f = resolved
			case "updated":
				updated := currentFinding(*decision.Finding)
				if reflect.DeepEqual(currentFinding(*f), updated) {
					continue
				}
				updated.Revision = f.Revision
				*f = updated
			}
		}
	}
	return out, run, nil
}

func currentFinding(f model.Finding) model.Finding {
	title, body, rank, confidence := reviewmd.FindingDisplay(f)
	suggestions := reviewmd.FindingDisplaySuggestions(f)
	if len(suggestions) == 0 {
		suggestions = nil
	}
	return model.Finding{ID: f.ID, Title: title, Body: body, Priority: &rank, ConfidenceScore: confidence,
		CodeLocation: f.CodeLocation, Suggestions: suggestions, Resolution: f.Resolution}
}

func parseFindingUpdates(raw string, selected *model.ReviewResult, disableSuggestions bool) ([]findingUpdateDecision, error) {
	var payload struct {
		Updates []findingUpdateDecision `json:"updates"`
	}
	if err := llm.LenientUnmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if len(payload.Updates) != len(selected.Findings) {
		return nil, fmt.Errorf("return exactly one update decision per requested finding")
	}
	var rawPayload struct {
		Updates []struct {
			Finding map[string]json.RawMessage `json:"finding"`
		} `json:"updates"`
	}
	if err := llm.LenientUnmarshal(raw, &rawPayload); err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, f := range selected.Findings {
		wanted[f.ID] = true
	}
	for i, d := range payload.Updates {
		if !wanted[d.ID] {
			return nil, fmt.Errorf("unknown or duplicate update ID %q", d.ID)
		}
		delete(wanted, d.ID)
		if strings.TrimSpace(d.Reason) == "" {
			return nil, fmt.Errorf("update %s needs evidence in reason", d.ID)
		}
		if d.Action != "resolved" {
			// basis and resolution describe a resolution only.
			payload.Updates[i].Basis, payload.Updates[i].Resolution = "", ""
		}
		switch d.Action {
		case "unchanged":
		case "resolved":
			switch d.Basis {
			case resolutionBasisFixed, resolutionBasisInvalid, resolutionBasisAccepted:
			default:
				return nil, fmt.Errorf("resolved finding %s needs basis fixed, invalid, or accepted", d.ID)
			}
			// Older outputs put the one-sentence resolution in reason; accept that
			// when it already satisfies the published-sentence rules.
			resolution := strings.TrimSpace(d.Resolution)
			if resolution == "" && validResolutionSentence(d.Reason) {
				resolution = strings.TrimSpace(d.Reason)
			}
			if !validResolutionSentence(resolution) {
				return nil, fmt.Errorf("resolved finding %s needs resolution: one short factual sentence (at most 240 characters), without lists or paragraphs; keep longer evidence in reason", d.ID)
			}
			payload.Updates[i].Resolution = resolution
		case "updated":
			for _, field := range []string{"id", "title", "body", "priority", "confidence_score", "code_location"} {
				value := rawPayload.Updates[i].Finding[field]
				if len(value) == 0 || string(value) == "null" {
					return nil, fmt.Errorf("updated finding requires %s", field)
				}
			}
			f := d.Finding
			if f == nil || f.ID != d.ID || strings.TrimSpace(f.Title) == "" || strings.TrimSpace(f.Body) == "" || f.Priority == nil || *f.Priority < 0 || *f.Priority > 3 || math.IsNaN(f.ConfidenceScore) || f.ConfidenceScore < 0 || f.ConfidenceScore > 1 {
				return nil, fmt.Errorf("updated finding %s needs complete valid finding fields", d.ID)
			}
			if f.Verification != nil || f.Finalization != nil || f.Summarization != nil || f.Resolution != nil || f.Revision != 0 {
				return nil, fmt.Errorf("agent provenance and revision fields are code-owned")
			}
			// Omission preserves existing suggestions; an explicit [] clears them.
			if _, provided := rawPayload.Updates[i].Finding["suggestions"]; !provided && !disableSuggestions {
				for _, original := range selected.Findings {
					if original.ID == d.ID {
						f.Suggestions = original.Suggestions
					}
				}
			}
			if err := validUpdateLocation(f.CodeLocation); err != nil {
				return nil, err
			}
			for _, s := range f.Suggestions {
				if strings.TrimSpace(s.Body) == "" {
					return nil, fmt.Errorf("empty suggestion")
				}
				if err := validUpdateLocation(s.CodeLocation); err != nil {
					return nil, err
				}
			}
			if disableSuggestions {
				f.Suggestions = nil
			}
		default:
			return nil, fmt.Errorf("invalid update action %q", d.Action)
		}
	}
	return payload.Updates, nil
}

func (e *Engine) checkUpdateLocations(ctx context.Context, root string, original model.Finding, updated *model.Finding, allowed []model.CodeLocation) error {
	check := func(loc *model.CodeLocation, old model.CodeLocation) error {
		if *loc == old || codeLocationMatchesAllowedEvidence(*loc, allowed) {
			return nil
		}
		repair := e.responseCodeLocationRepairer(root, false, nil)
		if repair == nil {
			return fmt.Errorf("changed location must be supported by the current diff or a readable checkout")
		}
		response := &llm.ReviewResponse{Findings: []model.Finding{{CodeLocation: *loc}}}
		result := repair(ctx, response)
		if len(result.RetryFields) > 0 {
			return fmt.Errorf("changed location is not supported by current code: %s", strings.Join(result.RetryFields, ", "))
		}
		*loc = response.Findings[0].CodeLocation
		return nil
	}
	if err := check(&updated.CodeLocation, original.CodeLocation); err != nil {
		return err
	}
	for i := range updated.Suggestions {
		var previous model.CodeLocation
		if i < len(original.Suggestions) {
			previous = original.Suggestions[i].CodeLocation
		}
		if err := check(&updated.Suggestions[i].CodeLocation, previous); err != nil {
			return err
		}
	}
	return nil
}

func validUpdateLocation(loc model.CodeLocation) error {
	p := loc.FilePath
	if p == "" || path.IsAbs(p) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") || strings.ContainsAny(p, "\\\x00\r\n") || loc.LineRange.Start < 1 || loc.LineRange.End < loc.LineRange.Start || strings.TrimSpace(loc.Content) == "" {
		return fmt.Errorf("updated code locations must contain a repo-relative path, valid range, and exact code")
	}
	return nil
}

// updateDecisionLogLine records the decision in the run log; without it a
// reply's "Review check complete." is the only trace of what the model chose.
func updateDecisionLogLine(d findingUpdateDecision) string {
	action := d.Action
	if d.Basis != "" {
		action += "/" + d.Basis
	}
	reason := strings.Join(strings.Fields(d.Reason), " ")
	if runes := []rune(reason); len(runes) > 300 {
		reason = string(runes[:300]) + "…"
	}
	return fmt.Sprintf("Update decision %s %s: %s", d.ID, action, reason)
}

func validResolutionSentence(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len([]rune(s)) > 240 || strings.ContainsAny(s, "\r\n<>!?") {
		return false
	}
	if strings.ContainsAny(s[:1], "-*+#>") {
		return false
	}
	rest := strings.TrimLeft(s, "0123456789")
	if rest != s && (strings.HasPrefix(rest, ". ") || strings.HasPrefix(rest, ") ")) {
		return false
	}
	// Require a single terminal period; dots within symbols and commit links
	// are fine, but another sentence boundary is not.
	return strings.HasSuffix(s, ".") && !strings.Contains(strings.TrimSuffix(s, "."), ". ")
}
