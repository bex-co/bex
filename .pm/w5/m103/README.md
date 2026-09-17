# w5 · m103 — Member-surface honesty: machine bindings, seat math, and owner attribution

**Worker:** worker5 **Goal:** the Team surface lists people, counts seats the way it charges for them, and names the workspace's owner truthfully **Status:** todo

## Tasks (in order)

| id   | title                                                                          | est | depends_on |
| ---- | ------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Evidence first: does an API-key binding appear as a member, a seat, a remove target? | 40m | —          |
| t002 | Keep machine bindings out of the member list and the seat formula                | 1h  | t001       |
| t003 | Refuse member mutations on a machine binding, pointing at the API-keys surface   | 30m | t002       |
| t004 | Owner attribution: surface the real owner instead of "oldest remaining admin"    | 45m | t001       |
| t005 | Render parity check for the member list, seat usage and owner fields             | 30m | t003, t004 |
| t006 | Simplify the code this milestone changed                                        | 30m | t005       |
| t007 | Test coverage: member/seat/owner projections                                    | 40m | t005       |
| t008 | Closeout                                                                        | 15m | t006, t007 |

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
