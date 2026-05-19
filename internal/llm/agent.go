package llm

import "context"

type agentLabelKey struct{}

// WithAgentLabel attaches an agent identifier (e.g. "reviewer #1 #3") to ctx
// so that downstream log calls can prefix verbose output with the agent's
// role/name/turn.
func WithAgentLabel(ctx context.Context, label string) context.Context {
	if label == "" {
		return ctx
	}
	return context.WithValue(ctx, agentLabelKey{}, label)
}

// AgentLabelFromContext returns the agent label set via WithAgentLabel, or "".
func AgentLabelFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(agentLabelKey{}).(string)
	return s
}
