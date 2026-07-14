# w6 · m18 — Deterministic ownerId threading: usage, API keys, GitHub connections

**Worker:** worker6 **Goal:** A multi-workspace caller can view/act on usage, API keys, and GitHub connections for a non-default workspace by naming it explicitly, on every surface — closing the read/bind-side gap `w6/m14` left when it fixed writes. **Status:** done

## Tasks (in order)

| id   | title                                                                                              | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | REST: `ownerId` param on `GET /v1/usage`, api-key list/create, GitHub connection endpoints → `core.WithWorkspace` | 40m | — — **DONE** |
| t002 | GraphQL: matching `ownerId` arg on the same three surfaces (m14 pattern)                            | 35m | t001 — **DONE** |
| t003 | MCP: honor `core.SelectedWorkspace` precedence for `get_usage`, api-key tools, GitHub tools          | 35m | t001 — **DONE** |
| t004 | Fix `use-current-workspace.ts`/`team-panel.tsx`; thread `currentWorkspaceId` into Usage + API-keys pages | 40m | — — **DONE** |
| t005 | Regression test: multi-workspace caller gets workspace-B's data when targeting B explicitly, workspace-A's by default (mirrors `TestMultiWorkspaceTargetingE2E`) | 30m | t001, t002, t003, t004 — **DONE** |
| t006 | Render parity — full-surface consistency check (REST/GraphQL/MCP/UI)                                 | 20m | t005 — **DONE** |
| t007 | Simplify — `/simplify` over the code this milestone changed                                          | 20m | t006 — **DONE** |
| t008 | Test coverage — meaningful tests for the behavior this milestone shipped                             | 30m | t006 — **DONE** |
| t009 | Closeout — DoD met → move milestone to `done/`                                                       | 10m | t008 — **DONE** |

## Definition of done

A caller belonging to 2+ workspaces can view/act on usage, API keys, and GitHub connections for a **non-default** workspace by naming it explicitly (REST `ownerId`, GraphQL arg, MCP selected-workspace), on every surface including the dashboard, with no regression to the default-workspace behavior for single-workspace callers.

## Source + Goal linkage

- **Source:** `w6/012.md`, filed during `w6/m14` (2026-07-13) — the read/bind-side residual `m14` explicitly left open. Proposed via `/pm-brainstorm more milestones to work on` 2026-07-13.
- **Goal linkage:** w6's multi-tenant-security mandate — `m14` closed this gap for writes (membership-checked `ownerId` on create); usage, api-keys, and GitHub connections were the noted residual reads/binds still resolving the caller's default workspace only.
- **Expected outcome:** no more silent wrong-workspace answers for multi-workspace users/agents on usage, keys, or GitHub connection status; the dashboard's workspace switcher actually drives what these pages show.
- **Why now:** root cause and fix shape are already fully diagnosed and written up in `w6/012.md`; small, self-contained, no new dependencies — was just sitting in the inbox since `m14`.
- **Render parity closing task: included** — touches REST/GraphQL/MCP and the dashboard.

## Evidence

**t001-t003 (REST/GraphQL/MCP):** `usage.monthToDateAt`, `apikeys.{CreateAPIKey,ListAPIKeys,RevokeAPIKey}`, and `github.{StartConnect,GetConnection,Disconnect,ListRepos}` all now take an `ownerID`/bind `core.WithWorkspace(ctx, ownerID)` before `Authorize` — the exact m14 seam. REST reads `?ownerId=` (GET) or `ownerId` (POST body); GraphQL adds an `ownerId` arg to every affected query/mutation; MCP resolves `core.SelectedWorkspace(s.Selections, req, in.OwnerID)`, requiring a new `Selections core.WorkspaceSelectionReader` field on all three services, wired to the same shared `core.WorkspaceSelections` instance in `internal/api/server.go` that `apps`/`postgres`/`keyvalue` already share (Usage's field is set post-construction since it's built in `cmd/api/main.go` before `NewServer` runs).

