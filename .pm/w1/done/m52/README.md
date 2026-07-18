# w1 · m52 — Zero-error deploy rolls: bex-api drain + dashboard roll resilience

**Worker:** worker1 **Goal:** a routine `deploy.yml` roll of bex-api + the dashboard is invisible to users — no failed API calls, no "Couldn't load resources", no SSR error page, no stranded error state that needs a manual reload. **Status:** done — shipped as `3cb335cf` and DoD verified on prod 2026-07-18 (two consecutive clean rolls of both Deployments under load; evidence below).

## The incident (evidence)

2026-07-18 06:54 UTC: the deploy run for `2c9a7143` finished at 06:54:12 and replaced both bex-api pods (RS `55c5498c6c` → `5b6db6f948`, pod starts 06:54:46 / 06:55:00), with a `FailedScheduling` stall in between (anti-affinity across only 3 platform nodes). The user was moving a service in/out of project `prj-d9dgeo0bd9nc73a0vh1g` at exactly that moment — the audit log shows three `projects.SetServices` calls at 06:54:43 / 06:54:49 / 06:55:09, **all allowed and committed** — yet the dashboard showed move-error toasts and `/project/prj-…` stranded on "Couldn't load resources" until a manual reload, because the route loader's failed query is dehydrated as a persistent `state: "error"` (`dashboard/src/common/lib/document-head/index.ts` `loadRouteResource`). Five deploy runs rolled prod between 06:22 and 07:03 that morning — every roll is currently a user-visible error window. The dashboard half (post-roll SSR error page, stale error `<title>`) was already filed as inbox `036` at the m51 closeout; folded in here as t002.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | bex-api pre-shutdown drain window + rollout strategy hardening — **DONE** | 45m | — |
| t002 | Dashboard roll: readiness gated on upstream reachability + post-roll SSR error page (folds `036`) — **DONE** | 60m | — |
| t003 | Dashboard client resilience: retry transient reads, re-run SSR-errored loaders on hydration — **DONE** | 45m | — |
| t004 | Controlled-roll end-to-end verification under synthetic load — **DONE** | 45m | t001, t002, t003 |
| t005 | Render parity check — **DONE** | 20m | t004 |
| t006 | Simplify — **DONE** | 30m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005 |
| t008 | Closeout — **DONE** | 15m | t007 |

## Verification evidence (2026-07-18, all times UTC)

**Baseline (pre-fix, prod, quiet hour):** controlled `kubectl rollout restart` of each Deployment under a ~1 rps HTTPS loop. Dashboard roll 07:36:05–07:36:28 → **4× 502** at 07:36:27–29 (old pod killed; Node exits instantly on SIGTERM while Traefik still routes to it), 33 requests. bex-api roll 07:37:03–07:37:27 → **3× 502** at 07:37:14 and 07:37:28 (one per old-pod kill), 227 requests. This is the incident mechanism, reproduced on demand.

**bex-api in-process drain (t001):** local run of the new binary — SIGTERM ⇒ `/readyz` 503 immediately while `/healthz` and regular requests keep answering 200 for exactly the 15s window, then clean exit. Cluster-level: new image (`bex-local:m52`) rolled twice on the local CAPD cluster (2 replicas, new probes/strategy) under a continuous in-cluster curl loop against the Service — rolls 08:25:12–08:25:36 and 08:26:02–08:26:26, **712 requests, 0 non-200**.

