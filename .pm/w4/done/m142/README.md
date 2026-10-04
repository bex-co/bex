# w4 · m142 — Admit the bounded image-v1 workloads selected by the Docker canary

**Worker:** worker4 **Goal:** a Docker service created in the already-enabled image-compatibility workspace deploys successfully without relaxing the strict workload boundary. **Status:** done 2026-10-02 — all tasks complete; live acceptance passed in production.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reconcile persisted image-v1 projection with workload admission — **DONE** | 60m | — |
| t002 | Verify the shared admission and workload-generator blast radius — **DONE** | 45m | t001 |
| t003 | Render parity — **DONE** | 20m | t002 |
| t004 | Simplify — **DONE** | 15m | t003 |
| t005 | Test coverage — **DONE** | 35m | t003, t004 |
| t006 | Closeout — **DONE** | 10m | t005 |

## Definition of done

- In workspace `tea-d98210cbbpdc73dcrkvg`, create a free Docker web service from `https://github.com/bex-co/bex`, branch `main`, root `examples/hello-go`, port `3000`, auto-deploy off. Its initial deploy reaches Live, the service reaches Running, and `curl -sSI https://<qa-name>.onbex.co` returns 200. A manual Deploy latest commit also reaches Live. These are the initial-create and retry journeys that failed in this hunt.
- A fresh free native Go service from the same root (`go build -o app .`, `./app`) still reaches Live/Running and HTTP 200. Saving `MESSAGE=qa-<date>-updated` with deploy selected reaches the running response. This control passed during the hunt.
- Delete the two fixtures through their exact IDs; REST reads return 404 and the corresponding App resources disappear. Revoke only the QA session used for the drill.

The remaining resource families and policy-negative cases are verification work in t002/t005, not live results claimed by this hunt. Closeout requires both those checks and the live outcomes above.

## Source + Goal linkage

- **Source:** live `$qa-find-bugs`, 2026-10-02, `muse.env` account, sweep r1; [research and complete API evidence](finding.md). Local screenshots `.playwright-mcp/qa-deploys-r1-1.png` and JSON `.playwright-mcp/qa-deploys-r1-1.json` exist but are gitignored. This schedules the explicit Docker residual in [w1/m163](../../../w1/done/m163/README.md), with a fresh web-service reproduction added to that milestone's earlier cron evidence.
- **Goal linkage:** ADR008's core hosting pillar; [ADR004](../../../../docs/ADR004-app-deployment.md), [ADR089](../../../../docs/ADR089-render-container-compatibility.md) image compatibility and its isolation gates.
- **Expected outcome:** the enabled canary's supported Docker creation flow produces a runnable service, with narrow, tested admission for its documented capability set.
- **Why now:** the backend persists `image-v1` on new Docker Apps in the canary, but the deployed admission policy rejects that exact generated workload. Retrying cannot repair it. Removing the creation flag alone leaves existing persisted Apps broken.
- **Render parity:** included because deploy state is visible through REST, GraphQL, MCP and the dashboard. Render supports Dockerfile deploys; its public documentation does not establish bex's internal capability or isolation policy.

## Outcome — 2026-10-02

Implementation is verified; [verification and rollout contract](verification.md) records the API-server regression, original-failure proof, caller matrix and unchanged rollout scope. t002, t004 and t005 are done. **BLOCKED:** the production-deploy pipeline must release the updated admission policy and operator image; then QA must execute the Docker/native initial-deploy, retry, environment-update, adapter parity and exact-ID cleanup probes in the DoD. t001 implementation is complete but its live criterion remains open; t003 live replay and t006 closeout remain open. No production fixture was created by this implementation run, and its QA session was revoked.

## Live acceptance — 2026-10-02

Production carried fix `d95d017ba` (ancestor of deployed `18958459c`; a later deploy also succeeded). Read-only check: the live `bex-operator-workloads` policy now declares `imageCompatibilityWorkload`/`imageCompatibilityCaps`. Workspace `tea-d98210cbbpdc73dcrkvg`, REST/GraphQL/MCP with a QA browser session; times UTC 2026-10-03.

- **Docker create:** `qa-20261002-w4x-docker2` (`srv-db09t0qtm2ss7389qmt0`), free Docker web, `bex-co/bex@main`, root `examples/hello-go`, port 3000, auto-deploy off. Initial deploy `dep-db09t0qtm2ss7389qmtg` → `live` 06:22:24; GraphQL phase `Running` rev-1; `curl -sSI https://qa-20261002-w4x-docker2.onbex.co/` → HTTP/2 200, body `qa-20261002-w4x-docker-initial`. (A first attempt `srv-db09qu2tm2ss7389qmb0` was a harness input error — explicit `dockerContext: "."` overrides rootDir — and failed at `go.mod not found` in the build, not admission; deleted.)
- **Retry:** REST manual deploy `dep-db09ufatm2ss7389qn3g` → `live` 06:24:24; dashboard Deploys → Manual Deploy → **Deploy latest commit** `dep-db09vhitm2ss7389qn90` → `live` 06:26:36; phase Running rev-3. No admission refusal on any deploy.
- **Native control:** `qa-20261002-w4x-native` (`srv-db09r08ehcmc739j11lg`, `go build -o app .` / `./app`) initial `dep-db09r08ehcmc739j11m0` → `live`, HTTP 200 `qa-20261002-w4x-native-initial`. Saving `MESSAGE=qa-20261002-w4x-updated` + deploy `dep-db09t32tm2ss7389qmvg` → `live`; HTTP body `qa-20261002-w4x-updated`.
- **Adapter parity (t003):** successful Docker deploys read `live` identically on REST list, GraphQL `deploys`, MCP `list_deploys` and the dashboard list. Real failure: native start command set to `./qa-missing-binary` → `dep-db09vioehcmc739j12k0` `update_failed` with reason "the deploy did not become healthy within the health-gate window; check the service logs" on REST, GraphQL, MCP `get_deploy` and dashboard list/detail (Failed). Observations, not blockers for this milestone: the generic gate reason landed although `deploy_started` carried the specific `stallReason` (exit code 127); the service header showed "Deploying" while GraphQL phase read `Hibernated` after the failure.
- **Cleanup:** all three services deleted by exact ID; REST GET → 404 for each; read-only `kubectl get apps.app.bex.co -A` shows no `w4x` App. QA session revoked at the end of the run (see w4 board report).
