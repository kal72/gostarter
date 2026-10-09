package storage

import (
	"gostarter/internal/config"

	"strings"

	"testing"

	"unicode/utf8"
)

func TestSummaryBoundedUTF8(t *testing.T) {
	s := Service{policy: config.StorageConfig{DiagnosticBytes: 256}}
	text := strings.Repeat("あ", 100)
	got := s.Summary(text)
	if len(got) > 256 || !utf8.ValidString(got) {
		t.Fatal("summary exceeds byte budget or splits UTF-8")
	}
	if !utf8.ValidString(s.Summary("bad\xffoutput")) {
		t.Fatal("diagnostic retains invalid UTF-8")
	}
	if s.Summary("short") != "short" {
		t.Fatal("short diagnostic changed")
	}
}

func TestSummaryRedactsGitHubToken(t *testing.T) {
	s := Service{policy: config.StorageConfig{DiagnosticBytes: 256}, githubToken: "secret-token"}
	if got := s.Summary("request failed: secret-token"); strings.Contains(got, "secret-token") || !strings.Contains(got, "[redacted]") {
		t.Fatal("token was not redacted")
	}
}
