# w6 · m14 — Deterministic workspace targeting: multi-workspace caller-tenant resolution

**Worker:** worker6 **Goal:** a caller with more than one workspace stops getting an arbitrary one — the implicit caller→workspace resolution becomes deterministic, write paths accept an explicit `ownerId` (Render's own contract), and the dashboard's mutations and audit log follow the switcher. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                             | est | depends_on             |
| ---- | --------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Deterministic default workspace: `TenantForIdentity` orders by membership `created_at` (oldest = default); document the contract  | 30m | — — **DONE** |
| t002 | `core.Base` seam: request-scoped workspace override for `Authorize`/`GetApp`/`Tenant`, honored only after a membership check      | 45m | t001 — **DONE** |
| t003 | REST: `POST /v1/services` + the other `LabelTenant`-stamping write paths honor `ownerId` (body/query), per Render's OpenAPI       | 45m | t002 — **DONE** |
| t004 | GraphQL: `createService` + sibling create mutations gain `ownerId`; MCP creates resolve via the session `select_workspace`        | 40m | t002 — **DONE** |
| t005 | Dashboard: thread `currentWorkspaceId` into create/write mutations (services wizard, env vars, databases, keyvalue)               | 40m | t004 — **DONE** |
| t006 | Root-cause + fix the `/settings` audit-log stale-workspace bug: align `use-audit-log.ts`'s workspace source with the switcher     | 30m | — — **DONE**           |
| t007 | Two-workspace end-to-end check: create in B lands in B; B's app never 403s its owner; audit log follows the switcher              | 30m | t003, t005, t006 — **DONE** |
| t008 | Render parity: sweep REST/GraphQL/MCP/UI `ownerId` semantics vs Render's OpenAPI; update ADR018 row 19's stale omission note      | 30m | t007 — **DONE** |
| t009 | Simplify: `/simplify` over the code this milestone changed                                                                        | 30m | t008 — **DONE** |
| t010 | Test coverage: meaningful tests for the resolution order, the membership-checked override, and wrong-workspace 403 regressions    | 40m | t008 — **DONE** |
| t011 | Closeout: verify the DoD holds, mark done, move to `w6/done/m14/`                                                                 | 15m | t010 — **DONE** |

## Definition of done

A user with two workspaces can, over REST and the dashboard, create a service in a **named** workspace and find it there (`ownerId` honored end-to-end); naming a workspace the caller is not a member of is rejected (403, never silently redirected); `GetApp` never rejects an owner because the membership join picked their other workspace (regression test at `core.Base` level); with no explicit workspace given, resolution is deterministically the caller's oldest membership — documented on `TenantForIdentity`; the Settings → Security & Compliance audit-log card shows the currently-selected workspace's events after switching (live or stub-verified click-through).

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w6` 2026-07-12 — `w6/done/m11/README.md` t003 ("no reliable way to target the second workspace over REST": `TenantForIdentity`, `lego/backend/internal/store/store.go:277`, is a bare join with no `ORDER BY`) and t004 (the `/settings` audit-log stale-workspace bug); `docs/ADR018-render-parity.md` row "Create service" whose `ownerId` omission rationale ("single workspace") went stale when w6/m1 shipped multi-workspace; `RESEARCH-workspaces.md` findings 7 (user id → default workspace) and 10 (every Render resource carries `ownerId` for workspace scoping).
- **Goal linkage:** w6's founding theme — multi-workspace as a real product surface (today only *lists* honor a workspace; auth checks, tenant gates, and creates all resolve arbitrarily); GOAL multi-tenancy.
- **Expected outcome:** multi-workspace goes from "lists work" to "everything works": no wrong-workspace creates, no spurious 403s on `core.Base.GetApp`'s tenant gate (base.go:255), no mis-scoped audit log.
- **Why now:** it is a live correctness bug class m11 hit on real prod, and every ingredient already exists (switcher state, m2's `ownerId` list plumbing, `core.WorkspaceSelections`, the membership store) — the wiring cost will never be lower.
- **Render parity:** **included** (t008) — the milestone touches REST/GraphQL/MCP and the dashboard, and its whole point is adopting Render's `ownerId` create contract; the parity task also corrects ADR018's stale row-19 note.

## The design decision that changed (t002) — and the escalation it prevented

The plan's shape for `core.Base.GetApp` was "let an owner reach an App in any workspace they are a **member** of". That is not safe, and the milestone shipped something stricter.

Roles are **per workspace** (`deploy/gitops/authz/model.fga`): the same person can be an `admin` of A and only a `viewer` of B. A membership-only gate would have let A's admin carry A's permissions into B — `Authorize(can_create)` passes against their own default workspace, then `GetApp` waves through B's App on membership alone. Proven, not theorised: with the gate weakened to membership-only, `TestMultiWorkspaceTargetingE2E` shows **carl, a viewer of `bravo`, successfully `DELETE`ing another workspace's service (204)**. With the shipped gate he gets 403.

So `GetApp` takes the **calling verb's relation** and authorizes it against the **App's own workspace** — Render's model, where a resource's permissions come from the owner it belongs to. That is what makes lifting the m11 403 safe. The cost is a signature change threaded to ~45 call sites (each passing the relation its own `Authorize` checked three lines above); the benefit is that the gate can no longer be weaker than the verb.

## Evidence (t007)

Two-workspace flows, run against **real infrastructure** — a live Postgres (throwaway container, the membership source of truth) and a live OpenFGA carrying the real `deploy/gitops/authz/model.json` — driving the **real REST router** through the **real** store-backed resolver (`api.tenantService`, not a fake). `lego/backend/internal/api/multiworkspace_e2e_test.go`, `TestMultiWorkspaceTargetingE2E` — **PASS**:

| leg                                                                                             | result                                                            |
| ----------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| dana's default workspace = her **oldest** membership (`alpha`), stable over 10 repeated calls   | ✅ (also `TestDefaultWorkspaceIsTheOldestMembership`, real PG)     |
| `POST /v1/services` with `ownerId: bravo` → App labeled `bravo` (not `alpha`)                   | ✅                                                                 |
| **the m11 bug**: `GET`/`restart` that service with **no** `ownerId` (resolution picks `alpha`)  | ✅ 200 — was 403 before                                            |
| `ownerId` of a workspace she is not a member of → **403**, and **nothing written** anywhere      | ✅                                                                 |
| carl (admin of `charlie`, **viewer** of `bravo`): `GET` bravo's service ok, `DELETE` → **403**   | ✅ — no cross-workspace privilege escalation                       |
| erin (member of neither) → 403                                                                   | ✅                                                                 |

Falsification checks (the tests are not vacuous): reverting `GetApp` to the pre-m14 gate makes the m11 regression test fail (`got forbidden, want it served`); weakening it to membership-only makes the escalation leg fail (`DELETE … 204, want 403`).

MCP leg: `TestMCP_CreateLandsInTheSelectedWorkspace` (real in-memory MCP session) — `create_web_service` after `select_workspace(B)` lands in B; an explicit `ownerId` beats the selection; a non-member `ownerId` is a tool error with nothing created. GraphQL + REST create legs: `apps/createowner_test.go`.

**Audit-log leg — stub-verified click-through** (real browser, offline dev stub; no live cluster was available, recorded not silently substituted, per the m10/m11 convention). Screenshots in `.playwright-mcp/`: `audit-workspace-a-acme-hq.png` shows five rows, **all `alpha-*`**; after clicking the workspace switcher (no reload, no navigation) `audit-workspace-b-acme-staging.png` shows three rows, **all `bravo-*`** — zero overlap, actor and status columns changed too, and switching back and forth tracked the selection every time. The network log confirms the `AuditLogs` operation was re-issued with the new owner each time (`ownerId: tea-localdefault…` ⇄ `tea-localsecond…`). This required teaching `dashboard/scripts/local-bex.mjs` an `auditLogs` resolver (it had none) plus Kratos settings-flow endpoints — without the latter the whole `/settings` page was unreachable offline, a pre-existing stub gap.

**Dashboard create legs are offline-verified** (vitest): the create mutations (`use-create-service` / `use-create-database` / `use-create-key-value`) each assert the switcher's `ownerId` reaches the mutation variables and that a workspace switch sends the new id.

Cosmetic, noticed in the screenshots and **not fixed** (out of scope): inside `/settings`' `max-w-2xl` column the audit table's Action column wraps one letter per line and Resource is clipped.

## Two bugs the milestone's own review caught (t009)

**1. A cross-tenant credential read in managed Postgres / Key Value — pre-existing, now closed.** `fetchDatabase` / `fetchKeyValue` were bare `client.Get`s with **no workspace check at all**: lists were scoped, but get-by-name was not. Any authenticated caller who knew a Database's name could read it from any workspace — and `PostgresConnectionInfo` / `KeyValueConnectionInfo` ride the same fetch, so that included **another workspace's connection string**. Proven with a probe (`mallory` of `tea-evil` reading `acme-db` of `tea-acme`), then closed by generalizing the App's gate into `core.Base.AuthorizeLabeled` and threading the verb's relation through both fetches (22 call sites). Regression tests: `postgres/ownerid_test.go`, `keyvalue/ownerid_test.go`. This was outside the milestone's DoD but squarely inside its subject; leaving a known cross-tenant credential read open while shipping a workspace-isolation milestone was not defensible.

**2. A wrong-workspace write this milestone nearly shipped — caught and fixed.** Because `GetApp` now serves an App from any workspace the caller has the relation in (which is what lifts the m11 403), the create path's "does this name exist?" probe started finding **other workspaces'** Apps. `POST /v1/services {name: web, ownerId: tea-2}` with `web` already existing in `tea-1` silently **redeployed tea-1's service** and created nothing in tea-2 — the exact wrong-workspace write the milestone exists to prevent. A name is claimed per workspace: the create now refuses with `ErrConflict`. Regression test: `TestCreate_NameTakenByAnotherWorkspaceIsAConflict` (plus `TestCreate_SameNameSameWorkspaceStillRedeploys`, so the create-or-update contract still holds).

## `/simplify` outcomes (t009)

Four review agents (reuse / simplification / efficiency / altitude) over the milestone diff. The core hoists were confirmed clean — `core.SelectedWorkspace` really is the single copy of the MCP ownerId precedence (all three private `resolveOwnerID` methods deleted), and `core.Base.resolveWorkspace` is the single decision point every gate reads.

**Applied:** the membership gate, written twice verbatim, extracted to `Base.requireMember` (one fail-closed contract, one place); `GetApp`'s five-step early-return ladder collapsed to three (two guards were already subsumed by `resolveWorkspace` returning `""`); the App-workspace gate generalized to `Base.AuthorizeLabeled` so Postgres/Key Value could share it (which is how the cross-tenant hole above got closed); an `IsMember` round-trip skipped when the named workspace **is** the caller's default — the dashboard names its workspace on every create, so without this every create paid an uncached SELECT; membership answers given their own `TTLCache` (they shared one with the tenant resolutions, and a `TTLCache` resets *wholesale* at `CacheMax`, so membership churn could have evicted every caller's default-workspace resolution); the MCP `deploy` tool now honors `select_workspace` like the other create tools; and the key-value list + Team page were made to follow the switcher (they didn't — the Team page had the identical `workspaces[0]` bug t006 fixed in the audit log).

**Added, not simplified:** `TestFetchByNameUsesTheVerbsOwnRelation` (`internal/api/relationguard_test.go`) — a reflection sweep over every verb of every feature asserting that the relation a verb hands the shared fetch is the one it authorized. It checks 46 cross-workspace verbs with no per-verb table, and it fails loudly when deliberately sabotaged (`Delete` fetching with `can_view`). The convention in `lego/backend/CLAUDE.md` is now enforced, not just written down.

**Skipped:** hoisting `gqlStr` into `gqlutil` (one line per site, matches local idiom); a shared factory for the three dashboard create hooks (a new abstraction, not reuse — the mutations differ in shape); collapsing the per-package test fakes (Go can't share unexported test helpers across packages, and a testing-support package would cost more than it saves).

## Known limitation (filed as `w6/013`, reproduced not theorised)

A verb still runs **two** gates: `Authorize` against the caller's **default** workspace, then the resource's own workspace. Effective permission is the intersection, and the first gate can only produce false negatives. Because `EnsureTenant` redeems invites **before** minting the personal tenant, an invited `viewer`'s default workspace is the **invited** one — so they cannot restart/delete/set-env-vars on services in **their own** workspace. Pre-existing (the same 403 happened before m14, from either gate) and outside this milestone's DoD, which scopes to `GetApp`'s gate. The fix (`core.Base.AuthorizeApp` — authorize once, against the resource's workspace) is designed and written up in `.pm/w6/013.md`.

## Root cause — the `/settings` audit-log stale-workspace bug (t006)

The audit table was never reading the switcher. `SecurityComplianceSection` resolved its owner with `useCurrentWorkspace()` (`dashboard/src/features/team/hooks/use-current-workspace.ts`) — a pre-switcher, w1/m9-era hook that runs its **own** `workspaces` query and returns `workspaces[0]`, i.e. the account's first (original auto-provisioned) workspace. It then passed that id into `useAuditLog(workspaceId)`, so `ownerId` was populated and the server scoped correctly — just to the wrong workspace. The switcher's selection lives in an entirely different source (`WorkspaceProvider` → `currentWorkspaceId`, persisted to `localStorage["bex.selectedWorkspaceId"]`), which the audit path never touched. It survived a hard reload because nothing was stale or cached: `workspaces[0]` is deterministically re-derived on every load, so a reload reproduced the same wrong id.

Fix: `useAuditLog()` now reads `useWorkspace().currentWorkspaceId` and skips until it resolves — the same shape as `useServices`/`useDatabases`. `loadMore`'s appended pages are tagged with the workspace they were fetched for, so a switch drops the previous workspace's paged-in tail (and any late-landing in-flight page) instead of stacking it under the new workspace's first page. Residual, out of scope here: `team-panel.tsx` still uses `useCurrentWorkspace()`, so the Team page has the same `workspaces[0]` pin.
