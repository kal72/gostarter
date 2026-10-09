package project

import (
	"context"
	"gostarter/internal/project/plan"
	"gostarter/internal/project/templates"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
	"strings"
)

func (h *Projects) Catalog(ctx context.Context) (map[string]any, error) {
	record, files, err := h.Active(ctx)
	if err != nil {
		return nil, ErrSnapshotUnavailable
	}
	catalog, err := manifest.Parse(files[manifest.FileName])
	if err != nil {
		return nil, err
	}
	adapter, err := profile.Registry("go-feature-based-clean", "1", catalog)
	if err != nil {
		return nil, err
	}
	report, err := profile.ValidateSource(catalog, adapter, files)
	if err != nil {
		return nil, err
	}
	cfg := plan.Config{Module: "github.com/example/my-service", ServiceName: "my-service", GoVersion: report.GoVersion, Protocol: "dual", ProjectVersion: "0.1.0", HTTPPort: "8080", GRPCPort: "50051"}
	settings, err := h.database.GetSettingsContext(ctx)
	if err != nil {
		return nil, err
	}
	cfg.Module = strings.TrimSuffix(settings.DefaultGroup, "/") + "/" + settings.DefaultService
	cfg.ServiceName = settings.DefaultService
	cfg.Protocol = settings.DefaultProtocol
	if settings.DefaultGoVersion != "" {
		cfg.GoVersion = settings.DefaultGoVersion
	}
	warnings := []string{}
	cfg, err = plan.Normalize(cfg, report.GoVersion)
	if err != nil {
		return nil, &Failure{503, "defaults_incompatible", "Admin defaults are incompatible with the active profile"}
	}
	if err = adapter.CheckCompatibility(cfg.GoVersion, cfg.Protocol); err != nil || cfg.GoVersion != report.GoVersion {
		cfg.GoVersion = report.GoVersion
		warnings = append(warnings, "Saved Go default is not validated for this source; using active source version")
	}
	defaults, err := plan.Build(files, record.Metadata.Identity(), cfg)
	if err != nil {
		return nil, &Failure{503, "defaults_incompatible", "Admin defaults are incompatible with the active profile"}
	}
	cfg = defaults.Config
	versions := []string{report.GoVersion}

	caps := []CapabilityItem{}
	for _, e := range catalog.Extensions {
		caps = append(caps, CapabilityItem{ID: e.ID, Name: e.Title, Category: e.Kind, Path: e.Path, Description: e.Description})
	}
	return map[string]any{"snapshot": record.Metadata.Identity(), "catalog": catalog, "default_config": cfg, "default_warnings": warnings, "source_go_version": report.GoVersion, "supported_go_versions": versions, "core": defaults.Resolved.Core, "protocols": []ProtocolOption{{ID: "dual", Name: "gRPC + REST Gateway"}, {ID: "grpc", Name: "gRPC"}}, "capabilities": caps, "engine_version": plan.EngineVersion, "template_version": templates.Version, "manifest_versions": []int{1}}, nil
}
