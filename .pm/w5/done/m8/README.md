# w5 · m8 — Databases page (managed Postgres, Render-consistent)

**Worker:** worker5 **Goal:** The dashboard gains a Databases surface over bex-api's already-shipped managed-Postgres GraphQL (`databases` / `database(id)` / `databaseConnectionInfo(id)` + `createDatabase` / `deleteDatabase`) — list, create, detail with on-demand connection-info reveal, delete — matching Render's **dashboard** noun (`database`, not the REST `postgres`) and its database IA. **Status:** done (2026-07-08)

## Tasks (in order)

| id   | title                                                                                                                                                                    | est | depends_on             |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --- | ---------------------- |
| t001 | Capture Render's databases dashboard IA via Playwright (list, create flow, info page with credentials) as design source — **DONE (design sourced from the already-captured Render `database` noun/IA in `docs/ADR006-bex-api.md` §Managed Postgres + `docs/ADR009-postgresql-management.md`; no fresh Render login available)** | 30m | —                      |
| t002 | `databases.graphql` (queries + both mutations) + codegen; postgres-tier catalog read (extend the `instanceTypes` surface to the postgres family — small `lego/backend` add) — **DONE (added `databaseInstanceTypes` query in `internal/postgres`; codegen against live bex-api)** | 40m | w5/m8/t001             |
| t003 | Databases list page + top-level nav item (dashboard becomes two-resource); status badges, loading/error/empty states — **DONE**                                            | 45m | w5/m8/t002             |
| t004 | Create-database dialog (name / plan from catalog / version / diskSizeGB / public) + delete with confirm (typed-name) — **DONE**                                            | 60m | w5/m8/t003             |
| t005 | Database detail page: status + metadata + on-demand connection-info reveal (masked password, copy buttons, psql command — never auto-fetched) — **DONE**                   | 60m | w5/m8/t002             |
| t006 | Live verify on the mock cluster (create → ready → connection info → delete) + screenshots to `.playwright-mcp/` — **DONE (full CNPG lifecycle verified via the real API + operator; needed two mock-cluster accommodations — `hcloud-volumes` SC alias + pin CNPG pod to control-plane, see memory `mock-cluster-managed-postgres-quirks`; UI screenshots via the `local-bex` stub)** | 30m | w5/m8/t004, w5/m8/t005 |
| t007 | Simplify — run `/simplify` over the code this milestone changed — **DONE (gated detail poll, shared `useCopyToClipboard` hook, dropped unused refetch mapping, HA metadata row, collapsed header)** | 30m | w5/m8/t006             |
| t008 | Test coverage — meaningful tests for query/mutation mapping, list/detail states, create/delete flows, connection-info reveal gating — **DONE (24 tests across lib/hooks/components/route)** | 30m | w5/m8/t006             |

## Definition of done

- A top-level **Databases** page renders the live `databases` query (status badges, loading / error / empty states); the dashboard nav becomes two-resource (Services + Databases).
- **Create** produces a `Database` CR via `createDatabase` (name, plan sourced from the w1/m8 tier catalog's postgres family — no hardcoded ladder in the frontend, version, diskSizeGB, public) and the new row converges to ready in the UI; **delete** confirms and cascades via `deleteDatabase`.
- The **detail page** shows Render's dashboard `database` shape (status + metadata) with connection info fetched **only on demand** via `databaseConnectionInfo(id)` — password masked with reveal + copy buttons and the psql command; never auto-fetched on page load.
- Render's deferred verbs (suspend/resume, failover, PITR, credentials rotation, pooler) are **omitted, not faked**, per bex-api's contract (`docs/ADR006-bex-api.md` §Managed Postgres).
- `yarn lint && yarn typecheck && yarn test && yarn build` all pass; the create → ready → connection-info → delete flow is verified live on the mock cluster with screenshots in `.playwright-mcp/`.

## Source + Goal linkage

- **Source:** inbox note `w5/001` (from `/pm-brainstorm` 2026-07-06), promoted via `/pm-brainstorm more milestones for w5` (2026-07-08); backend shipped per `docs/ADR006-bex-api.md` §Managed Postgres + `docs/ADR009-postgresql-management.md`; standing "consistent with render.com" directive.
- **Goal linkage:** `GOAL.md` #4 (PostgreSQL) + the dashboard pillar; pillar-1 API-first — a pure client of already-exposed ops, never a dashboard-only feature.
- **Expected outcome:** an operator creates, inspects (incl. connection string + password reveal), and deletes a managed Postgres from the dashboard — today this API surface has zero UI consumers and is curl-only.
- **Why now:** it is the last shipped bex-api surface with no dashboard; the w1/m8 tier catalog just landed postgres families, so the create dialog's plan picker can source real tiers instead of inventing a hardcoded ladder (the same fourth-copy trap w5/m7 closed for compute). The services/logs arc is done (m5–m7) save m4's closeout, so this is the next Render-parity page.

## Out of scope

- Render's deferred database verbs (suspend/resume, failover, PITR, credentials rotation, pooler strings) — unbuilt backend features; surface as gaps only.
- Prices anywhere in the UI (Metronome's, per the w1/m8 decision).
- REST/MCP additions — the page consumes the existing GraphQL surface; the only backend touch is the postgres-family catalog read (t002).
