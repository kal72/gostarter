# Application architecture

The application is organized by function. The top-level `internal` directories are:

```text
internal/
  app/                 dependency wiring and lifecycle
  config/              application configuration
  auth/                authentication, sessions, repository contracts and user data
  upstream/            catalog/settings/sync logic and upstream data
    manifest/          strict upstream manifest parser
    profile/           ownership and architecture conventions
    snapshot/          immutable archive identity and storage primitives
    source/            upstream download and source reading
  project/             resolution, catalog, preview, generation, ZIP and project data
    plan/              canonical configuration and file decisions
    resolver/          dependency closure and compatibility
    render/            source assembly, code generation and build
    templates/         embedded application-owned templates
    protocol/          supported transport rules
    validation/        validation pipeline and authority key
  database/            PostgreSQL repository adapter, SQL and migrations
  storage/             archive persistence coordination
  worker/              manual sync and validation job processing
  web/
    handler/           HTTP requests, middleware, cookies, JSON and HTML responses
    ui/                display models and presentation helpers
      pages/           page composition
      layouts/         shared document layout
      components/      reusable template fragments
```

`app` constructs repositories, archive storage, application services, workers and HTTP handlers. HTTP handlers receive dependencies and do not create database connections or archive storage. Application logic defines the repository interfaces it needs; `database` implements them. Shared data belongs to `auth`, `upstream` or `project`, not a generic model package. Database type aliases retain the existing adapter API during the move.

Algorithm packages remain below their functional owner where dependencies require independent boundaries. This avoids import cycles between immutable snapshot identity, planning, rendering and orchestration. The former root `service`, `model` and `generator` packages are removed. Preview/ZIP utilities now belong to `project`.

`web/ui` does not import application services or persistence. Handlers map application data into display models. Template files are authoritative; run `make generate` after editing them. Tailwind, Docker and CI use `internal/web/ui`.

`internal/app/architecture_test.go` enforces forbidden imports between application functions, adapters and presentation. Verification uses unit tests without upstream fixtures or a running PostgreSQL instance.

Behavior remains unchanged: upstream sync is manual, HTTP and workers share one process, dependencies are automatic and locked, Preview assembles source without build validation, Generate validates full output, and upstream docs are included. Generated ZIP files are streamed rather than persisted.
