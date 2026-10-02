# w4 · m142 — Admit the bounded image-v1 workloads selected by the Docker canary

**Worker:** worker4 **Goal:** a Docker service created in the already-enabled image-compatibility workspace deploys successfully without relaxing the strict workload boundary. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reconcile persisted image-v1 projection with workload admission | 60m | — |
| t002 | Verify the shared admission and workload-generator blast radius | 45m | t001 |
| t003 | Render parity | 20m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 35m | t003, t004 |
| t006 | Closeout | 10m | t005 |

## Definition of done

- In workspace `tea-d98210cbbpdc73dcrkvg`, create a free Docker web service from `https://github.com/bex-co/bex`, branch `main`, root `examples/hello-go`, port `3000`, auto-deploy off. Its initial deploy reaches Live, the service reaches Running, and `curl -sSI https://<qa-name>.onbex.co` returns 200. A manual Deploy latest commit also reaches Live. These are the initial-create and retry journeys that failed in this hunt.
- A fresh free native Go service from the same root (`go build -o app .`, `./app`) still reaches Live/Running and HTTP 200. Saving `MESSAGE=qa-<date>-updated` with deploy selected reaches the running response. This control passed during the hunt.
- Delete the two fixtures through their exact IDs; REST reads return 404 and the corresponding App resources disappear. Revoke only the QA session used for the drill.

The remaining resource families and policy-negative cases are verification work in t002/t005, not live results claimed by this hunt. Closeout requires both those checks and the live outcomes above.

## Source + Goal linkage

- **Source:** live `$qa-find-bugs`, 2026-10-02, `muse.env` account, sweep r1; [research and complete API evidence](finding.md). Local screenshots `.playwright-mcp/qa-deploys-r1-1.png` and JSON `.playwright-mcp/qa-deploys-r1-1.json` exist but are gitignored. This schedules the explicit Docker residual in [w1/m163](../../w1/done/m163/README.md), with a fresh web-service reproduction added to that milestone's earlier cron evidence.
- **Goal linkage:** ADR008's core hosting pillar; [ADR004](../../../docs/ADR004-app-deployment.md), [ADR089](../../../docs/ADR089-render-container-compatibility.md) image compatibility and its isolation gates.
- **Expected outcome:** the enabled canary's supported Docker creation flow produces a runnable service, with narrow, tested admission for its documented capability set.
- **Why now:** the backend persists `image-v1` on new Docker Apps in the canary, but the deployed admission policy rejects that exact generated workload. Retrying cannot repair it. Removing the creation flag alone leaves existing persisted Apps broken.
- **Render parity:** included because deploy state is visible through REST, GraphQL, MCP and the dashboard. Render supports Dockerfile deploys; its public documentation does not establish bex's internal capability or isolation policy.
