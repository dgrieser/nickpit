package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dgrieser/nickpit/internal/llm"
	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
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
	contextJSON, err := e.buildDiscussContext(req.ReviewCtx, req.Result, req.PinnedFindingID, req.DisableSuggestions, format)
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
	all := append(append([]llm.Message(nil), prefix...), req.Messages...)
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

	findings := result.Findings
	if disableSuggestions {
		findings = make([]model.Finding, len(result.Findings))
		copy(findings, result.Findings)
		for i := range findings {
			findings[i].Suggestions = nil
		}
	}
	reviewForPrompt := struct {
		ReviewID               string          `json:"review_id,omitempty"`
		OverallCorrectness     string          `json:"overall_correctness"`
		OverallExplanation     string          `json:"overall_explanation"`
		OverallConfidenceScore float64         `json:"overall_confidence_score"`
		Findings               []model.Finding `json:"findings"`
	}{
		ReviewID:               result.ReviewID,
		OverallCorrectness:     result.OverallCorrectness,
		OverallExplanation:     result.OverallExplanation,
		OverallConfidenceScore: result.OverallConfidenceScore,
		Findings:               findings,
	}
	enc, err := json.Marshal(reviewForPrompt)
	if err != nil {
		return "", fmt.Errorf("discuss: marshalling review: %w", err)
	}
	var reviewMap map[string]any
	if err := json.Unmarshal(enc, &reviewMap); err != nil {
		return "", fmt.Errorf("discuss: re-decoding review: %w", err)
	}
	combined["review"] = reviewMap

	if strings.TrimSpace(reviewCtx.Diff) != "" {
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
		title := strings.TrimSpace(f.Title)
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
		fmt.Fprintf(&b, ", priority P%d.", model.PriorityRank(f.Priority))
		b.WriteString(" Ask me anything about it, or push back if you think it's wrong.")
		return b.String()
	}
	return ""
}
