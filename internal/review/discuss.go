package review

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dgrieser/nickpit/internal/config"
	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/internal/scm/reviewmd"
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
	// overall verdict. Its full JSON is placed in the system prompt.
	Result *model.ReviewResult
	// PinnedFindingID, when set, focuses the conversation on one finding and makes
	// the agent open with a message pointing at it.
	PinnedFindingID string
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
	TokensUsed  model.TokenUsage
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
	// bounded) transcript, so the assembled prompt stays inside the budget.
	maxTokens := e.config.MaxContextTokens
	if maxTokens <= 0 {
		maxTokens = config.DefaultMaxContextToken
	}
	messages := boundDiscussTranscript(req.Messages, maxTokens/2, model.SimpleEstimator{})
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
		ToolInstructions           string
		StyleGuideToolchainSnippet string
		ContextJSON                string
	}{
		Pinned:                     pinned,
		HasTools:                   hasTools,
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

	loopResult, err := e.runAgentLoop(ctx, agentLoopRequest{
		AgentName:             "Discuss Review",
		AgentKind:             "discuss",
		Progress:              progress,
		Messages:              all,
		Tools:                 tools,
		Schema:                nil,
		SchemaKind:            llm.SchemaKindText,
		Model:                 e.config.Model,
		MaxTokens:             e.config.MaxTokens,
		Temperature:           e.config.Temperature,
		TopP:                  e.config.TopP,
		TopK:                  e.config.TopK,
		PresencePenalty:       e.config.PresencePenalty,
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
		// The system prompt is already rendered into the transcript, so the
		// no-tools fallback keeps the messages verbatim rather than re-rendering a
		// template (which would choke on style-guide braces).
		NoToolsMessages: func(messages []llm.Message) ([]llm.Message, error) {
			return append([]llm.Message(nil), messages...), nil
		},
	})
	if err != nil {
		return out, err
	}
	out.TokensUsed = loopResult.tokensUsed
	if loopResult.resp != nil {
		out.Reply = strings.TrimSpace(loopResult.resp.RawResponse)
	}

	if len(loopResult.messages) > prefixLen {
		out.NewMessages = append([]llm.Message(nil), loopResult.messages[prefixLen:]...)
	}
	// Some agent-loop fallbacks set the final response without appending it to the
	// transcript (tool-budget / duplicate-tool exits). Guarantee the reply is the
	// last persisted message so a resumed turn replays it.
	if out.Reply != "" {
		last := len(out.NewMessages) - 1
		if last < 0 || out.NewMessages[last].Role != "assistant" || strings.TrimSpace(out.NewMessages[last].Content) != out.Reply {
			out.NewMessages = append(out.NewMessages, llm.Message{Role: "assistant", Content: out.Reply})
		}
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
// prompt. With suggestions disabled the findings are deep-cloned before
// stripping: Finalization/Summarization are pointers, so model.StripSuggestions
// on a shallow copy would mutate the caller's result (and a shallow strip would
// leave their suggestions reachable).
func discussReviewForPrompt(result *model.ReviewResult, disableSuggestions bool) discussReviewPrompt {
	findings := result.Findings
	if disableSuggestions {
		findings = make([]model.Finding, len(result.Findings))
		copy(findings, result.Findings)
		for i := range findings {
			if f := findings[i].Finalization; f != nil {
				clone := *f
				findings[i].Finalization = &clone
			}
			if s := findings[i].Summarization; s != nil {
				clone := *s
				findings[i].Summarization = &clone
			}
		}
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
func boundDiscussTranscript(messages []llm.Message, budget int, estimator model.TokenEstimator) []llm.Message {
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
	estimator := model.SimpleEstimator{}
	overhead := discussPromptHeadroomTokens
	overhead += estimator.Estimate(styleGuideSnippet)
	if reviewJSON, err := json.Marshal(discussReviewForPrompt(result, disableSuggestions)); err == nil {
		overhead += estimator.EstimateLen(len(reviewJSON))
	}
	for _, msg := range messages {
		overhead += estimator.Estimate(msg.Content)
	}
	overhead += promptOverheadTokens(estimator, reviewCtx, format, maxTokens)
	trimmer := NewTrimmer(maxTokens, estimator, WithHeadroomTokens(overhead))
	return trimmer.Trim(reviewCtx)
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
