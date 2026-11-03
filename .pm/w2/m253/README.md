# w2 · m253 — A service that crash-loops for 24 hours is suspended with `stuck_crashlooping`, as on Render

**Worker:** worker2 **Goal:** a service that repeatedly fails to start and become healthy over 24 hours is suspended with Render's `stuck_crashlooping` suspender, its owners are told, and a user resume brings it back. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Decide and record the detection rule | 45m | — |
| t002 | Operator: detect and suspend with a recorded reason | 1.5h | t001 |
| t003 | Surface stuck_crashlooping on REST, GraphQL, MCP, events, webhooks and the dashboard | 1h | t002 |
| t004 | Suspension notice by email and push | 30m | t003, w2/m252/t005 |
| t005 | dev-2 walk with a shortened window | 45m | t002 |
| t006 | Render parity | 15m | t004, t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 30m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- A service with no Ready instance for 24h while restarts continue is suspended; REST/GraphQL/MCP report `suspended: true`, `suspenders: ["stuck_crashlooping"]`, and the dashboard explains why.
- A service that becomes Ready at any point in the window is not suspended; services that are building, deploying, hibernated or already suspended are excluded.
- A user resume clears the suspender and the service starts again.
- Owners receive the suspension notice by email (w2/m252 path) and push.
- An envtest proves the 24h boundary; dev-2 proves the flow with a shortened window override.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-11-02 #3. render.com/docs/notifications: "Render suspends a service that repeatedly fails to start and become healthy over a 24-hour period … often caused by a service configuration error, a bug in application code, or an undersized compute plan." Render's pinned `suspenderType` enum (`render-public-api-1.json`) includes `stuck_crashlooping`; bex has zero references to it. eden-cms-v2 (2026-08-20) was liveness-killed 33×/16h.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); dense bin-pack economics (crash loops waste node capacity).
- **Expected outcome:** forgotten crash loops stop consuming capacity, and owners learn why their service stopped.
- **Why now:** it is an unimplemented value of a pinned Render enum; w2/m252 provides the email path for the notice.
- **Render parity included:** `suspenders`, events, webhooks and the dashboard banner are tenant-facing.
- **Open design point:** Render does not publish the exact detection rule; t001 records bex's rule in an ADR.
