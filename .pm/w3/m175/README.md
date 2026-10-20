# w3 · m175 — Static sites and the activator survive a node drain

**Worker:** worker3 **Goal:** draining, rolling or losing one platform node — or one pod — never takes down static-site hosting or the activator's wake, first-deploy, suspended and maintenance responses; their availability is measured and alerted per component **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Multi-replica safety audit for static-server and activator | 45m | — |
| t002 | Two replicas, a PodDisruptionBudget and preferred anti-affinity | 30m | t001 |
| t003 | Metrics, scrape jobs and per-component availability alerts | 1h | t001 |
| t004 | Publish the static canary fixture and enable the canary's static leg (needs your OK) | 30m | — |
| t005 | Drain drill on the mock cluster: zero failed probes | 1h | t002, t003 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007, t004 |

## Definition of done

- On the mock cluster, continuous probes (≤1s interval) against a published static site, a hibernated free web service's wake path and a never-served first-deploy host record **zero** failed probes while each component's node is drained and while each Deployment rolls; recorded in `docs/drills/`.
- Production runs 2 Ready replicas of each behind a PDB (`kubectl get deploy,pdb` output recorded), spread across platform nodes.
- `StaticServerUnavailable` and `ActivatorUnavailable` fire when the component is scaled to 0 (rule test + mock cluster) and stay quiet during a normal roll.
- The deploy canary's static leg runs on `main` instead of printing `SKIP` (run id recorded).

## Evidence (2026-10-19)

- `lego/operator/config/staticserver/deployment.yaml:7` and `lego/operator/config/activator/deployment.yaml:7`: `replicas: 1`; both on the 3-node platform pool (`bex.co/pool: platform`). PDBs exist only for bex-api, ssh-gateway, OpenFGA and Traefik; the prod overlay raises only bex-api to 2 (`lego/operator/config/prod/kustomization.yaml:97-105`).
- static-server serves **every** tenant static site ([ADR029](../../../docs/ADR029-static-sites.md):20); the activator answers wake, first-deploy, suspended, failed-first-release and maintenance responses ([ADR007](../../../docs/ADR007-restart-suspend-and-resume.md):70-84).
- Neither binary exposes `/metrics` and neither is scraped; the fleet-wide `TraefikHigh5xxRate` 5% ratio can hide a static-only outage.
- Readiness is a TCP probe on the serving port (`cmd/staticserver/main.go:71`, `cmd/activator/main.go:146`; no reserved paths per w4/m162). `appwatch.Start` waits for cache sync (`internal/appwatch/appwatch.go:69`), but static-server starts its resolver in a goroutine (`cmd/staticserver/main.go:168`) before `ListenAndServe` (`:183`) — whether a cold replica can answer before its index is built is unverified.
- `scripts/deploy-canary.sh:222-232` skips the static leg every run because `BEX_CANARY_STATIC_REPO` (`deploy-canary.yml:51`) is unset — the owed ADR088 §6 step.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-10-19, item 4 (approved by the user the same day).
- **Goal linkage:** V0 roadmap 1 (deploy and mount on `*.onbex.co`) and 6 (add and remove machines): hosting must survive the node operations the platform performs routinely; ADR088 §6's owed static canary leg.
- **Expected outcome:** platform-node rolls, CAPI replacements and single-pod crashes stop being platform-wide static-hosting and wake outages; a regression is visible per component.
- **Why now:** every platform-node roll (Kubernetes upgrades, CAPI Machine replacements — e.g. the 2026-07-28 rolls) evicts the only pod of each with no PDB to hold it; one crash is a platform-wide outage for every static site and every sleeping free service, and nothing alerts on it specifically.
- **Render parity omitted:** no REST/GraphQL/MCP/UI change — replica count, disruption budget, metrics and alerts only; tenant-visible behavior changes only in availability.
