package model

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// HumanTokens renders a token count with k/M/G units and one decimal,
// stripping a trailing ".0": 841 → "841", 38192 → "38.2k", 2000000 → "2M".
func HumanTokens(n int) string {
	value := float64(n)
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	units := []string{"", "k", "M", "G", "T"}
	idx := 0
	// 999.95 is the smallest value that "%.1f" would render as "1000.0", so
	// promote to the next unit at that boundary (999950 → "1M", not "1000k").
	for value >= 999.95 && idx < len(units)-1 {
		value /= 1000
		idx++
	}
	if idx == 0 {
		return fmt.Sprintf("%s%d", sign, int(value))
	}
	formatted := strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")
	return sign + formatted + units[idx]
}

// HumanDuration renders a duration in the log style used for elapsed times:
// truncated to whole seconds, e.g. "4m12s".
func HumanDuration(d time.Duration) string {
	return d.Truncate(time.Second).String()
}

// HumanWait renders a duration for wait, backoff and countdown lines: exact
// below a millisecond, whole milliseconds below a second, one decimal below a
// minute, and whole seconds above it. Sub-second precision is noise at minute
// scale, so a 4m59.573s rate-limit wait reads as "4m59s". Negative durations
// render as "0s".
func HumanWait(d time.Duration) string {
	switch {
	case d <= 0:
		return "0s"
	case d < time.Millisecond:
		return d.String()
	case d < time.Second:
		return d.Round(time.Millisecond).String()
	case d < time.Minute:
		return d.Round(100 * time.Millisecond).String()
	default:
		return d.Truncate(time.Second).String()
	}
}

// RuntimeSeconds converts a duration to float seconds rounded to two
// decimals, the numeric runtime representation used in JSON output.
func RuntimeSeconds(d time.Duration) float64 {
	return math.Round(d.Seconds()*100) / 100
}

// NormalizeBaseURL canonicalizes an LLM endpoint URL for comparison and cache
// keys: surrounding whitespace and a trailing slash carry no meaning, so
// "https://host/v1/" and " https://host/v1" are the same endpoint. This is the
// single definition every layer shares — the config validation, the endpoint→
// client resolution, the capability cache key, and the chat session guard.
func NormalizeBaseURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

// SameEndpoint reports whether two LLM endpoint URLs address the same endpoint.
// Two empty values both mean the client default and match.
func SameEndpoint(a, b string) bool {
	return NormalizeBaseURL(a) == NormalizeBaseURL(b)
}
