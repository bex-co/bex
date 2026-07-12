# w6 · m10 — Member identity: user{email} on the members surface

**Worker:** worker6 **Goal:** the members CRUD surface (REST/GraphQL/MCP + dashboard Team page) identifies members by who they are (`userId` own- id + `email`), not by raw Kratos subjects — the same enriched `teamMember` shape the owners read API already ships. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                            | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Re-verify Render's member shape and decide the enriched field set + mutation keying                               | 30m | —          |
| t002 | Backend: enrich `members.MemberView` with `userId` (own- id) + `email` via `OwnerIDForSubject` + identity lookup  | 45m | t001       |
| t003 | Thread the enriched fields through the REST, GraphQL, and MCP fragments identically                               | 30m | t002       |
| t004 | Dashboard Team page: render email as the member's primary identity                                                | 40m | t003       |
| t005 | Docs: update ADR024's divergence note + refresh the ADR018 members row                                            | 20m | t004       |
| t006 | Render parity: sweep REST/GraphQL/MCP/UI for identical fields + semantics vs Render's team surface                | 30m | t005       |
| t007 | Simplify: `/simplify` over the code this milestone changed                                                        | 30m | t006       |
| t008 | Test coverage: meaningful tests for enrichment, omit-on-unset degradation, and Team page rendering                | 40m | t006       |
| t009 | Closeout: verify the DoD holds, mark done, move to `w6/done/m10/`                                                 | 15m | t008       |

## Definition of done

With `BEX_KRATOS_ADMIN_URL` configured against a live Kratos, `GET /v1/workspaces/{id}/members`, GraphQL `workspaceMembers`, and the MCP member-list tool all return each member's `userId` (opaque `own-` id) and `email`; with it unset, those fields degrade honestly (empty/omitted, no error) exactly like the owners API does. The dashboard Team page shows a member's email as the primary identity (raw subject demoted), with a sensible fallback when email is unavailable. `docs/ADR018-render-parity.md`'s "Workspace members & roles" row no longer lists the `user{email,name}` shape divergence as open (the no-`name`-trait residual is recorded).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for more w6` 2026-07-12 (proposal "Milestone 2", labeled m9 there — m8/m9 were taken by the security follow-up batch materialized the same day, so this landed as m10); `docs/ADR018-render-parity.md` row "Workspace members & roles" (◐ shape divergence: "members are keyed by identity subject, not `user{email,name}`"); w6/m7 closeout (opaque `own-` ids landed on the owners API only).
- **Goal linkage:** Render parity on the workspace surface — w6's founding theme (workspace lifecycle parity, `RESEARCH-workspaces.md`); GOAL.md #5 (multi-tenant) usability. Not a conflict with "Not in w6: Members & roles" — that bullet routes member *lifecycle* (invite/change-role/remove, shipped by `w4/m12`) away from w6; this milestone is identity-*shape* parity on that already-shipped surface, the exact axis w6/m7 owned.
- **Expected outcome:** both member surfaces (owners read API and members CRUD) present the identical enriched `teamMember` identity; the Team page becomes readable at a glance instead of showing raw identity ids.
- **Why now:** the enrichment ingredients shipped in w6 itself this week — `OwnerIDForSubject` (`lego/backend/internal/store/workspaces.go`, m7) and the Kratos-admin `IdentityReader` (`lego/backend/internal/workspaces/kratos.go`, m2) — and the owners API (`workspaces.ListMembers` → `renderTeamMember`) already proves the pattern end-to-end; the members surface merely never adopted it, so the wiring cost will never be lower. m7 also made the divergence more visible: the owners API now reports enriched members while the Team page still shows raw subjects.
- **Render parity:** **included** (t006) — feature dev touching all four surfaces (REST, GraphQL, MCP, dashboard UI).
