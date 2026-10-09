package validation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gostarter/internal/project/plan"
	"gostarter/internal/project/resolver"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
)

type memoryCases struct {
	values      map[string]Result
	lookupError bool
}

func (m *memoryCases) LookupValidationCase(_ context.Context, key string) (Result, bool, error) {
	if m.lookupError {
		return Result{}, false, errors.New("cache unavailable")
	}
	r, ok := m.values[key]
	return r, ok, nil
}
func (m *memoryCases) SaveValidationCase(_ context.Context, key string, r Result) error {
	m.values[key] = r
	return nil
}

func incrementalSource() profile.Source {
	return profile.Source{
		manifest.FileName: []byte(`schema_version: 1
extensions:
  - id: helper
    kind: pkg
    title: Helper
    description: Shared helper.
    path: pkg/helper
  - id: dependent
    kind: pkg
    title: Dependent
    description: Uses helper.
    path: pkg/dependent
    requires: [helper]
`),
		"go.mod":                     []byte("module example.com/source\ngo 1.27.1\n"),
		"pkg/logger/logger.go":       []byte("package logger"),
		"pkg/helper/helper.go":       []byte("package helper\nconst Value = 1"),
		"pkg/dependent/dependent.go": []byte("package dependent"),
	}
}

// Model matrix selection with the actual resolver, exercising transitive dependencies.
func incrementalCases(t *testing.T, files profile.Source, execution string) []validationCase {
	t.Helper()
	catalog, err := manifest.Parse(files[manifest.FileName])
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := profile.Default(catalog)
	if err != nil {
		t.Fatal(err)
	}
	base := &plan.Plan{ArchitectureID: adapter.ArchitectureID, ProfileRevision: adapter.Revision}
	global, err := globalInputs(files, base)
	if err != nil {
		t.Fatal(err)
	}
	global, _ = digest([]string{global, execution})
	var cases []validationCase
	for _, protocol := range []string{"dual", "grpc"} {
		for _, selection := range [][]string{{}, {"helper"}, {"dependent"}, {"helper", "dependent"}} {
			resolved, err := resolver.Resolve(catalog, adapter, selection, "1.27.1", protocol)
			if err != nil {
				t.Fatal(err)
			}
			p := &plan.Plan{Config: plan.Config{Protocol: protocol, RequestedExtensions: selection}, Resolved: *resolved}
			assembled := profile.Source{"go.mod": files["go.mod"], "pkg/logger/logger.go": files["pkg/logger/logger.go"]}
			for _, extension := range resolved.Extensions {
				for name, body := range files {
					if strings.HasPrefix(name, extension.Path+"/") {
						assembled[name] = body
					}
				}
			}
			key, err := caseContentKey(global, p, assembled)
			if err != nil {
				t.Fatal(err)
			}
			cases = append(cases, validationCase{p, key})
		}
	}
	return cases
}

func TestIncrementalReusesUnaffectedCasesAndInvalidatesDependents(t *testing.T) {
	files := incrementalSource()
	cache := &memoryCases{values: map[string]Result{}}
	executed := 0
	run := func(context.Context, *plan.Plan, profile.Source) (Result, error) {
		executed++
		return Result{strings.Repeat("a", 64), strings.Repeat("b", 64)}, nil
	}
	validate := func(id string) Result {
		t.Helper()
		r, err := runCases(context.Background(), plan.Snapshot{ID: id}, files, incrementalCases(t, files, "tools-v1"), cache, run)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	first := validate("first")
	if executed != 8 {
		t.Fatalf("initial builds=%d", executed)
	}
	second := validate("second")
	if executed != 8 {
		t.Fatal("unchanged input rebuilt")
	}
	if first.IdentitySHA256 == second.IdentitySHA256 {
		t.Fatal("reused matrix not bound to new snapshot")
	}
	files["pkg/helper/helper.go"] = []byte("package helper\nconst Value = 2")
	validate("helper-change")
	if executed != 14 {
		t.Fatalf("helper + transitive dependents should rebuild 6 cases, got %d", executed-8)
	}
	files["pkg/dependent/dependent.go"] = []byte("package dependent\nconst Value = 2")
	validate("dependent-change")
	if executed != 18 {
		t.Fatalf("dependent change should rebuild 4 cases, got %d", executed-14)
	}
	files["pkg/logger/logger.go"] = []byte("package logger\nconst Value = 2")
	validate("core-change")
	if executed != 26 {
		t.Fatal("core change did not invalidate all cases")
	}
}

func TestIncrementalGlobalInvalidation(t *testing.T) {
	for _, field := range []string{"go.mod", "gostarter.yaml", "bootstrap/wire.go", "docs/readme.md"} {
		t.Run(field, func(t *testing.T) {
			files := incrementalSource()
			before := incrementalCases(t, files, "tools-v1")
			files[field] = append(files[field], []byte("\n# changed\n")...)
			after := incrementalCases(t, files, "tools-v1")
			for i := range before {
				if before[i].input == after[i].input {
					t.Fatalf("%s did not invalidate case %d", field, i)
				}
			}
		})
	}
	files := incrementalSource()
	before, after := incrementalCases(t, files, "tools-v1"), incrementalCases(t, files, "tools-v2")
	for i := range before {
		if before[i].input == after[i].input {
			t.Fatal("tool identity change retained cache key")
		}
	}
}

func TestIncrementalFailsClosedAndNeverCachesFailures(t *testing.T) {
	files := incrementalSource()
	cases := incrementalCases(t, files, "tools-v1")[:1]
	cache := &memoryCases{values: map[string]Result{}}
	want := errors.New("build failed")
	_, err := runCases(context.Background(), plan.Snapshot{}, files, cases, cache, func(context.Context, *plan.Plan, profile.Source) (Result, error) { return Result{}, want })
	if !errors.Is(err, want) || len(cache.values) != 0 {
		t.Fatal("failure cached", err)
	}
	cache.values[cases[0].input] = Result{ContentSHA256: "invalid", IdentitySHA256: "invalid"}
	calls := 0
	run := func(context.Context, *plan.Plan, profile.Source) (Result, error) {
		calls++
		return Result{strings.Repeat("a", 64), strings.Repeat("b", 64)}, nil
	}
	if _, err = runCases(context.Background(), plan.Snapshot{}, files, cases, cache, run); err != nil || calls != 1 {
		t.Fatal("invalid evidence reused", err)
	}
	cache.lookupError = true
	if _, err = runCases(context.Background(), plan.Snapshot{}, files, cases, cache, run); err != nil || calls != 2 {
		t.Fatal("cache error did not fall back to build", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = runCases(ctx, plan.Snapshot{}, files, cases, cache, run); !errors.Is(err, context.Canceled) || calls != 2 {
		t.Fatal("cancelled work executed", err)
	}
}
