package render

import (
	"bytes"

	"os"
	"path/filepath"

	"testing"

	"gostarter/internal/upstream/profile"
)

func TestImportRewritePreservesLiterals(t *testing.T) {
	body := []byte("package p\nimport x \"old.example/module/pkg\"\n// old.example/module must stay\nvar s = \"old.example/module\"\nvar _ = x.X\n")
	got, err := rewriteImports("x.go", body, "old.example/module", "new.example/app")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(`"new.example/app/pkg"`)) || !bytes.Contains(got, []byte(`var s = "old.example/module"`)) || !bytes.Contains(got, []byte("// old.example/module must stay")) {
		t.Fatal(string(got))
	}
}

func TestWriteNewSafety(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "project")
	if err := WriteNew(dest, profile.Source{"../escape": []byte("bad")}); err == nil {
		t.Fatal("accepted traversal")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("created directory for invalid tree")
	}
	if err := WriteNew(dest, profile.Source{"a/b.go": []byte("package b")}); err != nil {
		t.Fatal(err)
	}
	if err := WriteNew(dest, profile.Source{"a/b.go": []byte("overwrite")}); err == nil {
		t.Fatal("overwrote existing output")
	}
	body, err := os.ReadFile(filepath.Join(dest, "a/b.go"))
	if err != nil || string(body) != "package b" {
		t.Fatal("existing file changed")
	}
}

func TestBoundedCommandOutput(t *testing.T) {
	var output limitedOutput
	data := bytes.Repeat([]byte("x"), 20000)
	if n, err := output.Write(data); err != nil || n != len(data) {
		t.Fatal("must drain all input")
	}
	output.Write(data)
	if output.Len() != 8192 {
		t.Fatal(output.Len())
	}
}
