package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gostarter/internal/project/validation"
)

func (d *DB) LookupValidationCase(ctx context.Context, input string) (validation.Result, bool, error) {
	var result validation.Result
	err := d.conn.QueryRowContext(ctx, `SELECT content_sha256,identity_sha256 FROM validation_case_cache WHERE input_sha256=$1 AND validation_key=$2 AND expires_at>clock_timestamp()`, input, validation.Key()).Scan(&result.ContentSHA256, &result.IdentitySHA256)
	if errors.Is(err, sql.ErrNoRows) {
		return result, false, nil
	}
	return result, err == nil, err
}

func (d *DB) SaveValidationCase(ctx context.Context, input string, result validation.Result) error {
	if !validDigest(input) || !validDigest(result.ContentSHA256) || !validDigest(result.IdentitySHA256) {
		return fmt.Errorf("invalid validation case evidence")
	}
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Separate lock bounds cache retention across concurrent workers.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(73410825)`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO validation_case_cache(input_sha256,validation_key,content_sha256,identity_sha256) VALUES($1,$2,$3,$4) ON CONFLICT(input_sha256) DO UPDATE SET validation_key=EXCLUDED.validation_key,content_sha256=EXCLUDED.content_sha256,identity_sha256=EXCLUDED.identity_sha256,validated_at=clock_timestamp(),expires_at=clock_timestamp()+interval '7 days'`, input, validation.Key(), result.ContentSHA256, result.IdentitySHA256); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM validation_case_cache WHERE expires_at<=clock_timestamp() OR input_sha256 IN (SELECT input_sha256 FROM validation_case_cache ORDER BY validated_at DESC,input_sha256 OFFSET 5000)`); err != nil {
		return err
	}
	return tx.Commit()
}
