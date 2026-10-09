package render

import (
	"os"
	"path/filepath"

	"testing"

	"gostarter/internal/project/plan"
	"gostarter/internal/upstream/profile"
)

func TestStrictTreeAndWriteRollback(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "project")
	if err := WriteNew(dir, profile.Source{"a": []byte("file"), "a/b.go": []byte("package b")}); err == nil {
		t.Fatal("accepted file/directory collision")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("partial output leaked")
	}
	if err := WriteNew(dir, profile.Source{"README.md": []byte("a"), "readme.md": []byte("b")}); err == nil {
		t.Fatal("accepted case collision")
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside"), filepath.Join(dir, "symlink")); err != nil {
		t.Fatal(err)
	}
	if _, err := readTree(dir); err == nil {
		t.Fatal("symlink ignored")
	}
}

func TestArtifactTamperDetection(t *testing.T) {
	files := profile.Source{"README.md": []byte("test")}
	receipt := Receipt{ContentSHA256: plan.ContentDigest(files), Files: []OutputFile{{Path: "README.md", Owner: "application", Action: profile.Render, Reason: "application_producer", SHA256: hashBytes(files["README.md"]), Mode: 0644}}}
	receipt.IdentitySHA256 = receipt.identityDigest()
	a := Artifact{Files: files, Receipt: receipt}
	if err := a.Verify(); err != nil {
		t.Fatal(err)
	}
	a.Files["README.md"][0] = 'x'
	if err := a.Verify(); err == nil {
		t.Fatal("artifact edit undetected")
	}
	a.Files["README.md"][0] = 't'
	a.Receipt.Files[0].Owner = "changed"
	if err := a.Verify(); err == nil {
		t.Fatal("receipt edit undetected")
	}
}
