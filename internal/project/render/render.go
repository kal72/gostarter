// Package render composes validated source and application-owned core templates.
// It is a development pipeline until the full feature matrix and worker are ready.
package render

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"gostarter/internal/project/plan"
	"gostarter/internal/project/templates"
	"gostarter/internal/upstream/profile"
)

// Assemble produces detached source bytes, never executing snapshot scripts.
// Codegen and build are separate explicit operations.
func Assemble(p *plan.Plan, source profile.Source) (profile.Source, error) {
	if p == nil {
		return nil, fmt.Errorf("render: missing plan")
	}
	if err := p.Verify(); err != nil {
		return nil, err
	}
	canonical, err := plan.Build(source, p.Snapshot, p.Config)
	if err != nil {
		return nil, err
	}
	if canonical.Fingerprint != p.Fingerprint {
		return nil, fmt.Errorf("render: plan/source/template/profile mismatch; resolve again")
	}
	if p.Config.Protocol != "dual" && p.Config.Protocol != "grpc" {
		return nil, fmt.Errorf("render: unsupported transport")
	}
	mod, err := modfile.Parse("go.mod", source["go.mod"], nil)
	if err != nil {
		return nil, err
	}
	data, err := composition(p, source)
	if err != nil {
		return nil, err
	}
	out := profile.Source{}
	for _, f := range p.Files {
		var body []byte
		switch f.Action {
		case profile.Exclude, profile.Generate:
			continue
		case profile.Render:
			body, err = templates.Render(f.Path, data)
		case profile.Copy, profile.Shared:
			var ok bool
			body, ok = source[f.Path]
			if !ok {
				return nil, fmt.Errorf("render: missing source %s", f.Path)
			}
			body = bytes.Clone(body)
		default:
			return nil, fmt.Errorf("render: unsupported producer %s", f.Action)
		}
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", f.Path, err)
		}
		if strings.HasSuffix(f.Path, ".go") {
			body, err = rewriteImports(f.Path, body, mod.Module.Mod.Path, p.Config.Module)
			if err != nil {
				return nil, err
			}
		}
		if strings.HasSuffix(f.Path, ".proto") && strings.HasPrefix(f.Path, "api/proto/") && !strings.HasPrefix(f.Path, "api/proto/plugins/") && f.Path != "api/proto/openapi.proto" {
			body, err = rewriteProtoPackage(f.Path, body, p.Config.Module)
			if err != nil {
				return nil, err
			}
		}
		if f.Path == "go.mod" {
			// Requirements and checksums come from the validated upstream version registry.
			// Tidy prunes requirements against the actual composed tree after codegen.
			dependencies, parseErr := modfile.Parse("go.mod", source["go.mod"], nil)
			if parseErr != nil {
				return nil, parseErr
			}
			if err = dependencies.AddModuleStmt(p.Config.Module); err != nil {
				return nil, err
			}
			if err = dependencies.AddGoStmt(p.Config.GoVersion); err != nil {
				return nil, err
			}
			body, err = dependencies.Format()
			if err != nil {
				return nil, err
			}
		}
		if f.Path == "go.sum" {
			body = bytes.Clone(source["go.sum"])
		}

		if !validOutput(f.Path) {
			return nil, fmt.Errorf("render: unsafe output %s", f.Path)
		}
		out[f.Path] = body
	}
	return out, nil
}

func validOutput(name string) bool {
	return name != "." && !strings.Contains(name, "\\") && !strings.HasPrefix(name, "/") && path.Clean(name) == name && name != ".." && !strings.HasPrefix(name, "../")
}

// rewriteImports changes import literals through the Go AST, preserving unrelated strings/comments.
func rewriteImports(name string, body []byte, from, to string) ([]byte, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, name, body, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("render parse %s: %w", name, err)
	}
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		if value == from || strings.HasPrefix(value, from+"/") {
			imp.Path.Value = strconv.Quote(to + strings.TrimPrefix(value, from))
		}
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, set, file); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
