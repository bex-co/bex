# w1 · m52 — Zero-error deploy rolls: bex-api drain + dashboard roll resilience

**Worker:** worker1 **Goal:** a routine `deploy.yml` roll of bex-api + the dashboard is invisible to users — no failed API calls, no "Couldn't load resources", no SSR error page, no stranded error state that needs a manual reload. **Status:** in progress — all code + manifests implemented and verified (local cluster rolls, prod dashboard rolls, live browser recovery); the strict prod DoD run of the shipped bex-api image rides the first post-`/ship` deploy roll (t004 note).

## The incident (evidence)

2026-07-18 06:54 UTC: the deploy run for `2c9a7143` finished at 06:54:12 and replaced both bex-api pods (RS `55c5498c6c` → `5b6db6f948`, pod starts 06:54:46 / 06:55:00), with a `FailedScheduling` stall in between (anti-affinity across only 3 platform nodes). The user was moving a service in/out of project `prj-d9dgeo0bd9nc73a0vh1g` at exactly that moment — the audit log shows three `projects.SetServices` calls at 06:54:43 / 06:54:49 / 06:55:09, **all allowed and committed** — yet the dashboard showed move-error toasts and `/project/prj-…` stranded on "Couldn't load resources" until a manual reload, because the route loader's failed query is dehydrated as a persistent `state: "error"` (`dashboard/src/common/lib/document-head/index.ts` `loadRouteResource`). Five deploy runs rolled prod between 06:22 and 07:03 that morning — every roll is currently a user-visible error window. The dashboard half (post-roll SSR error page, stale error `<title>`) was already filed as inbox `036` at the m51 closeout; folded in here as t002.

## Tasks (in order)

| id   | title                                                                                             | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | bex-api pre-shutdown drain window + rollout strategy hardening — **DONE**                         | 45m | —                |
| t002 | Dashboard roll: readiness gated on upstream reachability + post-roll SSR error page (folds `036`) — **DONE** | 60m | —                |
| t003 | Dashboard client resilience: retry transient reads, re-run SSR-errored loaders on hydration — **DONE** | 45m | —                |
| t004 | Controlled-roll end-to-end verification under synthetic load — verified local+prod-mechanism; prod run of the shipped image pends `/ship` | 45m | t001, t002, t003 |
| t005 | Render parity check — **DONE**                                                                    | 20m | t004             |
| t006 | Simplify — **DONE**                                                                               | 30m | t005             |
| t007 | Test coverage — **DONE**                                                                          | 45m | t005             |
| t008 | Closeout — pends t004's post-ship confirmation                                                    | 15m | t007             |

## Verification evidence (2026-07-18, all times UTC)

**Baseline (pre-fix, prod, quiet hour):** controlled `kubectl rollout restart` of each Deployment under a ~1 rps HTTPS loop. Dashboard roll 07:36:05–07:36:28 → **4× 502** at 07:36:27–29 (old pod killed; Node exits instantly on SIGTERM while Traefik still routes to it), 33 requests. bex-api roll 07:37:03–07:37:27 → **3× 502** at 07:37:14 and 07:37:28 (one per old-pod kill), 227 requests. This is the incident mechanism, reproduced on demand.

**bex-api in-process drain (t001):** local run of the new binary — SIGTERM ⇒ `/readyz` 503 immediately while `/healthz` and regular requests keep answering 200 for exactly the 15s window, then clean exit. Cluster-level: new image (`bex-local:m52`) rolled twice on the local CAPD cluster (2 replicas, new probes/strategy) under a continuous in-cluster curl loop against the Service — rolls 08:25:12–08:25:36 and 08:26:02–08:26:26, **712 requests, 0 non-200**.

