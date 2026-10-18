# w8 · m113 — Services list times out (524/502) for the bex workspace

**Worker:** worker8 **Goal:** `GET /v1/services` answers inside the edge timeout for every workspace, so `bex services` works in the largest production workspace **Status:** done — production CLI, REST, GraphQL and MCP acceptance passed on 2026-10-18

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Profile the slow segment of the services list for workspace bex](done/t001.md) | 40m | — — **DONE 2026-10-18** |
| t002 | [Fix the root cause so the list answers well under the edge limit](done/t002.md) | 60m | w8/m113/t001 — **DONE 2026-10-18** |
| t003 | [Render parity: REST/GraphQL/MCP list latency and shapes agree](done/t003.md) | 30m | w8/m113/t002 — **DONE 2026-10-18** |
| t004 | [Simplify the changed code](done/t004.md) | 20m | w8/m113/t003 — **DONE 2026-10-18** |
| t005 | [Add regression coverage for list latency/shape at scale](done/t005.md) | 30m | w8/m113/t003 — **DONE 2026-10-18** |
| t006 | [Close out the milestone](done/t006.md) | 10m | w8/m113/t005 — **DONE 2026-10-18** |

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
- **Acceptance:** [t006](done/t006.md) records two checkout-built human-device-login CLI runs plus REST, GraphQL and MCP agreement; the root cause and scale are measured in t001.
- **Root cause history (onset inferred from code history, not measured):** the slow segment t001 found is the env-group metadata sweep. On 2026-08-17 `cacde0622` (codex-security round 11, ADR066) bounded the then-global sweep with a 15s in-process snapshot (`envGroupMetaCacheTTL`). On 2026-08-19 `ffc1cd6bb` (w2/m80) replaced that global sweep with a workspace-prefix sweep plus a legacy-tenant union and **retired the cache**. From then on every list request paid the full serial OpenBao pass, so each deleted group's resurrection fence (`plantDeletedMetaFence`, w4/m97) cost roughly 50ms on every scan — about 260 listed ids in bex. The CLI's services journey pays that scan once per project, then again for datastores. The concurrent scan shipped here hides the cost but leaves it linear in groups ever deleted. Removing the sweep altogether is [w8/m114](../../blocked/m114/README.md).
