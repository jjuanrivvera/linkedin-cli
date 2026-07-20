package api

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"
)

// retryBase is the backoff unit for full-jitter waits: random(0, retryBase·2^attempt).
var retryBase = 500 * time.Millisecond

// sendWithRetry runs send with a SMALL, ban-aware retry budget. Unlike a generic REST client it
// deliberately does NOT retry LinkedIn's throttle signals: 429 (rate limited), 999 (soft-block),
// and challenge responses are surfaced immediately — retry-hammering them is exactly what gets an
// account flagged (DECISIONS.md). Only genuine transient failures retry: 5xx server errors (not
// 999) and transient network errors, on idempotent GETs, with full-jitter backoff honoring
// Retry-After.
func (c *Client) sendWithRetry(ctx context.Context, method string, send func() (*http.Response, error)) (*http.Response, error) {
	retries := c.maxRetries
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
		retries = 0 // never auto-retry a non-idempotent method
	}
	var resp *http.Response
	var err error
	for attempt := 0; ; attempt++ {
		resp, err = send()
		if err != nil {
			if attempt >= retries || !isTransient(err) {
				return nil, err
			}
		} else if !isRetryableStatus(resp.StatusCode) {
			return resp, nil
		} else if attempt >= retries {
			return resp, nil
		}

		var wait time.Duration
		if resp != nil {
			wait = retryAfter(resp.Header)
			_ = resp.Body.Close()
		}
		if wait == 0 {
			// Full jitter (deliberate design, not a bug): random(0, base·2^n).
			wait = time.Duration(rand.Int63n(int64(retryBase) << attempt)) // #nosec G404 -- non-crypto jitter
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// isRetryableStatus is TRUE only for genuine transient server errors. 429 and 999 are LinkedIn
// throttle signals and are NEVER retried (see the function comment above).
func isRetryableStatus(status int) bool {
	if status == http.StatusTooManyRequests || status == StatusSoftBlock {
		return false
	}
	return status >= 500 && status < 600
}

// retryAfter parses the Retry-After header: delta-seconds first, then HTTP-date.
func retryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// isTransient reports whether a network error is worth retrying. Context cancellation is never
// transient.
func isTransient(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}
