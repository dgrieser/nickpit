package model

import (
	"testing"
	"time"
)

func TestRetryCounter(t *testing.T) {
	for _, tc := range []struct {
		retry int
		max   int
		want  string
	}{
		{1, 5, "1/5"},
		{5, 5, "5/5"},
		{2, 0, "2"},
		{2, -1, "2"},
	} {
		if got := retryCounter(tc.retry, tc.max); got != tc.want {
			t.Fatalf("retryCounter(%d, %d) = %q, want %q", tc.retry, tc.max, got, tc.want)
		}
	}
}

func TestRetryCountLabel(t *testing.T) {
	for _, tc := range []struct {
		retries int
		want    string
	}{
		{0, "0 retries"},
		{1, "1 retry"},
		{3, "3 retries"},
	} {
		if got := RetryCountLabel(tc.retries); got != tc.want {
			t.Fatalf("RetryCountLabel(%d) = %q, want %q", tc.retries, got, tc.want)
		}
	}
}

func TestRetryLine(t *testing.T) {
	for _, tc := range []struct {
		name   string
		retry  int
		max    int
		reason string
		wait   time.Duration
		want   string
	}{
		{"bounded with wait", 2, 5, "network error", 4 * time.Second, "2/5 network error, waiting 4s"},
		{"unbounded without wait", 1, 0, "prompt too long", 0, "1 prompt too long"},
		{"no reason", 1, 3, "", 0, "1/3"},
	} {
		if got := RetryLine(tc.retry, tc.max, tc.reason, tc.wait); got != tc.want {
			t.Fatalf("%s: RetryLine = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestHumanWait(t *testing.T) {
	for _, tc := range []struct {
		in   time.Duration
		want string
	}{
		{-time.Second, "0s"},
		{0, "0s"},
		{800 * time.Microsecond, "800µs"},
		{1500 * time.Microsecond, "2ms"},
		{2134 * time.Millisecond, "2.1s"},
		{59 * time.Second, "59s"},
		// Sub-second precision is noise at minute scale.
		{4*time.Minute + 59*time.Second + 573*time.Millisecond, "4m59s"},
		{10 * time.Minute, "10m0s"},
	} {
		if got := HumanWait(tc.in); got != tc.want {
			t.Fatalf("HumanWait(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRetryBudget(t *testing.T) {
	for _, tc := range []struct {
		spent int
		limit int
		name  string
		want  string
	}{
		{2, 5, "request retries", "2/5 request retries"},
		{2, 0, "request retries", ""},
		{2, -1, "request retries", ""},
	} {
		if got := RetryBudget(tc.spent, tc.limit, tc.name); got != tc.want {
			t.Fatalf("RetryBudget(%d, %d, %q) = %q, want %q", tc.spent, tc.limit, tc.name, got, tc.want)
		}
	}
}

func TestRetryBudgetLine(t *testing.T) {
	for _, tc := range []struct {
		name   string
		retry  int
		reason string
		wait   time.Duration
		gauge  string
		want   string
	}{
		// The counter spans every budget the call has, so the one that bounds
		// this retry is named rather than shown as a second denominator.
		{"gauged with wait", 4, "status=500", 4 * time.Second, "2/5 request retries", "4 status=500, waiting 4s (2/5 request retries)"},
		{"unbounded retry has no gauge", 3, "prompt too long", 0, "", "3 prompt too long"},
		{"wait gauges need no fraction", 5, "rate limited (429)", 30 * time.Second, "waited 9m20s/10m0s", "5 rate limited (429), waiting 30s (waited 9m20s/10m0s)"},
	} {
		if got := RetryBudgetLine(tc.retry, tc.reason, tc.wait, tc.gauge); got != tc.want {
			t.Fatalf("%s: RetryBudgetLine = %q, want %q", tc.name, got, tc.want)
		}
	}
}
