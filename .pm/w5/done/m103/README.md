# w5 · m103 — Member-surface honesty: machine bindings, seat math, and owner attribution

**Worker:** worker5 **Goal:** the Team surface lists people, counts seats the way it charges for them, and names the workspace's owner truthfully **Status:** done

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Evidence first: does an API-key binding appear as a member, a seat, a remove target? | 40m | — — **DONE** |
| t002 | Keep machine bindings out of the member list and the seat formula                | 1h  | t001 — **DONE** |
| t003 | Refuse member mutations on a machine binding, pointing at the API-keys surface   | 30m | t002 — **DONE** |
| t004 | Owner attribution: surface the real owner instead of "oldest remaining admin"    | 45m | t001 — **DONE** |
| t005 | Render parity check for the member list, seat usage and owner fields             | 30m | t003, t004 — **DONE** |
| t006 | Simplify the code this milestone changed                                        | 30m | t005 — **DONE** |
| t007 | Test coverage: member/seat/owner projections                                    | 40m | t005 — **DONE** |
| t008 | Closeout                                                                        | 15m | t006, t007 — **DONE** |

## Definition of done

On a store-backed environment:

- A workspace with N human members and K bound API keys lists exactly N members on REST, GraphQL, MCP and the Team page, and reports seat usage computed over the same N (matching what `store.CanAddMember` refuses on) — or, if t001's evidence shows machine bindings are already excluded, the milestone is closed as already-satisfied with that evidence recorded and the remaining tasks re-scoped to owner attribution only.
- Attempting `ChangeRole` or `Remove` against a machine binding returns a coded refusal naming the API-keys surface; the key's binding is unaffected.
- The workspace's surfaced owner/billing email follows `tenants.owner_identity_id` when one exists, falling back to the oldest admin only when it does not — and the fallback is visible as a fallback, not asserted as ownership.
- `identityResolved: false` (w4/070) keeps meaning "a human whose Kratos identity did not resolve", not "a machine".

## Source + Goal linkage