**Dashboard preStop mechanism (t002):** prod patched to the new manifest (preStop `sleep 10` + `maxUnavailable: 0/maxSurge: 1` + `minReadySeconds: 5`; the `/healthz` readiness probe needs the new image, so it ships with the code). Roll killing a preStop-equipped pod (08:00:02–08:00:39) plus the subsequent Argo-window transitions: **~189 full-page loads, 0× 502** (one 10s-timeout on a cold pod's first SSR render — addressed by liveness `timeoutSeconds: 5` warm-up + the `/healthz` readiness gate). The patch matches the uncommitted manifest exactly, so prod front-runs the shipped state (no Argo drift observed). Cold-pod `GET /` readiness probe timeouts observed in events further confirm 036's readiness-measures-the-wrong-thing hypothesis.

**Client self-recovery incl. stale title (t003 + 036):** live dev-1 browser choreography through a TCP proxy giving a 5s API outage: client navigation mid-outage ⇒ "Couldn't load resources" + title "Something went wrong ・ bex Dashboard" at t=13.9s ⇒ **hands-off recovery to full data + correct title at t=15.2s** (~0.5s after the API returned) — no reload, no user action. Root causes fixed on the way: `router.invalidate()` re-runs loaders but never recomputes a match's head/`meta`, and even a retry navigation captures the error title because `meta` is computed at commit from the loaderData available then — hence the hook's second same-location replace navigation on error→ready.

**Parity (t005):** zero diffs under `internal/*/{rest,graphql,mcp}.go`; wire spot-check on the new binary: `/v1/services` and `/graphql` return the identical Render-dialect 401 shape, `/healthz` unchanged, `/readyz` additive and outside `/v1` (infra endpoint, no parity row).

**Tests (t007):** backend `go test ./...` green (new `internal/serve` drain-sequence + readiness tests, m30 tests migrated there); operator `make test` (envtest) green; dashboard 242 files / 1518 tests + typecheck + lint green (new: retry-link, loader-error-retry, health-latch, document-title reconcile tests).

**Remaining for closeout (t008):** after the next `/ship`, the first `deploy.yml` roll (or a manual quiet-hour restart of both prod Deployments) under the t004 load loop is the confirmatory end-to-end — expected zero non-2xx / zero error pages now that both halves are in the image + manifests.

## Definition of done

Two consecutive controlled rolls (bex-api and the dashboard, e.g. `kubectl rollout restart`) under continuous synthetic load — an API request loop against api.bex.co and repeated full-page dashboard loads — complete with **zero** non-2xx responses and **zero** served error pages. A dashboard page held open through a roll self-recovers to data without a manual reload (no stranded "Couldn't load resources"), and the post-roll first hit renders normally (no "Something went wrong" SSR page, no lingering stale error `<title>`) — the `036` symptom is gone. Evidence (timestamps, request-loop counts, pod transitions) recorded at closeout.

## Source + Goal linkage

- **Source:** 2026-07-18 prod incident (user report: move-to-project errors + "Couldn't load resources" on `/project/prj-d9dgeo0bd9nc73a0vh1g`; root-caused same day to the 06:54 UTC roll collision — audit log + k8s events above) + inbox note `036` (post-roll SSR error page, observed twice on 2026-07-18, filed at the m51 closeout).
- **Goal linkage:** w1's charter is "de-risk the live system" — production reliability of the Render-alternative core (docs/ADR008-vision.md). Render's own dashboard never surfaces their deploys to users; bex currently does, several times a day. Completes the arc m30 started (SIGTERM in-process graceful shutdown drains in-flight requests) — what remains is the window where Traefik still routes *new* requests to a terminating pod, and the dashboard's blind readiness/dehydrated-error behavior around it.
- **Expected outcome:** `deploy.yml`'s many daily rolls stop being user-visible error windows; mid-roll user actions and page loads succeed or transparently recover.
- **Why now:** deploy cadence is the risk multiplier — five prod rolls in 40 minutes the same morning; the incident reproduced twice in one day (m50, m51 rolls) and will keep reproducing on every deploy until fixed.
- **Render parity included** (t005): the fix touches the dashboard UI's user-visible error/recovery behavior; REST/GraphQL/MCP contracts are expected to be untouched, and t005 verifies exactly that plus compares the roll-window UX against Render's dashboard.
