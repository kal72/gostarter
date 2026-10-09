package manifest_test

import (
	"errors"

	"strings"
	"testing"

	"gostarter/internal/upstream/manifest"
)

const pkg = `schema_version: 1
extensions:
  - id: helper
    kind: pkg
    title: Helper
    description: Generic helpers.
    path: pkg/helper
`

func code(t *testing.T, err error, want string) {
	t.Helper()
	var diagnostic *manifest.Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Code != want || diagnostic.File == "" || diagnostic.Cause == "" {
		t.Fatalf("want %s diagnostic, got %v", want, err)
	}
}

func TestRejectInvalidYAML(t *testing.T) {
	tests := []struct{ name, body, want string }{
		{"duplicate key", pkg + "extensions: []\n", "duplicate_key"},
		{"unknown root", pkg + "core: []\n", "unknown_field"},
		{"unknown extension", pkg + "    default_on: true\n", "unknown_field"},
		{"missing required", strings.Replace(pkg, "    title: Helper\n", "", 1), "missing_field"},
		{"wrong version type", strings.Replace(pkg, "schema_version: 1", "schema_version: '1'", 1), "schema_version"},
		{"float version", strings.Replace(pkg, "schema_version: 1", "schema_version: 1.0", 1), "schema_version"},
		{"bool version", strings.Replace(pkg, "schema_version: 1", "schema_version: true", 1), "schema_version"},
		{"unknown version", strings.Replace(pkg, "schema_version: 1", "schema_version: 2", 1), "schema_version"},
		{"multiple docs", pkg + "---\nschema_version: 1\nextensions: []", "multiple_documents"},
		{"custom tag", strings.Replace(pkg, "title: Helper", "title: !display Helper", 1), "yaml_tag"},
		{"anchor", strings.Replace(pkg, "title: Helper", "title: &display Helper", 1), "yaml_alias"},
		{"merge", "schema_version: 1\nextensions: []\n<<: {}", "yaml_merge"},
		{"null extensions", "schema_version: 1\nextensions: null", "field_type"},
		{"null requires", pkg + "    requires: null\n", "field_type"},
		{"boolean title", strings.Replace(pkg, "title: Helper", "title: true", 1), "field_type"},
		{"object key", "? [a, b]\n: 1", "field_type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { _, err := manifest.Parse([]byte(tc.body)); code(t, err, tc.want) })
	}
	_, err := manifest.Parse([]byte(strings.Repeat(" ", manifest.MaxBytes+1)))
	code(t, err, "manifest_too_large")
}
func TestSemanticFailures(t *testing.T) {
	valid := manifest.Extension{ID: "helper", Kind: "pkg", Title: "Helper", Description: "Generic helpers.", Path: "pkg/helper"}
	tests := []struct {
		name, want string
		change     func(*manifest.Manifest)
	}{
		{"duplicate ID", "duplicate_id", func(m *manifest.Manifest) { m.Extensions = append(m.Extensions, valid) }},
		{"duplicate path", "path_conflict", func(m *manifest.Manifest) { e := valid; e.ID = "other"; m.Extensions = append(m.Extensions, e) }},
		{"traversal", "invalid_path", func(m *manifest.Manifest) { m.Extensions[0].Path = "pkg/../helper" }},
		{"absolute", "invalid_path", func(m *manifest.Manifest) { m.Extensions[0].Path = "/pkg/helper" }},
		{"nested", "invalid_path", func(m *manifest.Manifest) { m.Extensions[0].Path = "pkg/helper/nested" }},
		{"wrong kind path", "invalid_path", func(m *manifest.Manifest) { m.Extensions[0].Kind = "feature" }},
		{"unknown kind", "invalid_kind", func(m *manifest.Manifest) { m.Extensions[0].Kind = "infrastructure" }},
		{"missing dep", "missing_dependency", func(m *manifest.Manifest) { m.Extensions[0].Requires = []string{"missing"} }},
		{"self cycle", "dependency_cycle", func(m *manifest.Manifest) { m.Extensions[0].Requires = []string{"helper"} }},
		{"cycle", "dependency_cycle", func(m *manifest.Manifest) {
			e := valid
			e.ID = "other"
			e.Path = "pkg/other"
			e.Requires = []string{"helper"}
			m.Extensions[0].Requires = []string{"other"}
			m.Extensions = append(m.Extensions, e)
		}},
		{"duplicate dep", "duplicate_dependency", func(m *manifest.Manifest) {
			e := valid
			e.ID = "other"
			e.Path = "pkg/other"
			m.Extensions = append(m.Extensions, e)
			m.Extensions[0].Requires = []string{"other", "other"}
		}},
		{"invalid id", "invalid_id", func(m *manifest.Manifest) { m.Extensions[0].ID = "Bad_ID" }},
		{"surrounding whitespace", "invalid_text", func(m *manifest.Manifest) { m.Extensions[0].Title = " Helper " }},
		{"multiline", "invalid_text", func(m *manifest.Manifest) { m.Extensions[0].Description = "line\nbreak" }},
		{"title length", "invalid_text", func(m *manifest.Manifest) { m.Extensions[0].Title = strings.Repeat("a", 81) }},
		{"desc length", "invalid_text", func(m *manifest.Manifest) { m.Extensions[0].Description = strings.Repeat("a", 161) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &manifest.Manifest{SchemaVersion: 1, Extensions: []manifest.Extension{valid}}
			tc.change(m)
			code(t, manifest.Validate(m), tc.want)
		})
	}

	m := &manifest.Manifest{SchemaVersion: 1, Extensions: []manifest.Extension{valid}}
	m.Extensions[0].Title = strings.Repeat("界", 80)
	if err := manifest.Validate(m); err != nil {
		t.Fatal(err)
	}
}
func FuzzParse(f *testing.F) {
	f.Add([]byte(pkg))
	f.Add([]byte("schema_version: 1\nextensions: []"))
	f.Add([]byte("---"))
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := manifest.Parse(data)
		if err == nil {
			if err := manifest.Validate(m); err != nil {
				t.Fatalf("parser returned invalid catalog: %v", err)
			}
		}
	})
}
