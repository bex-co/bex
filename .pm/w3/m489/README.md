# w3 · m489 — A service's edge cache outlives its instances: restarts, scaling and crashes keep it, and every instance shares it, as Render's CDN does

**Worker:** worker3 **Goal:** each caching web service's edge cache lives in a per-service tier outside its instances, so it empties only on a successful deploy or a manual purge, as on Render; a Restart, a scale change, an instance crash or a node drain keeps it, all instances share one fill per key, and turning caching on rolls no instance. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | ADR004 amendment: a per-service cache tier outside the instances | 1.5h | — |
| [t002](t002.md) | Operator: a cache Deployment and Service per caching App; the Ingress moves to it only once it is Ready | 2h | t001 |
| [t003](t003.md) | The edge cache stores on a bounded disk with a larger object cap | 1.5h | t001 |
| [t004](t004.md) | A successful deploy or a manual purge advances the tier's purge generation; a failed deploy does not | 1h | t002, w3/m486/t003 |
| [t005](t005.md) | Existing sidecar services move to the tier without dropping a request; the sidecar leaves at their next deploy | 1h | t002, t004 |
| [t006](t006.md) | Fail-open drill: a killed cache pod costs no 5xx | 45m | t005 |
| [t007](t007.md) | Ship and verify on production | 45m | t006 |
| [t008](t008.md) | Render parity | 20m | t007 |
| [t009](t009.md) | Simplify | 15m | t008 |
| [t010](t010.md) | Test coverage | 30m | t008 |
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
