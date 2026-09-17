# w5 · m102 — Leave workspace: the self-service exit m101 requires

**Worker:** worker5 **Goal:** a member can deliberately leave a workspace they no longer belong in, through a verb that says so — without the members surface being able to do it by accident **Status:** todo

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | --------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Contract decision: what Leave refuses, what it invalidates, where the caller lands  | 40m | —          |
| t002 | `members.LeaveWorkspace` core verb with its guards and audit row                    | 1h  | t001       |
| t003 | Store: leave under the workspace advisory lock + resolver cache invalidation        | 45m | t002       |
| t004 | REST / GraphQL / MCP adapters for the Leave verb                                    | 45m | t002       |
| t005 | Dashboard: Leave workspace action, confirmation, and post-leave workspace re-select  | 1h  | t004       |
| t006 | Render parity check for the Leave surface                                           | 30m | t005       |
| t007 | Simplify the code this milestone changed                                            | 30m | t006       |
| t008 | Test coverage: leave, its refusals, and the post-leave resolution                    | 45m | t006       |
| t009 | Closeout                                                                            | 15m | t007, t008 |

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
