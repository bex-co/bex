# Production build-cache enablement — 2026-09-09

Decision: **HOLD the global trial until the corrected operator is deployed and the production cache footprint fits the budget below.** The user approved a conditional 48–72 hour trial on September 9. No trial has started and `BEX_BUILD_CACHE` remains unset. This records the live preflight and the remaining start conditions, rather than treating the earlier local-only correctness checks as production acceptance.

## Production preflight

Observed September 9, approximately 09:03–09:11 UTC, through the existing pinned-SSH bootstrap and an authenticated Kubernetes API connection. Prometheus and Zot were accessed through loopback-only port forwards. Credentials were used in memory; none belong in this report.

| Item | Observed state |
| --- | --- |
| Operator | `bex-system/bex-controller-manager`, 1/1 ready; `BEX_BUILD_CACHE` absent |
| Operator and API image | `ghcr.io/bex-co/bex-operator@sha256:7868ad4e222bce2f9947301e36aeee235c753fed3d8f182d3c235aaa4d1eb069` |
| Image provenance | `ce418be56381` pinned images built from `05d53ec197d1` on September 8 at 01:36 UTC |
| Required fixes | Native env invalidation: `1343b7f17070`, September 9 at 03:15 UTC; clear-cache: `3aea3310212f`, September 9 at 03:37 UTC. Both are later than the deployed image. |
| Concurrency | 2 builds per workspace, 4 cluster-wide; 2 App reconcile workers |
| GitOps | `argocd/bex-operator` follows `main`, path `lego/operator/config/prod`, automatic self-heal and server-side apply |
| Zot claim | `bex-registry/zot-pvc-zot-0`, requested/bound 100 GiB |
| Measured filesystem capacity | 98.31 GiB; use this denominator for the thresholds, not the nominal claim size |
| Used / available | 36.76 / 61.53 GiB; **37.39% used** |
| Retained-window maximum | 46.20 GiB, **47.00% used** |
| GC / retention | Dedupe enabled, GC enabled, `gcDelay=1h`, `gcInterval=1h`, delete untagged manifests, keep 5 recently pushed tags per repository |
| Current workload | 8 Apps, 7 with repositories; none of those 7 has a populated cache repository |
| Active / queued builds | 0 / 0 at the preflight snapshot |

