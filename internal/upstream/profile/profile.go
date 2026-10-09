// Package profile defines application-owned architecture and integration contracts.
package profile

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gostarter/internal/project/templates"
	"gostarter/internal/upstream/manifest"
)

// Action defines the sole producer of an output path.
type Action string

const (
	Copy     Action = "copy_upstream"
	Render   Action = "render_application_template"
	Generate Action = "generate_from_selected_source"
	Shared   Action = "copy_shared_source"
	Exclude  Action = "exclude"
)

// Rule assigns one exact file or subtree to one component/producer.
type Rule struct {
	Path    string `json:"path"`
	Subtree bool   `json:"subtree"`
	Owner   string `json:"owner"`
	Action  Action `json:"action"`
}

// Symbol defines an expected upstream declaration used by the application adapter.
type Symbol struct {
	File string `json:"file"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// FeatureIntegration follows the ProviderSet and ServiceRegister conventions.
type FeatureIntegration struct {
	ID            string   `json:"id"`
	SourcePath    string   `json:"source_path"`
	Provider      Symbol   `json:"provider"`
	Constructor   Symbol   `json:"constructor"`
	Handler       string   `json:"handler"`
	ProtoPath     string   `json:"proto_path"`
	GeneratedPath string   `json:"generated_path"`
	Migrations    []string `json:"migrations,omitempty"`
}

// Resource describes default DI construction and ordered shutdown responsibility.
type Resource struct {
	ID          string   `json:"id"`
	Provider    Symbol   `json:"provider"`
	Constructor Symbol   `json:"constructor"`
	Shutdown    []string `json:"shutdown"`
}

// Template records the versioned application-owned output contract.
type Template struct {
	Output   string `json:"output"`
	Action   Action `json:"action"`
	Revision string `json:"revision"`
}

// Compatibility adds application-owned limits for a particular optional extension.
type Compatibility struct {
	MinGo     string   `json:"min_go,omitempty"`
	Protocols []string `json:"protocols,omitempty"`
}

// Profile versions architecture assumptions independently of manifest and source commits.
type Profile struct {
	ArchitectureID         string                   `json:"architecture_id"`
	Revision               string                   `json:"revision"`
	TemplateVersion        string                   `json:"template_version"`
	MinGo                  string                   `json:"min_go"`
	Protocols              []string                 `json:"protocols"`
	Defaults               []string                 `json:"defaults"`
	Rules                  []Rule                   `json:"rules"`
	Features               []FeatureIntegration     `json:"features"`
	Resources              []Resource               `json:"resources"`
	ConfigSymbols          []Symbol                 `json:"config_symbols"`
	Templates              []Template               `json:"templates"`
	Conflicts              map[string][]string      `json:"conflicts"`
	ExtensionCompatibility map[string]Compatibility `json:"extension_compatibility,omitempty"`
}

// Default builds the initial architecture registry entry without changing the upstream catalog.
func Default(m *manifest.Manifest) (*Profile, error) {
	if err := manifest.Validate(m); err != nil {
		return nil, err
	}
	p := &Profile{ArchitectureID: "go-feature-based-clean", Revision: "1", TemplateVersion: templates.Version, MinGo: "1.27.1", Protocols: []string{"dual", "grpc"}, Defaults: []string{"internal/health", "pkg/contextutil", "pkg/errorutil", "pkg/jwtutil", "pkg/logger", "pkg/routine", "pkg/transaction"}, Conflicts: map[string][]string{}}
	add := func(name, owner string, action Action, tree bool) {
		p.Rules = append(p.Rules, Rule{name, tree, owner, action})
	}
	for _, root := range p.Defaults {
		add(root, "core:"+path.Base(root), Copy, true)
	}
	add("api/proto/health", "core:health", Copy, true)
	add("gen/pb/health", "core:health", Generate, true)
	// New conventional features are discovered from their directory, never from hardcoded UI IDs.
	for _, e := range m.Extensions {
		for _, root := range p.Defaults {
			if e.Path == root || e.ID == path.Base(root) {
				return nil, diag("default_conflict", e.ID, e.Path, "", "default components cannot be optional extensions")
			}
		}
		add(e.Path, e.ID, Copy, true)
		if e.Kind == "feature" {
			name := path.Base(e.Path)
			feature := FeatureIntegration{ID: e.ID, SourcePath: e.Path, Provider: Symbol{e.Path + "/provider.go", "ProviderSet", "wire_set"}, Constructor: Symbol{e.Path + "/handler/handler.go", "NewHandler", "constructor"}, Handler: e.Path + "/handler", ProtoPath: "api/proto/" + name, GeneratedPath: "gen/pb/" + name}
			switch e.Path {
			case "internal/user":
				feature.Migrations = []string{"migrations/000001_create_users_table.up.sql", "migrations/000001_create_users_table.down.sql"}
			case "internal/auth":
				feature.Migrations = []string{"migrations/000002_create_refresh_tokens_table.up.sql", "migrations/000002_create_refresh_tokens_table.down.sql"}
			}
			p.Features = append(p.Features, feature)
			add(feature.ProtoPath, e.ID, Copy, true)
			add(feature.GeneratedPath, e.ID, Generate, true)
			for _, file := range feature.Migrations {
				add(file, e.ID, Copy, false)
			}
		}
	}
	for _, name := range []string{"api/proto/plugins/google/api", "gen/pb/google/api"} {
		add(name, "shared:annotations", Shared, true)
	}
	// Explicit output registry: source content is a reference, not a template body.
	render := []string{
		".env.example", "go.mod", "go.sum", "Makefile", ".gitignore", "README.md",
		"bootstrap/app.go", "bootstrap/gateway.go", "bootstrap/grpc.go", "bootstrap/provider.go", "bootstrap/services.go", "bootstrap/wire.go", "cmd/api/main.go",
		"infrastructure/config/config.go", "infrastructure/config/provider.go",
		"infrastructure/database/postgres/db.go", "infrastructure/database/postgres/provider.go", "infrastructure/database/postgres/tx_manager.go",
		"infrastructure/middleware/auth_interceptor.go", "infrastructure/middleware/error_interceptor.go", "infrastructure/middleware/logging_interceptor.go", "infrastructure/middleware/recovery_interceptor.go", "infrastructure/middleware/timeout_interceptor.go",
		"infrastructure/swagger/swagger.go", "infrastructure/swagger/static/index.html",
		"infrastructure/telemetry/metrics.go", "infrastructure/telemetry/provider.go", "infrastructure/telemetry/tracer.go",
		"api/proto/openapi.proto", "gen/openapi/openapi.go",
		"deployments/Dockerfile", "deployments/docker-compose.yml", "scripts/protogen.sh", "scripts/protogen.bat", "scripts/new-feature.sh", "scripts/rename-module.sh", "scripts/rename-module.bat",
	}
	for _, file := range render {
		add(file, "application", Render, false)
		p.Templates = append(p.Templates, Template{file, Render, p.TemplateVersion})
	}
	for _, file := range []string{"bootstrap/wire_gen.go", "gen/pb/openapi.pb.go", "gen/openapi/api.swagger.json"} {
		add(file, "application", Generate, false)
		p.Templates = append(p.Templates, Template{file, Generate, p.TemplateVersion})
	}
	// Upstream documentation is shared output for every extension/transport selection.
	add("docs", "shared:documentation", Shared, true)
	// Optional infrastructure and examples do not become pkg entries or mandatory services.
	for _, root := range []string{"infrastructure/cache/redis", "infrastructure/database/mongo", "infrastructure/external/company-service", "api/external/company-service", "gen/externalpb/company-service", "tests"} {
		add(root, "excluded:unsupported_or_reference", Exclude, true)
	}
	for _, file := range []string{"AGENTS.md", "gostarter.yaml", "gen/.keep", "bootstrap/gateway_test.go", "infrastructure/config/config_test.go", "infrastructure/database/postgres/tx_manager_test.go", "infrastructure/middleware/auth_interceptor_test.go", "infrastructure/middleware/error_interceptor_test.go", "infrastructure/middleware/logging_interceptor_test.go", "infrastructure/middleware/timeout_interceptor_test.go"} {
		add(file, "excluded:repository_or_template_test", Exclude, false)
	}
	p.ConfigSymbols = []Symbol{{"infrastructure/config/config.go", "Config", "type"}, {"infrastructure/config/config.go", "Load", "function"}, {"bootstrap/services.go", "ServiceRegister", "type"}, {"bootstrap/services.go", "ProvideServices", "function"}, {"bootstrap/wire.go", "InitializeApp", "function"}, {"bootstrap/grpc.go", "NewGRPCServer", "function"}, {"bootstrap/gateway.go", "NewHTTPGateway", "function"}, {"bootstrap/app.go", "NewApp", "function"}}
	p.Resources = []Resource{
		{"config", Symbol{"infrastructure/config/provider.go", "ProviderSet", "wire_set"}, Symbol{"infrastructure/config/config.go", "Load", "function"}, nil},
		{"postgres", Symbol{"infrastructure/database/postgres/provider.go", "ProviderSet", "wire_set"}, Symbol{"infrastructure/database/postgres/db.go", "New", "function"}, []string{"DB", "Close"}},
		{"telemetry", Symbol{"infrastructure/telemetry/provider.go", "ProviderSet", "wire_set"}, Symbol{"infrastructure/telemetry/provider.go", "NewTracerFromConfig", "function"}, []string{"Shutdown"}},
		{"logger", Symbol{"bootstrap/provider.go", "ProvideLogger", "function"}, Symbol{"pkg/logger/zap.go", "New", "function"}, []string{"Sync"}},
	}
	if err := p.CheckRules(); err != nil {
		return nil, err
	}
	return p, nil
}

// Registry resolves an architecture revision; it never falls back on unknown versions.
func Registry(id, revision string, m *manifest.Manifest) (*Profile, error) {
	if id != "go-feature-based-clean" || revision != "1" {
		return nil, diag("unknown_profile", "", "", "", fmt.Sprintf("unsupported architecture %s/%s", id, revision))
	}
	return Default(m)
}
func matches(r Rule, file string) bool {
	return file == r.Path || r.Subtree && strings.HasPrefix(file, r.Path+"/")
}

// CheckRules rejects overlapping producers before any file can be materialized.
func (p *Profile) CheckRules() error {
	for i, a := range p.Rules {
		if !fs.ValidPath(a.Path) || strings.Contains(a.Path, "\\") || a.Owner == "" {
			return diag("invalid_ownership", "", a.Path, "", "ownership requires a clean relative path and named owner")
		}
		switch a.Action {
		case Copy, Render, Generate, Shared, Exclude:
		default:
			return diag("invalid_ownership", a.Owner, a.Path, "", "unknown file action")
		}
		for _, b := range p.Rules[i+1:] {
			if matches(a, b.Path) || matches(b, a.Path) {
				return diag("ownership_conflict", b.Owner, b.Path, "", a.Owner+" and "+b.Owner+" both claim this path")
			}
		}
	}
	return nil
}

// Classify refuses unrecognized files rather than guessing they are core.
func (p *Profile) Classify(file string) (Rule, error) {
	for _, r := range p.Rules {
		if matches(r, file) {
			return r, nil
		}
	}
	return Rule{}, diag("unclassified_file", "", file, "", "no ownership rule in architecture revision "+p.Revision)
}

// CheckCompatibility validates the transport and source language floor; build validation comes later.
func (p *Profile) CheckCompatibility(goVersion, transport string) error {
	if !validGoVersion(goVersion, p.MinGo) {
		return diag("incompatible_go", "", "go.mod", "go", "requires Go >= "+p.MinGo)
	}
	for _, supported := range p.Protocols {
		if transport == supported {
			return nil
		}
	}
	return diag("incompatible_protocol", "", "", "protocol", "current profile supports dual and grpc; REST-only requires a separate upstream profile")
}

// SortedRules returns a copy for deterministic reports.
func (p *Profile) SortedRules() []Rule {
	out := append([]Rule(nil), p.Rules...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
func diag(code, id, file, symbol, cause string) error {
	return manifest.NewDiagnostic(code, id, file, symbol, cause)
}

// TransportExcludes defines application assets absent from gRPC-only output.
// Upstream handlers retain their gateway adapter methods, but no HTTP server is constructed.
func TransportExcludes(name, transport string) bool {
	if transport != "grpc" {
		return false
	}
	return name == "bootstrap/gateway.go" || name == "api/proto/openapi.proto" || name == "gen/pb/openapi.pb.go" || strings.HasPrefix(name, "gen/openapi/") || strings.HasPrefix(name, "infrastructure/swagger/")
}
