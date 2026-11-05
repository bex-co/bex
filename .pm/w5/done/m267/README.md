# w5 · m267 — Render's workspace member mutations: PATCH and DELETE /owners/{ownerId}/members/{userId}

**Worker:** worker5 **Goal:** a Render API client can change a member's role and remove a member using the `userId` bex's own Render-shaped member list returns. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Store: batch reverse lookup from `own-` id to identity subject — **DONE** | 30m | — |
| t002 | REST: mount update-workspace-member and remove-workspace-member — **DONE** | 1h30m | t001 |
| t003 | Conformance tests, scope matrix and route-coverage guard — **DONE** | 45m | t002 |
| t004 | ADR018: drop the unsigned members omission and the stale environment-resources row — **DONE** | 15m | t003 |
| t005 | Render parity — **DONE** | 30m | t004 |
| t006 | Simplify — **DONE** | 20m | t005 |
| t007 | Test coverage — **DONE** | 45m | t005, t006 |
| t008 | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 4h45m**.

## Definition of done

- `PATCH /v1/owners/{ownerId}/members/{userId}` with a `userId` returned by `GET …/members` changes that member's role; `DELETE` removes them; owner removal is refused by name.
- Both operations pass the pinned-spec conformance and route-coverage guard; ADR018 carries no unsigned `—` for them and no stale environment-resources row.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w5` 2026-11-05 item 3; pinned OpenAPI `update-workspace-member`/`remove-workspace-member`; ADR018 omissions row (unsigned `—`); `own-` ids in GET cannot reach native routes.
- **Goal linkage:** ADR024 members; ADR006/ADR018 REST compatibility; ADR008 parity first.
- **Expected outcome:** Render API clients manage workspace members end to end.
- **Why now:** a client-visible contract gap with no owner, and the list endpoint already hands out ids nothing can use.
- **Render parity:** included (REST change; cross-surface semantics checked).

## Closeout (2026-11-05)

- **Routes:** `PATCH`/`DELETE /v1/owners/{ownerId}/members/{userId}` in the workspaces REST adapter (`UpdateMember`/`RemoveMember`), authorizing `can_manage` before any read, resolving the `own-` userId with the new indexed `store.SubjectForOwnerID` (`owner_ids.own_id` is unique), mapping Render's `teamMemberRole` by inverting `renderTeamMemberRole`, and delegating to `members.Service.ChangeRole`/`Remove` (wired through `memberMutator` in `api/server.go`), so authorization, the owner safeguards (w5/m101) and audit are the members verbs'. PATCH answers Render's `teamMember` (200), DELETE 204; an unknown userId is 404, a role outside the enum a named 400.
- **Contract guards:** the `userId` path pattern is widened to `^(?:usr|own)-[0-9a-v]{20}$` for these two operations (`renderPathParameterPatternCompatibility`, classified `ids.Owner`); route inventory, operation-omission list, scope matrix and verb sweeps updated.
- **Render parity (t005):** REST matches Render's two operations; Render's MCP and CLI have no member tools, and GraphQL/dashboard already mutate members through the same members verbs. Drift found and filed: Render's `userId` is `usr-…`, bex answers `own-…` (w5/179).
- **Simplify (t006):** review applied — `can_manage` first, indexed reverse lookup instead of listing members (no Kratos lookups or own-id minting on the mutation path), one identity lookup for the PATCH body, role table inverted from the forward map, narrow adapter.
- **Tests (t007):** workspaces service test (every Render role maps; lowercase role 400; unknown userId and raw subject 404; members refusals pass through), real-DB store test for `SubjectForOwnerID`, REST contract guards.
- **ADR018:** the unsigned members omission row is replaced by evidence on the Workspace members row; the stale "Environment resource mutation paths" omission (shipped in w8/188) is removed.
- **Not performed:** a production walk (it needs a second member in a QA workspace); the behavior is covered by the tests above and the members verbs' existing owner-safeguard tests.
