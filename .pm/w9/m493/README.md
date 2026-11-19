# w9 · m493 — Persistent `$XDG_CACHE_HOME` (`/opt/render/.cache`) across native builds, reset on `clearCache: "clear"`

**Worker:** worker9 **Goal:** every native build receives `XDG_CACHE_HOME=/opt/render/.cache`, persists its contents across deploys of the same service via w9/m492's S3 build-cache store without leaking into the final runtime image, and clears it on `clearCache: "clear"`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Set `XDG_CACHE_HOME=/opt/render/.cache` in native build recipes (`native.go`) and exclude `/opt/render/.cache` from final runtime stages | 45m | w9/m492 |
| [t002](t002.md) | Extend `cache-restore` / `cache-save` / `cache-purge` to round-trip `<tea-id>/<app-uid>/xdg-cache.tar.zst` via a dedicated named BuildKit context | 1h30m | t001 |
| [t003](t003.md) | Wire build-log cache status lines (`Restoring build cache` / `Uploading build cache` / `Build cache cleared`) and metrics; guard cross-tenant isolation & corrupt-archive fallback | 1h | t002 |
| [t004](t004.md) | Production verification on a native Next.js/Python fixture: cross-deploy `$XDG_CACHE_HOME` persistence, runtime image omission, and `--clear-cache` reset | 1h | t003 |
| [t005](t005.md) | Render parity | 30m | t004 |
| [t006](t006.md) | Simplify | 20m | t005 |
| [t007](t007.md) | Test coverage | 45m | t005, t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

- Every native build (web, private, worker, cron, static) sees `XDG_CACHE_HOME=/opt/render/.cache` pointing to a writable directory restored from the App's previous successful build.
- A file written to `$XDG_CACHE_HOME` during build 1 is present in build 2 after a source-only commit, is absent from the running service container, and is gone on a `clearCache: "clear"` deploy.
- Missing, oversized, or corrupt cache archives degrade cleanly to a cold build without failing the deploy; App deletion sweeps the object.
- ADR018 (`Runtime, build strategy + Root Directory`, `Default environment variables`), ADR060 D3, and `docs/render-artifacts/default-env-live.md` record `$XDG_CACHE_HOME`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for one aspect of render parity` (Build Caching & Build Environment Parity, 2026-11-18, item 2); code audit of `lego/operator/internal/build/native.go` and [default-env-live.md](../../../docs/render-artifacts/default-env-live.md) vs Render's documented `$XDG_CACHE_HOME` persistence (`render.com/docs/deploy-nextjs-app`, `render.com/docs/deploy-gatsby`, `render.com/docs/deploys`).
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)) — [ADR018](../../../docs/ADR018-render-parity.md) rows `Runtime, build strategy + Root Directory` and `Default environment variables`.
- **Expected outcome:** Standard Render Next.js/Gatsby/Astro/Python/Go build scripts that rely on `$XDG_CACHE_HOME` work unchanged on bex with real cross-deploy speedup.
- **Why now:** Builds directly on w9/m492's off-Zot S3 build-cache store and gives all native runtimes cross-deploy framework and XDG tool caching (cleanly distinct from `w8/blocked/m161`, which owns Node's `node_modules` directory and `NODE_MODULES_CACHE=true`).
- **Render parity:** included (t005). Adds a Render-documented build-time environment variable and persistent cache contract across native runtimes and updates ADR018 and `default-env-live.md`.
