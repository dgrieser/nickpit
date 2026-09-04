package review

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
	"github.com/dgrieser/nickpit/internal/tokenestimate"
)

// DiscussRequest drives a single turn of the discussion agent: a free-form,
// tool-enabled conversation about a completed review. Unlike the reviewer and
// verifier, the discussion agent is bound to no workflow and no output schema; it
// just answers the author. The caller owns the running conversation (Messages)
// and appends the returned NewMessages to it between turns.
type DiscussRequest struct {
	// ReviewCtx carries the diff, changed files, commits, and toolchain that the
	// reviewers saw. It is rebuilt from the current repo/MR at chat time.
	ReviewCtx *model.ReviewContext
	// Result is the complete review being discussed: every finding plus the
	// overall verdict. Its current JSON is placed in the system prompt; revision
	// history is omitted so old prose cannot crowd or confuse the live finding.
	Result *model.ReviewResult
	// PinnedFindingID, when set, focuses the conversation on one finding and makes
	// the agent open with a message pointing at it.
	PinnedFindingID string
	// AllowReviewUpdates switches the final response to a structured GitLab-chat
	// contract: a normal reply plus complete replacements for existing findings.
	// Other chat front-ends retain free-form text behavior.
	AllowReviewUpdates bool
	// DisableJSONResponseFormat keeps the structured prompt/parser contract but
	// omits the provider-side response_format schema.
	DisableJSONResponseFormat bool
	// Messages is the conversation so far and MUST end with the author's latest
	// user message. The system prompt and (for a pinned chat) the opener are
	// prepended internally, so they are not part of this slice.
	Messages []llm.Message
	RepoRoot string
	// DiffFormat selects the diff shape in the context payload; empty uses the
	// engine profile's configured format.
	DiffFormat model.DiffFormat

	DisableSuggestions       bool
	DisableParallelToolCalls bool

	// Tools overrides the tool set. A nil slice enables all reviewer tools (the
	// default); pass an empty non-nil slice to disable tools entirely.
	Tools []llm.ToolDefinition

	MaxToolCalls          int
	MaxDuplicateToolCalls int
	MaxOutputRetries      int
	MaxReasoningSeconds   int

	Section  *logging.ReasoningSection
	Progress logging.ProgressInfo
}

// DiscussResult is one discussion turn's output.
type DiscussResult struct {
	// Reply is the agent's answer as markdown.
	Reply string
	// Opener is the assistant message pointing at the pinned finding, when the
	// chat is pinned; empty otherwise. It is regenerated each turn from the
	// finding, so the caller need not persist it.
	Opener string
	// NewMessages are the messages appended during this turn (the assistant reply
	// and any tool-call / tool-result messages). The caller appends these to its
	// stored conversation so a later turn replays the same context.
	NewMessages []llm.Message
	Updates     []DiscussFindingUpdate
	TokensUsed  model.TokenUsage
}

// DiscussFindingUpdate is one evidence-backed replacement proposed by the
// discussion agent. Finding is complete rather than patch-shaped, so clearing
// suggestions and other optional values is unambiguous.
type DiscussFindingUpdate struct {
	ID         string        `json:"id"`
	State      string        `json:"state"`
	Resolution string        `json:"resolution,omitempty"`
	Reason     string        `json:"reason"`
	Finding    model.Finding `json:"finding"`
}

type discussStructuredResponse struct {
	Reply          string                 `json:"reply"`
	FindingUpdates []DiscussFindingUpdate `json:"finding_updates"`
}

