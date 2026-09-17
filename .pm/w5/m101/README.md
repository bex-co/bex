# w5 · m101 — Membership self-destruction refusals: the owner and self guards

**Worker:** worker5 **Goal:** no actor can sever the workspace's owner binding or their own access through the "manage members" verbs — the removal either sticks or is refused, never silently undone **Status:** todo

## Tasks (in order)

| id   | title                                                                                              | est | depends_on             |
| ---- | -------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Failing test: removing a personal-workspace owner resurrects admin on their next request            | 40m | —                      |
| t002 | Refuse removing the workspace owner (`OWNER_CANNOT_BE_REMOVED`), service gate + store-transaction guard | 1h  | t001                   |
| t003 | Refuse self-removal and self role change (`CANNOT_REMOVE_SELF` / `CANNOT_CHANGE_OWN_ROLE`)          | 45m | t002                   |
| t004 | Carry the three coded refusals across REST / GraphQL / MCP with stable codes                         | 40m | t003                   |
| t005 | Team panel: "you" marker, owner badge, disabled remove/role controls with the reason                 | 45m | t004                   |
| t006 | ADR024 amendment: the membership-invariant matrix                                                    | 30m | t003                   |
| t007 | Render parity check for the three refusals across every surface                                      | 30m | t005, t006             |
| t008 | Simplify the code this milestone changed                                                             | 30m | t007                   |
| t009 | Test coverage: the invariant matrix end to end                                                       | 45m | t007                   |
| t010 | Closeout                                                                                             | 15m | t008, t009             |

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
