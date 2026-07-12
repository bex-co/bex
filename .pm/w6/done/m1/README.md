# w6 · m1 — Workspace model & lifecycle verbs (create · rename · delete · plan limits)

**Worker:** worker6 **Goal:** Workspaces stop being an invisible auto-minted row: a user can create additional workspaces (name + plan), rename them, and delete them with full resource teardown — over the existing control-plane `tenants`/`tenant_members` tables and OpenFGA `workspace:tea-<id>` tuples, with Render's plan limits (5 Hobby workspaces/user, 25 services, 1 member on Hobby) enforced in Core. **Status:** done — backend workspace lifecycle shipped (commit `b06e301`) and verified end-to-end against real Postgres + real OpenFGA (`workspaces_e2e_test`: create/rename/delete, plan-limit refusals, FGA grant/revoke, App-CR teardown, non-admin 403). Deferred to follow-ups: OpenBao/Database purger concrete impls (secrets pkg + m9 tenant labels) and the t001 live Render-dashboard capture. **Update (w6/m5, 2026-07-11):** t001's live capture is now **resolved** — `docs/render-artifacts/workspace-lifecycle.md` was produced from a real authenticated `dashboard.render.com` session (settings/rename/delete semantics captured verbatim, incl. Render's `sudo delete workspace <name>` delete-confirm phrase); the delete guard was reconciled to match (see `w6/m5`).

## Tasks (in order)

| id   | title                                                                                             | est | depends_on   |
| ---- | ------------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Capture live Render workspace settings/rename/delete semantics → `docs/render-artifacts/`         | 30m | — | — **DONE** (acceptance criteria met retroactively in `w6/m5/t001`: `docs/render-artifacts/workspace-lifecycle.md` live-captured 2026-07-11) |
| t002 | Store schema: `plan` on tenants + lifecycle columns; migration + plan-limits constants            | 30m | w1/m9 | — **DONE** |
| t003 | Core verbs: CreateWorkspace + RenameWorkspace (tenant row + FGA admin tuple; 5-Hobby cap)          | 35m | t002 | — **DONE** |
| t004 | Core verb: DeleteWorkspace — guarded teardown of Apps, Databases, env-vars, FGA tuples            | 35m | t001, t003 | — **DONE** |
| t005 | Enforce plan limits in Core: 25-service Hobby cap on create; Hobby single-member guard            | 25m | t003 | — **DONE** |
| t006 | GraphQL surface: `workspaces` query + create/rename/delete mutations (dashboard-shaped)           | 25m | t004, t005 | — **DONE** |
| t007 | Render parity — REST stays mutation-free on owners; GraphQL/MCP/UI semantics match captured Render | 20m | t006 | — **DONE** |
| t008 | Simplify — `/simplify` over the code this milestone changed                                        | 20m | t007 | — **DONE** |
| t009 | Test coverage — meaningful tests for the behavior this milestone shipped                           | 30m | t007 | — **DONE** |
| t010 | Closeout — move milestone to done when DoD holds                                                   | 10m | t009 | — **DONE** |

## Definition of done

On a cluster with `BEX_CP_DB_URI` + enforced OpenFGA: an identity that already has its m9-minted workspace creates a second workspace (name + `hobby` plan) via GraphQL → new `tenants` row (`tea-` id), `tenant_members` row, FGA admin tuple; a 6th Hobby workspace is refused with a limit error; the 26th service create in a Hobby workspace is refused; rename is visible in list/get; delete tears down the workspace's App CRs, Database CRs, and OpenBao `tenants/<id>` secrets, removes FGA tuples, and subsequent access 404/403s; a non-admin member cannot rename/delete. REST `/v1/owners` remains free of POST/PATCH/DELETE.

## Source + Goal linkage

- **Source:** deep-research report [`w6/RESEARCH-workspaces.md`](../RESEARCH-workspaces.md) (findings 1–4, 9; open question 1), user request 2026-07-08 (Render `/new/workspace` + entire workspace lifecycle parity, composed with existing authn/authz/database).
- **Goal linkage:** GOAL.md #5 (multi-tenant); docs/vision.md pillar 1 (Render parity). Gap vs board: w1/m9 mints exactly one workspace per identity and has no user-initiated create/rename/delete or multi-workspace; w4/m12 covers members/roles, not the workspace object's own lifecycle.
- **Expected outcome:** the tenancy substrate becomes a user-facing product object — multiple workspaces per user with Render's plan limits enforced server-side, and a safe delete path that doesn't strand Apps, CNPG clusters, or OpenBao secrets.
- **Why now:** w1/m9 is in flight; designing lifecycle verbs against its tuple/row model while it lands avoids forking the tenancy write path (same rationale that gates w4/m12). m2 (owners API) and m3 (dashboard UX) both build on these verbs.
- **Render parity task included:** yes — feature work on a tenant-facing surface; parity here notably includes _not_ adding REST mutations (research finding 9).