// Discuss runs one turn of the discussion agent and returns its reply. The agent
// receives the full review context, the complete findings JSON, and the diff, and
// answers the author's latest message using the same retrieval tools a reviewer
// has (unless req.Tools restricts them).
func (e *Engine) Discuss(ctx context.Context, req DiscussRequest) (DiscussResult, error) {
	var out DiscussResult
	if req.ReviewCtx == nil {
		return out, fmt.Errorf("discuss: nil review context")
	}
	if req.Result == nil {
		return out, fmt.Errorf("discuss: nil review result")
	}
	if len(req.Messages) == 0 {
		return out, fmt.Errorf("discuss: no messages")
	}

	format := req.DiffFormat
	if format == "" {
		format = e.config.DiffFormat
	}
	pinned := strings.TrimSpace(req.PinnedFindingID) != ""

	tools := req.Tools
	if tools == nil {
		tools = reviewerToolDefinitions()
	}
	hasTools := len(tools) > 0

	systemTemplate, err := e.loadPrompt("agent_discuss_system_prompt.tmpl")
	if err != nil {
		return out, err
	}
	var toolInstructions string
	if hasTools {
		toolInstructions, err = e.renderToolInstructions(toolInstructionsConfig{
			agentRole:                "discuss",
			parallelToolCallGuidance: !req.DisableParallelToolCalls,
		})
		if err != nil {
			return out, err
		}
	}
	styleGuides, err := e.styleGuidesFor(req.ReviewCtx)
	if err != nil {
		return out, err
	}
	styleGuideToolchainSnippet, err := e.renderStyleGuideToolchainSnippet("discuss", styleGuides, len(req.ReviewCtx.ToolchainVersions) > 0)
	if err != nil {
		return out, err
	}
	// Bound the transcript first — a long session would otherwise exceed the
	// model window no matter how hard the context is trimmed — then re-trim the
	// context reserving room for the findings JSON, styleguides, and the (now
	// bounded) transcript, so the assembled prompt stays inside the budget. The
	// transcript's budget is derived from the space REMAINING after the
	// mandatory prompt parts: a review whose findings JSON alone eats most of
	// the window must squeeze the transcript accordingly, or the assembled
	// request stays oversized no matter how hard trimForDiscuss trims (it can
	// only remove review context).
	maxTokens := e.config.MaxContextTokens
	if maxTokens <= 0 {
		maxTokens = config.DefaultMaxContextToken
	}
	estimator := tokenestimate.SimpleEstimator{}
	fixedOverhead := discussFixedOverheadTokens(req.Result, req.DisableSuggestions, styleGuideToolchainSnippet, estimator)
	messages := boundDiscussTranscript(req.Messages, discussTranscriptBudget(maxTokens, fixedOverhead), estimator)
	reviewCtx, err := e.trimForDiscuss(req.ReviewCtx, req.Result, messages, styleGuideToolchainSnippet, req.DisableSuggestions, format)
	if err != nil {
		return out, fmt.Errorf("discuss: trimming context: %w", err)
	}
	contextJSON, err := e.buildDiscussContext(reviewCtx, req.Result, req.PinnedFindingID, req.DisableSuggestions, format)
	if err != nil {
		return out, err
	}
	systemPrompt, err := llm.RenderPrompt(systemTemplate, struct {
		Pinned                     bool
		HasTools                   bool
		AllowReviewUpdates         bool
		ToolInstructions           string
		StyleGuideToolchainSnippet string
		ContextJSON                string
	}{
		Pinned:                     pinned,
		HasTools:                   hasTools,
		AllowReviewUpdates:         req.AllowReviewUpdates,
		ToolInstructions:           toolInstructions,
		StyleGuideToolchainSnippet: styleGuideToolchainSnippet,
		ContextJSON:                contextJSON,
	})
	if err != nil {
		return out, fmt.Errorf("discuss: rendering system prompt: %w", err)
	}

	prefix := []llm.Message{{Role: "system", Content: systemPrompt}}
	if pinned {
		if opener := discussOpener(req.Result, req.PinnedFindingID); opener != "" {
			out.Opener = opener
			prefix = append(prefix, llm.Message{Role: "assistant", Content: opener})
		}
	}
	all := append(append([]llm.Message(nil), prefix...), messages...)
	prefixLen := len(all)

	progress := req.Progress
	if progress.IsZero() {
		progress = e.progressInfo("discuss", "Discuss Review", "")
	}

	schemaKind := llm.SchemaKindText
	var schema []byte
	var validate func(*llm.ReviewResponse) *llm.InvalidResponseError
	if req.AllowReviewUpdates {
		if !req.DisableJSONResponseFormat {
			schema, err = discussResponseSchema(req.Result, req.PinnedFindingID, req.DisableSuggestions)
			if err != nil {
				return out, err
			}
		}
		schemaKind = llm.SchemaKindJSON
		validate = discussResponseValidator(req.Result, req.PinnedFindingID, req.DisableSuggestions)
	}
	loopResult, err := e.runAgentLoop(ctx, agentLoopRequest{
		AgentName:             "Discuss Review",
		AgentKind:             "discuss",
		Progress:              progress,
		Messages:              all,
		Tools:                 tools,
		Schema:                schema,
		SchemaKind:            schemaKind,
		Model:                 e.config.Model,
		MaxTokens:             e.config.MaxTokens,
		Temperature:           e.config.Temperature,
		TopP:                  e.config.TopP,
		TopK:                  e.config.TopK,
		MinP:                  e.config.MinP,
		PresencePenalty:       e.config.PresencePenalty,
		RepetitionPenalty:     e.config.RepetitionPenalty,
		ExtraBody:             e.config.ExtraBody,
		ParallelToolCalls:     !req.DisableParallelToolCalls,
		ReasoningEffort:       e.config.ReasoningEffort,
		RepoRoot:              req.RepoRoot,
		MaxToolCalls:          req.MaxToolCalls,
		MaxDuplicateToolCalls: req.MaxDuplicateToolCalls,
		MaxOutputRetries:      req.MaxOutputRetries,
		MaxReasoningSeconds:   req.MaxReasoningSeconds,
		State:                 newAgentLoopState(),
		Section:               req.Section,
		NoToolsSystem:         systemPrompt,
		// The no-tools fallback must not receive tool-role messages or dangling
		// tool_calls — a request without a tools field carrying them is rejected
		// by strict OpenAI-compatible backends. noToolsMessagesFromRendered
		// rewrites them into plain turns; it takes the already-rendered system
		// prompt, so style-guide braces never hit a template.
		NoToolsMessages: func(messages []llm.Message) ([]llm.Message, error) {
			return noToolsMessagesFromRendered(systemPrompt, messages)
		},
		ValidateResponse: validate,
	})
	if err != nil {
		return out, err
	}
	out.TokensUsed = loopResult.tokensUsed
	if loopResult.resp != nil {
		if req.AllowReviewUpdates {
			if invalid := discussResponseValidator(req.Result, req.PinnedFindingID, req.DisableSuggestions)(loopResult.resp); invalid != nil {
				return out, invalid
			}
			parsed, parseErr := parseDiscussStructuredResponse(loopResult.resp.RawResponse)
			if parseErr != nil {
				return out, parseErr
			}
			out.Reply = strings.TrimSpace(parsed.Reply)
			out.Updates = parsed.FindingUpdates
		} else {
			out.Reply = strings.TrimSpace(loopResult.resp.RawResponse)
		}
	}

	if len(loopResult.messages) > prefixLen {
		out.NewMessages = append([]llm.Message(nil), loopResult.messages[prefixLen:]...)
	}
	// Some agent-loop fallbacks set the final response without appending it to the
	// transcript (tool-budget / duplicate-tool exits). Guarantee the reply is the
	// last persisted message so a resumed turn replays it.
	if out.Reply != "" {
		last := len(out.NewMessages) - 1
		replacedStructured := false
		if req.AllowReviewUpdates && last >= 0 && out.NewMessages[last].Role == "assistant" {
			if parsed, err := parseDiscussStructuredResponse(out.NewMessages[last].Content); err == nil && strings.TrimSpace(parsed.Reply) == out.Reply {
				out.NewMessages[last].Content = out.Reply
				out.NewMessages[last].ToolCalls = nil
				replacedStructured = true
			}
		}
		if !replacedStructured && (last < 0 || out.NewMessages[last].Role != "assistant" || strings.TrimSpace(out.NewMessages[last].Content) != out.Reply) {
			out.NewMessages = append(out.NewMessages, llm.Message{Role: "assistant", Content: out.Reply})
		}
	}
	return out, nil
}

