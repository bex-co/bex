# w3 · m486 — Purging a service's edge cache, or switching between its two caching profiles, restarts no instance, as on Render

**Worker:** worker3 **Goal:** a manual purge and a switch between `origin-controlled` and `origin-controlled-all` empty or re-scope the edge cache while every instance keeps running, as on Render, where the cache lives on its CDN; a disk-bearing caching service no longer goes down on a purge. **Status:** done (2026-11-19; verified on production pin `7af50a28168d`, § Production record)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Failing tests: a purge and a caching-profile switch change the pod template today — **DONE** | 30m | — |
| [t002](done/t002.md) | The edge-cache sidecar takes its purge generation and profile from a file it re-reads while running — **DONE** | 1h | t001 |
| [t003](done/t003.md) | The operator patches live pods on a purge or profile switch, and rolls no existing template — **DONE** | 1h | t002 |
| [t004](done/t004.md) | The dashboard's purge copy stops promising a restart (en/zh) — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Record the rule in ADR004 § Edge caching and ADR018 — **DONE** | 20m | t003 |
| [t006](done/t006.md) | Ship and verify on production — **DONE** | 45m | t004, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 30m | t007 |
| [t010](done/t010.md) | Closeout — **DONE** | 10m | t008, t009 |

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

## Implementation record (2026-11-18)

- **Deviation, the profile was release identity.** Besides the env var the brainstorm found, `spec.cacheProfile` was `IdentityRelease` in `AppSpecIdentityClasses`, so a dashboard save opened a deploy row and minted a release generation (a new revision label, so a roll whatever the sidecar did), and a REST `PATCH` waited for the next deploy. Render's caching profile is a CDN setting that opens no deploy, so it is `IdentityOperational` now. The operator drops it from the release fingerprint, and `adoptCacheProfileReleaseIdentity` adopts every fingerprint an older operator computed with it (against the saved spec under each profile, else the release record's adopted spec), so the upgrade opens no release. `TestEdgeCacheSavesOpenNoDeploy` (backend) covers the dashboard verbs, REST `PATCH` and the purge.
- **Mechanism.** The sidecar reads its pod's `metadata.annotations` from a downward-API file every 2 s (`BEX_EDGE_CACHE_CONTROL`); the operator patches `app.bex.co/cache-purged-at` and `app.bex.co/cache-profile` on each live pod on every pass (`annotateEdgeCachePods`), and the App watch passes a purge stamp change at once. A switch empties the cache too (Render documents nothing either way). Turning caching off keeps the sidecar as a `no-cache` pass-through, with the Service direct to the app, until the next deploy.
- **Older templates.** A template stored before this release keeps its shape on the upgrade (`keptEdgeCache`), and its next purge, switch or other change rolls it onto the controlled shape. On production the operator image still moves every edge-cached service's sidecar image on each deploy until `w6/m484` lands, so the release that ships this rolls them once for that reason.
- **Copy.** The purge toast reads "Cache purge started." in all eight dashboard locales, not only en and zh.

## Render parity (t007, 2026-11-18)

Checked against the shipped code and its tests; the production behavior is t006's record. Reference: [render.com/docs/web-service-caching](https://render.com/docs/web-service-caching), fetched 2026-11-18.

