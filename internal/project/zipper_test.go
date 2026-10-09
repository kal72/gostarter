package project

import (
	"bytes"
	"errors"
	"testing"
)

type brokenZIPWriter struct{}

func (brokenZIPWriter) Write([]byte) (int, error) { return 0, errors.New("disconnected") }
func TestZIPFinalizationError(t *testing.T) {

	if err := CreateZipArchive(map[string][]byte{}, "app", brokenZIPWriter{}); err == nil {
		t.Fatal("lost ZIP finalization error")
	}
}
func TestZIPDeterminismAndPaths(t *testing.T) {
	files := map[string][]byte{"b.txt": []byte("b"), "a.sh": []byte("echo hi")}
	var a, b bytes.Buffer
	root := "app"
	if err := CreateZipArchive(files, root, &a); err != nil {
		t.Fatal(err)
	}
	if err := CreateZipArchive(files, root, &b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("ZIP bytes changed")
	}
	for _, name := range []string{"../escape", "/absolute", "a/../b", "a\\b", "."} {
		var out bytes.Buffer
		if err := CreateZipArchive(map[string][]byte{name: nil}, root, &out); err == nil || out.Len() != 0 {
			t.Fatalf("unsafe path accepted: %s", name)
		}
	}
}
