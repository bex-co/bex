# w1 · m9 — Tenant onboarding: real workspaces + OpenFGA enforced in prod

**Worker:** worker1 **Goal:** The control-plane store (w1/m2) grows real tenancy: a new identity gets a tenant row + membership, API keys bind to their tenant's `workspace:tea-<id>`, every verb authorizes against the caller's workspace instead of `workspace:default`, and prod flips from allow-all to enforced OpenFGA. **Status:** todo

## Tasks (in order)

| id   | title                                                                           | est | depends_on   |
| ---- | ------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Tenant mint on first login: Kratos identity → tenant row + `tenant_members`     | 30m | w1/m2/t007   |
| t002 | Key→tenant binding: OAuth2 client → `tenant_oauth_clients` + FGA membership     | 30m | t001         |
| t003 | Scope `core.Authorize` + list/get to the caller's workspace (retire `default`)  | 30m | t002         |
| t004 | Enable `BEX_OPENFGA_URL` in the prod api Deployment + cross-tenant 403 check    | 25m | t003         |
| t005 | Simplify — `/simplify` over the code this milestone changed                     | 20m | t004         |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped        | 30m | t004         |

## Definition of done

On a cluster with `BEX_CP_DB_URI` set: two identities sign up → two tenant rows with `tenant_members` entries; each mints an API key; each key lists **only** its own services and gets **403** on the other tenant's service; the prod api Deployment has `BEX_OPENFGA_URL` uncommented. `TestAuthzGuardsEveryVerb` still passes with workspace-scoped checks.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` (2026-07-08); w1/m2 deferrals (t004's "tenant_members write path is a later onboarding task"; m2's OpenFGA note that the `workspace:default` placeholder is replaced "when the control plane grows real workspaces"); 2026-07-08 docs-vs-code audit (prod `BEX_OPENFGA_URL` deliberately commented out pending this work).
- **Goal linkage:** vision roadmap #1 (Postgres control plane — the tenants half); multi-tenancy per docs/control-plane.md. Auth stays bought (Kratos/Hydra/OpenFGA; only mapping keys in Postgres) — DO_NOT_DO-compliant.
- **Expected outcome:** prod authz stops being allow-all; tenants are isolated at the API layer; the OpenFGA model's "(w1/m2)" placeholder comment is retired.
- **Why now:** m2 is one acceptance task from done, and this is the sole blocker for enforced authz in prod — a standing security gap, not a nice-to-have.
