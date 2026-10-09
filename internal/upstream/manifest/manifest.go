// Package manifest parses the upstream catalog without executing upstream code.
package manifest

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// FileName is the only supported catalog location at the source root.
const FileName = "gostarter.yaml"

// MaxBytes bounds parser memory use for an upstream catalog.
const MaxBytes = 1 << 20

// Diagnostic is a stable, serializable contract error shared with source validation.
type Diagnostic struct {
	Code        string `json:"code"`
	ExtensionID string `json:"extension_id,omitempty"`
	File        string `json:"file"`
	Symbol      string `json:"symbol,omitempty"`
	Cause       string `json:"cause"`
	Line        int    `json:"line,omitempty"`
}

// Error returns the structured diagnostic as JSON.
func (d *Diagnostic) Error() string { b, _ := json.Marshal(d); return string(b) }

// Manifest contains only the upstream-owned optional catalog.
type Manifest struct {
	SchemaVersion int         `json:"schema_version"`
	Extensions    []Extension `json:"extensions"`
}

// Extension identifies source and its direct non-default dependencies.
type Extension struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Path        string   `json:"path"`
	Requires    []string `json:"requires,omitempty"`
}

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var pathPattern = regexp.MustCompile(`^(internal|pkg)/[a-z][a-z0-9_]*$`)

func failure(code, id, symbol, cause string, line int) error {
	return &Diagnostic{Code: code, ExtensionID: id, File: FileName, Symbol: symbol, Cause: cause, Line: line}
}

// Parse rejects ambiguous YAML and validates all structural and graph rules of v1.
func Parse(data []byte) (*Manifest, error) {
	if len(data) > MaxBytes {
		return nil, failure("manifest_too_large", "", "", "manifest exceeds 1 MiB", 0)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := decoder.Decode(&doc); err != nil {
		return nil, failure("yaml_syntax", "", "", err.Error(), 0)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, failure("multiple_documents", "", "", "exactly one YAML document is required", 0)
	}
	if err := checkNodes(&doc); err != nil {
		return nil, err
	}
	if len(doc.Content) != 1 {
		return nil, failure("missing_field", "", "", "manifest root is required", 0)
	}
	root, err := fields(doc.Content[0], []string{"schema_version", "extensions"}, []string{"schema_version", "extensions"}, "")
	if err != nil {
		return nil, err
	}
	version := root["schema_version"]
	var number int
	if version.Tag != "!!int" || version.Decode(&number) != nil || number != 1 {
		return nil, failure("schema_version", "", "schema_version", "expected integer 1", version.Line)
	}
	list := root["extensions"]
	if list.Kind != yaml.SequenceNode {
		return nil, failure("field_type", "", "extensions", "expected array", list.Line)
	}
	result := &Manifest{SchemaVersion: 1, Extensions: []Extension{}}
	for _, node := range list.Content {
		f, err := fields(node, []string{"id", "kind", "title", "description", "path", "requires"}, []string{"id", "kind", "title", "description", "path"}, "")
		if err != nil {
			return nil, err
		}
		e := Extension{}
		values := []struct {
			key  string
			dest *string
		}{{"id", &e.ID}, {"kind", &e.Kind}, {"title", &e.Title}, {"description", &e.Description}, {"path", &e.Path}}
		for _, value := range values {
			n := f[value.key]
			if n.Kind != yaml.ScalarNode || n.Tag != "!!str" {
				return nil, failure("field_type", e.ID, value.key, "expected string", n.Line)
			}
			*value.dest = n.Value
		}
		if n, ok := f["requires"]; ok {
			if n.Kind != yaml.SequenceNode {
				return nil, failure("field_type", e.ID, "requires", "expected array", n.Line)
			}
			for _, dep := range n.Content {
				if dep.Kind != yaml.ScalarNode || dep.Tag != "!!str" {
					return nil, failure("field_type", e.ID, "requires", "expected string IDs", dep.Line)
				}
				e.Requires = append(e.Requires, dep.Value)
			}
		}
		result.Extensions = append(result.Extensions, e)
	}
	if err := Validate(result); err != nil {
		return nil, err
	}
	return result, nil
}

func checkNodes(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return failure("yaml_alias", "", "", "anchors and aliases are not supported", node.Line)
	}
	if node.Tag == "!!merge" {
		return failure("yaml_merge", "", "", "merge keys are not supported", node.Line)
	}
	if node.Tag != "" && !strings.HasPrefix(node.Tag, "!!") {
		return failure("yaml_tag", "", "", "custom tags are not supported", node.Line)
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Tag == "!!merge" {
				return failure("yaml_merge", "", "", "merge keys are not supported", key.Line)
			}
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return failure("field_type", "", "", "mapping keys must be strings", key.Line)
			}
			if seen[key.Value] {
				return failure("duplicate_key", "", key.Value, "duplicate YAML key", key.Line)
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := checkNodes(child); err != nil {
			return err
		}
	}
	return nil
}
func fields(node *yaml.Node, allowed, required []string, id string) (map[string]*yaml.Node, error) {
	if node.Kind != yaml.MappingNode {
		return nil, failure("field_type", id, "", "expected object", node.Line)
	}
	valid := map[string]bool{}
	for _, key := range allowed {
		valid[key] = true
	}
	result := map[string]*yaml.Node{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		if !valid[key.Value] {
			return nil, failure("unknown_field", id, key.Value, "field is not part of schema v1", key.Line)
		}
		result[key.Value] = node.Content[i+1]
	}
	for _, key := range required {
		if _, ok := result[key]; !ok {
			return nil, failure("missing_field", id, key, "required field is missing", node.Line)
		}
	}
	return result, nil
}

