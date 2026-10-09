// Package resolver calculates selection closure without reading or mutating external state.
package resolver

import (
	"go/version"
	"gostarter/internal/project/protocol"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
	"path"
	"sort"
	"strings"
)

// Extension preserves user intent separately from dependency activation.
type Extension struct {
	ID              string     `json:"id"`
	Kind            string     `json:"kind"`
	Path            string     `json:"path"`
	SelectedByUser  bool       `json:"selected_by_user"`
	Locked          bool       `json:"locked"`
	RequiredBy      []string   `json:"required_by"`
	DependencyPaths [][]string `json:"dependency_paths"`
}

// Core is always active because the architecture profile requires it.
type Core struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Locked bool   `json:"locked"`
	Reason string `json:"reason"`
}

// Result has stable dependency-first extension order and sorted explicit selections.
type Result struct {
	RequestedExtensions []string    `json:"requested_extensions"`
	Extensions          []Extension `json:"extensions"`
	Core                []Core      `json:"core"`
}

func fail(code, id, cause string) error {
	return manifest.NewDiagnostic(code, id, manifest.FileName, "requested_extensions", cause)
}

// Resolve recomputes the complete result from explicit selections on every call.
func Resolve(m *manifest.Manifest, p *profile.Profile, requested []string, goVersion, transport string) (*Result, error) {
	if err := manifest.Validate(m); err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fail("unknown_profile", "", "profile is required")
	}
	if err := p.CheckRules(); err != nil {
		return nil, err
	}
	transport = protocol.Normalize(transport)
	if err := p.CheckCompatibility(goVersion, transport); err != nil {
		return nil, err
	}
	catalog := map[string]manifest.Extension{}
	for _, e := range m.Extensions {
		catalog[e.ID] = e
	}
	explicit := map[string]bool{}
	for _, raw := range requested {
		id := strings.TrimSpace(raw)
		if _, ok := catalog[id]; !ok {
			return nil, fail("unknown_extension", id, "ID is not in the optional catalog")
		}
		explicit[id] = true
	}
	roots := keys(explicit)
	result := &Result{RequestedExtensions: roots, Extensions: []Extension{}, Core: []Core{}}
	active := map[string]bool{}
	ordered := []string{}
	var visit func(string)
	visit = func(id string) {
		if active[id] {
			return
		}
		active[id] = true
		deps := append([]string(nil), catalog[id].Requires...)
		sort.Strings(deps)
		for _, dep := range deps {
			visit(dep)
		}
		ordered = append(ordered, id)
	}
	for _, id := range roots {
		visit(id)
	}
	coreIDs := map[string]bool{}
	for _, root := range p.Defaults {
		id := "core:" + path.Base(root)
		if coreIDs[id] {
			return nil, fail("default_conflict", id, "duplicate core identity")
		}
		coreIDs[id] = true
		result.Core = append(result.Core, Core{id, root, true, "architecture_default"})
	}
	sort.Slice(result.Core, func(i, j int) bool { return result.Core[i].ID < result.Core[j].ID })
	known := func(id string) bool { _, ok := catalog[id]; return ok || coreIDs[id] }
	selected := func(id string) bool { return active[id] || coreIDs[id] }
	for _, id := range sortedMapKeys(p.Conflicts) {
		if !known(id) {
			return nil, fail("invalid_profile", id, "conflict references an unknown component")
		}
		targets := append([]string(nil), p.Conflicts[id]...)
		sort.Strings(targets)
		for _, other := range targets {
			if !known(other) {
				return nil, fail("invalid_profile", other, "conflict references an unknown component")
			}
			if selected(id) && selected(other) {
				return nil, fail("extension_conflict", id, "conflicts with "+other)
			}
		}
	}
	for _, id := range sortedMapKeys(p.ExtensionCompatibility) {
		if _, ok := catalog[id]; !ok {
			return nil, fail("invalid_profile", id, "compatibility references an unknown extension")
		}
	}
	for _, id := range ordered {
		limits := p.ExtensionCompatibility[id]
		if limits.MinGo != "" && (!version.IsValid("go"+limits.MinGo) || version.Compare("go"+goVersion, "go"+limits.MinGo) < 0) {
			return nil, fail("incompatible_go", id, "extension requires Go >= "+limits.MinGo)
		}
		if len(limits.Protocols) > 0 && !has(limits.Protocols, transport) {
			return nil, fail("incompatible_protocol", id, "extension does not support "+transport)
		}
	}
	parents := map[string]map[string]bool{}
	paths := map[string][][]string{}
	count := 0
	for _, id := range ordered {
		for _, dep := range catalog[id].Requires {
			if parents[dep] == nil {
				parents[dep] = map[string]bool{}
			}
			parents[dep][id] = true
		}
	}
	var trace func(string, []string) error
	trace = func(id string, chain []string) error {
		chain = append(append([]string(nil), chain...), id)
		count++
		if count > 10000 {
			return fail("dependency_complexity", id, "more than 10000 dependency paths; simplify the catalog graph")
		}
		paths[id] = append(paths[id], chain)
		deps := append([]string(nil), catalog[id].Requires...)
		sort.Strings(deps)
		for _, dep := range deps {
			if err := trace(dep, chain); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range roots {
		if err := trace(root, nil); err != nil {
			return nil, err
		}
	}
	for _, id := range ordered {
		e := catalog[id]
		required := keys(parents[id])
		sort.Slice(paths[id], func(i, j int) bool { return strings.Join(paths[id][i], "/") < strings.Join(paths[id][j], "/") })
		result.Extensions = append(result.Extensions, Extension{id, e.Kind, e.Path, explicit[id], len(required) > 0, required, paths[id]})
	}
	return result, nil
}
func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func sortedMapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func has(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
