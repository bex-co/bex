# w3 · m486 — Purging a service's edge cache, or switching between its two caching profiles, restarts no instance, as on Render

**Worker:** worker3 **Goal:** a manual purge and a switch between `origin-controlled` and `origin-controlled-all` empty or re-scope the edge cache while every instance keeps running, as on Render, where the cache lives on its CDN; a disk-bearing caching service no longer goes down on a purge. **Status:** todo (t001–t005 and t007–t009 done; `c6bab1b92` is in production pin `7af50a28168d`; t006's production check, then t010)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Failing tests: a purge and a caching-profile switch change the pod template today — **DONE** | 30m | — |
| [t002](done/t002.md) | The edge-cache sidecar takes its purge generation and profile from a file it re-reads while running — **DONE** | 1h | t001 |
| [t003](done/t003.md) | The operator patches live pods on a purge or profile switch, and rolls no existing template — **DONE** | 1h | t002 |
| [t004](done/t004.md) | The dashboard's purge copy stops promising a restart (en/zh) — **DONE** | 30m | t003 |
| [t005](done/t005.md) | Record the rule in ADR004 § Edge caching and ADR018 — **DONE** | 20m | t003 |
| [t006](t006.md) | Ship and verify on production | 45m | t004, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 30m | t007 |
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
