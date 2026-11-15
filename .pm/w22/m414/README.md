# w22 · m414 — Sandbox egress is metered per workspace, and a workspace whose sandboxes egress far above its plan raises an alert

**Worker:** worker22 **Goal:** the operator's egress meter counts the public-internet bytes of each workspace's `/v1/sandboxes` sandboxes, an alert fires when one workspace's sandboxes send far more than its plan makes plausible, and a production run proves the counter against bytes a sandbox really moved. **Status:** todo (t001–t003 done and shipped; t004 waits for the next batched production deploy)

## Tasks (in order)

| id | title | est | depends_on |
| ---- | ------------------------------------------------------------------ | ----- | ---------- |
| t001 | Operator: the egress meter attributes sandbox pods to their workspace — **DONE** | 1h30m | — |
| t002 | The alert: a workspace whose sandboxes egress far above its plan — **DONE** | 1h30m | t001 |
| t003 | The panel and the docs — **DONE** | 45m | t002 |
| t004 | Live check: a sandbox's downloaded bytes show in the meter | 1h | t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 30m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- A sandbox's public egress appears in `bex_app_direct_egress_bytes_total` under its workspace's sandbox namespace, an App pod is attributed as before, and an agent-session sandbox is not counted
- The alert fires in a promtool test for one workspace above the threshold and stays silent below it and when the meter is unhealthy; its panel exists and `scripts/obs-coverage-check.sh` passes
- A production run moves a known number of bytes from a sandbox and the counter rises by about that much
- ADR104 gap 14 and D11 no longer list sandbox egress as open

## Source + Goal linkage

- **Source:** inbox note w22/002 (promoted 2026-11-14 by `/loopx w22`), whose gate (does any byte counter exist for a sandbox pod?) was answered by the production measurement of 2026-11-14/15: yes, on the node, but not in Prometheus; and ADR104 D11, which requires the meter and the alert.
- **Goal linkage:** Render parity first (ADR008 § Render parity first; ADR104 D1 and D11): `allow-all` is the default network policy and free workspaces can run sandboxes, so egress is the abuse and cost axis the plan does not bound; pillar 5 sandboxes (re-opened 2026-07-27, `.pm/DO_NOT_DO.md` #18).
- **Expected outcome:** abusive or runaway sandbox egress is visible and alerted per workspace, using the meter that already attributes app egress.
- **Why now:** w22/m311 shipped groups and the concurrency limit on 2026-11-14, and ADR104 D11 made this alert a requirement of the milestones that widen what a sandbox can do.
- **Render parity:** omitted. The meter, the alert and the panel are internal; no REST, GraphQL, MCP or dashboard surface changes. Render's pages checked on 2026-11-10 give no sandbox egress price, so egress is not billed (decided in the 2026-11-14 record of w22/002).