func parseDiscussStructuredResponse(raw string) (discussStructuredResponse, error) {
	var parsed discussStructuredResponse
	if err := llm.LenientUnmarshal(raw, &parsed); err != nil {
		return parsed, fmt.Errorf("discuss: parsing structured response: %w", err)
	}
	return parsed, nil
}

var additionalSentence = regexp.MustCompile(`[.!?]\s+\S`)

func discussResponseValidator(result *model.ReviewResult, pinnedID string, disableSuggestions bool) func(*llm.ReviewResponse) *llm.InvalidResponseError {
	allowed := make(map[string]struct{}, len(result.Findings))
	for _, finding := range result.Findings {
		allowed[finding.ID] = struct{}{}
	}
	return func(resp *llm.ReviewResponse) *llm.InvalidResponseError {
		parsed, err := parseDiscussStructuredResponse(resp.RawResponse)
		invalid := func(reason string) *llm.InvalidResponseError {
			return &llm.InvalidResponseError{RawContent: resp.RawResponse, Reason: reason, MissingFields: []string{"reply", "finding_updates"}}
		}
		if err != nil {
			return invalid(err.Error())
		}
		if strings.TrimSpace(parsed.Reply) == "" {
			return invalid("discussion reply is empty")
		}
		seen := make(map[string]struct{}, len(parsed.FindingUpdates))
		for _, update := range parsed.FindingUpdates {
			id := strings.TrimSpace(update.ID)
			if _, ok := allowed[id]; !ok || id == "" || update.Finding.ID != id {
				return invalid("finding update references an unknown or mismatched id")
			}
			if pinnedID != "" && id != pinnedID {
				return invalid("pinned discussion may update only its focused finding")
			}
			if _, duplicate := seen[id]; duplicate {
				return invalid("finding update id is duplicated")
			}
			seen[id] = struct{}{}
			state := strings.ToLower(strings.TrimSpace(update.State))
			if state != model.FindingStateActive && state != model.FindingStateResolved {
				return invalid("finding state must be active or resolved")
			}
			if strings.TrimSpace(update.Reason) == "" {
				return invalid("finding update lacks evidence reason")
			}
			if p := update.Finding.Priority; p == nil || *p < 0 || *p > 3 {
				return invalid("finding priority must be between 0 and 3")
			}
			if update.Finding.ConfidenceScore < 0 || update.Finding.ConfidenceScore > 1 {
				return invalid("finding confidence must be between 0 and 1")
			}
			if strings.TrimSpace(update.Finding.Title) == "" || strings.TrimSpace(update.Finding.Body) == "" {
				return invalid("active finding replacement requires title and body")
			}
			loc := update.Finding.CodeLocation
			if strings.TrimSpace(loc.FilePath) == "" || loc.LineRange.Start <= 0 || loc.LineRange.End < loc.LineRange.Start {
				return invalid("finding replacement has an invalid code location")
			}
			if disableSuggestions && len(update.Finding.Suggestions) > 0 {
				return invalid("suggestions are disabled")
			}
			if state == model.FindingStateResolved {
				summary := strings.TrimSpace(update.Resolution)
				if summary == "" || utf8.RuneCountInString(summary) > 160 || strings.ContainsAny(summary, "\r\n") || additionalSentence.MatchString(summary) {
					return invalid("resolved finding requires one factual sentence of at most 160 characters")
				}
				last, _ := utf8.DecodeLastRuneInString(summary)
				if !unicode.IsPunct(last) {
					return invalid("resolution sentence must end with punctuation")
				}
			}
		}
		return nil
	}
}