// Validate also protects callers constructing a Manifest without the YAML parser.
func Validate(m *Manifest) error {
	if m == nil || m.SchemaVersion != 1 {
		return failure("schema_version", "", "schema_version", "expected integer 1", 0)
	}
	ids := map[string]Extension{}
	paths := map[string]string{}
	for _, e := range m.Extensions {
		if !idPattern.MatchString(e.ID) {
			return failure("invalid_id", e.ID, "id", "expected lowercase ID, at most 64 characters", 0)
		}
		if _, ok := ids[e.ID]; ok {
			return failure("duplicate_id", e.ID, "id", "ID already declared", 0)
		}
		if e.Kind != "feature" && e.Kind != "pkg" {
			return failure("invalid_kind", e.ID, "kind", "expected feature or pkg", 0)
		}
		prefix := "pkg/"
		if e.Kind == "feature" {
			prefix = "internal/"
		}
		if !pathPattern.MatchString(e.Path) || !strings.HasPrefix(e.Path, prefix) {
			return failure("invalid_path", e.ID, "path", "expected direct source directory matching kind; no traversal or absolute path", 0)
		}
		if owner, ok := paths[e.Path]; ok {
			return failure("path_conflict", e.ID, "path", "path already owned by "+owner, 0)
		}
		for _, text := range []struct {
			name, value string
			max         int
		}{{"title", e.Title, 80}, {"description", e.Description, 160}} {
			if !utf8.ValidString(text.value) || text.value == "" || strings.TrimSpace(text.value) != text.value || strings.ContainsAny(text.value, "\r\n") || utf8.RuneCountInString(text.value) > text.max {
				return failure("invalid_text", e.ID, text.name, "expected nonempty single-line text within length limit, without surrounding whitespace", 0)
			}
		}
		paths[e.Path] = e.ID
		ids[e.ID] = e
	}
	for _, e := range m.Extensions {
		seen := map[string]bool{}
		for _, dep := range e.Requires {
			if !idPattern.MatchString(dep) {
				return failure("invalid_id", e.ID, "requires", "invalid dependency ID "+dep, 0)
			}
			if seen[dep] {
				return failure("duplicate_dependency", e.ID, "requires", "duplicate dependency "+dep, 0)
			}
			if _, ok := ids[dep]; !ok {
				return failure("missing_dependency", e.ID, "requires", "unknown optional dependency "+dep, 0)
			}
			if dep == e.ID {
				return failure("dependency_cycle", e.ID, "requires", "self dependency", 0)
			}
			seen[dep] = true
		}
	}
	state := map[string]int{}
	var visit func(string, []string) error
	visit = func(id string, chain []string) error {
		if state[id] == 1 {
			return failure("dependency_cycle", id, "requires", strings.Join(append(chain, id), " -> "), 0)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, dep := range ids[id].Requires {
			if err := visit(dep, append(chain, id)); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for _, e := range m.Extensions {
		if err := visit(e.ID, nil); err != nil {
			return err
		}
	}
	return nil
}

// Lookup finds an extension without treating undeclared source as an optional component.
func (m *Manifest) Lookup(id string) (Extension, bool) {
	for _, e := range m.Extensions {
		if e.ID == id {
			return e, true
		}
	}
	return Extension{}, false
}

// NewDiagnostic constructs source/profile errors in the same format as parser errors.
func NewDiagnostic(code, id, file, symbol, cause string) *Diagnostic {
	return &Diagnostic{Code: code, ExtensionID: id, File: file, Symbol: symbol, Cause: cause}
}

var _ error = (*Diagnostic)(nil)
