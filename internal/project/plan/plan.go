// Package plan builds deterministic, static generation plans without materializing output.
package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"gostarter/internal/project/protocol"
	"gostarter/internal/project/resolver"
	"gostarter/internal/project/templates"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
)

// EngineVersion changes when canonical planning semantics change.
const EngineVersion = "4"

// Config contains metadata and explicit catalog selections, never per-extension booleans.
type Config struct {
	Module              string   `json:"module"`
	ServiceName         string   `json:"service_name"`
	ProjectVersion      string   `json:"project_version"`
	Description         string   `json:"description"`
	GoVersion           string   `json:"go_version"`
	Protocol            string   `json:"protocol"`
	HTTPPort            string   `json:"http_port"`
	GRPCPort            string   `json:"grpc_port"`
	RequestedExtensions []string `json:"requested_extensions"`
}

// Snapshot pins source content. Fixture identities never claim an upstream commit.
// Immutable identities are supplied by trusted snapshot storage (stage 4), not proven by this type.
type Snapshot struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	CommitSHA     string `json:"commit_sha,omitempty"`
	ContentSHA256 string `json:"content_sha256"`
}

// FileAction has exactly one producer and records excluded source for auditability.
type FileAction struct {
	Path   string         `json:"path"`
	Owner  string         `json:"owner"`
	Action profile.Action `json:"action"`
	Reason string         `json:"reason"`
}

// Plan is the canonical input to future rendering, preview and persistence.
type Plan struct {
	SchemaVersion   int               `json:"schema_version"`
	EngineVersion   string            `json:"engine_version"`
	Snapshot        Snapshot          `json:"snapshot"`
	ArchitectureID  string            `json:"architecture_id"`
	ProfileRevision string            `json:"profile_revision"`
	ProfileSHA256   string            `json:"profile_sha256"`
	TemplateSHA256  string            `json:"template_sha256"`
	TemplateVersion string            `json:"template_version"`
	ValidationLevel string            `json:"validation_level"`
	Environment     map[string]string `json:"environment"`
	Config          Config            `json:"config"`
	Resolved        resolver.Result   `json:"resolved"`
	Files           []FileAction      `json:"files"`
	Fingerprint     string            `json:"fingerprint,omitempty"`
}

func fail(code, symbol, cause string) error {
	return manifest.NewDiagnostic(code, "", "", symbol, cause)
}

