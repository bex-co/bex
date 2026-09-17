# w5 · m102 — Leave workspace: the self-service exit m101 requires

**Worker:** worker5 **Goal:** a member can deliberately leave a workspace they no longer belong in, through a verb that says so — without the members surface being able to do it by accident **Status:** done

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | --------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Contract decision: what Leave refuses, what it invalidates, where the caller lands  | 40m | — — **DONE** |
| t002 | `members.LeaveWorkspace` core verb with its guards and audit row                    | 1h  | t001 — **DONE** |
| t003 | Store: leave under the workspace advisory lock + resolver cache invalidation        | 45m | t002 — **DONE** |
| t004 | REST / GraphQL / MCP adapters for the Leave verb                                    | 45m | t002 — **DONE** |
| t005 | Dashboard: Leave workspace action, confirmation, and post-leave workspace re-select  | 1h  | t004 — **DONE** |
| t006 | Render parity check for the Leave surface                                           | 30m | t005 — **DONE** |
| t007 | Simplify the code this milestone changed                                            | 30m | t006 — **DONE** |
| t008 | Test coverage: leave, its refusals, and the post-leave resolution                    | 45m | t006 — **DONE** |
| t009 | Closeout                                                                            | 15m | t007, t008 — **DONE** |

## Definition of done

On a store-backed environment with OpenFGA enforced:

- A non-owner, non-last-admin member can leave a workspace they belong to on REST, GraphQL, MCP and from the dashboard; afterwards their `tenant_members` row and role tuple are both gone and the workspace no longer appears in their workspace list.
- Leave is refused, with a coded reason, for the workspace owner (`OWNER_CANNOT_LEAVE`) and for the last admin (the existing last-admin rule).
- Leave acts only on the caller: no argument can make it remove somebody else.
- The action is audited as its own verb (not as a member removal by an admin).
- After leaving, the caller's next request resolves to another workspace they belong to — or their own — without a stale-cache window, and the dashboard lands them there rather than on a dead page.

## Source + Goal linkage

- **Source:** the 2026-09-16 owner/self-removal review that produced `w5/m101`. Once `Remove` refuses self-removal, bex has no exit at all for a member who legitimately wants out — today the only "exit" is an admin removing them, or the account-deletion path (ADR086), which is far larger than the intent.
- **Goal linkage:** ADR024 workspace members & roles; ADR018 Render parity — Render separates "remove a member" from "leave the team", and bex's members surface currently has only the former.
- **Expected outcome:** the membership lifecycle is complete in both directions: join by invite, exit by leaving, with the last-admin and owner invariants intact at both ends.
- **Why now:** it is the direct consequence of `m101`'s refusal. Shipping the refusal without the exit removes a capability tenants have today (self-removal, accidental as its current form is), so the two must land in sequence and close together.
- **Render parity task included:** yes — new REST/GraphQL/MCP verb plus a dashboard action.

## Contract (t001, decided 2026-09-17)

| Question | Decision |
| --- | --- |
| Wire shape | A dedicated verb — REST `DELETE /v1/workspaces/{id}/members/me`, GraphQL `leaveWorkspace(workspaceId:)`, MCP `leave_workspace` — never `removeWorkspaceMember(subject: me)`. None of the three accepts a subject, so the exit cannot be turned into a removal; that is the whole point of separating it from the verb `m101` just closed. |
| Authorization | Self-scoped: `can_view` on the named workspace (the `AcceptInvite` precedent), not `can_manage`. Leaving is not managing a teammate. The membership row fetch is what actually proves membership. |
| Refusals | The workspace owner → `OWNER_CANNOT_LEAVE` (409); the last admin → the existing shared `ErrLastAdmin` rule (400, uncoded, unchanged); a non-member → 404. |
| API keys | Leaving disposes the leaver's keys **in that workspace only**, exactly as `Remove` does (w2/m163) — a credential must not outlive the membership that justified it, whichever exit was taken. Keys in other workspaces are untouched. |
| Audit | Its own verb, `members.Leave`, with caller == target. The events feed must be able to say "left" rather than "was removed by an admin". |
| Cache | Two evictions, not one: `InvalidateTenant` (workspace resolution) **and** a new `InvalidateMembership(subject, tenantID)` for the positive membership cache, which `InvalidateTenant` never touched. |
| Seats | No special handling: `seatsUsed` counts membership rows, so a leave frees a seat by construction. |
| Landing | Select a remaining workspace from the **pre-leave** list, then refetch — the `DeleteWorkspaceCard` ordering, for the m13 reason. |

