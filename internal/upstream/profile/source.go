package profile

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	goversion "go/version"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"gostarter/internal/upstream/manifest"
)

// Source is an extracted, repository-relative snapshot, not a live executable tree.
type Source map[string][]byte

// File records one source/output ownership decision before selection.
type File struct {
	Path   string `json:"path"`
	Owner  string `json:"owner"`
	Action Action `json:"action"`
}

// Requirement records the module version from upstream go.mod, without duplicate manifest metadata.
type Requirement struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// Report describes static contract compatibility, not successful generated build validation.
type Report struct {
	ArchitectureID  string                   `json:"architecture_id"`
	ProfileRevision string                   `json:"profile_revision"`
	TemplateVersion string                   `json:"template_version"`
	SourceModule    string                   `json:"source_module"`
	GoVersion       string                   `json:"go_version"`
	Files           []File                   `json:"files"`
	Features        []FeatureIntegration     `json:"features"`
	Modules         map[string][]Requirement `json:"modules_by_owner"`
	ConfigKeys      []string                 `json:"config_keys"`
}

// ReadSource loads a local source tree without following symlinks or reading developer secrets.
// VCS metadata, build/cache directories and local .env files are not snapshot inputs.
func ReadSource(tree fs.FS) (Source, error) {
	result := Source{}
	total := 0
	err := fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return diag("source_read", "", name, "", err.Error())
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return diag("unsafe_source_path", "", name, "", "symlinks are not supported in snapshots")
		}
		if entry.IsDir() {
			if name != "." && (entry.Name() == ".git" || entry.Name() == ".cache" || entry.Name() == "bin") {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Name() == ".env" || strings.HasPrefix(entry.Name(), ".env.") && entry.Name() != ".env.example" || entry.Name() == ".DS_Store" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return diag("source_limit", "", name, "", "expected regular file <= 8 MiB")
		}
		if len(result) >= 10000 || total+int(info.Size()) > 64<<20 {
			return diag("source_limit", "", name, "", "snapshot exceeds file count/size limit")
		}
		data, err := fs.ReadFile(tree, name)
		if err != nil {
			return diag("source_read", "", name, "", err.Error())
		}
		result[name] = data
		total += len(data)
		return nil
	})
	return result, err
}
func validGoVersion(version, minimum string) bool {
	return goversion.IsValid("go"+version) && goversion.Compare("go"+version, "go"+minimum) >= 0
}

