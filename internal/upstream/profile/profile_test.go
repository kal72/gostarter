package profile_test

import (
	"errors"
	"os"
	"path/filepath"

	"testing"
	"testing/fstest"

	"gostarter/internal/project/templates"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
)

func assertCode(t *testing.T, err error, want string) {
	t.Helper()
	var d *manifest.Diagnostic
	if !errors.As(err, &d) || d.Code != want || d.Cause == "" {
		t.Fatalf("want %s, got %v", want, err)
	}
}
func TestReadSourceRejectsSymlinksAndOmitsSecrets(t *testing.T) {
	source, err := profile.ReadSource(fstest.MapFS{".env": {Data: []byte("secret")}, ".env.production": {Data: []byte("secret")}, ".env.example": {Data: []byte("example")}, "bin/server": {Data: []byte("binary")}, "go.mod": {Data: []byte("module test")}})
	if err != nil || len(source) != 2 {
		t.Fatalf("local artifacts not excluded: %+v, %v", source, err)
	}
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	_, err = profile.ReadSource(os.DirFS(root))
	assertCode(t, err, "unsafe_source_path")
}

func TestProfileTemplateOutputsMatchEmbeddedTemplates(t *testing.T) {
	p, err := profile.Default(&manifest.Manifest{SchemaVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{}
	for _, name := range templates.Names() {
		expected[name] = true
	}
	for _, tpl := range p.Templates {
		if tpl.Action != profile.Render {
			continue
		}
		if !expected[tpl.Output] {
			t.Errorf("profile references missing embedded template: %s", tpl.Output)
		}
		delete(expected, tpl.Output)
	}
	for name := range expected {
		t.Errorf("embedded template has no profile producer: %s", name)
	}
	rule, err := p.Classify("infrastructure/database/postgres/db.go")
	if err != nil || rule.Action != profile.Render {
		t.Fatalf("upstream PostgreSQL path is not owned: %v", err)
	}
}
