# w8 · m113 — Services list times out (524/502) for the bex workspace

**Worker:** worker8 **Goal:** `GET /v1/services` answers inside the edge timeout for every workspace, so `bex services` works in the largest production workspace **Status:** todo

## Tasks (in order)

| id   | title                                                              | est | depends_on    |
| ---- | ------------------------------------------------------------------ | --- | ------------- |
| t001 | Profile the slow segment of the services list for workspace bex   | 40m | —             |
| t002 | Fix the root cause so the list answers well under the edge limit  | 60m | w8/m113/t001  |
| t003 | Render parity: REST/GraphQL/MCP list latency and shapes agree     | 30m | w8/m113/t002  |
| t004 | Simplify the changed code                                          | 20m | w8/m113/t003  |
| t005 | Add regression coverage for list latency/shape at scale            | 30m | w8/m113/t003  |
| t006 | Close out the milestone                                            | 10m | w8/m113/t005  |

## Definition of done

- `BEX_WORKSPACE=tea-d98210cbbpdc73dcrkvg bex services -o json` (pinned Render CLI v2.27.0 contract, human device login) exits 0 with the full JSON array in under 10s, run twice at human pace (2026-10-18 repro: 110s → `Error: received response code 502`, then 125s → `524`; workspaces bex-canary/tian-personal answer in ~1s).
- The same list through GraphQL and MCP `list_services` for that workspace returns the same service id set in under 10s; no shape change on any surface.
- The slow segment is named with evidence (service count + per-segment timing), not guessed: handler `lego/backend/internal/apps/rest.go:675` → `Service.List` (`lego/backend/internal/apps/service.go:1362`, k8s `AppList` in the tenant namespace + `readViews` placements batch at `lego/backend/internal/apps/placement.go:35`) → `restServices` owner/credential batches (`lego/backend/internal/apps/render.go:632`). All three enrichments already batch, so the profile — not a rewrite — decides the fix.
- Control workspaces (bex-canary `tea-daif693dqjvc73e7as3g`, 8 services) still answer in ~1s with byte-identical shape.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` live sweep 2026-10-18 (this loop, sweep 1): HEAD-built bex (`799f509c`, checkout build labeled vdev; installed v0.2.1 first repro) against `https://api.bex.co/v1/`, human device login as the QA user. Deployed revision unobserved. Other list endpoints in the same workspace (`projects`, `postgres list`, `keyvalues list`) answer in 1s, isolating the defect to the services list.
- **Goal linkage:** Render parity (ADR018) — the pinned Render CLI's first read on any workspace is the services list; a core journey (list → pick → act) cannot start in the operator's own production workspace. Severity: blocker for that workspace.
- **Expected outcome:** the largest production workspace lists services through CLI/REST/GraphQL/MCP in seconds; the fix is measured, so the next order-of-magnitude growth is visible before it times out again.
- **Why now:** it is live in production now and silently worsens as the workspace grows; every other CLI journey in that workspace is blocked behind it.
- Render parity closing task applies: the list is exposed on REST, GraphQL, and MCP (t003).
- **Unverified:** the bex workspace's exact service count and which segment dominates (k8s list vs placements vs per-app `view()` vs owner resolution) — t001 measures it; no server-side timing was captured this sweep. Dashboard (GraphQL) latency for the same workspace was not timed this sweep.
