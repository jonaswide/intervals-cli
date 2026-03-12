package httpx

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

type RetryTransport struct {
	Base    http.RoundTripper
	Verbose bool
	Stderr  io.Writer
}

func (t RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if !isSafeMethod(req.Method) {
		return base.RoundTrip(req)
	}

	backoffs := []time.Duration{0, 200 * time.Millisecond, 500 * time.Millisecond, time.Second}
	var lastErr error
	for attempt, wait := range backoffs {
		if attempt > 0 {
			if t.Verbose && t.Stderr != nil {
				_, _ = fmt.Fprintf(t.Stderr, "retrying %s %s (attempt %d)\n", req.Method, req.URL.String(), attempt+1)
			}
			timer := time.NewTimer(wait)
			select {
			case <-req.Context().Done():
				timer.Stop()
				return nil, req.Context().Err()
			case <-timer.C:
			}
		}

		cloned := req.Clone(req.Context())
		resp, err := base.RoundTrip(cloned)
		if err != nil {
			lastErr = err
			continue
		}
		if !retryableStatus(resp.StatusCode) {
			return resp, nil
		}
		_ = resp.Body.Close()
		lastErr = fmt.Errorf("retryable status %d", resp.StatusCode)
	}
	return nil, lastErr
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func retryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
