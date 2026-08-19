# w1 · m70 — MCP parity pin: classify all 213 tools and fail closed on drift

**Worker:** worker1 **Goal:** make "is this MCP tool Render parity or a bex extension?" a CI answer instead of a hand-written comment — the same guarantee REST already has. **Status:** done

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | Capture + pin `render-oss/render-mcp-server`'s tool inventory | 45m | — | — **DONE** |
| t002 | Parity-class registry: every `mcp.AddTool` declares its class | 1h | t001 | — **DONE** |
| t003 | Classify all 213 existing tools against the pin | 2h | t002 | — **DONE** |
| t004 | Guard tests: 1:1 immutability + unclassified-tool build failure | 45m | t003 | — **DONE** |
| t005 | CI drift job mirroring `render-schema-drift.yml` | 45m | t004 | — **DONE** |
| t006 | Record the real inventory in ADR018 + ADR006 | 45m | t003 | — **DONE** |
| t007 | Simplify | 30m | t005, t006 | — **DONE** |
| t008 | Test coverage | 45m | t005, t006 | — **DONE** |
| t009 | Closeout | 15m | t007, t008 | — **DONE** |

## Outcome

Landed 2026-08-18. **Upstream registers 22 MCP tools; bex registers 213** — measured, not estimated, by building each server and reading its own `tools/list`. Class counts: **10 `Parity1to1`, 1 `Superset`, 8 `Divergent`, 194 `Extension`** ([full inventory](evidence/inventory.md)).

The pin earned its keep on the first run by finding things no comment or ledger row had:

- **8 tools share an upstream name while breaking its contract.** Four are genuine bex/Render differences (no caller-chosen `region`; no PR previews). **Four are unintended bugs** — `create_postgres` spells Render's `diskSizeGb` as `diskSizeGB`; `create_static_site` drops `autoDeploy`/`buildCommand`; `get_metrics` renames three arguments; `trigger_deploy` omits `clearCache`, which reaches REST and GraphQL but was never wired into MCP despite ADR018's w2/m30 row. Each is accepted with a written reason and filed for repair; none is fixed here (this milestone renames nothing).
- **A stale comment, caught exactly as designed.** `internal/secrets/mcp.go` claimed "Render's official MCP has no env-var tools"; upstream has shipped `update_environment_variables` since. bex covers it as `update_env_vars` — a name divergence now recorded in `mcpKnownUpstreamOnly`.
- **Upstream is shrinking, and it matters for m71.** `v0.3.0` had 24 tools, `main` has 22: Render **removed** `update_web_service`/`update_static_site`/`update_cron_job` in #89 (2026-07-23) — "stop exposing placeholder update tools that only return dashboard links". m71/t001 should choose its target grammar knowing upstream deliberately walked away from `update_*`.

**One deliberate deviation from the plan, and one addition.**

- Classes are **derived** from the pin, not declared at each of the 213 call sites. A hand-maintained 213-row table would rot exactly the way the ten dated comments it replaces did. The no-silent-regrowth property the plan wanted is instead held by `TestMCPParityInventory`, which pins the total and the per-class counts: any new tool fails it until both the test and ADR018 are updated together. What *is* hand-maintained is the small set of human decisions — accepted divergences and declined upstream tools — and both lists are guarded against going stale in either direction.
- A **fourth class, `Divergent`**, beyond the three scoped. The pin found eight on its first run and neither `Superset` (claims a call-compatibility bex lacks) nor `Extension` (claims bex owns a shared name) describes them honestly.

**Flagged, not fixed:** `internal/jobs` ships a Service, REST endpoints, GraphQL fields, and 4 MCP tools, while one-off jobs are a declared non-goal in `.pm/DO_NOT_DO.md` and `—` on all four surfaces in ADR018. Either the ledger row is stale or the surface should not exist; recorded in `internal/jobs/mcp.go` for a decision rather than silently resolved inside a parity-pin milestone.

## Definition of done

- A checked-in snapshot of `render-oss/render-mcp-server`'s tool list (names + argument shapes + upstream version/commit + capture date) lives beside `internal/api/openapi/render-public-api-1.json`, pinned by content hash the way `renderOpenAPISHA256` pins the REST spec.
- Every one of the 213 `mcp.AddTool` call sites declares a parity class — `Parity1to1`, `Superset`, or `Extension` — through a registry, not a comment. A tool with no declared class fails the build (the `internal/id` `id.Kind` registry + guard test is the working precedent).
- A test asserts that no `Parity1to1` tool's name or argument names change, and that every `Parity1to1` tool still exists in the pin.
- A CI job fails when the pinned upstream inventory goes stale, mirroring `.github/workflows/render-schema-drift.yml` → `scripts/render-schema-drift.sh`.
- ADR018 records the real per-class counts, replacing the current position where parity is asserted in ten hand-written comments carrying manual check dates (`"checked live 2026-07-13"`, `"@ 2a00be1, checked 2026-07-12"`, `"v0.3.0"`).
- No tool is renamed, removed, or behaviourally changed by this milestone.

## Source + Goal linkage

- **Source:** principal-engineer architecture review, 2026-08-18 (session hand-off, no inbox note). Measured at HEAD: 213 registered MCP tools; 86 of 212 parseable descriptions self-declare `"bex extension"`; whole packages declare it once at the file header instead of per tool (`internal/secrets/mcp.go` "Render's official MCP has no env-var tools" → 10 tools; `internal/envgroups/mcp.go` → 17; `internal/jobs/mcp.go` → 4); `internal/apps/mcp.go:32` records that of its 58 tools only `list_services`/`get_service` are 1:1.
- **Goal linkage:** ADR008's AI-native pillar and ADR018's parity ledger. MCP is the agent-facing surface; today it is the only one of the four with no machine-checked contract.
- **Expected outcome:** a real per-class inventory. The asymmetry this closes is concrete — REST parity is pinned (`renderOpenAPISHA256` + `render-schema-drift.yml` + `TestRenderConformance` + 119-operation request validation) while MCP has no pin, no drift job, and no test, which is how the surface reached 213 tools without anyone deciding to.
- **Why now:** it is a prerequisite for m71. The set of tools safe to collapse cannot be named until the parity class of each is machine-checkable, and a collapse done from comments would risk breaking agents written against Render's MCP. Pinning first is also non-destructive — it ships and delivers value even if m71 is never approved.
- **Render parity closing task omitted:** this milestone changes no REST/GraphQL/MCP/UI behaviour — it adds a pin, a registry, tests, and a CI job. Including a parity-check task would be circular: the milestone *is* the parity mechanism.