## Closeout evidence (2026-09-17)

**DONE.** `members.LeaveWorkspace` ships on all three surfaces plus a Leave card in the workspace-settings danger zone.

**A real stale-cache gap was found and closed.** `InvalidateTenant` evicts only the workspace-resolution cache; `IsMember` caches positives separately for `core.PositiveTTL`, so a request explicitly naming the workspace just left kept resolving into it for up to 30 s. It was never an access bypass — OpenFGA is the gate and the role tuple is revoked — but it is the m13 blank-switcher symptom. `InvalidateMembership` was added and is **proven necessary**: disabling the eviction fails `TestLeaveWorkspaceE2E` with `stale membership positive for the workspace just left`.

**DoD walk — live on `dev-5`, OpenFGA enforced**, with two real Kratos identities and three workspaces (a personal one with an owner binding, and `m102-shared` created through `createWorkspace`, which has none):

| Case | REST | GraphQL | MCP |
| --- | --- | --- | --- |
| Owner leaving | 409 `OWNER_CANNOT_LEAVE` | same code in `extensions` | `OWNER_CANNOT_LEAVE:` prefix |
| Last admin leaving | 400 last-admin | same | same |
| Ordinary member leaving | **204**, row + tuple gone | — | — |

The happy path was walked end to end: before the leave the caller listed two workspaces and held the `admin` tuple on the shared one; after `DELETE …/members/me` the tuple check returned `false` and `GET /v1/owners` returned only the remaining workspace **immediately** — no `PositiveTTL` window. The audit row is `members.Leave` with caller == target == the leaver, and no `members.Remove` row was written. The MCP tool's live schema exposes exactly one argument, `workspaceId` — no subject.

**Dashboard**, live at desktop (1440) and narrow-mobile (375): the Leave card sits in the danger zone above Delete; it is **enabled** for a non-owner, non-last-admin member, **disabled with the reason on hover** for the owner ("You own this workspace, so you cannot leave it. Transferring ownership is not available yet."), and disabled for the last admin. A real leave was performed through the UI — the confirmation named the workspace, and afterwards the caller landed on the overview of a workspace they still belong to with a populated switcher (the m13 failure avoided). The card renders within bounds at 375 px.

**A guard test caught a real regression mid-refactor.** t007 factored the shared teardown out of `Remove` and `LeaveWorkspace` into `endMembership` — and `TestMemberMutatingVerbsRunTheGuards` immediately failed, because the verbs no longer called the store mutators directly and would have silently stopped being swept. The sweep now follows the helper and only demands guards of **exported** verbs, and distinguishes the two legal shapes: a verb taking a `subject` must run `guardSelf` + `guardOwner`; a self-only verb must run `guardOwnerLeaving`. A self-only verb that later grows a subject parameter falls into the first branch and fails — that escalation is what is pinned shut. Mutation-checked both ways.

Two sibling enumeration tests were extended rather than bypassed: `TestEveryMembershipEndingVerbDisposesKeys` (w2/m163, which was written anticipating this milestone) now lists `LeaveWorkspace`, and the pinned `wantSweptVerbs` / MCP-inventory counts were bumped deliberately. The scope matrix was regenerated and reviewed: exactly three new entries, all `Write`, matching `remove_workspace_member` — no silent downgrade.

**Gates:** backend `go test -p 1 ./...` against real Postgres + real OpenFGA (pristine schema) green; `make lint` 0 issues across four modules; dashboard typecheck + lint + 3,497 tests green.

**Evidence limits, stated plainly:**

- Render's own leave-team mutation is **not captured** — the pinned artifact holds only the Team Members read query, and its workspace has a single member. Render does separate leaving from removing (which is why bex spells them as two verbs), but the exact mutation, arguments and refusal set are recorded as uncapturable from this tier rather than claimed. The REST route in particular is a deliberate bex extension: Render manages members only through its dashboard GraphQL.
- The Team panel's loading skeleton was not captured side by side; the danger-zone skeleton was widened to two cards when a second card can render, and the Leave card's geometry was checked at both widths in the ready state only.
- An unrelated defect surfaced during the walk and was filed rather than fixed here: the workspace-settings **Name input keeps the previous workspace's name after a switch** while Plan/ID/Created update — a wrong-target rename hazard, since Save sits next to it. Filed as `w5/059`.
