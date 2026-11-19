# w9 · m492 — Object-storage BuildKit layer cache (`BEX_BUILD_CACHE=s3`), enabled permanently in production

**Worker:** worker9 **Goal:** Docker and native builds reuse cached BuildKit layers across deploys in production via off-Zot Wasabi S3 storage, and `clearCache: "clear"` ("Clear build cache & deploy") clears and rebuilds cold on every workspace with zero Zot PVC growth. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Provision Wasabi build-cache bucket, scoped IAM policy, and 30-day lifecycle rule; amend ADR060 D3 & ADR019 | 45m | — |
| [t002](t002.md) | Implement `BEX_BUILD_CACHE=s3` in `lego/operator/internal/build/` (`cache-restore`, `cache-purge`, `cache-save` syncing local OCI layout to `<tea-id>/<app-uid>/layer-cache/`) | 1h30m | t001 |
| [t003](t003.md) | Wire S3 build-cache config, per-App cleanup in App finalizer, and `clearCache` purge across services and Build Source carriers | 1h | t002 |
| [t004](t004.md) | Enable `BEX_BUILD_CACHE=s3` + sealed credential in production overlays and add `bex_build_cache_*` metrics/alerts | 45m | t003 |
| [t005](t005.md) | Production verification: cold→warm Dockerfile & native rebuild speedup, `clearCache: "clear"`, Build Source clear-cache, and zero Zot PVC growth | 1h | t004 |
| [t006](t006.md) | Render parity | 30m | t005 |
| [t007](t007.md) | Simplify | 20m | t006 |
| [t008](t008.md) | Test coverage | 45m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

## Definition of done

- On production with `BEX_BUILD_CACHE=s3` enabled for every workspace: a multi-stage Dockerfile service and a native service each run a cold build, then a second build on the same dependency lockfile hits `CACHED` layers and finishes materially faster, while Zot PVC usage adds zero cache repositories.
- Triggering a deploy with `clearCache: "clear"` (via REST, CLI `--clear-cache`, MCP, Dashboard, or `POST /v1/build-sources/{id}/builds`) purges the S3 cache prefix and rebuilds cold, then repopulates a fresh cache.
- Missing or corrupted S3 cache objects never fail a build; BuildKit pods hold zero S3 or registry credentials (preserving ADR060 D3's credential-isolation invariant); deleting an App removes its S3 cache prefix.
- ADR060 D3, ADR019, and ADR018 (`Runtime, build strategy + Root Directory` and `Trigger a deploy`) record the S3 backend and production enablement.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for one aspect of render parity` (Build Caching & Build Environment Parity, 2026-11-18, item 1); [ADR060 § D3](../../../docs/ADR060-build-worker-reliability-and-performance.md#d3--per-app-remote-build-cache-in-zot-registry-backend-modemax), [w7/done/047.md](../../w7/done/047.md), [ADR018](../../../docs/ADR018-render-parity.md) (`Runtime, build strategy + Root Directory`, `Trigger a deploy`).
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)) — automatic Docker/native build layer caching and functional `clearCache: "clear"` in every workspace (`render.com/docs/docker`, `render.com/docs/deploys`).
- **Expected outcome:** Cuts repeat Dockerfile/native build wall time (50–75% on dependency-heavy builds, per ADR060 measurements) and makes `Clear build cache & deploy` truthful in production without threatening Zot's 100 GiB PVC.
- **Why now:** `BEX_BUILD_CACHE` has been off in production since the 2026-10-26 rollback with no open follow-up, and an off-Zot S3 build-cache store is the storage foundation required by w9/m493 (`$XDG_CACHE_HOME`), w9/m494 (per-runtime dependency caches), and w8/m161 (`NODE_MODULES_CACHE`).
- **Render parity:** included (t006). Although no API schema field is added, this makes `clearCache: "clear"` effective across REST, GraphQL, MCP, CLI, Build Sources, and Dashboard and closes a customer-visible parity gap in ADR018.
