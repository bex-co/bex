# w5 · m372 — The workspace creator is an ordinary Admin, as on Render: another Admin can demote or remove them, and they can leave

**Worker:** worker5 **Goal:** bex's member model matches Render's, which has no Owner role: the creator is an Admin like any other, so another Admin may change their role or remove them and they may leave, while nobody removes themselves through Remove and a workspace always keeps a human Admin. **Status:** blocked (t001–t009 done; code shipped with this milestone; t010 Closeout waits on the production deploy that carries it, and on DoD item 6's production replay)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Contract: the creator is an Admin; amend ADR024 and ADR018; decide the workspace's owner email and where a departed creator lands — **DONE** | 45m | — |
| [t002](done/t002.md) | Store: removing, demoting or the leave of the creator detaches the onboarding binding in the same transaction — **DONE** | 1h | t001 |
| [t003](done/t003.md) | Members service and REST/GraphQL/MCP: drop the owner refusals, keep the self and last-admin rules — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Onboarding and authorization: a detached creator is neither restored nor left without a workspace — **DONE** | 1h | t002 |
| [t005](done/t005.md) | Dashboard Team page: no Owner label; the creator gets the same actions as any Admin (en and zh) — **DONE** | 45m | t003 |
| [t006](done/t006.md) | Retire the FUTURE-MAYBE "Workspace ownership transfer" entry — **DONE** | 10m | t001 |
| [t007](done/t007.md) | Render parity — **DONE** | 20m | t003, t004, t005, t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 40m | t007, t008 |
| [t010](t010.md) | Closeout | 15m | t009 |

Total: **10 tasks, about 6h10m**.

## Definition of done

On dev-5, with a workspace created by identity A and a second human Admin B:

1. B removes A with `DELETE /v1/owners/{ownerId}/members/{A's usr- id}` and gets 204. A's next session request lands in a newly created personal workspace; the old workspace answers 404 or 403 to A, and A stays removed across three further session requests (no admin tuple restored by `EnsureTenant`).
2. B changes A's role to Developer with `PATCH …/members/{A}` and gets 200; A's role reads `developer` on REST, GraphQL and MCP.
3. A leaves while B is an Admin, and succeeds; A trying to leave as the last human Admin is refused with the existing last-admin refusal.
4. A cannot remove themselves through Remove (`guardSelf`, the 2026-09-16 rule), and no `OWNER_*` code is answered by any surface.
5. The Team page shows no Owner label and offers A the same actions as any Admin, in en and zh.
6. On production, the REST remove and role change are replayed in a `bex-canary`-owned test workspace if a second QA identity is available; otherwise § Findings records that the replay ran on dev-5 only and why.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 9, item 4 (2026-11-12), filed by `/pmx 1,2,3,4` with w5 chosen. Re-checked at filing against `origin/main` `98b3587a8`.
- **Render:** [render.com/docs/team-members](https://render.com/docs/team-members), read 2026-11-12: the roles are Admin, Developer, Contributor, Viewer and Billing; there is no Owner role; "A workspace's creator is automatically assigned this role" (Admin); "Leave workspace" is permitted for every role; Admins change other members' roles, including other Admins'. Render's public API serves `PATCH`/`DELETE /owners/{ownerId}/members/{userId}` (200 and 204), which bex has served since w5/m267.
- **bex today:** `OWNER_CANNOT_BE_REMOVED`, `OWNER_ROLE_CANNOT_CHANGE` and `OWNER_CANNOT_LEAVE` (`lego/backend/internal/members/service.go:89-106`; `guardOwner` `:1410`, `guardOwnerLeaving` `:1227`, whose message reads "transferring ownership is not available yet"); store backstops `ErrOwnerMember`/`ErrOwnerRole` (`lego/backend/internal/store/members.go:36-46`); `IsOwner` on the member view (`members/service.go:287-291`, `:525`) drives the dashboard's Owner label and disabled actions (`dashboard/src/features/team/lib/member-action-reasons.ts:20`, `leave-eligibility.ts:33`). [ADR018](../../../docs/ADR018-render-parity.md)'s "Workspace members & roles" row reads ✅ and does not record these refusals, so the divergence is unrecorded.
- **Why the refusals exist, and why they can go:** [ADR024](../../../docs/ADR024-members.md) (`:124-149`): `EnsureTenant` (`lego/backend/internal/api/tenancy.go:159-183`) resolves the personal workspace by `owner_identity_id` and re-grants admin on every session, so a removal either never sticks or leaves the account without a workspace. Account deletion already detaches an owner safely ([ADR086](../../../docs/ADR086-account-deletion.md); `PGStore.RemoveAccountMember`, `lego/backend/internal/store/accounts.go:355-370`): it requires another human Admin, deletes the membership and clears `owner_identity_id` in one transaction. This milestone reuses that detach for remove, demote and leave.
- **Open decision, recorded as the proposal's recommendation (not a user decision):** this amends the owner rule w5/m101 built from the user's 2026-09-16 request ("we should not allow the workspace owner to remove itself"). The recommendation keeps that request as written: nobody removes themselves through Remove (`guardSelf` stays) and the last-human-Admin rule stays. It changes only what another Admin may do to the creator, and lets the creator leave when another human Admin remains. It rests on ADR008 Render parity first and the root AGENTS.md rule that an earlier design conflicting with parity is amended, not preserved. Still open: the user's explicit confirmation of this amendment. The alternative, keeping the owner refusals, is a divergence that needs the user's sign-off recorded in ADR018. The user scheduled this item with `/pmx 1,2,3,4`, which approves the work, not the divergence question.
- **Goal linkage:** Render parity first (ADR008); ADR024 members and roles; the multi-tenant pillar.
- **Expected outcome:** a Render client's member removal or role change behaves as on Render, and a workspace's creator is no longer stuck in it.
- **Why now:** w5/m267 (2026-11-05) serves Render's member mutation endpoints, so a Render client now hits a 409 with a bex-only code where Render answers 204; and with w5/m101 and w5/m102 refusing the owner's removal and leave, the creator has no way out short of deleting the account.
- **Render parity:** included (REST, GraphQL, MCP and the Team page change).

## Findings (2026-11-13)

- **DoD 1–4, against real Postgres and OpenFGA through the real REST router, not on dev-5.** dev-5 runs on the local kind/CAPD cluster, which cannot start on this host (CAPD worker "Too many open files"; `fs.inotify.max_user_instances=128`, the w5/m370 gate). The same steps run in `api.TestMembershipInvariantsE2E` (self refusals 409 with the kept codes; the creator demoted 200, re-promoted, then removed 204 with row, tuple and binding gone), `api.TestLeaveWorkspaceE2E` (the creator leaves 204; their next `EnsureTenant` lands in a new personal workspace; no admin re-granted) and `api.TestRemovingTheCreatorNeitherRestoresAdminNorStrandsTheAccount` (three sessions in a row, one new workspace, the old row stays gone). REST, GraphQL and MCP share the `members.Service` verbs, so no surface answers an `OWNER_*` code.
- **DoD 5** by component tests (no Owner label; the creator row offers the Admin actions; en and zh keys removed).
- **DoD 6 (production replay)** waits on the next batched production deploy pinning this code, and on a second QA identity for the `bex-canary`-owned test workspace; without one, the replay is the e2e suite above.
- **Race closed in t008:** a session's onboarding grant racing the creator's removal or demotion could have left them an admin tuple; both orders are now covered (onboarding re-checks the binding after a grant; removal revokes again after its commit).
- **Decision state:** unchanged. The amendment of the 2026-09-16 rule is the proposal's recommendation under ADR008 Render parity first; the user's explicit confirmation is still open (ADR024).

