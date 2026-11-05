# w5 · m267 — Render's workspace member mutations: PATCH and DELETE /owners/{ownerId}/members/{userId}

**Worker:** worker5 **Goal:** a Render API client can change a member's role and remove a member using the `userId` bex's own Render-shaped member list returns. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Store: batch reverse lookup from `own-` id to identity subject | 30m | — |
| t002 | REST: mount update-workspace-member and remove-workspace-member | 1h30m | t001 |
| t003 | Conformance tests, scope matrix and route-coverage guard | 45m | t002 |
| t004 | ADR018: drop the unsigned members omission and the stale environment-resources row | 15m | t003 |
| t005 | Render parity | 30m | t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

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
