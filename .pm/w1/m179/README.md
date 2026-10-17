# w1 · m179 — A failed first release stops instead of restarting forever

**Worker:** worker1 **Goal:** a release that never served and settled failed has no running or restarting pod, and comes back only through a new deploy, restart or resume. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Reproduce the restart loop on dev-1 and record Render's contract for a failed first deploy | 40m | —          |
| t002 | Park a never-served failed release at zero replicas until a new deploy, restart or resume  | 1h  | t001       |
| t003 | Surfaces report the parked failed service truthfully                                       | 30m | t002       |
| t004 | Live check on dev-1                                                                        | 30m | t003       |
| t005 | Render parity                                                                              | 20m | t004       |
| t006 | Simplify                                                                                   | 15m | t005       |
| t007 | Test coverage                                                                              | 40m | t005       |
| t008 | Closeout                                                                                   | 10m | t007       |

## Definition of done

- Five minutes after a never-served release's failed verdict, its Deployment has 0 replicas and no pod; it stays that way across reconciles and public requests for at least 30 minutes (t004).
- `GET /v1/services/{id}/instances` returns `[]`, the service reads Failed and the deploy row keeps its diagnosis on REST, GraphQL, MCP and the dashboard.
- A new deploy, a restart and a resume each recover the service; the served-release paths (w8/m44, w1/m156, m157, m160, m172) behave exactly as before.

## Evidence (verified 2026-10-17 at `2c95fd35b`)

- `release_plan.go:146-190`: `parked` = suspended or auto-hibernating only; `nextAct` returns `actRoll` when `!f.servedRuntime`.
- `app_controller.go:2809-2842`: `desiredReplicas` returns the configured count unless auto-hibernating. `:3344-3376`: the never-served settle sets `PhaseFailed` and touches no replicas.
- Live: `.pm/w8/done/039.md:38` — a startup-probe kill every 15 minutes until deletion. Private services, background workers and paid web services are never auto-hibernated, so they cycle indefinitely while holding reserved capacity.
- Render: render.com/docs/health-checks — after 15 minutes "Render cancels the deploy".

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-10-17 item 2, verified the same day and approved by the user for filing; originally "Also observed (not filed)" in `w8/done/039.md`.
- **Goal linkage:** truthful hosting state and ADR003's dense bin-pack economics (paid pods reserve `request=limit`).
- **Expected outcome:** no wasted capacity, restart noise or possible metering for services that never ran; the state matches Render's canceled deploy.
- **Why now:** it is the last uncovered case in the failed-release family that w1/m156, m157, m160 and m172 just closed, and their guards are fresh.
- **Render parity included:** service status and the instances list are tenant-visible.
