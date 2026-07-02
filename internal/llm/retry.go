package llm

import (
	"context"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type Retrier struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	// MaxTotalRateLimitWait caps the cumulative time a single request is
	// allowed to spend waiting on rate-limit (429) retries. Once the total
	// would exceed this budget the caller stops retrying and surfaces the
	// last rate-limit error. Zero disables the cap.
	MaxTotalRateLimitWait time.Duration
	maxRateLimitDelay     atomic.Int64
	RetryableHTTP         map[int]struct{}
	now                   func() time.Time
	// rand returns a value in [0, 1) used to jitter computed backoffs so
	// concurrent lanes do not retry in lockstep. nil uses math/rand; tests
	// stub it for determinism.
	rand func() float64
}

func NewRetrier() *Retrier {
	r := &Retrier{
		MaxRetries:            5,
		InitialBackoff:        time.Second,
		MaxBackoff:            30 * time.Second,
		MaxTotalRateLimitWait: 10 * time.Minute,
		RetryableHTTP: map[int]struct{}{
			http.StatusTooManyRequests:     {},
			http.StatusInternalServerError: {},
			http.StatusBadGateway:          {},
			http.StatusServiceUnavailable:  {},
			http.StatusGatewayTimeout:      {},
		},
	}
	r.maxRateLimitDelay.Store(int64(5 * time.Minute))
	return r
}

func (r *Retrier) SetMaxRateLimitDelay(delay time.Duration) {
	r.maxRateLimitDelay.Store(int64(delay))
}

func (r *Retrier) MaxRateLimitDelay() time.Duration {
	return time.Duration(r.maxRateLimitDelay.Load())
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
	if resp != nil {
		if header := resp.Header.Get("Retry-After"); header != "" {
			if delay, ok := parseRetryAfter(header, r.timeNow()); ok {
				// Server-instructed delays are not jittered and may exceed
				// MaxBackoff: honor them up to the same ceiling as
				// message-embedded rate-limit reset hints.
				if delay < r.InitialBackoff {
					delay = r.InitialBackoff
				}
				ceiling := r.MaxRateLimitDelay()
				if ceiling <= 0 {
					ceiling = r.MaxBackoff
				}
				if delay > ceiling {
					delay = ceiling
				}
				return delay
			}
		}
	}
	return r.jitter(r.exponentialBackoff(attempt))
}

// exponentialBackoff computes InitialBackoff*2^attempt saturating at
// MaxBackoff. Doubling stops as soon as the backoff reaches MaxBackoff, so
// large attempt counts can never overflow time.Duration (a plain shift
// overflows int64 at attempt >= 34 and would clamp the wait back down to
// InitialBackoff).
func (r *Retrier) exponentialBackoff(attempt int) time.Duration {
	backoff := r.InitialBackoff
	for i := 0; i < attempt && backoff > 0 && backoff < r.MaxBackoff; i++ {
		if backoff > r.MaxBackoff/2 {
			backoff = r.MaxBackoff
			break
		}
		backoff *= 2
	}
	if backoff < r.InitialBackoff {
		backoff = r.InitialBackoff
	}
	if backoff > r.MaxBackoff {
		backoff = r.MaxBackoff
	}
	return backoff
}

// jitter spreads a computed backoff by +-20% so concurrent lanes hitting the
// same failure do not retry in lockstep. Server-instructed delays (Retry-After
// headers, message-embedded reset hints) are never jittered.
func (r *Retrier) jitter(backoff time.Duration) time.Duration {
	if backoff <= 0 {
		return backoff
	}
	random := rand.Float64
	if r.rand != nil {
		random = r.rand
	}
	factor := 0.8 + 0.4*random()
	return time.Duration(float64(backoff) * factor)
}

func (r *Retrier) timeNow() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// parseRetryAfter parses a Retry-After header value in either RFC 7231 form:
// delay seconds ("120") or an HTTP-date ("Mon, 02 Jan 2006 15:04:05 GMT").
func parseRetryAfter(header string, now time.Time) (time.Duration, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(header); err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	if when, err := http.ParseTime(header); err == nil {
		delay := when.Sub(now)
		if delay < 0 {
			delay = 0
		}
		return delay, true
	}
	return 0, false
}

func (r *Retrier) RateLimitMessageDelay(message string) (time.Duration, bool) {
	return r.rateLimitMessageDelay(message)
}

func (r *Retrier) rateLimitMessageDelay(message string) (time.Duration, bool) {
	cap := r.MaxRateLimitDelay()
	if cap <= 0 {
		return 0, false
	}
	now := time.Now
	if r.now != nil {
		now = r.now
	}
	current := now()
	best := time.Duration(0)
	found := false
	for _, resetAt := range parseRateLimitResetTimes(message) {
		delay := resetAt.Sub(current)
		if delay > 0 && delay <= cap && (!found || delay < best) {
			// A 429 may carry several reset windows (e.g. per-minute and
			// per-day); retry at the soonest one rather than the first parsed.
			best = delay
			found = true
		}
	}
	return best, found
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
	regexp.MustCompile(`(?i)\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:\s*(?:UTC|GMT|Z|[+-]\d{2}:?\d{2}))`),
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
	times := parseRateLimitResetTimes(message)
	if len(times) == 0 {
		return time.Time{}, false
	}
	return times[0], true
}

func parseRateLimitResetTimes(message string) []time.Time {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	var out []time.Time
	for _, pattern := range rateLimitResetTimePatterns {
		for _, candidate := range pattern.FindAllString(message, -1) {
			if resetAt, ok := parseRateLimitResetTimeCandidate(candidate); ok {
				out = append(out, resetAt)
			}
		}
	}
	return out
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
