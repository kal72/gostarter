# Incremental snapshot validation

Snapshot validation still constructs and checks every case in the complete
`dual` / `grpc` matrix. Each case includes the resolved transitive dependencies.
Build, code generation and smoke testing can reuse a previous successful result
when its input digest matches exactly. Validation of individual project plans
before download remains a full build and smoke test.

## Invalidation

The case digest includes:

- Effective configuration and resolved extension/dependency selection.
- All assembled source bytes and paths for that case.
- The complete upstream manifest and architecture profile identity.
- Shared/core/application source, upstream `go.mod` and `go.sum`.
- Validation policy, engine and embedded template identity.
- Controlled environment, server platform, actual Go/code-generation binary
  digests and protobuf include digests.

Changing an optional extension rebuilds cases that include it, including those
that require it transitively and the all-extensions case. Cases that exclude it
can reuse evidence. Changing shared inputs, manifest, module dependencies,
architecture, templates or execution identity invalidates the complete matrix.
Adding/removing source files changes the corresponding digest as well.

Static source/contract checks and planning always run. Actual tool identity is
checked before and after the matrix, including a matrix with only cache hits;
newly built cases check it before saving evidence. Reused evidence contributes to
a new aggregate digest bound to the current snapshot. Snapshot activation still
requires the worker's existing lease, candidate revision and validator fences.

## Persistence and limits

Migration `000002_validation_case_cache` creates a PostgreSQL cache containing
input, validator, output and evidence digests with timestamps. It stores no
generated ZIPs, binaries or command logs. Successful evidence expires after seven
days; writes prune expired rows and retain at most 5,000 entries across workers.
Failed/canceled cases are never saved. Successful cases from an otherwise failed
matrix can be reused independently after the failing input is fixed.

A cache miss, expired entry, invalid digest or read error causes full execution
of that case. Cache writes are best-effort; unavailable caching does not count as
successful validation. An execution or smoke-test failure still fails the job.

Logs provide a compact count, for example:

```text
[Validation] Snapshot matrix: 4 executed, 12 reused, 16 total
```

## Rollout

Restart the application to apply migration 2, or use `make migrate` first.
Validation policy version is now 2, so previous snapshot approvals require a
manual Admin Sync. The first validation warms the cache with a full matrix;
subsequent unchanged or partially changed inputs can reuse successful cases.

Unit tests cover unchanged reuse, transitive invalidation, shared-input and tool
invalidation, snapshot-bound aggregate identity, corrupted evidence, read errors,
failure handling and cancellation. They use in-memory evidence and do not execute
upstream builds or PostgreSQL integration tests. Production timing improvements
have not been benchmarked.