// ValidateSource checks actual declarations, file coverage, import dependencies and feature assets.
// It does not run Wire/protoc, compile the result, or activate a snapshot.
func ValidateSource(m *manifest.Manifest, p *Profile, source Source) (*Report, error) {
	if err := manifest.Validate(m); err != nil {
		return nil, err
	}
	if p == nil {
		return nil, diag("unknown_profile", "", "", "", "profile is required")
	}
	if err := p.CheckRules(); err != nil {
		return nil, err
	}
	module, err := modfile.Parse("go.mod", source["go.mod"], nil)
	if err != nil || module.Module == nil || module.Go == nil {
		return nil, diag("invalid_module", "", "go.mod", "", "module and go directives must be valid")
	}
	if len(module.Replace) > 0 {
		return nil, diag("unsupported_replace", "", "go.mod", "replace", "profile v1 does not support module replacements")
	}
	if err := p.CheckCompatibility(module.Go.Version, "dual"); err != nil {
		return nil, err
	}
	report := &Report{ArchitectureID: p.ArchitectureID, ProfileRevision: p.Revision, TemplateVersion: p.TemplateVersion, SourceModule: module.Module.Mod.Path, GoVersion: module.Go.Version, Files: []File{}, Features: append([]FeatureIntegration(nil), p.Features...), Modules: map[string][]Requirement{}}
	names := make([]string, 0, len(source))
	for name := range source {
		names = append(names, name)
	}
	sort.Strings(names)
	parsed := map[string]*ast.File{}
	owners := map[string]Rule{}
	packages := map[string]bool{}
	for _, name := range names {
		if !fs.ValidPath(name) || strings.Contains(name, "\\") {
			return nil, diag("unsafe_source_path", "", name, "", "snapshot paths must be clean repository-relative paths")
		}
		rule, err := p.Classify(name)
		if err != nil {
			return nil, err
		}
		owners[name] = rule
		report.Files = append(report.Files, File{name, rule.Owner, rule.Action})
		if strings.HasSuffix(name, ".go") {
			f, err := parser.ParseFile(token.NewFileSet(), name, source[name], 0)
			if err != nil {
				return nil, diag("go_syntax", rule.Owner, name, "", err.Error())
			}
			parsed[name] = f
			if !strings.HasSuffix(name, "_test.go") {
				packages[path.Dir(name)] = true
			}
		}
	}
	for _, e := range m.Extensions {
		if !packages[e.Path] {
			return nil, diag("missing_source", e.ID, e.Path, "", "extension root must contain non-test Go source")
		}
		owner, err := p.Classify(e.Path)
		if err != nil || owner.Owner != e.ID {
			return nil, diag("ownership_conflict", e.ID, e.Path, "", "manifest and profile disagree on extension ownership")
		}
	}
	for _, root := range p.Defaults {
		if !packages[root] {
			return nil, diag("missing_default", "", root, "", "required default package is absent")
		}
	}
	// Every application template contract is based on a real reference file.
	for _, tpl := range p.Templates {
		if _, ok := source[tpl.Output]; !ok {
			return nil, diag("missing_reference", "", tpl.Output, "", "template reference missing from upstream revision")
		}
	}
	for _, symbol := range p.ConfigSymbols {
		if err := checkSymbol(parsed, symbol, ""); err != nil {
			return nil, err
		}
	}
	for _, resource := range p.Resources {
		if err := checkSymbol(parsed, resource.Provider, ""); err != nil {
			return nil, err
		}
		if err := checkSymbol(parsed, resource.Constructor, ""); err != nil {
			return nil, err
		}
	}
	if err := checkFeature(parsed, source, report.SourceModule, FeatureIntegration{ID: "core:health", SourcePath: "internal/health", Provider: Symbol{"internal/health/provider.go", "ProviderSet", "wire_set"}, Constructor: Symbol{"internal/health/handler/handler.go", "NewHandler", "constructor"}, Handler: "internal/health/handler", ProtoPath: "api/proto/health", GeneratedPath: "gen/pb/health"}); err != nil {
		return nil, err
	}
	for _, feature := range p.Features {
		if err := checkFeature(parsed, source, report.SourceModule, feature); err != nil {
			return nil, err
		}
	}
	if err := checkProtoImports(m, p, source, names); err != nil {
		return nil, err
	}
	if err := checkProtoNames(source, p.Features); err != nil {
		return nil, err
	}
	// go.mod requirements are derived by owner from source imports, including tests.
	modules := append([]*modfile.Require(nil), module.Require...)
	sort.Slice(modules, func(i, j int) bool { return len(modules[i].Mod.Path) > len(modules[j].Mod.Path) })
	used := map[string]map[string]string{}
	prefix := report.SourceModule + "/"
	for _, name := range names {
		file := parsed[name]
		if file == nil {
			continue
		}
		owner := owners[name]
		if owner.Action == Exclude {
			continue
		}
		for _, spec := range file.Imports {
			imp, _ := strconv.Unquote(spec.Path.Value)
			if strings.HasPrefix(imp, prefix) {
				target := strings.TrimPrefix(imp, prefix)
				if !packages[target] {
					return nil, diag("missing_import", owner.Owner, name, imp, "local package does not exist")
				}
				targetRule, err := p.Classify(target + "/package.go")
				// Exact rules (app-owned files) are resolved through the actual package directory.
				if err != nil {
					for _, other := range names {
						if path.Dir(other) == target && strings.HasSuffix(other, ".go") && !strings.HasSuffix(other, "_test.go") {
							targetRule = owners[other]
							err = nil
							break
						}
					}
				}
				if err != nil {
					return nil, err
				}
				if targetRule.Action == Exclude {
					return nil, diag("unsupported_integration", owner.Owner, name, imp, "import depends on excluded infrastructure/example")
				}
				if ext, ok := m.Lookup(owner.Owner); ok && ext.Kind == "pkg" && (strings.HasPrefix(target, "internal/") || strings.HasPrefix(target, "infrastructure/") || strings.HasPrefix(target, "bootstrap")) {
					return nil, diag("pkg_boundary", ext.ID, name, imp, "pkg extensions must remain generic; no business/infrastructure imports")
				}
				if strings.HasPrefix(owner.Owner, "core:") {
					if _, optional := m.Lookup(targetRule.Owner); optional {
						return nil, diag("default_dependency", owner.Owner, name, imp, "default core may not require an optional extension")
					}
				}
				if ext, ok := m.Lookup(owner.Owner); ok && targetRule.Owner != ext.ID {
					if dep, optional := m.Lookup(targetRule.Owner); optional && !contains(ext.Requires, dep.ID) {
						return nil, diag("undeclared_dependency", ext.ID, name, imp, "declare direct requires: "+dep.ID)
					}
				}
			} else if strings.Contains(strings.Split(imp, "/")[0], ".") {
				var required *modfile.Require
				for _, req := range modules {
					if imp == req.Mod.Path || strings.HasPrefix(imp, req.Mod.Path+"/") {
						required = req
						break
					}
				}
				if required == nil {
					return nil, diag("missing_module", owner.Owner, name, imp, "no matching requirement in upstream go.mod")
				}
				if used[owner.Owner] == nil {
					used[owner.Owner] = map[string]string{}
				}
				used[owner.Owner][required.Mod.Path] = required.Mod.Version
			}
		}
	}
	for owner, requirements := range used {
		for name, version := range requirements {
			report.Modules[owner] = append(report.Modules[owner], Requirement{name, version})
		}
		sort.Slice(report.Modules[owner], func(i, j int) bool { return report.Modules[owner][i].Path < report.Modules[owner][j].Path })
	}
	config := parsed["infrastructure/config/config.go"]
	ast.Inspect(config, func(n ast.Node) bool {
		field, ok := n.(*ast.Field)
		if !ok || field.Tag == nil {
			return true
		}
		tag, _ := strconv.Unquote(field.Tag.Value)
		match := configTag.FindStringSubmatch(tag)
		if len(match) > 1 {
			report.ConfigKeys = append(report.ConfigKeys, match[1])
		}
		return true
	})
	sort.Strings(report.ConfigKeys)
	for _, required := range []string{"APP_NAME", "DB_HOST", "SERVER_GRPC_PORT", "SERVER_HTTP_PORT", "JWT_SECRET_KEY"} {
		if !contains(report.ConfigKeys, required) {
			return nil, diag("missing_config", "", "infrastructure/config/config.go", required, "expected configuration key is absent")
		}
	}
	if err := checkLifecycle(parsed); err != nil {
		return nil, err
	}
	// Known SQL runtime dependency, not inferable solely from Go imports.
	for _, feature := range p.Features {
		if feature.SourcePath == "internal/auth" {
			e, _ := m.Lookup(feature.ID)
			var userID string
			for _, candidate := range m.Extensions {
				if candidate.Path == "internal/user" {
					userID = candidate.ID
				}
			}
			if userID == "" || !contains(e.Requires, userID) {
				return nil, diag("runtime_dependency", feature.ID, feature.SourcePath, "requires", "refresh_tokens foreign key requires the User feature")
			}
		}
	}
	return report, nil
}

