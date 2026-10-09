package render

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"gostarter/internal/project/plan"
	"gostarter/internal/project/templates"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
)

// OutputFile records the one permitted producer and canonical mode of a final file.
type OutputFile struct {
	Path   string         `json:"path"`
	Owner  string         `json:"owner"`
	Action profile.Action `json:"action"`
	Reason string         `json:"reason"`
	SHA256 string         `json:"sha256"`
	Mode   uint32         `json:"mode"`
}
type ToolIdentity struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}
type ExecutionIdentity struct {
	GoVersion   string                  `json:"go_version"`
	Tools       map[string]ToolIdentity `json:"tools"`
	Includes    map[string]string       `json:"includes"`
	Environment map[string]string       `json:"environment"`
}

// Receipt is a bounded validation result, not a saved archive or command log.
// It binds the actual tools/includes, plan, ownership and source content produced.
type Receipt struct {
	PlanFingerprint string            `json:"plan_fingerprint"`
	Execution       ExecutionIdentity `json:"execution"`
	Files           []OutputFile      `json:"files"`
	Excluded        []plan.FileAction `json:"excluded"`
	ContentSHA256   string            `json:"content_sha256"`
	IdentitySHA256  string            `json:"identity_sha256"`
}
type Artifact struct {
	Files   profile.Source
	Receipt Receipt
}

func hashBytes(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
func hashFile(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func canonicalMode(name string) uint32 {
	if strings.HasSuffix(name, ".sh") {
		return 0755
	}
	return 0644
}

// readTree includes every file, including unexpected hidden files. Nothing is
// silently filtered as a source cache would be. Root prevents traversal escapes.
func readTree(dir string) (profile.Source, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	files := profile.Source{}
	total := int64(0)
	err = fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("materialize: symlink output %s", name)
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !validOutput(name) || info.Size() > 8<<20 || len(files) >= 10000 {
			return fmt.Errorf("materialize: invalid or excessive output %s", name)
		}
		total += info.Size()
		if total > 64<<20 {
			return fmt.Errorf("materialize: output exceeds 64 MiB")
		}
		body, err := root.ReadFile(name)
		if err != nil {
			return err
		}
		files[name] = body
		return nil
	})
	return files, err
}

func executionIdentity(ctx context.Context, dir string, p *plan.Plan) (ExecutionIdentity, error) {
	out := ExecutionIdentity{Tools: map[string]ToolIdentity{}, Includes: map[string]string{}, Environment: plan.GenerationEnvironment(p.Config.GoVersion)}
	env := environment(out.Environment)
	tools, err := CheckTools(ctx, dir, env)
	if err != nil {
		return out, err
	}
	version, err := run(ctx, dir, env, "go", "env", "GOVERSION")
	if err != nil {
		return out, err
	}
	if version != "go"+p.Config.GoVersion {
		return out, fmt.Errorf("materialize: expected Go %s, found %s", p.Config.GoVersion, version)
	}
	out.GoVersion = version
	goroot, err := run(ctx, dir, env, "go", "env", "GOROOT")
	if err != nil {
		return out, err
	}
	tools["go"] = filepath.Join(goroot, "bin", "go")
	versions := templates.ToolVersions()
	versions["go"] = version
	for name, bin := range tools {
		digest, err := hashFile(bin)
		if err != nil {
			return out, err
		}
		out.Tools[name] = ToolIdentity{versions[name], digest}
	}
	include, err := protocInclude(tools["protoc"])
	if err != nil {
		return out, err
	}
	cache, err := run(ctx, dir, env, "go", "env", "GOMODCACHE")
	if err != nil {
		return out, err
	}
	for key, dir := range map[string]string{"protobuf": filepath.Join(include, "google/protobuf"), "openapi_options": filepath.Join(cache, "github.com/grpc-ecosystem/grpc-gateway/v2@v2.29.0/protoc-gen-openapiv2/options")} {
		files, err := readTree(dir)
		if err != nil {
			return out, err
		}
		protos := profile.Source{}
		for name, body := range files {
			if strings.HasSuffix(name, ".proto") {
				protos[name] = body
			}
		}
		out.Includes[key] = plan.ContentDigest(protos)
	}
	return out, nil
}

