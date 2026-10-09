// Package templates owns the embedded architecture source, independently of upstream.
package templates

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

const Version = "4"

//go:embed all:files
var files embed.FS

// Names lists the application-owned outputs in stable order.
func Names() []string {
	var names []string
	fs.WalkDir(files, "files", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(p, "files/"), ".tmpl"))
		}
		return nil
	})
	sort.Strings(names)
	return names
}

// Digest binds plan identity to actual embedded bytes, not merely a version label.
func Digest() string {
	bodies := map[string][]byte{}
	for _, name := range Names() {
		bodies[name], _ = files.ReadFile("files/" + name + ".tmpl")
	}
	data, _ := json.Marshal(struct {
		Bodies map[string][]byte
		Tools  map[string]string
	}{bodies, ToolVersions()})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Render does not read template bodies from the upstream snapshot.
func Render(name string, data any) ([]byte, error) {
	body, err := files.ReadFile("files/" + name + ".tmpl")
	if err != nil {
		return nil, err
	}
	t, err := template.New(name).Delims("<<%", "%>>").Option("missingkey=error").Funcs(template.FuncMap{"quote": strconv.Quote, "shellquote": func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }}).Parse(string(body))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// ToolVersions pins executable code generators separately from module requirements.
func ToolVersions() map[string]string {
	return map[string]string{
		"protoc":                  "libprotoc 34.0",
		"wire":                    "v0.7.0",
		"protoc-gen-go":           "v1.36.11",
		"protoc-gen-go-grpc":      "v1.6.2",
		"protoc-gen-grpc-gateway": "v2.29.0",
		"protoc-gen-openapiv2":    "v2.29.0",
	}
}
