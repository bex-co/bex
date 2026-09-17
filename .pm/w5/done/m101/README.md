# w5 · m101 — Membership self-destruction refusals: the owner and self guards

**Worker:** worker5 **Goal:** no actor can sever the workspace's owner binding or their own access through the "manage members" verbs — the removal either sticks or is refused, never silently undone **Status:** done

## Tasks (in order)

| id   | title                                                                                              | est | depends_on             |
| ---- | -------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Failing test: removing a personal-workspace owner resurrects admin on their next request            | 40m | —                      — **DONE** |
| t002 | Refuse removing the workspace owner (`OWNER_CANNOT_BE_REMOVED`), service gate + store-transaction guard | 1h  | t001 — **DONE** |
| t003 | Refuse self-removal and self role change (`CANNOT_REMOVE_SELF` / `CANNOT_CHANGE_OWN_ROLE`)          | 45m | t002 — **DONE** |
| t004 | Carry the three coded refusals across REST / GraphQL / MCP with stable codes                         | 40m | t003 — **DONE** |
| t005 | Team panel: "you" marker, owner badge, disabled remove/role controls with the reason                 | 45m | t004 — **DONE** |
| t006 | ADR024 amendment: the membership-invariant matrix                                                    | 30m | t003 — **DONE** |
| t007 | Render parity check for the three refusals across every surface                                      | 30m | t005, t006 — **DONE** |
| t008 | Simplify the code this milestone changed                                                             | 30m | t007 — **DONE** |
| t009 | Test coverage: the invariant matrix end to end                                                       | 45m | t007 — **DONE** |
| t010 | Closeout                                                                                             | 15m | t008, t009 — **DONE** |

## Definition of done

On a store-backed environment with OpenFGA enforced:

- `members.Remove` targeting the subject in `tenants.owner_identity_id` is refused on REST (409), GraphQL (`OWNER_CANNOT_BE_REMOVED`), and MCP, and the `tenant_members` row still exists afterwards.
- `members.Remove` and `members.ChangeRole` targeting the **caller's own subject** are refused with `CANNOT_REMOVE_SELF` / `CANNOT_CHANGE_OWN_ROLE` on all three surfaces.
- A regression test proves the resurrection path is closed: after an attempted owner removal, `TenantForOwner` + `ensureGranted` cannot produce a workspace-admin tuple for a subject with no `tenant_members` row.
- The existing last-admin refusal, the ADR086 `AccountOffboarder` path, and invite acceptance are unchanged (their tests still pass untouched).
- The dashboard Team panel renders no enabled Remove control or role picker on the caller's own row or the owner's row, and states why.

## Source + Goal linkage

- **Source:** 2026-09-16 user request ("we should not allow the workspace owner to remove itself") plus the code walk it triggered: `lego/backend/internal/members/service.go:956` (`Remove`) and `:793` (`ChangeRole`) have no owner or self guard; `lego/backend/internal/api/tenancy.go:156` (`EnsureTenant`) resolves the personal workspace by `owner_identity_id` (`internal/store/workspaces.go:76`) and re-grants the admin tuple (`tenancy.go:344`) on every session request, while `core.Base.resolveWorkspaceUncached` (`internal/core/base.go:656`) skips the membership round-trip for the caller's default workspace.
- **Goal linkage:** ADR024 workspace members & roles / ADR012 auth — the role matrix is only meaningful if a revocation is durable; ADR008's multi-tenant hosting pillar depends on membership being the single source of truth.
- **Expected outcome:** removing a member is either durable or refused with a reason. No account can be left workspace-less by a member action (the owner binding blocks re-minting a personal workspace forever), and no removed owner keeps admin through the onboarding path.
- **Why now:** this is a live authorization defect, not a polish item — an admin removal that a background code path silently undoes is the worst shape an authz bug can take, and the opposite repair (honoring the removal) bricks the account. The refusal is the only correct answer in both directions and it is cheap; `m102`'s exit door and the deferred ownership transfer both depend on this rule existing first.
- **Render parity task included:** yes — the refusals change REST, GraphQL, MCP and the Team UI.

## Closeout evidence (2026-09-17)

**DONE.** Four refusals ship — `OWNER_CANNOT_BE_REMOVED`, `OWNER_ROLE_CANNOT_CHANGE`, `CANNOT_REMOVE_SELF`, `CANNOT_CHANGE_OWN_ROLE` — all 409 + stable code on REST, GraphQL and MCP, guarded twice (service `guardSelf`/`guardOwner`, plus `store.ErrOwnerMember`/`ErrOwnerRole` rechecks inside the advisory-lock transaction).