**A residual bug found and closed while implementing t001 (api-keys):** `w6/012.md`'s framing — "they already inherit m14's override for free... only adapter plumbing missing" — was true for usage and github (both already read through `Base.Tenant`), but **not** for api-keys. `ListAPIKeys` had **no tenant filter at all**: it returned every workspace's Hydra-backed keys to any caller who could manage their own. `RevokeAPIKey` had no ownership check: any key id was deletable by any caller with `can_manage_keys` on any workspace. Closed by adding `KeyBinder.TenantForKey` (a cache-backed reverse lookup, `internal/api/tenancy.go`, reusing `tenantService.Tenant`'s cache under the `oauth2` method key) and scoping both verbs to it — a cross-workspace revoke now refuses with `ErrForbidden`, mirroring the m14 gate `AuthorizeApp`/`AuthorizeDatabase`/`AuthorizeKeyValue` give CRD-backed resources.

**t004 (dashboard):** `use-current-workspace.ts` no longer exists — m14's own `/simplify` pass (t009) had already deleted it and fixed `team-panel.tsx`'s `workspaces[0]` bug; verified via grep (only stale comments reference the old hook, no live import). The actual remaining work was threading `currentWorkspaceId` into the Usage and API-keys pages: `use-usage.ts`/`use-usage-trend.ts`/`use-api-keys.ts`/`use-create-api-key.ts`/`use-revoke-api-key.ts` now read `useWorkspace().currentWorkspaceId`, skip until it resolves (mirroring `useServices`), and thread it as `ownerId` — `use-create-api-key` additionally refuses to mint until resolved (mirrors `useCreateService`'s null-ownerId guard). `definitions.ts` hand-edited (no live bex-api to regen codegen against, the established pattern other hand-added operations in that file already document).

**t005 (regression, against real Postgres + OpenFGA):** `internal/api/readside_ownerid_e2e_test.go`, `TestReadSideOwnerIDTargetingE2E` — dana (member of alpha + bravo) seeds distinct usage/git-connection/api-key data per workspace; proves `GET /v1/usage`, `GET /v1/git/connection`, and `GET/POST/DELETE /v1/api-keys` each answer bravo's data when targeted explicitly and alpha's (her default) otherwise, that a minted key binds to the named workspace, that revoking a key while targeting the WRONG workspace is 403, and that erin (a member of neither) is 403 on every targeted read. **PASS** against live ephemeral Postgres 17 + OpenFGA (docker, seeded with `deploy/gitops/authz/model.json`, same containers CI uses). Plus two fast unit tests with no live infra (`internal/apikeys/apikeys_test.go`): `TestListAPIKeysScopedToTargetWorkspace`, `TestRevokeAPIKeyRefusesCrossWorkspaceTarget`.

**t006 (Render parity):** ownerId is present on all three surfaces for every affected verb except GitHub's `StartConnect`/`Connect`/`Disconnect` on MCP (unregistered there before this milestone too — an admin/browser-flow action, not something Render's own MCP server exposes either) and the GitHub install **callback** (GitHub-driven redirect, no ownerId slot without new state-signing infra — explicitly out of scope, unchanged behavior). `docs/ADR018-render-parity.md` rows for API-key management, usage metering, and git connections updated with the `ownerId` targeting note + the two api-keys bugs closed.

**t007 (simplify):** four parallel review agents (reuse/simplification/efficiency/altitude). Reuse: clean (gqlStr package-local copies are the established m14-reviewed idiom; `TenantForKey` correctly delegates to `Tenant`'s cache). Altitude: clean (`RevokeAPIKey`'s inline ownership check is the right depth — apikeys has no CRD to fetch through, so it can't reuse `AuthorizeApp`'s shape; the `Selections` field extension follows the exact pattern `apps`/`postgres`/`keyvalue`/`projects` already use). Efficiency: one low-severity N+1 (`ListAPIKeys` calls `TenantForKey` once per key) — left as-is per the reviewing agent's own read: cache-backed after the first hit, and an admin-only, low-traffic surface. Simplification, applied: extracted `apikeys.Service.boundTenant` (removes a duplicated nil-Binding-check + `Tenant` lookup pair between `ListAPIKeys`/`RevokeAPIKey`); replaced a `gqlutil.IDArg()["id"]` map-index-and-discard with a direct `ArgumentConfig`; collapsed four duplicate anonymous-struct response types in the new e2e test into two named types + a generic `decodeJSON[T]` helper.

**t008 (test coverage):** covered inline above (t005) — the e2e test plus the two apikeys unit tests are the coverage this milestone shipped; all backend (`go test ./...`, including the live-infra-gated tests), dashboard (`yarn test`, 922 tests), `golangci-lint`, `yarn lint`, and `yarn typecheck` are green.