// ContentDigest hashes ordered paths and bytes with unambiguous JSON framing.
func ContentDigest(source profile.Source) string {
	data, _ := json.Marshal(source)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// FixtureIdentity explicitly labels a local analysis tree without guessing its commit.
func FixtureIdentity(id string, source profile.Source) Snapshot {
	return Snapshot{ID: id, Kind: "fixture", ContentSHA256: ContentDigest(source)}
}

var fullSHA = regexp.MustCompile(`^[0-9a-f]{40}(?:[0-9a-f]{24})?$`)
var serviceName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
var digits = regexp.MustCompile(`^[0-9]+$`)

// Build validates actual source, resolves intent, and returns a content-addressed static plan.
func Build(source profile.Source, snapshot Snapshot, config Config) (*Plan, error) {
	if snapshot.ID == "" || strings.TrimSpace(snapshot.ID) != snapshot.ID {
		return nil, fail("invalid_snapshot", "snapshot.id", "nonempty canonical identity is required")
	}
	if snapshot.Kind != "fixture" && snapshot.Kind != "immutable" {
		return nil, fail("invalid_snapshot", "snapshot.kind", "expected fixture or immutable")
	}
	if snapshot.Kind == "fixture" && snapshot.CommitSHA != "" || snapshot.Kind == "immutable" && !fullSHA.MatchString(snapshot.CommitSHA) {
		return nil, fail("invalid_snapshot", "snapshot.commit_sha", "fixture must not claim a commit; immutable source requires full commit SHA")
	}
	if snapshot.ContentSHA256 != ContentDigest(source) {
		return nil, fail("snapshot_mismatch", "snapshot.content_sha256", "source bytes do not match the snapshot identity")
	}
	catalog, err := manifest.Parse(source[manifest.FileName])
	if err != nil {
		return nil, err
	}
	adapter, err := profile.Registry("go-feature-based-clean", "1", catalog)
	if err != nil {
		return nil, err
	}
	report, err := profile.ValidateSource(catalog, adapter, source)
	if err != nil {
		return nil, err
	}
	effective, err := Normalize(config, report.GoVersion)
	if err != nil {
		return nil, err
	}
	result, err := resolver.Resolve(catalog, adapter, effective.RequestedExtensions, effective.GoVersion, effective.Protocol)
	if err != nil {
		return nil, err
	}
	effective.RequestedExtensions = append([]string{}, result.RequestedExtensions...)
	active := map[string]bool{}
	for _, e := range result.Extensions {
		active[e.ID] = true
	}
	descriptor, _ := json.Marshal(adapter)
	sum := sha256.Sum256(descriptor)
	out := &Plan{SchemaVersion: 1, EngineVersion: EngineVersion, Snapshot: snapshot, ArchitectureID: adapter.ArchitectureID, ProfileRevision: adapter.Revision, ProfileSHA256: hex.EncodeToString(sum[:]), TemplateVersion: adapter.TemplateVersion, TemplateSHA256: templates.Digest(), ValidationLevel: "static_contract", Config: effective, Environment: GenerationEnvironment(effective.GoVersion), Resolved: *result, Files: []FileAction{}}
	for _, file := range report.Files {
		action := file.Action
		reason := "profile_default"
		if _, optional := catalog.Lookup(file.Owner); optional {
			if active[file.Owner] {
				reason = "resolved_extension"
			} else {
				action = profile.Exclude
				reason = "extension_not_selected"
			}
		} else if action == profile.Exclude {
			reason = "profile_excluded"
		} else if action == profile.Render || action == profile.Generate {
			reason = "application_producer"
		}
		if profile.TransportExcludes(file.Path, effective.Protocol) {
			action = profile.Exclude
			reason = "protocol_not_selected"
		}
		out.Files = append(out.Files, FileAction{file.Path, file.Owner, action, reason})
	}
	out.Fingerprint = out.ComputeFingerprint()
	return out, nil
}

// Normalize canonicalizes metadata while rejecting unsupported or unsafe values.
func Normalize(c Config, sourceGo string) (Config, error) {
	c.Module = strings.TrimSpace(c.Module)
	if c.Module == "" {
		c.Module = "github.com/example/my-service"
	}
	if err := module.CheckPath(c.Module); err != nil {
		return c, fail("invalid_metadata", "module", err.Error())
	}
	c.ServiceName = strings.TrimSpace(c.ServiceName)
	if c.ServiceName == "" {
		c.ServiceName = path.Base(c.Module)
	}
	if len(c.ServiceName) > 63 || !serviceName.MatchString(c.ServiceName) {
		return c, fail("invalid_metadata", "service_name", "expected lowercase service name with optional hyphens, at most 63 characters")
	}
	c.ProjectVersion = strings.TrimPrefix(strings.TrimSpace(c.ProjectVersion), "v")
	if c.ProjectVersion == "" {
		c.ProjectVersion = "0.1.0"
	}
	canonical := semver.Canonical("v" + c.ProjectVersion)
	if canonical == "" {
		return c, fail("invalid_metadata", "project_version", "expected semantic version")
	}
	c.ProjectVersion = strings.TrimPrefix(canonical, "v") + semver.Build("v"+c.ProjectVersion)
	c.Description = strings.TrimSpace(c.Description)
	if !utf8.ValidString(c.Description) || utf8.RuneCountInString(c.Description) > 512 || strings.ContainsAny(c.Description, "\r\n") {
		return c, fail("invalid_metadata", "description", "expected single-line text, at most 512 characters")
	}
	c.GoVersion = strings.TrimSpace(c.GoVersion)
	if c.GoVersion == "" || c.GoVersion == "source" {
		c.GoVersion = sourceGo
	}
	c.Protocol = protocol.Normalize(c.Protocol)
	for _, entry := range []struct {
		value    *string
		fallback string
	}{{&c.HTTPPort, "8080"}, {&c.GRPCPort, "50051"}} {
		*entry.value = strings.TrimSpace(*entry.value)
		if *entry.value == "" {
			*entry.value = entry.fallback
		}
		n, err := strconv.Atoi(*entry.value)
		if err != nil || !digits.MatchString(*entry.value) || n < 1 || n > 65535 {
			return c, fail("invalid_metadata", "port", "expected TCP port between 1 and 65535")
		}
		*entry.value = strconv.Itoa(n)
	}
	if c.Protocol == "dual" && c.HTTPPort == c.GRPCPort {
		return c, fail("invalid_metadata", "ports", "dual requires distinct HTTP and gRPC ports")
	}
	c.RequestedExtensions = append([]string(nil), c.RequestedExtensions...)
	return c, nil
}

// ComputeFingerprint includes intent, versions, source and file decisions, excluding itself.
func (p Plan) ComputeFingerprint() string {
	p.Fingerprint = ""
	data, _ := json.Marshal(p)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Verify detects changed metadata or file decisions after plan creation/persistence.
func (p Plan) Verify() error {
	if p.Fingerprint == "" || p.Fingerprint != p.ComputeFingerprint() {
		return fail("plan_mismatch", "fingerprint", "plan changed or fingerprint is missing")
	}
	return nil
}

// SortedFiles returns detached stable decisions for a particular producer.
func (p Plan) SortedFiles(action profile.Action) []FileAction {
	out := []FileAction{}
	for _, file := range p.Files {
		if file.Action == action {
			out = append(out, file)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// GenerationEnvironment is part of plan identity. Source generation and build use
// one target regardless of the host; runtime smoke tests use their native host.
func GenerationEnvironment(goVersion string) map[string]string {
	return map[string]string{
		"GOOS": "linux", "GOARCH": "amd64", "GOAMD64": "v1", "CGO_ENABLED": "0",
		"GOTOOLCHAIN": "go" + goVersion, "GOENV": "off", "GOWORK": "off", "GOFLAGS": "",
		"GOEXPERIMENT": "", "GODEBUG": "", "GOPROXY": "off", "GOSUMDB": "sum.golang.org",
		"GOPRIVATE": "", "GONOSUMDB": "", "GONOPROXY": "", "GOVCS": "*:off",
		"GO111MODULE": "on", "TZ": "UTC", "LANG": "C", "LC_ALL": "C", "SOURCE_DATE_EPOCH": "0",
	}
}