A **fourth code beyond the three the milestone named** was added deliberately: demoting the owner is the same resurrection through `ChangeRole` that removal is through `Remove` (`ensureGranted` re-writes the admin tuple on the owner's next request), so `OWNER_ROLE_CANNOT_CHANGE` refuses it. Recorded in the ADR024 matrix (t002 left this decision to t006).

**The defect was reproduced before it was fixed.** With `guardOwner` and the store recheck disabled, `TestOwnerRemovalIsRefusedAndCannotResurrectAdmin` fails with `remove owner: <nil>, want OWNER_CANNOT_BE_REMOVED` — the owner's row is deleted while `TenantForOwner` + `ensureGranted` hand the admin tuple straight back. `TestMemberMutatingVerbsRunTheGuards` was likewise mutation-checked: removing the `guardSelf` call from `Remove` fails it by name.

**DoD walk — live on `dev-5`, OpenFGA enforced** (bex-api on `:54050` against the dev-5 control-plane Postgres and a real OpenFGA carrying `deploy/gitops/authz/model.json`; two real Kratos identities, the owner's personal workspace `tea-dalov21jg4r9du1omc00` with `owner_identity_id` set by onboarding, a second admin seated by invite):

| Surface | remove owner | demote owner | remove self | change own role |
| --- | --- | --- | --- | --- |
| REST | 409 `OWNER_CANNOT_BE_REMOVED` | 409 `OWNER_ROLE_CANNOT_CHANGE` | 409 `CANNOT_REMOVE_SELF` | 409 `CANNOT_CHANGE_OWN_ROLE` |
| GraphQL | same code in `extensions` | same | same | same |
| MCP | `CODE: message` prefix | same | same | same |

Both `tenant_members` rows survived every attempt, and both admin tuples were still `allowed` in OpenFGA afterwards (checked directly against the store). `workspaceMembers` returned `isOwner`/`isSelf` correctly on all three surfaces.

**Dashboard**, live at desktop (1440) and narrow-mobile (375): the owner's row carries an **Owner** badge, the caller's row a **You** badge, and on both rows the role picker and Remove are rendered-but-disabled with the reason in a tooltip ("The workspace owner cannot be removed. Transferring ownership is not available yet." captured on hover). The mobile pass **caught and fixed a real regression this milestone introduced**: the new badge competed for width inside a `break-all` cell and squeezed the email to one character per line — fixed with `flex-wrap` + `min-w-0` + `shrink-0` badges, re-verified at both widths.

**Gates:** backend `go test -p 1 ./...` against real Postgres (pristine schema) green; `make lint` 0 issues across all four modules; dashboard typecheck + lint + 3,469 tests green. The real-OpenFGA `TestMembershipInvariantsE2E` passes (row↔tuple consistent on all four refusals, and a permitted teammate removal still drops both row and tuple — so the assertions cannot pass on a surface that refuses everything).

**Evidence limits, stated plainly:**

- The **Render side** of the parity comparison is **not captured**. The pinned artifact holds only the Team Members read query, and its workspace has one member, so neither self- nor owner-removal can be exercised against Render without destroying a real team. Recorded as uncapturable-from-this-tier in `docs/render-artifacts/team-members.graphql` and the ADR018 row, not claimed as mirrored behavior. The owner rule is bex-specific regardless: Render has no per-team owner identity that onboarding re-resolves per request.
- The Team panel's **loading skeleton was not captured side by side** with the ready state. This milestone adds no region, column, or control to the row — only an inline badge in the existing identity cell — and the shared `PanelTableSkeleton` is unchanged; desktop row height is identical before and after (compare the two desktop captures). The pre-existing generic-skeleton geometry drift for long wrapped emails is unchanged in kind.
- Getting here required reprovisioning the local kind/CAPD cluster (its 18-day-old CAPD node containers were gone). `mock-cluster.sh` then hung ~35 min on the `kubelet-csr-approver` helm pull from ghcr; without it kubelet serving CSRs stay pending and **every `kubectl port-forward` fails** with `tls: internal error`, which is what blocked dev-5. Approving the pending CSRs by hand unblocked it. Worth a harness note if it recurs.
