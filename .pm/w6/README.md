# w6 — Workspaces: Render workspace lifecycle parity (worker6)

**Worker:** worker6 Created 2026-07-08 from user request + deep-research report ([RESEARCH-workspaces.md](RESEARCH-workspaces.md)). w1/m9 mints the tenant substrate (one auto-created workspace per identity, OpenFGA enforced); w6 makes workspaces a real product surface: user-initiated lifecycle (create/rename/delete, multi-workspace per user, plan limits), the Render `owners` read API + MCP workspace tools, and the dashboard flows (`/new/workspace`, switcher, settings). Composes with existing authn (Kratos/Hydra), authz (OpenFGA `workspace:tea-<id>`), and the control-plane Postgres — no parallel workspace store. Ordered by dependency: model + verbs → API surface → dashboard UX.

## Milestones

- [x] **m1** — Workspace model & lifecycle verbs: create · rename · delete · plan limits (10 tasks) ← from RESEARCH-workspaces.md — done 2026-07-09 (backend shipped `b06e301`, verified vs real Postgres + OpenFGA), moved to `done/m1/`
- [ ] **m2** — Render `owners` read API + MCP workspace tools (9 tasks) ← from RESEARCH-workspaces.md, needs m1; supersedes w2/002
- [ ] **m3** — Dashboard workspace UX: `/new/workspace` flow · switcher · settings (9 tasks) ← from RESEARCH-workspaces.md, needs m1

## Not in w6 (deliberate)

- **Members & roles** (invite/change-role/remove, Team page) — that's `w4/m12`; w6/m1 only enforces the Hobby single-member guard w4/m12 must respect.
- **Billing/payments** — bex has no billing system; the plan field ships without a payment step (Render's flat-subscription + inactivity-waiver model is documented in RESEARCH-workspaces.md for whenever billing becomes roadmap-worthy).
- **Workspace-mutation REST endpoints** — parity means their _absence_ (research finding 9: Render's REST owners surface is read-only; lifecycle is dashboard-only).
