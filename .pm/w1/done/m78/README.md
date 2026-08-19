# w1 · m78 — Single service-PATCH op table across REST and MCP

**Worker:** worker1 **Goal:** collapse the twice-maintained, already-drifted ordered op table behind `PATCH /v1/services/{id}` (REST) and `update_service` (MCP) into one table in the apps service, so the "must stay identical" contract three comments currently assert becomes structural. **Status:** done

## Tasks (in order)

| id   | title                                                       | est | depends_on |
| ---- | ----------------------------------------------------------- | --- | ---------- |
| t001 | `ServicePatch` value type + `ApplyServicePatch` core table — **DONE** | 1h  | —          |
| t002 | REST adapter: `patchService` fills `ServicePatch` — **DONE** | 45m | t001       |
| t003 | MCP adapter: `applyServicePatch` fills `ServicePatch` — **DONE** | 45m | t001       |
| t004 | Render parity — **DONE** | 45m | t002, t003 |
| t005 | Simplify — **DONE** | 30m | t004       |
| t006 | Test coverage — **DONE** | 1h  | t004       |
| t007 | Closeout — **DONE** | 15m | t006       |

## Definition of done

One ordered op table exists (in the apps service, on `core.PatchOps`); `rest.go` and `mcp.go` each reduce to a ~30-line wire→`ServicePatch` field mapping; the four current divergences (`notificationsToSend` and `autoscaling` MCP-only, `repo/image/imageOwnerId` and the maintenance-before-plan reorder REST-only) are preserved as **explicit** per-surface nils/flags at the two fill sites — zero behavior change on either surface, verified by the existing patch-pipeline, update-service-MCP, and CLI-compat suites plus a new cross-surface ordering test.

## Source + Goal linkage

- **Source:** 2026-08-19 architectural refactor review §2.2 (ledger artifact: https://claude.ai/code/artifact/fe4af1ce-211f-4109-a541-f0aabd273c73). Evidence: `apps/rest.go:864-942` hand-rolls the collector `core.PatchOps` already provides; `apps/mcp.go:763-841` builds the twin; identity is asserted at `rest.go:861-863`, `mcp.go:739-743`, `core/patchops.go:29-31` and has drifted in four fields anyway.
- **Goal linkage:** ADR006's core principle — one core, thin adapters; surfaces must not drift. This is the principle's own flagship violation.
- **Expected outcome:** a new PATCH field can no longer land on one surface and silently miss the other; the divergences become visible decisions at one call site; net ~−30 lines.
- **Why now:** the table is still drifting (four divergences and counting); every new service setting widens the gap. Render parity is included because the change sits directly on the Render-compatible PATCH surface.
