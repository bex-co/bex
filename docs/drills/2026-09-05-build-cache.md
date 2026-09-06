# Registry build-cache drill — 2026-09-05

Two same-commit cold/warm pairs completed on the production cluster, using real build Jobs, networking, worker storage, BuildKit and Zot. The local observer called `AppReconciler.buildFromSource` and read deltas from its actual `bex_build_run_seconds` and `bex_build_queue_seconds` histograms. App bookkeeping alone used an in-memory client: no live App or serving rollout was created. The production manager's `BEX_BUILD_CACHE` remained unset; only the drill reconciler enabled the cache. Concurrency limits matched production (2 per workspace, 4 total).

## Results

| Workload | Cold run | Warm run | Improvement | Cold/warm queue | Warm cached steps |
| --- | --: | --: | --: | --- | --: |
| bex dashboard, Dockerfile | 179s | 38s | 78.8% | 0s / 0s | 8 |
| Node API, native runtime | 75s | 64s | 14.7% | 0s / 0s | 3 |

Each completed build produced exactly one D5 run histogram sample. These are Job run durations, not test wall times. Container timestamps provide the phase breakdown below; overlapping phases and scheduling overhead mean their durations are not additive.

| Workload/run   | Clone | Cache restore | BuildKit | Cache save | Image push |
| -------------- | ----: | ------------: | -------: | ---------: | ---------: |
| Dashboard cold |    8s |            0s |     159s |         4s |         2s |
| Dashboard warm |    9s |            2s |      21s |         1s |         0s |
| Node API cold  |    1s |            0s |      63s |         3s |         4s |
| Node API warm  |    1s |            5s |      47s |         1s |         1s |

The dashboard's dependency installation took 32.8s and compilation 92.2s on the cold build. Because compilation dominated, a second fixture isolated dependency work: the native Node API's only application build command was dependency installation (22.3s cold, cached warm). Its remaining cost included base-image materialization (14.6s cold) and image export (19.4s cold). A successful dependency cache hit therefore produced only a modest total improvement. This is one pair per workload, not a statistical latency estimate or an estate-wide hit-rate measurement.

## Storage

Inventory traversed image and cache manifests, configs and compressed layers, deduplicating content digests within each workload. Values are reachable content bytes, not filesystem allocation or transferred network bytes.

| Workload | Image bytes | Cache bytes | Combined unique bytes | Incremental cache bytes |
| --- | --: | --: | --: | --: |
| Dashboard | 61,633,945 | 234,702,104 | 234,711,971 | 173,078,026 |
| Node API | 517,127,585 | 517,123,105 | 517,133,410 | 5,825 |

The multistage dashboard's combined footprint is 3.81 times its final image; `mode=max` retains intermediate build layers. The single-stage native image shares almost all cache content, so incremental storage is tiny, although restoring that cache still processes about 517 MB. Cache-save container durations quantify the additional publishing phase; they do not isolate a counterfactual increase in the critical path.

**Verdict:** do not trigger ADR060's persistent-volume escalation on these results: dependency cache hits worked, and the modest native gain comes from transfer/materialization/export overhead rather than missed dependency layers. Keep the feature opt-in. Estate-wide enablement still requires a registry capacity decision using the actual workload mix; these two fixtures cannot establish that capacity.

## Fixtures and reproducibility

The opt-in verifier is `lego/operator/internal/controller/build_cache_drill_test.go`. Run from `lego/operator` with explicit `KUBECONFIG`, `BEX_CACHE_DRILL_WORKSPACE`, a fresh `BEX_CACHE_DRILL_NAME` minted through backend `internal/id.New(id.Service)`, `BEX_CACHE_DRILL_COMMIT`, an absolute `BEX_CACHE_DRILL_LOG_DIR`, and `BEX_CACHE_DRILL_REGISTRY_FORWARD` pointing to an authenticated local port-forward to Zot:

```sh
go test -tags=e2e ./internal/controller -run '^TestRegistryBuildCacheDrill$' -count=1 -v -timeout=70m
```

The default workload is the bex dashboard. Set `BEX_CACHE_DRILL_WORKLOAD=node-api` for the native fixture. Credentials are read from the existing build-plane Secret in memory. The observer authenticates digest reads; BuildKit receives no publishing credentials. This drill uses shared build-plane publishing; the separate per-App ACL tests and m86/t003 evidence cover tenant isolation. Fresh image/cache repositories must return HTTP 404 before the drill creates Jobs. Job cleanup uses UID preconditions. Inspect the output repositories before retiring their exact tags; registry artifacts deliberately survive the test so their footprint can be measured.

| Fixture | Source/commit | Synthetic service |
| --- | --- | --- |
| Dashboard | `https://github.com/bex-co/bex`, root `dashboard`, commit `88e0520539f521796e0388eb4f0446d4ec2b2fae` | `srv-daea4229086i2jh0vbt0` |
| Node API | `https://github.com/hagopj13/node-express-boilerplate`, commit `179ae84efec61b14206d0305d941daed6c6d07f9` | `srv-daeabbq9086it6s8m73g` |

Both ran in workspace `tea-d98210cbbpdc73dcrkvg`, build namespace `bex-build`, on worker `bex-tenant-lg-4wvlk-s5z8z`. The native fixture used the platform's pinned `node:24-bookworm@sha256:934240a162082fd8b8a2f90cd5114446443f1eba1c5378f6687167ca405e6584`, build command `corepack yarn@1.22.22 install --frozen-lockfile`, and start command `node src/index.js`.

Cold and warm image digests matched within each pair:

- Dashboard: `sha256:86eff7a8e053e293950f6b3471ea794aba39be69e5029d3d38136c13ab6616a3`.
- Node API: `sha256:301c472436492c5787ebe1dd0abcde706cb8e689593ea6e3525e6c6c0ca2196e`.

The original upstream Node Dockerfile attempt failed because its floating `node:alpine` base had no `yarn`. It produced no published artifacts and is excluded from the results. The successful run used the platform's normal native runtime against the same unmodified source. Earlier observer/preflight corrections addressed the push Secret's `config.json` key and a local concurrency cap that was narrower than production. The dashboard's observer initially logged an anonymous digest-resolution fallback; publishing succeeded, and authenticated observation was corrected before the native pair.

## Cleanup and evidence

Both successful pairs passed (dashboard test 236.51s, native test 153.85s). Their Jobs/Pods were removed. After verifying no App/Job/Pod remained for each synthetic ID, inventory recorded each repository's exact tags, then the disposable image tags and cache tag were retired and absence verified. Unreferenced physical blobs remain subject to normal Zot garbage collection; logical tag removal does not assert immediate disk reclamation. Existing tenant images and the separately reviewed legacy forum repository were untouched.

Session-local raw evidence: `/tmp/w7-cache-drill-live.log`, `/tmp/w7-cache-node-live.log`, BuildKit logs under `/tmp/w7-cache-drill/` and `/tmp/w7-cache-node/`, and `/tmp/w7-cache-inventory.json` / `/tmp/w7-cache-node-inventory.json`. This document preserves the results independently of those temporary files.
