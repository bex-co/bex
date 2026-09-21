# w9 · m94 — Sandbox `sbx-` typed IDs

**Worker:** worker9 **Goal:** Every sandbox bex mints carries a Render-shaped `sbx-` ID, so the pinned CLI's `ea sandboxes copy` arg parser accepts it. **Status:** todo

## Tasks (in order)

| id   | title                                                     | est | depends_on         |
| ---- | --------------------------------------------------------- | --- | ------------------ |
| t001 | Mint and persist `sbx-` IDs, dual-accept legacy UUIDs     | 50m | —                  |
| t002 | Carry the canonical ID through exec/connect/stop/get/list | 45m | w9/m94/t001        |
| t003 | Contract + regression tests for the ID shape              | 40m | w9/m94/t002        |
| t004 | Live-verify with the distributed CLI, update evidence     | 40m | w9/m94/t003        |
| t005 | Render parity                                             | 30m | w9/m94/t004        |
| t006 | Simplify                                                  | 25m | w9/m94/t005        |
| t007 | Test coverage                                             | 45m | w9/m94/t005        |
| t008 | Closeout                                                  | 15m | w9/m94/t006, w9/m94/t007 |

## Definition of done

- [ ] `bex ea sandboxes create --plan=starter -o json` returns `"id": "sbx-…"` (never a bare UUID).
- [ ] `bex ea sandboxes copy ./file <sbx-id>:/tmp/file` passes the pinned client's arg parser — it must never fail with `must be a sandbox path`; full byte round-trip stays gated on w7/m150's routes.
- [ ] `exec`, `stop`, `list`, and `get` resolve `sbx-` IDs; pre-existing UUID IDs keep working (dual-accept) or 404 with the non-enumerating sandbox shape — never 500.
- [ ] REST, GraphQL, and MCP report the identical canonical ID for one sandbox.
- [ ] `docs/cli-compatibility-checklist.md` `ea sandboxes` rows cite the dated live result.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` live hunt 2026-09-20 (bex v0.2.1, pin v2.27.0 `a764810a7682`, production `https://api.bex.co/v1/`, workspace `bex-canary`). No open or done `.pm` item covers the ID shape (`sbx-` appears only as a placeholder in `.pm/w7/done/m147/README.md` and billing-test fixtures); w7/m150 (open) scopes routes/tokens/streaming with no ID-shape task; no `git log -S 'sbx-'` fix in flight.
- **Goal linkage:** ADR008 pillar 5 + ADR018 sandbox coverage: the m150 file-copy feature cannot work through the distributed CLI while IDs fail client-side validation.
- **Expected outcome:** The pinned, unmodified CLI addresses every bex sandbox for copy/exec/stop/list/get with a Render-shaped ID.
- **Why now:** Blocks w7/m150's t005 CLI round trip before it starts; the code already documents `sbx-<xid>` as the contract (`lego/backend/internal/sandbox/models.go:64`), so production UUIDs are a defect against the written contract, not a design question.
- **Render parity:** included — the fix changes the tenant-facing ID on REST/GraphQL/MCP; compare against Render's `sbx-` shape on each surface.
