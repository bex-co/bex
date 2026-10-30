# w3 · m175 — Static sites and the activator survive a node drain

**Worker:** worker3 **Goal:** draining, rolling or losing one platform node — or one pod — never takes down static-site hosting or the activator's wake, first-deploy, suspended and maintenance responses; their availability is measured and alerted per component **Status:** todo — t001–t003, t006, t007 done; 2026-10-29 unblock: t004 approved (public fixture repo), t005 drill on a rebuilt mock cluster (production replicas/PDB already verified); t008 closeout follows

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Multi-replica safety audit for static-server and activator — **DONE** | 45m | — |
| t002 | Two replicas, a PodDisruptionBudget and preferred anti-affinity — **DONE** | 30m | t001 |
| t003 | Metrics, scrape jobs and per-component availability alerts — **DONE** | 1h | t001 |
| t004 | Publish the static canary fixture and enable the canary's static leg (needs your OK) | 30m | — |
| t005 | Drain drill on the mock cluster: zero failed probes | 1h | t002, t003 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 30m | t005 |
| t008 | Closeout | 10m | t006, t007, t004 |

## Definition of done

- On the mock cluster, continuous probes (≤1s interval) against a published static site, a hibernated free web service's wake path and a never-served first-deploy host record **zero** failed probes while each component's node is drained and while each Deployment rolls; recorded in `docs/drills/`.
- Production runs 2 Ready replicas of each behind a PDB (`kubectl get deploy,pdb` output recorded), spread across platform nodes.
- `StaticHostingPathDown` fires when either component has no available replica (rule test + mock cluster) and stays quiet during a normal roll (custom metrics descoped: see t003).
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

## Unblock 记录（2026-10-29，/unblock w3，origin/main `e4d9c65d6`）

- **涉及：** t004、t005、t008。
- **用户明确决定 D3（2026-10-29）：** 批准新建公开仓库 `bex-co/bex-canary-static`（只放一个静态 `index.html`，不需要构建、不含 secret），并设置仓库变量 `BEX_CANARY_STATIC_REPO` 指向它。
- **现有规则（AGENTS.md「Local dev environments are pre-approved」）：** `scripts/mock-cluster.sh` 的完整重建（包括 reprovision）不需要用户批准，mock cluster 宕机不构成 blocked。2026-10-29 复核：`docker ps` 仍只剩 `bex-tenant-0-…` 一个容器，worker 需先重建，并在交接里报告重建了什么。
- **本次复核证据（生产只读）：** `kubectl -n bex-system get deploy,pdb`：`bex-activator` 2/2、`bex-static-server` 2/2，PDB `minAvailable: 1`（已存在 8 天）；副本分布在 `bex-platform-qs4bv-fzhbs` 和 `bex-platform-qs4bv-zsnxg`。t005 第 5 步（生产确认）已满足。
- **剩余阻塞：** 无。worker 做 t004（建仓库、设变量、dispatch canary，记录静态段执行且为绿的 run id，更新 ADR088 §6）→ t005（重建 mock cluster 后做 drain/roll 演练）→ t008。
