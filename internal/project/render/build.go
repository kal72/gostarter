package render

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"gostarter/internal/project/plan"
	"gostarter/internal/project/templates"
	"gostarter/internal/upstream/profile"
)

// WriteNew writes only into a newly created directory. Callers own its cleanup.
func WriteNew(dir string, files profile.Source) (err error) {
	folded := map[string]bool{}
	for name := range files {
		key := strings.ToLower(name)
		if folded[key] {
			return fmt.Errorf("render: case-colliding output %s", name)
		}
		folded[key] = true
		if !validOutput(name) {
			return fmt.Errorf("render: unsafe output %q", name)
		}
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		mode := fs.FileMode(0644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0755
		}
		f, openErr := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if openErr != nil {
			return openErr
		}
		_, writeErr := f.Write(files[name])
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		if err = os.Chmod(target, mode); err != nil {
			return err
		}
	}
	return nil
}

// limitedOutput drains commands while retaining at most 8 KiB of diagnostics.
type limitedOutput struct{ bytes.Buffer }

func (b *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if left := 8192 - b.Len(); left > 0 {
		if len(p) > left {
			p = p[:left]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}

func run(ctx context.Context, dir string, env []string, bin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir, cmd.Env = dir, env
	cmd.WaitDelay = time.Second
	configureProcess(cmd)
	var output limitedOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", filepath.Base(bin), err, strings.TrimSpace(output.String()))
	}
	return strings.TrimSpace(output.String()), nil
}

// CommandEnv fixes module/toolchain behavior and uses cached dependencies only.
// GOCACHE/GOMODCACHE may be configured by the calling development environment.
func CommandEnv(goVersion string) []string {
	replacements := plan.GenerationEnvironment(goVersion)
	replacements["GOOS"], replacements["GOARCH"], replacements["CGO_ENABLED"] = runtime.GOOS, runtime.GOARCH, "1"
	return environment(replacements)
}

func environment(replacements map[string]string) []string {
	env := []string{}
	// Host locations are not content inputs. Ambient GOFLAGS, CGO flags, GOEXPERIMENT,
	// locale, proxy configuration, or application secrets never leak into generation.
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "TMP", "TEMP", "GOCACHE", "GOMODCACHE", "GOPATH", "SYSTEMROOT"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}

	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		env = append(env, key+"="+replacements[key])
	}
	return env
}

// CheckTools rejects mismatched executable versions before code generation.
// Go build information is checked for plugins; no upstream command is run.
func CheckTools(ctx context.Context, dir string, env []string) (map[string]string, error) {
	paths := map[string]string{}
	versions := templates.ToolVersions()
	names := make([]string, 0, len(versions))
	for name := range versions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		version := versions[name]
		bin, err := exec.LookPath(name)
		if err != nil {
			return nil, fmt.Errorf("required tool %s %s: %w", name, version, err)
		}
		paths[name] = bin
		if name == "protoc" {
			got, err := run(ctx, dir, env, bin, "--version")
			if err != nil {
				return nil, err
			}
			if got != version {
				return nil, fmt.Errorf("protoc: requires %s, found %s", version, got)
			}
		} else {
			info, err := buildinfo.ReadFile(bin)
			if err != nil {
				return nil, fmt.Errorf("tool %s: %w", name, err)
			}
			module := map[string]string{"wire": "github.com/google/wire", "protoc-gen-go": "google.golang.org/protobuf", "protoc-gen-go-grpc": "google.golang.org/grpc/cmd/protoc-gen-go-grpc", "protoc-gen-grpc-gateway": "github.com/grpc-ecosystem/grpc-gateway/v2", "protoc-gen-openapiv2": "github.com/grpc-ecosystem/grpc-gateway/v2"}[name]
			if info.Main.Path != module || info.Main.Version != version || info.Main.Replace != nil {
				return nil, fmt.Errorf("tool %s: requires %s@%s, found %s@%s", name, module, version, info.Main.Path, info.Main.Version)
			}
		}
	}
	return paths, nil
}

