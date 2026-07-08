# w1 · m4 — Free tier = sleep: scale-to-zero + wake activator

**Worker:** worker1 **Goal:** Make `Free ≈ $0` real via `sleep = free`: idle free apps scale to 0 (occupy nothing → overcommit beyond the reserved sum) and wake on the next request via a **lean activator — a custom Go service, _not_ full Knative or KEDA** (see Decision). Paid tiers stay always-on. **Status:** done — **DONE**

## Tasks (in order)

| id | title | est | depends_on | |
| --- | --- | --- | --- | --- |
| t001 | Idle → scale Deployment to 0 (per `idleTTLSeconds`) | 30m | — | — **DONE** |
| t002 | Spike — KEDA HTTP add-on vs custom activator (pick one) | 30m | t001 | — **DONE** |
| t003 | Implement the wake path (chosen in t002) | 30m | t002 | — **DONE** |
| t004 | Traefik routing: sleeping app → activator/interceptor | 30m | t003 | — **DONE** |
| t005 | Verify sleep→wake; paid tiers stay always-on | 20m | t001,t004 | — **DONE** |

## Definition of done

An idle free app scales to 0; the next HTTP request wakes it and returns 200 after the cold start. Paid tiers never sleep. ✓

## Decision — full Knative? No. KEDA? No. Custom activator.

Evaluated full **Knative Serving** and rejected it. **Render** — a PaaS peer — adopted Knative for exactly this (free-tier scale-to-zero, Nov 2021); when their free tier exploded (Heroku's free tier ending) they hit Knative's per-app overhead — **2N+1 Kubernetes Services per app**, plus controllers watching every Pod/Service cluster-wide → CPU burn + network-programming latency — and **ripped Knative out for a home-grown activator** ([render.com/blog/knative](https://render.com/blog/knative)). Why full Knative is wrong for bex:

- **We need a slice, not the stack** — only free-tier 0↔1 wake, not Knative's revisions, traffic-splitting, concurrency autoscaling, or its networking layer (Kourier/Istio).
- **It fights the density thesis** — Knative's per-app overhead is the opposite of bin-pack-for-cost on cheap Hetzner nodes.
- **It duplicates what we already own** — bex has the `App` CRD, the operator, the Traefik edge, and a revision model; Knative Serving overlaps/conflicts with all four.
- **Peer precedent** — the closest peer did Knative → hit the wall → went custom. We start where they ended.

**KEDA HTTP add-on also rejected** (t002 spike decision): requires the KEDA operator in the cluster, per-app HTTPScaledObject CRs, and a separate interceptor controller. bex already owns the Ingress and operator loop; adding KEDA duplicates effort. The custom path is ~150 lines of Go.

**Custom activator (implemented):** `lego/operator/cmd/activator/main.go` — receives traffic for sleeping apps, touches `app.bex.co/last-active`, bumps `Deployment.spec.replicas = 1` (triggering operator reconcile), returns 503 Retry-After. Operator flips Ingress backend: app Service when running, `bex-activator` Service when sleeping. No new CRDs, no external operator dependencies.

## Implementation summary (2026-07-08)

**Operator changes** (`lego/operator/`):
- `internal/controller/app_controller.go`: `annotLastActive` constant; `isFreeApp()`, `lastActiveTime()`, `shouldAutoHibernate()`, `idleRequeueAfter()` helpers; auto-hibernate logic in `reconcileKubernetes()` — stamps `last-active` on first Running, schedules `RequeueAfter`, scales to 0 and flips Ingress to activator on TTL expiry.
- `AppReconciler.ActivatorService` / `ActivatorPort` from `BEX_ACTIVATOR_SERVICE` / `BEX_ACTIVATOR_PORT` env; empty ⇒ feature off.
- `cmd/manager/main.go`: wire new fields.

**Activator** (`lego/operator/cmd/activator/main.go`):
- HTTP server on `:8888`; `/healthz` for readiness; wake handler: find App by Host, patch annotation + Deployment, return 503 Retry-After with HTML loading page (browser) or JSON (API).
- Built as `/activator` in the shared `lego/Dockerfile` (same image, `command: ["/activator"]` in Deployment).

**k8s manifests** (`lego/operator/config/activator/`):
- Deployment, Service (`:8888`), ServiceAccount, Role (get/list/watch/patch Apps; get/patch Deployments), RoleBinding — all in `bex-system`.

**Tests**: `internal/controller/idle_test.go` — 17 new unit tests; all 23 controller tests pass.

## Source

Converted from `.tmp/003-free-sleep-activator.md`; Knative decision informed by https://render.com/blog/knative.
