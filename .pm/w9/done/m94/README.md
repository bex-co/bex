# w9 · m94 — Sandbox `sbx-` typed IDs

**Worker:** worker9 **Goal:** Every sandbox bex mints carries a Render-shaped `sbx-` ID, so the pinned CLI's `ea sandboxes copy` arg parser accepts it. **Status:** done (2026-10-02)

## Tasks (in order)

| id   | title                                                     | est | depends_on         |
| ---- | --------------------------------------------------------- | --- | ------------------ |
| t001 | Mint and persist `sbx-` IDs, dual-accept legacy UUIDs     | 50m | —                  | — **DONE** |
| t002 | Carry the canonical ID through exec/connect/stop/get/list | 45m | w9/m94/t001        | — **DONE** |
| t003 | Contract + regression tests for the ID shape              | 40m | w9/m94/t002        | — **DONE** |
| t004 | Live-verify with the distributed CLI, update evidence     | 40m | w9/m94/t003        | — **DONE** |
| t005 | Render parity                                             | 30m | w9/m94/t004        | — **DONE** |
| t006 | Simplify                                                  | 25m | w9/m94/t005        | — **DONE** |
| t007 | Test coverage                                             | 45m | w9/m94/t005        | — **DONE** |
| t008 | Closeout                                                  | 15m | w9/m94/t006, w9/m94/t007 | — **DONE** |

## Definition of done

- [x] `bex ea sandboxes create --plan=starter -o json` returns `"id": "sbx-…"` (never a bare UUID).
- [x] `bex ea sandboxes copy ./file <sbx-id>:/tmp/file` passes the pinned client's arg parser — it must never fail with `must be a sandbox path`; full byte round-trip stays gated on w7/m150's routes.
- [x] `exec`, `stop`, `list`, and `get` resolve `sbx-` IDs; pre-existing UUID IDs keep working (dual-accept) or 404 with the non-enumerating sandbox shape — never 500.
- [x] REST, GraphQL, and MCP report the identical canonical ID for one sandbox.
- [x] `docs/cli-compatibility-checklist.md` `ea sandboxes` rows cite the dated live result.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` live hunt 2026-09-20 (bex v0.2.1, pin v2.27.0 `a764810a7682`, production `https://api.bex.co/v1/`, workspace `bex-canary`). No open or done `.pm` item covers the ID shape (`sbx-` appears only as a placeholder in `.pm/w7/done/m147/README.md` and billing-test fixtures); w7/m150 (open) scopes routes/tokens/streaming with no ID-shape task; no `git log -S 'sbx-'` fix in flight.
- **Goal linkage:** ADR008 pillar 5 + ADR018 sandbox coverage: the m150 file-copy feature cannot work through the distributed CLI while IDs fail client-side validation.
- **Expected outcome:** The pinned, unmodified CLI addresses every bex sandbox for copy/exec/stop/list/get with a Render-shaped ID.
- **Why now:** Blocks w7/m150's t005 CLI round trip before it starts; the code already documents `sbx-<xid>` as the contract (`lego/backend/internal/sandbox/models.go:64`), so production UUIDs are a defect against the written contract, not a design question.
- **Render parity:** included — the fix changes the tenant-facing ID on REST/GraphQL/MCP; compare against Render's `sbx-` shape on each surface.

## Status 2026-09-20 — BLOCKED (needs the fix deployed to `api.bex.co` before the live CLI probes can run)

Implemented and shipped: bex mints the tenant-facing `sbx-<xid>` id itself
(`lego/backend/internal/id` + `bex.co/sandbox-id` sandbox metadata), every read
path reports it, every write path dual-accepts a pre-m94 substrate id, and the
three surfaces agree. t001/t002/t003/t005/t006/t007 are done with backend and
CLI contract tests (mutation-checked); full backend suite, `lego/cli` suite, and
`make lint-backend` green.

What remains is **only** t004: the four live journeys against production with the
distributed `bex` binary, which cannot run until this commit is deployed —
production still serves the pre-fix image. t008 closeout follows t004. The exact
commands and the one checklist sentence to replace are in `t004.md`.

Two deliberate carve-outs, recorded so a future reader does not read them as
drift:

- **Agent-session sandboxes are not stamped.** Their identity is load-bearing
  inside the platform — the exec gateway derives the pod as `<id>-0` and the
  usage display-name join matches `agent_sessions.sandbox_id` — so their
  canonical id stays the substrate id. They are addressed by session id on their
  own surfaces, never by `sbx-`.
- **Pre-m94 sandboxes keep their substrate ids** (dual-accept, not backfill),
  which the milestone's own scope already excluded.

## Done 2026-10-02 — live-verified on production

The released `bex v0.2.1` against production in `bex-canary` passed every check. `create` returned `sbx-…`. `copy` passed the client's parser, uploaded, and round-tripped the bytes (the m150 routes are live too). `exec -- true` exited 0 and `exit 7` exited 7. `list` showed the id. After `stop`, `list` was empty and `exec`/`GET` returned `404 SANDBOX_NOT_FOUND`. REST, GraphQL, and MCP reported the identical id. A bare UUID gave the non-enumerating 404, never 500. The full transcript is in `done/t004.md`, and the checklist row is updated. The fixture was stopped and is absent.
