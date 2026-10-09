# Web stack

The application uses Chi 5.3.2, templ 0.3.1070, HTMX 2.0.11, Alpine.js 3.17.4, and compiled Tailwind CSS 3.4.17. Tailwind stays on the existing design's major version to preserve its utilities and appearance; Play CDN is removed. Versions are pinned in go.mod, package.json, package-lock.json, and the Go tool directive.

## Ownership

- `internal/web/handler/router.go`: Chi routing for full pages, HTML fragments, JSON APIs, and public static assets. Unknown routes return 404.
- `internal/web/ui/*.templ`: shared document layout, home, login, dashboard, password dialog, status/snapshot fragments, extension cards. Go components escape values by default.
- `internal/web/handler/pages.go`: HTML/form transport, server-rendered admin authentication and actions. Settings validation is shared with the JSON transport.
- HTMX: admin form submissions, explicit sync and rollback, read-only status refreshes, modal catalog fragments. Status refresh does not request upstream sync.
- Alpine: theme, password visibility/dialog, and extension card events. These local interactions do not decide backend validation or dependency rules.
- `assets/js/app.js` and `plan-state.js`: specialized project configuration, abortable async plan resolution, preview, clipboard, and binary download. Binary responses stay outside HTML swapping. JSON API clients remain supported.
- Tailwind: class scanning covers templ and JavaScript; design tokens come from the original page configuration. Assets are bundled locally into the Go binary. Google Fonts remains external with font fallbacks.

## Build

Node.js 22+, npm, and the Go version from go.mod are needed for source development. `make start` regenerates templ components and builds local assets before compiling the single application binary. `make dev` also prepares the UI. `make ui` regenerates only frontend outputs; `make generate` (using the pinned `go tool templ`) and `make assets` are available separately.

Commit `.templ`, generated `*_templ.go`, compiled Tailwind CSS, vendored browser bundles, and package-lock.json. Ignore node_modules. Direct `go build ./...` can use checked-in outputs without Node; run `make ui` after template/style changes. CI regenerates and checks these outputs. Docker has a Node asset stage and generates templ components in its Go build stage; Node is absent from the runtime image.

## Sessions and failures

The HTML admin uses the existing database session through an HttpOnly cookie, not a token stored in localStorage. Authenticated HTML routes enforce session checks. HTML mutations validate same-origin Origin/Referer and bound form payload size. HTMX redirects expired sessions to login and displays HTML errors. API authentication and JSON payloads remain available for existing clients. State-changing forms disable their submit controls while requests run.

## Verification scope

Go unit tests cover routing, local assets, session boundaries, same-origin form rejection, component escaping, responsive layout markup, and locked dependency actions. Node unit tests cover plan lifecycle and Alpine card events. A temporary local preview with sample data was inspected in the browser for home/modal/admin layout and interactions. No new PostgreSQL integration suite or testdata fixture was introduced. Live database login/sync/generation and Docker image build require their existing runtime environment and have not been exercised by this refactor's preview.

References: [Chi](https://github.com/go-chi/chi), [templ](https://templ.guide/), [HTMX](https://htmx.org/docs/), [Alpine](https://alpinejs.dev/essentials/installation), [Tailwind 3 CLI](https://v3.tailwindcss.com/docs/installation).

## Browser interaction and output requests

The home page fetches catalog metadata once on load. Search, category filters, extension selection and dependency locking are computed in the browser without fragment or resolve requests. Form edits invalidate the current plan locally. Generate explicitly resolves and polls validation, then requests output; a ready unexpired plan is reused until the form changes. Preview sends configuration directly to source assembly without queueing validation or running codegen/build. Generated file paths are shown with explicit placeholders, and the response is labeled `preview_level: source`; preview is not a validated downloadable artifact. Server validation remains authoritative. The cURL popup contains only curl, unzip and ZIP removal chained with `&&`; extraction/removal stop when an earlier command fails.

## Caching and cancellation

Public assets use content-derived ETags and `public, no-cache`; browser revalidation prevents stale assets after deployment. Catalog responses use `private, no-cache` and ETags. The project service caches one serialized catalog; each lookup checks active snapshot status, validation authority and settings before reuse, avoiding repeated archive parsing/planning. A changed snapshot or settings invalidates that entry.

Preview requests use AbortController and a request revision to discard obsolete results. Form edits and navigation abort the active request. Browser memory retains up to three preview results, bounded to approximately 8 MiB of serialized text and two minutes per entry. Keys include catalog snapshot, engine/template versions and configuration. This short-lived source preview cache is not authority for Generate; downloads retain full server validation.

Recurring admin status requests are canceled while the document is hidden. Returning to a visible admin page triggers immediate status refresh. Explicit mutations are unaffected. Unit tests cover ETag responses, cache authority/settings invalidation, preview eviction/expiry and visibility behavior; live browser/network timing is not measured by those tests.

## Concurrent Preview requests

The project service uses single-flight to share source loading, planning and
assembly for identical concurrent Preview requests. The key includes the pinned
snapshot, complete request, engine version, template digest and validation key.
Different snapshots or configurations remain independent. Completed results and
errors are not cached by this mechanism; the existing browser cache remains.

Each caller can cancel independently. Shared work uses a separate context with a
15-second deadline, so the first caller disconnecting does not cancel followers.
If all callers disconnect, that bounded work may still complete. Each response
receives its own map and byte slices. Before returning, the service rechecks plan
expiry, active snapshot selection (for configuration requests), snapshot readiness
and validation authority. Generation and the validation queue are unchanged.

## Sync during validation

Admin Sync is disabled while any validation job has status `running`, including
project validation jobs that are not listed in the snapshot panel. The overview
exposes `validation_running`, and the server returns HTTP 409 with code
`validation_running` for conflicting API requests. The check runs under the same
PostgreSQL advisory transaction lock used by validation claims. Sync acquisition
also checks running jobs, so an already queued manual request waits if validation
starts before the worker acquires it. Completed, failed and queued validation jobs
do not disable Sync. Existing duplicate-Sync protection remains in place.
