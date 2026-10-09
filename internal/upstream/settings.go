package upstream

import (
	"fmt"
	"gostarter/internal/project/plan"
	"gostarter/internal/project/protocol"
	"gostarter/internal/upstream/manifest"
	"gostarter/internal/upstream/profile"
	"strings"
)

// SettingsRepository defines persistence required by settings logic.
type SettingsRepository interface {
	SaveSettings(AdminSettings) error
}
type Admin struct{ database SettingsRepository }

func NewSettings(repository SettingsRepository) *Admin { return &Admin{database: repository} }

// saveAdminSettings keeps validation identical for JSON and HTML transports.
func (h *Admin) SaveSettings(settings *AdminSettings) error {
	if err := protocol.Validate(settings.DefaultProtocol); err != nil {
		return err
	}
	settings.DefaultProtocol = protocol.Normalize(settings.DefaultProtocol)
	if h.database == nil {
		return fmt.Errorf("database unavailable")
	}
	adapter, err := profile.Registry("go-feature-based-clean", "1", &manifest.Manifest{SchemaVersion: 1})
	if err != nil {
		return err
	}
	defaults, err := plan.Normalize(plan.Config{Module: strings.TrimSuffix(settings.DefaultGroup, "/") + "/" + settings.DefaultService, ServiceName: settings.DefaultService, GoVersion: settings.DefaultGoVersion, Protocol: settings.DefaultProtocol}, adapter.MinGo)
	if err != nil {
		return err
	}
	if err = adapter.CheckCompatibility(defaults.GoVersion, defaults.Protocol); err != nil {
		return err
	}
	settings.DefaultGoVersion = defaults.GoVersion
	return h.database.SaveSettings(*settings)
}