var configTag = regexp.MustCompile(`mapstructure:"([A-Z][A-Z0-9_]*)"`)
var servicePattern = regexp.MustCompile(`(?m)\bservice\s+([A-Za-z][A-Za-z0-9_]*)\s*\{`)

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func checkSymbol(files map[string]*ast.File, s Symbol, id string) error {
	file := files[s.File]
	if file == nil {
		return diag("missing_symbol", id, s.File, s.Name, "declaration file is absent")
	}
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == s.Name && (s.Kind == "function" || s.Kind == "constructor") {
			if s.Kind == "constructor" && (fn.Type.Results == nil || len(fn.Type.Results.List) != 1 || typeName(fn.Type.Results.List[0].Type, file) != "*Handler") {
				return diag("invalid_signature", id, s.File, s.Name, "constructor must return *Handler")
			}
			return nil
		}
		if decl, ok := declaration.(*ast.GenDecl); ok {
			for _, spec := range decl.Specs {
				if typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.Name == s.Name && s.Kind == "type" {
					return nil
				}
				if value, ok := spec.(*ast.ValueSpec); ok && s.Kind == "wire_set" {
					for i, name := range value.Names {
						if name.Name == s.Name && i < len(value.Values) {
							if call, ok := value.Values[i].(*ast.CallExpr); ok && typeName(call.Fun, file) == "github.com/google/wire.NewSet" {
								return nil
							}
						}
					}
				}
			}
		}
	}
	return diag("missing_symbol", id, s.File, s.Name, "expected "+s.Kind+" declaration is absent or incompatible")
}
func checkFeature(files map[string]*ast.File, source Source, module string, feature FeatureIntegration) error {
	if err := checkSymbol(files, feature.Provider, feature.ID); err != nil {
		return err
	}
	if err := checkSymbol(files, feature.Constructor, feature.ID); err != nil {
		return err
	}
	if err := checkProviderReferences(files, module, feature); err != nil {
		return err
	}
	signatures := map[string][]string{
		"RegisterGRPC":    {"*google.golang.org/grpc.Server"},
		"RegisterGateway": {"context.Context", "*github.com/grpc-ecosystem/grpc-gateway/v2/runtime.ServeMux", "string", "[]google.golang.org/grpc.DialOption"},
	}
	for _, name := range []string{"RegisterGRPC", "RegisterGateway"} {
		want := signatures[name]
		found := false
		for _, fileName := range sortedParsed(files) {
			file := files[fileName]
			if path.Dir(fileName) != feature.Handler || strings.HasSuffix(fileName, "_test.go") {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name.Name != name || fn.Recv == nil || typeName(fn.Recv.List[0].Type, file) != "*Handler" {
					continue
				}
				got := fieldTypes(fn.Type.Params, file)
				result := fieldTypes(fn.Type.Results, file)
				if strings.Join(got, "|") != strings.Join(want, "|") || name == "RegisterGRPC" && len(result) != 0 || name == "RegisterGateway" && strings.Join(result, "|") != "error" {
					return diag("invalid_signature", feature.ID, fileName, name, "does not satisfy ServiceRegister")
				}
				found = true
			}
		}
		if !found {
			return diag("missing_symbol", feature.ID, feature.Constructor.File, name, "Handler must implement ServiceRegister")
		}
	}
	if err := checkSymbol(files, Symbol{feature.Constructor.File, "Handler", "type"}, feature.ID); err != nil {
		return err
	}
	services := []string{}
	for name, data := range source {
		if strings.HasPrefix(name, feature.ProtoPath+"/") && strings.HasSuffix(name, ".proto") {
			for _, match := range servicePattern.FindAllStringSubmatch(stripProtoComments(string(data)), -1) {
				services = append(services, match[1])
			}
		}
	}
	if len(services) != 1 {
		return diag("unsupported_proto", feature.ID, feature.ProtoPath, "service", "profile v1 requires exactly one protobuf service per feature")
	}
	service := services[0]
	for _, symbol := range []string{"Register" + service + "Server", "Register" + service + "HandlerFromEndpoint"} {
		found := false
		for name, file := range files {
			if path.Dir(name) != feature.GeneratedPath {
				continue
			}
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == symbol {
					found = true
				}
			}
		}
		if !found {
			return diag("missing_symbol", feature.ID, feature.GeneratedPath, symbol, "generated registration is missing; regenerate upstream protobuf")
		}
	}
	for _, registration := range []struct{ method, symbol string }{{"RegisterGRPC", "Register" + service + "Server"}, {"RegisterGateway", "Register" + service + "HandlerFromEndpoint"}} {
		invoked := false
		for fileName, file := range files {
			if path.Dir(fileName) != feature.Handler || strings.HasSuffix(fileName, "_test.go") {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name.Name != registration.method || fn.Recv == nil || typeName(fn.Recv.List[0].Type, file) != "*Handler" {
					continue
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if ok && typeName(call.Fun, file) == module+"/"+feature.GeneratedPath+"."+registration.symbol {
						invoked = true
					}
					return true
				})
			}
		}
		if !invoked {
			return diag("missing_registration", feature.ID, feature.Constructor.File, registration.method, "must invoke generated "+registration.symbol)
		}
	}
	for _, migration := range feature.Migrations {
		if len(source[migration]) == 0 {
			return diag("missing_asset", feature.ID, migration, "", "required migration pair is incomplete")
		}
	}
	return nil
}

