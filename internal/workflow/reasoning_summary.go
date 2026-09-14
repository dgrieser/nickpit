package workflow

// StepSupportsReasoningSummary identifies agents that explore with tools before
// handing their captured reasoning to a separate, tool-free summarizer.
func StepSupportsReasoningSummary(stepType string) bool {
	return stepType == StepCollectContext || isVerifyStep(stepType)
}

// ReasoningSummaryOverride resolves defaults without modifying the spec. Use
// this same resolution before scaling, capability discovery, and execution.
// Reasoning effort deliberately inherits the selected model's setting: urgent
// execution caps it at low, preserving an already lower setting such as none.
func ReasoningSummaryOverride(cfg *StepOverride) *AgentOverride {
	o := AgentOverride{}
	if cfg != nil && cfg.SummarizeReasoning != nil {
		o = *cfg.SummarizeReasoning
	}
	if o.Model == nil {
		alias := SmallModelAlias
		o.Model = &alias
	}
	tb := TimeBudget{}
	if o.TimeBudget != nil {
		tb = *o.TimeBudget
	}
	if tb.Weight == nil {
		weight := 25
		tb.Weight = &weight
	}
	if tb.MaxSeconds == nil {
		seconds := 30
		tb.MaxSeconds = &seconds
	}
	if tb.SpeedupThreshold == nil {
		threshold := 100
		tb.SpeedupThreshold = &threshold
	}
	o.TimeBudget = &tb
	return &o
}
