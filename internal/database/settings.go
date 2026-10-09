package database

import (
	"context"
	"fmt"
	"gostarter/internal/project/protocol"
	"strings"
)

// ─── Admin Settings Methods ───────────────────────────────────────────────────

// GetSettings retrieves current initializr settings from PostgreSQL
func (d *DB) GetSettings() (AdminSettings, error) {
	return d.GetSettingsContext(context.Background())
}

func (d *DB) GetSettingsContext(ctx context.Context) (AdminSettings, error) {
	settings := AdminSettings{
		DefaultGroup:     "github.com/example",
		DefaultService:   "my-service",
		DefaultGoVersion: "1.27.1",
		DefaultProtocol:  "dual",
		UpstreamRepo:     "kal72/go-feature-based-boilerplate",
		UpstreamBranch:   "main",
	}

	if d.conn == nil {
		return settings, nil
	}

	rows, err := d.conn.QueryContext(ctx, `SELECT key, value FROM admin_settings`)
	if err != nil {
		return settings, err
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		switch k {
		case "default_group":
			settings.DefaultGroup = v
		case "default_service":
			settings.DefaultService = v
		case "default_go_version":
			settings.DefaultGoVersion = v
		case "default_protocol":
			settings.DefaultProtocol = v
		case "upstream_repo":
			settings.UpstreamRepo = v
		case "upstream_branch":
			settings.UpstreamBranch = v
		}
	}

	if err := rows.Err(); err != nil {
		return settings, err
	}
	if err := protocol.Validate(settings.DefaultProtocol); err != nil {
		return settings, err
	}
	settings.DefaultProtocol = protocol.Normalize(settings.DefaultProtocol)
	return settings, nil
}

// SaveSettings persists updated initializr settings to PostgreSQL
func (d *DB) SaveSettings(settings AdminSettings) error {
	if err := protocol.Validate(settings.DefaultProtocol); err != nil {
		return err
	}
	settings.UpstreamRepo = strings.ToLower(settings.UpstreamRepo)
	if err := validateSource(settings.UpstreamRepo, settings.UpstreamBranch); err != nil {
		return err
	}
	settings.DefaultProtocol = protocol.Normalize(settings.DefaultProtocol)
	if d.conn == nil {
		return fmt.Errorf("database not connected")
	}

	mapping := map[string]string{
		"default_group":      settings.DefaultGroup,
		"default_service":    settings.DefaultService,
		"default_go_version": settings.DefaultGoVersion,
		"default_protocol":   settings.DefaultProtocol,
		"upstream_repo":      settings.UpstreamRepo,
		"upstream_branch":    settings.UpstreamBranch,
	}

	tx, err := d.storageTx(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO admin_settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for k, v := range mapping {
		if _, err := stmt.Exec(k, v); err != nil {
			return err
		}
	}

	_, err = tx.Exec(`INSERT INTO upstream_state(source_key,repository,source_ref) VALUES('primary',$1,$2) ON CONFLICT(source_key) DO UPDATE SET revision=upstream_state.revision+CASE WHEN upstream_state.repository<>$1 OR upstream_state.source_ref<>$2 THEN 1 ELSE 0 END,candidate_snapshot_id=CASE WHEN upstream_state.repository<>$1 OR upstream_state.source_ref<>$2 THEN NULL ELSE upstream_state.candidate_snapshot_id END,sync_requested=CASE WHEN upstream_state.repository<>$1 OR upstream_state.source_ref<>$2 THEN false ELSE upstream_state.sync_requested END,sync_token=CASE WHEN upstream_state.repository<>$1 OR upstream_state.source_ref<>$2 THEN NULL ELSE upstream_state.sync_token END,sync_until=CASE WHEN upstream_state.repository<>$1 OR upstream_state.source_ref<>$2 THEN NULL ELSE upstream_state.sync_until END,repository=$1,source_ref=$2`, settings.UpstreamRepo, settings.UpstreamBranch)
	if err != nil {
		return err
	}
	return tx.Commit()
}
