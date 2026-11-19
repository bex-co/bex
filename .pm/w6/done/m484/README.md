# w6 · m484 — A platform release keeps every edge-cached service's instances and its cache

**Worker:** worker6 **Goal:** an operator deploy that changes only bex's own image leaves every edge-cached web service's pods and edge cache alone, as it already leaves every other service's; the cache empties only on a successful deploy or a manual purge, as on Render. **Status:** done 2026-11-19 (t001/t002 were met by w3/m489 while in flight; three production releases kept the fixture's pods and its cached entry)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [A test shows a new operator image rolling an edge-cached service](done/t001.md) — **DONE** | 30m | — |
| t002 | [The edge-cache sidecar keeps its release's image until the template changes for another reason](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [Fleet guard: a new operator image changes no long-lived tenant pod template](done/t003.md) — **DONE** | 1h | t002 |
| t004 | [Record the rule in ADR004 § Edge caching](done/t004.md) — **DONE** | 15m | t002 |
| t005 | [Ship and verify on production](done/t005.md) — **DONE** | 45m | t003, t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t006, t007 |

## Definition of done

- t001's test fails on the commit before t002 and passes after: re-projecting an edge-cached web service with a new operator image leaves its pod template unchanged, and a deploy, a purge or a settings change still adopts the new sidecar image.
- t003's guard fails when any long-lived tenant workload template (Deployment or StatefulSet) changes under a new operator image alone.
- On production, an edge-cached fixture in `bex-canary` keeps its pod UIDs across a deploy that moves the operator image, and a cached path still answers as a cache hit.
- ADR004 § Edge caching records the rule.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` 2026-11-17 (item 3 after verification), approved by the user's `/pmx them all to w6` 2026-11-18. Evidence:
  - `lego/operator/internal/controller/release_config_snapshot.go:732` sets `params.edgeCacheImage = r.BackupHelperImage`, the operator's own image, which moves on every deploy.
  - `applyDeploymentSpec` (`internal/controller/deployment_projection.go:693-735`) keeps a stored template's health-probe image, drain and period "so an operator upgrade never rolls the fleet" (w8/m133), but has no such rule for the edge-cache sidecar (w5/m151 landed 2026-10-21, the lazy rule 2026-10-23).
  - Reproduced 2026-11-18 with a scratch test on `origin/main` (not committed): an edge-cached web service re-projected with a new helper image changed its `edge-cache` container image, and because the template differed the stored health-probe installer image was replaced too; the same upgrade on a service without edge caching kept the template.
- **Goal linkage:** ADR008 § Render parity first. [render.com/docs/web-service-caching](https://render.com/docs/web-service-caching): "Each time you successfully deploy a new version of your web service, Render purges all of the service's edge cache entries"; "Failed deploys do not trigger a purge"; a manual purge does the same. No other event purges it, and no ADR018 row signs off a purge on bex's own releases.
- **Expected outcome:** edge-cached services stop restarting and losing their cache on each production deploy (about 4–6 a day).
- **Why now:** it happens on every deploy, the fix is small, and t003 guards the whole class (the next helper-image consumer) in the line of w6/m469 and w6/m470.
- **Production usage not measured:** how many services enable edge caching was not read (production access failed on a stale host key during the brainstorm); t005 records the count.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard field or message changes; the tenant-visible effect is checked on production in t005.

## Progress — 2026-11-19 (parked on its production deploys)

- **Superseded while in flight.** w3/m489 (`a6b1658fb`, 03:27Z) replaced the per-pod sidecar with a per-service cache tier and adopted this milestone's rule. A pod template stored with the sidecar keeps it exactly as stored, image included, while nothing else changes. The tier keeps its image until a successful deploy (`edgeCacheTierImage`). This milestone's sidecar-image change (written, `make test` green on `e9890d5ad`) was therefore not shipped, and t001/t002 closed as met (their records cite m489's tests). DoD bullet 1's "a deploy, a purge or a settings change still adopts the new sidecar image" now reads: the next deploy or restart drops the stored sidecar, and the tier adopts the operator's image on a successful deploy.
- **Shipped:** the fleet guard (t003), `TestANewOperatorImageChangesNoLongLivedTenantPodTemplate`, which drives web, private, worker and Key Value workloads through their reconcilers under two operator images and fails when any Deployment or StatefulSet in the namespace changes, the tier included. It is red with each image hold reverted and on `e9890d5ad` (t007). Also ADR004's guard sentence (t004) and `/simplify` (t006).
- **Production at 03:31Z:** 0 of 31 Apps cache. t005's fixture `m484-t005-cache` (`srv-dbv6rrioonls73bi9kog`, `bex-canary`) answered `/m484.css` with `HIT`. Details: [t005 § Progress](done/t005.md#progress--2026-11-19-0350z-steps-12-done-step-3-waits-for-two-deploys).
- **Gate (clears by itself, then worker):** the second production deploy whose pin's producer contains `a6b1658fb`, the first operator release the tier lives through. The first creates the tier, which starts empty once. Then t005's "Next" and t008.

## Unblock record — 2026-11-19 14:45Z (the gate cleared; the cache check moves to the next deploy)

- **Affected:** t005, then t008.
- **Source:** verified evidence (`/loopx w6` dispatch). `a6b1658fb` is in producers `4b2cebd0bd03` (pin `19faf35f3`, 09:13Z) and `dff52c596d13` (pin `a91e9aeb3`, 11:30Z), so the second production deploy carrying w3/m489 has happened.
- **What held across it (read from production at 14:31Z).** The manager moved from `4407341b…` to `9850d357…` at 11:35:30Z. The fixture's App pod and both cache-tier pods kept their UIDs, their ReplicaSets and 0 restarts. The tier stayed on `4407341b…`, and its logs show no purge after its own start at 09:18Z. DoD bullet 3's pod half holds for that deploy. Details: [t005 § Progress 14:45Z](done/t005.md#progress--2026-11-19-1445z-deploy-3-kept-every-pod-the-cache-check-moves-to-the-next-deploy).
- **What could not be checked.** The tier started empty at 09:17:51Z. Loki's request log shows no fetch of the fixture between 05:01Z and 14:37Z, so the cache held no entry at that release, and no `HIT` across it can be reconstructed. The plan had two flaws: whoami's 120-minute default TTL, and a pin-gated worker that always arrives after the rollout.
- **Changed:** the fixture now runs `caddy respond`, which answers `/m484.css` with `Cache-Control: public, max-age=604800`. Its entry was filled at 14:40:06Z and stays fresh for 7 days, so a worker that arrives after the next deploy can still read whether the release kept it. The DoD is unchanged.
- **Remaining gate (clears by itself):** the next production deploy, so pin must carry `a91e9aeb3`. Then t005 § Progress 14:45Z "Next": compare the pods with the state recorded there, fetch `/m484.css` once (`HIT` with an `age` dating it to 14:40:06Z), delete the fixture, and run t008.

## Closed — 2026-11-19 20:05Z (the gate cleared; DoD 3 holds on production)

- **Affected:** t005, t008.
- **Source:** verified evidence (`/loopx w6` dispatch). After pin `a91e9aeb3` production deployed `aefb144dc` (pin `c81d2d447bd8`), `99bbc4239` (`c624a4f217ff`) and `d38508f70` (`1c63b6fe0a34`). Each moved the manager's image: `697fe9c0…` at 18:08:52Z, `c588e4d2…` at 18:52:55Z, and `092e0a87…` at 19:50:48Z, running since 19:54:13Z.
- **What held across all three.** Read at 20:02Z before any fetch: the fixture's App pod `-gl7hv` and both tier pods kept their UIDs, ReplicaSets (`-6995d5c744` revision 4, `-54447644fd` revision 2) and 0 restarts. The tier stayed on `9850d357…`. Neither Deployment has a write after 14:39:02Z, and neither tier log has a `purged` line after 14:39:06Z.
- **The cache check.** The first fetch of `/m484.css` at 20:02:29Z answered `HIT` with `age: 19343`, which dates the entry to its 14:40:06Z fill. As on Render, nothing but a successful deploy or a manual purge emptied it.
- **Then:** the fixture was deleted (`DELETE` 204, `GET` 404 at 20:03Z), and t008 closed the milestone. Details: [t005 § Progress 20:05Z](done/t005.md#progress--2026-11-19-2005z-three-operator-releases-kept-every-pod-and-the-entry-fixture-deleted).
