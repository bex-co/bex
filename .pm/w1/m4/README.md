# w1 · m4 — Free tier = sleep: scale-to-zero + wake activator

**Worker:** worker1 **Goal:** Make `Free ≈ $0` real via `sleep = free`: idle free apps scale to 0 (occupy nothing → overcommit beyond the reserved sum) and wake on the next request via a **lean activator — KEDA's HTTP add-on or a small custom Go service, _not_ full Knative** (see Decision). Paid tiers stay always-on. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Idle → scale Deployment to 0 (per `idleTTLSeconds`) | 30m | — |
| t002 | Spike — KEDA HTTP add-on vs custom activator (pick one) | 30m | t001 |
| t003 | Implement the wake path (chosen in t002) | 30m | t002 |
| t004 | Traefik routing: sleeping app → activator/interceptor | 30m | t003 |
| t005 | Verify sleep→wake; paid tiers stay always-on | 20m | t001,t004 |

## Definition of done

An idle free app scales to 0; the next HTTP request wakes it and returns 200 after the cold start. Paid tiers never sleep.

## Decision — full Knative? No (build lean)

Evaluated full **Knative Serving** and rejected it. **Render** — a PaaS peer — adopted Knative for exactly this (free-tier scale-to-zero, Nov 2021); when their free tier exploded (Heroku's free tier ending) they hit Knative's per-app overhead — **2N+1 Kubernetes Services per app**, plus controllers watching every Pod/Service cluster-wide → CPU burn + network-programming latency — and **ripped Knative out for a home-grown activator** ([render.com/blog/knative](https://render.com/blog/knative)). Why full Knative is wrong for bex:

- **We need a slice, not the stack** — only free-tier 0↔1 wake, not Knative's revisions, traffic-splitting, concurrency autoscaling, or its networking layer (Kourier/Istio).
- **It fights the density thesis** — Knative's per-app overhead is the opposite of bin-pack-for-cost on cheap Hetzner nodes.
- **It duplicates what we already own** — bex has the `App` CRD, the operator, the Traefik edge, and a revision model; Knative Serving overlaps/conflicts with all four.
- **Peer precedent** — the closest peer did Knative → hit the wall → went custom. We start where they ended.

**Middle ground (decided in t002):** **KEDA's HTTP add-on** = scale-to-zero + an HTTP interceptor (an off-the-shelf activator), far lighter than Knative, and may save building the buffer/scale loop ourselves. t002 picks KEDA-add-on vs. fully custom — either way, **not full Knative**.

## Source

Converted from `.tmp/003-free-sleep-activator.md`; Knative decision informed by https://render.com/blog/knative.
