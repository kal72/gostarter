# Database migrations

GoStarter uses golang-migrate v4 with the PostgreSQL driver. Versioned SQL lives in
`internal/database/migrations` and is embedded in the application binary.
`schema_migrations` records the version and dirty state.

Startup applies pending migrations before seeding default admin credentials and
settings. To migrate without starting HTTP or workers, use:

```sh
make migrate
```

The same configuration and database credentials as the server are used. The
compiled binary also supports `gostarter -migrate-only`.

## Existing databases

Version 1 is an idempotent baseline of the previous inline migrations. Existing
unversioned databases apply it without dropping tables or resetting stored data.
It preserves the existing legacy `rest` default conversion to `dual` and disables
legacy automatic sync schedules. Default settings and admin seeding remain
separate from schema versioning and do not replace existing values.

Migration SQL runs inside an explicit transaction. golang-migrate provides the
PostgreSQL migration lock; migration failures stop startup. A dirty version is
never automatically forced or skipped: inspect the failed SQL and database state,
repair the issue, then use the golang-migrate CLI with the same migration files
for an intentional recovery. Migration lock wait and individual SQL execution
are limited to 30 seconds.

## New migrations

Add the next sequential pair, for example `000003_description.up.sql` and
`000003_description.down.sql`. Do not edit an applied migration; create a new
version. Keep multi-statement changes inside `BEGIN` / `COMMIT` where supported.
Rebuild the binary to include new files.

Startup and `make migrate` only apply upgrades. Down migrations are available for
explicit maintenance with the golang-migrate CLI. The baseline down migration
drops application tables and their data, so it must only be used intentionally.

Unit tests verify embedded migration discovery, version progression, repeated
upgrades, and rollback/reapplication using an in-memory driver. They do not
execute SQL against a live PostgreSQL database.
