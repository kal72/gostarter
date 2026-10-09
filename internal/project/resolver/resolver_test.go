package resolver_test

import (
	"encoding/json"
	"errors"
	"gostarter/internal/project/resolver"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"

	"reflect"
	"testing"
)

func catalog(t *testing.T) (*manifest.Manifest, *profile.Profile) {
	t.Helper()
	m := &manifest.Manifest{SchemaVersion: 1, Extensions: []manifest.Extension{
		{ID: "user", Kind: "feature", Title: "User", Description: "User operations.", Path: "internal/user", Requires: []string{"cryptoutil", "validator"}},
		{ID: "auth", Kind: "feature", Title: "Auth", Description: "Authentication.", Path: "internal/auth", Requires: []string{"user", "cryptoutil", "validator"}},
		{ID: "cryptoutil", Kind: "pkg", Title: "Crypto", Description: "Hash helpers.", Path: "pkg/cryptoutil"},
		{ID: "validator", Kind: "pkg", Title: "Validator", Description: "Validation helpers.", Path: "pkg/validator"},
	}}
	p, err := profile.Default(m)
	if err != nil {
		t.Fatal(err)
	}
	return m, p
}
func get(t *testing.T, r *resolver.Result, id string) resolver.Extension {
	t.Helper()
	for _, e := range r.Extensions {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("missing %s", id)
	return resolver.Extension{}
}
func TestSelectionLifecycle(t *testing.T) {
	m, p := catalog(t)
	run := func(ids ...string) *resolver.Result {
		r, err := resolver.Resolve(m, p, ids, "1.27.1", "dual")
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	auth := run("auth")
	if len(auth.Extensions) != 4 || len(auth.Core) != 7 {
		t.Fatal(auth)
	}
	for _, id := range []string{"user", "cryptoutil", "validator"} {
		e := get(t, auth, id)
		if !e.Locked || e.SelectedByUser {
			t.Fatalf("bad dependency state: %+v", e)
		}
	}
	crypt := get(t, auth, "cryptoutil")
	if !reflect.DeepEqual(crypt.RequiredBy, []string{"auth", "user"}) || len(crypt.DependencyPaths) != 2 {
		t.Fatalf("shared reasons incomplete: %+v", crypt)
	}
	explicit := run("user", "auth")
	user := get(t, explicit, "user")
	if !user.SelectedByUser || !user.Locked {
		t.Fatal(user)
	}
	retained := run("user")
	if len(retained.Extensions) != 3 || !get(t, retained, "user").SelectedByUser || get(t, retained, "user").Locked {
		t.Fatal(retained)
	}
	empty := run()
	if len(empty.Extensions) != 0 || len(empty.Core) != 7 {
		t.Fatal(empty)
	}

	if !get(t, run("auth"), "user").Locked {
		t.Fatal("dependency could be removed")
	}
	retainedPkg := run("cryptoutil")
	if len(retainedPkg.Extensions) != 1 || !get(t, retainedPkg, "cryptoutil").SelectedByUser {
		t.Fatal(retainedPkg)
	}
}
func TestDeterministicPureResolver(t *testing.T) {
	m, p := catalog(t)
	before, _ := json.Marshal([]any{m, p})
	requested := []string{" user ", "auth", "auth"}
	saved := append([]string(nil), requested...)
	a, err := resolver.Resolve(m, p, requested, "1.27.1", " DUAL ")
	if err != nil {
		t.Fatal(err)
	}
	b, err := resolver.Resolve(m, p, []string{"auth", "user"}, "1.27.1", "dual")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("equivalent selection changed resolution")
	}
	after, _ := json.Marshal([]any{m, p})
	if string(before) != string(after) || !reflect.DeepEqual(saved, requested) {
		t.Fatal("resolver mutated inputs")
	}
	index := map[string]int{}
	for i, e := range a.Extensions {
		index[e.ID] = i
	}
	for _, e := range m.Extensions {
		if _, ok := index[e.ID]; ok {
			for _, dep := range e.Requires {
				if index[dep] >= index[e.ID] {
					t.Fatal("not dependency-first")
				}
			}
		}
	}
}
func TestResolverFailures(t *testing.T) {
	for _, tc := range []struct {
		name, code           string
		change               func(*manifest.Manifest, *profile.Profile)
		ids                  []string
		goVersion, transport string
	}{
		{name: "unknown", code: "unknown_extension", ids: []string{"missing"}},
		{name: "core not optional", code: "unknown_extension", ids: []string{"routine"}},
		{name: "rest", code: "incompatible_protocol", ids: []string{"auth"}, transport: "rest"},
		{name: "old Go", code: "incompatible_go", goVersion: "1.26.0"},
		{name: "cycle", code: "dependency_cycle", change: func(m *manifest.Manifest, _ *profile.Profile) {
			m.Extensions[0].Requires = append(m.Extensions[0].Requires, "auth")
		}},
		{name: "transitive conflict", code: "extension_conflict", ids: []string{"auth"}, change: func(_ *manifest.Manifest, p *profile.Profile) {
			p.Conflicts = map[string][]string{"cryptoutil": {"user"}}
		}},
		{name: "core conflict", code: "extension_conflict", ids: []string{"auth"}, change: func(_ *manifest.Manifest, p *profile.Profile) {
			p.Conflicts = map[string][]string{"user": {"core:jwtutil"}}
		}},
		{name: "dependency compatibility", code: "incompatible_protocol", ids: []string{"auth"}, change: func(_ *manifest.Manifest, p *profile.Profile) {
			p.ExtensionCompatibility = map[string]profile.Compatibility{"user": {Protocols: []string{"grpc"}}}
		}},
		{name: "dependency Go", code: "incompatible_go", ids: []string{"auth"}, change: func(_ *manifest.Manifest, p *profile.Profile) {
			p.ExtensionCompatibility = map[string]profile.Compatibility{"cryptoutil": {MinGo: "1.28.0"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, p := catalog(t)
			if tc.change != nil {
				tc.change(m, p)
			}
			g := tc.goVersion
			if g == "" {
				g = "1.27.1"
			}
			transport := tc.transport
			if transport == "" {
				transport = "dual"
			}
			_, err := resolver.Resolve(m, p, tc.ids, g, transport)
			var d *manifest.Diagnostic
			if !errors.As(err, &d) || d.Code != tc.code {
				t.Fatalf("want %s, got %v", tc.code, err)
			}
		})
	}
}