| Surface | Render | bex after this milestone | Verdict |
| --- | --- | --- | --- |
| REST `POST /v1/services/{id}/cache/purge` | `202`, no body; purges the CDN, touches no instance | `202`, no body; the stamp reaches each running sidecar, no restart, no deploy row | matched |
| GraphQL `purgeServiceCache`, dashboard **Purge cache** | Settings page purge | same verb; toast "Cache purge started." in all eight locales | matched (Render's toast wording unverified) |
| REST `PATCH /v1/services/{id}` `serviceDetails.cache.profile` | no deploy documented | applies at once, no deploy row, no restart (was held for the next deploy) | matched |
| GraphQL `setCacheProfile`, dashboard **Save edge caching** | "Save changes", then "Render begins caching" | no deploy row, no restart (was a deploy row and a roll) | matched |
| Turning caching off | serves every response from the service | sidecar passes through until the next deploy, Service direct to the app, no restart | matched |
| Turning caching on for a service without the sidecar | no restart | adds the sidecar, one roll | divergence, owned by `w3/m489` and recorded in ADR018 |
| A switch empties the cache | not documented | empties it, as the roll it replaces did | unverified against Render |
| MCP | no cache tool | none | matched |
| `edge_cache_enabled` / `edge_cache_disabled` / `edge_cache_purged` | events | unchanged | matched (w9/m345) |

No new drift to file: the one divergence is `w3/m489`'s, and ADR018's Edge caching row records it with this date.

## Blocked (2026-11-18)

- **Gate (clears by itself):** a production deploy carrying `c6bab1b92` (the implementation). `deploy.yml` deploys at most every 6 hours; the last pin, `e5ace7826` (2026-11-18 21:22Z), makes the earliest one about 2026-11-19 03:30Z plus its tests and build. The check: the newest `chore(deploy): pin platform images to <sha>` commit on `origin/main` with `git merge-base --is-ancestor c6bab1b92 <sha>`. No user action is needed.
- **Then:** run t006 as written (a caching fixture in `bex-canary`; pod UIDs and `restartCount` across a REST purge and a dashboard profile switch; time from purge to `MISS` and back to `HIT`; the count of caching services and of those with a disk), write the record here, then t010.
- **Expect on that deploy:** `w6/m484` has not landed, so the operator image the deploy moves still rolls every caching service once, onto the controlled shape. Purges after that roll none; measure on the fixture after the deploy settles. The sidecar logs `edge-cache: purged (app.bex.co/cache-purged-at=…)` for each purge it applies.

## Unblocked (2026-11-19, `/loopx w3` routine `pin-gated-recheck`)

- **Affected:** t006, t010. **Source:** verified evidence, no user decision.
- **Finding:** the newest pin on origin/main, `826d32e42` (`chore(deploy): pin platform images to 7af50a28168d`, which moves the `bex-operator` image), contains `c6bab1b92`: `git merge-base --is-ancestor c6bab1b92 7af50a28168d` holds. It also contains the admission-policy fix `6d428736b`, without which production would refuse the pod annotation patches. The release condition in § Blocked is met.
- **Before measuring:** The pin was pushed by `deploy.yml` run 44451653443, whose `deploy` job was still running at 2026-11-19 03:50Z: confirm that run finished green and that `bex-system`'s images match the pin before measuring. The pin does not carry `a6b1658fb` (`w3/m489`'s per-service tier), so production still runs the per-pod edge-cache sidecar this milestone changed; the deploy that carries `a6b1658fb` moves caching services to the tier.
- **Status:** pending verification. A deployed fix is not proof that the definition of done holds on production.
- **Next:** run t006 as written in § Blocked (its "Then" and "Expect" lines still hold), write the record here, then t010.

## Production record (t006, 2026-11-19)

**Release.** `deploy.yml` run 44451653443 finished green (its `deploy` job took 7m7s). Every `bex-system` Deployment ran `bex-operator@sha256:7ce6ae1e…`, the digest pin `826d32e42` writes, and `bex-controller-manager` restarted on it at about 03:52Z. The pin contains `c6bab1b92` and `6d428736b`.

**Existing caching services.** Production had one before the fixture: `m484-t005-cache` in `bex-canary`, which is w6/m484's fixture and was left untouched. The release rolled it once (ReplicaSet `6c78c9d9c9` → `cf8d5d895`). That roll was the sidecar image move w6/m484 owns (`62e7ab56` → `7ce6ae1e`), and it carried the pod onto the controlled shape (`BEX_EDGE_CACHE_CONTROL` and the `edge-cache-control` volume), as § Blocked expected. m486's own projection keeps a stored template's shape: `TestAnUpgradeKeepsAnEdgeCacheTemplateFromBeforeItsControlFile` covers it at the pin.

