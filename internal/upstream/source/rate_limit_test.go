package source

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOptionalGitHubAuthentication(t *testing.T) {
	for _, token := range []string{"", "test-token"} {
		t.Run(token, func(t *testing.T) {
			f := NewFetcher("owner", "repo", "main", token)
			f.httpClient = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				want := ""
				if token != "" {
					want = "Bearer " + token
				}
				if r.Header.Get("Authorization") != want {
					t.Fatal("unexpected authorization")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"sha":"` + strings.Repeat("a", 40) + `"}`))}, nil
			})}
			if _, err := f.FetchCommitInfo(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGitHubRateLimitRetryTime(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	for _, tc := range []struct {
		name    string
		status  int
		headers http.Header
		body    string
		wait    time.Duration
	}{
		{"primary", 403, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"1800000120"}}, "", 2 * time.Minute},
		{"secondary", 429, http.Header{"Retry-After": {"60"}}, "", time.Minute},
		{"secondary forbidden", 403, http.Header{"Retry-After": {"60"}}, "", time.Minute},
		{"http date", 429, http.Header{"Retry-After": {now.Add(time.Minute).Format(http.TimeFormat)}}, "", time.Minute},
		{"both limits", 403, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"1800000120"}, "Retry-After": {"60"}}, "", 2 * time.Minute},
		{"missing reset", 403, nil, `{"message":"API rate limit exceeded"}`, 0},
		{"invalid reset", 429, http.Header{"X-Ratelimit-Reset": {"bad"}, "Retry-After": {"bad"}}, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := githubResponseError(&http.Response{StatusCode: tc.status, Header: tc.headers}, []byte(tc.body), now, "")
			var limited *RateLimitError
			if !errors.As(err, &limited) {
				t.Fatal(err)
			}
			if tc.wait == 0 {
				if !limited.RetryAt.IsZero() || !strings.Contains(err.Error(), "did not provide") {
					t.Fatal(err)
				}
			} else if !limited.RetryAt.Equal(now.Add(tc.wait)) || !strings.Contains(err.Error(), now.Add(tc.wait).Format("2006-01-02 15:04:05 UTC")) {
				t.Fatal(err)
			}
		})
	}
}

func TestGitHubNonRateLimitErrors(t *testing.T) {
	for _, status := range []int{401, 403, 500} {
		err := githubResponseError(&http.Response{StatusCode: status}, []byte("test-token denied"), time.Now(), "test-token")
		var limited *RateLimitError
		if errors.As(err, &limited) || strings.Contains(err.Error(), "test-token") {
			t.Fatal(err)
		}
		if status == 401 && !strings.Contains(err.Error(), "authentication failed") {
			t.Fatal(err)
		}
	}
}

func TestRateLimitedFetchStopsBeforeArchiveDownload(t *testing.T) {
	f := NewFetcher("owner", "repo", "main")
	calls := 0
	f.httpClient = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 403, Header: http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"1800000120"}}, Body: io.NopCloser(strings.NewReader("rate limit exceeded"))}, nil
	})}
	_, _, err := f.Fetch(context.Background())
	var limited *RateLimitError
	if calls != 1 || !errors.As(err, &limited) {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