var protoComments = regexp.MustCompile(`(?s)/\*.*?\*/|(?m)//[^\n]*`)

func stripProtoComments(value string) string { return protoComments.ReplaceAllString(value, "") }
func sortedParsed(files map[string]*ast.File) []string {
	out := make([]string, 0, len(files))
	for name := range files {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
func fieldTypes(list *ast.FieldList, file *ast.File) []string {
	out := []string{}
	if list == nil {
		return out
	}
	for _, field := range list.List {
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			out = append(out, typeName(field.Type, file))
		}
	}
	return out
}
func typeName(expr ast.Expr, file *ast.File) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return "*" + typeName(value.X, file)
	case *ast.ArrayType:
		if value.Len == nil {
			return "[]" + typeName(value.Elt, file)
		}
	case *ast.SelectorExpr:
		if alias, ok := value.X.(*ast.Ident); ok {
			for _, imp := range file.Imports {
				name, _ := strconv.Unquote(imp.Path.Value)
				local := path.Base(name)
				if imp.Name != nil {
					local = imp.Name.Name
				}
				if local == alias.Name {
					return name + "." + value.Sel.Name
				}
			}
		}
	}
	return fmt.Sprintf("unsupported:%T", expr)
}

func checkLifecycle(files map[string]*ast.File) error {
	file := files["bootstrap/app.go"]
	if file == nil {
		return diag("missing_lifecycle", "", "bootstrap/app.go", "Shutdown", "application lifecycle is required")
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Shutdown" || fn.Recv == nil || typeName(fn.Recv.List[0].Type, file) != "*App" {
			continue
		}
		calls := []string{}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					calls = append(calls, sel.Sel.Name)
				}
			}
			return true
		})
		required := []string{"Shutdown", "GracefulStop", "WaitForShutdown", "DB", "Close", "Shutdown", "Shutdown", "Sync"}
		index := 0
		for _, name := range calls {
			if index < len(required) && name == required[index] {
				index++
			}
		}
		if index != len(required) {
			return diag("incompatible_lifecycle", "", "bootstrap/app.go", "App.Shutdown", "expected drain HTTP, stop gRPC, await routines, close DB, flush tracer/meter, sync logger in order")
		}
		return nil
	}
	return diag("missing_lifecycle", "", "bootstrap/app.go", "App.Shutdown", "shutdown method is absent")
}