**Fixture.** `m486-t006-cache` (`srv-dbv77hm85hks73fqim3g`) was a starter web service in `bex-canary` running `docker.io/traefik/whoami:v1.10.1`, created through REST at 03:57:58Z by the QA user (`scripts/qa-login.sh`). Caching was turned on with REST `PATCH` `serviceDetails.cache.profile=origin-controlled`. Adding the sidecar rolled the service once, the residual w3/m489 owns. Every step below ran on the same pod `…-74655f994d-692ct`, UID `3b6eaf95-e71e-4fb0-af06-6bdc7cc88924`.

| Step (UTC) | Cache | Pod UID, `restartCount` (app / edge-cache), ReplicaSet |
| --- | --- | --- |
| Before: `GET /m486.css` three times | `MISS`, then `HIT` (`Age: 0`), then `HIT` (`Age: 1`) | `3b6eaf95…`, 0 / 0, `74655f994d` |
| REST `POST …/cache/purge` answered `202` at 03:59:56.46. The App stamp was 03:59:56.32 and the sidecar logged `edge-cache: purged (app.bex.co/cache-purged-at=…)` at 03:59:57.29 | `MISS` at 03:59:59.55 (the first request after the purge), then `HIT` at 04:00:00.80 | unchanged |
| GraphQL `setCacheProfile` → `origin-controlled-all` at 04:01:25. The sidecar logged `profile "origin-controlled" -> "origin-controlled-all"` at 04:01:27.29 | `/` read `DYNAMIC` (04:01:26.94), `MISS` (04:01:28.10), then `HIT` (04:01:28.66); `/m486.css` read `MISS` once, because a switch empties the cache | unchanged |
| GraphQL `setCacheProfile` → `origin-controlled` at 04:01:45. The sidecar logged the switch at 04:01:47.29 | `/` read `DYNAMIC` again at 04:01:48.13 | unchanged |
| GraphQL `purgeServiceCache` (the dashboard's Purge cache verb). The stamp was 04:02:06.94 and the sidecar purged at 04:02:07.29 | `MISS` at 04:02:07.67, then `HIT` at 04:02:08.40 | unchanged |
| REST `PATCH` → `no-cache` at 04:02:48. The sidecar logged `profile "origin-controlled" -> "no-cache"` at 04:02:51 | No `CF-Cache-Status` header. The Service sends ports 80 and 10000 straight to the app's 80 | unchanged |

- **Deploys and releases.** The service kept one deploy row (its create), release generation 1 and active revision `rev-1` throughout, so no switch opened a deploy.
- **Timing.** From a purge or switch stamp, the sidecar applied it within 1 s, on its next 2 s tick. On this pod the kubelet refreshed the downward-API file on the pod update. The path then read `MISS` on the next request, 1–3 s after the API answered, and `HIT` on the request after that.
- **Fleet.** Production has 29 Apps. Two have caching on: w6's fixture and this one, both in `bex-canary`. Neither has a disk. Six Apps have disks, and none of them caches. The disk-bearing case is covered by `TestAPurgeOrAProfileSwitchKeepsTheEdgeCachedPodTemplate`'s disk cases (strategy `Recreate`, template unchanged).
- **Deviation.** Step 3 sent the dashboard's own `SetCacheProfile` GraphQL document from the QA session instead of clicking **Save edge caching**. The shared Playwright browser was held by another session, and the button sends the same request. The deployed toast reads "Cache purge started." (en) and "缓存清除已开始。" (zh).
- **Cleanup.** The fixture was deleted (`204`, then `GET` `404`), and both of this run's QA sessions were logged out.

**Definition of done.** Each item holds:

- Purges through REST and GraphQL kept the UID and restart counts and read `MISS`, then `HIT`.
- The disk case holds by its projection test.
- Both profile switches and turning caching off rolled nothing.
- m486's release kept each stored template's shape; the one roll was w6/m484's image move.
- The en and zh copy no longer mentions a restart.
- This record is in this README.