// auditOutput refuses missing files, changed copied/rendered source, excluded
// components, and unexpected generated paths. New generated files need an
// explicit Generate subtree in the application profile.
func auditOutput(p *plan.Plan, source, assembled, output profile.Source) ([]OutputFile, error) {
	catalog, err := manifest.Parse(source[manifest.FileName])
	if err != nil {
		return nil, err
	}
	prof, err := profile.Registry(p.ArchitectureID, p.ProfileRevision, catalog)
	if err != nil {
		return nil, err
	}
	decisions := map[string]plan.FileAction{}
	active := map[string]bool{}
	for _, e := range p.Resolved.Extensions {
		active[e.ID] = true
	}
	for _, f := range p.Files {
		if _, exists := decisions[f.Path]; exists {
			return nil, fmt.Errorf("duplicate output decision %s", f.Path)
		}
		decisions[f.Path] = f
	}
	for name, expected := range assembled {
		body, ok := output[name]
		if !ok {
			return nil, fmt.Errorf("materialize: missing output %s", name)
		}
		if name != "go.mod" && name != "go.sum" && !bytes.Equal(body, expected) {
			return nil, fmt.Errorf("materialize: producer changed immutable output %s", name)
		}
	}
	for _, f := range p.Files {
		if f.Action == profile.Generate {
			if _, ok := output[f.Path]; !ok {
				return nil, fmt.Errorf("materialize: missing generated output %s", f.Path)
			}
		}
	}
	expectedMod, err := modfile.Parse("go.mod", source["go.mod"], nil)
	if err != nil {
		return nil, err
	}
	actualMod, err := modfile.Parse("go.mod", output["go.mod"], nil)
	if err != nil {
		return nil, err
	}
	if actualMod.Module == nil || actualMod.Module.Mod.Path != p.Config.Module || actualMod.Go == nil || actualMod.Go.Version != p.Config.GoVersion || len(actualMod.Replace) != 0 || actualMod.Toolchain != nil && actualMod.Toolchain.Name != "go"+p.Config.GoVersion {
		return nil, fmt.Errorf("materialize: module/toolchain changed")
	}
	versions := map[string]string{}
	for _, req := range expectedMod.Require {
		versions[req.Mod.Path] = req.Mod.Version
	}
	for _, req := range actualMod.Require {
		if versions[req.Mod.Path] != req.Mod.Version {
			return nil, fmt.Errorf("materialize: unpinned module %s@%s", req.Mod.Path, req.Mod.Version)
		}
	}
	names := make([]string, 0, len(output))
	for name := range output {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]OutputFile, 0, len(names))
	folded := map[string]bool{}
	for _, name := range names {
		if strings.HasSuffix(name, ".go") {
			formatted, err := format.Source(output[name])
			if err != nil || !bytes.Equal(formatted, output[name]) {
				return nil, fmt.Errorf("materialize: noncanonical Go output %s: %v", name, err)
			}
		}
		key := strings.ToLower(name)
		if folded[key] {
			return nil, fmt.Errorf("materialize: case-colliding output %s", name)
		}
		folded[key] = true
		decision, ok := decisions[name]
		if !ok {
			rule, err := prof.Classify(name)
			if err != nil {
				return nil, err
			}
			if rule.Action != profile.Generate || profile.TransportExcludes(name, p.Config.Protocol) {
				return nil, fmt.Errorf("materialize: unexpected output %s", name)
			}
			if _, optional := catalog.Lookup(rule.Owner); optional && !active[rule.Owner] {
				return nil, fmt.Errorf("materialize: unselected output %s", name)
			}
			decision = plan.FileAction{Path: name, Owner: rule.Owner, Action: rule.Action, Reason: "generated_from_selected_source"}
		}
		if decision.Action == profile.Exclude {
			return nil, fmt.Errorf("materialize: excluded output %s", name)
		}
		result = append(result, OutputFile{name, decision.Owner, decision.Action, decision.Reason, hashBytes(output[name]), canonicalMode(name)})
	}
	return result, nil
}

func (r Receipt) identityDigest() string {
	r.IdentitySHA256 = ""
	body, _ := json.Marshal(r)
	return hashBytes(body)
}