func discussResponseSchema(result *model.ReviewResult, pinnedID string, disableSuggestions bool) ([]byte, error) {
	ids := make([]string, 0, len(result.Findings))
	for _, finding := range result.Findings {
		if pinnedID == "" || finding.ID == pinnedID {
			ids = append(ids, finding.ID)
		}
	}
	location := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{"type": "string"},
			"line_range": map[string]any{"type": "object", "properties": map[string]any{
				"start": map[string]any{"type": "integer"}, "end": map[string]any{"type": "integer"}, "count": map[string]any{"type": "integer"},
			}, "required": []string{"start", "end", "count"}},
			"language": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"},
		},
		"required": []string{"file_path", "line_range", "content"},
	}
	findingProps := map[string]any{
		"id": map[string]any{"type": "string", "enum": ids}, "title": map[string]any{"type": "string"},
		"body": map[string]any{"type": "string"}, "confidence_score": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		"priority": map[string]any{"type": "integer", "minimum": 0, "maximum": 3}, "code_location": location,
	}
	if !disableSuggestions {
		findingProps["suggestions"] = map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
			"body": map[string]any{"type": "string"}, "code_location": location,
		}, "required": []string{"body", "code_location"}}}
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"reply": map[string]any{"type": "string"},
			"finding_updates": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "properties": map[string]any{
					"id": map[string]any{"type": "string", "enum": ids}, "state": map[string]any{"type": "string", "enum": []string{model.FindingStateActive, model.FindingStateResolved}},
					"resolution": map[string]any{"type": "string", "maxLength": 160}, "reason": map[string]any{"type": "string"},
					"finding": map[string]any{"type": "object", "properties": findingProps, "required": []string{"id", "title", "body", "confidence_score", "priority", "code_location"}},
				}, "required": []string{"id", "state", "reason", "finding"},
			}},
		},
		"required": []string{"reply", "finding_updates"},
	}
	out, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("discuss: encoding response schema: %w", err)
	}
	return out, nil
}

