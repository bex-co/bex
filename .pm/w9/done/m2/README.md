# w9 · m2 — Render CLI compatibility: run the official CLI against bex-api

**Worker:** worker9 **Goal:** Prove — with captured evidence, command by command — how much of Render's official CLI (`render.com/docs/cli`, `render-oss/cli`) works against bex-api, and publish the result as `docs/cli-compatibility-checklist.md`. bex never builds its own CLI (`.pm/DO_NOT_DO.md`, 2026-07-14); the official CLI becomes the fifth verified surface the same way `render-oss/render-mcp-server` anchors the MCP surface. **Status:** done

> IMPORTANT: here is the render cli source code: ./cli FYI

## Tasks (in order)

| id   | title                                                                                                                                       | est | depends_on | status    |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- | --------- |
| t001 | CLI harness: install `render-oss/cli`, find the API-host override, authenticate against bex-api with a bex API key                          | 30m | —          | — **DONE** |
| t002 | Walk the full documented command surface against bex-api on the mock cluster; capture raw evidence per command                               | 45m | t001       | — **DONE** |
| t003 | Write `docs/cli-compatibility-checklist.md`: one row per CLI command, ✅/◐/✖/— + evidence (ADR018 style); index it in `CLAUDE.md`             | 30m | t002       | — **DONE** |
| t004 | Map every ✖/◐ row to an owner (existing milestone or new inbox note); cross-link the checklist from `docs/ADR018-render-parity.md`           | 20m | t003       | — **DONE** |
| t005 | Simplify — `/simplify` over whatever harness script/code this milestone added                                                                | 20m | t004       | — **DONE** |
| t006 | Test coverage — make the checklist's PASS claims reproducible (harness script assertions or a documented re-run procedure)                    | 30m | t004       | — **DONE** |
| t007 | Closeout — DoD met → move milestone to `done/`                                                                                               | 10m | t006       | — **DONE** |

## Definition of done

`docs/cli-compatibility-checklist.md` exists and covers every command documented at render.com/docs/cli, each marked ✅/◐/✖/— with evidence captured from a real run against bex-api (mock cluster or prod); the harness that produced the evidence is reproducible (a script or exact documented steps, including how the CLI was pointed at bex and authenticated); every ✖/◐ row names an owner (milestone or inbox note); `docs/ADR018-render-parity.md` references the checklist so the CLI surface is no longer untracked.

## Source + Goal linkage

- **Source:** user decision 2026-07-14 during `/pm-brainstorm more milestones for each worker to work on until feature parity` (round 8) — resolves the standing "bex CLI" scope question flagged since round 3; paired with the new `.pm/DO_NOT_DO.md` anti-goal (never develop a CLI from scratch).
- **Goal linkage:** Render parity (pillar 1) — the CLI is Render's fifth surface and the last one bex has never tracked; `docs/ADR006-bex-api.md`'s "Render-compatible API" claim is exactly what makes the official CLI usable as a free client.
- **Expected outcome:** an evidence-backed checklist saying which `render` CLI workflows (login, workspace, services, deploys, logs, psql, …) work against bex today, which don't and why, and who owns each gap — turning "CLI parity" from an unknown into a scoped backlog.
- **Why now:** the API-level gap-well is dry (ADR018 rows all owned; w7/m30's conformance suite guards drift in CI), so the untracked CLI surface is the largest remaining parity blind spot; the user's decision just closed the scope question that blocked this for five brainstorm rounds; w9's queue is empty.
- **Render parity closing task: omitted** — the milestone _is_ the parity check (the `w7/m30` precedent); it changes no REST/GraphQL/MCP/UI surface itself. Any surface change a gap demands is follow-up work t004 files, not this milestone's diff.
