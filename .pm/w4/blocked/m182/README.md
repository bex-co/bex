# w4 · m182 — Keep configured port changes available

**Worker:** worker4 **Goal:** a supported service-port edit keeps requests reaching an existing ready listener until the requested listener is ready, then exposes the new port without an unresolved backend or wrong-port endpoint. **Status:** blocked

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Keep configured-port serving routes valid through rollout and adoption — **DONE** | 90m | — |
| t002 | Audit configured-port callers types aliases and passing controls — **DONE** | 45m | t001 |
| t003 | Render parity — **DONE** | 20m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage and production-shaped route replay — **DONE** | 45m | t003, t004 |
| t006 | Closeout | 10m | t005 |

## Definition of done

Recreate the owned disk-free Free prebuilt web fixture in [finding.md](finding.md), command following injected PORT, TCP health-check mode, one replica, no pre-deploy command. The following are the observed production journeys, with the required target behavior named explicitly:

- Wait for port 3001 to be Live/HTTP 200, then Settings → Edit Port → 3000 → Save changes while sampling the public URL. Every response is the app's HTTP 200: old port=3001 body until ready new port=3000 body. No 404/502/503 or missing backend occurs. One new config_change deploy reaches Live and the public body reports the requested PORT.
- Hard-load Settings, change 3000→3001 and repeat. During candidate-unready observations, the primary Service's admitted old pod is routed to its existing 3000 listener; after promotion the new 3001 listener serves. Both CR-named and slug-named Service projections retain a resolvable binding rather than immediately pointing the old endpoint at 3001. Primary EndpointSlice/pod comparison and public sampling prove the behavior; live slug clients and their EndpointSlice are additional t002 work.
- At unchanged port 3001, Manual Deploy → Restart service → confirm still reaches Live with the same public marker/PORT. The replay's during-rollout and after-Live sampling returns HTTP 200, preserving this observed control. The stronger original m154 env-only continuous Live+60s probe remains required t002/t005 work before closeout.
- Complete t002's explicitly unverified compatibility work, including existing unnamed-pod adoption and m121/m154 contract audits, then remove the owned fixture and revoke only the test session. Do not close from desired-object assertions alone.

## Source + Goal linkage

- **Source:** user-requested infinite qa-find-bugs loop, muse.env, w4, 2026-10-08 cycle 10. [finding.md](finding.md) includes complete matching mutation/read probes, complete public response samples, selected cluster observation, primary consumer sources, dedupe and cleanup.
- **Goal linkage:** ADR008 pillar 1 / reliable Render-compatible hosting; ADR004 readiness/drain and immutable release behavior; ADR041 stable internal identities; ADR006 port setter/read contract; ADR018 parity.
- **Expected outcome:** configured-port edits finish on the requested listener while old ready pods remain useful during rollout, including existing unnamed deployments. Normal Restart, port validation, alias and backend precedence contracts remain intact.
- **Why now:** two fresh live edits each caused more than 12 seconds of sampled 404/502 failures. Cluster evidence shows a ready old 3000 pod being dialed on 3001; unchanged-port Restart sampled cleanly. This is an actual availability gap in an existing supported control.
- **Scope / dedupe:** filing only, **major**, ~3h45m / six tasks. Separate from w4/m181 job admission, w4/m180 source reuse and w1/done/m154 SIGTERM drain; not a claim that those fixes regressed. Shared helper/caller compatibility has its own t002, including every original m121 DoD clause and both m154 DoD clauses.
- **Standing closing tasks:** Render parity, simplify, coverage and closeout included because runtime availability and the dashboard/REST/GraphQL/MCP port contract are tenant-facing.

## Progress (2026-10-08)

t001–t005 done. A configured-port App's container port is named `bex-http` (`appContainer`). `applyClusterIPService`, shared by the CR-named Service, the slug alias and image-network promotion, names the Service port `bex-http` and targets that name. kube-proxy and the ingress therefore resolve each pod's own listener: ready old pods keep serving on the old port until new pods are ready on the requested one. The Service port number stays the configured port, so `<slug>:<port>`, the Ingress backend `Number`, `release_plan.go`'s prior-port read and the activator/maintenance/static numeric bindings are unchanged. Image-port private services (named `tcp-N`, revision-constrained) are untouched.

Adoption: `servingTargetPort` keeps a numeric target, set to that pod's own port, while any live selected pod's app container lacks the named port. So existing unnamed pods never drop out; the switch to the name rides the existing Pod watch once they are gone. Two caveats:

- **Fleet rollout:** the operator upgrade changes the pod template, so it rolls every running configured-port App once (parked Apps are untouched). During that rollout the Service stays numeric and old and new pods share a port, so it is safe.
- **Restored templates:** a cancel/failed-rollout restore of a template recorded **before** this change brings back unnamed pods. While they run, the Service is numeric on their port, which is the old behavior; that is temporary and limited to pre-upgrade records.

Evidence: `TestConfiguredPortServiceTargetsEachPodsOwnListener` covers the named container port, a named target with no pods and with mixed old/new named pods, a pre-upgrade unnamed pod keeping its numeric port, and terminating pods being ignored. Local replay on 2026-10-08 used real kube-proxy and EndpointSlices: a busybox Deployment edited from 3000 to 3001 with an 8 s start delay, sampled 120 times through the Service. The numeric target failed **41** samples; the named target failed **0**, cutting over from `port=3000` to `port=3001`. Operator `make test` and `make lint` pass. ADR004 records the rule.

Not verified until the live replay:

- The Traefik public path and both EndpointSlices in production.
- Restart at an unchanged port, and m154's Live+60 s probe.
- m121's edit/read/refusal controls (no code touched them).
