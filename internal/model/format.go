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

// RuntimeSeconds converts a duration to float seconds rounded to two
// decimals, the numeric runtime representation used in JSON output.
func RuntimeSeconds(d time.Duration) float64 {
	return math.Round(d.Seconds()*100) / 100
}