// buildDiscussContext assembles the JSON context injected into the discussion
// system prompt: the standard reviewer payload (repository, changed files, diff,
// commits, toolchain) plus the complete review (all findings and the overall
// verdict) and the raw unified diff.
func (e *Engine) buildDiscussContext(reviewCtx *model.ReviewContext, result *model.ReviewResult, pinnedID string, disableSuggestions bool, format model.DiffFormat) (string, error) {
	payload := model.PromptPayloadFromContextWithDiffFormat(reviewCtx, format)
	base, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("discuss: marshalling review payload: %w", err)
	}
	var combined map[string]any
	if err := json.Unmarshal(base, &combined); err != nil {
		return "", fmt.Errorf("discuss: re-decoding review payload: %w", err)
	}

	enc, err := json.Marshal(discussReviewForPrompt(result, disableSuggestions))
	if err != nil {
		return "", fmt.Errorf("discuss: marshalling review: %w", err)
	}
	var reviewMap map[string]any
	if err := json.Unmarshal(enc, &reviewMap); err != nil {
		return "", fmt.Errorf("discuss: re-decoding review: %w", err)
	}
	combined["review"] = reviewMap

	// The payload already carries the complete patch as diff_files or diff_hunks;
	// injecting the raw unified diff on top would double the patch token cost and
	// can push a near-budget context over the model window. Fall back to the raw
	// diff only when the payload carries no structured diff at all.
	if len(payload.DiffFiles) == 0 && len(payload.DiffHunks) == 0 && strings.TrimSpace(reviewCtx.Diff) != "" {
		combined["diff"] = reviewCtx.Diff
	}
	if strings.TrimSpace(pinnedID) != "" {
		combined["focus_finding_id"] = pinnedID
	}

	out, err := json.MarshalIndent(combined, "", "  ")
	if err != nil {
		return "", fmt.Errorf("discuss: encoding combined payload: %w", err)
	}
	return string(out), nil
}

// discussReviewPrompt is the review shape embedded in the discussion system
// prompt: the overall verdict plus the complete findings.
type discussReviewPrompt struct {
	ReviewID               string          `json:"review_id,omitempty"`
	OverallCorrectness     string          `json:"overall_correctness"`
	OverallExplanation     string          `json:"overall_explanation"`
	OverallConfidenceScore float64         `json:"overall_confidence_score"`
	Findings               []model.Finding `json:"findings"`
}

// discussReviewForPrompt builds the review object embedded in the discussion
// prompt. Revision history and mutation bookkeeping are deliberately omitted.
// Pointer-owned downstream values are cloned before suggestions may be stripped.
func discussReviewForPrompt(result *model.ReviewResult, disableSuggestions bool) discussReviewPrompt {
	findings := make([]model.Finding, len(result.Findings))
	copy(findings, result.Findings)
	for i := range findings {
		findings[i].Revision = 0
		findings[i].LastUpdateID = ""
		findings[i].History = nil
		if disableSuggestions {
			if f := findings[i].Finalization; f != nil {
				clone := *f
				findings[i].Finalization = &clone
			}
			if s := findings[i].Summarization; s != nil {
				clone := *s
				findings[i].Summarization = &clone
			}
		}
	}
	if disableSuggestions {
		model.StripSuggestions(findings)
	}
	return discussReviewPrompt{
		ReviewID:               result.ReviewID,
		OverallCorrectness:     result.OverallCorrectness,
		OverallExplanation:     result.OverallExplanation,
		OverallConfidenceScore: result.OverallConfidenceScore,
		Findings:               findings,
	}
}

// discussPromptHeadroomTokens approximates the fixed parts of the discussion
// prompt that are not the review payload or the transcript: the system template
// text, tool instructions, and the opener.
const discussPromptHeadroomTokens = 2000

