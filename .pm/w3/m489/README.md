# w3 · m489 — A service's edge cache outlives its instances: restarts, scaling and crashes keep it, and every instance shares it, as Render's CDN does

**Worker:** worker3 **Goal:** each caching web service's edge cache lives in a per-service tier outside its instances, so it empties only on a successful deploy or a manual purge, as on Render; a Restart, a scale change, an instance crash or a node drain keeps it, all instances share one fill per key, and turning caching on rolls no instance. **Status:** in progress (t001–t006 and t008–t010 done; t007 verifies on production once a deploy carries this, then t011 closes out)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | ADR004 amendment: a per-service cache tier outside the instances — **DONE** | 1.5h | — |
| [t002](done/t002.md) | Operator: a cache Deployment and Service per caching App; the Ingress moves to it only once it is Ready — **DONE** | 2h | t001 |
| [t003](done/t003.md) | The edge cache stores on a bounded disk with a larger object cap — **DONE** | 1.5h | t001 |
| [t004](done/t004.md) | A successful deploy or a manual purge advances the tier's purge generation; a failed deploy does not — **DONE** | 1h | t002, w3/m486/t003 |
| [t005](done/t005.md) | Existing sidecar services move to the tier without dropping a request; the sidecar leaves at their next deploy — **DONE** | 1h | t002, t004 |
| [t006](done/t006.md) | Fail-open drill: a killed cache pod costs no 5xx — **DONE** | 45m | t005 |
| [t007](t007.md) | Ship and verify on production | 45m | t006 |
| [t008](done/t008.md) | Render parity — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Simplify — **DONE** | 15m | t008 |
| [t010](done/t010.md) | Test coverage — **DONE** | 30m | t008 |
| [t011](t011.md) | Closeout | 10m | t009, t010 |

## Definition of done

