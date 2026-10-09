BEGIN;
CREATE TABLE validation_case_cache (
 input_sha256 TEXT PRIMARY KEY CHECK (input_sha256 ~ '^[a-f0-9]{64}$'),
 validation_key TEXT NOT NULL CHECK (validation_key ~ '^[a-f0-9]{64}$'),
 content_sha256 TEXT NOT NULL CHECK (content_sha256 ~ '^[a-f0-9]{64}$'),
 identity_sha256 TEXT NOT NULL CHECK (identity_sha256 ~ '^[a-f0-9]{64}$'),
 validated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 expires_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp() + interval '7 days'
);
CREATE INDEX validation_case_cache_expiry ON validation_case_cache(expires_at);
COMMIT;
