package source

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (fn roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }
func TestFetchPinsResolvedCommit(t *testing.T) {
	sha := strings.Repeat("a", 40)
	calls := 0
	f := NewFetcher("owner", "repo", "feature/test")
	f.httpClient = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		body := "archive"
		switch calls {
		case 1:
			if !strings.Contains(r.URL.EscapedPath(), "feature%2Ftest") {
				t.Fatal(r.URL)
			}
			body = fmt.Sprintf(`{"sha":%q,"commit":{"message":"first\nrest"}}`, sha)
		case 2:
			if r.URL.Path != "/owner/repo/archive/"+sha+".zip" {
				t.Fatal("branch archive downloaded:", r.URL)
			}
		default:
			t.Fatal("ref resolved more than once")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	info, data, err := f.Fetch(context.Background())
	if err != nil || info.SHA != sha || string(data) != "archive" || calls != 2 {
		t.Fatalf("fetch: %+v %v", info, err)
	}
	if _, err = f.DownloadCommit(context.Background(), "aaaaaaa"); err == nil {
		t.Fatal("accepted short SHA")
	}
}
func TestUnpackRejectsAmbiguousArchive(t *testing.T) {
	for _, names := range [][]string{{"../go.mod"}, {"repo/../go.mod"}, {"repo/go.mod", "repo/go.mod"}, {"repo/Foo", "repo/foo"}, {"repo/foo", "repo/foo/bar"}, {"repo/a", "other/b"}, {"repo\\a"}} {
		var b bytes.Buffer
		w := zip.NewWriter(&b)
		for _, name := range names {
			f, err := w.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			f.Write([]byte("x"))
		}
		w.Close()
		if _, err := UnpackZip(b.Bytes()); err == nil {
			t.Fatalf("accepted %v", names)
		}
	}
}

func TestFetchFailureDoesNotReturnSource(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusTooManyRequests, http.StatusBadGateway} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := NewFetcher("owner", "repo", "dev")
			calls := 0
			f.httpClient = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("upstream unavailable"))}, nil
			})}
			_, data, err := f.Fetch(context.Background())
			if err == nil || len(data) != 0 || calls != 1 {
				t.Fatal("failed upstream returned source or continued", err, calls)
			}
		})
	}
	f := NewFetcher("owner", "repo", "dev")
	f.httpClient = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
	})}
	if data, err := f.DownloadCommit(context.Background(), strings.Repeat("a", 40)); err == nil || len(data) != 0 {
		t.Fatal("failed archive returned source", err)
	}
}
