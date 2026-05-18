package llm

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Retrier struct {
	MaxRetries        int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	MaxRateLimitDelay time.Duration
	RetryableHTTP     map[int]struct{}
	now               func() time.Time
}

func NewRetrier() *Retrier {
	return &Retrier{
		MaxRetries:        5,
		InitialBackoff:    time.Second,
		MaxBackoff:        30 * time.Second,
		MaxRateLimitDelay: 5 * time.Minute,
		RetryableHTTP: map[int]struct{}{
			http.StatusTooManyRequests:     {},
			http.StatusInternalServerError: {},
			http.StatusBadGateway:          {},
			http.StatusServiceUnavailable:  {},
			http.StatusGatewayTimeout:      {},
		},
	}
}

func (r *Retrier) ShouldRetry(status int) bool {
	_, ok := r.RetryableHTTP[status]
	return ok
}

func (r *Retrier) BackoffForHTTPStatus(attempt, status int, resp *http.Response, message string) time.Duration {
	if status == http.StatusTooManyRequests {
		if delay, ok := r.rateLimitMessageDelay(message); ok {
			return delay
		}
	}
	return r.Backoff(attempt, resp)
}

func (r *Retrier) Backoff(attempt int, resp *http.Response) time.Duration {
	backoff := r.InitialBackoff * time.Duration(1<<attempt)
	if backoff > r.MaxBackoff {
		backoff = r.MaxBackoff
	}
	if resp != nil {
		if header := resp.Header.Get("Retry-After"); header != "" {
			if seconds, err := strconv.Atoi(header); err == nil {
				backoff = time.Duration(seconds) * time.Second
			}
		}
	}
	if backoff < r.InitialBackoff {
		backoff = r.InitialBackoff
	}
	if backoff > r.MaxBackoff {
		backoff = r.MaxBackoff
	}
	return backoff
}

func (r *Retrier) rateLimitMessageDelay(message string) (time.Duration, bool) {
	if r.MaxRateLimitDelay <= 0 {
		return 0, false
	}
	resetAt, ok := parseRateLimitResetTime(message)
	if !ok {
		return 0, false
	}
	now := time.Now
	if r.now != nil {
		now = r.now
	}
	delay := resetAt.Sub(now())
	if delay <= 0 || delay > r.MaxRateLimitDelay {
		return 0, false
	}
	return delay, true
}

func (r *Retrier) Wait(ctx context.Context, attempt int, resp *http.Response) error {
	return r.WaitFor(ctx, r.Backoff(attempt, resp))
}

func (r *Retrier) WaitFor(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var rateLimitResetTimePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:\s*(?:UTC|GMT|Z|[+-]\d{2}:?\d{2}))`),
	regexp.MustCompile(`(?i)[A-Za-z]{3},\s+\d{2}\s+[A-Za-z]{3}\s+\d{4}\s+\d{2}:\d{2}:\d{2}\s+(?:GMT|UTC)`),
}

var rateLimitResetTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	time.RFC1123,
	time.RFC1123Z,
	"2006-01-02 15:04:05 MST",
	"2006-01-02 15:04:05.999999999 MST",
	"2006-01-02T15:04:05 MST",
	"2006-01-02T15:04:05.999999999 MST",
	"2006-01-02 15:04:05 Z07:00",
	"2006-01-02 15:04:05.999999999 Z07:00",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05.999999999 -0700",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05.999999999Z07:00",
}

func parseRateLimitResetTime(message string) (time.Time, bool) {
	message = strings.TrimSpace(message)
	if message == "" {
		return time.Time{}, false
	}
	for _, pattern := range rateLimitResetTimePatterns {
		for _, candidate := range pattern.FindAllString(message, -1) {
			if resetAt, ok := parseRateLimitResetTimeCandidate(candidate); ok {
				return resetAt, true
			}
		}
	}
	return time.Time{}, false
}

func parseRateLimitResetTimeCandidate(candidate string) (time.Time, bool) {
	candidate = strings.Trim(strings.TrimSpace(candidate), `"'.,;()[]`)
	candidate = normalizeRateLimitResetTimeCandidate(candidate)
	if candidate == "" {
		return time.Time{}, false
	}
	for _, layout := range rateLimitResetTimeLayouts {
		if parsed, err := time.Parse(layout, candidate); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func normalizeRateLimitResetTimeCandidate(candidate string) string {
	parts := strings.Fields(candidate)
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if strings.EqualFold(last, "utc") || strings.EqualFold(last, "gmt") {
			parts[len(parts)-1] = strings.ToUpper(last)
			return strings.Join(parts, " ")
		}
	}
	if strings.HasSuffix(candidate, "z") {
		return candidate[:len(candidate)-1] + "Z"
	}
	return candidate
}
