package source_test

import (
	"archive/zip"
	"bytes"
	"testing"

	"gostarter/internal/upstream/source"
)

func createTestZip(t *testing.T) []byte {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	files := map[string]string{
		"repo-main/go.mod":                 "module github.com/example/test\n\ngo 1.24\n",
		"repo-main/cmd/api/main.go":        "package main\n\nfunc main() {}\n",
		"repo-main/bootstrap/app.go":       "package bootstrap\n",
		"repo-main/Makefile":               "build:\n\tgo build\n",
		"repo-main/deployments/Dockerfile": "FROM golang:1.24\n",
	}

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed creating zip entry: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed writing zip entry content: %v", err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed closing zip writer: %v", err)
	}

	return buf.Bytes()
}

func TestUnpackZip(t *testing.T) {
	zipData := createTestZip(t)

	files, err := source.UnpackZip(zipData)
	if err != nil {
		t.Fatalf("failed unpacking zip: %v", err)
	}

	if len(files) != 5 {
		t.Fatalf("expected 5 unpacked files, got %d", len(files))
	}

	if _, ok := files["go.mod"]; !ok {
		t.Errorf("missing go.mod in unpacked files")
	}

}
