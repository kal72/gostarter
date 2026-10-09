package source

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RateLimitError carries the earliest suggested manual retry time.
type RateLimitError struct {
	RetryAt time.Time
}

func (e *RateLimitError) Error() string {
	if e.RetryAt.IsZero() {
		return "GitHub API rate limit exceeded. Retry Sync later; GitHub did not provide a reset time."
	}
	return "GitHub API rate limit exceeded. Retry Sync after " + e.RetryAt.UTC().Format("2006-01-02 15:04:05 UTC") + "."
}

func githubResponseError(resp *http.Response, body []byte, now time.Time, token string) error {
	message := string(body)
	limited := resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode == http.StatusForbidden && (resp.Header.Get("X-RateLimit-Remaining") == "0" ||
			resp.Header.Get("Retry-After") != "" || strings.Contains(strings.ToLower(message), "rate limit")))
	if limited {
		var retryAt time.Time
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			if seconds, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil && seconds > 0 {
				retryAt = time.Unix(seconds, 0)
			}
		}
		if raw := resp.Header.Get("Retry-After"); raw != "" {
			var after time.Time
			if seconds, err := strconv.ParseInt(raw, 10, 32); err == nil && seconds >= 0 {
				after = now.Add(time.Duration(seconds) * time.Second)
			} else if date, err := http.ParseTime(raw); err == nil {
				after = date
			}
			if after.After(retryAt) {
				retryAt = after
			}
		}
		return &RateLimitError{RetryAt: retryAt}
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("GitHub authentication failed. Check the server GITHUB_TOKEN configuration")
	}
	if token != "" {
		message = strings.ReplaceAll(message, token, "[redacted]")
	}
	return fmt.Errorf("github api returned status %d: %s", resp.StatusCode, message)
}
