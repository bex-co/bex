# w3 · m486 — Purging a service's edge cache, or switching between its two caching profiles, restarts no instance, as on Render

**Worker:** worker3 **Goal:** a manual purge and a switch between `origin-controlled` and `origin-controlled-all` empty or re-scope the edge cache while every instance keeps running, as on Render, where the cache lives on its CDN; a disk-bearing caching service no longer goes down on a purge. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Failing tests: a purge and a caching-profile switch change the pod template today | 30m | — |
| [t002](t002.md) | The edge-cache sidecar takes its purge generation and profile from a file it re-reads while running | 1h | t001 |
| [t003](t003.md) | The operator patches live pods on a purge or profile switch, and rolls no existing template | 1h | t002 |
| [t004](t004.md) | The dashboard's purge copy stops promising a restart (en/zh) | 30m | t003 |
| [t005](t005.md) | Record the rule in ADR004 § Edge caching and ADR018 | 20m | t003 |
| [t006](t006.md) | Ship and verify on production | 45m | t004, t005 |
| [t007](t007.md) | Render parity | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007 |
| [t010](t010.md) | Closeout | 10m | t008, t009 |

## Definition of done

- A purge through REST `POST /v1/services/{id}/cache/purge`, GraphQL `purgeServiceCache` or the dashboard leaves every pod UID and `restartCount` unchanged; a previously cached path reads `CF-Cache-Status: MISS` within the kubelet's annotation refresh (measured in t006) and `HIT` after that.
- A disk-bearing (Recreate) caching service has no unavailable window on a purge (projection test in t001/t003).
- A switch between the two caching profiles rolls no instance; turning caching off rolls none either (the sidecar passes through until the next deploy).
- The operator release that ships this changes no existing caching service's pod template.
- No dashboard edge-caching string says the cache restarts with the instances (en/zh).
- Verified on production in t006, recorded in this README.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18 (topic: one aspect of Render parity, web-service edge caching), item 1; filed by `/pmx them to w3` 2026-11-18. Evidence on `origin/main` `4e3c87031`:
  - `lego/backend/internal/apps/edge_cache.go:99-117`: `PurgeCache` stamps `app.bex.co/cache-purged-at`; `lego/operator/internal/controller/deployment_projection.go:821-826` copies it into the pod template, so every purge rolls every instance.
  - The profile reaches the sidecar as the env var `BEX_EDGE_CACHE_PROFILE` (`lego/operator/internal/controller/edge_cache.go`), so a profile switch rolls too.
  - A disk forces `Recreate` (`lego/operator/internal/controller/disk.go:96-102`), and neither `checkCacheProfile` nor the App CEL rules refuse caching on a disk-bearing web service, so a purge there is an outage.
  - The dashboard's toast says so: "Cache purged. It restarts with your service's instances." (`dashboard/src/features/services/locales/en.ts:2201`).
- **Goal linkage:** ADR008 § Render parity first. [render.com/docs/web-service-caching](https://render.com/docs/web-service-caching) (fetched 2026-11-18): "Edge caching is powered by the same global CDN as Render static sites"; a purge is triggered "from its Settings page"; nothing in it touches instances. No ADR018 row signs off a restart on purge.
- **Expected outcome:** purging or re-profiling a cache stops restarting the customer's service, and stops taking a disk-bearing one down.
- **Why now:** every purge on production restarts the service today, and a disk-bearing one goes down. The runtime purge generation built here is the signal `w3/m489` reuses for its per-service cache tier.
- **Design recommendation, not a user decision:** a downward-API annotations file rather than an admin endpoint (no listener reachable through the Service's port 19099, no credential, asynchronous like Render's `202`).
- **Sequencing:** `w6/m484` (todo) edits the same projection block to keep a stored template's sidecar image; t003 rebases onto it or keeps its rule.
- **Residual, owned by `w3/m489`:** turning caching on from `no-cache` still adds the sidecar, so it rolls once.
- **Render parity included:** the purge's observable behavior and the dashboard's edge-caching copy change.
