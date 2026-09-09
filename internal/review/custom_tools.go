package review

import (
	"context"

	"github.com/dgrieser/nickpit/internal/llm"
)

// Retrieval remains parallel; side-effecting callbacks run serially after the
// reads, with result order and provider tool-call IDs preserved.
func (e *Engine) executeAgentTools(ctx context.Context, req agentLoopRequest, calls []llm.ToolCall, state *toolRoundState) ([]llm.Message, error) {
	var reads []llm.ToolCall
	var indexes []int
	out := make([]llm.Message, len(calls))
	for i, call := range calls {
		if req.ToolHandlers[call.Name] == nil {
			reads = append(reads, call)
			indexes = append(indexes, i)
		}
	}
	for j, message := range e.executeToolCalls(ctx, req.RepoRoot, reads, state) {
		out[indexes[j]] = message
	}
	for i, call := range calls {
		if handler := req.ToolHandlers[call.Name]; handler != nil {
			body, err := handler(ctx, call)
			if err != nil {
				return nil, err
			}
			out[i] = llm.Message{Role: "tool", ToolCallID: call.ID, Name: call.Name, Content: body}
			e.logToolCall(ctx, call, body)
		}
	}
	return out, nil
}