// discussFixedOverheadTokens estimates the mandatory, untrimmable parts of the
// discussion prompt: the fixed template/tool text, the styleguide snippet, and
// the complete review JSON (the discussion's substance, injected verbatim). The
// transcript and the review context must share whatever the window has left.
func discussFixedOverheadTokens(result *model.ReviewResult, disableSuggestions bool, styleGuideSnippet string, estimator tokenestimate.Estimator) int {
	overhead := discussPromptHeadroomTokens
	overhead += estimator.Estimate(styleGuideSnippet)
	// Indented, matching how buildDiscussContext actually embeds the review
	// (MarshalIndent) — the compact form underestimates by 15-30%, enough to
	// push a findings-heavy review past max_context_tokens despite trimming.
	if reviewJSON, err := json.MarshalIndent(discussReviewForPrompt(result, disableSuggestions), "", "  "); err == nil {
		overhead += estimator.Estimate(string(reviewJSON))
	}
	return overhead
}

// discussTranscriptBudget returns the token budget for the running transcript:
// the space left after the mandatory prompt parts, capped at half the window so
// the review context always keeps room, and floored at one token — with the
// extras already filling the window, the transcript must shrink to its own
// minimum (boundDiscussTranscript keeps the newest user message
// unconditionally, so the latest question still goes out).
func discussTranscriptBudget(maxTokens, fixedOverhead int) int {
	return max(min(maxTokens/2, maxTokens-fixedOverhead), 1)
}

// discussOmittedTurnsNote is prepended to the oldest kept user message when
// earlier turns were dropped to fit the context window, so the model knows the
// transcript is partial rather than inventing continuity.
const discussOmittedTurnsNote = "[Earlier conversation turns were omitted to fit the context window.]"

// boundDiscussTranscript caps the conversation sent to the model at budget
// tokens by dropping the OLDEST turns. Without this a long resumable session
// eventually exceeds the model window no matter how hard the context is
// trimmed, because every historical message is appended to the request. The cut
// only happens at a user message so an assistant tool-call message is never
// stranded from its tool results (strict providers reject that), and the newest
// user message is always kept even when it alone exceeds the budget. When turns
// were dropped, a short note is prepended to the oldest kept user message. The
// input slice is never mutated.
func boundDiscussTranscript(messages []llm.Message, budget int, estimator tokenestimate.Estimator) []llm.Message {
	if budget <= 0 || len(messages) == 0 {
		return messages
	}
	total := 0
	for _, msg := range messages {
		total += estimator.Estimate(msg.Content)
	}
	if total <= budget {
		return messages
	}
	// Walk backwards accumulating whole turns: `start` is only ever moved to an
	// index holding a user message. The newest user turn is kept unconditionally.
	start := len(messages)
	used := 0
	tail := 0 // tokens in messages after the last examined user message
	for i, msg := range slices.Backward(messages) {
		tail += estimator.Estimate(msg.Content)
		if msg.Role != "user" {
			continue
		}
		if start != len(messages) && used+tail > budget {
			break
		}
		start = i
		used += tail
		tail = 0
	}
	if start == len(messages) {
		// No user message found at all (should not happen: callers end the
		// transcript with the author's question); send everything rather than
		// nothing.
		return messages
	}
	if start == 0 {
		return messages
	}
	bounded := append([]llm.Message(nil), messages[start:]...)
	bounded[0].Content = discussOmittedTurnsNote + "\n\n" + bounded[0].Content
	return bounded
}

// trimForDiscuss re-trims a prepared review context for the discussion prompt.
// The context was trimmed against the budget of a REVIEW prompt; a discussion
// adds the complete findings JSON, the styleguides, and the running transcript
// on top, so a near-budget context would push the first (or a later) chat turn
// over the model window. Reserving that extra content as trimmer headroom keeps
// the assembled prompt inside max_context_tokens; the trimmer clones, so the
// caller's (possibly session-cached) context is never mutated. When the extras
// alone exceed the budget the context is trimmed to its minimum rather than
// failing — the findings and the question are the discussion's substance.
func (e *Engine) trimForDiscuss(reviewCtx *model.ReviewContext, result *model.ReviewResult, messages []llm.Message, styleGuideSnippet string, disableSuggestions bool, format model.DiffFormat) (*model.ReviewContext, error) {
	maxTokens := e.config.MaxContextTokens
	if maxTokens <= 0 {
		maxTokens = config.DefaultMaxContextToken
	}
	estimator := tokenestimate.SimpleEstimator{}
	overhead := discussFixedOverheadTokens(result, disableSuggestions, styleGuideSnippet, estimator)
	for _, msg := range messages {
		overhead += estimator.Estimate(msg.Content)
	}
	overhead += promptOverheadTokens(estimator, reviewCtx, format, maxTokens)
	trimmer := NewTrimmer(maxTokens, estimator, WithHeadroomTokens(overhead))
	trimmed, err := trimmer.Trim(reviewCtx)
	if err != nil {
		return nil, err
	}
	// The trimmer deliberately never evicts the final diff file or hunk, so when
	// the extras leave less room than that last item the trimmed context can
	// still exceed the budget and the provider would reject the request. Verify
	// the final size and truncate the remaining diff as a last resort.
	enforceDiscussBudget(trimmed, max(maxTokens-overhead, 1), estimator)
	return trimmed, nil
}