// Verify detects edits between validated materialization and preview/publication.
// The receipt is not an authenticity signature; trusted storage comes in stage 4.
func (a Artifact) Verify() error {
	if a.Receipt.IdentitySHA256 == "" || a.Receipt.IdentitySHA256 != a.Receipt.identityDigest() || a.Receipt.ContentSHA256 != plan.ContentDigest(a.Files) || len(a.Receipt.Files) != len(a.Files) {
		return fmt.Errorf("materialize: artifact/receipt mismatch")
	}
	seen := map[string]bool{}
	for _, file := range a.Receipt.Files {
		if seen[file.Path] || !validOutput(file.Path) || file.Owner == "" || file.Reason == "" || file.Action == profile.Exclude {
			return fmt.Errorf("materialize: invalid file ownership %s", file.Path)
		}
		seen[file.Path] = true
		switch file.Action {
		case profile.Copy, profile.Shared, profile.Render, profile.Generate:
		default:
			return fmt.Errorf("materialize: invalid producer %s", file.Path)
		}
		body, ok := a.Files[file.Path]
		if !ok || hashBytes(body) != file.SHA256 || file.Mode != canonicalMode(file.Path) {
			return fmt.Errorf("materialize: changed file %s", file.Path)
		}
	}
	return nil
}

// VerifyFor binds publication to the requested plan as well as the file bytes.
func (a Artifact) VerifyFor(p *plan.Plan) error {
	if p == nil {
		return fmt.Errorf("materialize: missing expected plan")
	}
	if err := p.Verify(); err != nil {
		return err
	}
	if err := a.Verify(); err != nil {
		return err
	}
	if a.Receipt.PlanFingerprint != p.Fingerprint || p.EngineVersion != plan.EngineVersion || p.TemplateVersion != templates.Version || p.TemplateSHA256 != templates.Digest() || a.Receipt.Execution.GoVersion != "go"+p.Config.GoVersion || !reflect.DeepEqual(a.Receipt.Execution.Environment, p.Environment) {
		return fmt.Errorf("materialize: receipt/plan/execution mismatch")
	}
	versions := templates.ToolVersions()
	versions["go"] = "go" + p.Config.GoVersion
	for name, version := range versions {
		identity, ok := a.Receipt.Execution.Tools[name]
		digest, err := hex.DecodeString(identity.SHA256)
		if !ok || identity.Version != version || err != nil || len(digest) != 32 {
			return fmt.Errorf("materialize: invalid tool identity %s", name)
		}
	}
	return nil
}

// Materialize owns and always removes its workspace. No partial result escapes.
// It never writes source/cache, runs upstream hooks, or persists binaries/logs.
func Materialize(ctx context.Context, p *plan.Plan, source profile.Source) (*Artifact, error) {
	return MaterializeAt(ctx, p, source, "")
}

// MaterializeAt allows each worker to choose its workspace volume without global environment mutation.
func MaterializeAt(ctx context.Context, p *plan.Plan, source profile.Source, root string) (result *Artifact, err error) {
	assembled, err := Assemble(p, source)
	if err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(root, "gostarter-materialize-")
	if err != nil {
		return nil, err
	}
	defer func() {
		if cleanupErr := os.RemoveAll(stage); cleanupErr != nil {
			result = nil
			err = errors.Join(err, fmt.Errorf("materialize: cleanup workspace: %w", cleanupErr))
		}
	}()
	work := filepath.Join(stage, "project")
	if err = WriteNew(work, assembled); err != nil {
		return nil, err
	}
	before, err := executionIdentity(ctx, work, p)
	if err != nil {
		return nil, err
	}
	if err = GenerateAndBuild(ctx, work, p); err != nil {
		return nil, err
	}
	after, err := executionIdentity(ctx, work, p)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(before, after) {
		return nil, fmt.Errorf("materialize: tool/include/environment changed during generation")
	}
	output, err := readTree(work)
	if err != nil {
		return nil, err
	}
	files, err := auditOutput(p, source, assembled, output)
	if err != nil {
		return nil, err
	}
	receipt := Receipt{PlanFingerprint: p.Fingerprint, Execution: after, Files: files, Excluded: p.SortedFiles(profile.Exclude), ContentSHA256: plan.ContentDigest(output)}
	receipt.IdentitySHA256 = receipt.identityDigest()
	artifact := &Artifact{Files: output, Receipt: receipt}
	if err = artifact.VerifyFor(p); err != nil {
		return nil, err
	}
	return artifact, nil
}