- **Source:** the 2026-09-16 owner/self-removal review. Two P2 observations came out of it: `store.BindClient` writes a `tenant_members` row keyed by the Hydra client id (`lego/backend/internal/store/store.go:800`) and `ListTenantMembers` (`internal/store/workspaces.go:209`) does not filter it, so a machine credential is shaped exactly like a member; and `workspaces.ownerEmail` (`internal/workspaces/service.go:760`) returns the oldest remaining admin's email, so removing a founding admin silently reassigns who a workspace appears to belong to.
- **Goal linkage:** ADR024 workspace members & roles, ADR018 Render parity (Render's `owner.usage.users` is people), ADR040/ADR030 billing — seat counts that disagree with the refusal formula are a billing-visible defect.
- **Expected outcome:** the Team page and seat usage describe humans; machine credentials live on the API-keys surface only; the owner shown is the owner.
- **Why now:** it is the same membership model `m101` and `m102` are hardening, and the guards those add must be written against the right definition of "member" — doing it afterwards means touching the same guards twice. It ranks last of the three because nothing here is an authorization defect.
- **Render parity task included:** yes — member list, seat usage and owner fields are all Render-shaped surfaces.

## t001 evidence (live on dev-5, 2026-09-17) — verdict: REAL GAP, and worse than filed

Bound an API key to a workspace that already had two human admins, then observed every surface. The milestone's premise was a code read; this is what the running system did.

| Observation | Before |
| --- | --- |
| Seat usage (`GET …/seat-usage`) | `used` went **2 → 3** the moment the key was created — a machine consumed a plan seat |
| REST / GraphQL / MCP member list | the key appeared as a member: `role: DEVELOPER`, `email: ""`, `identityResolved: false` — **indistinguishable from a human whose Kratos identity did not resolve** (w4/070), exactly as predicted |
| `PATCH …/members/{clientId}` `{"role":"ADMIN"}` | **HTTP 200.** The key was promoted, and the OpenFGA check flipped from `developer=true admin=false` to `developer=false admin=true` — a real `workspace:admin` tuple for a machine subject, written through a surface that presents itself as "the people in this workspace" |
| `DELETE …/members/{clientId}` | **HTTP 204.** The binding was deleted while the Hydra client stayed alive and still listed under `GET /v1/api-keys` — an orphaned key that silently authorizes nothing, with no feedback anywhere |

The escalation is the finding the filing did not anticipate: this was not only a cosmetic "shaped like a member" defect, it was a privilege-escalation path for machine credentials. Proceeded with the full milestone rather than the already-satisfied re-scope.

**Discriminator decision (t002):** an explicit `tenant_members.kind` column (migration 0129), not a subject-shape test — a Kratos identity id and a Hydra client id are both UUIDs, so any shape heuristic would be a guess.

## Closeout evidence (2026-09-17)

**DONE.** A member is a person; a key is a key; the owner is the owner.

**Reads split by intent, not by table.** `ListTenantMembers`, `CountTenantMembers`, `CountTenantAdmins`, both in-transaction last-admin rechecks, `SubjectIsWorkspaceAdmin` and ADR086's "another admin remains" preflight now filter `kind='user'`. `GetTenantMember`, `IsMember`, `TenantForIdentity` and `TenantForKey` deliberately do **not** — filtering those would break every API key. `TestEveryMemberQueryDecidesAboutKind` parses the package AST and fails any `tenant_members` SELECT that neither carries a kind **predicate** nor is listed with its reason; mutation-checked by deleting each filter in turn. (An earlier version of that sweep accepted merely *selecting* the column, which let a filter be removed silently — tightened to require a WHERE-clause predicate.)

**The sweep found two more recipient lists nobody had audited**, both now people-only: `ListNotifyRecipients` (every deploy failure was trying to notify bound client ids) and `ListBillingOwnerSubjects` (an API key was a billing contact).

**Live re-verification on dev-5, same probe as t001:** seat usage back to **2**, the member list shows only the two humans on REST and MCP, and `PATCH`/`DELETE`/GraphQL/MCP all answer **409 `MEMBER_IS_MACHINE`** ("that is an API key's workspace binding, not a member; manage it on the API keys page") while the binding survives and the key stays listed under `/v1/api-keys`.

**The startup backfill ran live against Hydra:** `machine-membership backfill: reclassified 1 API-key binding(s) as machine`. Keys bound before the migration default to `user` and SQL cannot classify them (Hydra is the only registry of client ids), so bex-api reconciles once at startup — logged, never fatal, since a failure degrades to the pre-m103 behavior rather than to a broken API.

**Owner attribution** now follows `tenants.owner_identity_id`, falling back to the oldest admin only for workspaces with no binding, and answering `""` rather than naming somebody else when a bound owner is unresolvable. Three unit tests cover the matrix; disabling the binding lookup reproduces both defects (`ownerEmail = "founder@example.com"` where the owner is somebody else).

**Migration 0129 verified up AND down against real Postgres** — column, CHECK constraint and partial index drop and recreate cleanly, and the up is re-runnable (`IF NOT EXISTS` / `DROP … IF EXISTS`).

**Gates:** backend `go test -p 1 ./...` against real Postgres + real OpenFGA (pristine schema) green; `make lint` 0 issues across four modules; dashboard 3,497 tests green.

**Evidence limits, stated plainly:**

- **The live owner-attribution flip was not completed.** The dev-5 stack degraded mid-walk — the CNPG Postgres pod restarted twice and `kubectl port-forward` kept resetting (`lost connection to pod`), so bex-api could not stay up long enough to re-read `GET /v1/owners/{id}` with a moved binding. The behavior is covered by three unit tests with a mutation check, and an earlier live read on the same workspace did return the owner's address. The flip itself is unproven live.
- No dashboard screenshot was taken after the fix: the Team list is server-driven and was verified on REST, GraphQL and MCP, which is where the member set is decided.
- The backfill reclassifies rows; it does **not** revoke OpenFGA tuples a previous escalation may have written. On dev-5 the `admin` tuple my own t001 probe created for a key persisted after reclassification. Any production workspace where somebody actually performed that promotion would need the tuple revoked by hand — worth a check before this is relied on, and called out here rather than silently assumed impossible.