var protoPackage = regexp.MustCompile(`(?m)\bpackage\s+([A-Za-z_][A-Za-z0-9_.]*)\s*;`)

func checkProtoNames(source Source, features []FeatureIntegration) error {
	seen := map[string]string{}
	all := append([]FeatureIntegration{{ID: "core:health", ProtoPath: "api/proto/health"}}, features...)
	names := make([]string, 0, len(source))
	for name := range source {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, feature := range all {
		for _, name := range names {
			if !strings.HasPrefix(name, feature.ProtoPath+"/") || !strings.HasSuffix(name, ".proto") {
				continue
			}
			text := stripProtoComments(string(source[name]))
			services := servicePattern.FindAllStringSubmatch(text, -1)
			if len(services) == 0 {
				continue
			}
			pkg := protoPackage.FindStringSubmatch(text)
			if len(pkg) != 2 {
				return diag("unsupported_proto", feature.ID, name, "package", "service proto must declare a package")
			}
			for _, service := range services {
				full := pkg[1] + "." + service[1]
				if previous, ok := seen[full]; ok {
					return diag("proto_conflict", feature.ID, name, full, "service already declared in "+previous)
				}
				seen[full] = name
			}
		}
	}
	return nil
}

func checkProviderReferences(files map[string]*ast.File, module string, feature FeatureIntegration) error {
	file := files[feature.Provider.File]
	var provider ast.Expr
	for _, decl := range file.Decls {
		if group, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range group.Specs {
				if value, ok := spec.(*ast.ValueSpec); ok {
					for i, name := range value.Names {
						if name.Name == feature.Provider.Name && i < len(value.Values) {
							provider = value.Values[i]
						}
					}
				}
			}
		}
	}
	constructorReferenced := false
	var failure error
	ast.Inspect(provider, func(node ast.Node) bool {
		if failure != nil {
			return false
		}
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		reference := typeName(selector, file)
		if !strings.HasPrefix(reference, module+"/") {
			return true
		}
		split := strings.LastIndex(reference, ".")
		if split < 0 {
			return true
		}
		target := strings.TrimPrefix(reference[:split], module+"/")
		symbol := reference[split+1:]
		if target == feature.Handler && symbol == feature.Constructor.Name {
			constructorReferenced = true
		}
		found := false
		for fileName, other := range files {
			if path.Dir(fileName) != target || strings.HasSuffix(fileName, "_test.go") {
				continue
			}
			for _, decl := range other.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == symbol {
					found = true
				}
				if group, ok := decl.(*ast.GenDecl); ok {
					for _, spec := range group.Specs {
						if typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.Name == symbol {
							found = true
						}
						if value, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range value.Names {
								if name.Name == symbol {
									found = true
								}
							}
						}
					}
				}
			}
		}
		if !found {
			failure = diag("missing_provider_dependency", feature.ID, feature.Provider.File, reference, "ProviderSet references an absent source declaration")
		}
		return true
	})
	if failure != nil {
		return failure
	}
	if !constructorReferenced {
		return diag("missing_provider_constructor", feature.ID, feature.Provider.File, feature.Constructor.Name, "ProviderSet must include the handler constructor")
	}
	return nil
}