// enforceDiscussBudget hard-caps a trimmed context at budget tokens by halving
// the remaining diff content until it fits (possibly to nothing). Unlike the
// trimmer's eviction loops it is allowed to gut the LAST diff file or hunk:
// losing diff detail is recoverable — the model still has the findings JSON,
// the conversation, and the retrieval tools — while an over-budget request is
// rejected outright. All diff representations shrink together because
// renderContextText counts ctx.Diff when set while the prompt payload renders
// DiffFiles/DiffHunks.
func enforceDiscussBudget(ctx *model.ReviewContext, budget int, estimator tokenestimate.Estimator) {
	if ctx == nil {
		return
	}
	over := func() bool { return estimator.Estimate(renderContextText(ctx)) > budget }
	if !over() {
		return
	}
	truncated := false
	for over() {
		reduced := false
		for i := range ctx.DiffFiles {
			if next, ok := halveAtRuneBoundary(ctx.DiffFiles[i].Content); ok {
				ctx.DiffFiles[i].Content = next
				reduced, truncated = true, true
			}
		}
		for i := range ctx.DiffHunks {
			if next, ok := halveAtRuneBoundary(ctx.DiffHunks[i].Content); ok {
				ctx.DiffHunks[i].Content = next
				reduced, truncated = true, true
			}
		}
		if next, ok := halveAtRuneBoundary(ctx.Diff); ok {
			ctx.Diff = next
			reduced, truncated = true, true
		}
		if !reduced {
			// Nothing left to cut: the residue is title/paths/etc., and the
			// remaining overage (if any) is the findings and transcript — the
			// discussion's substance, kept by design.
			break
		}
	}
	if truncated {
		ctx.OmittedSections = append(ctx.OmittedSections, "diff truncated to fit the discussion context budget")
	}
}

// halveAtRuneBoundary returns s cut to (at most) half its byte length, moved
// back to a UTF-8 rune boundary so the truncation never leaves a split rune
// (which would render as U+FFFD in the prompt). ok is false when nothing can
// be removed.
func halveAtRuneBoundary(s string) (string, bool) {
	cut := len(s) / 2
	if cut == 0 {
		return s, false
	}
	end := len(s) - cut
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end], true
}

// DiscussOpener renders the assistant's first message for a finding-pinned chat,
// pointing the author at the finding. It returns "" when the id is not found. It
// is exported so front-ends can display the opener without running a turn.
func DiscussOpener(result *model.ReviewResult, findingID string) string {
	return discussOpener(result, findingID)
}

// discussOpener renders the assistant's first message for a finding-pinned chat,
// pointing the author at the finding. It returns "" when the id is not found.
func discussOpener(result *model.ReviewResult, findingID string) string {
	for _, f := range result.Findings {
		if f.ID != findingID {
			continue
		}
		// Use the same summarization/finalization precedence as the published
		// comment (reviewmd.FindingDisplay), so the opener names the title and
		// priority the user actually selected, not the pre-finalize originals.
		title, _, rank, _ := reviewmd.FindingDisplay(f)
		title = strings.TrimSpace(title)
		if title == "" {
			title = "this finding"
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Let's discuss the finding **%s**", title)
		if loc := f.CodeLocation.FilePath; loc != "" {
			if f.CodeLocation.LineRange.Start > 0 {
				fmt.Fprintf(&b, " (`%s:%d`)", loc, f.CodeLocation.LineRange.Start)
			} else {
				fmt.Fprintf(&b, " (`%s`)", loc)
			}
		}
		fmt.Fprintf(&b, ", priority P%d.", rank)
		b.WriteString(" Ask me anything about it, or push back if you think it's wrong.")
		return b.String()
	}
	return ""
}