**Dashboard preStop mechanism (t002):** prod patched to the new manifest (preStop `sleep 10` + `maxUnavailable: 0/maxSurge: 1` + `minReadySeconds: 5`; the `/healthz` readiness probe needs the new image, so it ships with the code). Roll killing a preStop-equipped pod (08:00:02–08:00:39) plus the subsequent Argo-window transitions: **~189 full-page loads, 0× 502** (one 10s-timeout on a cold pod's first SSR render — addressed by liveness `timeoutSeconds: 5` warm-up + the `/healthz` readiness gate). The patch matches the uncommitted manifest exactly, so prod front-runs the shipped state (no Argo drift observed). Cold-pod `GET /` readiness probe timeouts observed in events further confirm 036's readiness-measures-the-wrong-thing hypothesis.

**Client self-recovery incl. stale title (t003 + 036):** live dev-1 browser choreography through a TCP proxy giving a 5s API outage: client navigation mid-outage ⇒ "Couldn't load resources" + title "Something went wrong ・ bex Dashboard" at t=13.9s ⇒ **hands-off recovery to full data + correct title at t=15.2s** (~0.5s after the API returned) — no reload, no user action. Root causes fixed on the way: `router.invalidate()` re-runs loaders but never recomputes a match's head/`meta`, and even a retry navigation captures the error title because `meta` is computed at commit from the loaderData available then — hence the hook's second same-location replace navigation on error→ready.

**Parity (t005):** zero diffs under `internal/*/{rest,graphql,mcp}.go`; wire spot-check on the new binary: `/v1/services` and `/graphql` return the identical Render-dialect 401 shape, `/healthz` unchanged, `/readyz` additive and outside `/v1` (infra endpoint, no parity row).

**Tests (t007):** backend `go test ./...` green (new `internal/serve` drain-sequence + readiness tests, m30 tests migrated there); operator `make test` (envtest) green; dashboard 242 files / 1518 tests + typecheck + lint green (new: retry-link, loader-error-retry, health-latch, document-title reconcile tests).

**Prod DoD run (post-ship, 2026-07-18 10:24–10:51 UTC):** shipped as `3cb335cf`; under continuous load loops, deploy roll #1 (Argo convergence onto the new image/manifests) and a manual roll #2 of both Deployments completed with **api 3523 requests / dashboard 1454 requests, zero non-2xx, zero error-page bodies** (one correlated client-side rc=28 timeout on both loops at 10:43:48, between rolls — a shared network-path blip, not a roll effect). The old bex-api pod exited `Completed` after its drain window; `maxUnavailable: 0` kept service up even while the interim mixed-generation state below blocked the handover.

**Two latent prod bugs surfaced and fixed at closeout** — the readiness latch refused to lie about them: (1) the dashboard image has always baked `VITE_SSR_API_URL=http://bex-api.bex-system.svc:8090/graphql` (deploy.yml) but the bex-api Service only exposed port 80, and (2) `deny-tenant-ingress` (bex-system) never admitted the dashboard namespace — so **every prod SSR data fetch had been silently connection-refused**, dehydrating error states that client hydration papered over (the deeper root of 036's error page). Fixed live and in git: a port-8090 `http-alt` alias on the Service (`lego/operator/config/api/service.yaml` — keeps the URL baked into every existing image generation valid, rollbacks included) + a scoped `allow-dashboard-ssr-to-bex-api` NetworkPolicy (`deploy/gitops/base/network-policies.yaml`). **These two files were changed after `3cb335cf` shipped and ride the next `/ship`; prod carries equivalent live objects meanwhile.** With them in place, prod SSR pages fetch real data server-side for the first time.

Also from the roll: a deploy-ordering gap worth knowing — Argo syncs manifest changes the moment they land on main, ~10 min before CI's digest write-back, so a probe/spec change referencing new-binary behavior briefly produces a NotReady surge pod on the old image; `maxUnavailable: 0` makes that window harmless (old pods keep serving) and the roll self-heals when the pin lands. The operator CI flake that cost one deploy cycle is `w1/038`.

## Definition of done

Two consecutive controlled rolls (bex-api and the dashboard, e.g. `kubectl rollout restart`) under continuous synthetic load — an API request loop against api.bex.co and repeated full-page dashboard loads — complete with **zero** non-2xx responses and **zero** served error pages. A dashboard page held open through a roll self-recovers to data without a manual reload (no stranded "Couldn't load resources"), and the post-roll first hit renders normally (no "Something went wrong" SSR page, no lingering stale error `<title>`) — the `036` symptom is gone. Evidence (timestamps, request-loop counts, pod transitions) recorded at closeout.

## Source + Goal linkage

- **Source:** 2026-07-18 prod incident (user report: move-to-project errors + "Couldn't load resources" on `/project/prj-d9dgeo0bd9nc73a0vh1g`; root-caused same day to the 06:54 UTC roll collision — audit log + k8s events above) + inbox note `036` (post-roll SSR error page, observed twice on 2026-07-18, filed at the m51 closeout).
- **Goal linkage:** w1's charter is "de-risk the live system" — production reliability of the Render-alternative core (docs/ADR008-vision.md). Render's own dashboard never surfaces their deploys to users; bex currently does, several times a day. Completes the arc m30 started (SIGTERM in-process graceful shutdown drains in-flight requests) — what remains is the window where Traefik still routes _new_ requests to a terminating pod, and the dashboard's blind readiness/dehydrated-error behavior around it.
- **Expected outcome:** `deploy.yml`'s many daily rolls stop being user-visible error windows; mid-roll user actions and page loads succeed or transparently recover.
- **Why now:** deploy cadence is the risk multiplier — five prod rolls in 40 minutes the same morning; the incident reproduced twice in one day (m50, m51 rolls) and will keep reproducing on every deploy until fixed.
- **Render parity included** (t005): the fix touches the dashboard UI's user-visible error/recovery behavior; REST/GraphQL/MCP contracts are expected to be untouched, and t005 verifies exactly that plus compares the roll-window UX against Render's dashboard.
