# Development utilities

These optional commands are not application processes. Run `make start` to use GoStarter; trigger upstream synchronization from Admin.

- `validate-upstream`: inspect a local upstream contract.
- `plan-project`: inspect resolution and file decisions from local source.
- `render-project`: materialize a local project for debugging; requires the build runtime.
- `capture-snapshot`: operator/debugging utility that writes a candidate snapshot; ordinary sync uses Admin instead.

Use `go run ./tools/<name> -h` for options. These tools are not included in the production image.