var protoImport = regexp.MustCompile(`(?m)\bimport\s+(?:(?:public|weak)\s+)?"([^"\n]+)"\s*;`)

func checkProtoImports(m *manifest.Manifest, p *Profile, source Source, names []string) error {
	builtins := []string{"any", "api", "descriptor", "duration", "empty", "field_mask", "source_context", "struct", "timestamp", "type", "wrappers"}
	for _, name := range names {
		if !strings.HasPrefix(name, "api/proto/") || !strings.HasSuffix(name, ".proto") {
			continue
		}
		owner, err := p.Classify(name)
		if err != nil {
			return err
		}
		for _, match := range protoImport.FindAllStringSubmatch(stripProtoComments(string(source[name])), -1) {
			imp := match[1]
			if !fs.ValidPath(imp) || strings.Contains(imp, "\\") {
				return diag("unsafe_proto_import", owner.Owner, name, imp, "proto imports must be clean relative paths")
			}
			if imp == "protoc-gen-openapiv2/options/annotations.proto" {
				continue
			}
			if strings.HasPrefix(imp, "google/protobuf/") && contains(builtins, strings.TrimSuffix(strings.TrimPrefix(imp, "google/protobuf/"), ".proto")) && strings.HasSuffix(imp, ".proto") {
				continue
			}
			targets := []string{}
			for _, candidate := range []string{"api/proto/plugins/" + imp, "api/proto/" + imp} {
				if _, ok := source[candidate]; ok {
					targets = append(targets, candidate)
				}
			}
			if len(targets) != 1 {
				return diag("missing_proto_import", owner.Owner, name, imp, "expected one source proto across supported include roots")
			}
			target, err := p.Classify(targets[0])
			if err != nil {
				return err
			}
			dep, optional := m.Lookup(target.Owner)
			if !optional || target.Owner == owner.Owner {
				continue
			}
			if ext, ok := m.Lookup(owner.Owner); ok {
				if !contains(ext.Requires, dep.ID) {
					return diag("undeclared_proto_dependency", ext.ID, name, imp, "declare direct requires: "+dep.ID)
				}
			} else if strings.HasPrefix(owner.Owner, "core:") {
				return diag("default_proto_dependency", owner.Owner, name, imp, "default core may not depend on optional protobuf source")
			}
		}
	}
	return nil
}