- ADR004 § Edge caching records the tier (t001), and ADR018's Edge caching row no longer lists a per-instance cache.
- On production (t007): after a Restart and after a scale-out, a previously cached path still reads `CF-Cache-Status: HIT`; with two instances a new key reaches the app once; a successful deploy and a manual purge each turn the path to MISS once, and a failed deploy does not.
- Turning caching on rolls no app instance (envtest + production).
- The local fail-open drill (t006) shows zero 5xx while the tier pod is deleted.
- The operator upgrade rolls no existing tenant template (envtest, `w6/m484`'s guard).

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w3 2026-11-18 (topic: one aspect of Render parity, web-service edge caching), item 5; filed by `/pmx them to w3` 2026-11-18. Evidence on `origin/main` `4e3c87031`: `docs/ADR004-app-deployment.md` § Edge caching ("The cache is per instance and in memory, bounded at 48 MiB"; 6 MiB per object in `lego/operator/cmd/edge-cache/main.go`). A Restart (`spec.restartedAt`), a scale-out, a scale-in, an OOM or eviction, a node drain and turning caching on (sidecar injection) each empty or cold-start the cache, and N instances fill each key N times. ADR018's Edge caching row calls this a "divergence in mechanism, not behavior", which these effects contradict.
- **Goal linkage:** ADR008 § Render parity first. [render.com/docs/web-service-caching](https://render.com/docs/web-service-caching): "Edge caching is powered by the same global CDN as Render static sites"; Render purges "each time you successfully deploy", and "failed deploys do not trigger a purge". `w6/m484` adopted "the cache empties only on a successful deploy or a manual purge, as on Render"; that rule can only fully hold once the cache is outside the instances.
- **Recommendation, not a user decision:** selecting this item schedules it. The per-service tier (option A) is the brainstorm's recommendation under ADR008 Render parity first, which does not park parity work on an optional decision, so the tasks implement it. The alternative, keeping the per-pod sidecar and recording its effects in ADR018 as a divergence (option B), would need the user's sign-off and remains the user's call; t001 records the per-service cost so that choice can be revisited with numbers.
- **Expected outcome:** edge-cached services keep their cache through restarts, scaling and instance loss, fill each key once across instances, cache larger objects, and no longer roll when caching is turned on.
- **Why now:** `w6/m484` just made "only a successful deploy or a purge empties it" the rule; this is what makes it true beyond operator releases. Sequenced after `w3/m486` (its runtime purge generation, t004 depends on `w3/m486/t003`) and `w6/m484` (its template guard).
- **Render parity included:** the dashboard's edge-caching copy and the ADR018 Edge caching row change with the mechanism.

## Triage (2026-11-19)

**Work.** The premise held on `origin/main` `2a8e1f119`. ADR004 § Edge caching still read "The cache is per instance and in memory, bounded at 48 MiB". `edge_cache.go` projected a per-pod sidecar that the Service targeted, and `cmd/edge-cache` capped objects at `maxBytes / 8` (6 MiB). Nothing gates the per-service tier under ADR008 § Render parity first, so the tasks implement option A, and option B stays the user's call (ADR004 records its cost). `w6/m484` has not landed, so its fleet guard is not on main. This milestone keeps its rule itself: an operator release changes neither the tier's template nor a stored sidecar template (`TestTheTierKeepsItsImageUntilASuccessfulDeploy`, `TestAnUpgradeKeepsAStoredEdgeCacheSidecarUntilTheTemplateChanges`). That also resolves m484 t001/t002's premise for caching services; its fleet-wide guard (t003) is still its own.

**Found on the way, shipped first and alone:** the `bex-operator-pod-labels` admission policy let the operator change no pod annotation. So w3/m486's purge and profile patches (`c6bab1b92`, not yet deployed) would have been refused on production, ending each such service's pass before any deploy step. The fix, `6d428736b`, admits exactly the edge-cache control annotations on a pod running the proxy and has an envtest against the real policy.

## Fail-open drill (t006, 2026-11-19)

The drill ran on the local CAPD cluster with the operator built from this tree. A paid `traefik/whoami` web service ran with two instances and `origin-controlled-all`. Requests went in-cluster through Traefik over HTTPS, a fresh connection each, about 7.5 a second, across 20 keys, for 60 s. One tier pod was deleted a third of the way in.

| deletion | answers | non-2xx | by `CF-Cache-Status` |
| --- | --- | --- | --- |
| `kubectl delete pod` (graceful, as an eviction or node drain does) | 450 | **0** | 406 HIT, 44 MISS |
| `--grace-period=0 --force` | 451 | 2 with no answer at all (curl timeout or reset), no 5xx | 442 HIT, 7 MISS |

The forced kill is the hard-crash case: Traefik still had the dead pod's address for the requests in flight to it, as it does for an app pod killed the same way. On the same cluster, a scale-out from 2 to 3 instances and deleting an app instance both kept a cached path at `HIT`. A new key read `MISS` once, then `HIT` on five fresh connections spread across both tier replicas. A manual purge turned the path to `MISS` once within about 2 s. A successful deploy (an environment change) turned it to `MISS` once, right after the rollout completed, and the tier pods were not restarted. A deploy stuck on a missing image kept it at `HIT`. A Restart through bex-api runs as a new release of the serving one, which a direct CR edit cannot reproduce, so that case is covered by `TestTheCacheEmptiesOnlyOnASuccessfulDeploy`, and t007 checks it on production.

## Render parity (t008, 2026-11-19)

Render's reference is [render.com/docs/web-service-caching](https://render.com/docs/web-service-caching), fetched 2026-11-19.

| surface or behavior | bex (this milestone) | Render | result |
| --- | --- | --- | --- |
| REST `serviceDetails.cache.profile`, GraphQL `cacheProfile`/`setCacheProfile` | unchanged: three profiles, refused on free plans and non-web types, a save opens no deploy | same | matches |
| REST `POST /v1/services/{id}/cache/purge` (202), GraphQL `purgeServiceCache`, dashboard **Purge cache** | unchanged contract; empties every tier replica at run time, restarts nothing | "purges all of your web service's associated edge cache entries" | matches |
| MCP | no cache tool | Render's MCP has none | matches |
| Dashboard note | "The cache is purged on every successful deploy." (all eight locales) | "Each time you successfully deploy a new version … Render purges" | matches (was "every deploy") |
| Successful deploy | empties the cache once the rollout has no old instance left | purges | matches |
| Failed deploy | keeps it | "Failed deploys do not trigger a purge" | matches |
| Restart, scaling, instance loss, operator release | keep it | not documented; the CDN lives outside the instances | unverified, consistent with the CDN |
| Instances behind the cache | one fill per key for the whole service | one CDN | matches |
| `CF-Cache-Status` | HIT, MISS, EXPIRED, BYPASS, DYNAMIC | same five | matches |
| Capacity | 2 GiB per service, 128 MiB per object | not documented (Cloudflare's default object cap is 512 MB) | recorded in ADR018 |
| Geography | in-cluster tier per service | global CDN | recorded in ADR018 |
| Private network | not cached | the CDN fronts only the public URL | matches by construction |
