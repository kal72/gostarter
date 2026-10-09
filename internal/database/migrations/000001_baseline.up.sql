-- Idempotent baseline also adopts existing, unversioned installations.
BEGIN;
CREATE TABLE IF NOT EXISTS admin_users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(100) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL
		);
CREATE TABLE IF NOT EXISTS admin_sessions (
			token VARCHAR(128) PRIMARY KEY,
			user_id INT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL
		);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON admin_sessions(expires_at);
CREATE TABLE IF NOT EXISTS admin_settings (
			key VARCHAR(100) PRIMARY KEY,
			value TEXT NOT NULL
		);

CREATE TABLE IF NOT EXISTS template_snapshots (
 id TEXT PRIMARY KEY CHECK (id ~ '^[a-f0-9]{64}$'),
 repository TEXT NOT NULL, source_ref TEXT NOT NULL,
 commit_sha TEXT NOT NULL CHECK (commit_sha ~ '^[a-f0-9]{40}$'),
 archive_sha256 TEXT NOT NULL CHECK (archive_sha256 ~ '^[a-f0-9]{64}$'),
 content_sha256 TEXT NOT NULL CHECK (content_sha256 ~ '^[a-f0-9]{64}$'),
 archive_key TEXT NOT NULL UNIQUE CHECK (archive_key = id || '.zip'),
 manifest JSONB NOT NULL, compatibility JSONB NOT NULL,
 status TEXT NOT NULL DEFAULT 'downloaded' CHECK (status IN ('downloaded','validating','ready','failed')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS upstream_state (
 source_key TEXT PRIMARY KEY, repository TEXT NOT NULL, source_ref TEXT NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1,
 active_snapshot_id TEXT REFERENCES template_snapshots(id) ON DELETE RESTRICT,
 candidate_snapshot_id TEXT REFERENCES template_snapshots(id) ON DELETE RESTRICT,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS generation_plans (
 id TEXT PRIMARY KEY CHECK (id ~ '^[a-f0-9]{64}$'),
 snapshot_id TEXT NOT NULL REFERENCES template_snapshots(id) ON DELETE RESTRICT,
 fingerprint TEXT NOT NULL CHECK (fingerprint ~ '^[a-f0-9]{64}$'),
 plan JSONB NOT NULL, template_version TEXT NOT NULL, engine_version TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','validating','ready','failed','expired')),
 content_sha256 TEXT, validation_identity TEXT,
 diagnostic TEXT NOT NULL DEFAULT '' CHECK (octet_length(diagnostic)<=8192),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), expires_at TIMESTAMPTZ NOT NULL,
 CHECK (status <> 'ready' OR (content_sha256 ~ '^[a-f0-9]{64}$' AND validation_identity ~ '^[a-f0-9]{64}$' AND content_sha256 IS NOT NULL AND validation_identity IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS generation_plans_fingerprint ON generation_plans(fingerprint,expires_at);
CREATE INDEX IF NOT EXISTS generation_plans_snapshot_expiry ON generation_plans(snapshot_id, expires_at);
CREATE TABLE IF NOT EXISTS validation_jobs (
 id TEXT PRIMARY KEY, snapshot_id TEXT NOT NULL REFERENCES template_snapshots(id) ON DELETE RESTRICT,
 plan_id TEXT REFERENCES generation_plans(id) ON DELETE RESTRICT,
 source_key TEXT REFERENCES upstream_state(source_key) ON DELETE RESTRICT, source_revision BIGINT,
 status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','succeeded','failed','canceled')),
 attempts INT NOT NULL DEFAULT 0 CHECK (attempts>=0),
 lease_token TEXT, lease_until TIMESTAMPTZ,
 diagnostic TEXT NOT NULL DEFAULT '' CHECK (octet_length(diagnostic)<=8192),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK ((source_key IS NULL) = (source_revision IS NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS validation_jobs_pending_unique ON validation_jobs(snapshot_id, COALESCE(plan_id,''), COALESCE(source_key,''), COALESCE(source_revision,0)) WHERE status IN ('queued','running');
CREATE OR REPLACE FUNCTION gostarter_snapshot_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.id,NEW.repository,NEW.source_ref,NEW.commit_sha,NEW.archive_sha256,NEW.content_sha256,NEW.archive_key,NEW.manifest,NEW.compatibility) IS DISTINCT FROM ROW(OLD.id,OLD.repository,OLD.source_ref,OLD.commit_sha,OLD.archive_sha256,OLD.content_sha256,OLD.archive_key,OLD.manifest,OLD.compatibility) THEN RAISE EXCEPTION 'immutable snapshot identity'; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS snapshot_immutable ON template_snapshots;
CREATE TRIGGER snapshot_immutable BEFORE UPDATE ON template_snapshots FOR EACH ROW EXECUTE FUNCTION gostarter_snapshot_immutable();
CREATE OR REPLACE FUNCTION gostarter_plan_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.id,NEW.snapshot_id,NEW.fingerprint,NEW.plan,NEW.template_version,NEW.engine_version,NEW.created_at,NEW.expires_at) IS DISTINCT FROM ROW(OLD.id,OLD.snapshot_id,OLD.fingerprint,OLD.plan,OLD.template_version,OLD.engine_version,OLD.created_at,OLD.expires_at) THEN RAISE EXCEPTION 'immutable plan input'; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS plan_immutable ON generation_plans;
CREATE TRIGGER plan_immutable BEFORE UPDATE ON generation_plans FOR EACH ROW EXECUTE FUNCTION gostarter_plan_immutable();



ALTER TABLE upstream_state ADD COLUMN IF NOT EXISTS candidate_epoch BIGINT NOT NULL DEFAULT 0;
ALTER TABLE upstream_state ADD COLUMN IF NOT EXISTS active_validation_key TEXT NOT NULL DEFAULT '';
ALTER TABLE upstream_state ADD COLUMN IF NOT EXISTS sync_token TEXT;
ALTER TABLE upstream_state ADD COLUMN IF NOT EXISTS sync_until TIMESTAMPTZ;
ALTER TABLE upstream_state ADD COLUMN IF NOT EXISTS next_sync_at TIMESTAMPTZ NOT NULL DEFAULT 'infinity';
ALTER TABLE upstream_state ALTER COLUMN next_sync_at SET DEFAULT 'infinity';
UPDATE upstream_state SET next_sync_at='infinity' WHERE next_sync_at<>'infinity';
ALTER TABLE upstream_state ADD COLUMN IF NOT EXISTS sync_requested BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE upstream_state ADD COLUMN IF NOT EXISTS sync_manual_lease BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE upstream_state ADD COLUMN IF NOT EXISTS last_sync_error TEXT NOT NULL DEFAULT '' CHECK(octet_length(last_sync_error)<=8192);
ALTER TABLE validation_jobs ADD COLUMN IF NOT EXISTS validation_key TEXT NOT NULL DEFAULT '';
ALTER TABLE validation_jobs ADD COLUMN IF NOT EXISTS candidate_epoch BIGINT NOT NULL DEFAULT 0;
ALTER TABLE validation_jobs ADD COLUMN IF NOT EXISTS result_digest TEXT;
DROP INDEX IF EXISTS validation_jobs_pending_unique;
CREATE UNIQUE INDEX validation_jobs_pending_unique ON validation_jobs(snapshot_id,COALESCE(plan_id,''),COALESCE(source_key,''),COALESCE(source_revision,0),candidate_epoch,validation_key) WHERE status IN ('queued','running');
CREATE INDEX IF NOT EXISTS validation_jobs_claim ON validation_jobs(status,lease_until,created_at);

UPDATE admin_settings SET value = 'dual' WHERE key = 'default_protocol' AND LOWER(BTRIM(value)) = 'rest';
COMMIT;
