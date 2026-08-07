// Package tokenestimate provides the application's prompt-token estimation.
package tokenestimate

// Estimator estimates the number of LLM tokens in text.
type Estimator interface {
	Estimate(text string) int
}

// LengthEstimator enables incremental estimation when token count depends only
// on the byte length of the text.
type LengthEstimator interface {
	EstimateLen(length int) int
}

// SimpleEstimator uses the application's current four-bytes-per-token
// heuristic. Keep the heuristic here so every caller changes together when a
// model-aware tokenizer replaces it.
type SimpleEstimator struct{}

func (SimpleEstimator) Estimate(text string) int {
	return Estimate(text)
}

func (SimpleEstimator) EstimateLen(length int) int {
	return EstimateLen(length)
}

// Estimate returns the estimated token count for text.
func Estimate(text string) int {
	return EstimateLen(len(text))
}

// EstimateLen returns the estimated token count for a byte length.
func EstimateLen(length int) int {
	return length / 4
}
