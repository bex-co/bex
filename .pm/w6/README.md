# w6 — Workspaces: Render workspace lifecycle parity (worker6)

**Worker:** worker6 Created 2026-07-08 from user request + deep-research report ([RESEARCH-workspaces.md](RESEARCH-workspaces.md)). w1/m9 mints the tenant substrate (one auto-created workspace per identity, OpenFGA enforced); w6 makes workspaces a real product surface: user-initiated lifecycle (create/rename/delete, multi-workspace per user, plan limits), the Render `owners` read API + MCP workspace tools, and the dashboard flows (`/new/workspace`, switcher, settings). Composes with existing authn (Kratos/Hydra), authz (OpenFGA `workspace:tea-<id>`), and the control-plane Postgres — no parallel workspace store. Ordered by dependency: model + verbs → API surface → dashboard UX.

## Milestones

- [x] **m1** — Workspace model & lifecycle verbs: create · rename · delete · plan limits (10 tasks) ← from RESEARCH-workspaces.md — done 2026-07-09 (backend shipped `b06e301`, verified vs real Postgres + OpenFGA), moved to `done/m1/`
- [x] **m2** — Render `owners` read API + MCP workspace tools (9 tasks) ← from RESEARCH-workspaces.md, needs m1; supersedes w2/002 — done, moved to `done/m2/`
- [x] **m3** — Dashboard workspace UX: `/new/workspace` flow · switcher · settings (9 tasks) ← from RESEARCH-workspaces.md, needs m1 — done 2026-07-09 (switcher/create/settings/delete shipped, verified end-to-end against the offline dev stub — no live cluster available this session, see `done/m3/README.md`), moved to `done/m3/`
- [ ] **m4** — Workspace-scoped datastores: fix ownerId labeling + wire real delete purgers (10 tasks) ← from `/pm-brainstorm for w6` 2026-07-09, closing a gap m1 itself deferred ("OpenBao/Database purger concrete impls") plus a discovered Postgres label-mismatch bug and a KeyValue labeling gap
- [ ] **m5** — Live-verify workspace dashboard UX against Render + real infrastructure (9 tasks) ← from `/pm-brainstorm for w6` ("more for w6") 2026-07-09, closing m1/t001's unmet live-capture acceptance criteria and m3's own "Follow-ups" (offline-stub-only verification, ungenerated codegen diff)

## Not in w6 (deliberate)

- **Members & roles** (invite/change-role/remove, Team page) — that's `w4/m12`; w6/m1 only enforces the Hobby single-member guard w4/m12 must respect.
- **Billing/payments** — bex has no billing system; the plan field ships without a payment step (Render's flat-subscription + inactivity-waiver model is documented in RESEARCH-workspaces.md for whenever billing becomes roadmap-worthy).
- **Workspace-mutation REST endpoints** — parity means their _absence_ (research finding 9: Render's REST owners surface is read-only; lifecycle is dashboard-only).
