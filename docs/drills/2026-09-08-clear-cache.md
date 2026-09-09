# Clear-cache deploys — 2026-09-08

Proves the w7/m88 contract: with registry caching enabled, an ordinary warm deploy may reuse prior layers; `clearCache=clear` rebuilds without importing them and still exports a fresh `:cache`; a later ordinary deploy can reuse that fresh cache. Reset intent is release-scoped (`app.bex.co/clear-cache-release-generation`), not a sticky App boolean.

## Behavior table (encoded)

| `clearCache` | `BEX_BUILD_CACHE` | native / Dockerfile | kpack / image-backed |
| --- | --- | --- | --- |
| omit / `do_not_clear` | `registry` | restore + import; export + save | accept; no BuildKit registry-cache phases |
| `clear` | `registry` | purge (best-effort) + **no** import; still export + save | accept; no invented cache effect |
| either | off / unset | accept; no cache phases | accept |

Lifecycle: retry of the same clear release keeps the marker; success consumes it; a superseding ordinary trigger deletes it; clear-cache dispatch deletes other active Jobs for the App so an older `cache-save` cannot reinstall pre-clear layers.

## Failure mode (pre-fix)

w3/m46 accepted the Render enum and documented it as a no-op because builds were cache-free. w7/m86 added registry import/export behind `BEX_BUILD_CACHE=registry`, so `clear` became an accepted and silently ignored promise: warm rebuilds still imported `<app>-cache:cache`.

## Fix

1. API stamps `app.bex.co/clear-cache-release-generation` to the opened release on `clear`; omission/`do_not_clear` remove any prior marker.
2. Operator sets `Options.SkipCacheImport` when the marker matches `releaseGeneration`.
3. Build Job: no `cache-restore` / no `--import-cache`; best-effort `cache-purge` (`skopeo delete`); keep `cache-save` + `--export-cache`.
4. On clear dispatch, delete sibling active build Jobs for the App; on successful build, consume the spent annotation.

## Unit / envtest coverage

- `TestBuildCacheSkipImportPurgesAndStillExports` — Job shape for SkipCacheImport.
- `TestClearCacheAppliesOnlyMatchingRelease` — release-scoped marker.
- `TestTriggerAcceptsClearCacheEnum` — stamp / clear annotations via shared Trigger.
- envtest `skips restore and purges on a clear-cache release when the gate is on`.

## Local / host proof

Host Docker's default `docker` driver rejected `--cache-to type=local` (`Cache export is not supported for the docker driver`), so a host-local warm→clear→warm cache round-trip was not re-measured here. The Job-shape unit tests and envtest dispatch above encode the same contract the build plane runs (omit import, purge, still export). Re-run with a containerd/BuildKit driver or the opt-in cluster drill once the mock cluster kubeconfig is healthy.

## Cluster drill

The local kind/CAPD kubeconfig available during this session failed TLS verification (`x509: certificate signed by unknown authority`), so an in-cluster warm→clear→warm Job timing drill was not re-executed here. Re-run once the mock cluster is healthy: stamp the clear-cache annotation (or trigger via API), confirm the Job has `cache-purge` and no `cache-restore`/`--import-cache`, then a subsequent ordinary deploy restores and hits.

## Scope boundary

Production `BEX_BUILD_CACHE` enablement remains inbox `w7/043`. Native env revision (m87) still composes: clear skips import entirely; later warm deploys still bust on `NativeEnvRevision` when build env changes.