// GenerateAndBuild runs only application-defined commands in a disposable output
// workspace assembled by this package. It does not run upstream scripts or tests.
// The caller must discard the workspace on failure. No binary/log is retained.
func GenerateAndBuild(ctx context.Context, dir string, p *plan.Plan) error {
	if p == nil {
		return fmt.Errorf("build: missing plan")
	}
	if err := p.Verify(); err != nil {
		return err
	}
	if p.TemplateVersion != templates.Version || p.TemplateSHA256 != templates.Digest() || (p.Config.Protocol != "dual" && p.Config.Protocol != "grpc") || p.EngineVersion != plan.EngineVersion || !reflect.DeepEqual(p.Environment, plan.GenerationEnvironment(p.Config.GoVersion)) {
		return fmt.Errorf("build: unsupported template plan")
	}
	env := environment(p.Environment)
	tools, err := CheckTools(ctx, dir, env)
	if err != nil {
		return err
	}
	cache, err := run(ctx, dir, env, "go", "env", "GOMODCACHE")
	if err != nil {
		return err
	}
	gateway := filepath.Join(cache, "github.com/grpc-ecosystem/grpc-gateway/v2@v2.29.0")
	if _, err = os.Stat(filepath.Join(gateway, "protoc-gen-openapiv2/options/annotations.proto")); err != nil {
		return fmt.Errorf("cached grpc-gateway v2.29.0 proto include: %w", err)
	}
	for _, name := range []string{"gen/pb"} {
		if err = os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
			return err
		}
	}
	include, err := protocInclude(tools["protoc"])
	if err != nil {
		return err
	}
	if p.Config.Protocol == "dual" {
		if err = os.MkdirAll(filepath.Join(dir, "gen/openapi"), 0755); err != nil {
			return err
		}
	}
	args := []string{"-I", "api/proto/plugins", "-I", "api/proto", "-I", gateway, "-I", include}
	for _, name := range []string{"protoc-gen-go", "protoc-gen-go-grpc", "protoc-gen-grpc-gateway", "protoc-gen-openapiv2"} {
		args = append(args, "--plugin="+name+"="+tools[name])
	}
	args = append(args, "--go_out=gen/pb", "--go_opt=paths=source_relative", "--go-grpc_out=gen/pb", "--go-grpc_opt=paths=source_relative", "--grpc-gateway_out=gen/pb", "--grpc-gateway_opt=paths=source_relative")
	if p.Config.Protocol == "dual" {
		args = append(args, "--openapiv2_out=gen/openapi", "--openapiv2_opt=allow_merge=true,merge_file_name=api")
	}
	for _, f := range p.Files {
		if strings.HasPrefix(f.Path, "api/proto/") && !strings.HasPrefix(f.Path, "api/proto/plugins/") && strings.HasSuffix(f.Path, ".proto") && f.Action != profile.Exclude {
			args = append(args, f.Path)
		}
	}
	if _, err = run(ctx, dir, env, tools["protoc"], args...); err != nil {
		return err
	}
	if _, err = run(ctx, dir, env, "go", "mod", "tidy"); err != nil {
		return err
	}
	if _, err = run(ctx, dir, env, tools["wire"], "./bootstrap"); err != nil {
		return err
	}
	// Normalize requirements after the selected injector has been generated.
	if _, err = run(ctx, dir, env, "go", "mod", "tidy"); err != nil {
		return err
	}
	if _, err = run(ctx, dir, env, "go", "build", "-trimpath", "-mod=readonly", "./..."); err != nil {
		return err
	}
	return nil
}

func protocInclude(bin string) (string, error) {
	real, err := filepath.EvalSymlinks(bin)
	if err != nil {
		return "", err
	}
	include := filepath.Join(filepath.Dir(filepath.Dir(real)), "include")
	if _, err = os.Stat(filepath.Join(include, "google/protobuf/descriptor.proto")); err != nil {
		return "", fmt.Errorf("pinned protoc include: %w", err)
	}
	return include, nil
}

// RunSmoke executes only the application-owned mandatory test, never upstream tests/scripts.
func RunSmoke(ctx context.Context, dir, goVersion string) error {
	env := plan.GenerationEnvironment(goVersion)
	env["GOOS"] = runtime.GOOS
	env["GOARCH"] = runtime.GOARCH
	_, err := run(ctx, dir, environment(env), "go", "test", "-mod=readonly", "-timeout=30s", "-run=^TestGoStarterValidation$", "./bootstrap")
	return err
}
