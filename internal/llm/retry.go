package llm

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

type Retrier struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	RetryableHTTP  map[int]struct{}
}

func NewRetrier() *Retrier {
	return &Retrier{
		MaxRetries:     3,
		InitialBackoff: time.Second,
		MaxBackoff:     30 * time.Second,
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

func (r *Retrier) Wait(ctx context.Context, attempt int, resp *http.Response) error {
	timer := time.NewTimer(r.Backoff(attempt, resp))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