The most recent deployment inspected, [run 34329225161](https://github.com/bex-co/bex/actions/runs/34329225161), did not build or deploy an image. The operator suite passed. Backend failed at `start OpenFGA (in-memory)` with exit 125; Dashboard failed typechecking fixtures missing current `ServiceView` / `EnvGroupView` fields, including `outboundIps`. Retrying the same source does not repair those type errors. These are deployment blockers, not evidence that the cache fixes failed their tests. Production must run an image containing **both** fixes before the global gate changes.

## Baseline and capacity budget

The queries requested seven days, but the retained PVC and build-counter series start at **September 6, 06:00 UTC**: only about **75 hours** are present. Do not label these results a full-week baseline. The 24-hour queries have a complete retained window. Counts below are rounded Prometheus counter increases; quantiles are histogram estimates over mixed workloads, not paired cache benchmarks.

| Build signal         |   Last 24 hours | Available part of the 7-day query |
| -------------------- | --------------: | --------------------------------: |
| Run samples          |              54 |                               136 |
| Successful outcomes  |              51 |                               130 |
| User-failed outcomes |               2 |                                 2 |
| Run p50 / p95        |    800 / 1707 s |                      734 / 1698 s |
| Queue p50 / p95      |    2.5 / 4.75 s |                      2.5 / 4.75 s |
| Push p50 / p95       | 11.38 / 34.50 s |                   12.72 / 53.17 s |
| Push errors          |               — |                                 0 |

The live oldest-queue gauge never exceeded zero during the last 24 hours. There were no firing `Zot*` or `Build*` alerts at the snapshot. An absent `infra_failed` series is not an independent proof that no infrastructure failure ever occurred.

PVC usage varied from 28.91 to 46.20 GiB in the retained window; its downward regression slope includes reclamation and is not a reliable forecast of new cache growth. The live image repositories retain approximately **22.35 GiB of unique reachable compressed content across all seven Apps**. That is a manifest-content count, not allocated disk usage or bytes transferred. The largest single repository retains about 10.38 GiB across its image tags. Two repositories still have six tags between GC passes, confirming that the keep-5 policy is not an instantaneous disk bound.

Use **60% as the normal target and 65% as the trial admission ceiling**. At this snapshot those are 58.99 and 63.90 GiB. Headroom to 65% is 27.14 GiB from current usage, but only **17.70 GiB above the recent peak**. Admission must fit ordinary image growth, new unique cache content, four concurrent uploads, and old layers awaiting the delayed GC within that 17.70 GiB reserve. Do not count a cache tag deletion as immediate disk reclamation.

The [September 5 measurements](2026-09-05-build-cache.md) remain the benefit evidence: Dockerfile 179 → 38 s and native Node 75 → 64 s, one cold/warm pair each. Their compressed-content accounting found approximately 173 MB extra for the dashboard cache, and a native cache containing approximately 517 MB while adding little unique content beside the image. At 54 builds/day, a 517 MB full restore on every build would process about **28 GB/day**; equal-sized full uploads would double that scenario. This is a sizing example, not measured network traffic, and does not describe the much larger production repositories.

**Remaining capacity measurement:** capture the cache manifest sizes and restore/save timings for the largest actively rebuilt production workload using an isolated fixture with the same build shape. Check resulting allocated PVC growth across at least two GC intervals. The tiny correctness fixtures below cannot establish the production fleet's `mode=max` cache size. Current disk headroom passes the snapshot check; the fleet-size forecast is still conditional.

## Real build-plane correctness acceptance

The opt-in `TestRegistryBuildCacheDrill` creates real Jobs in `bex-build`, using production BuildKit, Zot, networking, worker disks, and the existing build-plane credential. App bookkeeping and reconciliation run in the local test process against the corrected source. It creates no serving App and does **not** prove that the old deployed manager contains the fixes.

Both fixtures use the public Node repository at fixed commit `179ae84efec61b14206d0305d941daed6c6d07f9`, workspace `tea-d98210cbbpdc73dcrkvg`, and fresh service IDs minted through `backend/internal/id.New(id.Service)`. Registry repositories had to return 404 before the first build.

| Fixture | Generation | Expected artifact | Observed artifact | Run / queue |
| --- | --: | --- | --- | --- |
| `srv-dagi3ba9086kn9f3pg3g`, native env | 1 | A | A | 36 / 0 s |
| Same fixture, only MESSAGE changes | 2 | B | B | 43 / 0 s |
| Same fixture, unchanged env | 3 | B | B, 3 BuildKit cache hits | 44 / 0 s |
| `srv-dagi3ba9086kn9f3pg40`, clear-cache cold | 1 | Fresh UUID X | `b394c806-c7e8-4bae-9366-deac68354a8f` | 36 / 0 s |
| Same fixture, warm | 2 | X again | X, 3 cache hits | 43 / 0 s |
| Same fixture, clear requested | 3 | Fresh UUID Y, different from X | `a210c5f4-cdc1-4940-b0c2-3d6e280d0798`, 0 cache hits | 37 / 0 s |
| Same fixture, warm after clear | 4 | Y again | Y, 3 cache hits | 41 / 0 s |

**Both drills passed, seven builds total.** The native-env revision in the actual BuildKit logs was 1 → 2 → 2; the env-dependent RUN missed on generation 2 and hit on generation 3. The new `native-clear` workload writes a random UUID during the native build and checks cold → warm → clear → warm: the second artifact equals the first, the clear artifact differs, and the final artifact equals the clear artifact. This proves both clearing and subsequent repopulation without assuming deterministic image digests.

The clear build ran `cache-purge` and no `cache-restore`; the following warm build restored in 4 s. Normal restores took 4–5 s, cache saves 0–2 s, and image pushes 1–2 s. All cache and push phases exited zero. The tiny command itself is much cheaper than restoring a Node base image, so these timings are correctness evidence, not a production performance improvement claim.

Replay each workload with a **different fresh ID**, an explicit authenticated production `KUBECONFIG`, the workspace and commit above, an absolute private `BEX_CACHE_DRILL_LOG_DIR`, and `BEX_CACHE_DRILL_REGISTRY_FORWARD` pointing at the local Zot port forward:

```sh
cd lego/operator
BEX_CACHE_DRILL_WORKLOAD=native-env GOWORK=off go test -tags=e2e \
  ./internal/controller -run '^TestRegistryBuildCacheDrill$' -count=1 -v -timeout=70m
# Use another freshly minted BEX_CACHE_DRILL_NAME for the clear-cache run.
BEX_CACHE_DRILL_WORKLOAD=native-clear GOWORK=off go test -tags=e2e \
  ./internal/controller -run '^TestRegistryBuildCacheDrill$' -count=1 -v -timeout=70m
```

The harness deletes its Jobs, reader Pods, and App-owned native env projection using UID preconditions. Registry retirement is a separate step after collecting manifest-size evidence, restricted to the freshly minted fixture repositories; GC reclaims unreachable content later.

Retirement was verified for this run: both fixtures have **zero remaining Jobs, Pods or Secrets**, and all four fixture image/cache repositories have **empty tag lists** after deleting only their manifests. Before retirement, their cache manifests each referenced about 409 MB of compressed content; extra unique cache content beside their retained images was only 1,954 and 1,944 bytes respectively. This small overlap cost is specific to these tiny native fixtures and cannot bound production multi-stage caches. No tenant App was created, changed or deleted.

## Conditional trial procedure

1. Verify the deployed immutable image contains both fixes, all deployment suites passed, both acceptance drills passed, and the production-size capacity projection stays at or below 65%. Record the exact operator image and fresh PVC measurements.
2. Record a start time, an initial end time at **48 hours**, the observer, and an independently verified scheduled rollback at **72 hours maximum**. No start/end time or scheduled rollback exists for this held trial. Reaching 48 hours without 20 comparable repeat builds may justify observing until the already bounded 72-hour stop; it does not justify leaving the switch on indefinitely.
3. Add `BEX_BUILD_CACHE=registry` to the manager env in the **production overlay** `lego/operator/config/prod/kustomization.yaml`, using its existing Deployment JSON-patch list. Roll out through the normal GitOps workflow, wait for `bex-controller-manager` readiness, and verify a newly created build Job has the cache phases. Because Argo self-heals this Deployment, a standalone `kubectl set env` is not a durable trial configuration.
4. Sample PVC use/available space and `bex_build_*` throughout the window. Save per-build App identity, source commit, run/queue/push durations, and cache restore/save durations before completed Jobs expire. Compare at least **20 comparable repeat builds** by workload; mixed-workload aggregate histograms alone cannot demonstrate a cache speedup. Do not record env values or registry credentials.
5. At 48–72 hours, either record an explicit retain decision with the measured benefit, storage growth and transfer cost, or roll back. Keep native and Dockerfile results separate: their earlier gains and transfer overhead differ materially.

Rollback immediately for a stale artifact or incorrect clear-cache behavior. Also stop if PVC use is **over 70% for 10 minutes** (the existing `ZotRegistryFillingUp` warning), a cache-related push failure occurs, or push p95 is persistently above **69 seconds** (twice the complete 24-hour baseline, and above a 60-second floor) for 15 minutes with at least five push samples in the observation window. The existing critical alert at 85% or below 10 GiB free remains an emergency threshold, not the trial's operating target.

Durable rollback removes the production overlay entry and reconciles GitOps. For an immediate stop, remove it from the running manager too:

```sh
kubectl --kubeconfig "$KUBECONFIG" -n bex-system set env \
  deployment/bex-controller-manager --containers=manager BEX_BUILD_CACHE-
kubectl --kubeconfig "$KUBECONFIG" -n bex-system rollout status \
  deployment/bex-controller-manager --timeout=180s
```

Coordinate the desired-state removal with the emergency command: while Git still enables the cache, Argo can restore it. Verify newly created Jobs no longer have cache phases. Jobs already created retain their specs; do not cancel tenant builds solely to remove the cache flag. Disabling the switch does not delete cache tags or immediately release disk space.

## 2026-09-15 follow-up: corrected image is live; capacity re-check still owed

`deploy.yml` recovered on 2026-09-10 ([run 34444812690](https://github.com/bex-co/bex/actions/runs/34444812690), from `46e16ec38835`, which contains both `1343b7f17070` and `3aea3310212f`). Every green run since has built and deployed; the latest at the time of writing, [run 34936503315](https://github.com/bex-co/bex/actions/runs/34936503315) from `c4212ec71909`, rolled `bex-controller-manager` onto `ghcr.io/bex-co/bex-operator@sha256:1f3648adb9598bd2d73304b2ebc346bfffe04542d3db13a55e4d198168711a46`, the digest pinned in `deploy/gitops/base/bex.yaml`. The first start condition (corrected image live) is therefore met.

The second condition was re-checked later the same day (next section): the Zot PVC and peak are within budget and the size projection fits. `BEX_BUILD_CACHE` remains unset pending the trial start (`w7/m89` t004).

## 2026-09-15 23:30 UTC re-check (w7/m89 t002 + t003)

Read through the production kubeconfig (`scripts/fetch-app-kubeconfig.sh`, trust-on-first-use) with loopback port-forwards to Prometheus and Zot; the build-plane push credential was used in memory for the catalog read only. Nothing was written to the cluster.

| Item | Observed 2026-09-15 |
| --- | --- |
| Operator | `bex-system/bex-controller-manager` 1/1 ready; `BEX_BUILD_CACHE` absent (env: `BEX_CNB_BUILDER`, `BEX_BUILD_NAMESPACE`, `BEX_MAX_CONCURRENT_BUILDS`, `BEX_MAX_ACTIVE_BUILDS`) |
| Live image | `ghcr.io/bex-co/bex-operator@sha256:b00306f7514a89a46d55aecd556a291c4b1b30f97eaf6a2ac02252dc021d5c09`, pinned by `1df779f8a` ("pin platform images to 1cb1f2d27dda"); Argo `bex-operator` Synced/Healthy. `1cb1f2d27dda` contains `1343b7f17070` (m87) and `3aea3310212f` (m88) — **start condition 1 met** |
| Zot filesystem | capacity 98.31 GiB (105,559,699,456 B); used **40.79 GiB, 41.49%**; available 57.50 GiB |
| Retained-window peak | **43.87 GiB, 44.62%** (7-day query; retention starts 2026-09-12 18:59 UTC, so ~3.2 days present); minimum 40.11 GiB |
| Thresholds | 60% = 58.99 GiB; 65% admission ceiling = 63.90 GiB; 70% rollback = 68.82 GiB |
| Headroom | 23.11 GiB from current use to 65%; **20.03 GiB above the peak to 65%**; 24.95 GiB above the peak to the 70% rollback line |
| GC / retention | unchanged: dedupe, GC on, `gcDelay=1h`, `gcInterval=1h`, delete untagged, keep 5 most recently pushed tags per repository |
| Alerts firing | only `TenantCustomDomainCertNotReady`; no `Zot*` / `Build*` |
| Catalog | 49 repositories, **38.20 GiB dedupe-unique reachable content** (sum over repositories 43.66 GiB; the workspace-scoped and legacy copies of the same App share blobs); **no `*-cache` repository exists** |

Build signals (Prometheus, counters rounded; the 7-day query again covers only the retained ~3.2 days):

| Signal | Last 24 h | Retained window |
| --- | --: | --: |
| Run samples | 102 | 272 |
| Outcomes succeeded / user-failed / canceled / superseded | 91 / 8 / 5 / 24 | 260 / 8 / 21 / 65 |
| Run p50 / p95 | 713 / 1652 s | 688 / 1679 s |
| Queue p50 / p95 | 2.5 / 4.75 s | 2.5 / 4.75 s |
| Push p50 / p95 | 9.1 / 27.4 s | 7.8 / 24.8 s |
| Push errors | 0 | 0 |
| Oldest-queued maximum | 620 s | 1104 s |

The push p95 baseline for the trial's stop rule is therefore ~27 s; the rule's floor (60 s) still governs, i.e. stop at a persistent push p95 above 60 s.

**Representative cache-size projection (t003).** There is no per-App cache switch (`BEX_BUILD_CACHE` is a manager-wide flag, `lego/operator/cmd/manager/main.go`), and the largest actively rebuilt Apps build private sources, so an isolated same-shape fixture could not be built; this is an analytic bound from the live catalog, not a measurement, and is labelled as such. The actively rebuilt production shapes (5 retained tags each) and their per-image compressed size: `block-eden-mono` (native) 2.32 GiB, `tianpan-v4-web` (native) 1.41 GiB, `beancount-forum` (Dockerfile) 1.07 GiB, `beancount-cms-v2` (auto) 0.61 GiB, `eden-cms-v2` (auto) 0.36 GiB, `agentmarketcap-1` (auto) 0.18 GiB — 5.95 GiB of images in total. The cache is one rolling `cache` tag per App in `<repo>-cache` (`lego/operator/internal/build/build.go`), so steady state retains one cache set per App; final-image layers dedupe against the image repository, and the unique cache content is the builder-stage layers. The September 5 native measurement (517 MB restored, ~2 KB unique beside the image) puts native shapes near 1×; a multi-stage Dockerfile is the 2–3× case. Projection: **expected steady-state growth ≈ 6–9 GiB** (1× for the two native Apps, up to 3× for the Dockerfile and auto Apps); worst case at 3× for every App ≈ 17.9 GiB; during a rebuild burst the previous cache set lingers for up to ~2 h (`gcDelay` + `gcInterval`), so a transient of one extra set per rebuilt App (≈ 6–18 GiB) can stack on top. **Fit:** expected growth fits the 20.03 GiB reserve above the retained peak with room to spare; the all-3× worst case fits only without a concurrent transient, and a burst on top of it would cross the 65% line but is caught by the trial's own 70%-for-10-minutes stop rule (24.95 GiB above the peak). Transfer: at ~100 builds/day a full restore of the largest cache set (≈2.3–7 GiB) on every build of that App is the dominant cost; restores are per-App, so the fleet figure is far below the 28 GB/day sizing example above.

**Go/no-go:** admission passes on the expected footprint, so t004 may start — with the observer and the independently verified 72-hour rollback the procedure requires, and with the understanding that the 3× worst case relies on the 70% stop rule rather than on the 65% reserve. The switch remains **off** at the time of writing.

## Closeout state

Global enablement remains **off**. Both live correctness drills and the full operator `make test` passed, and fixture retirement is verified. Pending: deployment of the already merged correctness fixes through passing CI and the representative cache-size measurement. The approved 48–72 hour observation clock has not started.

The repository-wide `make lint` gate is **not green**: its local Go 1.26-built tools cannot analyze the Go 1.27 CLI. Rebuilding the pinned tools with Go 1.27 still hits the pinned linter's unsupported export-data format, and whole-program deadcode reports the existing backend `Service.collectDatabasePodLogs` and `Service.collectKeyValuePodLogs` functions. Those failures are outside this test-harness change; no linter was disabled or unrelated backend code removed to mask them.
